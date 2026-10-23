package main

import (
	"fmt"
	"os/exec"
	"time"
)

var storeSections = map[string]struct{}{
	"discover": {}, "library": {}, "rices": {}, "lockscreens": {},
	"barstyles": {}, "fastfetch": {}, "plugins": {}, "bundles": {}, "decors": {},
	"launcher-images": {}, "fastfetch-emblems": {}, "zenith-tunes-skins": {},
}

func storeSection(section string) bool {
	_, ok := storeSections[section]
	return ok
}

// settingsSection maps a Store category to the Hub page that applies or manages
// it. The lockscreen and app-launcher pages are only for editing config, not for
// applying a store install, and the flat-image categories (decors, launcher
// images) have no page at all, so none of them appear here and none shows an
// "open in settings" action.
func settingsSection(category string) (string, bool) {
	section, ok := map[string]string{
		"rices":     "appearance",
		"plugins":   "addons",
		"bundles":   "addons",
		"barstyles": "bar-studio",
		"fastfetch": "fastfetch",
	}[category]
	return section, ok
}

func hasSettingsSection(category string) bool {
	_, ok := settingsSection(category)
	return ok
}

// storeEntry is the window's entry file and storeWrapper the script that starts
// it; the wrapper holds the single-instance lock. The wrapper is a variable only
// so the test can point it at a stand-in.
const storeEntry = "/usr/share/zenith/shell/store.qml"

var storeWrapper = "/usr/share/zenith/session/zenith-store"

func runOpen(args []string) error {
	if len(args) != 1 || !storeSection(args[0]) {
		return fmt.Errorf("open needs a valid section")
	}
	start := exec.Command(storeWrapper)
	if err := start.Start(); err != nil {
		return fmt.Errorf("start the store window: %w", err)
	}
	_ = start.Process.Release()

	// The window needs a moment before its IPC target answers; a second
	// instance exits at the lock and the call reaches the first.
	var last error
	for attempt := 0; attempt < 30; attempt++ {
		call := exec.Command("quickshell", "ipc", "--path", storeEntry, "call", "nav", "open", args[0])
		if last = call.Run(); last == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("open section %q: %w", args[0], last)
}

// runSettings opens Zenith Settings on the page that manages a category. The
// daemon owns the Hub window, so this asks it instead of starting one.
func runSettings(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("settings needs <category> [id]")
	}
	section, ok := settingsSection(args[0])
	if !ok {
		return fmt.Errorf("unknown settings category %q", args[0])
	}
	return exec.Command("zenith-shell", "hub", "open", section).Run()
}
