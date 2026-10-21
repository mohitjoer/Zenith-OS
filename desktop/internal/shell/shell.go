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
//
// The shell's tree holds two kinds of file, and the difference runs through
// everything here.
//
// Zenith's own files are the shell root, the Hub's windows and the settings
// bridge. Every rule in this package is a rule about those: a theme that has
// to be a theme, a glyph that has to be in the font, an import that has to be
// on a closed list, a path that has to be inside the boundary.
//
// The vendored tree is upstream's: the component kit, the bar, the dock, the
// side panel and the services, vendored from a pinned commit at upstream's own
// paths. A vendored file is a shipped file, and the project's rule is that a
// shipped file is never edited in place — so a rule that would demand an edit
// of one is the wrong rule for it. The vendored tree is checked instead,
// against the manifest it was pinned from, byte for byte.
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
	// EntryPoint is the file `quickshell -p` is expected to find. Both windows
	// name `shell.qml`, because that is the name `quickshell -p` looks for in a
	// directory — the Hub was renamed to it rather than given an entry point
	// whose only job would have been to be a second name for the same file.
	// There is no default, because a default would mean one window silently
	// passes another's check.
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
	// SchemaDir is where the tree's .gschema.xml files live, without Root. It
	// is empty in the checkout — schemas sit beside the QML that reads them —
	// and is set only for an installed tree, where GLib installs schemas
	// nowhere near the shell: /usr/share/glib-2.0/schemas, not
	// /usr/share/zenith/shell.
	//
	// Without it doctor checks an installed shell against a directory that
	// holds no schemas, reports the schema missing, and then reports every
	// settings property unreachable — two faults that do not exist, on the one
	// machine the project is actually running on.
	SchemaDir string

	// HyprlandConfig is the compositor's configuration, relative to Root, when
	// the tree has one. It is where the keybinds live, and every bind that
	// drives the shell names a target the shell registers over IPC.
	//
	// The two are checked against each other because nothing in either file
	// checks the other. A rename on one side leaves a keybind that runs,
	// succeeds, and does nothing — the quietest failure in the desktop, and the
	// one a user reports as "the shortcut stopped working" rather than as a
	// bug.
	//
	// It is empty for the Hub, which is a window and not a session, and empty
	// wherever the compositor's configuration is not part of the tree being
	// checked.
	HyprlandConfig string

	// BindsCatalogue is the keybinding catalogue, relative to Root, when the
	// tree has one. It is the list of what the keyboard does — every chord,
	// with a label and a category — and the compositor's configuration is
	// checked against it.
	//
	// The two are the same list written down twice, once for a person and
	// once for the compositor, and a list written down twice is a list that
	// drifts. The catalogue is where a person looks to find out what the
	// keys are, so a chord that is bound but not listed is a key nobody can
	// find, and a chord that is listed but not bound is a key that does
	// nothing.
	//
	// It is empty for the Hub, and empty wherever the compositor's
	// configuration is not part of the tree being checked: a catalogue with
	// nothing to compare it against is not a fault, it is a window.
	BindsCatalogue string
}

func (e Env) path(relative string) string {
	return filepath.Join(e.Root, e.Dir, relative)
}

// schemaDir is where schema files are searched for: the schema directory when
// one is given, and the tree itself otherwise.
func (e Env) schemaDir() string {
	if e.SchemaDir == "" {
		return e.Dir
	}
	return e.SchemaDir
}

func (e Env) schemaPath(relative string) string {
	return filepath.Join(e.Root, e.schemaDir(), relative)
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
			Path:   env.path(env.EntryPoint),
			Detail: "no QML files found; the window would start and immediately exit",
		}}
	}

	// The shell's own files, and the vendored upstream tree.
	//
	// Every rule below is a rule about Zenith's own code: a theme that has to
	// be a theme, a glyph that has to be in the font, an import that has to be
	// on a closed list, a path that has to be inside the boundary. A vendored
	// file is upstream's — a shipped file that is never edited in place — so a
	// rule that would demand an edit of one is the wrong rule for it. The
	// vendored tree is checked instead, byte for byte, against the manifest.
	own, _ := partition(files)

	findings = append(findings, checkEntryPoint(env, own)...)

	if env.OwnsSettings {
		schemas, schemaFindings := loadSchemas(env)
		declared := map[string]bool{}
		for id := range schemas {
			declared[id] = true
		}
		findings = append(findings, schemaFindings...)
		findings = append(findings, checkSchemaIds(env, own, declared)...)
		findings = append(findings, checkCatalogue(env, own)...)
		findings = append(findings, checkSettingsReads(env, own, schemas)...)
		findings = append(findings, checkUnusedWrapperProperties(env, own)...)
		findings = append(findings, checkGlyphs(env, own)...)
		findings = append(findings, checkPropertyAssignments(env, own)...)
	}

	findings = append(findings, checkCommands(env, own)...)
	findings = append(findings, checkHardcodedColours(env, own)...)
	findings = append(findings, checkImports(env, own)...)
	findings = append(findings, checkBoundaries(env, own)...)
	findings = append(findings, checkIPCTargets(env, own)...)
	findings = append(findings, checkIPCFunctions(env, own)...)
	findings = append(findings, checkBinds(env)...)

	// The vendored tree, checked against the manifest it was pinned from.
	findings = append(findings, CheckVendored(env).Findings...)

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Rule < findings[j].Rule
	})
	return findings
}

// partition splits the tree's files into the ones Zenith owns and the ones
// the vendored upstream tree holds.
//
// The vendored tree is upstream's namespace inside the shell: the component
// kit, the bar, the dock, the side panel and the services, at the paths
// upstream keeps them at. Everything else — the shell root, the Hub's
// windows, the settings bridge — is Zenith's own code, and only Zenith's own
// code is subject to the shell's rules.
//
// The overlay files are the exception that is not an exception: they are
// Zenith's files that live inside the vendored namespace, at upstream's
// paths. They are excluded here for the same reason the rest of the
// namespace is: their shape is upstream's contract, because upstream's
// components read them by upstream's names.
func partition(files []string) (own, vendored []string) {
	for _, file := range files {
		if isVendored(file) {
			vendored = append(vendored, file)
			continue
		}
		own = append(own, file)
	}
	return own, vendored
}

// ipcTargetPattern finds an IPC target the shell registers.
var ipcTargetPattern = regexp.MustCompile(`target:\s*"([^"]+)"`)

// ipcCallPattern finds a call the compositor makes into the shell.
//
// The command is spelled `quickshell ipc --path <dir> call <target>
// <function>`, and the path between "ipc" and "call" is the reason the
// pattern does not look for "ipc call": the original one did, and it never
// matched the configuration the compositor actually reads, so the check it
// belonged to had never once fired on a real machine.
//
// The three groups are the target, the function and the function's argument.
// The target is who is being called and the function is what is being asked
// for, and they are different faults: a call to a target the shell does not
// register reaches nothing, and a call to a function the shell does not
// define reaches the shell and asks it for something it has not got.
var ipcCallPattern = regexp.MustCompile(
	`\bipc\b.*?\bcall\s+([A-Za-z0-9_.-]+)\s+([A-Za-z0-9_]+)(?:\s+([A-Za-z0-9_.-]+))?`)

// checkIPCTargets checks the compositor's keybinds against the shell's IPC
// surface.
//
// A keybind is a string in one file and the surface it names is a string in
// another, and nothing in either file checks the other. The failure is the
// quietest one in the desktop: the compositor runs the command, the command
// succeeds, and no launcher opens. A user reports that as "the shortcut
// stopped working", and nothing in the journal says why.
//
// So the two are checked against each other here.
func checkIPCTargets(env Env, files []string) []Finding {
	if env.HyprlandConfig == "" {
		return nil
	}

	registered := map[string]bool{}
	for _, file := range files {
		data, err := os.ReadFile(env.path(file))
		if err != nil {
			continue
		}
		text := string(data)
		// Only a file that declares a handler has targets. Quickshell's
		// Hyprland module has a `target` property of its own, and reading
		// that as an IPC target would invent one that does not exist.
		if !strings.Contains(text, "IpcHandler") {
			continue
		}
		for _, match := range ipcTargetPattern.FindAllStringSubmatch(text, -1) {
			registered[match[1]] = true
		}
	}

	configPath := filepath.Join(env.Root, env.HyprlandConfig)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// A tree with no compositor configuration is not a fault here. The
		// check is about two files disagreeing, and a tree with one of them
		// has nothing to disagree about.
		return nil
	}

	var findings []Finding
	seen := map[string]bool{}
	for _, match := range ipcCallPattern.FindAllStringSubmatch(string(data), -1) {
		target := match[1]
		if registered[target] || seen[target] {
			continue
		}
		seen[target] = true
		findings = append(findings, Finding{
			Rule: "ipc target",
			Path: configPath,
			Detail: "the compositor calls the IPC target \"" + target + "\", and " +
				"the shell does not register it; the keybind would run and do " +
				"nothing",
		})
	}
	return findings
}

// ipcHandler matches the shell's IPC handler by its type.
//
// The type rather than a file name, because a file name is a convention and
// the type is the thing that registers the target. A handler in a file
// called something else is still a handler.
var ipcHandler = regexp.MustCompile(`\bIpcHandler\s*\{`)

// ipcFunction matches a function the handler defines.
var ipcFunction = regexp.MustCompile(`\bfunction\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

// ipcFunctions returns the functions the shell's IPC handler defines.
//
// The handler's own block is read rather than the whole file, because a
// function elsewhere in the file is not something the compositor can call.
func ipcFunctions(env Env, files []string) map[string]bool {
	functions := map[string]bool{}
	for _, file := range files {
		body, err := source(env, file)
		if err != nil {
			continue
		}
		for _, block := range ipcBlocks(body) {
			for _, match := range ipcFunction.FindAllStringSubmatch(block, -1) {
				functions[match[1]] = true
			}
		}
	}
	return functions
}

// ipcBlocks returns the bodies of the shell's IPC handlers.
func ipcBlocks(body string) []string {
	var blocks []string
	for _, match := range ipcHandler.FindAllStringIndex(body, -1) {
		open := match[1] - 1
		end := braceEnd(body, open)
		if end < 0 {
			continue
		}
		blocks = append(blocks, body[open:end])
	}
	return blocks
}

// checkIPCFunctions checks the compositor's calls against the shell's own
// surface.
//
// The target is one half of a call and the function is the other, and the
// check above covers the target. The function is the quieter of the two
// faults: the compositor runs the command, the command succeeds, the shell
// receives it, and the surface the key was supposed to open does not open —
// which a user reports as "the shortcut stopped working" and which nothing
// in the journal explains.
//
// A renamed function is the usual way to get here. The compositor's
// configuration and the shell's QML are two files, and a rename in one of
// them leaves a keybind that is still perfectly valid syntax and does
// nothing at all.
func checkIPCFunctions(env Env, files []string) []Finding {
	if env.HyprlandConfig == "" {
		return nil
	}

	configPath := filepath.Join(env.Root, env.HyprlandConfig)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// As above: a tree with no compositor configuration has nothing to
		// disagree with.
		return nil
	}

	functions := ipcFunctions(env, files)
	if len(functions) == 0 {
		return nil
	}

	var findings []Finding
	seen := map[string]bool{}
	for _, match := range ipcCallPattern.FindAllStringSubmatch(string(data), -1) {
		name := match[2]
		if name == "" || functions[name] || seen[name] {
			continue
		}
		seen[name] = true
		findings = append(findings, Finding{
			Rule: "ipc function",
			Path: configPath,
			Detail: fmt.Sprintf("the compositor calls the shell's %q, and "+
				"the shell's IpcHandler does not define it; the keybind "+
				"would run and do nothing", name),
		})
	}
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

// zenithSchemaPrefix is the id prefix Zenith owns, and the same value the
// settings catalogue filters on — TestZenithSchemaPrefixAgreesWithSettings
// asserts the two are equal so the checker and the catalogue cannot drift
// apart.
//
// The filter is the point, not an optimisation. The schema directory on an
// installed machine is /usr/share/glib-2.0/schemas, and it holds every schema
// the desktop has: GTK, dconf, gsettings-desktop-schemas, shotwell, everything
// the base layer ships. Reading all of it makes this check audit the base layer
// — a "settings unreachable" finding for every key of every third-party schema,
// which is a true statement about files this project does not own and has no
// opinion about, and a run of doctor whose output nobody can read.
//
// It matches on the file name rather than on the parsed id because the name is
// the shipping contract: docs/architecture.md records the schema path as
// /usr/share/glib-2.0/schemas/org.zenith.*.gschema.xml, and build-debs.sh
// copies exactly that pattern. A file that is not named that is somebody else's,
// including when it is malformed — its parse errors are not Zenith's findings.
const zenithSchemaPrefix = "org.zenith."

// loadSchemas parses the schema files the tree owns, keyed by the id they
// declare.
//
// More than one, because the desktop schema is a real dependency: Config.qml
// binds `org.zenith.desktop` alongside the shell's, and it was never parsed at
// all. A key declared there was not checked for reachability, so a setting the
// shell ships and no surface can read went unreported — the exact fault the
// reachability rule exists for, in the half of the database nobody looked at.
//
// A missing schema directory yields one finding rather than none. "No schemas"
// is indistinguishable from "no problems", which is the same failure mode as
// the profile linter's empty database.
func loadSchemas(env Env) (map[string]Schema, []Finding) {
	dir := env.schemaDir()
	entries, err := os.ReadDir(filepath.Join(env.Root, env.schemaDir()))
	if err != nil {
		// Name the path the reader would look in rather than the error OS gave.
		return nil, []Finding{{
			Rule: "schema", Path: filepath.Join(env.Root, dir),
			Detail: "no such directory; schema keys cannot be checked",
		}}
	}

	schemas := map[string]Schema{}
	var findings []Finding
	var found int

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".gschema.xml") {
			continue
		}
		if !strings.HasPrefix(entry.Name(), zenithSchemaPrefix) {
			continue
		}
		path := env.schemaPath(entry.Name())
		schema, err := ParseSchema(path)
		if err != nil {
			// Reported as itself rather than as a missing id. A schema that will
			// not parse is the problem; "no schema declares it" sends the reader
			// looking for a schema that is sitting right there, malformed.
			findings = append(findings, Finding{Rule: "schema", Path: path, Detail: err.Error()})
			continue
		}
		if schema.ID == "" {
			findings = append(findings, Finding{Rule: "schema", Path: path, Detail: "declares no schema id"})
			continue
		}
		if previous, ok := schemas[schema.ID]; ok {
			findings = append(findings, Finding{
				Rule: "schema", Path: path,
				Detail: fmt.Sprintf("redeclares id %q, which declares %d key(s); "+
					"a schema id may only be defined once", schema.ID, len(previous.Keys)),
			})
			continue
		}

		found++
		schemas[schema.ID] = schema
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
	}

	if found == 0 {
		findings = append(findings, Finding{
			Rule: "schema", Path: filepath.Join(env.Root, dir),
			Detail: "contains no org.zenith.*.gschema.xml, so no Zenith key " +
				"can be checked",
		})
	}
	return schemas, findings
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
func checkSchemaIds(env Env, files []string, declared map[string]bool) []Finding {
	var findings []Finding

	// Every schema id the tree declares. An allowlist of known-good ids would
	// be worse than useless: it would let the shell depend on a schema this
	// project does not own.
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
				Rule: "schema id", Path: env.path(f),
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
		Rule: "entry point", Path: env.path(env.EntryPoint),
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
					Rule: "widget catalogue", Path: env.path(f),
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
func checkSettingsReads(env Env, files []string, schemas map[string]Schema) []Finding {
	wrapper := wrapperProperties(env, files)

	reachable := ""
	for _, f := range files {
		if filepath.Base(f) == "Config.qml" {
			body, err := source(env, f)
			if err == nil {
				reachable += body
			}
		}
	}
	lowered := strings.ToLower(reachable)

	ids := make([]string, 0, len(schemas))
	for id := range schemas {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var findings []Finding
	for _, id := range ids {
		schema := schemas[id]
		// The object Config.qml reads this schema off. The shell's is `shell`
		// and the desktop's is `desktop`, and the id's last label is that name
		// in both — so one rule covers both schemas instead of one schema being
		// checked and the other being ignored.
		object := wrapperObject(id)

		// Every schema key must be reachable from the wrapper, because the
		// object is the only thing a surface can read through.
		for _, key := range schema.Keys {
			if !strings.Contains(lowered, object+"."+normalise(key)) {
				findings = append(findings, Finding{
					Rule: "settings unreachable", Path: env.path("Config.qml"),
					Detail: fmt.Sprintf("%s declares %q, and Config.qml never reads "+
						"it, so no surface can reach it", id, key),
				})
			}
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
			// The wrapper is the settings singleton, and a singleton is
			// called by the name of the file that declares it — Config.qml,
			// so `Config`. The comparison ignores the case because the id
			// inside the file is still the lower-case one, and a check that
			// only knew one spelling of it would miss a read.
			if !strings.EqualFold(object, "config") {
				continue
			}
			if wrapper[key] {
				continue
			}
			findings = append(findings, Finding{
				Rule: "settings read", Path: env.path(f),
				Detail: fmt.Sprintf("reads config.%s, which Config.qml does not "+
					"declare; in QML that is undefined rather than an error", key),
			})
		}
	}
	return findings
}

// wrapperObject is the property name Config.qml reads a schema off: the last
// label of the schema id.
func wrapperObject(schemaID string) string {
	parts := strings.Split(schemaID, ".")
	return parts[len(parts)-1]
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
			// A property the wrapper writes as well as reads — the shell's
			// root pushes the values it read into the wrapper — is as much a
			// part of the wrapper's vocabulary as one that is readonly, and a
			// check that only knew the readonly ones would report a write of
			// the wrapper's own values as a read of a property that does not
			// exist.
			`(?:readonly\s+)?property\s+\w+\s+([A-Za-z_][A-Za-z0-9_]*)\s*:`,
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
		// The wrapper is read as `Config`, the name of the file that declares
		// it: a QML singleton is called by the name of its file, and the file
		// is Config.qml.
		for _, match := range regexp.MustCompile(
			`\bConfig\.([A-Za-z_][A-Za-z0-9_]*)`,
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
			Rule: "unused setting", Path: env.path("Config.qml"),
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

// hexColour matches a hard-coded sRGB colour in a string literal.
var hexColour = regexp.MustCompile(`"#[0-9a-fA-F]{6}"`)

// colourExemptions are the files allowed to name a colour, and why.
//
// Theme.qml is where colours come from, so naming one there is the definition
// rather than a departure from it. IdleOverlay.qml is a scrim: it darkens
// whatever is behind it, so it has to be black regardless of the palette, and a
// scrim tinted by the palette would tint the wallpaper along with the desktop.
//
// The file is not called Palette.qml because `Palette` is a name QML already
// has: a file of that name in the shell's own directory is shadowed by it, and
// every surface that read `Palette.text` read nothing at all. The name of a
// file is the only name a QML type can have, so the file's name is a decision
// about what the shell can call its own things.
var colourExemptions = map[string]string{
	"Theme.qml":       "the theme itself",
	"IdleOverlay.qml": "a scrim, which has to be black whatever the theme is",
}

// checkHardcodedColours asserts that no surface names a colour of its own.
//
// This is the check that makes a theme a theme rather than a suggestion. A
// hex value in a QML file is a colour that survives a palette change without
// being noticed, and one such colour is enough to give a themed desktop the two
// themes users complain about: the one they chose, and the one the code has.
//
// The whole shell reads through Palette, so a literal here is always either a
// mistake or a value that should have been derived.
// commandMap is the `commands` table in cmd/zenith/main.go, matched by its
// opening line and its closing brace rather than by any Go parser.
//
// The shell validator already reads files by regex — the schemas, the QML, the
// glyph tables — and importing package main to ask it for its own key set is not
// possible from here. The match is anchored to the exact declaration so a
// `commands` mentioned in a comment or a string does not open the block early.
var commandMap = regexp.MustCompile(`(?s)var commands = map\[string\]func\(args \[\]string\) int\{.*?\n\}`)

// commandKey is one entry in that table.
var commandKey = regexp.MustCompile(`"([a-z][a-z-]*)"\s*:`)

// zenithCall is a shell invoking the CLI. `zenith` as the executable and
// `zenith <cmd>` as the first two argv elements are the only two spellings the
// shell uses, and both are matched.
var zenithCall = regexp.MustCompile(`"zenith"\s*,\s*"([a-z][a-z-]*)"`)

// objectId matches `id: name`, which is what makes a file's own properties
// addressable. Property names are collected separately because an assignment
// names the object first and the property second.
var objectId = regexp.MustCompile(`\bid:\s*([A-Za-z_][A-Za-z0-9_]*)`)

// objectDecl matches `Type { id: name`, which is how an id's own type is
// known. Both parts are needed: whether a property exists depends on where the
// object's type is declared, not on which file happens to assign to it.
var objectDecl = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\{[^{}]*\bid:\s*([A-Za-z_][A-Za-z0-9_]*)`)

// propertyDecl matches both `property string name:` and
// `readonly property string name:`; readonlyProperty narrows it.
var propertyDecl = regexp.MustCompile(`\bproperty\s+[A-Za-z_][A-Za-z0-9_]*\s+([A-Za-z_][A-Za-z0-9_]*)`)
var readonlyProperty = regexp.MustCompile(`\breadonly\s+property\s+[A-Za-z_][A-Za-z0-9_]*\s+([A-Za-z_][A-Za-z0-9_]*)`)

// propertyAssign matches `object.property = value`, stopping before a second
// '=' so that `==`, `===` and `>=` are not read as assignments.
var propertyAssign = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*=[^=]`)

// checkCommands reports a shell calling a subcommand the CLI does not have.
//
// Nothing else in this validator, and nothing in CI, would notice. A QML file
// naming `zenith winodws` is a panel that is empty forever — the Process runs,
// exits non-zero, the shell leaves its last document in place — with no test
// failing anywhere and nothing in the journal saying so. It is the cheapest
// possible break to make by accident and the most expensive to notice.
func checkCommands(env Env, files []string) []Finding {
	entry, err := os.ReadFile(filepath.Join(env.Root, "desktop", "cmd", "zenith", "main.go"))
	if err != nil {
		return nil
	}

	block := commandMap.Find(entry)
	if block == nil {
		return nil
	}

	known := map[string]bool{}
	for _, match := range commandKey.FindAllSubmatch(block, -1) {
		known[string(match[1])] = true
	}
	if len(known) == 0 {
		return nil
	}

	var findings []Finding
	for _, f := range files {
		body, err := source(env, f)
		if err != nil {
			continue
		}
		for _, call := range zenithCall.FindAllStringSubmatch(body, -1) {
			name := string(call[1])
			if known[name] {
				continue
			}
			findings = append(findings, Finding{
				Rule: "unknown command",
				Path: env.path(f),
				Detail: fmt.Sprintf("runs `zenith %s`, which is not in the "+
					"commands table in cmd/zenith/main.go; the call would fail "+
					"at runtime and the surface would silently stay empty", name),
			})
		}
	}
	return findings
}

// assignsToProperty reports whether a source assigns to a bare name.
//
// A bare `name = value` is a write to the property of that name, and a
// write to a readonly property throws. Two things are not writes to it.
//
// A name after a dot is a property of something else: `shell.dockAutohide =
// value` writes to shell's property, and this file declaring a property of
// the same name does not make that line a write to it.
//
// And a declaration is not a write at all: `const items = control.items`
// binds a name inside the function and leaves the property alone. Reporting
// it sent the reader looking for a write that was not there, which is the
// most expensive kind of finding — a true statement about the wrong thing.
func assignsToProperty(body, name string) bool {
	write := regexp.MustCompile(regexp.QuoteMeta(name) + `\s*=[^=]`)
	for _, match := range write.FindAllStringIndex(body, -1) {
		start := match[0]
		if start > 0 {
			previous := body[start-1]
			if previous == '.' || isWordByte(previous) {
				continue
			}
		}
		if declares(body[:start], name) {
			continue
		}
		return true
	}
	return false
}

// declares reports whether the text before a name ends in a declaration.
//
// The keywords are the three a QML file's JavaScript can bind a name with,
// and the name has to start the declaration rather than end it: `constant`
// is not `const`.
func declares(before, name string) bool {
	trimmed := strings.TrimRight(before, " \t")
	for _, keyword := range []string{"const", "let", "var"} {
		if !strings.HasSuffix(trimmed, keyword) {
			continue
		}
		if len(trimmed) == len(keyword) {
			return true
		}
		return !isWordByte(trimmed[len(trimmed)-len(keyword)-1])
	}
	return false
}

// isWordByte reports whether a byte is part of an identifier.
func isWordByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

// braceEnd is the index of the brace closing the one at open, or -1 when the
// file stops before that happens. A string containing a brace can end the scan
// early, and the consequence is an id whose block is not matched and therefore
// not reported on: the check passes over it rather than guessing at it.
func braceEnd(body string, open int) int {
	depth := 0
	for i := open; i < len(body); i++ {
		switch body[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// checkPropertyAssignments reports two mistakes that QML turns into a broken
// surface rather than an error anybody reads.
//
// The first is writing to a readonly property: the assignment throws, the
// function stops there, and everything after it in the same function never
// runs. That is what Palette.apply did — every colour it was supposed to load
// was declared readonly, so the first line threw, `generated` never became
// true, and a generated palette was never applied to a running shell. The
// feature had a generator, a file, a loader and a comment explaining that the
// shell reads it, and it never did.
//
// The second is naming a property the object does not have. QML reports that
// as a TypeError in a runtime nobody is watching, so `text_dim` against a
// property spelled `textDim` fails the same quiet way.
//
// The second rule only runs where the answer is knowable: the id's own object
// block must declare at least one property. `Singleton { id: palette;
// readonly property string textDim }` declares the properties that exist on
// `palette`, so `palette.text_dim` is provably wrong. `ListView { id: list;
// visible: false }` declares none, and its real properties — `currentIndex`
// and the rest — live in a type this repository has no source for, so the
// absence of a declaration there means nothing. Reporting those is how a
// validator gets a name for being wrong: the Hub assigns `list.currentIndex`,
// which is exactly right, and the check said it was not.
func checkPropertyAssignments(env Env, files []string) []Finding {
	var findings []Finding
	for _, f := range files {
		body, err := source(env, f)
		if err != nil {
			continue
		}

		owned := map[string]bool{}
		for _, match := range objectId.FindAllStringSubmatch(body, -1) {
			owned[string(match[1])] = true
		}
		if len(owned) == 0 {
			continue
		}

		readonly := map[string]bool{}
		for _, match := range readonlyProperty.FindAllStringSubmatch(body, -1) {
			readonly[string(match[1])] = true
		}

		// The objects this file declares, and the block each one owns.
		//
		// A property belongs to the innermost object that contains it, not
		// to every object above it: a `readonly property var currentPage`
		// declared inside a page view is the page view's property, and
		// reading it as the window's made a write to the window's own
		// `currentPage` look like a write to a readonly one.
		type objectBlock struct {
			name      string
			open, end int
		}
		var objects []objectBlock
		for _, match := range objectDecl.FindAllStringSubmatchIndex(body, -1) {
			// The regex keeps braces out of the span between the type and the
			// id, so there is exactly one opening brace in it.
			name := body[match[4]:match[5]]
			open := strings.IndexByte(body[match[3]:match[4]], '{') + match[3]
			end := braceEnd(body, open)
			if end < 0 {
				continue
			}
			objects = append(objects, objectBlock{name: name, open: open, end: end})
		}

		// owner is the object a declaration at a position belongs to.
		owner := func(position int) string {
			name, span := "", 0
			for _, object := range objects {
				if position < object.open || position > object.end {
					continue
				}
				if span == 0 || object.end-object.open < span {
					name, span = object.name, object.end-object.open
				}
			}
			return name
		}

		// For each id this file declares, the properties that object declares
		// itself. An id with no custom properties of its own has no known
		// property set and is left alone.
		type objectProperties struct {
			readOnly map[string]bool
			writable map[string]bool
		}
		blocks := map[string]objectProperties{}
		blockOf := func(name string) objectProperties {
			block, known := blocks[name]
			if !known {
				block = objectProperties{
					readOnly: map[string]bool{}, writable: map[string]bool{},
				}
			}
			return block
		}
		for _, match := range readonlyProperty.FindAllStringSubmatchIndex(body, -1) {
			name := owner(match[0])
			if name == "" {
				continue
			}
			block := blockOf(name)
			block.readOnly[body[match[2]:match[3]]] = true
			blocks[name] = block
		}
		for _, match := range propertyDecl.FindAllStringSubmatchIndex(body, -1) {
			name := owner(match[0])
			if name == "" {
				continue
			}
			property := body[match[2]:match[3]]
			block := blockOf(name)
			// A readonly property is a property too, and the plain pattern
			// matches it as well. It is recorded as readonly, which is what
			// the check below reads first.
			if !block.readOnly[property] {
				block.writable[property] = true
			}
			blocks[name] = block
		}
		for name, block := range blocks {
			if len(block.readOnly)+len(block.writable) == 0 {
				// A framework type with nothing of its own declared: its
				// properties are in a source file this repository does not
				// have, so a name we do not recognise proves nothing.
				continue
			}
			blocks[name] = block
		}

		report := func(rule, detail string) {
			findings = append(findings, Finding{
				Rule: rule, Path: env.path(f), Detail: detail,
			})
		}

		for _, match := range propertyAssign.FindAllStringSubmatch(body, -1) {
			object, property := string(match[1]), string(match[2])
			if !owned[object] {
				continue
			}
			block, known := blocks[object]
			if !known {
				continue
			}
			if block.readOnly[property] {
				report("readonly assignment", fmt.Sprintf(
					"assigns %s.%s, which this file declares readonly; the "+
						"assignment throws, and everything after it in the "+
						"same function is silently skipped", object, property))
				continue
			}
			if block.writable[property] {
				continue
			}
			report("property assignment", fmt.Sprintf(
				"assigns %s.%s, which is not a property %s declares; the "+
					"assignment fails at runtime and the surface keeps "+
					"whatever it had", object, property, object))
		}

		// A bare `name = value` against a readonly property of this file throws
		// for the same reason the dotted form does, and it is the form a
		// function inside the object uses: `generated = true` at the end of
		// Palette.apply, which is the line that was supposed to record that a
		// palette was loaded.
		for name := range readonly {
			if assignsToProperty(body, name) {
				report("readonly assignment", fmt.Sprintf(
					"assigns %s, which this file declares readonly; the "+
						"assignment throws", name))
			}
		}
	}
	return findings
}

func checkHardcodedColours(env Env, files []string) []Finding {
	var findings []Finding

	for _, f := range files {
		if _, exempt := colourExemptions[filepath.Base(f)]; exempt {
			continue
		}
		body, err := source(env, f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(body, "\n") {
			for _, hex := range hexColour.FindAllString(line, -1) {
				findings = append(findings, Finding{
					Rule: "hard-coded colour", Path: env.path(f),
					Detail: fmt.Sprintf("names %s; every colour the shell draws "+
						"comes from Palette, and a literal here survives a "+
						"theme change without being noticed", hex),
				})
			}
		}
	}
	return findings
}

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
					Rule: "glyph", Path: env.path(f),
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
	"Quickshell.Services.Pipewire":      true,
	"Quickshell.Networking":             true,
	// The shell's own module. A file in a subdirectory cannot see a type from
	// the directory above it by proximity the way a file beside it can, and
	// the settings and the theme are both in the directory above.
	"qs":               true,
	"QtQuick":          true,
	"QtQuick.Controls": true,
	"QtQuick.Layouts":  true,
	"QtQuick.Window":   true,
}

// importLine matches an import statement and captures the module.
//
// The version and the alias are both matched and both ignored: the module
// is what the rule is about, and an import that renames the module is
// still an import of it. The alias used to go unmatched, which meant a
// file could import anything it liked by giving the import a name — the one
// form of an import the rule could not see.
var importLine = regexp.MustCompile(
	`^\s*import\s+([A-Za-z0-9_.]+)\s*(?:[0-9.]+\s*)?(?:as\s+[A-Za-z_][A-Za-z0-9_]*\s*)?$`)

// shellOwnModule reports whether a module is the shell's own.
//
// The shell's root is importable as `qs`, and the vendored component kit
// lives inside that namespace: `qs.modules.common.widgets` is this
// project's own component kit, pinned to a commit and shipped by this
// project. Nothing a user could have installed provides it, so the closed
// list does not have to name it — and the rule the list exists for, which
// is that the shell's behaviour may not depend on what happens to be on the
// machine, is not weakened by it.
//
// Everything else is on the list, and the list is still closed.
func shellOwnModule(module string) bool {
	return module == "qs" || strings.HasPrefix(module, "qs.")
}

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
			if allowedImports[match[1]] || shellOwnModule(match[1]) {
				continue
			}
			findings = append(findings, Finding{
				Rule: "import", Path: env.path(f),
				Detail: fmt.Sprintf("imports %s, which is not on the "+
					"shell's closed module list", match[1]),
			})
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
				Rule: "boundary", Path: env.path(f),
				Detail: fmt.Sprintf("names %s, outside /usr/share/zenith/ "+
					"and not on the sanctioned list", path),
			})
		}
	}
	return findings
}
