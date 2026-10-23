package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// stubCatalogue points the legend at a catalogue of the test's own and an
// empty config home, so keybinds() runs with no live compositor.
func stubCatalogue(t *testing.T, catalogue, store string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "binds.json")
	if err := os.WriteFile(path, []byte(catalogue), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZENITH_BINDS_JSON", path)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	if store != "" {
		if err := os.MkdirAll(filepath.Join(dir, "zenith"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "zenith", "desktop.json"), []byte(store), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func catByName(l legend, name string) *category {
	for i := range l.Categories {
		if l.Categories[i].Name == name {
			return &l.Categories[i]
		}
	}
	return nil
}

const testCatalogue = `{
  "categories": [{"id":"windows","label":"Windows"},{"id":"apps","label":"Apps"}],
  "binds": [
    {"id":"close","category":"windows","label":"Close window","chord":"SUPER + Q","kind":"wm","action":"killactive"},
    {"id":"terminal","category":"apps","label":"Terminal","hint":"Open a terminal","chord":"SUPER + Return","kind":"app","action":"ghostty"},
    {"id":"fullscreen","category":"windows","label":"Fullscreen","chord":"SUPER + F","kind":"wm","action":"fullscreen","argument":"0"},
    {"id":"volume-up","category":"apps","label":"Volume up","chord":"XF86AudioRaiseVolume","kind":"app","action":"zenith-volume"}
  ]
}`

// The catalogue's rows are grouped into sections in arrival order: a category
// opens the first time one of its rows appears and later rows join it, so two
// non-adjacent Windows rows are one section that keeps its place ahead of Apps.
// combo mirrors the row's Default (the shipped chord a rebind records over), not
// the effective Chord, and desc mirrors Label.
func TestKeybindsGroupsRowsInArrivalOrder(t *testing.T) {
	stubCatalogue(t, testCatalogue, `{"desktop":{"keybindRebinds":{"SUPER + Return":"SUPER + X"}}}`)

	l := keybinds()

	wantOrder := []string{"Windows", "Apps"}
	if len(l.Categories) != len(wantOrder) {
		t.Fatalf("got %d categories, want %d: %+v", len(l.Categories), len(wantOrder), l.Categories)
	}
	for i, n := range wantOrder {
		if l.Categories[i].Name != n {
			t.Errorf("category %d = %q, want %q", i, l.Categories[i].Name, n)
		}
	}

	win := catByName(l, "Windows")
	if win == nil || len(win.Binds) != 2 {
		t.Fatalf("Windows category malformed: %+v", win)
	}
	if win.Binds[0].ID != "close" || win.Binds[1].ID != "fullscreen" {
		t.Errorf("Windows binds out of order: %q, %q", win.Binds[0].ID, win.Binds[1].ID)
	}
	if win.Binds[0].Desc != "Close window" {
		t.Errorf("desc = %q, want the row label", win.Binds[0].Desc)
	}
	if win.Binds[0].Combo != "SUPER + Q" {
		t.Errorf("combo = %q, want the row default", win.Binds[0].Combo)
	}

	apps := catByName(l, "Apps")
	if apps == nil || len(apps.Binds) != 2 {
		t.Fatalf("Apps category malformed: %+v", apps)
	}
	term := apps.Binds[0]
	// combo is the default chord, not the rebound effective chord.
	if term.Combo != "SUPER + Return" {
		t.Errorf("combo = %q, want the default, not the rebound chord", term.Combo)
	}
	if term.Chord != "SUPER + X" {
		t.Errorf("chord = %q, want the effective rebound chord", term.Chord)
	}
	if term.Hint != "Open a terminal" {
		t.Errorf("hint = %q, want the row hint passed through", term.Hint)
	}
	if !term.Rebindable || term.Locked {
		t.Errorf("a plain chord must be rebindable and not locked: %+v", term.BindRow)
	}
	// A dedicated media key is locked and cannot be rebound.
	vol := apps.Binds[1]
	if vol.Rebindable || !vol.Locked {
		t.Errorf("an XF86 key must be locked and not rebindable: %+v", vol.BindRow)
	}
}

// The wire shape the cheatsheet and Hub page read: every BindRow field plus desc
// and combo, so a rename of one drops a field one of those surfaces expects.
func TestKeybindsJSONShape(t *testing.T) {
	stubCatalogue(t, testCatalogue, "")

	raw, err := json.Marshal(keybinds())
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Categories []struct {
			Name  string                       `json:"name"`
			Binds []map[string]json.RawMessage `json:"binds"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Categories) != 2 || len(got.Categories[0].Binds) != 2 {
		t.Fatalf("unexpected shape: %s", raw)
	}
	b := got.Categories[0].Binds[0]
	for _, field := range []string{"keys", "combo", "desc", "rebindable", "id", "kind", "locked", "default", "chord", "label", "category"} {
		if _, ok := b[field]; !ok {
			t.Errorf("emitted bind is missing the %q field: %s", field, raw)
		}
	}
}

// No honest legend to draw when the catalogue cannot be read, so the Hub gets an
// empty, non-nil set and shows its empty state rather than a partial sheet.
func TestKeybindsEmptyWithoutCatalogue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZENITH_BINDS_JSON", filepath.Join(dir, "missing.json"))
	t.Setenv("XDG_CONFIG_HOME", dir)

	l := keybinds()
	if l.Categories == nil {
		t.Fatal("categories must be an empty slice, not nil, so the JSON is [] not null")
	}
	if len(l.Categories) != 0 {
		t.Errorf("got %d categories without a catalogue, want 0", len(l.Categories))
	}
}
