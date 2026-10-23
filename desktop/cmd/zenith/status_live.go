package main

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"zenith/desktop/internal/runner"
)

// The live half of `zenith status`: which units are up, which versions run, and
// with --json the report the shell shows as its Updates page.
//
// `zenith status --json` is called by the daemon, on login and then on a timer.
// It publishes the line this prints, unread, on its "updates" topic, and the
// bar's update glyph and the Hub's Updates page read the fields below. The
// names are the ones Ryoku's `status --json` used, because the QML reading them
// is the same QML. Where Ryoku asked pacman, this asks apt.

// updateItem is one row of the Updates page: a package and the two versions
// it would move between.
type updateItem struct {
	Name string `json:"name"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

type statusReport struct {
	Installed string `json:"installedVersion"`
	Latest    string `json:"latestVersion"`
	Available bool   `json:"available"`
	// Behind counts the Zenith packages with a newer version waiting.
	Behind  int          `json:"pendingUpdates"`
	Updates []updateItem `json:"updates"`
	// Recent is the history a current machine shows; apt keeps none.
	Recent  []updateItem `json:"recent"`
	Channel string       `json:"channel"`
	// Packages is the other lane: what Ubuntu's archive has waiting, kernel
	// included. `zenith update` moves Zenith's own packages and nothing else,
	// so these never make Available true.
	Packages      []updateItem `json:"packages"`
	SystemPending int          `json:"systemUpdates"`
}

// upgradable matches a line of `apt list --upgradable`:
//
//	name/suite[,suite...] new-version arch [upgradable from: old-version]
var upgradable = regexp.MustCompile(`^([^/\s]+)/\S+\s+(\S+)\s+\S+\s+\[upgradable from:\s+(\S+)\]`)

func parseUpgradable(output string) []updateItem {
	var items []updateItem
	for _, line := range strings.Split(output, "\n") {
		if match := upgradable.FindStringSubmatch(line); match != nil {
			items = append(items, updateItem{Name: match[1], New: match[2], Old: match[3]})
		}
	}
	return items
}

// buildStatusReport never fails: a machine without apt or dpkg reports itself as
// current, which is what the page shows until there is something to say.
func buildStatusReport(root string, run runner.Func) statusReport {
	report := statusReport{
		Installed: version, Latest: version,
		Updates: []updateItem{}, Recent: []updateItem{}, Packages: []updateItem{},
	}
	if out, err := run("dpkg-query", "-W", "-f=${Version}", "zenith"); err == nil && strings.TrimSpace(out) != "" {
		report.Installed = strings.TrimSpace(out)
		report.Latest = report.Installed
	}
	if out, err := run("apt", "list", "--upgradable"); err == nil {
		for _, item := range parseUpgradable(out) {
			switch {
			case item.Name == "zenith":
				report.Latest = item.New
				report.Updates = append(report.Updates, item)
			case strings.HasPrefix(item.Name, "zenith-"):
				report.Updates = append(report.Updates, item)
			default:
				report.Packages = append(report.Packages, item)
			}
		}
	}
	report.Behind = len(report.Updates)
	report.Available = report.Behind > 0
	report.SystemPending = len(report.Packages)
	report.Channel, _ = aptChannel(root)
	return report
}

// statusJSON prints the report as one line: the daemon republishes it as a line
// on a socket, so it must not span several.
func statusJSON(root string, out io.Writer, run runner.Func) int {
	encoded, err := json.Marshal(buildStatusReport(root, run))
	if err != nil {
		fmt.Fprintln(out, err)
		return exitFail
	}
	fmt.Fprintf(out, "%s\n", encoded)
	return exitOK
}

// printLiveStatus adds the lines that need the running machine.
func printLiveStatus(out io.Writer, run runner.Func) {
	fmt.Fprintf(out, "shell:     %s\n", unitSummary(run, "zenith-shell.service"))
	fmt.Fprintf(out, "daemon:    %s\n", unitSummary(run, "zenithd.service"))

	versions := []string{"zenith " + version}
	// "Quickshell 0.3.2 (revision ...)".
	if fields := strings.Fields(firstOutput(run, "quickshell", "--version")); len(fields) > 1 {
		versions = append(versions, "quickshell "+fields[1])
	}
	var hypr struct {
		Version string `json:"version"`
	}
	if json.Unmarshal([]byte(firstOutput(run, "hyprctl", "version", "-j")), &hypr) == nil && hypr.Version != "" {
		versions = append(versions, "hyprland "+hypr.Version)
	}
	fmt.Fprintf(out, "versions:  %s\n", strings.Join(versions, ", "))
}

func firstOutput(run runner.Func, name string, args ...string) string {
	out, err := run(name, args...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// unitSummary is one unit's state as a phrase: "active (running)".
func unitSummary(run runner.Func, unit string) string {
	out, err := run("systemctl", "--user", "show", unit,
		"-p", "LoadState", "-p", "ActiveState", "-p", "SubState", "--no-pager")
	if err != nil {
		return "unknown (the user service manager is not reachable)"
	}
	state := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if key, value, found := strings.Cut(strings.TrimSpace(line), "="); found {
			state[key] = value
		}
	}
	switch {
	case state["LoadState"] == "not-found":
		return "not installed"
	case state["SubState"] != "" && state["SubState"] != state["ActiveState"]:
		return state["ActiveState"] + " (" + state["SubState"] + ")"
	}
	return state["ActiveState"]
}
