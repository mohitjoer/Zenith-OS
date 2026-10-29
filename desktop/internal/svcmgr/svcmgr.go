// Package svcmgr is how Zenith's tools drive the session's user services.
//
// Every control command ends in someone else's program: on the reference base,
// Void Linux, that is sv(8) standing in front of the services seeded into
// ~/.config/service under turnstile's runsvdir; on the systemd machines the
// project is developed on, it is systemctl --user. Rather than carrying both
// sets of verbs and an if-statement at every call site, a call site asks for an
// operation and a Manager resolves it on whatever base it is on.
//
// The base is where the division actually lives, because runit has fewer verbs
// than systemd and a different idea of "enabled". Runit's enabled is nothing
// more than a service directory present in the watched tree and carrying no
// `down` file; runit has no reload, so a reload takes the closest promise a
// restart gives — the unit comes back fresh — and the caller already checks
// that outcome, because it checks it for systemd too. An operation runit could
// not say no to is written as the systemd verb it most resembles, and the
// difference is worth one comment rather than a call-site if-chain.
//
// Detection never runs a program: it reads PID 1's comm and the tools the
// machine has, and answers on what it saw. A container with a cropped /proc
// answers unknown, and a call site that wants a second opinion gets one from
// its own LookPath, not from a different guess here.
package svcmgr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Kind is the service-manager family a Manager resolves operations against.
type Kind string

const (
	Systemd Kind = "systemd"
	Runit   Kind = "runit"
	Unknown Kind = "unknown"
)

// Runner stands in for os/exec in tests. runner.Func satisfies it directly,
// so a package's own runner drops in without an adapter.
type Runner interface {
	Run(name string, args ...string) (string, error)
}

// Manager resolves session-service operations against a service-manager family.
//
// kind is decided once per command run — by Detect at the entry point — and
// handed in with the runner, so a test on a systemd machine constructs a
// runit Manager with a fake and tries the other family honestly.
type Manager struct {
	Kind Kind
	run  Runner
}

// New returns a Manager for kind. run is required: the zero runner here is
// the same trap it is in package runner, and the same answer — construct
// without one and the missing dependencies are loud, not silent.
func New(kind Kind, run Runner) *Manager {
	if run == nil {
		panic("svcmgr: New needs a runner, as a test without one would reach the live session")
	}
	return &Manager{Kind: kind, run: run}
}

// Detect answers which family supervises this machine from PID 1, falling
// back to the tools that are present when /proc is not the whole story —
// a chroot or a container can run one family's tools beside another's init.
func Detect() Kind {
	if comm, err := os.ReadFile("/proc/1/comm"); err == nil {
		if kind := kindFromComm(string(comm)); kind != Unknown {
			return kind
		}
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		return Systemd
	}
	if _, err := exec.LookPath("sv"); err == nil {
		return Runit
	}
	return Unknown
}

// kindFromComm maps PID 1's comm to a family. The kernel writes kernel-thread
// names in brackets; userspace comms are bare, and a systemd PID 1 can report
// either form.
func kindFromComm(comm string) Kind {
	switch strings.TrimSpace(strings.Trim(strings.TrimSpace(comm), "()")) {
	case "runit":
		return Runit
	case "systemd":
		return Systemd
	}
	return Unknown
}

// unitService maps a systemctl unit name to the runit service directory name
// when runit and systemd name the same job differently. The desktop's names
// were the systemd names first and the runit services were seeded with their
// own, so the mapping records the decision instead of leaving each caller to
// rediscover it — or to diverge from the docs.
var unitService = map[string]string{
	"zenith-shell.service":          "quickshell",
	"zenithd.service":               "zenithd",
	"zenith-hubd.service":           "zenith-hubd",
	"zenith-wallpaper.service":      "zenith-wallpaper",
	"zenith-idle.service":           "zenith-idle",
	"zenith-agent.service":          "zenith-agent",
	"zenith-palette-bridge.service": "zenith-palette-bridge",
	"zenith-portal.service":         "portal",
}

// RunitServiceName maps a unit name to its runit service directory name. An
// unmapped unit keeps its base name: the seeded dirs carry the unit names the
// manifest puts in them, so the default is right more often than not.
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

// ServicesDir is where a user's runit services live. Turnstile seeds it at
// login and runsvdir supervises it; the same layout is what
// session/runit/zenith-user-services lays down on a machine that has no
// turnstile yet.
func ServicesDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "service")
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "service")
	}
	return ".config/service"
}

// SvcDirPath is the path sv(8) must be handed for a unit's service. Path form,
// not bare name: runit's sv defaults SVDIR to /var/service, which hold the
// system's services, and a bare name would read the wrong tree.
func SvcDirPath(unit string) string {
	return filepath.Join(ServicesDir(), RunitServiceName(unit))
}

// systemctlUnit trims the suffix systemd's unit names carry, for the system
// service checks where runit's /var/service dirs are named bare.
func systemctlUnit(unit string) string {
	for _, suffix := range []string{".service", ".target", ".timer", ".socket"} {
		if strings.HasSuffix(unit, suffix) {
			return strings.TrimSuffix(unit, suffix)
		}
	}
	return unit
}

// Active reports whether a user service is running.
func (m *Manager) Active(unit string) bool {
	switch m.Kind {
	case Runit:
		out, err := m.run.Run("sv", "status", SvcDirPath(unit))
		return err == nil && strings.HasPrefix(strings.TrimSpace(firstLine(out)), "run:")
	default:
		_, err := m.run.Run("systemctl", "--user", "is-active", "--quiet", unit)
		return err == nil
	}
}

// Enabled reports whether a user service survives the next login: a systemd
// unit with the enable bit, a runit service directory carrying no `down`
// marker.
func (m *Manager) Enabled(unit string) bool {
	switch m.Kind {
	case Runit:
		dir := filepath.Join(ServicesDir(), RunitServiceName(unit))
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return false
		}
		_, err = os.Stat(filepath.Join(dir, "down"))
		return err != nil
	default:
		_, err := m.run.Run("systemctl", "--user", "is-enabled", "--quiet", unit)
		return err == nil
	}
}

// EnableNow enables a service and starts it — enable --now in both families.
// Under runit that is the `down` marker's removal followed by sv up. When the
// service directory has not been seeded into the user's tree, enabling is a
// reported error rather than a pretended success.
func (m *Manager) EnableNow(unit string) error {
	switch m.Kind {
	case Runit:
		dir := filepath.Join(ServicesDir(), RunitServiceName(unit))
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("service directory %s is not seeded", dir)
		}
		_ = os.Remove(filepath.Join(dir, "down"))
		_, err := m.run.Run("sv", "up", dir)
		return err
	default:
		_, err := m.run.Run("systemctl", "--user", "enable", "--now", unit)
		return err
	}
}

// DisableNow stops a service and keeps it from starting again — a systemd
// disable --now, a runit sv down plus the `down` marker that turns the stop
// into something that survives a login.
func (m *Manager) DisableNow(unit string) error {
	switch m.Kind {
	case Runit:
		dir := filepath.Join(ServicesDir(), RunitServiceName(unit))
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("service directory %s is not seeded", dir)
		}
		if _, err := m.run.Run("sv", "down", dir); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "down"), nil, 0o644)
	default:
		_, err := m.run.Run("systemctl", "--user", "disable", "--now", unit)
		return err
	}
}

// Restart restarts a user service. Both families carry the verb, and the
// outcomes callers check afterwards — a new PID, a ready shell — are the same.
func (m *Manager) Restart(unit string) error {
	switch m.Kind {
	case Runit:
		_, err := m.run.Run("sv", "restart", SvcDirPath(unit))
		return err
	default:
		_, err := m.run.Run("systemctl", "--user", "restart", unit)
		return err
	}
}

// Reload re-reads a running unit's configuration. Runit has no reload, so a
// reload takes the closest promise a restart gives — the unit comes back
// fresh — and the caller checking for a new PID and a ready answer makes the
// promise, because it makes it for systemd as well.
func (m *Manager) Reload(unit string) error {
	switch m.Kind {
	case Runit:
		_, err := m.run.Run("sv", "restart", SvcDirPath(unit))
		return err
	default:
		_, err := m.run.Run("systemctl", "--user", "reload", unit)
		return err
	}
}

// TryReloadOrRestart does what systemctl's try-reload-or-restart does: reload
// when the unit supports it, restart when it does not. Runit has one answer
// to both, and that answer is the restart.
func (m *Manager) TryReloadOrRestart(unit string) error {
	switch m.Kind {
	case Runit:
		return m.Restart(unit)
	default:
		_, err := m.run.Run("systemctl", "--user", "try-reload-or-restart", unit)
		return err
	}
}

// Summary is one service's state as the phrase the reports print — the shape
// of "active (running)", "not installed", and the manager-not-reachable
// answer. The phrase's consumers sit outside this package, so both families
// answer with the same words.
func (m *Manager) Summary(unit string) string {
	switch m.Kind {
	case Runit:
		dir := filepath.Join(ServicesDir(), RunitServiceName(unit))
		out, _ := m.run.Run("sv", "status", dir)
		line := firstLine(out)
		if line == "" {
			// sv itself could not speak: no runsvdir over the dir. Whether the
			// service exists decides which brief the caller is owed.
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
		case strings.HasPrefix(line, "down: "+service+":"):
			return "inactive"
		default:
			return "inactive"
		}
	default:
		out, err := m.run.Run("systemctl", "--user", "show", unit,
			"-p", "LoadState", "-p", "ActiveState", "-p", "SubState", "--no-pager")
		if err != nil {
			return "unknown (the user service manager is not reachable)"
		}
		state := map[string]string{}
		for _, item := range strings.Split(out, "\n") {
			if key, value, found := strings.Cut(strings.TrimSpace(item), "="); found {
				state[key] = value
			}
		}
		switch {
		case state["LoadState"] == "not-found":
			return "not installed"
		case state["SubState"] != "" && state["SubState"] != state["ActiveState"]:
			return state["ActiveState"] + " (" + state["SubState"] + ")"
		}
		return state["ActiveState"]
	}
}

// firstLine is a program's first non-empty output line; sv(8) prints one
// status line per service.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// MainPID is the service's main pid, "" when the manager cannot say. The
// shell-reload flow waits for the pid to change; under runit, the pid rides
// on the sv status line's "(pid N)".
func (m *Manager) MainPID(unit string) string {
	switch m.Kind {
	case Runit:
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
	default:
		out, err := m.run.Run("systemctl", "--user", "show", "-p", "MainPID", "--value", unit)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(out)
	}
}

// SetUserEnv publishes one variable into the user session so the services
// started after it inherit it.
//
// Under systemd that is the user manager's environment. Under turnstile's
// runit backend the session's variables travel in its envdir, and the helper
// Void ships for the job is what updates them. The dbus activation
// environment — the other consumer, for services activated on demand — is
// outside this package: callers update it beside this call, where they need
// it, and the two writes are halves of one fact and not one operation.
func (m *Manager) SetUserEnv(keyval string) error {
	switch m.Kind {
	case Runit:
		_, err := m.run.Run("turnstile-update-runit-env", keyval)
		return err
	default:
		_, err := m.run.Run("systemctl", "--user", "set-environment", keyval)
		return err
	}
}

// SystemActive reports whether a SYSTEM service — one under /var/service on
// the runit side, under /etc/systemd/system on the other — is running.
// Reading state needs no privilege on either base.
func (m *Manager) SystemActive(unit string) bool {
	name := systemctlUnit(unit)
	switch m.Kind {
	case Runit:
		_, err := m.run.Run("sv", "status", "/var/service/"+name)
		return err == nil
	default:
		_, err := m.run.Run("systemctl", "is-active", "--quiet", unit)
		return err == nil
	}
}

// PowerArgv resolves a session power action to the program that performs it —
// reboot and poweroff, the two argv actions the session carries.
//
// On systemd, systemctl carries the verbs. On the Void base, elogind carries
// the same logind API the session daemon already speaks, and its loginctl
// carries the same verbs; runit itself stays out of the way. Data, not code:
// the mapping is what gets reviewed and tested, never the off.
var powerTables = map[Kind]map[string][]string{
	Systemd: {
		"reboot":   {"systemctl", "reboot"},
		"poweroff": {"systemctl", "poweroff"},
	},
	Runit: {
		"reboot":   {"loginctl", "reboot"},
		"poweroff": {"loginctl", "poweroff"},
	},
}

// PowerArgv returns the argv for a power action on a kind, or false for an
// action the base has no powered answer to. An unknown kind answers with the
// systemd table, the base the project is developed on talks anyway.
func PowerArgv(kind Kind, action string) ([]string, bool) {
	table := powerTables[Systemd]
	if kind == Runit {
		table = powerTables[Runit]
	}
	argv, ok := table[action]
	return argv, ok
}
