package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// repo.go builds asura-repo.md: a pre-indexed map of the Asura monorepo
// itself, so agents fixing the system can navigate its source without
// crawling it. The snapshot is generated where the checkout exists (package
// build, deploy.sh) and shipped to the installed target, which has none.

// shippedRepoCandidates lists where a generated snapshot may live, packaged
// first, dev-deploy second. Reindex copies the newest one into the vault.
func shippedRepoCandidates() []string {
	if v := os.Getenv(envName("SHIPPED")); v != "" {
		return []string{v}
	}
	return []string{
		"/usr/share/asura/" + productName + "/asura-repo.md",
		statePath("repo.md"),
	}
}

func xdgState() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	return filepath.Join(home(), ".local", "state")
}

// newestShippedRepoIndex returns the freshest existing snapshot, or "".
func newestShippedRepoIndex() string {
	best, bestT := "", time.Time{}
	for _, p := range shippedRepoCandidates() {
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			continue
		}
		if fi.ModTime().After(bestT) {
			best, bestT = p, fi.ModTime()
		}
	}
	return best
}

// RepoIndexDoc walks a Asura checkout and renders the repo map body (the
// fenced part of asura-repo.md). Git details are best effort.
func RepoIndexDoc(root string) (string, error) {
	fi, err := os.Stat(root)
	if err != nil || !fi.IsDir() {
		return "", fmt.Errorf("not a directory: %s", root)
	}
	var b strings.Builder

	if commit, branch, when := gitFacts(root); commit != "" {
		b.WriteString("## Checkout\n\n")
		fmt.Fprintf(&b, "- Commit: %s (%s)\n", commit, branch)
		if when != "" {
			fmt.Fprintf(&b, "- Committed: %s\n", when)
		}
		fmt.Fprintf(&b, "- Indexed: %s\n\n", time.Now().Format("2006-01-02 15:04"))
	}

	b.WriteString("## Layout\n\n")
	b.WriteString("| Path | Files | Purpose |\n|---|---|---|\n")
	for _, row := range repoLayout(root) {
		fmt.Fprintf(&b, "| `%s` | %d | %s |\n", row.path, row.files, row.purpose)
	}

	b.WriteString("\n## Key entry points\n\n")
	b.WriteString("| Concern | Path |\n|---|---|\n")
	for _, e := range [][2]string{
		{"Hyprland config entry", "session/hyprland/hyprland.conf"},
		{"Shell entry (QML)", "shell/shell.qml"},
		{"Settings app (QML)", "shell/hub/"},
		{"User CLI (Go)", "desktop/cmd/asura/"},
		{"Shell daemon (Go)", "desktop/cmd/asura-shell/"},
		{"Settings data plane (Go)", "desktop/cmd/asura-hubd/"},
		{displayName + " (this system)", "desktop/cmd/" + binName + "/"},
		{"Session units", "session/units/"},
		{"Package builds", "release/"},
	} {
		if _, err := os.Stat(filepath.Join(root, e[1])); err == nil {
			fmt.Fprintf(&b, "| %s | `%s` |\n", e[0], e[1])
		}
	}

	if docs := docsList(root); len(docs) > 0 {
		b.WriteString("\n## Docs\n\n")
		for _, d := range docs {
			fmt.Fprintf(&b, "- `docs/%s` %s\n", d[0], d[1])
		}
	}
	return b.String(), nil
}

func gitFacts(root string) (commit, branch, when string) {
	git := func(args ...string) string {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	commit = git("rev-parse", "--short", "HEAD")
	branch = git("rev-parse", "--abbrev-ref", "HEAD")
	when = git("log", "-1", "--format=%cs")
	return
}

type layoutRow struct {
	path    string
	files   int
	purpose string
}

// repoLayout counts files under the repo's meaningful top paths, two levels
// deep, skipping VCS and build noise.
func repoLayout(root string) []layoutRow {
	purposes := map[string]string{
		"desktop":      "the control tools in Go: asura, asura-shell, asura-hubd, " + binName,
		"shell":        "the Quickshell surfaces: bar, dock, launcher, Settings, widgets",
		"session":      "the Hyprland config, session wrappers and systemd user units",
		"system":       "machine definition: hardware detection, package sets",
		"installation": "how a machine is built: installer and ISO",
		"release":      "packaging: the .deb, the apt repository, update checks",
		"docs":         "the guides",
		"agents":       "how to work on the repo: rules and design system",
		"tests":        "standalone check scripts",
	}
	skip := map[string]bool{".git": true, "node_modules": true, ".worktrees": true, "local": true}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var rows []layoutRow
	for _, e := range entries {
		if !e.IsDir() || skip[e.Name()] {
			continue
		}
		purpose, known := purposes[e.Name()]
		if !known {
			continue
		}
		n := 0
		_ = filepath.WalkDir(filepath.Join(root, e.Name()), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() && skip[d.Name()] {
				return filepath.SkipDir
			}
			if !d.IsDir() {
				n++
			}
			return nil
		})
		rows = append(rows, layoutRow{path: e.Name() + "/", files: n, purpose: purpose})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].files > rows[j].files })
	return rows
}

// docsList returns [file, first-heading] pairs for docs/*.md.
func docsList(root string) [][2]string {
	entries, err := os.ReadDir(filepath.Join(root, "docs"))
	if err != nil {
		return nil
	}
	var out [][2]string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		title := ""
		if b, err := os.ReadFile(filepath.Join(root, "docs", e.Name())); err == nil {
			if line := firstLine(string(b)); strings.HasPrefix(line, "# ") {
				title = strings.TrimPrefix(line, "# ")
			}
		}
		out = append(out, [2]string{e.Name(), title})
	}
	return out
}

// writeRepoVaultDoc lays asura-repo.md into the vault: regenerated from a
// live checkout when ASURA_AGENT_REPO points at one, else copied from the
// newest shipped snapshot, else a stub explaining how to generate it.
func writeRepoVaultDoc() error {
	meta := OKFMeta{
		Type:        "Repository Architecture",
		Title:       "Asura Monorepo Architecture",
		Description: "The Asura OS monorepo: where source lives, directory layout, and what each component does.",
		Resource:    "ASURA-OS",
		Tags:        []string{"asura", "architecture", "repository", "source-tree"},
	}
	intro := "# Asura Source Map\n\n[← Back to Index](./index.md)"

	body := ""
	if repo := os.Getenv(envName("REPO")); repo != "" {
		if doc, err := RepoIndexDoc(repo); err == nil {
			body = doc
		}
	}
	if body == "" {
		if snap := newestShippedRepoIndex(); snap != "" {
			if b, err := os.ReadFile(snap); err == nil {
				body = string(b)
			}
		}
	}
	if body == "" {
		body = "No repo snapshot found. On a packaged system it ships at\n" +
			"/usr/share/asura/" + productName + "/asura-repo.md; on a checkout, run\n" +
			"`" + binName + " repo-index <checkout>`.\n"
	}
	return writeVaultDoc("asura-repo.md", meta, intro, body)
}

// cmdRepoIndex is the generation verb used by the PKGBUILD and deploy.sh.
func cmdRepoIndex(root, out string) error {
	if root == "" {
		return fmt.Errorf("usage: " + binName + " repo-index <checkout-root> [out-file]")
	}
	doc, err := RepoIndexDoc(root)
	if err != nil {
		return err
	}
	if out == "" {
		fmt.Print(doc)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return atomicWrite(out, []byte(doc), 0o644)
}
