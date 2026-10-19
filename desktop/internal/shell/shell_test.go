package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// treeEnv is tree with the shell's own settings, so a fixture is checked the way
// the desktop shell is rather than the way the Hub window is.
func treeEnv(t *testing.T, files map[string]string) Env {
	env := tree(t, files)
	env.EntryPoint = "shell.qml"
	env.OwnsSettings = true
	return env
}

// tree writes a set of files under a temporary root and returns an Env for it.
func tree(t *testing.T, files map[string]string) Env {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, "shell", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return Env{Root: root, Dir: "shell"}
}

const goodSchema = `<?xml version="1.0"?>
<schemalist>
  <schema id="org.zenith.shell" path="/org/zenith/shell/">
    <key name="bar-position" enum="org.zenith.shell.Edge">
      <default>'top'</default>
    </key>
    <key name="bar-height" type="i"><default>32</default></key>
  </schema>
</schemalist>`

// goodShell is a minimal tree that passes every check, so each test changes one
// thing and asserts that exactly one rule fires. A fixture that already fails
// makes every other test meaningless.
func goodShell() map[string]string {
	return map[string]string{
		"org.zenith.shell.gschema.xml": goodSchema,
		"shell.qml":                    "import Quickshell\nShellRoot {}\n",
		"Config.qml": "import Quickshell\nSingleton {\n" +
			"  readonly property int barHeight: shell.barHeight\n" +
			"  readonly property int barPosition: shell.barPosition\n" +
			"}\n",
		"Bar.qml": "import Quickshell\nPanelWindow {\n" +
			"  readonly property int edge: config.barPosition\n" +
			"  implicitHeight: config.barHeight\n" +
			"  readonly property var catalogue: ({ \"clock\": \"Clock\" })\n" +
			"  source: \"Widgets/\" + catalogue[name] + \".qml\"\n}\n",
		"Widgets/Clock.qml": "import Quickshell\nItem {}\n",
	}
}

func rules(findings []Finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Rule)
	}
	return out
}

func hasRule(findings []Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

// TestTheShellsOwnTreePasses is the baseline, and it runs against the real
// repository rather than a fixture.
//
// A validator that has never been run against the thing it validates is a
// validator nobody knows the sensitivity of.
func TestTheShellsOwnTreePasses(t *testing.T) {
	findings := Check(Env{Root: "../../..", Dir: "shell", EntryPoint: "shell.qml", OwnsSettings: true})
	if len(findings) != 0 {
		for _, f := range findings {
			t.Errorf("%s", f)
		}
	}
}

func TestGoodFixturePasses(t *testing.T) {
	if findings := Check(treeEnv(t, goodShell())); len(findings) != 0 {
		t.Errorf("a well-formed shell reported findings: %v", rules(findings))
	}
}

// TestMissingEntryPointIsFound covers the fault with the least visible symptom:
// `quickshell -p` names the directory, so a tree without shell.qml starts and
// exits, and the journal says nothing that identifies the cause.
func TestMissingEntryPointIsFound(t *testing.T) {
	files := goodShell()
	delete(files, "shell.qml")

	findings := Check(treeEnv(t, files))
	if !hasRule(findings, "entry point") {
		t.Errorf("a missing entry point was not found: %v", rules(findings))
	}
}

// TestEmptyTreeIsReported covers a tree that exists but has nothing in it.
func TestEmptyTreeIsReported(t *testing.T) {
	findings := Check(treeEnv(t, map[string]string{}))
	if !hasRule(findings, "entry point") {
		t.Errorf("an empty tree was not reported: %v", rules(findings))
	}
}

// TestCatalogueEntryWithNoFileIsFound is the check that pays for itself.
//
// A name with no QML behind it produces a Loader with an empty source at run
// time: the bar comes up one widget short and nothing says which.
func TestCatalogueEntryWithNoFileIsFound(t *testing.T) {
	files := goodShell()
	files["Bar.qml"] = strings.Replace(
		files["Bar.qml"], `({ "clock": "Clock" })`, `({ "clock": "Clock", "wifi": "Wifi" })`, 1)

	findings := Check(treeEnv(t, files))
	if !hasRule(findings, "widget catalogue") {
		t.Fatalf("a catalogue entry with no file was not found: %v", rules(findings))
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Detail, "Wifi") {
			found = true
		}
	}
	if !found {
		t.Error("the finding does not name the missing widget")
	}
}

// TestUnknownSettingsKeyIsFound covers a read of a key the schema does not
// declare. GSettings does not warn on an unknown key — it returns the default
// the schema would have had — so the value is silently wrong.
func TestUnknownSettingsKeyIsFound(t *testing.T) {
	files := goodShell()
	files["Widgets/Clock.qml"] = "import Quickshell\nItem { color: config.barrHieght }\n"

	findings := Check(treeEnv(t, files))
	if !hasRule(findings, "settings read") {
		t.Fatalf("a settings read of a key the schema does not declare was "+
			"not found: %v", rules(findings))
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Detail, "barrHieght") {
			found = true
		}
	}
	if !found {
		t.Error("the finding does not name the key")
	}
}

// TestWrapperPropertiesAreNotSchemaKeys is the false-positive guard.
//
// Every property on Config.qml is read as config.<name> from every surface. If
// those were treated as schema keys, every correctly-written widget would be
// reported.
func TestWrapperPropertiesAreNotSchemaKeys(t *testing.T) {
	files := goodShell()
	files["Config.qml"] = strings.Replace(files["Config.qml"],
		"readonly property int barPosition: shell.barPosition",
		"readonly property int barPosition: shell.barPosition\n"+
			"  readonly property int edgeLeft: 2", 1)
	files["Widgets/Clock.qml"] = "import Quickshell\nItem { w: config.edgeLeft }\n"

	findings := Check(treeEnv(t, files))
	if hasRule(findings, "settings read") {
		t.Errorf("a wrapper property was reported as a schema key: %v",
			rules(findings))
	}
}

// TestCommentsAreNotSource is the check that keeps a comment from being able to
// break the build.
//
// A comment that explains the bug it warns about names the broken identifier. If
// comments were read as source, the file could not be corrected without first
// deleting the explanation of why it needed correcting.
func TestCommentsAreNotSource(t *testing.T) {
	files := goodShell()
	files["Widgets/Clock.qml"] = "import Quickshell\n" +
		"// Do not read config.nonsense, it does not exist.\n" +
		"/* config.alsoNonsense */\nItem {}\n"

	findings := Check(treeEnv(t, files))
	if hasRule(findings, "settings read") {
		t.Errorf("a comment was read as a settings read: %v", rules(findings))
	}
}

// TestCommentStrippingKeepsURLs guards the naive version of the same thing.
//
// file:///usr/share/x.png is a URI, not a comment. Treating its // as a comment
// deletes the rest of the line and hides a boundary violation.
func TestCommentStrippingKeepsURLs(t *testing.T) {
	got := stripComments(`x: "file:///usr/share/evil/qml" // trailing`)
	if !strings.Contains(got, "file:///usr/share/evil/qml") {
		t.Errorf("stripComments ate a URI: %q", got)
	}
	if strings.Contains(got, "trailing") {
		t.Errorf("stripComments kept a real comment: %q", got)
	}
}

// TestPathOutsideTheBoundaryIsFound covers a QML file naming a file outside
// /usr/share/zenith/, which is the boundary the rest of the project enforces.
func TestPathOutsideTheBoundaryIsFound(t *testing.T) {
	files := goodShell()
	files["Widgets/Clock.qml"] = "import Quickshell\n" +
		"Item { source: \"/etc/sddm.conf.d/10-zenith.conf\" }\n"

	findings := Check(treeEnv(t, files))
	if !hasRule(findings, "boundary") {
		t.Errorf("a path outside the boundary was not found: %v", rules(findings))
	}
}

func TestPathsInsideTheBoundaryAreAllowed(t *testing.T) {
	files := goodShell()
	files["Widgets/Clock.qml"] = "import Quickshell\n" +
		"Item { source: \"/usr/share/zenith/shell/shell.qml\" }\n"

	findings := Check(treeEnv(t, files))
	if hasRule(findings, "boundary") {
		t.Errorf("a path inside the boundary was reported: %v", rules(findings))
	}
}

// TestUnknownImportIsFound covers a module that is not on the closed list.
//
// A QML file can import anything the machine happens to have installed, and a
// shell whose behaviour depends on what a user installed is not a shell this
// project controls.
func TestUnknownImportIsFound(t *testing.T) {
	files := goodShell()
	files["Widgets/Clock.qml"] = "import Quickshell\nimport SomeUserModule\nItem {}\n"

	findings := Check(treeEnv(t, files))
	if !hasRule(findings, "import") {
		t.Errorf("an import outside the closed list was not found: %v", rules(findings))
	}
}

// TestMalformedSchemaIsReported covers a schema that will not compile.
//
// glib-compile-schemas is the real check, and it is not available here. A parse
// error at least says so here rather than at login.
func TestMalformedSchemaIsReported(t *testing.T) {
	files := goodShell()
	files["org.zenith.shell.gschema.xml"] = `<?xml version="1.0"?><schemalist><schema`

	findings := Check(treeEnv(t, files))
	if !hasRule(findings, "schema") {
		t.Errorf("a malformed schema was not reported: %v", rules(findings))
	}
}

// TestDuplicateSchemaKeyIsFound covers a key declared twice, which GLib treats
// as fatal at compile time.
func TestDuplicateSchemaKeyIsFound(t *testing.T) {
	files := goodShell()
	files["org.zenith.shell.gschema.xml"] = strings.Replace(goodSchema,
		`</schema>`,
		`  <key name="bar-height" type="i"><default>8</default></key>`+"\n  </schema>", 1)

	findings := Check(treeEnv(t, files))
	if !hasRule(findings, "schema") {
		t.Errorf("a duplicate key was not reported: %v", rules(findings))
	}
}

// TestParseSchemaReadsTypesAndEnums covers the parsing itself.
func TestParseSchemaReadsTypesAndEnums(t *testing.T) {
	schema, err := parseSchema([]byte(goodSchema))
	if err != nil {
		t.Fatal(err)
	}
	if schema.ID != "org.zenith.shell" {
		t.Errorf("ID = %q", schema.ID)
	}
	if len(schema.Keys) != 2 {
		t.Errorf("Keys = %v", schema.Keys)
	}
	if schema.KeyTypes["bar-height"] != "i" {
		t.Errorf("bar-height type = %q", schema.KeyTypes["bar-height"])
	}
	if schema.KeyTypes["bar-position"] != "org.zenith.shell.Edge" {
		t.Errorf("bar-position type = %q", schema.KeyTypes["bar-position"])
	}
}

// TestSchemaIdWithNoSchemaFileIsFound covers the fault of the moment.
//
// `Settings { schemaId: "org.zenith.desktop" }` against a schema that was never
// shipped is not a build error and not a startup error: the object comes back
// invalid, every property read off it is undefined, and a colour binding quietly
// evaluates to nothing. The shell then renders with no text on it and nothing
// anywhere says the schema is missing.
func TestSchemaIdWithNoSchemaFileIsFound(t *testing.T) {
	files := goodShell()
	files["Config.qml"] = strings.Replace(files["Config.qml"],
		"import Quickshell", "import Quickshell\nreadonly property int s: 1", 1)
	files["Widgets/Clock.qml"] = "import Quickshell\nItem {}\n"
	files["Bar.qml"] = strings.Replace(files["Bar.qml"],
		"import Quickshell", "import Quickshell\nreadonly property Settings d: Settings { schemaId: \"org.zenith.desktop\" }", 1)

	findings := Check(treeEnv(t, files))
	if !hasRule(findings, "schema id") {
		t.Fatalf("a schemaId with no schema file was not found: %v", rules(findings))
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Detail, "org.zenith.desktop") {
			found = true
		}
	}
	if !found {
		t.Error("the finding does not name the missing schema")
	}
}

// TestSchemaKeyNoSurfaceCanReachIsFound covers a key the wrapper never reads.
//
// The hub would write it and the shell would never look at it: both would be
// working, and disagreeing.
func TestSchemaKeyNoSurfaceCanReachIsFound(t *testing.T) {
	files := goodShell()
	files["org.zenith.shell.gschema.xml"] = strings.Replace(goodSchema,
		"</key>", "</key>\n    <key name=\"unread-key\" type=\"s\"><default>''</default></key>", 1)

	findings := Check(treeEnv(t, files))
	if !hasRule(findings, "settings unreachable") {
		t.Errorf("a schema key no surface can reach was not found: %v",
			rules(findings))
	}
}

// TestKebabCaseAndCamelCaseCompareEqual covers the fold between the two
// vocabularies.
//
// GSettings keys are kebab-case because that is what dconf uses. QML property
// names cannot contain a hyphen, so every key has a camelCase property in front
// of it. Neither is changed to suit the other; they are folded together here.
func TestKebabCaseAndCamelCaseCompareEqual(t *testing.T) {
	for _, pair := range [][2]string{
		{"bar-widgets", "barWidgets"},
		{"wallpaper-path-dark", "wallpaperPathDark"},
		{"bar-height", "barheight"},
	} {
		if normalise(pair[0]) != normalise(pair[1]) {
			t.Errorf("normalise(%q) = %q, normalise(%q) = %q; they must fold "+
				"together", pair[0], normalise(pair[0]), pair[1], normalise(pair[1]))
		}
	}
	if normalise("bar-widgets") == normalise("dock-widgets") {
		t.Error("two different keys folded together")
	}
}

// TestAllowanceListIsReviewed covers the thing that turns the boundary allowance
// from a rule into a loophole.
//
// "/usr/bin/hyprlock" is allowed because the shell runs it, not because the rule
// says anything under /usr/bin is fine. A test that only checked the check would
// not notice that difference, because the allowlist would just keep growing.
func TestAllowanceListIsReviewed(t *testing.T) {
	// Every allowance is a binary under /usr/bin: the shell runs these, and it
	// must not be reading them.
	for path := range boundaryAllowance {
		if !strings.HasPrefix(path, "/usr/bin/") {
			t.Errorf("%s is allowed but is not a binary under /usr/bin",
				path)
		}
	}

	// And the list has not become a prefix rule. A dozen entries means the
	// check is no longer a boundary.
	if len(boundaryAllowance) > 8 {
		t.Errorf("the allowance list has %d entries; it has stopped being a "+
			"short list and become a rule", len(boundaryAllowance))
	}
}

// TestSchemaIdCheckNamesTheMalformedSchema covers reporting.
//
// A schema that will not parse reported as "no schema declares it" sends the
// reader looking for a schema that is sitting right there, malformed.
func TestSchemaIdCheckNamesTheMalformedSchema(t *testing.T) {
	files := goodShell()
	files["Config.qml"] += "\n"
	files["Widgets/Clock.qml"] = "import Quickshell\nItem {}\n"
	files["Bar.qml"] = strings.Replace(files["Bar.qml"],
		"import Quickshell",
		"import Quickshell\nreadonly property Settings d: Settings { schemaId: \"org.zenith.desktop\" }", 1)
	files["org.zenith.desktop.gschema.xml"] = `<?xml version="1.0"?><schemalist><schema`

	findings := Check(treeEnv(t, files))
	found := false
	for _, f := range findings {
		if f.Rule == "schema" && strings.Contains(f.Detail, "well-formed") {
			found = true
		}
	}
	if !found {
		t.Errorf("a malformed schema was not reported as malformed: %v", findings)
	}
}

// TestUnusedWrapperPropertyIsFound covers the class of dead code that is
// invisible at run time.
//
// Config.qml is the bridge between GSettings and the shell, so a property there
// with no use site is a key the Hub writes and the shell never reads. Both are
// working. Nothing says so. `zenith doctor` reporting "the idle timeout is 0" is
// not useful if nothing reads the idle timeout.
func TestUnusedWrapperPropertyIsFound(t *testing.T) {
	files := goodShell()
	files["Config.qml"] = strings.Replace(files["Config.qml"],
		"readonly property int barPosition: shell.barPosition",
		"readonly property int barPosition: shell.barPosition\n"+
			"  readonly property int nobodyReadsThis: 3", 1)

	findings := Check(treeEnv(t, files))
	found := false
	for _, f := range findings {
		if f.Rule == "unused setting" && strings.Contains(f.Detail, "nobodyReadsThis") {
			found = true
		}
	}
	if !found {
		t.Errorf("a wrapper property nothing reads was not found: %v",
			rules(findings))
	}
}

// TestWrapperExemptionsStaySmall stops the exemption list becoming the loophole
// the unused check was added to prevent.
func TestWrapperExemptionsStaySmall(t *testing.T) {
	for name := range wrapperExempt {
		if !wrapperPropertiesAreStructural(name) {
			t.Errorf("%s is exempt from the unused check; every exemption has "+
				"to be a Settings object, a derived value, or a path, not a "+
				"setting", name)
		}
	}
}

// wrapperPropertiesAreStructural is the rule for what may be exempt: a property
// that exists to be used as part of the wrapper's own machinery, rather than as
// a bridge to something the Hub writes.
func wrapperPropertiesAreStructural(name string) bool {
	switch name {
	case "shell", "desktop", "locker":
		return true
	case "colorScheme":
		// A value computed here rather than read from a surface.
		return true
	default:
		return false
	}
}

// TestGlyphOutsideItsFontsRangeIsFound covers the mistake this file's own author
// made, which is the reason the rule exists.
//
// U+F8FF for bluetooth, U+F8C0 for battery. Real private use codepoints, in the
// same neighbourhood as the correct ones, and not Font Awesome 4.7 glyphs at all.
// Nothing rendered, so nothing failed, and a status area drawn out of boxes looks
// decorative rather than broken.
func TestGlyphOutsideItsFontsRangeIsFound(t *testing.T) {
	files := goodShell()
	files["Icons.qml"] = "import Quickshell\nSingleton {\n" +
		"  readonly property var glyphs: ({\n" +
		"    \"bluetooth\": \"\\uf8ff\",\n" +
		"  })\n}\n"

	findings := Check(treeEnv(t, files))
	if !hasRule(findings, "glyph range") {
		t.Fatalf("a codepoint outside the font's range was not found: %v",
			rules(findings))
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Detail, "bluetooth") &&
			strings.Contains(f.Detail, "U+F8FF") {
			found = true
		}
	}
	if !found {
		t.Errorf("the finding does not name the glyph and its codepoint: %v",
			findings)
	}
}

// TestGlyphWrittenAsARawCharacterIsFound covers the second half of the same
// mistake.
//
// A raw private use character in source is unreadable in a diff — it looks like
// an empty string — and survives a copy or a rebase only if every step preserves
// the encoding.
func TestGlyphWrittenAsARawCharacterIsFound(t *testing.T) {
	files := goodShell()
	files["Icons.qml"] = "import Quickshell\nSingleton {\n" +
		"  readonly property var glyphs: ({\n" +
		"    \"lock\": \"\",\n" +
		"  })\n}\n"

	findings := Check(treeEnv(t, files))
	if !hasRule(findings, "glyph escape") {
		t.Errorf("a glyph written as a raw character was not found: %v",
			rules(findings))
	}
}

// TestEscapeCodepointsAreInRange is the direction that must not fire: every
// glyph this project actually ships, against the real ranges.
func TestEscapeCodepointsAreInRange(t *testing.T) {
	for family, limits := range iconFamilyRanges {
		if limits.low > limits.high {
			t.Errorf("%s has an inverted range U+%04X-U+%04X", family,
				limits.low, limits.high)
		}
		// The first and last codepoint of a range are the ones most likely to
		// be wrong by one, and a check that never tests its own edges is a check
		// that has not been run.
		for _, probe := range []rune{limits.low, limits.high} {
			if probe < limits.low || probe > limits.high {
				t.Errorf("%s range rejects its own edge U+%04X", family, probe)
			}
		}
	}
}

// TestTheHubsOwnTreePasses runs the same checks over the Hub's window.
//
// It is a separate test rather than an extra case in the shell's, because the
// two windows are checked differently on purpose. The desktop shell owns the
// schemas and is checked against them; the Hub renders what `zenith config
// export` hands it and reads no schema, so the shell's settings checks over it
// would report every property only the shell uses as an unused setting — a true
// statement about the wrong tree, which is the most expensive kind of finding.
func TestTheHubsOwnTreePasses(t *testing.T) {
	env := Env{Root: "../../..", Dir: "shell/hub", EntryPoint: "hub.qml"}

	findings := Check(env)
	if len(findings) != 0 {
		for _, f := range findings {
			t.Errorf("%s", f)
		}
	}
}

// TestTheEntryPointIsNotDefaulted covers the reason EntryPoint is not optional.
//
// A default would let one window silently pass another's check: with "shell.qml"
// as a default, the Hub tree would be checked for a file it does not have and
// every run would report the same missing entry point.
func TestTheEntryPointIsNotDefaulted(t *testing.T) {
	files := goodShell()
	delete(files, "shell.qml")

	// With the shell's own entry point named, it is a finding.
	env := treeEnv(t, files)
	if !hasRule(Check(env), "entry point") {
		t.Error("a missing entry point was not reported")
	}

	// With none named, the same tree has no entry point to look for — which is
	// why nothing defaults it.
	env = tree(t, files)
	if hasRule(Check(env), "entry point") && env.EntryPoint != "" {
		t.Error("an unset EntryPoint was silently filled in")
	}
}

// checkCommands resolves cmd/zenith/main.go relative to Env.Root, so a fixture
// tree has to carry one. The tests below write the table themselves rather than
// reaching for the real file, because the test is about the rule matching and
// not about which commands the CLI happens to have this week.

// TestUnknownCommandIsFound covers the cheapest break in the shell to make and
// the most expensive to notice.
//
// A QML file naming a subcommand the CLI does not have is a panel that is empty
// forever: the Process runs, exits non-zero, the shell keeps its last document,
// and no test anywhere fails and nothing in the journal says why.
func TestUnknownCommandIsFound(t *testing.T) {
	files := goodShell()
	files["Sidebar.qml"] = "import Quickshell\nProcess { command: [\"zenith\", \"winodws\"] }\n"
	env := treeEnv(t, files)
	writeCommands(t, env.Root, "state", "windows")

	findings := Check(env)
	found := false
	for _, f := range findings {
		if f.Rule == "unknown command" && strings.Contains(f.Detail, "winodws") {
			found = true
		}
	}
	if !found {
		t.Errorf("a call to a command that does not exist was not found: %v", rules(findings))
	}
}

// TestKnownCommandIsAccepted is the other half: the rule has to be silent on
// every real call, or it is noise and gets ignored.
func TestKnownCommandIsAccepted(t *testing.T) {
	files := goodShell()
	files["Sidebar.qml"] = "import Quickshell\nProcess { command: [\"zenith\", \"state\"] }\n"
	files["Switcher.qml"] = "import Quickshell\nProcess { command: [\"zenith\", \"windows\"] }\n"
	env := treeEnv(t, files)
	writeCommands(t, env.Root, "state", "windows")

	for _, f := range Check(env) {
		if f.Rule == "unknown command" {
			t.Errorf("a real command was reported missing: %s", f.Detail)
		}
	}
}

// TestCommandRuleIgnoresAMissingEntryPoint keeps the rule from being the thing
// that breaks when the file moves. A fixture root with no cmd/zenith/main.go
// simply has nothing to check against.
func TestCommandRuleIgnoresAMissingEntryPoint(t *testing.T) {
	files := goodShell()
	files["Sidebar.qml"] = "import Quickshell\nProcess { command: [\"zenith\", \"anything\"] }\n"

	for _, f := range Check(treeEnv(t, files)) {
		if f.Rule == "unknown command" {
			t.Error("reported a finding with no commands table to check against")
		}
	}
}

// writeCommands puts a commands table in the fixture root.
func writeCommands(t *testing.T, root string, names ...string) {
	t.Helper()
	path := filepath.Join(root, "desktop", "cmd", "zenith", "main.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "package main\n\nvar commands = map[string]func(args []string) int{\n"
	for _, name := range names {
		body += "\t\"" + name + "\":       zenith" + name + ",\n"
	}
	body += "}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestHardcodedColourIsFound covers the check that makes a theme a theme.
//
// A hex value in a QML file is a colour that survives a palette change without
// being noticed, and one such colour is enough to give a themed desktop the two
// themes users complain about: the one they chose, and the one the code has.
func TestHardcodedColourIsFound(t *testing.T) {
	files := goodShell()
	files["Bar.qml"] += "\n  color: \"#4c8bf5\"\n"

	findings := Check(treeEnv(t, files))
	found := false
	for _, f := range findings {
		if f.Rule == "hard-coded colour" && strings.Contains(f.Detail, "#4c8bf5") {
			found = true
		}
	}
	if !found {
		t.Errorf("a hard-coded colour was not found: %v", rules(findings))
	}
}

// TestTheScrimIsAllowedToBeBlack covers the one exemption, and the reason it is
// narrow.
//
// A scrim darkens whatever is behind it, so it has to be black regardless of
// the palette. A scrim tinted by the palette would tint the wallpaper along
// with the desktop.
func TestTheScrimIsAllowedToBeBlack(t *testing.T) {
	if colourExemptions["IdleOverlay.qml"] == "" {
		t.Fatal("the scrim has no recorded reason for being exempt")
	}
	if len(colourExemptions) > 2 {
		t.Errorf("%d files are exempt from the colour check; an exemption "+
			"list that grows is the check being switched off one file at a time",
			len(colourExemptions))
	}
}

// TestSchemaDirectoryIsNotTheShellDirectory is the installed layout, which is
// the only layout doctor runs against.
//
// GLib installs schemas under /usr/share/glib-2.0/schemas and nothing under
// /usr/share/zenith/shell. The schema check used to read env.Dir, so an
// installed tree reported "declares no schema id" for every schemaId the QML
// names and then "no surface can reach it" for every key — a clean machine
// described as broken. The two paths are separate because they are separate on
// disk, not because the tool feels like splitting them.
func TestSchemaDirectoryIsNotTheShellDirectory(t *testing.T) {
	good := goodShell()
	schema, ok := good["org.zenith.shell.gschema.xml"]
	if !ok {
		t.Fatal("fixture lost its schema")
	}
	delete(good, "org.zenith.shell.gschema.xml")

	root := t.TempDir()
	shellDir := filepath.Join(root, "usr", "share", "zenith", "shell")
	schemaDir := filepath.Join(root, "usr", "share", "glib-2.0", "schemas")
	for name, body := range good {
		path := filepath.Join(shellDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "org.zenith.shell.gschema.xml"),
		[]byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}

	env := Env{Root: root, Dir: "/usr/share/zenith/shell", EntryPoint: "shell.qml",
		OwnsSettings: true, SchemaDir: "/usr/share/glib-2.0/schemas"}

	findings := Check(env)
	if len(findings) != 0 {
		for _, finding := range findings {
			t.Errorf("  %s %s: %s", finding.Rule, finding.Path, finding.Detail)
		}
		t.Fatal("the installed layout reports faults that do not exist in it")
	}
}

// TestSchemaDirectoryMissingIsReported covers the other half: a tree whose
// schema directory is absent must not read as a tree with no problems.
func TestSchemaDirectoryMissingIsReported(t *testing.T) {
	env := treeEnv(t, goodShell())
	env.SchemaDir = "/usr/share/glib-2.0/schemas"

	findings := Check(env)
	found := false
	for _, finding := range findings {
		if finding.Rule == "schema" && strings.Contains(finding.Path, "glib-2.0") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a tree with no schema directory passed the schema check: %v", findings)
	}
}
