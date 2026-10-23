package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// introspect.go reads what the dashboard shows about the vault and the skills on
// disk. It knows nothing about the engine: what an engine remembers or can do is
// the engine's to show.

const agentVersion = "0.2.0"

// ---- skills ----------------------------------------------------------------

type Skill struct {
	Name        string `json:"name"`
	Dir         string `json:"dir"`
	Description string `json:"description"`
	Version     string `json:"version,omitempty"`
	Origin      string `json:"origin"` // shipped | yours
}

type SkillCategory struct {
	Name   string  `json:"name"`
	Skills []Skill `json:"skills"`
}

type SkillsReport struct {
	Counts     map[string]int  `json:"counts"`
	Categories []SkillCategory `json:"categories"`
}

// skillFrontmatter pulls name/description/version from a SKILL.md header.
func skillFrontmatter(path string) (name, desc, version string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", ""
	}
	s := string(b)
	if !strings.HasPrefix(s, "---") {
		return "", "", ""
	}
	body := s[3:]
	end := strings.Index(body, "\n---")
	if end < 0 {
		return "", "", ""
	}
	for _, line := range strings.Split(body[:end], "\n") {
		t := strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(t, "name:"); ok && name == "" {
			name = strings.Trim(strings.TrimSpace(v), `"'`)
		}
		if v, ok := strings.CutPrefix(t, "description:"); ok && desc == "" {
			desc = strings.Trim(strings.TrimSpace(v), `"'`)
		}
		if v, ok := strings.CutPrefix(t, "version:"); ok && version == "" {
			version = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return name, desc, version
}

// SkillsReportNow lists the skills on disk: the ones Zenith ships (skillRoots)
// and the ones the user or an agent keeps in the vault's skills/ directory. A
// skill is a directory with a SKILL.md; a name found twice is listed once.
func SkillsReportNow() SkillsReport {
	rep := SkillsReport{Counts: map[string]int{"shipped": 0, "yours": 0}, Categories: []SkillCategory{}}
	found := map[string][]Skill{}
	seen := map[string]bool{}
	scan := func(origin, root string) {
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, e := range entries {
			md := filepath.Join(root, e.Name(), "SKILL.md")
			if _, err := os.Stat(md); err != nil || seen[e.Name()] {
				continue
			}
			seen[e.Name()] = true
			name, desc, version := skillFrontmatter(md)
			if name == "" {
				name = e.Name()
			}
			found[origin] = append(found[origin], Skill{
				Name: name, Dir: e.Name(), Description: desc, Version: version, Origin: origin,
			})
		}
	}
	for _, root := range skillRoots() {
		scan("shipped", root)
	}
	scan("yours", filepath.Join(VaultDir(), "skills"))
	for _, origin := range []string{"shipped", "yours"} {
		list := found[origin]
		if len(list) == 0 {
			continue
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
		rep.Counts[origin] = len(list)
		rep.Categories = append(rep.Categories, SkillCategory{Name: origin, Skills: list})
	}
	return rep
}

// ---- memory ----------------------------------------------------------------

type MemoryReport struct {
	// Learned is what has accumulated in the vault's own directories.
	Learned struct {
		VaultNotes  int `json:"vaultNotes"`
		JournalDays int `json:"journalDays"`
	} `json:"learned"`
	Graph   GraphModel   `json:"graph"`
	Heatmap []HeatmapDay `json:"heatmap"`
}

type GraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Group string `json:"group"`
	Size  int64  `json:"size"`
}

type GraphLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type GraphModel struct {
	Nodes []GraphNode `json:"nodes"`
	Links []GraphLink `json:"links"`
}

type HeatmapDay struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// buildMemoryGraph turns the vault into nodes and reference links: markdown
// files are nodes, a mention of another vault file's name is an edge.
func buildMemoryGraph() GraphModel {
	g := GraphModel{}
	files, err := VaultTree()
	if err != nil {
		return g
	}
	group := func(rel string) string {
		switch {
		case rel == "AGENTS.md" || rel == "CLAUDE.md":
			return "hub"
		case strings.HasPrefix(rel, "memory/"):
			return "memory"
		case strings.HasPrefix(rel, "journal/"):
			return "journal"
		default:
			return "generated"
		}
	}
	var nodes []GraphNode
	byBase := map[string]string{} // base name -> id
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".md") || f.Path == "CLAUDE.md" {
			continue
		}
		nodes = append(nodes, GraphNode{
			ID:    f.Path,
			Label: strings.TrimSuffix(filepath.Base(f.Path), ".md"),
			Group: group(f.Path),
			Size:  f.Size,
		})
		byBase[filepath.Base(f.Path)] = f.Path
	}

	var links []GraphLink
	seen := map[string]bool{}
	for _, n := range nodes {
		content, _ := ReadVaultFile(n.ID)
		if len(content) == 0 || len(content) > 512*1024 {
			continue
		}
		text := string(content)
		for base, id := range byBase {
			if id == n.ID || !strings.Contains(text, base) {
				continue
			}
			if key := n.ID + "->" + id; !seen[key] {
				seen[key] = true
				links = append(links, GraphLink{Source: n.ID, Target: id})
			}
		}
	}
	g.Nodes, g.Links = nodes, links
	return g
}

// buildHeatmap counts activity per day over the trailing 26 weeks, from the
// journal files named by date.
func buildHeatmap() []HeatmapDay {
	counts := map[string]int{}
	if entries, err := os.ReadDir(filepath.Join(VaultDir(), "journal")); err == nil {
		for _, e := range entries {
			name := strings.TrimSuffix(e.Name(), ".md")
			if _, err := time.Parse("2006-01-02", name); err == nil {
				counts[name]++
			}
		}
	}
	days := 26 * 7
	out := make([]HeatmapDay, 0, days)
	today := time.Now()
	for i := days - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, HeatmapDay{Date: d, Count: counts[d]})
	}
	return out
}

func countFiles(dir string) int {
	n := 0
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				n++
			}
		}
	}
	return n
}

func MemoryReportNow() MemoryReport {
	var rep MemoryReport
	rep.Learned.VaultNotes = countFiles(filepath.Join(VaultDir(), "memory"))
	rep.Learned.JournalDays = countFiles(filepath.Join(VaultDir(), "journal"))
	rep.Graph = buildMemoryGraph()
	rep.Heatmap = buildHeatmap()
	return rep
}

// ---- about -----------------------------------------------------------------

type AboutReport struct {
	Version string       `json:"version"`
	Port    int          `json:"port"`
	Vault   string       `json:"vault"`
	Engine  EngineStatus `json:"engine"`
	Prowl   struct {
		Installed bool `json:"installed"`
	} `json:"prowl"`
}

func AboutReportNow(cfg Config) AboutReport {
	var rep AboutReport
	rep.Version = agentVersion
	rep.Port = cfg.Port
	rep.Vault = VaultDir()
	rep.Engine = engineStatus(cfg)
	_, rep.Prowl.Installed = findProwl()
	return rep
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
