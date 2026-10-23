package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// productName is the one place the assistant's name is spelled. The binary, the
// systemd unit, the socket, the config, data, state and runtime directories, the
// environment prefix and the markers written into other agents' files all
// derive from it. Renaming the product is this line plus the names a compiler
// cannot compute (the cmd directory, the unit file, the doc, the Hub page and the
// shell's call sites); product_test.go fails until those follow.
const productName = "agent"

const (
	// binName is the executable and the CLI every QML call site runs.
	binName = "zenith-" + productName
	// unitName is the systemd user unit that keeps the daemon up.
	unitName = binName + ".service"
	// socketName lives directly in $XDG_RUNTIME_DIR.
	socketName = binName + ".sock"
)

// displayName is what a person reads: "Zenith Agent".
var displayName = "Zenith " + strings.ToUpper(productName[:1]) + productName[1:]

// envName spells one environment override: envName("VAULT") is ZENITH_AGENT_VAULT.
func envName(suffix string) string {
	return strings.ToUpper("zenith_" + productName + "_" + suffix)
}

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}

// VaultDir is the knowledge base every agent reads and writes.
func VaultDir() string {
	if d := os.Getenv(envName("VAULT")); d != "" {
		return d
	}
	return filepath.Join(dataHome(), "zenith", productName)
}

// ConfigPath holds the enabled gate and the port.
func ConfigPath() string {
	return filepath.Join(configHome(), "zenith", productName, "config.json")
}

// StateDir holds what can be regenerated: ask history, recipes, run reports.
func StateDir() string {
	if d := os.Getenv(envName("STATE")); d != "" {
		return d
	}
	return filepath.Join(xdgState(), "zenith", productName)
}

// statePath is one named file under StateDir.
func statePath(name string) string { return filepath.Join(StateDir(), name) }

// runtimeRoot is $XDG_RUNTIME_DIR, which is per-user and 0700 and is the whole
// access control of the socket. Without it the fallback is a directory of our own
// under the temp dir, never a shared path.
func runtimeRoot() string {
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		return rt
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", binName, os.Getuid()))
}

// RuntimeDir holds the lock and the pidfile.
func RuntimeDir() string { return filepath.Join(runtimeRoot(), binName) }

// SocketPath is the unix socket the CLI, the shell and the Hub talk to.
func SocketPath() string { return filepath.Join(runtimeRoot(), socketName) }
