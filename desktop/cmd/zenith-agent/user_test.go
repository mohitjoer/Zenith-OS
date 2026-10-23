package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiffUserConfig(t *testing.T) {
	base, cfg := t.TempDir(), t.TempDir()
	seedTree(t, base, map[string]string{
		"hyprland/hyprland.conf": "shipped",
		"ghostty/config":         "shipped",
		"zsh/zshrc":              "shipped",
	})
	seedTree(t, cfg, map[string]string{
		"hyprland/hyprland.conf": "user edited", // modified
		"ghostty/config":         "shipped",     // untouched
		"hyprland/user.conf":     "user override",
		// zsh/zshrc deleted
	})

	d, err := diffUserConfig(base, cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Modified) != 1 || d.Modified[0] != "hyprland/hyprland.conf" {
		t.Fatalf("modified = %v", d.Modified)
	}
	if len(d.Missing) != 1 || d.Missing[0] != "zsh/zshrc" {
		t.Fatalf("missing = %v", d.Missing)
	}
	if len(d.Overrides) != 1 || d.Overrides[0] != "hyprland/user.conf" {
		t.Fatalf("overrides = %v", d.Overrides)
	}
}

func TestUserDocBodyWithoutBase(t *testing.T) {
	t.Setenv(envName("CONFIG_BASE"), filepath.Join(t.TempDir(), "absent"))
	t.Setenv(envName("REPO"), "")
	t.Setenv("XDG_STATE_HOME", t.TempDir()) // no recorded dev checkout
	body := userDocBody()
	if !strings.Contains(body, "No shipped baseline found") {
		t.Fatalf("expected no-baseline note, got:\n%s", body)
	}
}

func TestUserDocBodyCleanBaseline(t *testing.T) {
	base := t.TempDir()
	cfg := t.TempDir()
	seedTree(t, base, map[string]string{"ghostty/config": "same"})
	seedTree(t, cfg, map[string]string{"zenith/ghostty/config": "same"})
	t.Setenv(envName("CONFIG_BASE"), base)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if body := userDocBody(); !strings.Contains(body, "matches the shipped Zenith baseline") {
		t.Fatalf("expected clean baseline note, got:\n%s", body)
	}
}

// dev checkout: no packaged base, but a deploy recorded a checkout, so its
// Hyprland tree is the baseline and divergences there are reported.
func TestUserDocBodyDevBaseline(t *testing.T) {
	checkout := t.TempDir()
	seedTree(t, checkout, map[string]string{
		"session/hyprland/binds.conf":    "shipped binds",
		"session/hyprland/hyprland.conf": "entry",
	})
	cfg := t.TempDir()
	seedTree(t, cfg, map[string]string{
		"zenith/hyprland/binds.conf":    "user edited binds", // modified
		"zenith/hyprland/hyprland.conf": "entry",             // untouched
		"zenith/hyprland/user.conf":     "override",          // override present
	})
	state := t.TempDir()
	seedTree(t, state, map[string]string{"zenith/repo": checkout + "\n"})
	t.Setenv(envName("CONFIG_BASE"), filepath.Join(t.TempDir(), "absent"))
	t.Setenv(envName("REPO"), "")
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	body := userDocBody()
	if !strings.Contains(body, "dev checkout") {
		t.Fatalf("expected dev-baseline note, got:\n%s", body)
	}
	if !strings.Contains(body, "hyprland/binds.conf") {
		t.Fatalf("expected modified bind listed, got:\n%s", body)
	}
	if !strings.Contains(body, "hyprland/user.conf") {
		t.Fatalf("expected override listed, got:\n%s", body)
	}
}
