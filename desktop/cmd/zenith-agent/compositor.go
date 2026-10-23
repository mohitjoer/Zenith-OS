package main

import (
	"encoding/json"
	"fmt"
)

// compositorConfigDir is the Hyprland config directory under ~/.config/zenith,
// where the Hub writes settings.conf and rebinds.conf and the user keeps user.conf.
const compositorConfigDir = "hyprland"

// monitorRows describes the outputs the compositor reports, one line each. It
// reads `hyprctl monitors -j` once and returns nil when Hyprland is not running.
func monitorRows() []string {
	out, ok := probe(3, "hyprctl", "monitors", "-j")
	if !ok {
		return nil
	}
	var ms []struct {
		Name   string  `json:"name"`
		Width  int     `json:"width"`
		Height int     `json:"height"`
		Scale  float64 `json:"scale"`
	}
	if json.Unmarshal([]byte(out), &ms) != nil {
		return nil
	}
	rows := make([]string, 0, len(ms))
	for _, m := range ms {
		rows = append(rows, fmt.Sprintf("%s: %dx%d, scale %g", m.Name, m.Width, m.Height, m.Scale))
	}
	return rows
}
