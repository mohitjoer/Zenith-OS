package main

import (
	"os"
	"testing"
)

// TestMain keeps the whole suite off the real session, whatever environment it is
// started from: no Hyprland instance to reload, no shell daemon socket to patch,
// and no user config to write into. A test that needs its own paths sets them
// with t.Setenv, which is restored afterwards.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "zenith-hubd-test-")
	if err != nil {
		panic(err)
	}
	for k, v := range map[string]string{
		"HOME":            dir,
		"XDG_CONFIG_HOME": dir + "/config",
		"XDG_DATA_HOME":   dir + "/data",
		"XDG_STATE_HOME":  dir + "/state",
		"XDG_CACHE_HOME":  dir + "/cache",
		"XDG_RUNTIME_DIR": dir,
	} {
		_ = os.Setenv(k, v)
	}
	_ = os.Unsetenv("HYPRLAND_INSTANCE_SIGNATURE")
	_ = os.Unsetenv("WAYLAND_DISPLAY")
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
