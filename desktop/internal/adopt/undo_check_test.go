package adopt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestUndoScriptIsValidShell runs the generated script through sh -n.
//
// A generated undo script is only useful if it parses. The moment it is needed
// there may be no desktop to run `zenith --undo` from, so a syntax error in this
// file is the difference between a machine that comes back and one that does
// not — and nobody finds out until they need it.
func TestUndoScriptIsValidShell(t *testing.T) {
	plan := Plan{
		BackupDir: "/etc/zenith/adopt/backup/T",
	}
	state := &State{
		User:          User{Name: "mohit", Home: "/home/mohit", UID: 1000},
		Packages:      Packages{Removed: []string{"gnome-core", "yelp"}},
		Registrations: []Registration{{Path: "/usr/share/wayland-sessions/zenith.desktop"}},
		UserWrites: []UserWrite{{
			Path:       "/home/mohit/.config/xdg-desktop-portal/portals.conf",
			ArchivedTo: "/etc/zenith/adopt/backup/T/portals.conf.gnome",
		}},
		Prune: Prune{Performed: true},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "undo.sh")
	if err := WriteUndoScript(path, plan, state); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if body[0] != '#' {
		t.Error("the script does not start with a comment")
	}
	if !strings.Contains(string(body), "#!/bin/sh") {
		t.Error("the script has no shebang")
	}

	// sh -n parses without executing, which is what is wanted: the script's
	// whole body is guarded by a root check that must not run here.
	cmd := exec.Command("sh", "-n", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the generated undo script does not parse: %v\n%s", err, out)
	}
}

// TestUndoScriptIsExecutable covers the property that matters at the moment it
// is needed: the user can run it without first reading its permissions.
func TestUndoScriptIsExecutable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "undo.sh")
	if err := WriteUndoScript(path, Plan{BackupDir: dir}, &State{
		User: User{Name: "u", Home: dir},
	}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("mode is %v; an undo script that is not executable is a "+
			"script that needs one more step at the worst time",
			info.Mode().Perm())
	}
}

// TestUndoQuotesPathsWithSpaces covers the path that a developer never has and a
// user does.
func TestUndoQuotesPathsWithSpaces(t *testing.T) {
	plan := Plan{BackupDir: "/etc/zenith/adopt/backup/T"}
	state := &State{
		User: User{Name: "mohit", Home: "/home/mohit"},
		UserWrites: []UserWrite{{
			Path:       "/home/mohit/My Portal Config/portals.conf",
			ArchivedTo: "/etc/zenith/adopt/backup/T/portals.conf.gnome",
		}},
	}

	script := UndoScript(plan, state)

	// The whole path is quoted, not the spacey segment on its own: quoting a
	// substring would leave the rest of the path unquoted and still broken.
	if !strings.Contains(script, `'/home/mohit/My Portal Config/portals.conf'`) {
		t.Error("a path with a space was not quoted as one argument; a script " +
			"that handles only the paths the developer tested fails on the " +
			"user it was written for")
	}
	if strings.Contains(script, "My Portal Config/portals.conf'") &&
		!strings.Contains(script, "'/home/mohit/My Portal Config/portals.conf'") {
		t.Error("the quoted path does not start at the beginning of the argument")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "undo.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("sh", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("does not parse: %v\n%s", err, out)
	}
}

// TestUndoRefusesWithoutRoot covers the guard that keeps it from half-running
// as a normal user: a script that removes registrations as an unprivileged user
// fails on the first one and leaves the rest undone, and the user is left
// guessing how far it got.
func TestUndoRefusesWithoutRoot(t *testing.T) {
	script := UndoScript(Plan{BackupDir: "/tmp/x"}, &State{
		User: User{Name: "u", Home: "/tmp"},
	})
	if !strings.Contains(script, `id -u`) || !strings.Contains(script, "exit 1") {
		t.Error("the script has no root guard")
	}
}

// TestUndoRemovesEveryRecordedRegistration covers the completeness property.
//
// An undo that removes some of the registrations leaves a session that still
// appears in the login manager and still fails to start, which is worse than
// not undoing at all because it looks done.
func TestUndoRemovesEveryRecordedRegistration(t *testing.T) {
	registrations := []Registration{
		{Path: "/usr/share/wayland-sessions/zenith.desktop"},
		{Path: "/usr/lib/systemd/user/zenith-shell.service"},
		{Path: "/usr/lib/systemd/user/zenith-portal.service"},
	}
	script := UndoScript(Plan{BackupDir: "/tmp/x"},
		&State{User: User{Name: "u", Home: "/tmp"}, Registrations: registrations})

	for _, registration := range registrations {
		if !strings.Contains(script, "rm -f '"+registration.Path+"'") {
			t.Errorf("the script does not remove %s", registration.Path)
		}
	}
}

// TestUndoStaysSilentWhenItRecordedNothing covers the dry-run case.
//
// A dry run writes no state. An undo script generated from an empty plan that
// still removes registrations would be a script deleting files adoption never
// created.
func TestUndoStaysSilentWhenItRecordedNothing(t *testing.T) {
	script := UndoScript(Plan{BackupDir: "/tmp/x"},
		&State{User: User{Name: "u", Home: "/tmp"}})

	if !strings.Contains(script, "nothing was removed") {
		t.Error("no package was recorded as removed, and the script does not say so")
	}
	for _, path := range []string{
		"/usr/share/wayland-sessions/zenith.desktop",
		"/usr/lib/systemd/user/zenith-shell.service",
	} {
		if strings.Contains(script, "rm -f '"+path+"'") {
			t.Errorf("the script removes %s although nothing was recorded", path)
		}
	}
}
