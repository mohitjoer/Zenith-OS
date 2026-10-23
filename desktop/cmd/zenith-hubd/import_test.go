package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zenith/desktop/cmd/zenith-hubd/internal/wm"
)

// Zenith's shipped legend for the test: the bind catalogue. SUPER+Q (close) is
// the chord the imported config shadows.
const fxBinds = `{"categories":[{"id":"w","label":"Windows"},{"id":"a","label":"Apps"}],"binds":[
 {"id":"close","category":"w","label":"Close window","chord":"SUPER + Q","kind":"wm","action":"killactive"},
 {"id":"fullscreen","category":"w","label":"Fullscreen","chord":"SUPER + F","kind":"wm","action":"fullscreen","argument":"0"},
 {"id":"terminal","category":"a","label":"Terminal","chord":"SUPER + Return","kind":"app","action":"zenith-app terminal"}]}`

// A migrating user's native hyprland.conf: a shadowing bind (Q), a couple of
// clean ingestable binds, a self-duplicate (G twice), a native
// non-ingestable bind (X, movetoworkspace), one Hyprland would reject (Z), two
// window rules, and raw settings (env / exec-once / monitor / a section).
const fxHypr = `$mainMod = SUPER

# apps
bind = $mainMod, Q, exec, foot           # shadows Zenith's close
bind = $mainMod SHIFT, E, exec, thunar   # clean
bind = $mainMod, T, togglefloating       # ingestable dispatcher
bind = $mainMod, G, exec, gimp           # duplicate 1
bind = $mainMod, G, exec, inkscape       # duplicate 2
bind = $mainMod, X, movetoworkspace, 3   # non-ingestable, native
bind = $mainMod, Z, somethingweird       # non-ingestable, rejected by Hyprland

# window rules
windowrule = float, ^(pavucontrol)$
windowrulev2 = opacity 0.9, class:^(Alacritty)$

# raw settings
env = GTK_THEME, Adwaita-dark
exec-once = swww init
monitor = eDP-1, 1920x1080@60, 0x0, 1
general {
    gaps_in = 3
}
`

const fxKitty = `# kitty
font_family JetBrains Mono
font_size 12
background_opacity 0.9
`

const fxFish = `# fish
set -gx EDITOR nvim
alias ll 'ls -la'
`

const fxFastfetch = `{
  // fastfetch
  "logo": { "source": "arch" },
  "modules": ["title", "os"]
}
`

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// importHome seeds a temp config home with the shipped legend (and a couple of
// pre-existing user-include files) and returns it. HOME + XDG_CONFIG_HOME point
// here, so nothing touches the real config.
func importHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	catalogue := filepath.Join(home, "binds.json")
	mustWrite(t, catalogue, fxBinds)
	t.Setenv("ZENITH_BINDS_JSON", catalogue)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	mustWrite(t, filepath.Join(home, ".config", "zenith", "hyprland", "user.conf"), "# user overrides\n")
	mustWrite(t, filepath.Join(home, ".config", "kitty", "user.conf"), "# kitty user\n")
	return home
}

// importSource lays out a dotfiles tree to import from.
func importSource(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "hypr", "hyprland.conf"), fxHypr)
	mustWrite(t, filepath.Join(src, "kitty", "kitty.conf"), fxKitty)
	mustWrite(t, filepath.Join(src, "fish", "config.fish"), fxFish)
	mustWrite(t, filepath.Join(src, "fastfetch", "config.jsonc"), fxFastfetch)
	return src
}

func findApp(r scanResult, id string) *scanApp {
	for i := range r.Apps {
		if r.Apps[i].ID == id {
			return &r.Apps[i]
		}
	}
	return nil
}

func findConflict(a *scanApp, norm string) *scanConflict {
	for i := range a.Conflicts {
		if a.Conflicts[i].Norm == norm {
			return &a.Conflicts[i]
		}
	}
	return nil
}

func TestScanResultModel(t *testing.T) {
	importHome(t)
	src := importSource(t)
	r := scanSource(src)

	for _, id := range []string{"hyprland", "kitty", "fish", "fastfetch"} {
		if findApp(r, id) == nil {
			t.Fatalf("app %q not detected", id)
		}
	}
	if len(r.Apps) != 4 {
		t.Fatalf("got %d apps, want 4 (no generic): %+v", len(r.Apps), r.Apps)
	}

	h := findApp(r, "hyprland")
	if h.Tier != "deep" {
		t.Errorf("hyprland tier = %q, want deep", h.Tier)
	}
	// 7 binds, 2 rules, 2 conflicts.
	binds, rules := 0, 0
	for _, it := range h.Items {
		switch it.Kind {
		case "bind":
			binds++
		case "windowrule":
			rules++
		}
	}
	if binds != 7 || rules != 2 {
		t.Errorf("hyprland items: %d binds, %d rules; want 7, 2", binds, rules)
	}
	if len(h.Conflicts) != 2 {
		t.Fatalf("got %d conflicts, want 2: %+v", len(h.Conflicts), h.Conflicts)
	}

	shadow := findConflict(h, "q+super")
	if shadow == nil || shadow.Kind != "shipped" {
		t.Fatalf("q+super conflict = %+v, want kind shipped", shadow)
	}
	if shadow.Zenith.Action != "close" {
		t.Errorf("shadow zenith.action = %q, want close", shadow.Zenith.Action)
	}
	if !strings.Contains(shadow.Mine.Raw, "exec, foot") {
		t.Errorf("shadow mine.raw = %q, want the foot bind", shadow.Mine.Raw)
	}
	if shadow.Combo != "SUPER + Q" {
		t.Errorf("shadow combo = %q, want SUPER + Q", shadow.Combo)
	}

	dup := findConflict(h, "g+super")
	if dup == nil || dup.Kind != "duplicate" {
		t.Fatalf("g+super conflict = %+v, want kind duplicate", dup)
	}
	if !strings.Contains(dup.Mine.Raw, "inkscape") {
		t.Errorf("dup mine.raw = %q, want the second (inkscape) bind", dup.Mine.Raw)
	}

	// a non-ingestable bind is still reported, just not ingestable.
	var sawRawBind bool
	for _, it := range h.Items {
		if it.Kind == "bind" && strings.Contains(it.Raw, "movetoworkspace") {
			sawRawBind = true
			if it.Ingestable {
				t.Error("movetoworkspace bind should not be ingestable")
			}
		}
	}
	if !sawRawBind {
		t.Error("non-ingestable bind missing from items")
	}

	if k := findApp(r, "kitty"); k.Tier != "layer" || len(k.Items) != 3 {
		t.Errorf("kitty: tier=%q items=%d, want layer/3", k.Tier, len(k.Items))
	}
	if f := findApp(r, "fish"); f.Tier != "layer" {
		t.Errorf("fish tier = %q, want layer", f.Tier)
	}
	if ff := findApp(r, "fastfetch"); ff.Tier != "layer" {
		t.Errorf("fastfetch tier = %q, want layer", ff.Tier)
	}
}

// scanning the same tree twice yields byte-identical JSON.
func TestScanIdempotent(t *testing.T) {
	importHome(t)
	src := importSource(t)
	a, _ := json.Marshal(scanSource(src))
	b, _ := json.Marshal(scanSource(src))
	if string(a) != string(b) {
		t.Fatalf("re-scan differs:\n%s\n%s", a, b)
	}
}

func allApps() map[string]appDecision {
	return map[string]appDecision{
		"hyprland":  {Include: true},
		"kitty":     {Include: true},
		"fish":      {Include: true},
		"fastfetch": {Include: true},
	}
}

func TestApplyIngestsAndUnbinds(t *testing.T) {
	importHome(t)
	src := importSource(t)

	dec := decisions{
		Source:    src,
		Apps:      allApps(),
		Conflicts: map[string]json.RawMessage{"q+super": json.RawMessage(`"mine"`), "g+super": json.RawMessage(`"zenith"`)},
	}
	res, err := applyImport(dec)
	if err != nil {
		t.Fatal(err)
	}
	// mine foot + thunar + togglefloating + gimp(first dup) = 4; inkscape dropped.
	if res.BindsIngested != 4 {
		t.Errorf("bindsIngested = %d, want 4", res.BindsIngested)
	}
	if res.RulesIngested != 2 {
		t.Errorf("rulesIngested = %d, want 2", res.RulesIngested)
	}
	if res.Unbinds != 1 {
		t.Errorf("unbinds = %d, want 1", res.Unbinds)
	}

	// the store carries the ingested binds, rules and the unbind of the shadowed
	// chord; the provider renders settings.conf from it.
	m := readJSONMap(desktopStorePath())
	d, _ := m["desktop"].(map[string]any)
	blob, _ := json.Marshal(d)
	s := string(blob)
	for _, want := range []string{
		`"keys":"SUPER + Q"`, `"value":"foot"`,
		`"keys":"SUPER + SHIFT + E"`, `"value":"thunar"`,
		`"action":"togglefloating"`,
		`"keys":"SUPER + G"`, `"value":"gimp"`,
		`"SUPER + Q"`,
		`pavucontrol`, `"action":"opacity"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("store missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "inkscape") {
		t.Error("dropped duplicate (inkscape) leaked into the store")
	}

	// the non-ingestable bind + raw settings layered into user.conf.
	ul, err := os.ReadFile(filepath.Join(wm.HyprConfigDir(), "user.conf"))
	if err != nil {
		t.Fatal(err)
	}
	u := string(ul)
	if !strings.Contains(u, "# >>> zenith-import "+res.Ts+" >>>") || !strings.Contains(u, "# <<< zenith-import <<<") {
		t.Errorf("user.conf missing the marked block:\n%s", u)
	}
	for _, want := range []string{
		"env = GTK_THEME, Adwaita-dark",
		"exec-once = swww init",
		"monitor = eDP-1, 1920x1080@60, 0x0, 1",
		"bind = SUPER, X, movetoworkspace, 3",
		"general:gaps_in = 3",
		"# zenith-import (port by hand: somethingweird): bind = $mainMod, Z, somethingweird",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("user.conf missing %q:\n%s", want, u)
		}
	}
	// the overlay copy carries the same bytes, so a materialize cannot drop them.
	if ov, _ := os.ReadFile(filepath.Join(wm.UserEditsHyprDir(), "user.conf")); string(ov) != u {
		t.Errorf("user_edits copy differs from the live user.conf")
	}
	if len(res.Unresolved) != 1 || !strings.Contains(res.Unresolved[0], "somethingweird") {
		t.Errorf("unresolved = %v, want the somethingweird bind", res.Unresolved)
	}
	if !confContentOK(ul) {
		t.Errorf("generated user.conf is not valid hyprlang:\n%s", u)
	}

	// the layer includes got the imported config in a marked block.
	kitty, _ := os.ReadFile(filepath.Join(configHome(), "kitty", "user.conf"))
	if !strings.Contains(string(kitty), "font_family JetBrains Mono") || !strings.Contains(string(kitty), "# >>> zenith-import ") {
		t.Errorf("kitty/user.conf missing imported block:\n%s", kitty)
	}
}

// apply then undo restores every touched file to its exact pre-import bytes.
func TestApplyUndoRoundTrip(t *testing.T) {
	home := importHome(t)
	src := importSource(t)
	cfg := filepath.Join(home, ".config")

	before := snapshotTree(t, cfg)

	dec := decisions{
		Source:    src,
		Apps:      allApps(),
		Conflicts: map[string]json.RawMessage{"q+super": json.RawMessage(`"mine"`)},
	}
	res, err := applyImport(dec)
	if err != nil {
		t.Fatal(err)
	}
	if after := snapshotTree(t, cfg); mapsEqual(before, after) {
		t.Fatal("apply changed nothing")
	}

	undo, err := undoImport("")
	if err != nil {
		t.Fatal(err)
	}
	if undo.Ts != res.Ts {
		t.Errorf("undo ts = %q, want %q", undo.Ts, res.Ts)
	}
	after := snapshotTree(t, cfg)
	if !mapsEqual(before, after) {
		for k, v := range before {
			if after[k] != v {
				t.Errorf("file %q not restored:\nbefore=%q\nafter =%q", k, v, after[k])
			}
		}
		for k := range after {
			if _, ok := before[k]; !ok {
				t.Errorf("file %q left behind after undo", k)
			}
		}
	}
}

// snapshotTree maps every file under root (except the import-backups store) to
// its bytes, so a round-trip can be checked for byte identity.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	m := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.Contains(p, "import-backups") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		m[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestNormCombo(t *testing.T) {
	cases := map[string]string{
		"SUPER + Q":         "q+super",
		"Q + SUPER":         "q+super",
		"SUPER + SHIFT + E": "e+shift+super",
		"CTRL + ALT + Del":  "alt+ctrl+del",
	}
	for in, want := range cases {
		if got := normCombo(in); got != want {
			t.Errorf("normCombo(%q) = %q, want %q", in, got, want)
		}
	}
}

// a conflict defaulting to "zenith" (absent from decisions) drops the imported
// bind and adds no unbind, so Zenith's shipped chord is untouched.
func TestApplyDefaultKeepsZenith(t *testing.T) {
	importHome(t)
	src := importSource(t)
	dec := decisions{Source: src, Apps: map[string]appDecision{"hyprland": {Include: true}}}
	res, err := applyImport(dec)
	if err != nil {
		t.Fatal(err)
	}
	if res.Unbinds != 0 {
		t.Errorf("unbinds = %d, want 0 (default keeps Zenith)", res.Unbinds)
	}
	d, _ := readJSONMap(desktopStorePath())["desktop"].(map[string]any)
	blob, _ := json.Marshal(d)
	if strings.Contains(string(blob), `"value":"foot"`) {
		t.Error("the shadowing foot bind was ingested despite defaulting to zenith")
	}
}

// generic settings are flattened to their colon path so they stand alone in
// user.conf: sections nest into the key, a dotted key stays whole.
func TestFlattenSettings(t *testing.T) {
	_, hs := parseHyprland("general {\n    gaps_in = 6\n    border_size = 3\n}\n" +
		"decoration {\n    col.active_border = rgb(ff0000)\n    blur {\n        enabled = true\n    }\n}\n")
	var out []string
	for _, rl := range hs.Raws {
		out = append(out, rl.Conf)
	}
	joined := strings.Join(out, "\n")
	for _, want := range []string{
		"general:gaps_in = 6",
		"general:border_size = 3",
		"decoration:col.active_border = rgb(ff0000)",
		"decoration:blur:enabled = true",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}

// a bind outside the Hub's keybind model is carried natively with its variables
// resolved, and nativeBindOK refuses a dispatcher Hyprland does not know.
func TestNativeBinds(t *testing.T) {
	_, hs := parseHyprland("$m = SUPER\nbind = $m SHIFT, right, movewindow, r\nbindl = , XF86AudioMute, exec, mute\n")
	if len(hs.Binds) != 2 {
		t.Fatalf("got %d binds", len(hs.Binds))
	}
	if got, want := hs.Binds[0].Native, "bind = SUPER SHIFT, right, movewindow, r"; got != want {
		t.Errorf("Native = %q, want %q", got, want)
	}
	if got, want := hs.Binds[1].Native, "bindl = , XF86AudioMute, exec, mute"; got != want {
		t.Errorf("Native keeps the bind flags: got %q, want %q", got, want)
	}
	if _, err := exec.LookPath("Hyprland"); err == nil && nativeBindOK("bind = SUPER, Y, somethingweird, 1") {
		t.Error("a dispatcher Hyprland rejects must not be carried into user.conf")
	}
}

// "use mine" on a shadow whose dispatcher cannot be translated must NOT unbind
// the shipped chord (that would leave a dead key); it keeps Zenith's and records
// the imported bind as unresolved instead.
func TestApplyMineUnknownDispatcherNoUnbind(t *testing.T) {
	importHome(t)
	src := sourceWithHypr(t, "$mainMod = SUPER\nbind = $mainMod, Q, somethingweird\n")
	dec := decisions{
		Source:    src,
		Apps:      map[string]appDecision{"hyprland": {Include: true}},
		Conflicts: map[string]json.RawMessage{"q+super": json.RawMessage(`"mine"`)},
	}
	res, err := applyImport(dec)
	if err != nil {
		t.Fatal(err)
	}
	if res.Unbinds != 0 {
		t.Errorf("unbinds = %d, want 0 (untranslatable mine must not unbind)", res.Unbinds)
	}
	if len(res.Unresolved) != 1 {
		t.Errorf("unresolved = %v, want the somethingweird bind", res.Unresolved)
	}
	// no hypr overrides changed, so the store records no unbind.
	if store, err := os.ReadFile(desktopStorePath()); err == nil && strings.Contains(string(store), "unbinds") {
		t.Errorf("store recorded an unbind for a dead key:\n%s", store)
	}
	u, _ := os.ReadFile(filepath.Join(wm.HyprConfigDir(), "user.conf"))
	if !strings.Contains(string(u), "port by hand: somethingweird") {
		t.Errorf("user.conf missing the port-by-hand note:\n%s", u)
	}
}

func sourceWithHypr(t *testing.T, conf string) string {
	t.Helper()
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "hypr", "hyprland.conf"), conf)
	return src
}
