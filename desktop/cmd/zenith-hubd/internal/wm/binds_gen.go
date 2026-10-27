package wm

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Keybind, unbind and rebind emission for the generated hyprlang.
//
// A rebind is the one place this backend has to know what a shipped bind does,
// because Hyprland has no "move this bind to another chord": it is an unbind of
// the old chord and a bind of the new one with the same dispatcher. Ryoku's Lua
// bound every shipped key through a K(chord) lookup that consulted a generated
// table, so it never had to; hyprlang has no such hook. The dispatcher is read
// from the packaged hyprland.conf, the file that says what the keys do, and
// falls back to the words in the catalogue when that file cannot be read.

func genKeybind(k Keybind) string {
	if strings.TrimSpace(k.Keys) == "" {
		return ""
	}
	var dsp string
	switch k.Action {
	case "exec", "":
		if strings.TrimSpace(k.Value) == "" {
			return ""
		}
		dsp = "exec, " + hlStr(k.Value)
	case "close":
		dsp = "killactive,"
	case "fullscreen":
		dsp = "fullscreen, 0"
	case "togglefloating":
		dsp = "togglefloating,"
	default:
		return ""
	}
	flag := ""
	if k.Release {
		flag = "r"
	}
	line := func(chord string) string {
		mods, key := SplitChord(chord)
		if key == "" {
			return ""
		}
		return fmt.Sprintf("bind%s = %s, %s, %s\n", flag, mods, hlStr(key), dsp)
	}
	out := line(k.Keys)
	if out == "" {
		return ""
	}
	// The number pad fires whichever way NumLock sits: a custom chord on a
	// KP_<digit> binds its NumLock-off twin to the same dispatcher too, the way
	// the shipped families cover both keypad faces.
	for _, twin := range NumpadAliases(k.Keys) {
		out += line(twin)
	}
	return out
}

// genUnbinds drops the shipped chords a config import must release so an
// imported bind that shadows a shipped one wins. Emitted before the binds, so
// after the packaged config registered the shipped chord and before the
// imported bind re-adds it.
func genUnbinds(o Overrides) string {
	var b strings.Builder
	for _, c := range o.Unbinds {
		mods, key := SplitChord(strings.TrimSpace(c))
		if key == "" {
			continue
		}
		fmt.Fprintf(&b, "unbind = %s, %s\n", mods, hlStr(key))
	}
	return b.String()
}

// --- reading the packaged binds -----------------------------------------------

// confBind is one bind line of the packaged configuration.
type confBind struct {
	flags string // the letters after "bind": l, r, e, ...
	chord string // comparison form, see chordKey
	rest  string // dispatcher and arguments, exactly as written
}

var (
	reVarDef  = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$`)
	reBindDef = regexp.MustCompile(`^bind([a-z]*)\s*=\s*(.*)$`)
	reSource  = regexp.MustCompile(`^source\s*=\s*(.*)$`)
)

// generatedNames are the files this backend or the user owns. They are never
// read back as the shipped binds, or a rebind would be built from a rebind.
var generatedNames = map[string]bool{
	"settings.conf": true, "rebinds.conf": true, "user.conf": true, "zenith.conf": true,
}

// PackagedConfigPath is the hyprland.conf the session loads.
func PackagedConfigPath() string {
	if p := os.Getenv("ZENITH_HYPRLAND_CONFIG"); p != "" {
		return p
	}
	for _, p := range []string{
		"/usr/share/zenith/hyprland/hyprland.conf",
		filepath.Join(HyprConfigDir(), "hyprland.conf"),
	} {
		if fileExists(p) {
			return p
		}
	}
	return "/usr/share/zenith/hyprland/hyprland.conf"
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		return filepath.Join(os.Getenv("HOME"), strings.TrimPrefix(p, "~"))
	}
	return p
}

// stripComment drops a trailing "# comment". A '#' only starts one at the start
// of a line or after whitespace, and "##" is a literal '#', so a command that
// carries a '#' keeps it.
func stripComment(line string) string {
	for i := 0; i < len(line); i++ {
		if line[i] != '#' {
			continue
		}
		if i+1 < len(line) && line[i+1] == '#' {
			i++
			continue
		}
		if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
			return line[:i]
		}
	}
	return line
}

// parseConfBinds reads the bind lines of a hyprlang file and of every file it
// sources, in order. seen guards a source cycle.
func parseConfBinds(path string, vars map[string]string, seen map[string]bool) []confBind {
	path = filepath.Clean(path)
	if seen[path] {
		return nil
	}
	seen[path] = true
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []confBind
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(stripComment(line))
		if line == "" {
			continue
		}
		if m := reVarDef.FindStringSubmatch(line); m != nil {
			vars[m[1]] = strings.TrimSpace(m[2])
			continue
		}
		for name, val := range vars {
			line = strings.ReplaceAll(line, "$"+name, val)
		}
		if m := reSource.FindStringSubmatch(line); m != nil {
			target := expandHome(strings.TrimSpace(m[1]))
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			if generatedNames[filepath.Base(target)] {
				continue
			}
			out = append(out, parseConfBinds(target, vars, seen)...)
			continue
		}
		m := reBindDef.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		flags := m[1]
		// bindm binds a mouse button to a drag; nothing to rebind.
		if strings.Contains(flags, "m") {
			continue
		}
		parts := strings.SplitN(m[2], ",", 3)
		if len(parts) < 3 {
			continue
		}
		mods, key := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		out = append(out, confBind{
			flags: flags,
			chord: chordOf(mods, key),
			rest:  strings.TrimSpace(parts[2]),
		})
	}
	return out
}

// chordKey is the comparison form of a chord: normalised, and lower case so a
// key typed "Q" in the catalogue and "q" in the configuration are the same key.
func chordKey(chord string) string { return strings.ToLower(NormChord(chord)) }

// chordOf builds the catalogue spelling of a hyprlang modifier list and key.
// Hyprland finds a modifier by substring, so "SUPER_SHIFT" and "SUPER SHIFT" are
// the same and so are they here.
func chordOf(mods, key string) string {
	up := strings.ToUpper(mods)
	var tokens []string
	for _, m := range []struct{ name, find string }{
		{"SUPER", "SUPER"}, {"SUPER", "WIN"}, {"SUPER", "LOGO"}, {"SUPER", "MOD4"}, {"SUPER", "META"},
		{"CTRL", "CTRL"}, {"CTRL", "CONTROL"},
		{"ALT", "ALT"}, {"ALT", "MOD1"},
		{"SHIFT", "SHIFT"},
	} {
		if strings.Contains(up, m.find) {
			tokens = append(tokens, m.name)
		}
	}
	tokens = append(tokens, key)
	return chordKey(strings.Join(tokens, " + "))
}

// --- rebinds.conf -------------------------------------------------------------

// renderRebinds is the generated rebinds.conf: for each rebind, an unbind of the
// shipped chord and the same bind on the chosen one. Always a whole file, so
// clearing the last rebind leaves no stale line behind.
func renderRebinds(o Overrides) []byte {
	var b strings.Builder
	b.WriteString(rebindsHeader)
	if len(o.KeybindRebinds) == 0 {
		return []byte(b.String())
	}
	cat, err := LoadCatalogue()
	if err != nil {
		fmt.Fprintf(&b, "# the bind catalogue could not be read (%v), so no rebind is written\n", err)
		return []byte(b.String())
	}
	shipped := parseConfBinds(PackagedConfigPath(), map[string]string{}, map[string]bool{})
	byChord := map[string][]confBind{}
	for _, cb := range shipped {
		byChord[cb.chord] = append(byChord[cb.chord], cb)
	}

	byDefault := map[string]CatalogBind{}
	for _, c := range cat {
		byDefault[c.Chord] = c
	}
	defaults := make([]string, 0, len(o.KeybindRebinds))
	for d := range o.KeybindRebinds {
		defaults = append(defaults, d)
	}
	sort.Strings(defaults)

	for _, def := range defaults {
		c, known := byDefault[strings.TrimSpace(def)]
		if !known || c.Locked {
			continue
		}
		to := strings.TrimSpace(o.KeybindRebinds[def])
		if to == "" || to == c.Chord {
			continue
		}
		if c.Family {
			eff, ok := FamilyRebind(c.Chord, o.KeybindRebinds)
			if !ok {
				continue
			}
			for n := 1; n <= 10; n++ {
				emitRebind(&b, c, ExpandChord(c.Chord, n), ExpandChord(eff, n), byChord)
				// The keypad sends a second keysym with NumLock off, and the
				// shipped family binds both, so the rebind moves both.
				fa, ta := NumpadAliases(ExpandChord(c.Chord, n)), NumpadAliases(ExpandChord(eff, n))
				if len(fa) > 0 && len(ta) > 0 {
					emitRebind(&b, c, fa[0], ta[0], byChord)
				}
			}
			continue
		}
		emitRebind(&b, c, c.Chord, to, byChord)
	}
	return []byte(b.String())
}

func emitRebind(b *strings.Builder, c CatalogBind, from, to string, byChord map[string][]confBind) {
	fromMods, fromKey := SplitChord(from)
	toMods, toKey := SplitChord(to)
	if fromKey == "" || toKey == "" {
		return
	}
	binds := byChord[chordKey(from)]
	if len(binds) == 0 {
		binds = synthesise(c, from)
	}
	if len(binds) == 0 {
		return
	}
	fmt.Fprintf(b, "unbind = %s, %s\n", fromMods, hlStr(fromKey))
	existingOnTo := byChord[chordKey(to)]
	for _, cb := range binds {
		alreadyBound := false
		for _, existing := range existingOnTo {
			if existing.flags == cb.flags && existing.rest == cb.rest {
				alreadyBound = true
				break
			}
		}
		if !alreadyBound {
			fmt.Fprintf(b, "bind%s = %s, %s, %s\n", cb.flags, toMods, hlStr(toKey), cb.rest)
		}
	}
}

// synthesise rebuilds a shipped bind from the catalogue's own words, for when the
// packaged hyprland.conf cannot be read: a compositor action is the dispatcher
// itself, a shell call goes through the control tool, an app is run. A chord
// that releases to something else gets its release bind too.
func synthesise(c CatalogBind, chord string) []confBind {
	var press string
	switch c.Kind {
	case BindWM:
		press = c.Action
		if c.Argument != "" {
			press += ", " + c.Argument
		} else {
			press += ","
		}
	case BindShell:
		press = "exec, zenith ipc call zenith " + c.Action
		if c.Argument != "" {
			press += " " + c.Argument
		}
	case BindApp:
		press = "exec, " + c.Action
	default:
		return nil
	}
	if c.Action == "" {
		return nil
	}
	out := []confBind{{chord: chord, rest: press}}
	if c.ReleaseAction != "" {
		out = append(out, confBind{flags: "r", chord: chord, rest: "exec, zenith ipc call zenith " + c.ReleaseAction})
	}
	return out
}
