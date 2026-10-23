package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reload goes through the shell's unit, never past it: the cover first, then
// `systemctl --user reload`, then a wait for a new main pid that answers ready.
// systemctl and quickshell are shims, so the test restarts nothing.
func TestReloadRestartsTheUnitAndWaitsForTheShell(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	state := filepath.Join(bin, "reloaded")
	writeShim(t, filepath.Join(bin, "systemctl"), `echo "$*" >> `+log+`
case "$*" in
  *" reload "*) : > `+state+` ;;
  *MainPID*) if [ -e `+state+` ]; then echo 4242; else echo 4141; fi ;;
esac`)
	writeShim(t, filepath.Join(bin, "quickshell"), `echo true`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	oldCover := reloadCoverBegin
	reloadCoverBegin = func() string { return "" }
	defer func() { reloadCoverBegin = oldCover }()

	if got := (&daemon{}).reload(); got != "ok" {
		t.Fatalf("reload = %q, want ok", got)
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "--user reload zenith-shell.service") {
		t.Fatalf("the unit was never reloaded; systemctl saw:\n%s", calls)
	}
}

func TestReloadReportsAUnitThatWillNotReload(t *testing.T) {
	bin := t.TempDir()
	writeShim(t, filepath.Join(bin, "systemctl"), `echo "Unit zenith-shell.service not loaded." >&2; exit 5`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	oldCover := reloadCoverBegin
	reloadCoverBegin = func() string { return "" }
	defer func() { reloadCoverBegin = oldCover }()

	if got := (&daemon{}).reload(); !strings.HasPrefix(got, "err reload: systemctl --user reload zenith-shell.service: ") {
		t.Fatalf("reload = %q, want an error naming the failed command", got)
	}
}
