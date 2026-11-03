// Package svcmgr is how Asura's tools drive the session's user services.
//
// In Asura OS on Void Linux, that is sv(8) standing in front of the services seeded
// into ~/.config/service under turnstile's runsvdir / asura-user-services.
package svcmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Kind is the service-manager family a Manager resolves operations against.
type Kind string

const (
	Runit Kind = "runit"
)

// Runner stands in for os/exec in tests. runner.Func satisfies it directly.
type Runner interface {
	Run(name string, args ...string) (string, error)
}

// Manager resolves session-service operations against runit.
type Manager struct {
	Kind Kind
	run  Runner
}

// New returns a Manager for kind. run is required.
func New(kind Kind, run Runner) *Manager {
	if run == nil {
		panic("svcmgr: New needs a runner, as a test without one would reach the live session")
	}
	return &Manager{Kind: Runit, run: run}
}

// Detect answers Runit on Asura OS.
func Detect() Kind {
	return Runit
}

// unitService maps legacy unit names to the runit service directory name.
var unitService = map[string]string{
	"asura-shell.service":          "quickshell",
	"asurad.service":               "asurad",
	"asura-hubd.service":           "asura-hubd",
	"asura-wallpaper.service":      "asura-wallpaper",
	"asura-idle.service":           "asura-idle",
	"asura-agent.service":          "asura-agent",
	"asura-palette-bridge.service": "asura-palette-bridge",
	"asura-portal.service":         "portal",
}

// RunitServiceName maps a unit name to its runit service directory name.
func RunitServiceName(unit string) string {
	if name, ok := unitService[unit]; ok {
		return name
	}
	for _, suffix := range []string{".service", ".target", ".timer", ".socket"} {
		if strings.HasSuffix(unit, suffix) {
			return strings.TrimSuffix(unit, suffix)
		}
	}
	return unit
}

// ServicesDir is where a user's runit services live (~/.config/service).
func ServicesDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "service")
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "service")
	}
	return ".config/service"
}

// SvcDirPath is the path sv(8) must be handed for a unit's service.
func SvcDirPath(unit string) string {
	return filepath.Join(ServicesDir(), RunitServiceName(unit))
}

// systemctlUnit trims legacy unit suffixes for system services in /var/service.
func systemctlUnit(unit string) string {
	for _, suffix := range []string{".service", ".target", ".timer", ".socket"} {
		if strings.HasSuffix(unit, suffix) {
			return strings.TrimSuffix(unit, suffix)
		}
	}
	return unit
}

// Active reports whether a user service is running under runit.
func (m *Manager) Active(unit string) bool {
	out, err := m.run.Run("sv", "status", SvcDirPath(unit))
	return err == nil && strings.HasPrefix(strings.TrimSpace(firstLine(out)), "run:")
}

// Enabled reports whether a user service survives the next login (no `down` marker).
func (m *Manager) Enabled(unit string) bool {
	dir := filepath.Join(ServicesDir(), RunitServiceName(unit))
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "down"))
	return err != nil
}

// EnableNow enables a service (removes `down` marker) and starts it (sv up).
func (m *Manager) EnableNow(unit string) error {
	dir := filepath.Join(ServicesDir(), RunitServiceName(unit))
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("service directory %s is not seeded", dir)
	}
	_ = os.Remove(filepath.Join(dir, "down"))
	_, err := m.run.Run("sv", "up", dir)
	return err
}

// DisableNow stops a service (sv down) and lays the `down` marker.
func (m *Manager) DisableNow(unit string) error {
	dir := filepath.Join(ServicesDir(), RunitServiceName(unit))
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("service directory %s is not seeded", dir)
	}
	if _, err := m.run.Run("sv", "down", dir); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "down"), nil, 0o644)
}

// Restart restarts a user service.
func (m *Manager) Restart(unit string) error {
	_, err := m.run.Run("sv", "restart", SvcDirPath(unit))
	return err
}

// Reload restarts a user service under runit.
func (m *Manager) Reload(unit string) error {
	_, err := m.run.Run("sv", "restart", SvcDirPath(unit))
	return err
}

// TryReloadOrRestart restarts a user service under runit.
func (m *Manager) TryReloadOrRestart(unit string) error {
	return m.Restart(unit)
}

// Summary returns one service's state as a human-readable phrase.
func (m *Manager) Summary(unit string) string {
	dir := filepath.Join(ServicesDir(), RunitServiceName(unit))
	out, _ := m.run.Run("sv", "status", dir)
	line := firstLine(out)
	if line == "" {
		if _, statErr := os.Stat(filepath.Join(dir, "run")); statErr == nil {
			return "active (running)"
		}
		return "not installed"
	}
	service := RunitServiceName(unit)
	switch {
	case strings.HasPrefix(line, "run: "):
		return "active (running)"
	case strings.HasPrefix(line, "want: "):
		return "active (waiting)"
	case strings.HasPrefix(line, "down: "+service+":"), strings.HasPrefix(line, "down:"):
		return "inactive"
	default:
		return "inactive"
	}
}

// firstLine is a program's first non-empty output line.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// MainPID returns the service's PID or empty string.
func (m *Manager) MainPID(unit string) string {
	out, err := m.run.Run("sv", "status", SvcDirPath(unit))
	if err != nil {
		return ""
	}
	if line := firstLine(out); strings.HasPrefix(line, "run:") {
		if _, rest, found := strings.Cut(line, "(pid "); found {
			if pid, _, _ := strings.Cut(rest, ")"); pid != "" {
				return strings.TrimSpace(pid)
			}
		}
	}
	return ""
}

// SetUserEnv publishes a variable to user runit via turnstile-update-runit-env.
func (m *Manager) SetUserEnv(keyval string) error {
	_, err := m.run.Run("turnstile-update-runit-env", keyval)
	return err
}

// SystemActive reports whether a system service under /var/service is running.
func (m *Manager) SystemActive(unit string) bool {
	name := systemctlUnit(unit)
	_, err := m.run.Run("sv", "status", "/var/service/"+name)
	return err == nil
}

// PowerArgv resolves a session power action (reboot, poweroff) to loginctl.
var powerActions = map[string][]string{
	"reboot":   {"loginctl", "reboot"},
	"poweroff": {"loginctl", "poweroff"},
}

// PowerArgv returns the argv for a power action or false if unsupported.
func PowerArgv(kind Kind, action string) ([]string, bool) {
	argv, ok := powerActions[action]
	return argv, ok
}
