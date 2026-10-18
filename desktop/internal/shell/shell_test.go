package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	findings := Check(Env{Root: "../../..", Dir: "shell"})
	if len(findings) != 0 {
		for _, f := range findings {
			t.Errorf("%s", f)
		}
	}
}

func TestGoodFixturePasses(t *testing.T) {
	if findings := Check(tree(t, goodShell())); len(findings) != 0 {
		t.Errorf("a well-formed shell reported findings: %v", rules(findings))
	}
}

// TestMissingEntryPointIsFound covers the fault with the least visible symptom:
// `quickshell -p` names the directory, so a tree without shell.qml starts and
// exits, and the journal says nothing that identifies the cause.
func TestMissingEntryPointIsFound(t *testing.T) {
	files := goodShell()
	delete(files, "shell.qml")

	findings := Check(tree(t, files))
	if !hasRule(findings, "entry point") {
		t.Errorf("a missing entry point was not found: %v", rules(findings))
	}
}

// TestEmptyTreeIsReported covers a tree that exists but has nothing in it.
func TestEmptyTreeIsReported(t *testing.T) {
	findings := Check(tree(t, map[string]string{}))
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

	findings := Check(tree(t, files))
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

	findings := Check(tree(t, files))
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

	findings := Check(tree(t, files))
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

	findings := Check(tree(t, files))
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

	findings := Check(tree(t, files))
	if !hasRule(findings, "boundary") {
		t.Errorf("a path outside the boundary was not found: %v", rules(findings))
	}
}

func TestPathsInsideTheBoundaryAreAllowed(t *testing.T) {
	files := goodShell()
	files["Widgets/Clock.qml"] = "import Quickshell\n" +
		"Item { source: \"/usr/share/zenith/shell/shell.qml\" }\n"

	findings := Check(tree(t, files))
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

	findings := Check(tree(t, files))
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

	findings := Check(tree(t, files))
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

	findings := Check(tree(t, files))
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

	findings := Check(tree(t, files))
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

	findings := Check(tree(t, files))
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

	findings := Check(tree(t, files))
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
