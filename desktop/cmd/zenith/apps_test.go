package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseDesktopEntryReadsWhatALauncherNeeds pins the rules for what a
// launcher offers.
//
// A desktop entry is a file with a great deal in it, and a launcher wants
// three things from it: a name, an icon and a command. Everything else in the
// format is about actions, MIME types and languages.
func TestParseDesktopEntryReadsWhatALauncherNeeds(t *testing.T) {
	body := `[Desktop Entry]
Type=Application
Name=Terminal
Exec=ghostty --title=Zenith %U
Icon=com.mitchellh.ghostty
Terminal=false
`

	app, ok := parseDesktopEntry("com.mitchellh.ghostty", body)
	if !ok {
		t.Fatal("a valid application was not offered")
	}
	if app.Name != "Terminal" {
		t.Errorf("name is %q, want Terminal", app.Name)
	}
	// The icon is resolved to a file, because the shell cannot ask the icon
	// engine for a name: the engine's theme is a setting of the session, and
	// an icon that is not in that theme is a broken-image placeholder.
	if !strings.HasPrefix(app.Icon, "file://") {
		t.Errorf("icon is %q, want a file", app.Icon)
	}
	if app.Exec != "ghostty" {
		t.Errorf("exec is %q, want ghostty", app.Exec)
	}
	if len(app.Args) != 1 || app.Args[0] != "--title=Zenith" {
		t.Errorf("args are %q, want the command's own arguments", app.Args)
	}
}

// TestParseDesktopEntryDropsWhatIsNotLaunchable pins the other half.
//
// A file that is not an application, or that says it is not to be shown, is
// not something a launcher offers. A launcher that offered them would list
// the preferences of the desktop and the uninstallers of the archive.
func TestParseDesktopEntryDropsWhatIsNotLaunchable(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"not an application", "[Desktop Entry]\nType=Link\nName=A Link\n"},
		{"no display", "[Desktop Entry]\nType=Application\nName=Hidden\nNoDisplay=true\n"},
		{"hidden", "[Desktop Entry]\nType=Application\nName=Hidden\nHidden=true\n"},
		{"no name", "[Desktop Entry]\nType=Application\nExec=thing\n"},
		{"no command", "[Desktop Entry]\nType=Application\nName=Thing\n"},
		{"another section", "[Desktop Action New]\nName=New Window\nType=Application\n"},
	}

	for _, test := range tests {
		if _, ok := parseDesktopEntry("thing", test.body); ok {
			t.Errorf("%s was offered to the launcher", test.name)
		}
	}
}

// TestSplitCommandRemovesFieldCodes pins the field-code rule.
//
// The codes stand for a file or a URL that the launcher has none of, and a
// command with %U in it is a command that fails.
func TestSplitCommandRemovesFieldCodes(t *testing.T) {
	command, args := splitCommand("firefox %u")
	if command != "firefox" {
		t.Errorf("command is %q, want firefox", command)
	}
	if len(args) != 0 {
		t.Errorf("args are %q, want none", args)
	}

	command, args = splitCommand("code --new-window %F")
	if command != "code" {
		t.Errorf("command is %q, want code", command)
	}
	if len(args) != 1 || args[0] != "--new-window" {
		t.Errorf("args are %q, want the one argument", args)
	}
}

// TestIconReferenceKeepsNamesAndPathsApart pins the two kinds of icon.
//
// A desktop entry names its icon either by name in the theme or by path to a
// file, and the shell needs the two in different forms: a name stays a name,
// and a path becomes a URL. A path handed over as a path is a path that
// nothing can open, and the icon is missing with nothing to say why.
//
// The icon directory below is one this test writes and points the lookup at
// through XDG_DATA_HOME, rather than the machine's own. Whether the machine
// running the test has an icon theme installed is not a fact about
// iconReference: a test that asked the host passes on a desktop and fails on a
// runner that carries no theme, and the failure is then about the runner. That
// is what this test used to do, and a CI runner with an empty /usr/share/icons
// is how it was found out.
//
// This is also the whole of what iconReference is tested for. A second test
// once repeated the name, path and empty cases without the installed one, and
// a second copy of the same three assertions is a second place for them to
// drift.
func TestIconReferenceKeepsNamesAndPathsApart(t *testing.T) {
	icons := t.TempDir()
	t.Setenv("XDG_DATA_HOME", icons)
	// A directory that does not exist, so that nothing the machine happens to
	// have installed can answer for a name this test did not write.
	t.Setenv("XDG_DATA_DIRS", filepath.Join(icons, "no-themes"))

	// Laid out the way the icon specification lays a theme out — the theme,
	// then the size, then the context — because that is the shape the lookup
	// has to walk.
	installed := filepath.Join(icons, "icons", "hicolor", "scalable", "status",
		"multimedia-volume-control.svg")
	if err := os.MkdirAll(filepath.Dir(installed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed, []byte("<svg xmlns='http://www.w3.org/2000/svg'/>\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A name that is installed becomes the file it is, because the shell
	// cannot ask the icon engine for it: the engine's theme is a setting of
	// the session, and an icon that is not in that theme is a broken-image
	// placeholder where the application's icon should be.
	if got, want := iconReference("multimedia-volume-control"), "file://"+installed; got != want {
		t.Errorf("a themed name became %q, want %q", got, want)
	}
	// A name that is not installed stays a name: there is nothing to point
	// at, and a name is a better answer than a path to nothing.
	if got := iconReference("definitely-not-an-icon-anywhere"); got != "definitely-not-an-icon-anywhere" {
		t.Errorf("a name that is not installed became %q", got)
	}
	if got := iconReference("/opt/thing/icon.png"); got != "file:///opt/thing/icon.png" {
		t.Errorf("a path became %q", got)
	}
	if got := iconReference(""); got != "" {
		t.Errorf("no icon became %q", got)
	}
}

// TestListApplicationsFindsTheMachineApplications checks the catalogue against
// the machine rather than against a fixture.
//
// The whole point of the command is that it reads the desktop entries that are
// really there, so a test that used a fixture would be testing the fixture.
func TestListApplicationsFindsTheMachineApplications(t *testing.T) {
	found := listApplications()
	if len(found) == 0 {
		t.Skip("this machine has no desktop entries")
	}

	for _, app := range found {
		if app.Name == "" {
			t.Errorf("%s has no name", app.ID)
		}
		if app.Exec == "" {
			t.Errorf("%s has no command", app.ID)
		}
		if app.ID == "" {
			t.Error("an application has no id")
		}
	}
}
