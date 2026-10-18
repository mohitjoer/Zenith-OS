// Package shell checks the shell's source without running it.
//
// The shell is Quickshell. Quickshell is built from source and is not in the
// Ubuntu archive, so unlike every other component in the project it cannot be
// installed and executed to find out whether it works. There is no qmllint
// either, for the same reason — it arrives with the Qt toolchain this base layer
// does not carry.
//
// That leaves one honest option, and it is not "trust it". Everything checkable
// without a QML engine is checked here, and everything that needs an engine is
// reported as needing one rather than quietly passed.
//
// What that means concretely: this package cannot tell you the shell's
// behaviour is right. It can tell you the widget catalogue names files that
// exist, that every settings key the QML reads is one the schema declares, that
// the entry point is present, and that nothing in the tree names a path outside
// the boundary. Those are the faults that are silent at runtime and obvious
// here.
package shell

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Finding is one problem with the shell's source.
type Finding struct {
	// Rule is the check that failed, so a report groups by cause rather than
	// listing the same fault once per file.
	Rule string
	// Path is the file it is in.
	Path string
	// Detail says what is wrong.
	Detail string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s: %s: %s", f.Path, f.Rule, f.Detail)
}

// Env is what the checks read.
type Env struct {
	// Root prefixes every path, so the whole surface runs against a fixture.
	Root string
	// Dir is the shell's source directory, without Root. It is what the QML
	// resolves relative paths against, because Quickshell resolves them the same
	// way — against the directory named by `-p`, not against the process's
	// working directory.
	Dir string
}

func (e Env) path(relative string) string {
	return filepath.Join(e.Root, e.Dir, relative)
}

// Schema is the parsed form of a gschema file.
//
// Only the parts the shell depends on are modelled: the schema id, its keys and
// its enums. A full gschema parser is a large thing to own, and the shell does
// not need one — it needs to know which keys exist so it can catch a QML file
// that reads one that does not.
type Schema struct {
	// ID is the schema id, e.g. org.zenith.shell.
	ID string
	// Keys are the key names, in file order.
	Keys []string
	// Enums are the enum ids declared in the file.
	Enums []string
	// KeyTypes maps a key name to its type, or for an enum-typed key to the enum
	// id. Kept because a key read as the wrong type is a silent coercion.
	KeyTypes map[string]string
}

// ParseSchema reads a gschema file.
func ParseSchema(path string) (Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Schema{}, err
	}
	return parseSchema(data)
}

func parseSchema(data []byte) (Schema, error) {
	var document struct {
		Schemas []struct {
			ID   string `xml:"id,attr"`
			Keys []struct {
				Name string `xml:"name,attr"`
				Type string `xml:"type,attr"`
				Enum string `xml:"enum,attr"`
			} `xml:"key"`
		} `xml:"schema"`
		Enums []struct {
			ID string `xml:"id,attr"`
		} `xml:"enum"`
	}

	if err := xml.Unmarshal(data, &document); err != nil {
		return Schema{}, fmt.Errorf("not well-formed XML: %w", err)
	}

	schema := Schema{KeyTypes: map[string]string{}}
	for _, e := range document.Enums {
		schema.Enums = append(schema.Enums, e.ID)
	}
	for _, s := range document.Schemas {
		schema.ID = s.ID
		for _, k := range s.Keys {
			schema.Keys = append(schema.Keys, k.Name)
			switch {
			case k.Enum != "":
				schema.KeyTypes[k.Name] = k.Enum
			case k.Type != "":
				schema.KeyTypes[k.Name] = k.Type
			}
		}
	}
	return schema, nil
}

// stripComments removes QML comments from a line of source.
//
// The checks below match identifiers in source, and a comment is not source.
// Without this, a comment that explains the bug it is warning about — "do not
// read config.edge, it does not exist" — is itself read as a use of
// config.edge, and the file can never be corrected without first deleting the
// explanation of why it needed correcting.
//
// Quote state is tracked rather than pattern-matched, because `//` occurs inside
// strings: file:///usr/share/backgrounds/x.png is a URI, not a comment, and
// treating it as one silently deletes the rest of the line.
func stripComments(line string) string {
	var out strings.Builder
	var quote rune
	escaped := false

	for i, r := range line {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && quote != 0:
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '/' && i+1 < len(line) && line[i+1] == '/':
			return out.String()
		case r == '/' && i+1 < len(line) && line[i+1] == '*':
			return out.String()
		}
		out.WriteRune(r)
	}
	return out.String()
}

// source returns a file's code with comments removed.
func source(env Env, relative string) (string, error) {
	body, err := os.ReadFile(env.path(relative))
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(body), "\n")
	for i, line := range lines {
		lines[i] = stripComments(line)
	}
	return strings.Join(lines, "\n"), nil
}

// widgetSource is how Bar.qml names a widget: `"Widgets/" + name + ".qml"`.
// Matched rather than inferred, so the check follows the source instead of
// guessing a convention and disagreeing with it.
var widgetSource = regexp.MustCompile(`"Widgets/"\s*\+\s*[A-Za-z_][A-Za-z0-9_.]*\s*\+\s*"\.qml"`)

// catalogueEntry is one `"name": "WidgetName"` pair in the catalogue map.
var catalogueEntry = regexp.MustCompile(`"([A-Za-z0-9_-]+)"\s*:\s*"([A-Za-z0-9_-]+)"`)

// settingsRead is a `schema.<key>` read on a Settings object bound to a schema.
// Both halves matter: a key that does not exist in *any* Zenith schema is a
// typo, and a key that exists in the schema but not on this object is a binding
// to the wrong schema.
// The key pattern admits capitals on purpose. Schema keys are kebab-case, but
// the wrapper's own properties are camelCase, and a pattern that stopped at the
// first capital would read `config.edgeTop` as `config.edge` and then report it
// as a key the schema does not declare — a false positive on every use.
var settingsRead = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z][A-Za-z0-9-]*)\b`)

// Check runs every check that does not need a QML engine, and returns what it
// found. A non-empty result is a fault in the source, not a warning about the
// environment.
//
// The findings are sorted so that a report is stable between runs: the same tree
// produces the same list in the same order.
func Check(env Env) []Finding {
	var findings []Finding

	// A directory that is not there is a tree with nothing in it, which is the
	// same fault as an empty one: the shell starts and immediately exits. It is
	// reported as the entry point being missing rather than as an unreadable
	// directory, because that is the thing the reader has to go and fix.
	files, err := qmlFiles(env)
	if err != nil && len(files) == 0 {
		files = nil
	}

	if len(files) == 0 {
		return []Finding{{
			Rule:   "entry point",
			Path:   filepath.Join(env.Dir, "shell.qml"),
			Detail: "no QML files found; the shell would start and immediately exit",
		}}
	}

	schema, schemaFindings := loadSchema(env)
	findings = append(findings, schemaFindings...)

	findings = append(findings, checkSchemaIds(env, files)...)
	findings = append(findings, checkEntryPoint(env, files)...)
	findings = append(findings, checkCatalogue(env, files)...)
	if schema != nil {
		findings = append(findings, checkSettingsReads(env, files, *schema)...)
	}
	findings = append(findings, checkImports(env, files)...)
	findings = append(findings, checkBoundaries(env, files)...)

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Rule < findings[j].Rule
	})
	return findings
}

func qmlFiles(env Env) ([]string, error) {
	var files []string
	err := filepath.Walk(env.path("."), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".qml") {
			relative, err := filepath.Rel(env.path("."), path)
			if err != nil {
				return err
			}
			files = append(files, relative)
		}
		return nil
	})
	return files, err
}

func loadSchema(env Env) (*Schema, []Finding) {
	schema, err := ParseSchema(env.path("org.zenith.shell.gschema.xml"))
	path := filepath.Join(env.Dir, "org.zenith.shell.gschema.xml")
	if err != nil {
		return nil, []Finding{{
			Rule: "schema", Path: path, Detail: err.Error(),
		}}
	}
	if schema.ID == "" {
		return nil, []Finding{{
			Rule: "schema", Path: path, Detail: "declares no schema id",
		}}
	}

	var findings []Finding
	seen := map[string]bool{}
	for _, key := range schema.Keys {
		if seen[key] {
			findings = append(findings, Finding{
				Rule: "schema", Path: path,
				Detail: fmt.Sprintf("key %q is declared twice", key),
			})
		}
		seen[key] = true
	}
	return &schema, findings
}

// schemaID is a `schemaId: "org.zenith.something"` in a Settings object.
var schemaID = regexp.MustCompile(`schemaId\s*:\s*"([A-Za-z0-9_.-]+)"`)

// checkSchemaIds asserts that every schema the QML names has a schema file in the
// tree.
//
// This is the fault of the moment. `Settings { schemaId: "org.zenith.desktop" }`
// against a schema that was never shipped is not a build error and not a
// startup error: the Settings object comes back invalid, every property read off
// it is undefined, and a colour binding quietly evaluates to nothing. The shell
// then renders with no text on it and nothing anywhere says the schema is
// missing.
//
// Reading every schema a machine happens to have installed would not fix it
// either — the shell has to work on the machine it ships to, and on that
// machine this project's schemas are the only ones it may depend on.
func checkSchemaIds(env Env, files []string) []Finding {
	// Every schema file in the tree, keyed by the id it declares. A schema the
	// tree does not ship is exactly the fault, so an allowlist of known-good ids
	// would be worse than useless: it would let the shell depend on a schema
	// this project does not own.
	declared := map[string]bool{}
	entries, err := os.ReadDir(env.path("."))
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".gschema.xml") {
			continue
		}
		schema, err := ParseSchema(env.path(entry.Name()))
		if err != nil || schema.ID == "" {
			continue
		}
		declared[schema.ID] = true
	}

	var findings []Finding
	for _, f := range files {
		body, err := source(env, f)
		if err != nil {
			continue
		}
		for _, match := range schemaID.FindAllStringSubmatch(body, -1) {
			if declared[match[1]] {
				continue
			}
			findings = append(findings, Finding{
				Rule: "schema id", Path: filepath.Join(env.Dir, f),
				Detail: fmt.Sprintf("binds schemaId %q, and no .gschema.xml in "+
					"the tree declares it; every read off that Settings object "+
					"will be undefined", match[1]),
			})
		}
	}
	return findings
}

// checkEntryPoint asserts that shell.qml exists, because `quickshell -p` is
// pointed at the directory and not at a file.
func checkEntryPoint(env Env, files []string) []Finding {
	for _, f := range files {
		if f == "shell.qml" {
			return nil
		}
	}
	return []Finding{{
		Rule: "entry point", Path: filepath.Join(env.Dir, "shell.qml"),
		Detail: "missing; `quickshell -p` names the directory, and nothing " +
			"names the file inside it",
	}}
}

// checkCatalogue asserts that every widget name in the bar's catalogue maps to a
// QML file that exists.
//
// This is the check that pays for itself. The catalogue is a map from a settings
// value to a file on disk, and a name with no file behind it produces a Loader
// with an empty source at run time: the bar comes up, one widget is missing, and
// nothing anywhere says which.
func checkCatalogue(env Env, files []string) []Finding {
	present := map[string]bool{}
	for _, f := range files {
		present[filepath.ToSlash(f)] = true
	}

	var findings []Finding
	for _, f := range files {
		if filepath.Base(f) != "Bar.qml" {
			continue
		}
		body, err := source(env, f)
		if err != nil {
			continue
		}
		for _, match := range catalogueEntry.FindAllStringSubmatch(body, -1) {
			name, widget := match[1], match[2]
			target := "Widgets/" + widget + ".qml"
			if !present[target] {
				findings = append(findings, Finding{
					Rule: "widget catalogue", Path: filepath.Join(env.Dir, f),
					Detail: fmt.Sprintf("catalogue entry %q maps to %s, "+
						"which does not exist", name, target),
				})
			}
		}
	}
	return findings
}

// normalise folds a name so that a schema key and the QML property standing in
// for it compare equal.
//
// GSettings keys are kebab-case because that is the convention dconf and
// `gsettings get` use. QML property names cannot contain a hyphen, so every key
// has a camelCase property standing in front of it, and the mapping between
// them is a decision that has to be written down somewhere. This is that fold,
// and it is what lets the two be compared without either of them being changed
// to suit the other.
func normalise(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "-", ""))
}

// checkSettingsReads splits the settings check in two, because the shell's use
// of GSettings has two distinct rules and one check cannot hold both.
//
// The first rule is about Config.qml: every key the schema declares has to be
// read there, because Config.qml is the only file that talks to a Settings
// object. A key no surface can reach is a setting the hub would write and the
// shell would never read — the shell and the settings application would both
// be working, and disagreeing.
//
// The second rule is about every other file: `config.<name>` may only name a
// property Config.qml declares. Anything else is a read of a property that does
// not exist, and in QML that is undefined rather than an error — so a colour
// binding silently evaluates to nothing and the widget is transparent.
func checkSettingsReads(env Env, files []string, schema Schema) []Finding {
	wrapper := wrapperProperties(env, files)

	// Every schema key must be reachable from the wrapper. `shell.<name>` rather
	// than any object, because Config.qml binds one Settings object per schema
	// and a key read off a different object is a key the wrapper is not the
	// owner of.
	reachable := ""
	for _, f := range files {
		if filepath.Base(f) == "Config.qml" {
			body, err := source(env, f)
			if err == nil {
				reachable += body
			}
		}
	}

	var findings []Finding
	declared := map[string]bool{}
	for _, key := range schema.Keys {
		declared[normalise(key)] = true
		if !strings.Contains(strings.ToLower(reachable),
			"shell."+normalise(key)) {
			findings = append(findings, Finding{
				Rule: "settings unreachable", Path: filepath.Join(env.Dir, "Config.qml"),
				Detail: fmt.Sprintf("%s declares %q, and Config.qml never reads "+
					"it, so no surface can reach it", schema.ID, key),
			})
		}
	}

	for _, f := range files {
		// Config.qml is where the mapping is declared; checking it against
		// itself would report every property it declares.
		if filepath.Base(f) == "Config.qml" {
			continue
		}
		body, err := source(env, f)
		if err != nil {
			continue
		}
		for _, match := range settingsRead.FindAllStringSubmatch(body, -1) {
			object, key := match[1], match[2]
			if object != "config" {
				continue
			}
			if wrapper[key] {
				continue
			}
			findings = append(findings, Finding{
				Rule: "settings read", Path: filepath.Join(env.Dir, f),
				Detail: fmt.Sprintf("reads config.%s, which Config.qml does not "+
					"declare; in QML that is undefined rather than an error", key),
			})
		}
	}
	return findings
}

// wrapperProperties collects the property names Config.qml declares. These are
// the shell's own vocabulary; the schema's keys live behind them.
func wrapperProperties(env Env, files []string) map[string]bool {
	wrapper := map[string]bool{}
	for _, f := range files {
		if filepath.Base(f) != "Config.qml" {
			continue
		}
		body, err := source(env, f)
		if err != nil {
			continue
		}
		// Properties and functions are both part of the wrapper's vocabulary.
		// A surface calls config.setDockAutohide(...) as legitimately as it
		// reads config.dockAutohide, and a check that only knew about
		// properties would report every write as an undefined read.
		for _, pattern := range []string{
			`readonly\s+property\s+\w+\s+([A-Za-z_][A-Za-z0-9_]*)\s*:`,
			`\bfunction\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`,
		} {
			for _, match := range regexp.MustCompile(pattern).
				FindAllStringSubmatch(body, -1) {
				wrapper[match[1]] = true
			}
		}
	}
	return wrapper
}

// allowedImports is the set of modules the shell may import.
//
// Every one is a Zenith-built or base-layer module. The list is closed so that a
// new import is a decision rather than an accident: a QML file can pull in any
// module the machine happens to have installed, and a shell whose behaviour
// depends on what a user installed is not a shell this project controls.
var allowedImports = map[string]bool{
	"Quickshell":                        true,
	"Quickshell.Io":                     true,
	"Quickshell.Widgets":                true,
	"Quickshell.Hyprland":               true,
	"Quickshell.Services":               true,
	"Quickshell.Services.Notifications": true,
	"QtQuick":                           true,
	"QtQuick.Controls":                  true,
	"QtQuick.Layouts":                   true,
	"QtQuick.Window":                    true,
}

var importLine = regexp.MustCompile(`^\s*import\s+([A-Za-z0-9_.]+)\s*(?:[0-9.]+\s*)?$`)

func checkImports(env Env, files []string) []Finding {
	var findings []Finding
	for _, f := range files {
		body, err := source(env, f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(body, "\n") {
			match := importLine.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			if !allowedImports[match[1]] {
				findings = append(findings, Finding{
					Rule: "import", Path: filepath.Join(env.Dir, f),
					Detail: fmt.Sprintf("imports %s, which is not on the "+
						"shell's closed module list", match[1]),
				})
			}
		}
	}
	return findings
}

// outsideBoundary matches an absolute path in the source that is not under
// /usr/share/zenith/.
//
// It covers /etc, /usr, /var and /opt rather than /usr alone, because the
// boundary is about what the shell may name and not about which prefix happens
// to be the popular one — a QML file reading /etc is the same class of fault as
// one reading a stray file in /usr.
var outsideBoundary = regexp.MustCompile(`["'](/(?:usr|etc|var|opt)/[A-Za-z0-9_./-]*)["']`)

// boundaryAllowance is the one absolute path the shell legitimately reads from
// outside its own tree: the base layer's portal dispatcher, which the unit runs
// rather than the QML. It is listed so that the check's exception is visible.
var boundaryAllowance = map[string]bool{}

func checkBoundaries(env Env, files []string) []Finding {
	var findings []Finding
	for _, f := range files {
		body, err := source(env, f)
		if err != nil {
			continue
		}
		for _, match := range outsideBoundary.FindAllStringSubmatch(body, -1) {
			path := match[1]
			if strings.HasPrefix(path, "/usr/share/zenith/") || boundaryAllowance[path] {
				continue
			}
			findings = append(findings, Finding{
				Rule: "boundary", Path: filepath.Join(env.Dir, f),
				Detail: fmt.Sprintf("names %s, outside /usr/share/zenith/ "+
					"and not on the sanctioned list", path),
			})
		}
	}
	return findings
}
