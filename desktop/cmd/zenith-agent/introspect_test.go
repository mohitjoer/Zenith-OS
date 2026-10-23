package main

import (
	"os"
	"path/filepath"
	"testing"
)

func seedFiles(t *testing.T, root string, files map[string]string) {
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

// Skills come from two places only: what Zenith ships and what lives in the
// vault's skills/. A name found twice is listed once, shipped first.
func TestSkillsReportShippedAndYours(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv(envName("REPO"), "")
	t.Setenv(envName("VAULT"), filepath.Join(h, "vault"))
	shipped := filepath.Join(h, "shipped")
	t.Setenv(envName("SKILLS"), shipped)
	seedFiles(t, shipped, map[string]string{
		"zenith/SKILL.md":       "---\nname: zenith\ndescription: Customize the desktop\nversion: 1.0.0\n---\nbody",
		"zenith/bar.md":         "a sibling doc is not a skill",
		"not-a-skill/README.md": "no SKILL.md here",
	})
	seedFiles(t, filepath.Join(h, "vault", "skills"), map[string]string{
		"my-fix/SKILL.md": "---\nname: my-fix\ndescription: Grown by the agent\n---\nbody",
		"zenith/SKILL.md": "---\nname: zenith\ndescription: shadow copy\n---\nbody",
	})
	rep := SkillsReportNow()
	if rep.Counts["shipped"] != 1 || rep.Counts["yours"] != 1 {
		t.Fatalf("counts %+v", rep.Counts)
	}
	var sawShipped, sawYours bool
	for _, c := range rep.Categories {
		for _, s := range c.Skills {
			if s.Name == "zenith" && c.Name == "shipped" && s.Origin == "shipped" && s.Version == "1.0.0" {
				sawShipped = true
			}
			if s.Name == "my-fix" && c.Name == "yours" && s.Origin == "yours" {
				sawYours = true
			}
		}
	}
	if !sawShipped || !sawYours {
		t.Fatalf("categories %+v", rep.Categories)
	}
}

// The memory view is the vault and nothing else: notes, journal days, and a graph
// whose edges are file names mentioned in other files.
func TestMemoryReportFromTheVault(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	vault := filepath.Join(h, "vault")
	t.Setenv(envName("VAULT"), vault)
	seedFiles(t, vault, map[string]string{
		"AGENTS.md":             "read desktop.md first",
		"desktop.md":            "the map",
		"memory/notes.md":       "- likes ghostty",
		"journal/2026-10-22.md": "did a thing",
		"journal/not-a-day.md":  "ignored by the heatmap",
	})
	rep := MemoryReportNow()
	if rep.Learned.VaultNotes != 1 || rep.Learned.JournalDays != 2 {
		t.Fatalf("learned %+v", rep.Learned)
	}
	groups := map[string]string{}
	for _, n := range rep.Graph.Nodes {
		groups[n.ID] = n.Group
	}
	if groups["AGENTS.md"] != "hub" || groups["memory/notes.md"] != "memory" || groups["journal/2026-10-22.md"] != "journal" || groups["desktop.md"] != "generated" {
		t.Fatalf("node groups %v", groups)
	}
	var linked bool
	for _, l := range rep.Graph.Links {
		if l.Source == "AGENTS.md" && l.Target == "desktop.md" {
			linked = true
		}
	}
	if !linked {
		t.Fatalf("AGENTS.md mentions desktop.md, links %+v", rep.Graph.Links)
	}
	if len(rep.Heatmap) != 26*7 {
		t.Fatalf("heatmap spans %d days, want %d", len(rep.Heatmap), 26*7)
	}
}
