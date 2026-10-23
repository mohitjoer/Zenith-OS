package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// performance.json is the file the Performance section of Zenith Settings
// writes; a few daemon behaviours (the battery power-saver, the power profile
// restore) read their opt-ins from it.

// parsePerfFlag reads a boolean opt-in from a performance.json body. Anything
// malformed, absent, or the wrong type is false, so an optimisation stays off
// unless the user clearly turned it on.
func parsePerfFlag(b []byte, key string) bool {
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return false
	}
	v, _ := m[key].(bool)
	return v
}

// zenithConfigDir is ~/.config/zenith (honouring XDG_CONFIG_HOME), the dir the
// Zenith Settings sections write. Empty only when the home dir is unknowable.
func zenithConfigDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "zenith")
}

// perfPath is ~/.config/zenith/performance.json, the file the Performance section
// in Zenith Settings writes. Empty only when the home dir is unknowable.
func perfPath() string {
	dir := zenithConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "performance.json")
}

// perfFlag reads one opt-in out of performance.json. A missing file is off.
func perfFlag(key string) bool {
	b, err := os.ReadFile(perfPath())
	if err != nil {
		return false
	}
	return parsePerfFlag(b, key)
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
