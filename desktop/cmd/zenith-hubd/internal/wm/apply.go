package wm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// apply, defaults and the live push: the write half of the seam. applyStore
// renders the neutral store into settings.conf and rebinds.conf and reports what
// it could not express; with preview it pushes the state to the running
// compositor instead and writes nothing.
//
// Ported from Ryoku's Hyprland provider (GPL-3.0).

// ApplyReport is what an apply returns. Unhonored names every setting the
// running compositor cannot express, so the Hub can mark it, doctor can report
// it and a switch can be previewed before it happens.
type ApplyReport struct {
	Provider  string      `json:"provider"`
	Written   []string    `json:"written,omitempty"`
	Unhonored []Unhonored `json:"unhonored,omitempty"`
	// ReloadNeeded is false on a compositor that watches its own config file.
	ReloadNeeded bool `json:"reloadNeeded,omitempty"`
}

// Unhonored.Reason is user copy, shown verbatim.
type Unhonored struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

// applyStore renders the neutral store at storePath. With preview it pushes the
// full option set live and writes no config; it still reports what it cannot
// honour, because an empty list there would say nothing would be lost.
func applyStore(storePath string, preview bool) (ApplyReport, error) {
	o := loadStore(storePath)

	if preview {
		if live() {
			pushKeywords(previewCommands(o))
		}
		return ApplyReport{
			Provider:  ProviderHyprland,
			Unhonored: unhonoredFor(o, storePath),
		}, nil
	}

	if err := WriteGenerated(o); err != nil {
		return ApplyReport{}, err
	}
	return ApplyReport{
		Provider: ProviderHyprland,
		Written: []string{
			filepath.Join(HyprConfigDir(), "settings.conf"),
			filepath.Join(HyprConfigDir(), "rebinds.conf"),
		},
		Unhonored:    unhonoredFor(o, storePath),
		ReloadNeeded: true,
	}, nil
}

// WriteGenerated writes settings.conf and rebinds.conf for an override set.
func WriteGenerated(o Overrides) error {
	follow := borderFollowsPalette(o)
	if err := writeOverlay("settings.conf", []byte(genSettings(o, follow))); err != nil {
		return err
	}
	return writeOverlay("rebinds.conf", renderRebinds(o))
}

// RegenerateFromStore re-renders both files from desktop.json, for a writer (the
// display page) that changed an input of settings.conf other than the store.
func RegenerateFromStore() error {
	return WriteGenerated(loadStore(DesktopStorePath()))
}

// pushKeywords applies hyprctl `keyword` commands in one batch. A batch is one
// round trip instead of a hundred, and keeps a slider drag from falling behind.
func pushKeywords(cmds []string) {
	if len(cmds) == 0 {
		return
	}
	for i, c := range cmds {
		cmds[i] = strings.ReplaceAll(c, ";", " ")
	}
	_, _ = ctl("--batch", strings.Join(cmds, " ; "))
}

// hyprlandDesktopFields are the desktop.* top-level keys this provider turns into
// Hyprland config. A key present in the store but absent here has no expression
// on Hyprland and is reported unhonored rather than silently dropped.
var hyprlandDesktopFields = map[string]bool{
	"appearance": true, "input": true, "cursor": true, "env": true,
	"windowRules": true, "appOverrides": true, "autostart": true,
	"keybinds": true, "keybindRebinds": true, "unbinds": true, "apps": true,
	"windows": true,
}

// unhonored names every setting the store carries that Hyprland 0.53 cannot
// express.
func unhonored(storePath string) []Unhonored { return unhonoredFor(loadStore(storePath), storePath) }

func unhonoredFor(o Overrides, storePath string) []Unhonored {
	var out []Unhonored
	if ns, ok := readNeutralStore(storePath); ok {
		for k := range ns.Desktop {
			if !hyprlandDesktopFields[k] {
				out = append(out, Unhonored{Key: "desktop." + k, Reason: "Hyprland has no setting for this."})
			}
		}
	}
	if o.Appearance.GlowEnabled {
		out = append(out, Unhonored{
			Key:    "desktop.appearance.glowEnabled",
			Reason: "Hyprland 0.53 has no window glow.",
		})
	}
	if o.Appearance.Layout == "scrolling" && !scrollingAvailable() {
		out = append(out, Unhonored{
			Key:    "desktop.appearance.layout",
			Reason: "The scrolling layout needs hyprland-plugin-hyprscrolling, which is not installed.",
		})
	}
	out = append(out, pluginUnhonored(o)...)
	return out
}

// probeAnimations reads the compositor's animation tree: every overridable,
// non-internal leaf as an AnimItem at its baseline, and the default bezier
// curves. Empty slices when nothing answers, so the defaults payload always
// carries them.
func probeAnimations() ([]AnimItem, []AnimCurve) {
	items, curves := []AnimItem{}, []AnimCurve{}
	if !live() {
		return items, curves
	}
	out, err := ctl("animations", "-j")
	if err != nil {
		return items, curves
	}
	var raw []json.RawMessage
	if json.Unmarshal(out, &raw) != nil || len(raw) < 2 {
		return items, curves
	}
	var anims []struct {
		Name       string  `json:"name"`
		Overridden bool    `json:"overridden"`
		Enabled    bool    `json:"enabled"`
		Speed      float64 `json:"speed"`
		Bezier     string  `json:"bezier"`
		Style      string  `json:"style"`
	}
	_ = json.Unmarshal(raw[0], &anims)
	for _, a := range anims {
		if !a.Overridden || strings.HasPrefix(a.Name, "__") {
			continue
		}
		items = append(items, AnimItem{Leaf: a.Name, Enabled: a.Enabled, Speed: a.Speed, Bezier: a.Bezier, Style: a.Style})
	}
	var cs []struct {
		Name string  `json:"name"`
		X0   float64 `json:"X0"`
		Y0   float64 `json:"Y0"`
		X1   float64 `json:"X1"`
		Y1   float64 `json:"Y1"`
	}
	_ = json.Unmarshal(raw[1], &cs)
	for _, c := range cs {
		curves = append(curves, AnimCurve{Name: c.Name, X0: c.X0, Y0: c.Y0, X1: c.X1, Y1: c.Y1})
	}
	return items, curves
}

// liveKbDefaults reads the running compositor's effective kb_* values so the
// unsaved baseline matches the real session; the hardcoded defaults stand when
// nothing answers.
func liveKbDefaults(in *Input) {
	if !live() {
		return
	}
	opts := []struct {
		name string
		dst  *string
	}{
		{"input:kb_layout", &in.KbLayout},
		{"input:kb_variant", &in.KbVariant},
		{"input:kb_options", &in.KbOptions},
	}
	for _, o := range opts {
		b, err := ctl("getoption", o.name, "-j")
		if err != nil {
			continue
		}
		var v struct {
			Str string `json:"str"`
		}
		if json.Unmarshal(b, &v) != nil {
			continue
		}
		if s := strings.TrimSpace(v.Str); s != "" {
			*o.dst = s
		}
	}
}

// GeneratedFiles are the files apply authors, relative to ~/.config: the live
// copies and the user_edits copies that keep them across a materialize. They are
// a pure function of the store, so a caller that restores a state with no store
// removes them instead of emitting defaults the user never had.
func GeneratedFiles() []string {
	return []string{
		"zenith/hyprland/settings.conf", "zenith/hyprland/rebinds.conf",
		"zenith/user_edits/hyprland/settings.conf", "zenith/user_edits/hyprland/rebinds.conf",
	}
}

// ClearGenerated drops what apply authored: the overlay copies, and any live copy
// that still carries the generated header, which becomes the empty seed again.
// A live file without the header is the packaged seed or the user's own and is
// left alone. The seed rather than nothing, because the packaged hyprland.conf
// sources these files and Hyprland treats a missing source as an error.
func ClearGenerated() {
	for _, name := range []string{"settings.conf", "rebinds.conf"} {
		_ = os.Remove(filepath.Join(userEditsHyprDir(), name))
		live := filepath.Join(HyprConfigDir(), name)
		if b, err := os.ReadFile(live); err == nil && strings.HasPrefix(string(b), generatedMarker) {
			_ = atomicWrite(live, []byte("# "+name+": written by Zenith Settings when you change a setting there.\n"), 0o644)
		}
	}
}

const generatedMarker = "# Generated by Zenith Settings"
