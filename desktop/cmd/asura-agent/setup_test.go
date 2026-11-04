package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// setup only talks: it must name the key and the file, and say plainly what is
// there now, with and without an engine.
func TestSetupNamesTheKeyAndTheFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "cfg"))

	none := setupText(defaultConfig())
	for _, want := range []string{engineKey, "config.json", "no agent engine configured"} {
		if !strings.Contains(none, want) {
			t.Fatalf("setup text without an engine is missing %q:\n%s", want, none)
		}
	}

	cfg := defaultConfig()
	cfg.Engine.Command = []string{"sh", "-c", "exit 0"}
	if got := setupText(cfg); !strings.Contains(got, "now: sh -c exit 0") {
		t.Fatalf("setup text does not show the configured engine:\n%s", got)
	}
}
