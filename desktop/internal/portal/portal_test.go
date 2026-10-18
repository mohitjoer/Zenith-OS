package portal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodConfig = `[default]
use=hyprland;

[org.gnome.impl.portal.ScreenCast]
use=hyprland;

[org.gnome.impl.portal.Screenshot]
use=hyprland;

[org.gnome.impl.portal.FileChooser]
use=gtk;

[org.gnome.impl.portal.OpenURI]
use=gtk;

[org.gnome.impl.portal.Print]
use=gtk;

[org.gnome.impl.portal.Secret]
use=lxqt;
`

// machine is a synthetic machine with a portal configuration on disk.
type machine struct {
	t       *testing.T
	config  string
	home    string
	desktop string
	system  map[string]string
}

func newMachine(t *testing.T) *machine {
	return &machine{
		t:       t,
		home:    t.TempDir(),
		desktop: ExpectedDesktop,
		system:  map[string]string{},
	}
}

func (m *machine) env() Env {
	return Env{
		XDGConfigHome:  filepath.Join(m.home, ".config"),
		Home:           m.home,
		CurrentDesktop: m.desktop,
		Root:           filepath.Join(m.home, "root"),
	}
}

// writeBackend writes a .portal descriptor, which is what the dispatcher reads
// to decide what a backend implements.
//
// The descriptor is written rather than a Go table of backends on purpose: the
// dispatcher builds its whole view from these files, so a table here would be a
// second thing to keep in step with what is installed.
// writeShippedBackends writes the backends the packaged configuration names,
// declaring the interfaces they really implement.
func (m *machine) writeShippedBackends() {
	m.t.Helper()
	m.writeBackend("hyprland", "org.freedesktop.portal.impl.portal.Hyprland",
		"org.freedesktop.impl.portal.ScreenCast;"+
			"org.freedesktop.impl.portal.Screenshot;")
	m.writeBackend("gtk", "org.freedesktop.portal.impl.portal.Gtk",
		"org.freedesktop.impl.portal.FileChooser;"+
			"org.freedesktop.impl.portal.OpenURI;"+
			"org.freedesktop.impl.portal.Print;")
	m.writeBackend("lxqt", "org.freedesktop.portal.impl.portal.LXQt",
		"org.freedesktop.impl.portal.Secret;")
}

func (m *machine) writeBackend(name, busName, interfaces string) {
	m.t.Helper()
	dir := filepath.Join(m.home, "root", backendDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.t.Fatal(err)
	}
	body := "[portal]\nDBusName=" + busName + "\nInterfaces=" + interfaces + "\n"
	if err := os.WriteFile(filepath.Join(dir, name+".portal"), []byte(body), 0o644); err != nil {
		m.t.Fatal(err)
	}
}

// writeUser puts a per-desktop configuration where the dispatcher will find it,
// under the name derived from the *current* desktop.
func (m *machine) writeUser(body string) {
	m.t.Helper()
	m.writeUserNamed(ConfigFile(m.desktop), body)
}

// writeUserNamed writes a configuration under a specific file name, which is how
// the silent failure is staged: adoption writes zenith-portals.conf, and the
// session identity is then wrong, so the dispatcher never looks at that file.
func (m *machine) writeUserNamed(name, body string) {
	m.t.Helper()
	dir := filepath.Join(m.home, ".config", "xdg-desktop-portal")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		m.t.Fatal(err)
	}
}

// TestConfigFileDerivation pins the case-folding the dispatcher does.
//
// It lower-cases XDG_CURRENT_DESKTOP. That is why the session exports
// "Zenith" for the current desktop and "zenith" for the session name: getting
// them the other way round still resolves a filename, just the wrong one, and
// nothing complains.
func TestConfigFileDerivation(t *testing.T) {
	tests := []struct{ desktop, want string }{
		{"Zenith", "zenith-portals.conf"},
		{"zenith", "zenith-portals.conf"},
		{"ZENITH", "zenith-portals.conf"},
		{" Zenith ", "zenith-portals.conf"},
		{"GNOME", "gnome-portals.conf"},
		{"", "-portals.conf"},
	}
	for _, tc := range tests {
		if got := ConfigFile(tc.desktop); got != tc.want {
			t.Errorf("ConfigFile(%q) = %q, want %q", tc.desktop, got, tc.want)
		}
	}
}

// TestCorrectSessionResolvesCleanly is the baseline.
func TestCorrectSessionResolvesCleanly(t *testing.T) {
	m := newMachine(t)
	m.writeUser(goodConfig)
	m.writeShippedBackends()

	resolution := Resolve(m.env())
	if resolution.Source != SourceUserConfig {
		t.Errorf("source = %q, want %q", resolution.Source, SourceUserConfig)
	}
	if len(resolution.Problems) != 0 {
		t.Errorf("a correct machine reported problems: %v", resolution.Problems)
	}
	if len(resolution.Notices) != 0 {
		t.Errorf("a correct machine reported notices: %v", resolution.Notices)
	}
}

// TestWrongDesktopIdentityIsTheRootCause is the silent failure this package
// exists for.
//
// A session that does not declare itself correctly finds no per-desktop file,
// falls back to the system default, and quietly uses whatever that names. On a
// machine adopted from GNOME that means use=gnome and a dispatcher activating a
// backend that is not installed — with no error anywhere.
func TestWrongDesktopIdentityIsTheRootCause(t *testing.T) {
	// These three resolve to a name that has no file, so the dispatcher lands in
	// the system fallback. That is the dangerous case.
	for _, desktop := range []string{"", "GNOME", "ubuntu:GNOME"} {
		m := newMachine(t)
		// The file adoption wrote, named for the session it was written *for*.
		m.writeUserNamed(ConfigFile(ExpectedDesktop), goodConfig)
		// And then a session that does not declare itself as that session.
		m.desktop = desktop

		resolution := Resolve(m.env())

		if len(resolution.Problems) == 0 {
			t.Errorf("desktop %q produced no problem", desktop)
			continue
		}
		if !strings.Contains(resolution.Problems[0], "XDG_CURRENT_DESKTOP") {
			t.Errorf("desktop %q: the first problem is not the identity: %q",
				desktop, resolution.Problems[0])
		}
		if resolution.Source != SourceSystem && resolution.Source != SourceNone {
			t.Errorf("desktop %q: source = %q; a fallback is what must be reported",
				desktop, resolution.Source)
		}
	}
}

// TestLowercaseIdentityFindsTheFileButIsStillWrong covers the case that looks
// like it should pass and must not be counted as passing.
//
// The dispatcher lower-cases the desktop name, so XDG_CURRENT_DESKTOP=zenith
// resolves exactly the same configuration file as Zenith does. The file is
// found. The session is still wrong, and the identity assertion still fires —
// because everything else in the stack branches on the exact string, and a
// session that reports the wrong one is a session that has not declared itself.
func TestLowercaseIdentityFindsTheFileButIsStillWrong(t *testing.T) {
	m := newMachine(t)
	m.writeUser(goodConfig)
	m.desktop = "zenith"

	resolution := Resolve(m.env())

	if resolution.Source != SourceUserConfig {
		t.Errorf("source = %q; lower-casing should find the file",
			resolution.Source)
	}
	found := false
	for _, problem := range resolution.Problems {
		if strings.Contains(problem, "XDG_CURRENT_DESKTOP") {
			found = true
		}
	}
	if !found {
		t.Errorf("the wrong identity was not reported even though the file was "+
			"found: %v", resolution.Problems)
	}
}

// TestStaleGNOMEPreferenceIsNamed is the most likely single fault on an adopted
// machine, and it produces a dispatcher error rather than a fallback.
func TestStaleGNOMEPreferenceIsNamed(t *testing.T) {
	m := newMachine(t)
	m.writeUser(strings.ReplaceAll(goodConfig, "use=hyprland;", "use=gnome;"))

	resolution := Resolve(m.env())

	named := 0
	for _, problem := range resolution.Problems {
		if strings.Contains(problem, "stale preference") {
			named++
		}
	}
	if named == 0 {
		t.Errorf("a stale use=gnome was not named as such: %v", resolution.Problems)
	}
}

// TestMissingSectionsAreReported covers a config that is present but incomplete.
func TestMissingSectionsAreReported(t *testing.T) {
	m := newMachine(t)
	m.writeUser("[default]\nuse=hyprland;\n")

	resolution := Resolve(m.env())
	if len(resolution.Problems) < 5 {
		t.Errorf("an almost-empty configuration produced %d problem(s): %v",
			len(resolution.Problems), resolution.Problems)
	}
}

// TestInterfaceNamedToABackendThatCannotServeIt is the fault this check exists
// for, and it is the one this project had.
//
// Secret named gtk for months. gtk is installed, gtk's .portal file is present
// and readable, and gtk does not implement Secret. Every other check passed. The
// configuration was confident, the file read as though the interface were
// handled, and every Flatpak that stores a password still had nowhere to put
// one.
func TestInterfaceNamedToABackendThatCannotServeIt(t *testing.T) {
	m := newMachine(t)

	// The configuration as it was: Secret named gtk. Every other interface
	// names the backend that serves it, so this is a file that reads correctly.
	m.writeUser(strings.Replace(goodConfig,
		"[org.gnome.impl.portal.Secret]\nuse=lxqt;",
		"[org.gnome.impl.portal.Secret]\nuse=gtk;", 1))

	// gtk is installed, its descriptor is readable, and it does not implement
	// Secret. That is the whole failure.
	m.writeBackend("hyprland", "org.freedesktop.portal.impl.portal.Hyprland",
		"org.freedesktop.impl.portal.ScreenCast;"+
			"org.freedesktop.impl.portal.Screenshot;")
	m.writeBackend("gtk", "org.freedesktop.portal.impl.portal.Gtk",
		"org.freedesktop.impl.portal.FileChooser;"+
			"org.freedesktop.impl.portal.OpenURI;"+
			"org.freedesktop.impl.portal.Print;")

	resolution := Resolve(m.env())

	found := false
	for _, problem := range resolution.Problems {
		if strings.Contains(problem, "Secret") {
			found = true
		}
	}
	if !found {
		t.Errorf("Secret named a backend that cannot serve it, and nothing "+
			"said so: %v", resolution.Problems)
	}
}

// TestBackendNotInstalledIsReported covers the other half: the backend named by
// a correct configuration is simply not there.
func TestBackendNotInstalledIsReported(t *testing.T) {
	m := newMachine(t)
	m.writeUser(goodConfig)

	resolution := Resolve(m.env())
	if len(resolution.Problems) == 0 {
		t.Fatal("named backends that are not installed were not reported")
	}
	found := false
	for _, problem := range resolution.Problems {
		if strings.Contains(problem, "not installed") {
			found = true
		}
	}
	if !found {
		t.Errorf("a missing backend was not named as missing: %v",
			resolution.Problems)
	}
}

// TestBackendImplementingEveryInterfaceIsClean is the other direction, so the
// check is not simply always reporting.
func TestBackendImplementingEveryInterfaceIsClean(t *testing.T) {
	m := newMachine(t)
	m.writeUser(goodConfig)
	m.writeShippedBackends()

	if findings := CheckInterfaces(m.env(), mustParse(t, goodConfig)); len(findings) != 0 {
		t.Errorf("a correct configuration reported findings: %v", findings)
	}
}

// TestInterfaceMatchingIsNotSubstring covers the direction a substring check
// gets wrong.
//
// org.freedesktop.impl.portal.Notification is a prefix of
// org.freedesktop.impl.portal.NotificationFilter. Treating one as the other means
// a backend that only has the filtered one is credited with the plain one.
func TestInterfaceMatchingIsNotSubstring(t *testing.T) {
	portalFile := "[portal]\nInterfaces=org.freedesktop.impl.portal.NotificationFilter;\n"

	rc := 0
	if declaresInterface(portalFile, "org.freedesktop.impl.portal.Notification") {
		rc = 1
	}
	if rc != 0 {
		t.Error("a filtered interface was matched as the plain one")
	}

	rc = 0
	if !declaresInterface(portalFile, "org.freedesktop.impl.portal.NotificationFilter") {
		rc = 1
	}
	if rc != 0 {
		t.Error("the interface it does declare was not matched")
	}
}

// mustParse is a helper for the tests above.
func mustParse(t *testing.T, body string) *Config {
	t.Helper()
	config, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	return config
}

// TestNoConfigurationAtAll covers a machine with neither file.
func TestNoConfigurationAtAll(t *testing.T) {
	m := newMachine(t)
	resolution := Resolve(m.env())

	if resolution.Source != SourceNone {
		t.Errorf("source = %q, want %q", resolution.Source, SourceNone)
	}
	if len(resolution.Problems) == 0 {
		t.Error("a machine with no portal configuration produced no problem")
	}
}

// TestParseHandlesCommentsAndSpacing covers the INI details.
func TestParseHandlesCommentsAndSpacing(t *testing.T) {
	config, err := Parse("# a comment\n; another\n\n" +
		"  [default]  \n  use = hyprland ;\n" +
		"[org.gnome.impl.portal.Print]\nuse=gtk;\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if backend, ok := config.Backend("default"); !ok || backend != "hyprland" {
		t.Errorf("default backend = %q, %v", backend, ok)
	}
	if backend, ok := config.Backend("org.gnome.impl.portal.Print"); !ok || backend != "gtk" {
		t.Errorf("Print backend = %q, %v", backend, ok)
	}
	if _, ok := config.Backend("org.gnome.impl.portal.Nothing"); ok {
		t.Error("an absent section reported a backend")
	}
}

// TestBackendTrimsTheListSemicolon covers `use=hyprland;` being a single
// backend in a list rather than a name with a semicolon in it.
func TestBackendTrimsTheListSemicolon(t *testing.T) {
	config, err := Parse("[default]\nuse=hyprland;\n")
	if err != nil {
		t.Fatal(err)
	}
	if backend, _ := config.Backend("default"); backend != "hyprland" {
		t.Errorf("backend = %q, want the semicolon trimmed", backend)
	}
}
