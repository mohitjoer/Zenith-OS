package wm

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// The compositor state this backend reads: the outputs, for the display page. The
// window and workspace lists are the shell daemon's to serve, so the snapshot
// leaves them empty.

type hyprMonitor struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	Scale         float64 `json:"scale"`
	Focused       bool    `json:"focused"`
	Make          string  `json:"make"`
	Model         string  `json:"model"`
	PhysicalWidth int     `json:"physicalWidth"`
	// Hyprland can report disabled=true for a live DPMS-awake output, so the
	// state is derived from the real mode instead.
	Disabled              bool     `json:"disabled"`
	X                     int      `json:"x"`
	Y                     int      `json:"y"`
	RefreshRate           float64  `json:"refreshRate"`
	Transform             int      `json:"transform"`
	VRR                   bool     `json:"vrr"`
	AvailableModes        []string `json:"availableModes"`
	MirrorOf              string   `json:"mirrorOf"`
	ColorManagementPreset string   `json:"colorManagementPreset"`
	SdrBrightness         float64  `json:"sdrBrightness"`
	ActiveWorkspace       struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"activeWorkspace"`
}

func readMonitors() ([]hyprMonitor, error) {
	if !live() {
		return nil, fmt.Errorf("no live Hyprland session")
	}
	out, err := ctl("monitors", "all", "-j")
	if err != nil {
		return nil, err
	}
	var mons []hyprMonitor
	if err := json.Unmarshal(out, &mons); err != nil {
		return nil, err
	}
	return mons, nil
}

func readState() (Snapshot, error) {
	mons, err := readMonitors()
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		FocusedOutput: focusedName(mons),
		Outputs:       monitorOutputs(mons),
	}, nil
}

func monitorOutputs(mons []hyprMonitor) []Output {
	outputs := make([]Output, 0, len(mons))
	for _, m := range mons {
		modes := make([]string, 0, len(m.AvailableModes))
		for _, am := range m.AvailableModes {
			modes = append(modes, hyprModeString(am))
		}
		outputs = append(outputs, Output{
			Name:            m.Name,
			Width:           m.Width,
			Height:          m.Height,
			Scale:           m.Scale,
			Focused:         m.Focused,
			ActiveWorkspace: workspaceKey(m.ActiveWorkspace.ID, m.ActiveWorkspace.Name),
			Make:            m.Make,
			Model:           m.Model,
			PhysicalWidth:   m.PhysicalWidth,
			Disabled:        m.Width <= 0 || m.Height <= 0,
			X:               m.X,
			Y:               m.Y,
			Transform:       m.Transform,
			VRR:             m.VRR,
			Modes:           modes,
			Mode:            currentHyprMode(m, modes),
			Mirror:          mirrorName(m.MirrorOf),
			ColorMode:       hyprColorMode(m.ColorManagementPreset),
			SdrBrightness:   m.SdrBrightness,
		})
	}
	return outputs
}

// hyprModeString strips Hyprland's trailing "Hz" so an advertised mode reads as
// "WxH@rate" and round-trips cleanly back into a monitor line.
func hyprModeString(s string) string {
	s = strings.TrimSpace(s)
	if n := len(s); n >= 2 && strings.EqualFold(s[n-2:], "Hz") {
		s = strings.TrimSpace(s[:n-2])
	}
	return s
}

// currentHyprMode picks the advertised mode matching the monitor's live width,
// height and refresh, so the editor's current mode is one the picker also lists.
func currentHyprMode(m hyprMonitor, modes []string) string {
	prefix := fmt.Sprintf("%dx%d@", m.Width, m.Height)
	want := int(m.RefreshRate + 0.5)
	for _, s := range modes {
		if !strings.HasPrefix(s, prefix) {
			continue
		}
		if f, err := strconv.ParseFloat(s[len(prefix):], 64); err == nil && int(f+0.5) == want {
			return s
		}
	}
	return fmt.Sprintf("%dx%d@%d", m.Width, m.Height, want)
}

// mirrorName drops Hyprland's "none" sentinel so an un-mirrored output reads as
// the empty string the neutral shape uses.
func mirrorName(s string) string {
	if s == "" || s == "none" {
		return ""
	}
	return s
}

// hyprColorMode folds Hyprland's colour-management preset onto the colour mode
// the editor shows: an HDR preset is hdr, sRGB/auto/none are srgb, anything else
// is a wide-gamut profile.
func hyprColorMode(preset string) string {
	switch preset {
	case "hdr", "hdredid":
		return "hdr"
	case "", "srgb", "auto":
		return "srgb"
	}
	return "wide"
}

func focusedName(mons []hyprMonitor) string {
	for _, m := range mons {
		if m.Focused {
			return m.Name
		}
	}
	return ""
}

// The name is what a dispatcher accepts and what a user recognises; the id is the
// fallback for an unnamed workspace.
func workspaceKey(id int, name string) string {
	if name != "" {
		return name
	}
	if id == 0 {
		return ""
	}
	return strconv.Itoa(id)
}
