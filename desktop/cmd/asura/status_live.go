package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"asura/desktop/internal/runner"
)

// The live half of `asura status`: which services are up, which versions run, and
// with --json the report the shell shows as its Updates page.
//
// `asura status --json` is called by the daemon, on login and then on a timer.
// It publishes the line this prints, unread, on its "updates" topic, and the
// bar's update glyph and the Hub's Updates page read the fields below.

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
	// Behind counts the Asura packages with a newer version waiting.
	Behind  int          `json:"pendingUpdates"`
	Updates []updateItem `json:"updates"`
	// Recent is the history a current machine shows.
	Recent  []updateItem `json:"recent"`
	Channel string       `json:"channel"`
	// Packages is what the upstream Void repository has waiting.
	Packages      []updateItem `json:"packages"`
	SystemPending int          `json:"systemUpdates"`
}

// xbpsUpgradable matches lines from `xbps-install -un`:
//
//	[*] <name>-<old-version> update to <new-version> ...
var xbpsUpgradable = regexp.MustCompile(`^(\S+?)-([0-9]\S*)\s+update to\s+(\S+)`)

func parseUpgradable(output string) []updateItem {
	var items []updateItem
	for _, line := range strings.Split(output, "\n") {
		clean := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "[*]"))
		if match := xbpsUpgradable.FindStringSubmatch(clean); match != nil {
			newVer := strings.TrimRight(match[3], ",)")
			items = append(items, updateItem{Name: match[1], Old: match[2], New: newVer})
		}
	}
	return items
}

// buildStatusReport never fails: a machine without xbps reports itself as
// current, which is what the page shows until there is something to say.
func buildStatusReport(root string, run runner.Func) statusReport {
	report := statusReport{
		Installed: version, Latest: version,
		Updates: []updateItem{}, Recent: []updateItem{}, Packages: []updateItem{},
		Channel: "void",
	}
	if out, err := run("xbps-query", "-p", "pkgver", "asura"); err == nil && strings.TrimSpace(out) != "" {
		report.Installed = strings.TrimPrefix(strings.TrimSpace(out), "asura-")
		report.Latest = report.Installed
	}
	if out, err := run("xbps-install", "-un"); err == nil {
		for _, item := range parseUpgradable(out) {
			switch {
			case item.Name == "asura":
				report.Latest = item.New
				report.Updates = append(report.Updates, item)
			case strings.HasPrefix(item.Name, "asura-"):
				report.Updates = append(report.Updates, item)
			default:
				report.Packages = append(report.Packages, item)
			}
		}
	}
	report.Behind = len(report.Updates)
	report.Available = report.Behind > 0
	report.SystemPending = len(report.Packages)
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
	fmt.Fprintf(out, "shell:     %s\n", unitSummary(run, "quickshell"))
	fmt.Fprintf(out, "daemon:    %s\n", unitSummary(run, "asurad"))

	versions := []string{"asura " + version}
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

// unitSummary is one service's state as a phrase: "active (running)", "inactive", etc.
func unitSummary(run runner.Func, service string) string {
	svcPath := filepath.Join(userServiceDir(), service)
	out, err := run("sv", "status", svcPath)
	if err != nil {
		if strings.Contains(out, "file does not exist") || strings.Contains(err.Error(), "file does not exist") {
			return "not installed"
		}
		return "unknown (the user service manager is not reachable)"
	}
	line := strings.TrimSpace(out)
	switch {
	case strings.HasPrefix(line, "run:"):
		return "active (running)"
	case strings.HasPrefix(line, "down:"):
		return "inactive"
	case strings.HasPrefix(line, "want:"):
		return "active (waiting)"
	case strings.HasPrefix(line, "fail:") && strings.Contains(line, "file does not exist"):
		return "not installed"
	default:
		return "active"
	}
}
