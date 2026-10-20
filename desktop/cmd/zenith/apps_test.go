package main

import (
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
	if app.Icon != "com.mitchellh.ghostty" {
		t.Errorf("icon is %q, want the entry's own icon", app.Icon)
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
func TestIconReferenceKeepsNamesAndPathsApart(t *testing.T) {
	// A name that is installed becomes the file it is, because the shell
	// cannot ask the icon engine for it: the engine's theme is a setting of
	// the session, and an icon that is not in that theme is a broken-image
	// placeholder where the application's icon should be.
	if got := iconReference("multimedia-volume-control"); !strings.HasPrefix(got, "file://") {
		t.Errorf("a themed name became %q, want a file", got)
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

// TestIconReferenceResolvesNamesToFiles pins the rule that an icon name is
// looked up and a path is not.
//
// A name is a name in the session's icon theme, and the session's theme is a
// setting rather than a fact about the machine. Handing the name to the shell
// means handing it to whatever theme happens to be configured, and an icon that
// is not in it is a broken-image placeholder where the application's icon
// should be.
func TestIconReferenceResolvesNamesToFiles(t *testing.T) {
	if got := iconReference("/opt/thing/icon.png"); got != "file:///opt/thing/icon.png" {
		t.Errorf("a path became %q", got)
	}
	if got := iconReference("definitely-not-an-icon-anywhere"); got != "definitely-not-an-icon-anywhere" {
		t.Errorf("a name that is not installed became %q", got)
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
