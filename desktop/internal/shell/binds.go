package shell

// The catalogue of what the keyboard does.
//
// The keybindings used to be a list of lines in the compositor's configuration
// and nothing else: no categories, no labels, and no way for a person to find
// out what the keys were. They are a list here as well, and this list says
// what each one is for, so the shell can draw it and a person can read it.
//
// The catalogue and the compositor's configuration are the same list written
// down twice — once for a person and once for the compositor — and a list
// written down twice is a list that drifts. So the two are compared here, in
// both directions, and the comparison is what `zenith doctor` reports.
//
// It is the same argument as the one the settings catalogue makes: a claim
// about two lists is only true if somebody compares them, and the comparison
// has to be a check rather than a habit.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The kinds of binding.
//
// A closed set, because "what the chord does" has to be answerable without
// reading the compositor's source. A kind nobody has a rule for is a kind the
// checks cannot check, and a check that cannot check a thing is a check that
// passes.
const (
	// KindShell is a call into the shell's own IPC surface: the compositor
	// says the key was pressed and the shell decides what that means.
	KindShell = "shell"
	// KindApp is a program the compositor runs.
	KindApp = "app"
	// KindWM is one of the compositor's own actions.
	KindWM = "wm"
)

// Bind is one entry in the catalogue.
//
// The fields are what a person needs in order to use a key: which one it is,
// what it is called, which chord presses it, and what it does. The kind says
// how to read the action, because the same word means three different things
// depending on it.
type Bind struct {
	// ID names the binding, and is unique in the catalogue.
	ID string `json:"id"`
	// Category is the group the binding is listed under.
	Category string `json:"category"`
	// Label is what the binding is called, in words.
	Label string `json:"label"`
	// Chord is the key combination, in the catalogue's spelling.
	Chord string `json:"chord"`
	// Kind is what the chord does: "shell", "app" or "wm".
	Kind string `json:"kind"`
	// Action is what the chord does, in the terms of its kind: the shell
	// function it calls, the program it runs, or the compositor's dispatch.
	Action string `json:"action"`
	// Argument is what the action is given, when it is given anything.
	Argument string `json:"argument,omitempty"`
	// ReleaseAction is what the chord does when the key comes back up, for
	// the bindings that do something else on the release. The window
	// switcher is the one: it advances on the press and closes on the
	// release. The compositor's flag for it is `r` (`bindr`); `bindl` is the
	// lock-screen flag and fires on the press.
	ReleaseAction string `json:"releaseAction,omitempty"`
	// Also lists other chords that do the same thing, so the cheat sheet
	// draws one row for a thing and the check still accounts for every key:
	// the other arrow keys, the scroll wheel, the second media key.
	Also []Alternate `json:"also,omitempty"`
}

// Alternate is another chord for a catalogue entry. Its argument replaces the
// entry's when it has one: the other three arrow keys name other directions.
type Alternate struct {
	Chord    string `json:"chord"`
	Argument string `json:"argument,omitempty"`
}

// Category is a group of bindings.
type Category struct {
	// ID is the group's name in the catalogue.
	ID string `json:"id"`
	// Label is what the group is called on screen.
	Label string `json:"label"`
}

// Catalogue is the whole file.
type Catalogue struct {
	Categories []Category `json:"categories"`
	Binds      []Bind     `json:"binds"`
}

// LoadCatalogue reads the keybinding catalogue.
//
// The unknown fields are refused rather than ignored, because a field the
// reader does not know is a field the writer spelled wrong, and a binding
// whose argument was spelled wrong is a binding that does the wrong thing
// without anybody noticing.
func LoadCatalogue(path string) (Catalogue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Catalogue{}, err
	}

	var catalogue Catalogue
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&catalogue); err != nil {
		return Catalogue{}, fmt.Errorf("the catalogue is not readable: %w", err)
	}
	return catalogue, nil
}

// The catalogue's spelling of a chord.
//
// Modifiers are written in a fixed order and separated from the key by a
// plus, and the key keeps the spelling the keyboard itself uses: "SUPER +
// SHIFT + {n}", not "shift+super+1". The order is a convention rather than
// anybody's requirement, and it exists so that a chord is written the same
// way every time it is written — a catalogue with two spellings of one chord
// is a catalogue that cannot be compared with anything.
const separator = " + "

// modifierOrder is the order the modifiers are written in.
//
// The comparison sorts both sides into it, so the compositor's configuration
// is free to spell "SUPER SHIFT" in whatever order it likes and the two
// files still agree.
var modifierOrder = []string{"SUPER", "CTRL", "ALT", "SHIFT"}

// Family is the placeholder that stands for the number keys.
//
// A family is one entry in the catalogue and thirty binds in the compositor's
// configuration, and this is the one thing that says so.
const Family = "{n}"

type familyKey struct {
	Key     string
	Ordinal string
}

// familyKeys is the number row and the number pad, and the number each key
// stands for.
//
// The tenth is the 0 key, which is the tenth workspace rather than a
// workspace called zero: the number row has no room for a tenth digit, so
// the tenth of something is spelled 0. That is Hyprland's own convention and
// the one its default configuration uses, and the placeholder therefore takes
// the number of the position rather than the key.
//
// The pad is there twice: as KP_<n>, the keysym with NumLock on, and as the
// navigation keysym the same key sends with NumLock off, so a shortcut fires
// either way.
var familyKeys = func() []familyKey {
	pad := []string{"KP_End", "KP_Down", "KP_Next", "KP_Left", "KP_Begin",
		"KP_Right", "KP_Home", "KP_Up", "KP_Prior", "KP_Insert"}
	var keys []familyKey
	for _, prefix := range []string{"", "KP_"} {
		for i := 1; i <= 10; i++ {
			keys = append(keys, familyKey{prefix + strconv.Itoa(i%10), strconv.Itoa(i)})
		}
	}
	for i, key := range pad {
		keys = append(keys, familyKey{key, strconv.Itoa(i + 1)})
	}
	return keys
}()

// Chord is a key and the modifiers that go with it.
type Chord struct {
	Modifiers []string
	Key       string
}

// ParseChord reads a chord in the catalogue's spelling.
//
// A chord that does not follow the spelling is a fault rather than something
// to be interpreted, because a catalogue with two spellings of one chord is a
// catalogue that cannot be compared with anything.
func ParseChord(text string) (Chord, error) {
	parts := strings.Split(text, separator)
	if len(parts) == 0 {
		return Chord{}, fmt.Errorf("the chord %q names no key", text)
	}

	chord := Chord{Key: strings.TrimSpace(parts[len(parts)-1])}
	if chord.Key == "" {
		return Chord{}, fmt.Errorf("the chord %q names no key", text)
	}
	for _, part := range parts[:len(parts)-1] {
		modifier := strings.TrimSpace(part)
		if !knownModifier(modifier) {
			return Chord{}, fmt.Errorf("the chord %q names the modifier %q, "+
				"which is not one of %s", text, modifier,
				strings.Join(modifierOrder, ", "))
		}
		chord.Modifiers = append(chord.Modifiers, modifier)
	}

	if sorted := sortModifiers(chord.Modifiers); !sameOrder(sorted, chord.Modifiers) {
		return Chord{}, fmt.Errorf("the chord %q writes its modifiers out of "+
			"order; they go %s", text, strings.Join(modifierOrder, ", "))
	}
	return chord, nil
}

// String is the chord in the catalogue's spelling.
func (c Chord) String() string {
	parts := append([]string{}, c.Modifiers...)
	return strings.Join(append(parts, c.Key), separator)
}

func knownModifier(name string) bool {
	for _, modifier := range modifierOrder {
		if modifier == name {
			return true
		}
	}
	return false
}

// sortModifiers puts the modifiers into the catalogue's order.
//
// A modifier nobody knows sorts last rather than first, so that a chord with
// a typo in it is still readable in a message.
func sortModifiers(modifiers []string) []string {
	rank := func(name string) int {
		for i, modifier := range modifierOrder {
			if modifier == name {
				return i
			}
		}
		return len(modifierOrder)
	}

	sorted := append([]string{}, modifiers...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return rank(sorted[i]) < rank(sorted[j])
	})
	return sorted
}

func sameOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Expanded is one binding of a catalogue entry: a chord, and the action that
// chord performs.
//
// A family of keys is one entry in the catalogue and thirty of these.
type Expanded struct {
	Chord    string
	Action   string
	Argument string
}

// Expand returns the bindings a catalogue entry covers, its alternates
// included.
//
// The argument is expanded along with the chord, because a family of
// workspaces is ten chords and ten different workspaces: "SUPER + {n}" with
// the argument "{n}" is the tenth workspace on the 0 key and the first on the
// 1 key, and the two are not the same binding.
func (b Bind) Expand() ([]Expanded, error) {
	expanded, err := expandChord(b.Chord, b.Action, b.Argument)
	if err != nil {
		return nil, err
	}
	for _, alternate := range b.Also {
		argument := b.Argument
		if alternate.Argument != "" {
			argument = alternate.Argument
		}
		more, err := expandChord(alternate.Chord, b.Action, argument)
		if err != nil {
			return nil, err
		}
		expanded = append(expanded, more...)
	}
	return expanded, nil
}

func expandChord(text, action, argument string) ([]Expanded, error) {
	chord, err := ParseChord(text)
	if err != nil {
		return nil, err
	}

	if !strings.Contains(text, Family) {
		return []Expanded{{
			Chord:    chord.String(),
			Action:   action,
			Argument: argument,
		}}, nil
	}

	var expanded []Expanded
	for _, family := range familyKeys {
		expanded = append(expanded, Expanded{
			Chord: Chord{
				Modifiers: chord.Modifiers,
				Key:       strings.ReplaceAll(chord.Key, Family, family.Key),
			}.String(),
			Action:   action,
			Argument: strings.ReplaceAll(argument, Family, family.Ordinal),
		})
	}
	return expanded, nil
}

// compositorBind is one bind in the compositor's configuration.
type compositorBind struct {
	// Directive is which of Hyprland's forms this is: "bind" plus flags, in
	// any order. "r" makes it fire on the release; "l" (locked) only lets it
	// work over the lock screen and does not change when it fires.
	Directive string
	// Chord is the key and its modifiers, in the catalogue's spelling.
	Chord string
	// Dispatcher is what the compositor does: "exec" for a program, or the
	// name of one of its own actions.
	Dispatcher string
	// Arguments is what the dispatcher is given.
	Arguments string
	// Line is where the bind is in the file.
	Line int
}

// bindDirective matches a bind line in the compositor's configuration: "bind"
// and any of the flags Hyprland defines (l r c g o e n m t i s d p u), so
// "bindrl" and "bindre" are read as well as "bindr".
var bindDirective = regexp.MustCompile(
	`^\s*(bind[lrcgonmtisdpue]*)\s*=\s*(.+?)\s*$`)

// submapDirective matches the line that opens a submap, and "submap = reset"
// the one that closes it.
var submapDirective = regexp.MustCompile(`^\s*submap\s*=\s*([^,\s]+)`)

// parseBinds reads the binds out of the compositor's configuration.
//
// Only the bind lines are read. The rest of the file is the compositor's own
// business — monitors, rules, environment — and none of it is a key. Two kinds
// of bind line are not chords the catalogue lists: the keys inside a submap
// (resize mode's bare arrows), which only exist while the mode is on, and the
// lid switch, which is a device.
func parseBinds(body string) []compositorBind {
	var binds []compositorBind
	submap := ""
	for number, line := range strings.Split(body, "\n") {
		if match := submapDirective.FindStringSubmatch(line); match != nil {
			submap = match[1]
			if submap == "reset" {
				submap = ""
			}
			continue
		}
		match := bindDirective.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if submap != "" && !strings.Contains(match[1], "u") {
			continue
		}

		// The modifiers, the key, the dispatcher, and whatever the
		// dispatcher is given. The command an exec dispatcher runs is the
		// rest of the line, and it is joined back together rather than
		// taken field by field, because a command is a command and not a
		// list of arguments.
		fields := strings.Split(match[2], ",")
		if len(fields) < 3 {
			continue
		}
		key := strings.TrimSpace(fields[1])
		if key == "" || strings.HasPrefix(key, "switch:") {
			continue
		}

		binds = append(binds, compositorBind{
			Directive:  match[1],
			Chord:      canonicalChord(strings.Fields(fields[0]), key),
			Dispatcher: strings.TrimSpace(fields[2]),
			Arguments:  strings.TrimSpace(strings.Join(fields[3:], ", ")),
			Line:       number + 1,
		})
	}
	return binds
}

// canonicalChord sorts a chord's modifiers into the catalogue's order.
//
// The key is left exactly as it was written, because the key is what the
// keyboard reports and a chord whose key had been rewritten would be a chord
// that no longer matched what the user pressed.
func canonicalChord(modifiers []string, key string) string {
	return Chord{Modifiers: sortModifiers(modifiers), Key: key}.String()
}

// action says what a bind does, in the terms the catalogue uses.
//
// A call into the shell's IPC surface is named by the function it calls, a
// program by its name, and a compositor action by its dispatch. That is the
// same three-way split the catalogue's kinds describe, so the two can be
// compared without either of them being changed to suit the other.
func (b compositorBind) action() (string, string) {
	if b.Dispatcher != "exec" {
		return b.Dispatcher, b.Arguments
	}

	if match := ipcCallPattern.FindStringSubmatch(b.Arguments); match != nil {
		return match[2], match[3]
	}

	command := strings.TrimSpace(b.Arguments)
	if command == "" {
		return "", ""
	}
	fields := strings.Fields(command)
	return fields[0], strings.TrimSpace(strings.TrimPrefix(command, fields[0]))
}

// describe says what a bind does, in words.
func (b compositorBind) describe() string {
	action, argument := b.action()
	if argument == "" {
		return action
	}
	return action + " " + argument
}

// isRelease says whether a bind fires when the key comes up: the `r` flag.
// `bindl` is the lock-screen flag, and a bind/bindl pair fires both on the
// press, which is how a switcher once opened and closed in the same instant.
func (b compositorBind) isRelease() bool {
	return strings.Contains(strings.TrimPrefix(b.Directive, "bind"), "r")
}

// covered is a chord the catalogue accounts for, and what it says about it.
type covered struct {
	Entry    Bind
	Expanded Expanded
}

// checkBinds checks the catalogue against the compositor's configuration.
//
// The two are the same list written down twice, and the comparison is in
// both directions: a catalogue entry with no bind is a key that does not
// work, and a bind with no catalogue entry is a key that nobody can find.
func checkBinds(env Env) []Finding {
	if env.BindsCatalogue == "" || env.HyprlandConfig == "" {
		return nil
	}

	cataloguePath := filepath.Join(env.Root, env.BindsCatalogue)
	catalogue, err := LoadCatalogue(cataloguePath)
	if err != nil {
		return []Finding{{
			Rule:   "keybinding catalogue",
			Path:   cataloguePath,
			Detail: err.Error(),
		}}
	}

	configPath := filepath.Join(env.Root, env.HyprlandConfig)
	body, err := readCompositorConfig(configPath)
	if err != nil {
		// A tree with no compositor configuration is not a fault here. The
		// check is about two files disagreeing, and a tree with one of them
		// has nothing to disagree about.
		return nil
	}

	return compareBinds(catalogue, cataloguePath, configPath, string(body))
}

// readCompositorConfig returns the compositor's configuration as one text: the
// file Hyprland is started with, and every other .conf beside it. The binds are
// in files the first one sources, and a source line is not worth parsing for
// the one directory they all live in.
func readCompositorConfig(path string) ([]byte, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	siblings, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.conf"))
	sort.Strings(siblings)
	for _, sibling := range siblings {
		if sibling == path {
			continue
		}
		if more, err := os.ReadFile(sibling); err == nil {
			body = append(append(body, '\n'), more...)
		}
	}
	return body, nil
}

// compareBinds reports where the catalogue and the configuration disagree.
func compareBinds(catalogue Catalogue, cataloguePath, configPath, body string) []Finding {
	binds := parseBinds(body)

	var findings []Finding
	findings = append(findings, checkCatalogueShape(catalogue, cataloguePath)...)

	// Every chord the catalogue accounts for, and what it says about it.
	coveredChords := map[string]covered{}

	for _, entry := range catalogue.Binds {
		expanded, err := entry.Expand()
		if err != nil {
			findings = append(findings, Finding{
				Rule: "keybind", Path: cataloguePath, Detail: err.Error(),
			})
			continue
		}

		for _, item := range expanded {
			if previous, taken := coveredChords[item.Chord]; taken {
				findings = append(findings, Finding{
					Rule: "keybind", Path: cataloguePath,
					Detail: fmt.Sprintf("%q and %q both claim the chord %q, "+
						"and only one of them can be what the key does",
						previous.Entry.ID, entry.ID, item.Chord),
				})
				continue
			}
			coveredChords[item.Chord] = covered{Entry: entry, Expanded: item}

			matches := bindsFor(binds, item.Chord)
			if len(matches) == 0 {
				findings = append(findings, Finding{
					Rule: "keybind", Path: cataloguePath,
					Detail: fmt.Sprintf("lists %q, and the compositor's "+
						"configuration does not bind it", item.Chord),
				})
				continue
			}

			pressed, released := false, false
			for _, bind := range matches {
				action, argument := bind.action()
				switch {
				case action == item.Action && argument == item.Argument &&
					!bind.isRelease():
					pressed = true
				case entry.ReleaseAction != "" && action == entry.ReleaseAction &&
					argument == "" && bind.isRelease():
					released = true
				}
			}

			if !pressed {
				findings = append(findings, Finding{
					Rule: "keybind", Path: cataloguePath,
					Detail: fmt.Sprintf("says %q does %s, and the "+
						"compositor's configuration does not",
						item.Chord, describeExpanded(item)),
				})
			}
			if entry.ReleaseAction != "" && !released {
				findings = append(findings, Finding{
					Rule: "keybind", Path: cataloguePath,
					Detail: fmt.Sprintf("says %q does %s when the key "+
						"comes up, and the compositor's configuration "+
						"does not", item.Chord, entry.ReleaseAction),
				})
			}
		}
	}

	// The other direction: a bind the catalogue does not account for.
	for _, bind := range binds {
		entry, known := coveredChords[bind.Chord]
		if !known {
			findings = append(findings, Finding{
				Rule: "keybind", Path: configPath,
				Detail: fmt.Sprintf("binds %q, and the catalogue does "+
					"not list it", bind.Chord),
			})
			continue
		}

		action, argument := bind.action()
		presses := action == entry.Expanded.Action &&
			argument == entry.Expanded.Argument && !bind.isRelease()
		releases := entry.Entry.ReleaseAction != "" &&
			action == entry.Entry.ReleaseAction && argument == "" &&
			bind.isRelease()
		if presses || releases {
			continue
		}

		findings = append(findings, Finding{
			Rule: "keybind", Path: configPath,
			Detail: fmt.Sprintf("binds %q to %s, and the catalogue says "+
				"it does %s", bind.Chord, bind.describe(),
				describeExpanded(entry.Expanded)),
		})
	}

	return findings
}

// bindsFor returns the binds of one chord.
func bindsFor(binds []compositorBind, chord string) []compositorBind {
	var found []compositorBind
	for _, bind := range binds {
		if bind.Chord == chord {
			found = append(found, bind)
		}
	}
	return found
}

// describeExpanded says what a binding does, in words.
func describeExpanded(item Expanded) string {
	if item.Argument == "" {
		return item.Action
	}
	return item.Action + " " + item.Argument
}

// checkCatalogueShape asserts that the catalogue is a catalogue: that its
// entries are the kinds the rules know, that their ids are unique, and that
// they are filed under a category the file declares.
//
// The category is the one that matters. A binding filed under a category the
// file does not declare is a binding the cheat sheet never draws: it is in
// the catalogue, it is bound, and the one place a person could look for it
// does not show it.
func checkCatalogueShape(catalogue Catalogue, path string) []Finding {
	var findings []Finding

	categories := map[string]bool{}
	for _, category := range catalogue.Categories {
		categories[category.ID] = true
	}

	ids := map[string]bool{}
	for _, entry := range catalogue.Binds {
		switch entry.Kind {
		case KindShell, KindApp, KindWM:
		default:
			findings = append(findings, Finding{
				Rule: "keybinding catalogue", Path: path,
				Detail: fmt.Sprintf("%q is the kind of %q, and it is "+
					"neither %q, %q nor %q", entry.Kind, entry.ID,
					KindShell, KindApp, KindWM),
			})
		}

		if ids[entry.ID] {
			findings = append(findings, Finding{
				Rule: "keybinding catalogue", Path: path,
				Detail: fmt.Sprintf("%q is declared twice; an id has to "+
					"mean one binding", entry.ID),
			})
		}
		ids[entry.ID] = true

		if entry.Category != "" && !categories[entry.Category] {
			findings = append(findings, Finding{
				Rule: "keybinding catalogue", Path: path,
				Detail: fmt.Sprintf("%q is filed under the category %q, "+
					"which the catalogue does not declare", entry.ID,
					entry.Category),
			})
		}
	}

	return findings
}
