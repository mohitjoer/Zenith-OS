package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

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
	// Search in local catalog directory or /usr/share/asura/store/catalog/plugins/
	candidates := []string{
		"catalog/plugins",
		"/usr/share/asura/store/catalog/plugins",
		"/etc/asura/store/catalog/plugins",
	}

	var catDir string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			catDir = c
			break
		}
	}

	var items []Item
	installedPlugins := getInstalledPlugins()

	if catDir != "" {
		entries, _ := os.ReadDir(catDir)
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".json" {
				data, err := os.ReadFile(filepath.Join(catDir, e.Name()))
				if err == nil {
					var it Item
					if err := json.Unmarshal(data, &it); err == nil {
						it.Category = "plugins"
						if _, ok := installedPlugins[it.ID]; ok {
							it.Installed = true
						}
						items = append(items, it)
					}
				}
			}
		}
	}

	// Always provide built-in featured community plugins if catalog directory is sparse
	if len(items) == 0 {
		items = featuredPlugins(installedPlugins)
	}

	return items, SourceState{}, nil
}

func (pluginProvider) Install(ctx context.Context, id string) error {
	dest := filepath.Join(os.Getenv("HOME"), ".local/share/asura/plugins", id)
	return os.MkdirAll(dest, 0755)
}

func (pluginProvider) Remove(ctx context.Context, id string) error {
	dest := filepath.Join(os.Getenv("HOME"), ".local/share/asura/plugins", id)
	return os.RemoveAll(dest)
}

func getInstalledPlugins() map[string]bool {
	res := make(map[string]bool)
	home := os.Getenv("HOME")
	if home == "" {
		return res
	}
	plugDir := filepath.Join(home, ".local/share/asura/plugins")
	entries, _ := os.ReadDir(plugDir)
	for _, e := range entries {
		if e.IsDir() {
			res[e.Name()] = true
		}
	}
	return res
}

func featuredPlugins(installed map[string]bool) []Item {
	plugins := []Item{
		{
			ID:          "pomodoro",
			Category:    "plugins",
			Name:        "Pomodoro Timer",
			Summary:     "Clean focus and break timer embedded into the top bar",
			Description: "Configurable work intervals, break chimes, and session history counter.",
			Author:      "Asura Core Team",
			Version:     "1.2.0",
			Upstream:    "https://github.com/ASURA-OS/plugin-pomodoro",
			Tags:        []string{"productivity", "bar", "timer"},
		},
		{
			ID:          "weather-radar",
			Category:    "plugins",
			Name:        "Weather & Radar",
			Summary:     "Live local forecasts, temperature metrics, and radar overlay",
			Description: "Pulls NOAA and Open-Meteo forecasts into the bar weather flyout.",
			Author:      "Asura Community",
			Version:     "2.0.1",
			Upstream:    "https://github.com/ASURA-OS/plugin-weather",
			Tags:        []string{"weather", "status", "desktop"},
		},
		{
			ID:          "git-workspaces",
			Category:    "plugins",
			Name:        "Git Workspace Monitor",
			Summary:     "Real-time dirty tree and branch indicators for active repos",
			Description: "Displays active branch, uncommitted changes, and push/pull counts.",
			Author:      "Dev Collective",
			Version:     "1.0.4",
			Upstream:    "https://github.com/ASURA-OS/plugin-git",
			Tags:        []string{"developer", "git", "status"},
		},
		{
			ID:          "audio-visualizer",
			Category:    "plugins",
			Name:        "Cava Audio Visualizer",
			Summary:     "Real-time spectrum wave bar integrated into the desktop dock",
			Description: "Reactive GLSL audio spectrum visualizer synchronized with PipeWire audio sinks.",
			Author:      "SoundLab",
			Version:     "0.9.5",
			Upstream:    "https://github.com/ASURA-OS/plugin-cava",
			Tags:        []string{"audio", "media", "cava"},
		},
	}
	for i := range plugins {
		plugins[i].Installed = installed[plugins[i].ID]
	}
	return plugins
}
