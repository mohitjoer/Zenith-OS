package wm

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// outputs is the write half of the display seam for Hyprland. The layout the
// display page built is applied to the running compositor with `keyword monitor`
// and kept in outputs-applied.json; settings.conf renders that file as monitor
// lines, so the layout outlives a reload and a login without a second generated
// file for the session configuration to source.
//
// Ryoku handed this to a bash engine (ryoku-monitor) that also autoscales from
// DPI. The scale snapping is kept here, because it is what stops Hyprland's
// "Invalid scale" overlay: Hyprland only accepts a scale that is a 1/120 multiple
// and divides the mode's pixel size into whole logical pixels.

// OutputLayout is one output's requested configuration for ApplyOutputs.
type OutputLayout struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	// Mode is "WxH@Hz"; empty asks for the panel's preferred (automatic) mode.
	Mode string `json:"mode,omitempty"`
	// Scale 0 asks the compositor to choose the scale itself.
	Scale     float64 `json:"scale,omitempty"`
	X         int     `json:"x,omitempty"`
	Y         int     `json:"y,omitempty"`
	Transform int     `json:"transform,omitempty"`
	VRR       bool    `json:"vrr,omitempty"`
	// Mirror is another output's name to clone this one onto. ColorMode
	// ("srgb"|"wide"|"hdr") and SdrBrightness are the colour pipeline.
	Mirror        string  `json:"mirror,omitempty"`
	ColorMode     string  `json:"colorMode,omitempty"`
	SdrBrightness float64 `json:"sdrBrightness,omitempty"`
}

// AppliedLayoutPath holds the last layout applied from the Hub.
func AppliedLayoutPath() string { return filepath.Join(zenithConfigDir(), "outputs-applied.json") }

func readAppliedLayout() []OutputLayout {
	b, err := os.ReadFile(AppliedLayoutPath())
	if err != nil {
		return nil
	}
	var layout []OutputLayout
	if json.Unmarshal(b, &layout) != nil {
		return nil
	}
	return layout
}

func applyOutputsFile(layoutPath string) (ApplyReport, error) {
	raw, err := os.ReadFile(layoutPath)
	if err != nil {
		return ApplyReport{}, err
	}
	var layout []OutputLayout
	if err := json.Unmarshal(raw, &layout); err != nil {
		return ApplyReport{}, fmt.Errorf("outputs: %w", err)
	}
	layout = cleanLayout(layout)
	if len(layout) == 0 {
		return ApplyReport{}, fmt.Errorf("outputs: the layout names no output")
	}

	body, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return ApplyReport{}, err
	}
	if err := atomicWrite(AppliedLayoutPath(), append(body, '\n'), 0o644); err != nil {
		return ApplyReport{}, err
	}
	// settings.conf carries the monitor lines, so it is rewritten with the new
	// layout in it; every other part of it is a function of the store.
	if err := RegenerateFromStore(); err != nil {
		return ApplyReport{}, err
	}

	rep := ApplyReport{
		Provider: ProviderHyprland,
		Written:  []string{filepath.Join(HyprConfigDir(), "settings.conf")},
	}
	if live() {
		known := liveModes()
		var cmds []string
		for _, o := range layout {
			cmds = append(cmds, "keyword monitor "+monitorSpec(o, known[o.Name]))
		}
		pushKeywords(cmds)
	}
	return rep, nil
}

// cleanLayout drops entries with no output name.
func cleanLayout(layout []OutputLayout) []OutputLayout {
	out := layout[:0:0]
	for _, o := range layout {
		o.Name = strings.TrimSpace(o.Name)
		if o.Name == "" || strings.ContainsAny(o.Name, ", \t\r\n") {
			continue
		}
		out = append(out, o)
	}
	return out
}

// genMonitors renders the applied layout as monitor lines for settings.conf.
func genMonitors() string {
	layout := readAppliedLayout()
	if len(layout) == 0 {
		return ""
	}
	known := map[string]modeSize{}
	if live() {
		known = liveModes()
	}
	var b strings.Builder
	for _, o := range cleanLayout(layout) {
		fmt.Fprintf(&b, "monitor = %s\n", monitorSpec(o, known[o.Name]))
	}
	return b.String()
}

// modeSize is an output's current pixel size, for snapping a scale when the
// layout leaves the mode on automatic.
type modeSize struct{ w, h int }

func liveModes() map[string]modeSize {
	out := map[string]modeSize{}
	mons, err := readMonitors()
	if err != nil {
		return out
	}
	for _, m := range mons {
		out[m.Name] = modeSize{m.Width, m.Height}
	}
	return out
}

// monitorSpec is the right-hand side of a monitor line (and the argument of
// `keyword monitor`) for one output.
func monitorSpec(o OutputLayout, current modeSize) string {
	if !o.Enabled {
		return o.Name + ", disable"
	}
	mode := strings.TrimSpace(o.Mode)
	if mode == "" {
		mode = "preferred"
	}
	w, h, _ := parseModeDims(mode)
	if w == 0 || h == 0 {
		w, h = current.w, current.h
	}
	scale := o.Scale
	switch {
	case scale <= 0:
		scale = 1
	case w > 0 && h > 0:
		scale = snapScale(w, h, scale)
	}
	parts := []string{o.Name, mode, fmt.Sprintf("%dx%d", o.X, o.Y), hlNum(scale)}
	if o.Transform != 0 {
		parts = append(parts, "transform", strconv.Itoa(o.Transform))
	}
	if o.VRR {
		parts = append(parts, "vrr", "1")
	}
	if o.Mirror != "" && o.Mirror != o.Name {
		parts = append(parts, "mirror", o.Mirror)
	}
	switch o.ColorMode {
	case "hdr", "wide":
		parts = append(parts, "cm", o.ColorMode)
		if o.ColorMode == "hdr" && o.SdrBrightness > 0 && o.SdrBrightness != 1 {
			parts = append(parts, "sdrbrightness", hlNum(o.SdrBrightness))
		}
	}
	return strings.Join(parts, ", ")
}

// parseModeDims pulls the physical width, height and rounded refresh from a
// "WxH@Hz" mode string. Zeroes for anything else, which is how "preferred" reads.
func parseModeDims(s string) (w, h, refresh int) {
	if s == "" {
		return 0, 0, 0
	}
	dims := s
	if at := strings.IndexByte(s, '@'); at >= 0 {
		dims = s[:at]
		if f, err := strconv.ParseFloat(strings.TrimSuffix(strings.ToLower(s[at+1:]), "hz"), 64); err == nil {
			refresh = int(f + 0.5)
		}
	}
	if x := strings.IndexByte(dims, 'x'); x >= 0 {
		w, _ = strconv.Atoi(dims[:x])
		h, _ = strconv.Atoi(dims[x+1:])
	}
	return w, h, refresh
}

// snapScale returns the scale Hyprland will accept nearest to target for a mode
// of w by h pixels. Hyprland's rule (src/helpers/Monitor.cpp) is an exact
// integer check: the scale is k/120 and both dimensions times 120 divide by k.
// k = 120 (1x) always qualifies, so the search never comes up empty.
func snapScale(w, h int, target float64) float64 {
	best, bestDiff := 120, math.MaxFloat64
	for k := 30; k <= 720; k++ {
		if (w*120)%k != 0 || (h*120)%k != 0 {
			continue
		}
		if d := math.Abs(float64(k)/120 - target); d < bestDiff {
			best, bestDiff = k, d
		}
	}
	return math.Round(float64(best)/120*10000) / 10000
}
