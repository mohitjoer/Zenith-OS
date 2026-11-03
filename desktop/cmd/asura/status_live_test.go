package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"asura/desktop/internal/runner/runnertest"
)

const xbpsListOutput = `[*] brave-browser-1.97.57_1 update to 1.98.60_1 ...
[*] ca-certificates-20260601_1 update to 20260816_1 ...
[*] asura-0.1-pre-alpha update to 0.2 ...
[*] asura-agent-0.1 update to 0.2 ...
`

// The shell's Updates page reads this line, so the field names, the split
// between Asura's packages and upstream Void's, and the one-line form are what is
// pinned. A machine with nothing waiting must still send arrays, not nulls.
func TestStatusReport(t *testing.T) {
	fake := runnertest.New().
		Out("xbps-query -p pkgver asura", "asura-0.1-pre-alpha\n").
		Out("xbps-install -un", xbpsListOutput)

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
		t.Errorf("asura lane wrong: %+v", got)
	}
	if got.System != 2 || len(got.Packages) != 2 || got.Packages[0].Name != "brave-browser" ||
		got.Packages[1].Old != "20260601_1" {
		t.Errorf("void lane wrong: %+v", got.Packages)
	}
}

func TestStatusReportWithNothingToSay(t *testing.T) {
	fake := runnertest.New().
		Fail("xbps-query -p pkgver asura", runnertest.NotInstalled("xbps-query")).
		Fail("xbps-install -un", runnertest.NotInstalled("xbps-install"))
	var out bytes.Buffer
	statusJSON(t.TempDir(), &out, fake.Run)
	for _, want := range []string{`"available":false`, `"updates":[]`, `"packages":[]`, `"recent":[]`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report lacks %s: %s", want, out.String())
		}
	}
}

func TestUnitSummary(t *testing.T) {
	svcDir := filepath.Join(t.TempDir(), "service")
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(svcDir))

	status := func(svc string) string {
		return "sv status " + filepath.Join(svcDir, svc)
	}
	fake := runnertest.New().
		Out(status("a"), "run: a: (pid 1234) 10s\n").
		Out(status("b"), "fail: b: unable to change to service directory: file does not exist\n").
		Out(status("c"), "down: c: 10s, normally up\n").
		Fail(status("d"), runnertest.Exit(1, "fail: d: unable to change to service directory: permission denied"))

	for svc, want := range map[string]string{
		"a": "active (running)",
		"b": "not installed",
		"c": "inactive",
		"d": "unknown (the user service manager is not reachable)",
	} {
		if got := unitSummary(fake.Run, svc); got != want {
			t.Errorf("%s: %q, want %q", svc, got, want)
		}
	}
}
