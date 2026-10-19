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
	"strconv"
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
	// EntryPoint is the file `quickshell -p` is expected to find. It differs per
	// window: the desktop shell's is shell.qml, the Hub's is hub.qml. There is no
	// default, because a default would mean one window silently passes another's
	// check.
	EntryPoint string
	// OwnsSettings says this tree is the one that reads GSettings. The desktop
	// shell is; the Hub window is not — it renders what `zenith config export`
	// hands it and never touches a schema.
	//
	// It matters because Quickshell's `-p` sets the import path for one window
	// only. The Hub has its own copy of the settings wrapper installed beside it,
	// so running the shell's settings checks over the Hub's tree would report
	// every property that only the shell uses as an unused setting — a true
	// statement about the wrong tree, which is the most expensive kind.
	OwnsSettings bool
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
			Path:   filepath.Join(env.Dir, env.EntryPoint),
			Detail: "no QML files found; the window would start and immediately exit",
		}}
	}

	findings = append(findings, checkEntryPoint(env, files)...)

	if env.OwnsSettings {
		schema, schemaFindings := loadSchema(env)
		findings = append(findings, schemaFindings...)
		findings = append(findings, checkSchemaIds(env, files)...)
		findings = append(findings, checkCatalogue(env, files)...)
		if schema != nil {
			findings = append(findings, checkSettingsReads(env, files, *schema)...)
		}
		findings = append(findings, checkUnusedWrapperProperties(env, files)...)
		findings = append(findings, checkGlyphs(env, files)...)
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
	var findings []Finding

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
		if err != nil {
			// Reported as itself rather than as a missing id. A schema that will
			// not parse is the problem; "no schema declares it" sends the reader
			// looking for a schema that is sitting right there, malformed.
			findings = append(findings, Finding{
				Rule: "schema", Path: filepath.Join(env.Dir, entry.Name()),
				Detail: err.Error(),
			})
			continue
		}
		if schema.ID == "" {
			findings = append(findings, Finding{
				Rule: "schema", Path: filepath.Join(env.Dir, entry.Name()),
				Detail: "declares no schema id",
			})
			continue
		}
		declared[schema.ID] = true
	}

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
		if f == env.EntryPoint {
			return nil
		}
	}
	return []Finding{{
		Rule: "entry point", Path: filepath.Join(env.Dir, env.EntryPoint),
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

// wrapperExempt are properties Config.qml owns for itself rather than as a
// bridge to a setting: the two Settings objects, the locker path, and the
// colour scheme the dark flag is derived from.
//
// They are exempt because the "is it read anywhere else" question has the wrong
// shape for them. A Settings object exists to be used, so finding no use site
// for it is not evidence of anything, and colorScheme is read here to compute
// dark rather than by a surface.
//
// An exemption is only for a property of that shape. An ordinary setting added
// here to stop the unused check complaining is the check being defeated rather
// than satisfied, which is why TestWrapperExemptionsStaySmall exists.
var wrapperExempt = map[string]bool{
	"shell":       true,
	"desktop":     true,
	"locker":      true,
	"colorScheme": true,
}

// checkUnusedWrapperProperties asserts that every property Config.qml re-exports
// is read somewhere else.
//
// This is the check that catches a setting that appears to work and does
// nothing. Config.qml is the bridge between GSettings and the shell, so a key
// there with no use site is a key the Hub writes and the shell never looks at:
// both are working, and they disagree, and nothing says so. `zenith doctor`
// reporting "the idle timeout is 0" is not useful if nothing reads the idle
// timeout.
//
// It is also the check that catches an over-built wrapper. A property added for
// a surface that was never written is dead weight that reads as an unfinished
// feature, which is exactly what it is.
func checkUnusedWrapperProperties(env Env, files []string) []Finding {
	declared := wrapperProperties(env, files)
	if len(declared) == 0 {
		return nil
	}

	// Everything outside Config.qml. A property's declaration is not a use of
	// it — counting the declaration would make every property "read".
	used := map[string]bool{}
	for _, f := range files {
		if filepath.Base(f) == "Config.qml" {
			continue
		}
		body, err := source(env, f)
		if err != nil {
			continue
		}
		for _, match := range regexp.MustCompile(
			`\bconfig\.([A-Za-z_][A-Za-z0-9_]*)`,
		).FindAllStringSubmatch(body, -1) {
			used[match[1]] = true
		}
	}

	var findings []Finding
	for name := range declared {
		if used[name] || wrapperExempt[name] {
			continue
		}
		findings = append(findings, Finding{
			Rule: "unused setting", Path: filepath.Join(env.Dir, "Config.qml"),
			Detail: fmt.Sprintf("config.%s is declared and read by nothing "+
				"outside Config.qml; a setting nothing reads is a setting "+
				"that does nothing", name),
		})
	}
	return findings
}

// privateUseGlyph matches a codepoint in the Unicode private use area, which is
// where every icon font puts its glyphs.
//
// Only private use, because that is the range an icon font has to use — there is
// nowhere else for several thousand icons to go. A codepoint outside it that
// looks like an icon is an ordinary character that happens to render as a box in
// the chosen font.
var privateUseGlyph = regexp.MustCompile(`[\x{E000}-\x{F8FF}\x{F0000}-\x{FFFFD}\x{100000}-\x{10FFFD}]`)

// declaredGlyph matches an entry in the icon map: "name": "\uf293",
var declaredGlyph = regexp.MustCompile(`"([A-Za-z0-9-]+)"\s*:\s*"(\S+?)"`)

// escapedGlyph matches a glyph written as a \uXXXX escape.
var escapedGlyph = regexp.MustCompile(`^\\u([0-9A-Fa-f]{4,6})$`)

// rawEscape is a glyph written as raw bytes rather than as an escape.
var rawEscape = regexp.MustCompile(`[\x{E000}-\x{F8FF}]`)

// iconFamilyRanges is the private use range each icon family actually draws in.
//
// This is the rule that catches a plausible-looking codepoint from memory. An
// earlier version of Icons.qml used U+F8FF for bluetooth and U+F8C0 for battery —
// both real private use codepoints, both in the same neighbourhood as the
// correct ones, and neither of them a Font Awesome 4.7 glyph. Nothing rendered,
// so nothing failed, and the status area would have drawn boxes forever.
//
// The range is the font's, not a guess: Font Awesome 4.7 packs its glyphs into
// F000-F2E0, and Powerline uses E0A0-E0D4. A codepoint outside its family's range
// is not in the font, whatever it looks like.
var iconFamilyRanges = map[string]struct{ low, high rune }{
	"Font Awesome 4.7": {'\uf000', '\uf2e0'},
	"PowerlineSymbols": {'\ue0a0', '\ue0d4'},
}

// glyphMapFiles is the one place a glyph may be declared.
const glyphMapFile = "Icons.qml"

// checkGlyphs asserts that every private-use glyph in the tree is declared in the
// icon map, and that nothing outside the map names one.
//
// A codepoint a font does not contain renders as a hollow box. There is no
// error, nothing in the log, and a status area that looks decorative rather than
// broken — and a status area is the one place on a desktop where a wrong glyph
// is a wrong statement about the machine's state. Volume-off drawn as volume-high
// is not a cosmetic bug.
//
// The map is also where the family each glyph comes from is recorded, so a
// reader can check a codepoint against the font rather than against the
// author's memory.
func checkGlyphs(env Env, files []string) []Finding {
	declared := map[string]string{}
	var findings []Finding

	for _, f := range files {
		if filepath.Base(f) != glyphMapFile {
			continue
		}
		body, err := source(env, f)
		if err != nil {
			continue
		}
		for _, match := range declaredGlyph.FindAllStringSubmatch(body, -1) {
			if privateUseGlyph.MatchString(match[2]) {
				declared[match[2]] = match[1]
			}
		}
		findings = append(findings, checkGlyphFamilies(body)...)
	}

	for _, f := range files {
		// The map declares them, so scanning it would report every one of them.
		if filepath.Base(f) == glyphMapFile {
			continue
		}
		body, err := source(env, f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(body, "\n") {
			for _, glyph := range privateUseGlyph.FindAllString(line, -1) {
				if _, ok := declared[glyph]; ok {
					continue
				}
				findings = append(findings, Finding{
					Rule: "glyph", Path: filepath.Join(env.Dir, f),
					Detail: fmt.Sprintf("uses %q, which %s does not declare; "+
						"a glyph the font does not have renders as a box, with "+
						"no error and nothing in the log", glyph, glyphMapFile),
				})
			}
		}
	}
	return findings
}

// checkGlyphFamilies asserts that every glyph in the map is written as a
// \uXXXX escape and sits in its family's range.
//
// Two rules, both from the same mistake. A glyph written as raw bytes is
// unreadable in a diff, survives a copy or a rebase only if every step preserves
// the encoding, and is indistinguishable from an empty string when you look at
// the file. And a glyph written from memory in the right neighbourhood but the
// wrong font's range is a box that renders silently forever.
func checkGlyphFamilies(body string) []Finding {
	var findings []Finding

	// Which map a line belongs to, so a Powerline separator is not checked
	// against Font Awesome's range.
	family := ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "separators") {
			family = "PowerlineSymbols"
		} else if strings.Contains(trimmed, "glyphs") {
			family = "Font Awesome 4.7"
		}

		match := declaredGlyph.FindStringSubmatch(trimmed)
		if match == nil {
			continue
		}
		name, value := match[1], match[2]

		if rawEscape.MatchString(value) {
			findings = append(findings, Finding{
				Rule: "glyph escape", Path: filepath.Join(glyphMapFile),
				Detail: fmt.Sprintf("%q is written as a raw character; write "+
					"it as a \\uXXXX escape so it is readable, diffable and "+
					"checkable against the font", name),
			})
			continue
		}

		escape := escapedGlyph.FindStringSubmatch(value)
		if escape == nil {
			continue
		}
		codepoint, err := strconv.ParseInt(escape[1], 16, 32)
		if err != nil {
			continue
		}

		limits, known := iconFamilyRanges[family]
		if !known {
			continue
		}
		if rune(codepoint) < limits.low || rune(codepoint) > limits.high {
			findings = append(findings, Finding{
				Rule: "glyph range", Path: filepath.Join(glyphMapFile),
				Detail: fmt.Sprintf("%q is U+%04X, outside %s's range "+
					"U+%04X-U+%04X; a codepoint the font does not contain "+
					"renders as a box, with no error and nothing in the log",
					name, codepoint, family, limits.low, limits.high),
			})
		}
	}
	return findings
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

// boundaryAllowance is the short list of absolute paths the shell legitimately
// reaches for outside its own tree, and each one is a binary it runs rather than
// a file it reads.
//
// It is an explicit list rather than a rule, because "the shell may exec
// anything under /usr/bin" is the same as no rule at all: the shell would then
// be able to depend on a tool any user installed, and the base image would stop
// being the thing that decides what the desktop is made of. Naming each one
// makes the dependency reviewable and makes adding one a decision.
var boundaryAllowance = map[string]bool{
	// The locker. The shell owns *when* to lock and this does the locking,
	// because it authenticates against PAM and a QML lock screen cannot.
	"/usr/bin/hyprlock": true,
}

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
