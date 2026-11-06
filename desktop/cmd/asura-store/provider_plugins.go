package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// pluginIDRe is the id grammar `asura plugin` enforces: lowercase alphanumerics
// and dashes, never starting with a dash, so a store id can never escape the
// plugins directory or read as an option.
var pluginIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type pluginProvider struct{}

func (pluginProvider) Category() Category {
	return Category{
		ID:          "plugins",
		Name:        "Plugins",
		Group:       "customize",
		Description: "Community widgets, overlays, integrations, and tools for Asura OS.",
	}
}

func (pluginProvider) Load(ctx context.Context, _ bool) ([]Item, SourceState, error) {
	installed := getInstalledPlugins()
	byID := map[string]Item{}
	var order []string

	if catDir := catalogDirFor("plugins"); catDir != "" {
		entries, _ := os.ReadDir(catDir)
		for _, e := range entries {
			if filepath.Ext(e.Name()) != ".json" || e.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(catDir, e.Name()))
			if err != nil {
				continue
			}
			var it Item
			if err := json.Unmarshal(data, &it); err != nil {
				continue
			}
			if !pluginIDRe.MatchString(it.ID) {
				continue
			}
			it.Category = "plugins"
			if _, ok := byID[it.ID]; !ok {
				order = append(order, it.ID)
			}
			byID[it.ID] = it
		}
	}

	// An installed plugin with no catalogue entry still lists, so an installed
	// plugin never vanishes from the shelf.
	for id := range installed {
		if _, seen := byID[id]; !seen {
			byID[id] = Item{ID: id, Category: "plugins", Name: id}
			order = append(order, id)
		}
	}

	items := make([]Item, 0, len(order))
	for _, id := range order {
		it := byID[id]
		it.Installed = installed[id]
		items = append(items, it)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].ID < items[j].ID
	})
	return items, SourceState{}, nil
}

func (pluginProvider) Install(ctx context.Context, id string) error {
	if !pluginIDRe.MatchString(id) {
		return fmt.Errorf("invalid plugin id %q", id)
	}
	// A catalogue entry carries the upstream repo `asura plugin install`
	// clones from. Without that link there is no payload to install -- an
	// empty folder is not a plugin -- so refuse rather than claim success.
	upstream := ""
	if catDir := catalogDirFor("plugins"); catDir != "" {
		if data, err := os.ReadFile(filepath.Join(catDir, id+".json")); err == nil {
			var it Item
			if json.Unmarshal(data, &it) == nil && it.Upstream != "" {
				upstream = it.Upstream
			}
		}
	}
	if !have("asura") {
		if upstream == "" {
			return fmt.Errorf("plugin %q has no upstream to install from and asura is not installed", id)
		}
		return fmt.Errorf("asura is not installed; run asura plugin install %s", upstream)
	}
	if upstream == "" {
		// Already installed under a different id, or a local folder install:
		// `asura plugin list` owns the roster; the store never mkdirs empties.
		return fmt.Errorf("plugin %q has no upstream to install from; use asura plugin install <git-url|dir>", id)
	}
	emitStage("FETCHING", -1)
	cmd := exec.CommandContext(ctx, "asura", "plugin", "install", upstream, "--yes")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install failed: %s", withOutput(string(out), err))
	}
	emitStage("APPLYING", 100)
	return nil
}

func (pluginProvider) Remove(ctx context.Context, id string) error {
	if !pluginIDRe.MatchString(id) {
		return fmt.Errorf("invalid plugin id %q", id)
	}
	if have("asura") {
		cmd := exec.CommandContext(ctx, "asura", "plugin", "remove", id)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("remove failed: %s", withOutput(string(out), err))
		}
		return nil
	}
	dest := filepath.Join(pluginDir(), id)
	return os.RemoveAll(dest)
}

// withOutput folds captured helper output into the error without leaking an
// empty prefix when the helper was silent.
func withOutput(out string, err error) string {
	if s := strings.TrimSpace(out); s != "" {
		return fmt.Sprintf("%s (%v)", s, err)
	}
	return err.Error()
}

func pluginDir() string {
	home, _ := os.UserHomeDir()
	if b := os.Getenv("XDG_DATA_HOME"); b != "" {
		return filepath.Join(b, "asura", "plugins")
	}
	return filepath.Join(home, ".local", "share", "asura", "plugins")
}

func getInstalledPlugins() map[string]bool {
	res := map[string]bool{}
	entries, _ := os.ReadDir(pluginDir())
	for _, e := range entries {
		if e.IsDir() {
			res[e.Name()] = true
		}
	}
	return res
}
