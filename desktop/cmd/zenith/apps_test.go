package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// appsTestWrite writes a file, making its directories.
func appsTestWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// appsTestSession points every lookup at directories the test owns and fixes
// the desktop and the locale.
//
// Whether the machine running the test has an icon theme, a Flatpak install or
// GNOME's panels is not a fact about the parser: a test that asked the host
// passes on a desktop and fails on a runner that carries none, and the failure
// is then about the runner. That is how the first version of these tests was
// found out.
func appsTestSession(t *testing.T) (home, system string) {
	t.Helper()
	root := t.TempDir()
	home = filepath.Join(root, "home")
	system = filepath.Join(root, "system")
	t.Setenv("HOME", filepath.Join(root, "user"))
	t.Setenv("XDG_DATA_HOME", home)
	t.Setenv("XDG_DATA_DIRS", system)
	t.Setenv("XDG_CURRENT_DESKTOP", "Zenith")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE"} {
		t.Setenv(key, "")
	}
	t.Setenv("LANG", "C")
	return home, system
}

func appsTestIDs(list []application) []string {
	ids := []string{}
	for _, app := range list {
		ids = append(ids, app.ID)
	}
	return ids
}

// TestParseDesktopEntryReadsWhatALauncherNeeds pins the rules for what a
// launcher offers.
//
// A desktop entry is a file with a great deal in it, and a launcher wants a
// name, an icon and a command from it, and now also what the search matches on.
func TestParseDesktopEntryReadsWhatALauncherNeeds(t *testing.T) {
	home, _ := appsTestSession(t)
	icon := filepath.Join(home, "icons", "hicolor", "scalable", "apps", "com.mitchellh.ghostty.svg")
	appsTestWrite(t, icon, "<svg xmlns='http://www.w3.org/2000/svg'/>\n")

	body := `[Desktop Entry]
Type=Application
Name=Terminal
GenericName=Terminal Emulator
Comment=Fast, native
Exec=ghostty --title=Zenith %U
Icon=com.mitchellh.ghostty
Terminal=false
Keywords=shell;prompt;
Categories=System;TerminalEmulator;
Path=/tmp
StartupWMClass=com.mitchellh.ghostty
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
	if want := "file://" + icon; app.Icon != want {
		t.Errorf("icon is %q, want %q", app.Icon, want)
	}
	if app.Exec != "ghostty" {
		t.Errorf("exec is %q, want ghostty", app.Exec)
	}
	if len(app.Args) != 1 || app.Args[0] != "--title=Zenith" {
		t.Errorf("args are %q, want the command's own arguments", app.Args)
	}
	if app.GenericName != "Terminal Emulator" || app.Comment != "Fast, native" {
		t.Errorf("generic name %q, comment %q", app.GenericName, app.Comment)
	}
	if !reflect.DeepEqual(app.Keywords, []string{"shell", "prompt"}) {
		t.Errorf("keywords are %q", app.Keywords)
	}
	if !reflect.DeepEqual(app.Categories, []string{"System", "TerminalEmulator"}) {
		t.Errorf("categories are %q", app.Categories)
	}
	if app.WorkingDir != "/tmp" || app.WMClass != "com.mitchellh.ghostty" {
		t.Errorf("path %q, wm class %q", app.WorkingDir, app.WMClass)
	}
}

// TestParseDesktopEntryDropsWhatIsNotLaunchable pins the other half.
//
// A file that is not an application, or that says it is not to be shown, is
// not something a launcher offers. A launcher that offered them would list
// the preferences of the desktop and the uninstallers of the archive.
func TestParseDesktopEntryDropsWhatIsNotLaunchable(t *testing.T) {
	appsTestSession(t)
	tests := []struct {
		name string
		body string
	}{
		{"not an application", "[Desktop Entry]\nType=Link\nName=A Link\n"},
		{"no display", "[Desktop Entry]\nType=Application\nName=Hidden\nExec=thing\nNoDisplay=true\n"},
		{"hidden", "[Desktop Entry]\nType=Application\nName=Hidden\nExec=thing\nHidden=true\n"},
		{"no name", "[Desktop Entry]\nType=Application\nExec=thing\n"},
		{"no command", "[Desktop Entry]\nType=Application\nName=Thing\n"},
		{"another section", "[Desktop Action New]\nName=New Window\nType=Application\nExec=thing\n"},
		{"only a field code", "[Desktop Entry]\nType=Application\nName=Thing\nExec=%U\n"},
	}

	for _, test := range tests {
		if _, ok := parseDesktopEntry("thing", test.body); ok {
			t.Errorf("%s was offered to the launcher", test.name)
		}
	}
}

// TestDesktopEntryHonoursTheSessionDesktop pins OnlyShowIn and NotShowIn.
//
// GNOME's control-centre panels, its extension manager and the Ubuntu-only
// session tools are all on a stock install, and every one of them says which
// desktop it belongs to. A launcher that ignored the lines listed them all.
func TestDesktopEntryHonoursTheSessionDesktop(t *testing.T) {
	appsTestSession(t)
	entry := func(extra string) string {
		return "[Desktop Entry]\nType=Application\nName=Thing\nExec=thing\n" + extra
	}
	tests := []struct {
		desktop string
		extra   string
		offered bool
	}{
		{"Zenith", "", true},
		{"Zenith", "OnlyShowIn=GNOME;Unity;\n", false},
		{"Zenith", "OnlyShowIn=Zenith;\n", true},
		{"Zenith", "NotShowIn=GNOME;\n", true},
		{"Zenith", "NotShowIn=GNOME;Zenith;\n", false},
		{"", "OnlyShowIn=Zenith;\n", true},
		{"", "OnlyShowIn=GNOME;\n", false},
		{"ubuntu:GNOME", "OnlyShowIn=GNOME;\n", true},
		{"ubuntu:GNOME", "NotShowIn=GNOME;\n", false},
	}
	for _, test := range tests {
		t.Setenv("XDG_CURRENT_DESKTOP", test.desktop)
		_, ok := parseDesktopEntry("thing", entry(test.extra))
		if ok != test.offered {
			t.Errorf("desktop %q with %q: offered %v, want %v",
				test.desktop, strings.TrimSpace(test.extra), ok, test.offered)
		}
	}
}

// TestTryExecHidesWhatIsNotInstalled pins TryExec.
//
// A desktop entry left behind by a removed package names a binary that is no
// longer there, and offering it is offering a click that does nothing.
func TestTryExecHidesWhatIsNotInstalled(t *testing.T) {
	appsTestSession(t)
	entry := func(tryExec string) string {
		return "[Desktop Entry]\nType=Application\nName=Thing\nExec=thing\nTryExec=" + tryExec + "\n"
	}

	if _, ok := parseDesktopEntry("thing", entry("/no/such/zenith-binary")); ok {
		t.Error("an entry whose TryExec is missing was offered")
	}
	if _, ok := parseDesktopEntry("thing", entry("zenith-no-such-binary")); ok {
		t.Error("an entry whose relative TryExec is missing was offered")
	}
	if _, ok := parseDesktopEntry("thing", entry("sh")); !ok {
		t.Error("an entry whose TryExec is installed was dropped")
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

// TestExecLineFollowsTheQuotingRules pins how a command line is split.
//
// Splitting on spaces is wrong for the entries that matter most: the ones that
// run a shell, whose whole script is one quoted argument.
func TestExecLineFollowsTheQuotingRules(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{`sh -c "echo hello world"`, []string{"sh", "-c", "echo hello world"}},
		{`env FOO=bar app --flag %U`, []string{"env", "FOO=bar", "app", "--flag"}},
		{`app "a \"quoted\" word" tail`, []string{"app", `a "quoted" word`, "tail"}},
		{`app --title=%c`, []string{"app", "--title="}},
		{`app 100%% %i`, []string{"app", "100%", "--icon", "star"}},
		{`app "" last`, []string{"app", "", "last"}},
		{`"/opt/My App/run" %F`, []string{"/opt/My App/run"}},
	}
	for _, test := range tests {
		got := desktopExecArgv(test.line, "", "star", "")
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%q split into %q, want %q", test.line, got, test.want)
		}
	}

	if got := desktopExecArgv("app --title=%c", "Some App", "", ""); got[1] != "--title=Some App" {
		t.Errorf("%%c became %q, want the name", got[1])
	}
}

// TestUserEntryShadowsSystemEntry pins the search order.
//
// The user's directory comes first, so a person can override an application's
// entry, and a Hidden=true copy there is how a person removes one from the
// menus. The old order took the system file first, which made both impossible.
func TestUserEntryShadowsSystemEntry(t *testing.T) {
	home, system := appsTestSession(t)
	appsTestWrite(t, filepath.Join(system, "applications", "edit.desktop"),
		"[Desktop Entry]\nType=Application\nName=Theirs\nExec=theirs\n")
	appsTestWrite(t, filepath.Join(home, "applications", "edit.desktop"),
		"[Desktop Entry]\nType=Application\nName=Mine\nExec=mine\n")
	appsTestWrite(t, filepath.Join(system, "applications", "noisy.desktop"),
		"[Desktop Entry]\nType=Application\nName=Noisy\nExec=noisy\n")
	appsTestWrite(t, filepath.Join(home, "applications", "noisy.desktop"),
		"[Desktop Entry]\nType=Application\nName=Noisy\nExec=noisy\nHidden=true\n")

	offered := classifyApplications(scanDesktopFiles(xdgDataDirs()), currentAppEnv(), nil)

	var edit *application
	for i := range offered {
		if offered[i].ID == "edit" {
			edit = &offered[i]
		}
		if offered[i].ID == "noisy" {
			t.Error("a system entry the user hid was offered")
		}
	}
	if edit == nil || edit.Name != "Mine" || edit.Source != "user" {
		t.Errorf("the user's entry did not win: %+v", edit)
	}
}

// TestFlatpakExportsAreSearched pins that Flatpak applications are listed
// without the session having put its exports on XDG_DATA_DIRS.
func TestFlatpakExportsAreSearched(t *testing.T) {
	home, _ := appsTestSession(t)
	real := filepath.Join(home, "flatpak", "app", "org.example.App", "export", "share", "applications", "org.example.App.desktop")
	appsTestWrite(t, real, "[Desktop Entry]\nType=Application\nName=Flat\nExec=flatpak run org.example.App\n")
	link := filepath.Join(home, "flatpak", "exports", "share", "applications", "org.example.App.desktop")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	// Exports are links into the installed application, as Flatpak makes them.
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	offered := classifyApplications(scanDesktopFiles(xdgDataDirs()), currentAppEnv(), nil)
	if len(offered) != 1 || offered[0].ID != "org.example.App" || offered[0].Source != "flatpak" {
		t.Errorf("the Flatpak entry was not listed: %+v", offered)
	}
}

// TestSubdirectoryIDsUseDashes pins how an entry below applications/ is named.
func TestSubdirectoryIDsUseDashes(t *testing.T) {
	_, system := appsTestSession(t)
	appsTestWrite(t, filepath.Join(system, "applications", "kde4", "okular.desktop"),
		"[Desktop Entry]\nType=Application\nName=Okular\nExec=okular\n")

	got := appsTestIDs(listApplications())
	if !reflect.DeepEqual(got, []string{"kde4-okular"}) {
		t.Errorf("ids are %q, want kde4-okular", got)
	}
}

// TestLocalisedNameFollowsTheLocale pins that a user who runs the desktop in
// German is not shown English names.
func TestLocalisedNameFollowsTheLocale(t *testing.T) {
	appsTestSession(t)
	body := "[Desktop Entry]\nType=Application\nName=Files\nName[de]=Dateien\nName[de_AT]=Dateien AT\nName[fr]=Fichiers\nExec=files\n"

	for _, test := range []struct {
		lang, language, want string
	}{
		{"C", "", "Files"},
		{"en_US.UTF-8", "", "Files"},
		{"de_DE.UTF-8", "", "Dateien"},
		{"de_AT.UTF-8", "", "Dateien AT"},
		{"de_DE.UTF-8", "fr:de", "Fichiers"},
		{"C", "fr", "Files"},
	} {
		t.Setenv("LANG", test.lang)
		t.Setenv("LANGUAGE", test.language)
		app, ok := parseDesktopEntry("files", body)
		if !ok || app.Name != test.want {
			t.Errorf("LANG=%s LANGUAGE=%s: name %q, want %q", test.lang, test.language, app.Name, test.want)
		}
	}
}

// TestDesktopActionsAreRead pins the extra entries an application offers.
func TestDesktopActionsAreRead(t *testing.T) {
	appsTestSession(t)
	body := `[Desktop Entry]
Type=Application
Name=Browser
Exec=browser %U
Actions=new-window;private;missing;

[Desktop Action new-window]
Name=New Window
Exec=browser --new-window

[Desktop Action private]
Name=Private Window
Exec=browser --private-window
Icon=browser-private
`
	app, ok := parseDesktopEntry("browser", body)
	if !ok {
		t.Fatal("the application was not offered")
	}
	if len(app.Actions) != 2 {
		t.Fatalf("actions are %+v, want the two that are declared", app.Actions)
	}
	if app.Actions[0].ID != "new-window" || app.Actions[0].Exec != "browser" || !reflect.DeepEqual(app.Actions[0].Args, []string{"--new-window"}) {
		t.Errorf("first action is %+v", app.Actions[0])
	}
	if app.Actions[1].Name != "Private Window" || app.Actions[1].Icon != "browser-private" {
		t.Errorf("second action is %+v", app.Actions[1])
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
