package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// The bar styles are built into the shell, so this category only reports them
// and which one shell.json selects: Bar Studio's style shelf reads it. Nothing
// installs or removes a bar style.
type barProvider struct{}

var builtinBarStyles = []Item{
	{ID: "controlbar", Name: "Control Bar", Summary: "Top bar", Description: "The default top bar.", Tags: []string{"top", "horizontal", "built-in"}},
	{ID: "qsbar", Name: "Control Bar (Legacy)", Summary: "Top bar", Description: "The default top bar (legacy alias).", Tags: []string{"top", "horizontal", "built-in"}},
	{ID: "kairos", Name: "Kairos", Summary: "Island clock", Description: "One floating island at the top of the screen: the clock that grows on hover.", Tags: []string{"top", "island", "built-in"}},
	{ID: "sumi", Name: "Sumi", Summary: "Ink spine", Description: "A vertical rail down the left edge.", Tags: []string{"rail", "vertical", "built-in"}},
}

func (barProvider) Category() Category {
	return Category{
		ID:          "barstyles",
		Name:        "Bar styles",
		Group:       "wear",
		Description: "Complete bar compositions for the Zenith shell.",
	}
}

func (barProvider) Load(context.Context, bool) ([]Item, SourceState, error) {
	active := readBarStyleSelection(filepath.Join(configHome(), "zenith", "shell.json"))
	if active == "" {
		active = "qsbar"
	}
	items := make([]Item, len(builtinBarStyles))
	for i, style := range builtinBarStyles {
		style.Category = "barstyles"
		style.Installed = true
		style.Active = style.ID == active
		items[i] = style
	}
	return items, SourceState{}, nil
}

func (barProvider) Install(_ context.Context, id string) error {
	return fmt.Errorf("the %s bar style is built in and already installed", id)
}

func (barProvider) Remove(_ context.Context, id string) error {
	return fmt.Errorf("the %s bar style is built in and cannot be removed", id)
}

func readBarStyleSelection(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var state struct {
		BarStyle string `json:"barStyle"`
	}
	if json.Unmarshal(raw, &state) != nil {
		return ""
	}
	return state.BarStyle
}

func configHome() string {
	if b := os.Getenv("XDG_CONFIG_HOME"); b != "" {
		return b
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}
