package main

import (
	"context"
	"testing"
)

func TestPluginProviderLoadFeatured(t *testing.T) {
	p := pluginProvider{}
	items, _, err := p.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error loading plugins: %v", err)
	}
	// A checkout without catalog/plugins (the ASURA-OS embedded tree before it
	// carried desktop/catalog) shows an empty shelf (plus any installed
	// plugin), which is the correct answer there. With the catalogue present,
	// the featured shelf below is asserted on in both trees.
	if catalogDirFor("plugins") == "" {
		for _, it := range items {
			if it.ID == "" || it.Category != "plugins" {
				t.Errorf("bad plugin item: %+v", it)
			}
		}
		return
	}
	if len(items) == 0 {
		t.Fatalf("expected at least 1 featured plugin, got 0")
	}
	foundPomodoro := false
	for _, it := range items {
		if it.ID == "pomodoro" {
			foundPomodoro = true
			if it.Name == "" || it.Summary == "" {
				t.Errorf("plugin item missing name or summary: %+v", it)
			}
		}
	}
	if !foundPomodoro {
		t.Errorf("expected featured plugin 'pomodoro' to be present")
	}
}

func TestThemeProviderLoad(t *testing.T) {
	p := themeProvider{}
	items, _, err := p.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error loading themes: %v", err)
	}
	if len(items) < 3 {
		t.Fatalf("expected at least 3 themes, got %d", len(items))
	}
	for _, it := range items {
		if it.Accent == "" || it.Surface == "" {
			t.Errorf("theme missing accent or surface color: %+v", it)
		}
	}
}
