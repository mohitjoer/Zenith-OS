package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain keeps every test off the real session: no compositor, no gsettings,
// no picker process, and no shipped wallpaper directory.
func TestMain(m *testing.M) {
	os.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	os.Setenv("ZENITH_WALLPAPER_HEADLESS", "1")
	gsettingsGet = func(string, string) string { return "" }
	os.Exit(m.Run())
}

func TestParseMonitors(t *testing.T) {
	raw := []byte(`[{"name":"eDP-1","width":2560,"height":1440,"scale":1.6,"focused":true},
		{"name":"HDMI-A-1","width":1920,"height":1080,"scale":0,"focused":false},
		{"name":"DP-9","width":0,"height":0,"scale":1,"disabled":true}]`)
	got, err := parseMonitors(raw)
	if err != nil || len(got) != 2 {
		t.Fatalf("parseMonitors = %+v, %v; want the two enabled outputs", got, err)
	}
	if !got[0].Focused || got[0].Width != 2560 || got[1].Scale != 1 {
		t.Fatalf("fields wrong: %+v", got)
	}
}

// A fresh account starts on the wallpaper the settings name, in the colour
// scheme's own key first, and ignores one that is not an image on disk.
func TestSettingsWallpaper(t *testing.T) {
	dir := t.TempDir()
	dark, light := filepath.Join(dir, "dark one.png"), filepath.Join(dir, "light.jpg")
	for _, f := range []string{dark, light} {
		writeE2EPNG(t, f)
	}
	vals := map[string]string{
		"color-scheme":        "'prefer-dark'",
		"wallpaper-path":      "'file://" + light + "'",
		"wallpaper-path-dark": "'file://" + filepath.Join(dir, "dark%20one.png") + "'",
	}
	t.Setenv("ZENITH_WALLPAPER_HEADLESS", "")
	gsettingsGet = func(_, key string) string { return vals[key] }
	t.Cleanup(func() { gsettingsGet = func(string, string) string { return "" } })

	if got := settingsWallpaper(); got != dark {
		t.Fatalf("dark scheme = %q, want %q", got, dark)
	}
	vals["color-scheme"] = "'default'"
	if got := settingsWallpaper(); got != light {
		t.Fatalf("light scheme = %q, want %q", got, light)
	}
	vals["wallpaper-path"] = "''"
	vals["wallpaper-path-dark"] = "'file:///nonexistent.png'"
	if got := settingsWallpaper(); got != "" {
		t.Fatalf("missing file must yield \"\", got %q", got)
	}
}
