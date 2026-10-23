package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"zenith/desktop/internal/runner/runnertest"
)

const aptListOutput = `Listing...
brave-browser/stable 1.98.60 amd64 [upgradable from: 1.97.57]
ca-certificates/resolute-updates,resolute-updates,resolute-security 20260816ubuntu0.26.04.1 all [upgradable from: 20260601~26.04.1]
zenith/stable 0.2 amd64 [upgradable from: 0.1-pre-alpha]
zenith-agent/stable 0.2 amd64 [upgradable from: 0.1]
`

// The shell's Updates page reads this line, so the field names, the split
// between Zenith's packages and Ubuntu's, and the one-line form are what is
// pinned. A machine with nothing waiting must still send arrays, not nulls.
func TestStatusReport(t *testing.T) {
	fake := runnertest.New().
		Out("dpkg-query -W -f=${Version} zenith", "0.1-pre-alpha").
		Out("apt list --upgradable", aptListOutput)

	var out bytes.Buffer
	if code := statusJSON(t.TempDir(), &out, fake.Run); code != exitOK {
		t.Fatalf("exit %d", code)
	}
	if strings.Count(strings.TrimSuffix(out.String(), "\n"), "\n") != 0 {
		t.Fatalf("the report spans several lines:\n%s", out.String())
	}

	var got struct {
		Installed string       `json:"installedVersion"`
		Latest    string       `json:"latestVersion"`
		Available bool         `json:"available"`
		Behind    int          `json:"pendingUpdates"`
		Updates   []updateItem `json:"updates"`
		Packages  []updateItem `json:"packages"`
		System    int          `json:"systemUpdates"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Installed != "0.1-pre-alpha" || got.Latest != "0.2" || !got.Available || got.Behind != 2 {
		t.Errorf("zenith lane wrong: %+v", got)
	}
	if got.System != 2 || len(got.Packages) != 2 || got.Packages[0].Name != "brave-browser" ||
		got.Packages[1].Old != "20260601~26.04.1" {
		t.Errorf("ubuntu lane wrong: %+v", got.Packages)
	}
}

func TestStatusReportWithNothingToSay(t *testing.T) {
	// No dpkg, no apt: current, and arrays rather than null.
	fake := runnertest.New().
		Fail("dpkg-query -W -f=${Version} zenith", runnertest.NotInstalled("dpkg-query")).
		Fail("apt list --upgradable", runnertest.NotInstalled("apt"))
	var out bytes.Buffer
	statusJSON(t.TempDir(), &out, fake.Run)
	for _, want := range []string{`"available":false`, `"updates":[]`, `"packages":[]`, `"recent":[]`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report lacks %s: %s", want, out.String())
		}
	}
}

func TestUnitSummary(t *testing.T) {
	show := func(unit string) string {
		return "systemctl --user show " + unit + " -p LoadState -p ActiveState -p SubState --no-pager"
	}
	fake := runnertest.New().
		Out(show("a.service"), "LoadState=loaded\nActiveState=active\nSubState=running\n").
		Out(show("b.service"), "LoadState=not-found\nActiveState=inactive\nSubState=dead\n").
		Out(show("c.service"), "LoadState=loaded\nActiveState=inactive\nSubState=inactive\n").
		Fail(show("d.service"), runnertest.Exit(1, "Failed to connect to bus"))

	for unit, want := range map[string]string{
		"a.service": "active (running)",
		"b.service": "not installed",
		"c.service": "inactive",
		"d.service": "unknown (the user service manager is not reachable)",
	} {
		if got := unitSummary(fake.Run, unit); got != want {
			t.Errorf("%s: %q, want %q", unit, got, want)
		}
	}
}
