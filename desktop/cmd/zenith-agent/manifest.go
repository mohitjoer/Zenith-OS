package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// manifest.go answers the question "how do I make MY coding agent Zenith-aware?"
// for any agent, supported or not. It lists every path Zenith Agent exposes -- the
// skill, the generated vault maps, prowl for code intelligence, the MCP
// server config -- plus a ready-to-paste instruction snippet. The supported
// agents get all of this in one click (Wire); an unsupported one gets the same
// power by pasting the snippet into its own instructions file.

// ManifestItem is one resource an agent can be pointed at. Owner says who writes
// it: "generated" (Zenith Agent regenerates it each reindex -- read, never edit),
// "yours" (write your durable notes here), "read-only" (a shipped file), or
// "tool" (a binary to run).
type ManifestItem struct {
	Label  string `json:"label"`
	Path   string `json:"path"` // tilde-abbreviated for display
	Kind   string `json:"kind"` // skill | map | dir | tool | mcp
	Owner  string `json:"owner"`
	Desc   string `json:"desc"`
	Exists bool   `json:"exists"`
}

// ChatBackendInfo is the engine as `agent --json` lists it for the sidebar.
type ChatBackendInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Available   bool   `json:"available"`
	Recommended bool   `json:"recommended"`
	Active      bool   `json:"active"`
}

// Manifest is the whole agent-facing surface: what to read, what to run, who is
// wired, the engine, and the paste snippet for anything else.
type Manifest struct {
	Skill   ManifestItem   `json:"skill"`
	Prowl   ManifestItem   `json:"prowl"`
	Vault   []ManifestItem `json:"vault"`
	MCP     []any          `json:"mcp"`
	Agents  []Agent        `json:"agents"`
	Engine  EngineStatus   `json:"engine"`
	Pointer string         `json:"pointer"`
	Snippet string         `json:"snippet"`
}

// vaultManifest lists the generated vault docs and note dirs with a one-line
// gloss each, in read order (index first).
func vaultManifest() []ManifestItem {
	root := VaultDir()
	docs := []struct{ name, desc, kind string }{
		{"AGENTS.md", "index: where every config lives, the binary that owns it, how to reload", "map"},
		{"desktop.md", "the desktop map: bar, dock, widgets, launcher, Hub", "map"},
		{"system.md", "hardware, kernel, GPU, disks, displays", "map"},
		{"packages.md", "installed packages and pending updates", "map"},
		{"zenith-repo.md", "the Zenith source map", "map"},
		{"user.md", "your own config changes over the shipped base", "map"},
		{"habits.md", "learned usage patterns on this machine", "map"},
		{"memory", "durable notes the agent keeps across sessions", "dir"},
		{"journal", "dated notes (journal/YYYY-MM-DD.md)", "dir"},
	}
	out := make([]ManifestItem, 0, len(docs))
	for _, d := range docs {
		p := filepath.Join(root, d.name)
		exists := fileExists(p) || dirExists(p)
		owner := "generated" // the .md maps are rewritten every reindex
		if d.kind == "dir" {
			owner = "yours" // memory/ and journal/ are where the agent writes
		}
		out = append(out, ManifestItem{
			Label: d.name, Path: tildeAbbrev(p), Kind: d.kind, Owner: owner, Desc: d.desc, Exists: exists,
		})
	}
	return out
}

// BuildManifest assembles the agent-facing surface for the given config.
func BuildManifest(cfg Config) Manifest {
	skillDir := skillSourceDir()
	skill := ManifestItem{
		Label: "zenith skill", Kind: "skill", Owner: "read-only",
		Desc:   "safety rules, the bar/dock/plugins/themes guide, and the command catalogue",
		Exists: skillDir != "",
	}
	if skillDir != "" {
		skill.Path = tildeAbbrev(filepath.Join(skillDir, "SKILL.md"))
	}

	prowl := ManifestItem{Label: "prowl", Kind: "tool", Owner: "tool",
		Desc: "cited code intelligence, reindexed each run: search/find/def/references/outline/impact"}
	if bin, ok := findProwl(); ok {
		prowl.Path = tildeAbbrev(bin)
		prowl.Exists = true
	}

	return Manifest{
		Skill:   skill,
		Prowl:   prowl,
		Vault:   vaultManifest(),
		MCP:     prowlMCPServers(),
		Agents:  DetectAgents(),
		Engine:  engineStatus(cfg),
		Pointer: PointerBlock,
		Snippet: manifestSnippet(skillDir),
	}
}

// manifestSnippet is the plain-language block a user pastes into any coding
// agent's instructions file so it works like the supported ones. Paths are
// resolved and tilde-abbreviated.
func manifestSnippet(skillDir string) string {
	vault := tildeAbbrev(VaultDir())
	skill := "the zenith skill"
	if skillDir != "" {
		skill = tildeAbbrev(filepath.Join(skillDir, "SKILL.md"))
	}
	var b strings.Builder
	b.WriteString("You are working on a Zenith machine (Ubuntu + Hyprland + Quickshell).\n\n")
	fmt.Fprintf(&b, "- Read the Zenith vault first, at %s/ -- start with AGENTS.md (where every\n", vault)
	b.WriteString("  config lives, the binary that owns it, and how to reload it), then desktop.md,\n")
	b.WriteString("  system.md, packages.md, zenith-repo.md, user.md.\n")
	b.WriteString("- For code questions prefer prowl (cited, reindexed each run) over grep:\n")
	b.WriteString("  `prowl search \"<question>\"`, `find`, `def`, `references`, `outline`, `impact`.\n")
	fmt.Fprintf(&b, "- Read the Zenith desktop skill for safe how-tos and the command catalogue: %s\n", skill)
	fmt.Fprintf(&b, "- The .md maps are generated by %s and refreshed on every reindex: read\n", displayName)
	b.WriteString("  them, never edit them. Your durable notes go in the vault's memory/ and dated\n")
	b.WriteString("  notes in journal/ -- those are yours and survive.\n")
	b.WriteString("- Act through the zenith commands (zenith, zenith-shell, zenith-hubd, zenith-wallpaper,\n")
	fmt.Fprintf(&b, "  %s); never edit shipped files.\n", binName)
	return b.String()
}
