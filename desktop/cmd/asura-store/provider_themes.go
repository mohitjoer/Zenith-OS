package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// themeIDRe is the theme id grammar: lowercase alphanumerics, dots and dashes,
// never starting with a dash or dot, so an id can never escape a theme
// directory or read as an option.
var themeIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// shippedThemeIDs are the themes that ship with Asura OS itself. They are
// always listed as installed even though none of them lives in a user theme
// directory; the active one is the one named by the current theme file.
var shippedThemeIDs = map[string]bool{
	"asura-dark": true,
}

type themeProvider struct{}

func (themeProvider) Category() Category {
	return Category{
		ID:          "themes",
		Name:        "Themes & Rices",
		Group:       "customize",
		Description: "Color palettes, wallpapers, and desktop appearance rices.",
	}
}

func (themeProvider) Load(ctx context.Context, _ bool) ([]Item, SourceState, error) {
	currentTheme := getCurrentTheme()
	installed := installedThemeIDs()

	byID := map[string]Item{}
	var order []string
	add := func(it Item) {
		if it.ID == "" {
			return
		}
		if _, seen := byID[it.ID]; !seen {
			order = append(order, it.ID)
			byID[it.ID] = it
		}
	}

	// Catalogue files win over the built-in fallbacks for field content, so a
	// checked-out catalogue updates the shelf without a binary rebuild.
	builtins := builtinThemes()
	builtinByID := map[string]Item{}
	for _, it := range builtins {
		builtinByID[it.ID] = it
	}
	for _, it := range loadThemeCatalog() {
		add(it)
	}
	for _, it := range builtins {
		if _, seen := byID[it.ID]; !seen {
			add(it)
		} else {
			byID[it.ID] = mergeThemeItem(byID[it.ID], builtinByID[it.ID])
		}
	}

	// Merge anything actually installed on disk (a theme dir with no catalogue
	// entry still lists, so an installed theme never vanishes).
	for id := range installed {
		if _, seen := byID[id]; !seen {
			byID[id] = Item{ID: id, Category: "themes", Name: id}
			order = append(order, id)
		}
	}

	items := make([]Item, 0, len(order))
	for _, id := range order {
		it := byID[id]
		it.Category = "themes"
		it.Installed = installed[id] || shippedThemeIDs[id]
		it.Active = it.Installed && currentTheme == id
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

// builtinThemes are the themes the binary ships entries for. They are metadata
// only -- installed state comes from the shipped set and the theme dirs, so a
// built-in entry for a theme whose files are absent reads as available, never
// as installed.
func builtinThemes() []Item {
	return []Item{
		{ID: "asura-dark", Category: "themes", Name: "Asura Cyber Dark", Summary: "Default deep obsidian theme with electric crimson accents", Author: "Asura Core Team", Version: "1.0.0", Accent: "#ff2a5f", Surface: "#121214"},
		{ID: "catppuccin-mocha", Category: "themes", Name: "Catppuccin Mocha", Summary: "Soothing pastel theme for the high-spirited", Author: "Catppuccin Org", Version: "0.2.0", Accent: "#cba6f7", Surface: "#1e1e2e"},
		{ID: "tokyo-night", Category: "themes", Name: "Tokyo Night", Summary: "Clean dark theme celebrating the lights of downtown Tokyo", Author: "TokyoNight Team", Version: "1.1.0", Accent: "#7aa2f7", Surface: "#1a1b26"},
		{ID: "nord-aurora", Category: "themes", Name: "Nord Aurora", Summary: "An arctic, north-bluish clean color palette", Author: "Arctic Ice Studio", Version: "0.8.0", Accent: "#88c0d0", Surface: "#2e3440"},
	}
}

// mergeThemeItem fills empty fields of dst from src, so catalogue metadata
// enriches a built-in entry without erasing anything the entry already says.
func mergeThemeItem(dst, src Item) Item {
	if dst.Name == "" {
		dst.Name = src.Name
	}
	if dst.Summary == "" {
		dst.Summary = src.Summary
	}
	if dst.Description == "" {
		dst.Description = src.Description
	}
	if dst.Author == "" {
		dst.Author = src.Author
	}
	if dst.Version == "" {
		dst.Version = src.Version
	}
	if dst.Accent == "" {
		dst.Accent = src.Accent
	}
	if dst.Surface == "" {
		dst.Surface = src.Surface
	}
	if dst.Upstream == "" {
		dst.Upstream = src.Upstream
	}
	if len(dst.Tags) == 0 {
		dst.Tags = src.Tags
	}
	return dst
}

func loadThemeCatalog() []Item {
	var items []Item
	catDir := catalogDirFor("themes")
	if catDir == "" {
		return items
	}
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
		if !themeIDRe.MatchString(it.ID) {
			continue
		}
		it.Category = "themes"
		items = append(items, it)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (themeProvider) Install(ctx context.Context, id string) error {
	if !themeIDRe.MatchString(id) {
		return fmt.Errorf("invalid theme id %q", id)
	}
	// A theme is applied, not downloaded: activation means naming it in the
	// current theme file, which the shell's theme engine picks up live. Only
	// an installed theme can be activated, so a catalogue-only entry that was
	// never installed is refused rather than claimed.
	if !installedThemeIDs()[id] && !shippedThemeIDs[id] {
		return fmt.Errorf("theme %q is not installed; install its files first", id)
	}
	dir := currentThemeDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "theme.name"), []byte(id+"\n"), 0o644); err != nil {
		return err
	}
	emitStage("APPLYING", 100)
	return nil
}

func (themeProvider) Remove(ctx context.Context, id string) error {
	if !themeIDRe.MatchString(id) {
		return fmt.Errorf("invalid theme id %q", id)
	}
	if shippedThemeIDs[id] {
		return fmt.Errorf("theme %q is built in and cannot be removed", id)
	}
	if currentTheme := getCurrentTheme(); currentTheme == id {
		return fmt.Errorf("theme %q is active; activate another theme first", id)
	}
	home, _ := os.UserHomeDir()
	for _, base := range themeInstallDirs(home) {
		if err := os.RemoveAll(filepath.Join(base, id)); err != nil {
			return err
		}
	}
	return nil
}

// getCurrentTheme reads the active theme name. The shell's theme engine writes
// and watches <current>/theme.name; the older bare ~/.config/asura/theme.name
// is honoured as a fallback for checkouts that still carry it. The value is
// trimmed: a trailing newline must not leave every theme inactive.
func getCurrentTheme() string {
	home, _ := os.UserHomeDir()
	for _, path := range []string{
		filepath.Join(currentThemeDir(), "theme.name"),
		filepath.Join(home, ".config", "asura", "theme.name"),
	} {
		if b, err := os.ReadFile(path); err == nil {
			if name := strings.TrimSpace(string(b)); name != "" {
				return name
			}
		}
	}
	return ""
}

func currentThemeDir() string {
	home, _ := os.UserHomeDir()
	if b := os.Getenv("XDG_CONFIG_HOME"); b != "" {
		return filepath.Join(b, "asura", "current")
	}
	return filepath.Join(home, ".config", "asura", "current")
}

// themeInstallDirs are the directories a theme copy can live in, in the order
// the shell's theme carousels scan them.
func themeInstallDirs(home string) []string {
	var dirs []string
	if b := os.Getenv("XDG_DATA_HOME"); b != "" {
		dirs = append(dirs, filepath.Join(b, "asura", "themes"))
	} else {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "asura", "themes"))
	}
	if b := os.Getenv("XDG_CONFIG_HOME"); b != "" {
		dirs = append(dirs, filepath.Join(b, "asura", "themes"))
	} else {
		dirs = append(dirs, filepath.Join(home, ".config", "asura", "themes"))
	}
	return dirs
}

func installedThemeIDs() map[string]bool {
	home, _ := os.UserHomeDir()
	set := map[string]bool{}
	for _, base := range themeInstallDirs(home) {
		entries, _ := os.ReadDir(base)
		for _, e := range entries {
			if e.IsDir() {
				set[e.Name()] = true
			}
		}
	}
	return set
}
