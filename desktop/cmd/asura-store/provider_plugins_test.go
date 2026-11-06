package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPluginIDGrammar(t *testing.T) {
	for _, id := range []string{"a", "pomodoro", "weather-radar", "2fast"} {
		if !pluginIDRe.MatchString(id) {
			t.Errorf("%q should match the plugin id grammar", id)
		}
	}
	for _, id := range []string{"", "-lead", "UPPER", "has space", "snake_case", "esc/slash", "../escape"} {
		if pluginIDRe.MatchString(id) {
			t.Errorf("%q should not match the plugin id grammar", id)
		}
	}
}

func TestThemeIDGrammar(t *testing.T) {
	for _, id := range []string{"a", "asura-dark", "mocha.luna", "n-2_o.1"} {
		if !themeIDRe.MatchString(id) {
			t.Errorf("%q should match the theme id grammar", id)
		}
	}
	for _, id := range []string{"", ".hidden", "-lead", "UPPER", "has space", "esc/slash", "../escape"} {
		if themeIDRe.MatchString(id) {
			t.Errorf("%q should not match the theme id grammar", id)
		}
	}
}

// TestPluginProviderInstalled isolates the provider from the machine: with
// XDG_DATA_HOME pointed at a temp tree, the shelf is exactly what the
// environment says it is -- an installed folder lists, an empty install root
// lists nothing, and no catalogue fiction appears either way.
func TestPluginProviderInstalled(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))

	p := pluginProvider{}
	items, _, err := p.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error loading plugins: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("empty install root should list no plugins, got %+v", items)
	}

	if err := os.MkdirAll(filepath.Join(tmp, "asura", "plugins", "realwidget"), 0o755); err != nil {
		t.Fatal(err)
	}
	items, _, err = p.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error loading plugins: %v", err)
	}
	if len(items) != 1 || items[0].ID != "realwidget" || !items[0].Installed {
		t.Fatalf("installed folder 'realwidget' should be the shelf, got %+v", items)
	}
}

// TestThemeProviderInstalledAndActive does the same for themes, and adds the
// activation pair: a theme whose files are on disk reads installed, the one
// the current-theme file names reads active, and nothing invented by the
// binary ever appears.
func TestThemeProviderInstalledAndActive(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	cfg := filepath.Join(tmp, "config")
	t.Setenv("XDG_CONFIG_HOME", cfg)

	if err := os.MkdirAll(filepath.Join(tmp, "asura", "themes", "obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfg, "asura", "current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "asura", "current", "theme.name"), []byte("obsidian\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := themeProvider{}
	items, _, err := p.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error loading themes: %v", err)
	}
	if len(items) != 1 || items[0].ID != "obsidian" {
		t.Fatalf("the on-disk theme should be the shelf, got %+v", items)
	}
	if !items[0].Installed || !items[0].Active {
		t.Fatalf("obsidian should read installed and active, got %+v", items[0])
	}
	for _, it := range items {
		switch it.ID {
		case "asura-dark", "asura-cyber", "catppuccin-mocha", "tokyo-night", "nord-aurora":
			t.Errorf("phantom theme %q is offered nowhere in the shell and must not be listed", it.ID)
		}
	}

	// A catalogue-only id without files cannot be activated...
	if err := p.Install(context.Background(), "not-on-disk"); err == nil {
		t.Errorf("activating a theme with no files must refuse")
	}
	// ...and the legacy bare theme.name must not override the current-theme
	// file: with both present, the current one wins.
	if err := os.MkdirAll(filepath.Join(tmp, ".config", "asura"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".config", "asura", "theme.name"), []byte("elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	items, _, err = p.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error re-loading themes: %v", err)
	}
	if len(items) != 1 || items[0].ID != "obsidian" || !items[0].Active {
		t.Fatalf("the current-theme file should win over the legacy one, got %+v", items)
	}
}
