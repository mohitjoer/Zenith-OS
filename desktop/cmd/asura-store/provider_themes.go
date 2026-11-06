package main

import (
	"context"
	"os"
	"path/filepath"
)

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
	themes := []Item{
		{
			ID:          "asura-dark",
			Category:    "themes",
			Name:        "Asura Cyber Dark",
			Summary:     "Default deep obsidian theme with electric crimson accents",
			Author:      "Asura Core Team",
			Version:     "1.0.0",
			Accent:      "#ff2a5f",
			Surface:     "#121214",
			Installed:   true,
			Active:      currentTheme == "asura-dark" || currentTheme == "",
		},
		{
			ID:          "catppuccin-mocha",
			Category:    "themes",
			Name:        "Catppuccin Mocha",
			Summary:     "Soothing pastel theme for the high-spirited",
			Author:      "Catppuccin Org",
			Version:     "0.2.0",
			Accent:      "#cba6f7",
			Surface:     "#1e1e2e",
			Installed:   true,
			Active:      currentTheme == "catppuccin-mocha",
		},
		{
			ID:          "tokyo-night",
			Category:    "themes",
			Name:        "Tokyo Night",
			Summary:     "Clean dark theme celebrating the lights of downtown Tokyo",
			Author:      "TokyoNight Team",
			Version:     "1.1.0",
			Accent:      "#7aa2f7",
			Surface:     "#1a1b26",
			Installed:   true,
			Active:      currentTheme == "tokyo-night",
		},
		{
			ID:          "nord-aurora",
			Category:    "themes",
			Name:        "Nord Aurora",
			Summary:     "An arctic, north-bluish clean color palette",
			Author:      "Arctic Ice Studio",
			Version:     "0.8.0",
			Accent:      "#88c0d0",
			Surface:     "#2e3440",
			Installed:   true,
			Active:      currentTheme == "nord-aurora",
		},
	}
	return themes, SourceState{}, nil
}

func (themeProvider) Install(ctx context.Context, id string) error {
	return nil
}

func (themeProvider) Remove(ctx context.Context, id string) error {
	return nil
}

func getCurrentTheme() string {
	home := os.Getenv("HOME")
	if home == "" {
		return "asura-dark"
	}
	f, err := os.ReadFile(filepath.Join(home, ".config/asura/theme.name"))
	if err != nil {
		return "asura-dark"
	}
	return string(f)
}
