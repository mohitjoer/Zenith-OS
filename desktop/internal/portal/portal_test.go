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
use=gtk;
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

	resolution := Resolve(m.env())
	if resolution.Source != SourceUserConfig {
		t.Errorf("source = %q, want %q", resolution.Source, SourceUserConfig)
	}
	if len(resolution.Problems) != 0 {
		t.Errorf("a correct machine reported problems: %v", resolution.Problems)
	}
	// The Secret limitation is a notice, not a problem: it is not something the
	// user can fix by editing a configuration file.
	if len(resolution.Notices) != 1 || !strings.Contains(resolution.Notices[0], "Secret") {
		t.Errorf("the Secret limitation was not stated as a notice: %v", resolution.Notices)
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

// TestSecretCaveatIsStatedOnceAndLast covers the reporting decision.
//
// The Secret interface does not work on this stack at all. Mentioning it
// alongside six real problems buries the real ones, so it is stated once, and
// only when everything else is right — otherwise a reader stops at the first
// paragraph and never sees the one that needs a decision.
func TestSecretCaveatIsStatedOnceAndLast(t *testing.T) {
	m := newMachine(t)
	m.writeUser(goodConfig)

	resolution := Resolve(m.env())
	if len(resolution.Notices) != 1 {
		t.Errorf("the Secret limitation appears %d time(s), want once: %v",
			len(resolution.Notices), resolution.Notices)
	}

	// With a real problem present, it is not repeated. A known limitation
	// alongside six real misconfigurations buries the six.
	m = newMachine(t)
	m.writeUser("[default]\nuse=gnome;\n")
	resolution = Resolve(m.env())
	if len(resolution.Notices) != 0 {
		t.Errorf("the limitation crowded out a real problem: %v", resolution.Notices)
	}
	if len(resolution.Problems) == 0 {
		t.Error("the misconfiguration was not reported")
	}
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
