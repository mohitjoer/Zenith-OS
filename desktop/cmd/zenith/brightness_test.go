package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"zenith/desktop/internal/backlight"
)

// A /sys with a screen at 10% of 1000 and a keyboard at level 2 of 3, written
// through the direct route the CLI falls back to when logind is not there.
func TestBrightnessCommand(t *testing.T) {
	root := t.TempDir()
	put := func(class, name, file, content string) {
		dir := filepath.Join(root, "class", class, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("backlight", "intel_backlight", "brightness", "100")
	put("backlight", "intel_backlight", "max_brightness", "1000")
	put("leds", "asus::kbd_backlight", "brightness", "2")
	put("leds", "asus::kbd_backlight", "max_brightness", "3")

	var out, stderr bytes.Buffer
	env := brightnessEnv{
		Sys: backlight.Sysfs{Root: root}, Chain: backlight.Chain{backlight.Direct{}},
		Out: &out, Err: &stderr, State: t.TempDir(),
	}
	level := func(class, name string) int {
		raw, _ := os.ReadFile(filepath.Join(root, "class", class, name, "brightness"))
		n, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
		return n
	}
	run := func(args ...string) int {
		out.Reset()
		return runBrightness(args, env)
	}
	screen := func() int { return level("backlight", "intel_backlight") }
	keys := func() int { return level("leds", "asus::kbd_backlight") }

	if code := run("set", "50"); code != exitOK || strings.TrimSpace(out.String()) != "50" || screen() != 500 {
		t.Fatalf("set 50: exit %d, printed %q, level %d", code, out.String(), screen())
	}
	// "-5" has to reach the parser as a value. As a flag it is a usage error and
	// the dim key does nothing.
	if code := run("set", "-5"); code != exitOK || screen() >= 500 {
		t.Fatalf("set -5: exit %d, level %d", code, screen())
	}
	dimmed := screen()
	if code := run("up"); code != exitOK || screen() <= dimmed {
		t.Fatalf("up: exit %d, %d -> %d", code, dimmed, screen())
	}

	// A keyboard steps whole levels, in either spelling of the flag.
	if code := run("down", "--device", "kbd"); code != exitOK || keys() != 1 {
		t.Fatalf("down kbd: exit %d, level %d", code, keys())
	}
	if code := run("up", "--device=kbd"); code != exitOK || keys() != 2 {
		t.Fatalf("up kbd: exit %d, level %d", code, keys())
	}

	if code := run("name"); code != exitOK || strings.TrimSpace(out.String()) != "intel_backlight" {
		t.Fatalf("name: exit %d, printed %q", code, out.String())
	}
	if code := run("get", "--device", "nope"); code != exitFail || !strings.Contains(stderr.String(), `no backlight named "nope"`) {
		t.Fatalf("unknown device: exit %d, stderr %q", code, stderr.String())
	}
	if code := run("set", "bright"); code != exitUsage {
		t.Fatalf("a word as a level: exit %d, want the usage code", code)
	}
}
