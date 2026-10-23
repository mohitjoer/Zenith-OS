package wm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func configHome() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return base
}

func zenithConfigDir() string { return filepath.Join(configHome(), "zenith") }

// DesktopStorePath is the neutral settings store the Hub writes and this package
// reads.
func DesktopStorePath() string { return filepath.Join(zenithConfigDir(), "desktop.json") }

// HyprConfigDir is where the generated hyprlang lives: beside the hyprland.conf
// the session loads, so the overlay files can be sourced by a bare relative name.
func HyprConfigDir() string { return filepath.Join(zenithConfigDir(), "hyprland") }

// userEditsHyprDir is the hyprland slice of the user overlay tree. The generated
// files live here too so a `zenith materialize` that re-lays the packaged seed
// files cannot replace them; writeOverlay reflects the same bytes into the live
// directory so a reload picks them up at once.
func userEditsHyprDir() string { return filepath.Join(zenithConfigDir(), "user_edits", "hyprland") }

// UserEditsHyprDir is the overlay copy of HyprConfigDir.
func UserEditsHyprDir() string { return userEditsHyprDir() }

func shellStorePath() string { return filepath.Join(zenithConfigDir(), "shell.json") }

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func atomicWrite(path string, b []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// writeOverlay authors a generated file in the user_edits overlay (kept across
// updates) and reflects the same bytes into the live hyprland directory.
func writeOverlay(name string, body []byte) error {
	if err := atomicWrite(filepath.Join(userEditsHyprDir(), name), body, 0o644); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(HyprConfigDir(), name), body, 0o644)
}

// staticThemeActive reports whether shell.json names a fixed catalogue palette
// rather than the dynamic Default/Wallpaper variants, whose palette then drives
// the window border.
func staticThemeActive() bool {
	b, err := os.ReadFile(shellStorePath())
	if err != nil {
		return false
	}
	var s struct {
		Theme struct {
			Theme string `json:"theme"`
		} `json:"theme"`
	}
	if json.Unmarshal(b, &s) != nil {
		return false
	}
	switch s.Theme.Theme {
	case "", "Default", "Wallpaper":
		return false
	}
	return true
}

func parseFloat(s string, fallback float64) float64 {
	var f float64
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%g", &f); err != nil {
		return fallback
	}
	return f
}

// parseWxH: "1500x850" (or "1500 850" / "1500,850") -> two ints.
func parseWxH(s string) (int, int) {
	s = strings.NewReplacer("x", " ", "X", " ", ",", " ").Replace(s)
	var a, b int
	fmt.Sscan(s, &a, &b)
	return a, b
}

func themeStatePath() string { return filepath.Join(zenithConfigDir(), "theme.json") }

func getenv(k string) string { return os.Getenv(k) }
