package main

// Per-app parsers for config import. The native Hyprland reader is the mirror of
// keybind legend: it turns a hand-written hyprland.conf into the
// same structured model the hub already renders (binds, window rules), plus the
// raw lines that layer into user.conf. kitty, fish and fastfetch are not parsed
// deeply (their user-include already wins); they are catalogued for the review
// UI and copied wholesale on apply.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// importBind is one parsed hyprland.conf bind. Action is the hub Keybind action
// (exec|close|fullscreen|togglefloating) when Ingestable, else the raw native
// dispatcher token for display. Raw is the verbatim source line.
type importBind struct {
	Raw        string
	Combo      string // display form, SUPER + Q
	Norm       string // stable conflict key
	Dispatcher string // native token, verbatim
	Action     string
	Value      string
	Ingestable bool
	Desc       string
	// Native is the bind as a loadable hyprlang line (variables resolved), for a
	// bind the Hub's keybind model cannot hold.
	Native string
}

// importRule is one parsed window rule, mapped onto the hub WindowRule model.
type importRule struct {
	Raw  string
	Rule WindowRule
}

// rawLine is a non-ingestable Hyprland line bound for user.conf. Conf is the
// line with its variables resolved and its section path flattened, so it loads
// on its own; the config language is the same one the line came from, which is
// why Ryoku's translation into a Lua API is gone.
type rawLine struct {
	Kind string // monitor|env|exec|setting|raw
	Raw  string
	Conf string
}

type hyprScan struct {
	Binds []importBind
	Rules []importRule
	Raws  []rawLine
}

// parseHyprland walks a hyprland.conf once, producing the ordered scan items for
// the review UI and the structured model apply consumes. Variables ($mainMod)
// are resolved so a bind's chord matches the shipped legend; the original line
// is kept verbatim as each item's raw.
func parseHyprland(src string) ([]scanItem, hyprScan) {
	vars := collectVars(src)
	items := []scanItem{}
	var hs hyprScan
	var section []string

	for _, raw := range strings.Split(src, "\n") {
		line := stripInlineComment(raw)
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasSuffix(t, "{") && !strings.Contains(t, "=") {
			section = append(section, strings.TrimSpace(strings.TrimSuffix(t, "{")))
			continue
		}
		if t == "}" {
			if len(section) > 0 {
				section = section[:len(section)-1]
			}
			continue
		}
		key, val, ok := splitKV(t)
		if !ok {
			continue
		}
		val = substVars(val, vars)
		lkey := strings.ToLower(key)
		origin := strings.TrimSpace(raw)
		if strings.HasPrefix(key, "$") {
			continue // variable definition, already resolved via collectVars
		}

		switch {
		case isBindKeyword(lkey):
			b := parseBind(val, origin)
			b.Native = key + " = " + val
			hs.Binds = append(hs.Binds, b)
			items = append(items, scanItem{Kind: "bind", Raw: origin, Combo: b.Combo, Dispatcher: b.Dispatcher, Ingestable: b.Ingestable})
		case lkey == "windowrule" || lkey == "windowrulev2":
			r := parseWindowRule(val, origin)
			hs.Rules = append(hs.Rules, r)
			items = append(items, scanItem{Kind: "windowrule", Raw: origin, Ingestable: true})
		case lkey == "monitor":
			rl := rawLine{Kind: "monitor", Raw: origin, Conf: key + " = " + val}
			hs.Raws = append(hs.Raws, rl)
			items = append(items, scanItem{Kind: "monitor", Raw: origin, Ingestable: false})
		case lkey == "env" || lkey == "envd":
			rl := rawLine{Kind: "env", Raw: origin, Conf: key + " = " + val}
			hs.Raws = append(hs.Raws, rl)
			items = append(items, scanItem{Kind: "env", Raw: origin, Ingestable: false})
		case isExecKeyword(lkey):
			rl := rawLine{Kind: "exec", Raw: origin, Conf: key + " = " + val}
			hs.Raws = append(hs.Raws, rl)
			items = append(items, scanItem{Kind: "exec", Raw: origin, Ingestable: false})
		default:
			// generic setting (bare or inside a section { } block), flattened to its
			// colon path so it stands alone: decoration { rounding = 4 } becomes
			// decoration:rounding = 4.
			path := append(append([]string(nil), section...), splitColonPath(key)...)
			rl := rawLine{Kind: "setting", Raw: origin, Conf: strings.Join(path, ":") + " = " + val}
			hs.Raws = append(hs.Raws, rl)
			items = append(items, scanItem{Kind: "setting", Raw: origin, Ingestable: false})
		}
	}
	return items, hs
}

// parseBind splits `MODS, KEY, DISPATCHER, ARGS` (args may hold commas) and maps
// the native dispatcher onto the hub Keybind model. killactive is Zenith's
// "close"; anything outside the four ingestable dispatchers stays raw.
func parseBind(val, raw string) importBind {
	f := splitNCommaTrim(val, 4)
	b := importBind{Raw: raw}
	mods, keyName, disp, args := "", "", "", ""
	if len(f) > 0 {
		mods = f[0]
	}
	if len(f) > 1 {
		keyName = f[1]
	}
	if len(f) > 2 {
		disp = f[2]
	}
	if len(f) > 3 {
		args = f[3]
	}
	var toks []string
	for _, m := range strings.Fields(mods) {
		toks = append(toks, strings.ToUpper(m))
	}
	if keyName != "" {
		toks = append(toks, keyName)
	}
	b.Combo = strings.Join(toks, " + ")
	b.Norm = normCombo(b.Combo)
	b.Dispatcher = disp

	switch strings.ToLower(disp) {
	case "exec":
		b.Action, b.Value, b.Ingestable = "exec", args, true
		b.Desc = capitalize(describeExec(args))
	case "killactive", "close":
		b.Action, b.Ingestable, b.Desc = "close", true, "Close window"
	case "fullscreen":
		b.Action, b.Ingestable, b.Desc = "fullscreen", true, "Fullscreen"
	case "togglefloating":
		b.Action, b.Ingestable, b.Desc = "togglefloating", true, "Toggle floating"
	default:
		// keep the native dispatcher + args; apply writes the bind as it came.
		b.Action, b.Value, b.Ingestable = strings.ToLower(disp), args, false
		b.Desc = capitalize(strings.TrimSpace(disp + " " + args))
	}
	return b
}

// parseWindowRule maps a windowrule / windowrulev2 line onto the hub WindowRule.
// The first field is the rule (action plus optional argument); the rest are
// match specifiers (class:/title: for v2, a bare class regex for v1).
func parseWindowRule(val, raw string) importRule {
	f := splitCommaTrim(val)
	r := WindowRule{}
	if len(f) > 0 {
		toks := strings.Fields(f[0])
		if len(toks) > 0 {
			r.Action = strings.ToLower(toks[0])
			if len(toks) > 1 {
				r.Value = ruleValue(r.Action, toks[1:])
			}
		}
	}
	for _, spec := range f[1:] {
		switch {
		case strings.HasPrefix(spec, "class:"):
			r.Class = strings.TrimPrefix(spec, "class:")
		case strings.HasPrefix(spec, "title:"):
			r.Title = strings.TrimPrefix(spec, "title:")
		case !strings.Contains(spec, ":") && r.Class == "":
			r.Class = spec // v1 bare class regex
		}
	}
	return importRule{Raw: raw, Rule: r}
}

// ruleValue joins a rule's argument tokens into the form the hub WindowRule
// renderer expects: WxH for size, space-separated for move, first token else.
func ruleValue(action string, toks []string) string {
	switch action {
	case "size":
		return strings.Join(toks, "x")
	case "move":
		return strings.Join(toks, " ")
	default:
		return toks[0]
	}
}

// splitColonPath splits a Hyprland category path on ':' (decoration:rounding);
// a '.' stays part of the leaf key name (col.active_border).
func splitColonPath(key string) []string {
	var out []string
	for _, p := range strings.Split(key, ":") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// hyprConflicts flags every imported chord that clashes: shipped (shadows a
// Zenith bind) takes priority over duplicate (repeats an earlier imported chord),
// one row per chord, keyed by the stable norm.
func hyprConflicts(binds []importBind) []scanConflict {
	shipped := zenithShipped()
	seen := map[string]importBind{}
	done := map[string]bool{}
	out := []scanConflict{}
	for _, b := range binds {
		n := b.Norm
		if n == "" || done[n] {
			continue
		}
		if si, ok := shipped[n]; ok {
			out = append(out, scanConflict{
				Combo: b.Combo, Norm: n,
				Zenith: conflictZenith{Action: si.Action, Desc: si.Desc},
				Mine:   conflictMine{Raw: b.Raw, Desc: b.Desc},
				Kind:   "shipped",
			})
			done[n] = true
			continue
		}
		if first, ok := seen[n]; ok {
			out = append(out, scanConflict{
				Combo: b.Combo, Norm: n,
				Zenith: conflictZenith{Action: first.Action, Desc: first.Desc},
				Mine:   conflictMine{Raw: b.Raw, Desc: b.Desc},
				Kind:   "duplicate",
			})
			done[n] = true
			continue
		}
		seen[n] = b
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Norm < out[j].Norm })
	return out
}

// --- app scanners ------------------------------------------------------------

func scanHyprland(source string) (scanApp, bool) {
	path, ok := findConfig(source, "hypr", "hyprland.conf")
	if !ok {
		return scanApp{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return scanApp{}, false
	}
	items, hs := parseHyprland(string(b))
	return scanApp{
		ID: "hyprland", Name: "Hyprland", Present: true, Path: filepath.Dir(path),
		Tier:    "deep",
		Summary: fmt.Sprintf("%d keybinds, %d window rules, %d settings", len(hs.Binds), len(hs.Rules), len(hs.Raws)),
		Items:   items, Conflicts: hyprConflicts(hs.Binds),
	}, true
}

func scanKitty(source string) (scanApp, bool) {
	path, ok := findConfig(source, "kitty", "kitty.conf")
	if !ok {
		return scanApp{}, false
	}
	b, _ := os.ReadFile(path)
	items := confSettingItems(string(b), "#")
	return scanApp{
		ID: "kitty", Name: "kitty", Present: true, Path: filepath.Dir(path),
		Tier: "layer", Summary: fmt.Sprintf("%d settings", len(items)),
		Items: items, Conflicts: []scanConflict{},
	}, true
}

func scanFish(source string) (scanApp, bool) {
	path, ok := findConfig(source, "fish", "config.fish")
	if !ok {
		return scanApp{}, false
	}
	dir := filepath.Dir(path)
	items := []scanItem{{Kind: "file", Raw: "config.fish", Ingestable: false}}
	extra := 0
	for _, sub := range []string{"functions", "conf.d"} {
		entries, _ := os.ReadDir(filepath.Join(dir, sub))
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			items = append(items, scanItem{Kind: "file", Raw: filepath.Join(sub, e.Name()), Ingestable: false})
			extra++
		}
	}
	summary := "config.fish"
	if extra > 0 {
		summary = fmt.Sprintf("config.fish, %d extra files", extra)
	}
	return scanApp{
		ID: "fish", Name: "fish", Present: true, Path: dir,
		Tier: "layer", Summary: summary, Items: items, Conflicts: []scanConflict{},
	}, true
}

func scanFastfetch(source string) (scanApp, bool) {
	path, ok := findConfig(source, "fastfetch", "config.jsonc")
	if !ok {
		return scanApp{}, false
	}
	return scanApp{
		ID: "fastfetch", Name: "fastfetch", Present: true, Path: filepath.Dir(path),
		Tier: "layer", Summary: "fastfetch config",
		Items:     []scanItem{{Kind: "file", Raw: "config.jsonc", Ingestable: false}},
		Conflicts: []scanConflict{},
	}, true
}

// scanGeneric offers any other config dir the user brought as a drop-tier app:
// listed and copyable, never parsed.
func scanGeneric(source string) []scanApp {
	var out []scanApp
	seen := map[string]bool{}
	for _, root := range []string{source, filepath.Join(source, ".config")} {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() || knownApps[name] || strings.HasPrefix(name, ".") || seen[name] {
				continue
			}
			files := listRegularFiles(filepath.Join(root, name))
			if len(files) == 0 {
				continue
			}
			seen[name] = true
			items := make([]scanItem, 0, len(files))
			for _, f := range files {
				items = append(items, scanItem{Kind: "file", Raw: f, Ingestable: false})
			}
			out = append(out, scanApp{
				ID: name, Name: name, Present: true, Path: filepath.Join(root, name),
				Tier: "drop", Summary: fmt.Sprintf("%d files", len(files)),
				Items: items, Conflicts: []scanConflict{},
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// confSettingItems turns a line-oriented config (kitty) into one setting item
// per non-comment, non-blank line.
func confSettingItems(src, comment string) []scanItem {
	items := []scanItem{}
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, comment) {
			continue
		}
		items = append(items, scanItem{Kind: "setting", Raw: t, Ingestable: false})
	}
	return items
}

func listRegularFiles(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if rel, err := filepath.Rel(dir, p); err == nil {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// --- small text helpers ------------------------------------------------------

// collectVars gathers `$name = value` definitions for substitution into later
// lines (Hyprland's $mainMod idiom).
func collectVars(src string) map[string]string {
	vars := map[string]string{}
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(stripInlineComment(line))
		if !strings.HasPrefix(t, "$") {
			continue
		}
		if k, v, ok := splitKV(t); ok {
			vars[k] = v
		}
	}
	return vars
}

// substVars replaces variable references longest-name-first, so $mainMod is
// resolved before a hypothetical $main.
func substVars(s string, vars map[string]string) string {
	if len(vars) == 0 {
		return s
	}
	names := make([]string, 0, len(vars))
	for k := range vars {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	for _, n := range names {
		s = strings.ReplaceAll(s, n, vars[n])
	}
	return s
}

// stripInlineComment drops a trailing # comment that is not inside a quote, so a
// value keeps a literal # while a real comment is removed.
func stripInlineComment(line string) string {
	inS, inD := false, false
	for i, r := range line {
		switch r {
		case '\'':
			if !inD {
				inS = !inS
			}
		case '"':
			if !inS {
				inD = !inD
			}
		case '#':
			if !inS && !inD {
				return line[:i]
			}
		}
	}
	return line
}

func splitKV(t string) (string, string, bool) {
	i := strings.Index(t, "=")
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(t[:i]), strings.TrimSpace(t[i+1:]), true
}

func splitCommaTrim(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func splitNCommaTrim(s string, n int) []string {
	parts := strings.SplitN(s, ",", n)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isBindKeyword(lkey string) bool {
	if !strings.HasPrefix(lkey, "bind") {
		return false
	}
	for _, r := range lkey[len("bind"):] {
		if !strings.ContainsRune("lrenmtispd", r) {
			return false
		}
	}
	return true
}

func isExecKeyword(lkey string) bool {
	switch lkey {
	case "exec", "exec-once", "execr", "exec-shutdown":
		return true
	}
	return false
}
