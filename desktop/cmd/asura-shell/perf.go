package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// performance.json is the file the Performance section of Asura Settings
// writes; a few daemon behaviours (the battery power-saver, the power profile
// restore) read their opt-ins from it.

// asuraConfigDir is ~/.config/asura (honouring XDG_CONFIG_HOME), the dir the
// Asura Settings sections write. Empty only when the home dir is unknowable.
func asuraConfigDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "asura")
}

// perfPath is ~/.config/asura/performance.json, the file the Performance section
// in Asura Settings writes. Empty only when the home dir is unknowable.
func perfPath() string {
	dir := asuraConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "performance.json")
}

// perfFlagDefault is perfFlag with an explicit default, for optimisations that
// ship on by default (the user opts out). A missing file, missing key, or wrong
// type all yield def.
func perfFlagDefault(key string, def bool) bool {
	b, err := os.ReadFile(perfPath())
	if err != nil {
		return def
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return def
	}
	if v, ok := m[key].(bool); ok {
		return v
	}
	return def
}
