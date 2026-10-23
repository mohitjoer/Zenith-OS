package shell

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The catalogue and the compositor's configuration are the same list written
// down twice, and the tests below are about the two of them drifting apart.
//
// The first test is the one that matters: it runs against the repository's
// own files rather than a fixture, because a check that only ever runs on
// fixtures finds the mistake in the example and not in the file the
// compositor actually reads.

// repositoryEnv is the repository's own tree, with the compositor's
// configuration and the catalogue beside it.
func repositoryEnv() Env {
	return Env{
		Root:           "../../..",
		Dir:            "shell",
		EntryPoint:     "shell.qml",
		OwnsSettings:   true,
		HyprlandConfig: "session/hyprland/hyprland.conf",
		BindsCatalogue: "session/binds.json",
	}
}

// TestTheRepositorysCatalogueMatchesItsConfiguration is the check that pays
// for itself.
//
// A chord in the catalogue with no bind is a key that does nothing, and a
// bind with no catalogue entry is a key nobody can find. Both are silent at
// run time, and the user finds them by pressing a key that does not work.
func TestTheRepositorysCatalogueMatchesItsConfiguration(t *testing.T) {
	findings := checkBinds(repositoryEnv())
	if len(findings) != 0 {
		for _, finding := range findings {
			t.Errorf("%s %s: %s", finding.Rule, finding.Path, finding.Detail)
		}
	}
}

// TestTheRepositorysCatalogueAndConfigurationUseTheSameSpelling asserts that
// the two files agree about the keys themselves, not only about which chords
// exist.
//
// The catalogue spells a chord as "SUPER + SHIFT + {n}" and the compositor's
// configuration spells it as "SUPER SHIFT, 1", because Hyprland's syntax is
// comma-separated. That difference is the compositor's constraint and not a
// choice, so the comparison has to fold the two spellings together rather
// than insisting on one of them.
func TestTheRepositorysCatalogueAndConfigurationUseTheSameSpelling(t *testing.T) {
	env := repositoryEnv()

	body, err := readCompositorConfig(filepath.Join(env.Root, env.HyprlandConfig))
	if err != nil {
		t.Fatal(err)
	}
	binds := parseBinds(string(body))
	if len(binds) == 0 {
		t.Fatal("the compositor's configuration has no binds in it at all")
	}

	for _, bind := range binds {
		if bind.Chord == "" {
			t.Errorf("a bind on line %d has no chord", bind.Line)
		}
	}
}

// writeTreeFile writes one file into a fixture tree.
func writeTreeFile(t *testing.T, env Env, relative, body string) {
	t.Helper()
	path := filepath.Join(env.Root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// bindsTree writes a catalogue and a configuration into a temporary tree and
// returns the environment that reads them.
func bindsTree(t *testing.T, catalogue, config string) Env {
	t.Helper()
	env := Env{
		Root:           t.TempDir(),
		Dir:            "shell",
		EntryPoint:     "shell.qml",
		HyprlandConfig: "session/hyprland/hyprland.conf",
		BindsCatalogue: "session/binds.json",
	}
	writeTreeFile(t, env, env.HyprlandConfig, config)
	writeTreeFile(t, env, env.BindsCatalogue, catalogue)
	return env
}

// A catalogue with one binding in it, and a configuration with one bind.
const oneCatalogue = `{
  "categories": [{ "id": "desktop", "label": "Desktop" }],
  "binds": [
    {
      "id": "launcher",
      "category": "desktop",
      "label": "Open the launcher",
      "chord": "SUPER + Space",
      "kind": "shell",
      "action": "togglePopover",
      "argument": "launcher"
    }
  ]
}`

const oneConfig = `bind = SUPER, Space, exec, quickshell ipc --path /usr/share/zenith/shell call zenith togglePopover launcher
`

// TestACatalogueThatAgreesWithTheConfigurationIsQuiet is the baseline.
//
// A fixture that already fails makes every other test meaningless, so the
// first thing checked is that the shape the other tests break does not fail
// on its own.
func TestACatalogueThatAgreesWithTheConfigurationIsQuiet(t *testing.T) {
	env := bindsTree(t, oneCatalogue, oneConfig)
	if findings := checkBinds(env); len(findings) != 0 {
		t.Errorf("a catalogue and a configuration that agree reported findings: %v",
			findings)
	}
}

// TestACatalogueChordWithNoBindIsFound covers the fault the catalogue exists
// to prevent: a key that is listed and does not work.
func TestACatalogueChordWithNoBindIsFound(t *testing.T) {
	env := bindsTree(t, oneCatalogue, "bind = SUPER, T, exec, ghostty\n")

	findings := checkBinds(env)
	found := false
	for _, finding := range findings {
		if finding.Rule == "keybind" &&
			strings.Contains(finding.Detail, "SUPER + Space") {
			found = true
		}
	}
	if !found {
		t.Errorf("a chord the configuration does not bind was not found: %v",
			rules(findings))
	}
}

// TestABindWithNoCatalogueEntryIsFound covers the other direction: a key
// that works and that nobody can find, because the list of the keys does not
// mention it.
func TestABindWithNoCatalogueEntryIsFound(t *testing.T) {
	env := bindsTree(t, oneCatalogue, oneConfig+
		"bind = SUPER, T, exec, ghostty\n")

	findings := checkBinds(env)
	found := false
	for _, finding := range findings {
		if finding.Rule == "keybind" &&
			strings.Contains(finding.Detail, "SUPER + T") {
			found = true
		}
	}
	if !found {
		t.Errorf("a bind the catalogue does not list was not found: %v",
			rules(findings))
	}
}

// TestABindThatDoesSomethingElseIsFound covers the drift neither of the other
// two catches: the chord is in both files, and the two disagree about what
// it does.
func TestABindThatDoesSomethingElseIsFound(t *testing.T) {
	env := bindsTree(t, oneCatalogue,
		"bind = SUPER, Space, exec, quickshell ipc --path /usr/share/zenith/shell call zenith togglePopover quick\n")

	findings := checkBinds(env)
	found := false
	for _, finding := range findings {
		if finding.Rule == "keybind" &&
			strings.Contains(finding.Detail, "togglePopover quick") {
			found = true
		}
	}
	if !found {
		t.Errorf("a bind that does something else was not found: %v",
			rules(findings))
	}
}

// TestAFamilyOfTenKeysIsTenBinds covers the placeholder.
//
// The catalogue has one entry for the ten workspaces and the configuration
// has ten binds, and the placeholder in the argument has to expand with the
// key: the 0 key is the tenth workspace and not a workspace called zero.
func TestAFamilyOfTenKeysIsTenBinds(t *testing.T) {
	const catalogue = `{
  "categories": [{ "id": "workspaces", "label": "Workspaces" }],
  "binds": [
    {
      "id": "workspace",
      "category": "workspaces",
      "label": "Switch to a workspace",
      "chord": "SUPER + {n}",
      "kind": "wm",
      "action": "workspace",
      "argument": "{n}"
    }
  ]
}`

	var config strings.Builder
	for i := 1; i <= 10; i++ {
		ordinal := strconv.Itoa(i)
		// The last key is 0, and it is the tenth workspace; the pad has the
		// same ten keys twice, with NumLock on and off.
		for _, key := range []string{strconv.Itoa(i % 10), "KP_" + strconv.Itoa(i%10),
			[]string{"KP_End", "KP_Down", "KP_Next", "KP_Left", "KP_Begin",
				"KP_Right", "KP_Home", "KP_Up", "KP_Prior", "KP_Insert"}[i-1]} {
			config.WriteString("bind = SUPER, " + key + ", workspace, " + ordinal + "\n")
		}
	}

	env := bindsTree(t, catalogue, config.String())
	findings := checkBinds(env)
	for _, finding := range findings {
		if finding.Rule == "keybind" {
			t.Errorf("the family did not expand the way the catalogue says: %s",
				finding.Detail)
		}
	}
}

// TestTheTenthKeyIsTheTenthOne covers the one thing about the family that is
// a convention rather than an arithmetic: the 0 key is the tenth of
// something, because the number row has no room for a tenth digit.
func TestTheTenthKeyIsTheTenthOne(t *testing.T) {
	entry := Bind{
		Chord: "SUPER + {n}", Kind: KindWM, Action: "workspace", Argument: "{n}",
	}
	expanded, err := entry.Expand()
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded) != 30 {
		t.Fatalf("the number row and the pad expanded to %d, want 30", len(expanded))
	}
	for _, want := range []string{"SUPER + KP_0", "SUPER + KP_Insert"} {
		found := false
		for _, item := range expanded {
			if item.Chord == want && item.Argument == "10" {
				found = true
			}
		}
		if !found {
			t.Errorf("%q is not the tenth workspace", want)
		}
	}
	if expanded[9].Chord != "SUPER + 0" {
		t.Errorf("the tenth key is %q, want %q", expanded[9].Chord, "SUPER + 0")
	}
	if expanded[9].Argument != "10" {
		t.Errorf("the tenth argument is %q, want %q", expanded[9].Argument, "10")
	}
	if expanded[0].Chord != "SUPER + 1" || expanded[0].Argument != "1" {
		t.Errorf("the first key is %q %q, want %q %q", expanded[0].Chord,
			expanded[0].Argument, "SUPER + 1", "1")
	}
}

// TestAMalformedChordIsFound covers a catalogue entry that cannot be read as
// a chord at all.
//
// A chord with an unknown modifier, or with its modifiers in the wrong order,
// is two spellings of one chord waiting to happen, and the comparison cannot
// be made against a chord it cannot read.
func TestAMalformedChordIsFound(t *testing.T) {
	const catalogue = `{
  "categories": [{ "id": "desktop", "label": "Desktop" }],
  "binds": [
    {
      "id": "launcher",
      "category": "desktop",
      "label": "Open the launcher",
      "chord": "SHIFT + SUPER + Space",
      "kind": "shell",
      "action": "togglePopover",
      "argument": "launcher"
    }
  ]
}`

	env := bindsTree(t, catalogue, oneConfig)
	findings := checkBinds(env)
	if !hasRule(findings, "keybind") {
		t.Errorf("a chord written out of order was not found: %v", rules(findings))
	}
}

// TestTwoEntriesCannotClaimTheSameChord covers the catalogue entry that is
// listed twice.
//
// Two entries claiming one chord is a catalogue that says the key does two
// things, and the comparison cannot say which of them the compositor is
// bound to.
func TestTwoEntriesCannotClaimTheSameChord(t *testing.T) {
	const catalogue = `{
  "categories": [{ "id": "desktop", "label": "Desktop" }],
  "binds": [
    {
      "id": "launcher",
      "category": "desktop",
      "label": "Open the launcher",
      "chord": "SUPER + Space",
      "kind": "shell",
      "action": "togglePopover",
      "argument": "launcher"
    },
    {
      "id": "launcher-again",
      "category": "desktop",
      "label": "Open the launcher",
      "chord": "SUPER + Space",
      "kind": "shell",
      "action": "togglePopover",
      "argument": "launcher"
    }
  ]
}`

	env := bindsTree(t, catalogue, oneConfig)
	findings := checkBinds(env)
	found := false
	for _, finding := range findings {
		if finding.Rule == "keybind" &&
			strings.Contains(finding.Detail, "both claim") {
			found = true
		}
	}
	if !found {
		t.Errorf("two entries claiming one chord were not found: %v",
			rules(findings))
	}
}

// TestACatalogueEntryMustBeFileableUnderACategory covers the entry that is in
// the catalogue, is bound, and that the cheat sheet never draws.
//
// The cheat sheet groups the bindings by their category, so a binding filed
// under a category the file does not declare is a binding in a list nobody
// reads.
func TestACatalogueEntryMustBeFileableUnderACategory(t *testing.T) {
	const catalogue = `{
  "categories": [{ "id": "desktop", "label": "Desktop" }],
  "binds": [
    {
      "id": "launcher",
      "category": "nowhere",
      "label": "Open the launcher",
      "chord": "SUPER + Space",
      "kind": "shell",
      "action": "togglePopover",
      "argument": "launcher"
    }
  ]
}`

	env := bindsTree(t, catalogue, oneConfig)
	findings := checkBinds(env)
	if !hasRule(findings, "keybinding catalogue") {
		t.Errorf("a binding filed under a category that does not exist was "+
			"not found: %v", rules(findings))
	}
}

// TestACatalogueEntryMustHaveAKindTheRulesKnow covers the kind.
//
// A kind nobody has a rule for is a kind the checks cannot check, and a
// binding whose kind is a typo is a binding whose action is read the wrong
// way.
func TestACatalogueEntryMustHaveAKindTheRulesKnow(t *testing.T) {
	const catalogue = `{
  "categories": [{ "id": "desktop", "label": "Desktop" }],
  "binds": [
    {
      "id": "launcher",
      "category": "desktop",
      "label": "Open the launcher",
      "chord": "SUPER + Space",
      "kind": "shelll",
      "action": "togglePopover",
      "argument": "launcher"
    }
  ]
}`

	env := bindsTree(t, catalogue, oneConfig)
	findings := checkBinds(env)
	if !hasRule(findings, "keybinding catalogue") {
		t.Errorf("a binding with an unknown kind was not found: %v", rules(findings))
	}
}

// TestAReleaseBindingIsChecked covers the window switcher, which is the one
// binding that does something different when the key comes back up.
//
// The catalogue says the press advances and the release closes, and a
// configuration with only the press is a switcher that keeps its overlay up
// after the user let go of the key. The release is `bindr`: `bindl` is the
// lock-screen flag and fires on the press, so a press/bindl pair opens and
// closes the card in the same instant.
func TestAReleaseBindingIsChecked(t *testing.T) {
	const catalogue = `{
  "categories": [{ "id": "desktop", "label": "Desktop" }],
  "binds": [
    {
      "id": "switcher",
      "category": "desktop",
      "label": "Switch to another window",
      "chord": "ALT + Tab",
      "kind": "shell",
      "action": "advanceSwitcher",
      "releaseAction": "closeSwitcher"
    }
  ]
}`

	press := "bind = ALT, Tab, exec, zenith ipc call zenith advanceSwitcher\n"
	release := "bindr = ALT, Tab, exec, zenith ipc call zenith closeSwitcher\n"

	// Every spelling of a release Hyprland allows counts.
	for _, directive := range []string{"bindr", "bindrl", "bindlr", "bindre"} {
		env := bindsTree(t, catalogue, press+
			strings.Replace(release, "bindr", directive, 1))
		for _, finding := range checkBinds(env) {
			if finding.Rule == "keybind" {
				t.Errorf("%s as the release reported a finding: %s",
					directive, finding.Detail)
			}
		}
	}

	env := bindsTree(t, catalogue, press)
	found := false
	for _, finding := range checkBinds(env) {
		if finding.Rule == "keybind" &&
			strings.Contains(finding.Detail, "closeSwitcher") {
			found = true
		}
	}
	if !found {
		t.Error("a switcher with no release binding was not found")
	}

	// bindl is not a release: it fires on the press.
	env = bindsTree(t, catalogue, press+strings.Replace(release, "bindr", "bindl", 1))
	found = false
	for _, finding := range checkBinds(env) {
		if finding.Rule == "keybind" &&
			strings.Contains(finding.Detail, "closeSwitcher") {
			found = true
		}
	}
	if !found {
		t.Error("a switcher closed by bindl, which fires on the press, was accepted")
	}
}

// TestAlternatesSubmapsAndTheLidAreRead covers what the repository's real
// configuration leans on: an entry's other chords, binds inside a submap
// (resize mode's bare arrows), the lid switch, and binds in a second file.
func TestAlternatesSubmapsAndTheLidAreRead(t *testing.T) {
	const catalogue = `{
  "categories": [{ "id": "windows", "label": "Windows" }],
  "binds": [
    {
      "id": "focus",
      "category": "windows",
      "label": "Focus the window in that direction",
      "chord": "SUPER + Left",
      "kind": "wm",
      "action": "movefocus",
      "argument": "l",
      "also": [{ "chord": "SUPER + Right", "argument": "r" }]
    }
  ]
}`
	env := bindsTree(t, catalogue, "source = binds.conf\n")
	writeTreeFile(t, env, "session/hyprland/binds.conf",
		"bind = SUPER, Left, movefocus, l\n"+
			"bind = SUPER, Right, movefocus, r\n"+
			"bindl = , switch:on:Lid Switch, exec, zenith-clamshell lid close\n"+
			"submap = resize\n"+
			"binde = , Left, resizeactive, -40 0\n"+
			"submap = reset\n")
	if findings := checkBinds(env); len(findings) != 0 {
		t.Errorf("alternates, a submap and the lid reported findings: %v", findings)
	}

	// The alternate is accounted for: dropping its bind is a finding.
	writeTreeFile(t, env, "session/hyprland/binds.conf",
		"bind = SUPER, Left, movefocus, l\n")
	found := false
	for _, finding := range checkBinds(env) {
		if strings.Contains(finding.Detail, "SUPER + Right") {
			found = true
		}
	}
	if !found {
		t.Error("an alternate chord with no bind was not found")
	}
}
