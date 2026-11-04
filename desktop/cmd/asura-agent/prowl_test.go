package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProwlRepoPrecedence(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("XDG_DATA_HOME", filepath.Join(h, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(h, ".local", "state"))
	t.Setenv(envName("VAULT"), "") // VaultDir derives from XDG_DATA_HOME
	t.Setenv(envName("REPO"), "")

	// No checkout, no mirror: nothing to report.
	if r := prowlRepo(); r != "" {
		t.Fatalf("want empty, got %q", r)
	}

	// A dev checkout that carries an index wins.
	checkout := filepath.Join(h, "checkout")
	if err := os.MkdirAll(filepath.Join(checkout, ".prowl"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envName("REPO"), checkout)
	if r := prowlRepo(); r != checkout {
		t.Fatalf("dev checkout should win, got %q", r)
	}

	// With no checkout in the resolution but an indexed mirror, the mirror wins.
	t.Setenv(envName("REPO"), "")
	mirror := sourceMirrorDir()
	if err := os.MkdirAll(filepath.Join(mirror, ".prowl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if r := prowlRepo(); r != mirror {
		t.Fatalf("mirror should win with no checkout, got %q", r)
	}
}
