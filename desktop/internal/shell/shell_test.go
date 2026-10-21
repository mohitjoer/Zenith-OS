package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"zenith/desktop/internal/settings"
)

// treeEnv is tree with the shell's own settings, so a fixture is checked the way
// the desktop shell is rather than the way the Hub window is.
func treeEnv(t *testing.T, files map[string]string) Env {
	env := tree(t, files)
	env.EntryPoint = "shell.qml"
	env.OwnsSettings = true
	return env
}

// keys is the file list Check would pass, in the fixture's own layout.
func keys(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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

// TestIPCTargetMustBeRegistered keeps a keybind and the surface it names from
// drifting apart.
//
// A keybind is a string in the compositor's configuration and the surface it
// names is a string in the shell's QML, and nothing in either file checks the
// other. The failure is the quietest one in the desktop: the compositor runs
// the command, the command succeeds, and no launcher opens.
func TestIPCTargetMustBeRegistered(t *testing.T) {
	const handler = `import Quickshell.Io
IpcHandler {
    target: "zenith"
    function togglePopover(name: string): void { root.togglePopover(name) }
}
`
	const config = `bind = SUPER, Space, exec, quickshell ipc call %s togglePopover launcher
`

	// The two agreeing: no finding.
	env := tree(t, map[string]string{"shell.qml": handler})
	env.HyprlandConfig = "hyprland/hyprland.conf"
	writeConfig(t, env, "hyprland.conf", fmt.Sprintf(config, "zenith"))
	if findings := checkIPCTargets(env, keys(map[string]string{"shell.qml": handler})); len(findings) != 0 {
		t.Errorf("findings = %v, want none: the compositor and the shell name "+
			"the same target", findings)
	}

	// The compositor naming a target the shell does not register: a finding,
	// because that keybind runs and does nothing.
	env = tree(t, map[string]string{"shell.qml": handler})
	env.HyprlandConfig = "hyprland/hyprland.conf"
	writeConfig(t, env, "hyprland.conf", fmt.Sprintf(config, "zenit"))
	findings := checkIPCTargets(env, keys(map[string]string{"shell.qml": handler}))
	if len(findings) != 1 {
		t.Fatalf("findings = %v, want exactly one", findings)
	}
	if findings[0].Rule != "ipc target" {
		t.Errorf("rule = %q, want %q", findings[0].Rule, "ipc target")
	}

	// A tree with no compositor configuration is not a fault: there is
	// nothing to disagree with.
	env = tree(t, map[string]string{"shell.qml": handler})
	if findings := checkIPCTargets(env, keys(map[string]string{"shell.qml": handler})); len(findings) != 0 {
		t.Errorf("findings = %v, want none: there is no compositor configuration "+
			"to disagree with", findings)
	}
}

// writeConfig writes the compositor's configuration into the tree.
func writeConfig(t *testing.T, env Env, name, body string) {
	t.Helper()
	dir := filepath.Join(env.Root, filepath.Dir(env.HyprlandConfig))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(env.Root, env.HyprlandConfig)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTheIPCCallMustNameAFunctionTheShellHas covers the other half of a call.
//
// The target is who is being called and the function is what is being asked
// for. A call to a target the shell does not register reaches nothing at all,
// and a call to a function the shell does not define reaches the shell and
// asks it for something it has not got — which is the quieter of the two
// faults, because the command succeeds and nothing anywhere says why.
func TestTheIPCCallMustNameAFunctionTheShellHas(t *testing.T) {
	const handler = `import Quickshell.Io
IpcHandler {
    target: "zenith"
    function togglePopover(name: string): void { root.togglePopover(name) }
}
`
	// The command as the compositor writes it, with the path between "ipc"
	// and "call" that the first version of this rule did not allow for.
	const config = `bind = SUPER, Space, exec, quickshell ipc --path /usr/share/zenith/shell call zenith togglePopover launcher
`

	env := tree(t, map[string]string{"shell.qml": handler})
	env.HyprlandConfig = "hyprland/hyprland.conf"
	writeConfig(t, env, "hyprland.conf", config)
	if findings := checkIPCFunctions(env, keys(map[string]string{"shell.qml": handler})); len(findings) != 0 {
		t.Errorf("findings = %v, want none: the compositor names a "+
			"function the shell defines", findings)
	}

	// A function the shell does not have: a renamed function, which is how
	// this is reached in practice.
	env = tree(t, map[string]string{"shell.qml": handler})
	env.HyprlandConfig = "hyprland/hyprland.conf"
	writeConfig(t, env, "hyprland.conf", strings.Replace(config,
		"togglePopover launcher", "toggleLauncher launcher", 1))

	findings := checkIPCFunctions(env, keys(map[string]string{"shell.qml": handler}))
	if len(findings) != 1 {
		t.Fatalf("findings = %v, want exactly one", findings)
	}
	if findings[0].Rule != "ipc function" {
		t.Errorf("rule = %q, want %q", findings[0].Rule, "ipc function")
	}
	if !strings.Contains(findings[0].Detail, "toggleLauncher") {
		t.Errorf("the finding does not name the function: %s", findings[0].Detail)
	}
}

// TestTheRepositorysKeybindsCallFunctionsTheShellHas runs the rule against the
// repository's own files rather than a fixture.
//
// The compositor's configuration and the shell's QML are two files that name
// each other, and the check that compares them is worth nothing if it is
// never run against the two it is about.
func TestTheRepositorysKeybindsCallFunctionsTheShellHas(t *testing.T) {
	env := Env{
		Root:           "../../..",
		Dir:            "shell",
		EntryPoint:     "shell.qml",
		OwnsSettings:   true,
		HyprlandConfig: "session/hyprland/hyprland.conf",
	}

	files, err := qmlFiles(env)
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range checkIPCFunctions(env, files) {
		t.Errorf("%s %s: %s", finding.Rule, finding.Path, finding.Detail)
	}
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
			"  readonly property int edge: Config.barPosition\n" +
			"  implicitHeight: Config.barHeight\n" +
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
	files["Widgets/Clock.qml"] = "import Quickshell\nItem { color: Config.barrHieght }\n"

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
// Every property on Config.qml is read as Config.<name> from every surface. If
// those were treated as schema keys, every correctly-written widget would be
// reported.
func TestWrapperPropertiesAreNotSchemaKeys(t *testing.T) {
	files := goodShell()
	files["Config.qml"] = strings.Replace(files["Config.qml"],
		"readonly property int barPosition: shell.barPosition",
		"readonly property int barPosition: shell.barPosition\n"+
			"  readonly property int edgeLeft: 2", 1)
	files["Widgets/Clock.qml"] = "import Quickshell\nItem { w: Config.edgeLeft }\n"

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
		"// Do not read Config.nonsense, it does not exist.\n" +
		"/* Config.alsoNonsense */\nItem {}\n"

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

// TestAnAliasedImportIsFound covers the form of an import the rule used to
// be blind to.
//
// An import can be given a name — `import SomeUserModule as Whatever` — and
// the rule used to match only the lines that ended after the module. So the
// one spelling of an import that brought in a module the machine happened
// to have was the one the rule could not see.
func TestAnAliasedImportIsFound(t *testing.T) {
	files := goodShell()
	files["Widgets/Clock.qml"] = "import Quickshell\nimport SomeUserModule as Whatever\nItem {}\n"

	findings := Check(treeEnv(t, files))
	if !hasRule(findings, "import") {
		t.Errorf("an aliased import outside the closed list was not found: %v",
			rules(findings))
	}
}

// TestTheShellsOwnNamespaceIsAllowed covers the one namespace the closed
// list does not have to name.
//
// The shell's root is importable as `qs`, and the vendored component kit
// lives inside it. Those are this project's own modules, pinned to a
// commit and shipped by this project, so nothing a user installed provides
// them — which is the thing the closed list is for.
func TestTheShellsOwnNamespaceIsAllowed(t *testing.T) {
	files := goodShell()
	files["Widgets/Clock.qml"] = "import Quickshell\nimport qs\n" +
		"import qs.modules.common as Kit\nimport qs.modules.common.widgets\nItem {}\n"

	for _, f := range Check(treeEnv(t, files)) {
		if f.Rule == "import" {
			t.Errorf("the shell's own namespace was reported against the "+
				"closed list: %s", f)
		}
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

// TestTheHubsEntryPointIsInTheTree covers the one claim the shell's pass cannot.
//
// Both windows live in one directory. The Hub is drawn with the same component
// kit as the desktop, and the kit's files reach each other through `qs.modules.*`
// — imports that only resolve when the configuration's root is the directory
// both entry files are in. So there is no second tree, the shell's pass already
// covers every file the Hub draws, and the reason the windows used to be checked
// apart is gone: the Hub reads no schema, and the shell's settings checks are
// about `Config.qml`, which no Hub file is.
//
// What the shell's pass cannot see is that `hub.qml` is still the file at the top
// of the tree. The entry point it looks for is `shell.qml`, and `zenith-hub`
// names the Hub's path rather than the tree's — a rename would leave the settings
// window starting a file that is not there, with nothing in this package naming
// the path it used to have.
func TestTheHubsEntryPointIsInTheTree(t *testing.T) {
	env := Env{Root: "../../..", Dir: "shell", EntryPoint: "hub.qml"}

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
// as a default, a tree whose own entry file is named something else is checked
// against a file that may well be there — the desktop's — and passes as if the
// window it was asked about had been checked.
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

// TestThirdPartySchemasAreNotReported is `zenith doctor` on a machine that has
// a desktop on it, which is the machine the tool ships to.
//
// The schema directory is shared. /usr/share/glib-2.0/schemas holds every
// schema the base layer has — GTK, dconf, gsettings-desktop-schemas, whatever
// the user installed — and Zenith's two files are a rounding error in it. The
// check used to read all of them, and on a live Ubuntu install doctor reported
// thousands of findings: a "settings unreachable" line for every key of every
// third-party schema, which is a true statement about files this project does
// not own, and "declared twice" for base-layer files that repeat a key name
// across two schema elements in one file, which GLib accepts and this parser
// flattens into one id.
//
// The staged tree in CI holds only Zenith's schemas, which is why no test saw
// it: the fault only exists on a machine with other people's schemas on it.
func TestThirdPartySchemasAreNotReported(t *testing.T) {
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
	for _, dir := range []string{shellDir, schemaDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "org.zenith.shell.gschema.xml"),
		[]byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}

	thirdParty := map[string]string{
		// The shape shotwell ships: one file, two schema elements declaring the
		// same key name under different paths. Legal to GLib, and reported as a
		// duplicate declaration by a parser that flattens the file.
		"org.gnome.shotwell.gschema.xml": `<?xml version="1.0"?>
<schemalist>
  <schema id="org.gnome.shotwell" path="/org/gnome/shotwell/">
    <key name="strip-metadata" type="b"><default>false</default></key>
  </schema>
  <schema id="org.gnome.shotwell.imports" path="/org/gnome/shotwell/imports/">
    <key name="strip-metadata" type="b"><default>false</default></key>
  </schema>
</schemalist>
`,
		// Malformed, and somebody else's to repair. A parse error in a file
		// Zenith does not ship is not a Zenith finding.
		"org.example.broken.gschema.xml": `<?xml version="1.0"?><schemalist><schema`,
		// Well-formed and declaring nothing at all.
		"org.example.empty.gschema.xml": `<?xml version="1.0"?><schemalist/>`,
		// An ordinary schema whose key no Zenith surface reads, which describes
		// every schema the desktop has and is not a fault in any of them.
		"org.gnome.desktop.interface.gschema.xml": `<?xml version="1.0"?>
<schemalist>
  <schema id="org.gnome.desktop.interface" path="/org/gnome/desktop/interface/">
    <key name="color-scheme" type="s"><default>'prefer-light'</default></key>
  </schema>
</schemalist>
`,
	}
	for name, body := range thirdParty {
		if err := os.WriteFile(filepath.Join(schemaDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	env := Env{Root: root, Dir: "/usr/share/zenith/shell", EntryPoint: "shell.qml",
		OwnsSettings: true, SchemaDir: "/usr/share/glib-2.0/schemas"}

	findings := Check(env)
	if len(findings) != 0 {
		for _, finding := range findings {
			t.Errorf("%s %s: %s", finding.Rule, finding.Path, finding.Detail)
		}
		t.Fatal("the base layer's schemas were reported as Zenith's own findings")
	}
}

// TestZenithSchemaPrefixAgreesWithSettings keeps the shell's source check and
// the settings catalogue filtering on the same prefix.
//
// They are two readers of one shared directory with one owner: `zenith config`
// and the Hub's pages come from the catalogue, and what doctor checks for
// reachability comes from here. A prefix that changed in one place and not the
// other splits them silently — settings the catalogue lists and the shell never
// checks, or the reverse — and neither tool's output would show the split.
func TestZenithSchemaPrefixAgreesWithSettings(t *testing.T) {
	if zenithSchemaPrefix != settings.ZenithSchemaPrefix {
		t.Errorf("the shell filter is %q and the settings catalogue is %q; "+
			"the two readers of the schema directory must agree on which "+
			"schemas are Zenith's", zenithSchemaPrefix,
			settings.ZenithSchemaPrefix)
	}
}

// TestPropertyAssignmentCheck is the rule that exists because Palette.apply
// threw on its first line and nothing said so.
//
// Every colour the generator wrote was assigned onto a property declared
// readonly, so the assignment threw, the function stopped, and `generated` — the
// flag saying a palette was loaded — never became true. A generator, a file, a
// loader and a comment saying the shell reads it, and a running shell that never
// did. The second half is naming a property that does not exist: `text_dim`
// against `textDim` fails the same quiet way, in a runtime nobody watches.
func TestPropertyAssignmentCheck(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  int
		rule  string
	}{
		{
			name: "assigning to a readonly property",
			files: map[string]string{
				"shell.qml": "import Quickshell\nSingleton {\n" +
					"id: palette\n" +
					"readonly property string background: \"#15171a\"\n" +
					"function apply(doc) { palette.background = doc.background }\n" +
					"}\n",
			},
			want: 1, rule: "readonly assignment",
		},
		{
			name: "assigning to a readonly property without the object",
			files: map[string]string{
				"shell.qml": "import Quickshell\nSingleton {\n" +
					"id: palette\n" +
					"readonly property bool generated: false\n" +
					"function apply(doc) { generated = true }\n" +
					"}\n",
			},
			want: 1, rule: "readonly assignment",
		},
		{
			name: "assigning to a property that does not exist",
			files: map[string]string{
				"shell.qml": "import Quickshell\nSingleton {\n" +
					"id: palette\n" +
					"property string textDim: \"#9aa0a6\"\n" +
					"function apply(doc) { palette.text_dim = doc.text_dim }\n" +
					"}\n",
			},
			want: 1, rule: "property assignment",
		},
		{
			name: "assigning to a property another file declares",
			files: map[string]string{
				"Popover.qml": "import QtQuick\nRectangle {\n" +
					"property bool opened: false\n}\n",
				"shell.qml": "import Quickshell\nShellRoot {\n" +
					"Popover { id: popover }\n" +
					"function toggle() { popover.opened = true }\n}\n",
			},
			want: 0,
		},
		{
			// A property declared inside a nested object belongs to that
			// object, and the rule used to read it as the outer object's: a
			// write to the window's own `currentPage` looked like a write
			// to a readonly property, because a page view inside the window
			// declared a readonly property of the same name.
			name: "a property declared in a nested object",
			files: map[string]string{
				"shell.qml": "import Quickshell\nSingleton {\n" +
					"id: window\n" +
					"property int currentPage: 0\n" +
					"PageView {\n" +
					"  id: page\n" +
					"  readonly property var currentPage: window.currentPage\n" +
					"}\n" +
					"function go(index) { window.currentPage = index }\n" +
					"}\n",
			},
			want: 0,
		},
		{
			name: "assigning to a writable property of this file",
			files: map[string]string{
				"shell.qml": "import Quickshell\nSingleton {\n" +
					"id: palette\n" +
					"property string background: \"#15171a\"\n" +
					"function apply(doc) { palette.background = doc.background }\n" +
					"}\n",
			},
			want: 0,
		},
		{
			// A name bound inside a function is not a write to the property
			// that happens to have the same name, and the rule used to
			// report it: `const items = control.items` was read as an
			// assignment to a readonly property, and the reader was sent
			// looking for a write that was not there.
			name: "declaring a local that shares a name with a readonly property",
			files: map[string]string{
				"shell.qml": "import Quickshell\nSingleton {\n" +
					"id: palette\n" +
					"readonly property var items: []\n" +
					"function apply(doc) { const items = doc.items }\n" +
					"}\n",
			},
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := treeEnv(t, tc.files)
			// treeEnv points at shell/ and the fixtures write their own files
			// beside it, which is what Check hands this rule.
			findings := checkPropertyAssignments(env, keys(tc.files))
			if len(findings) != tc.want {
				for _, finding := range findings {
					t.Logf("  %s %s: %s", finding.Rule, finding.Path, finding.Detail)
				}
				t.Fatalf("%d finding(s), want %d", len(findings), tc.want)
			}
			if tc.want > 0 && findings[0].Rule != tc.rule {
				t.Errorf("rule = %q, want %q", findings[0].Rule, tc.rule)
			}
		})
	}
}

// TestShellTreeHasNoPropertyAssignments is the rule against the real shell: the
// palette loader included. A check that only ever runs on fixtures finds the
// mistake in the example and not in the file the desktop draws from.
func TestShellTreeHasNoPropertyAssignments(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.ReadDir(filepath.Join(root, "shell"))
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, entry := range dir {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".qml") {
			files = append(files, entry.Name())
		}
	}
	if len(files) == 0 {
		t.Fatal("no QML files beside the shell")
	}

	env := Env{Root: root, Dir: "shell", EntryPoint: "shell.qml", OwnsSettings: true}
	if _, err := source(env, "Theme.qml"); err != nil {
		t.Fatalf("the check would silently read nothing: %v", err)
	}
	for _, finding := range checkPropertyAssignments(env, files) {
		t.Errorf("%s %s: %s", finding.Rule, finding.Path, finding.Detail)
	}
}
