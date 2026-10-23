package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"sync"
	"time"
)

// The compositor seam, reduced to what the wallpaper pipeline reads: the
// connected outputs (to size video and to say which display the picker opens
// on). Hyprland is the only compositor Zenith runs, so this is
// `hyprctl -j monitors` and nothing else.

// output is a compositor output. Width and Height are physical pixels; Scale is
// the compositor's scale factor, so the logical width is Width / Scale.
type output struct {
	Name    string
	Width   int
	Height  int
	Scale   float64
	Focused bool
}

type hyprMonitor struct {
	Name     string  `json:"name"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Scale    float64 `json:"scale"`
	Focused  bool    `json:"focused"`
	Disabled bool    `json:"disabled"`
}

// hyprctlMonitors runs hyprctl; a variable so a test can serve a fixed list.
var hyprctlMonitors = func(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, "hyprctl", "-j", "monitors").Output()
}

// parseMonitors turns `hyprctl -j monitors` into outputs. A disabled monitor
// has no mode, so it is not a place to paint.
func parseMonitors(raw []byte) ([]output, error) {
	var mons []hyprMonitor
	if err := json.Unmarshal(raw, &mons); err != nil {
		return nil, err
	}
	outs := make([]output, 0, len(mons))
	for _, m := range mons {
		if m.Disabled || m.Width <= 0 || m.Height <= 0 || m.Name == "" {
			continue
		}
		scale := m.Scale
		if scale <= 0 {
			scale = 1
		}
		outs = append(outs, output{m.Name, m.Width, m.Height, scale, m.Focused})
	}
	return outs, nil
}

// hyprlandRunning: the instance signature is in the environment of anything the
// session started, and hyprctl is the client that speaks to it.
func hyprlandRunning() bool {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return false
	}
	_, err := exec.LookPath("hyprctl")
	return err == nil
}

// compositorCaps answers the picker's wm.caps. The picker gates one control on
// it, the overview backdrop, which needs a compositor-native overview; Hyprland
// has none (the shell draws its own), so the list is empty.
func compositorCaps() map[string]interface{} {
	return map[string]interface{}{"name": "hyprland", "supports": []string{}}
}

// outputs is read per call: a hyprctl query is a few milliseconds and always
// current, so a monitor plugged in mid-session needs no watcher. A test pins the
// list by setting probed.
var outputs = &outputCache{}

type outputCache struct {
	mu     sync.Mutex
	outs   []output
	probed bool
}

func (c *outputCache) list() []output {
	c.mu.Lock()
	if c.probed {
		outs := c.outs
		c.mu.Unlock()
		return outs
	}
	c.mu.Unlock()
	return probeOutputs()
}

// probeOutputs is empty without a compositor: callers fall back to a default
// width.
func probeOutputs() []output {
	if !hyprlandRunning() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := hyprctlMonitors(ctx)
	if err != nil {
		return nil
	}
	outs, err := parseMonitors(raw)
	if err != nil {
		return nil
	}
	return outs
}
