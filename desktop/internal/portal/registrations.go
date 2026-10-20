package portal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Registration is a pointer a third-party binary has to be able to find.
//
// The architecture record enforces the base-layer boundary as a closed list
// rather than an absolute, because a handful of files must live outside
// /usr/share/zenith/ and /etc/zenith/: a systemd unit directory, a GLib schema
// directory, the login manager's fixed config directory. Those files are
// registrations, and a registration is a pointer and never a place to put
// content.
//
// That gives this checker its shape. A registration whose target is missing is
// the failure worth catching, and it is a failure that never announces itself:
// systemd starts nothing and says nothing, SDDM greets the user and offers no
// session, and the desktop the user expected is simply absent.
type Registration struct {
	// Name is what the checker calls it.
	Name string
	// Path is the registration's own location, the fixed directory something scans.
	Path string
	// Target is what it points at.
	Target string
	// Sanctioned records whether Path appears on the closed list in
	// docs/architecture.md. A registration outside that list is a boundary
	// violation even when everything else is right.
	Sanctioned bool
}

// registrations are the pointers this milestone ships, and the fixed directory
// each one has to live in.
//
// The list is the reason the check can be mechanical: "everything Zenith
// registers outside its own two directories" is a closed set, so a registration
// that is not on it is a finding rather than something nobody thought to look.
var registrations = []Registration{
	{
		Name:       "session file",
		Path:       "/usr/share/wayland-sessions/zenith.desktop",
		Target:     "/usr/share/zenith/session/zenith.desktop",
		Sanctioned: true,
	},
	{
		Name:       "portal preference",
		Path:       "/usr/share/xdg-desktop-portal/zenith-portals.conf",
		Target:     "/usr/share/zenith/session/zenith-portals.conf",
		Sanctioned: true,
	},
	{
		Name:       "shell unit",
		Path:       "/usr/lib/systemd/user/zenith-shell.service",
		Target:     "/usr/share/zenith/units/zenith-shell.service",
		Sanctioned: true,
	},
	{
		Name:       "portal unit",
		Path:       "/usr/lib/systemd/user/zenith-portal.service",
		Target:     "/usr/share/zenith/units/zenith-portal.service",
		Sanctioned: true,
	},
	{
		Name:       "session target",
		Path:       "/usr/lib/systemd/user/zenith-session.target",
		Target:     "/usr/share/zenith/units/zenith-session.target",
		Sanctioned: true,
	},
	{
		Name:       "shell schema",
		Path:       "/usr/share/glib-2.0/schemas/org.zenith.shell.gschema.xml",
		Target:     "",
		Sanctioned: true,
	},
	{
		Name:       "desktop schema",
		Path:       "/usr/share/glib-2.0/schemas/org.zenith.desktop.gschema.xml",
		Target:     "",
		Sanctioned: true,
	},
	{
		Name: "login manager preferences",
		Path: "/etc/sddm.conf.d/10-zenith.conf",
		// No target yet, because there is no greeter. The drop-in's whole
		// purpose is DisplayServer=wayland with CompositorCommand=zenith-greeter,
		// and shipping it before the greeter exists would take the login screen
		// down rather than leave it alone: SDDM has no way to start a Wayland
		// greeter that is not there. It is registered here so the gap is visible
		// rather than forgotten, and it ships with the greeter.
		Target:     "",
		Sanctioned: true,
	},
}

// RegistrationEnv is what the checker reads.
type RegistrationEnv struct {
	// Root prefixes every path, for the same reason the probes take one.
	Root string
}

// RegistrationFinding is one problem with one registration.
type RegistrationFinding struct {
	// Name is the registration.
	Name string
	// Path is where it should be.
	Path string
	// Reason is the problem, phrased so it says what to do.
	Reason string
	// Installed is false when the registration is not on disk at all, which is
	// different from being on disk and pointing at nothing.
	Installed bool
}

// CheckRegistrations verifies that every registration is present and that its
// target exists.
//
// Both halves matter and they fail differently. A registration that is not
// installed means the package that should have written it did not — a packaging
// fault, found here rather than by a user whose desktop did not appear. A
// registration whose target is missing means the content was not shipped — also
// a packaging fault, and also silent.
//
// Content that exists and is not registered is not reported. That is a different
// fault with a different fix, and mixing them would make this check report
// things it cannot diagnose.
func CheckRegistrations(env RegistrationEnv) []RegistrationFinding {
	var findings []RegistrationFinding

	for _, registration := range registrations {
		if !registration.Sanctioned {
			findings = append(findings, RegistrationFinding{
				Name: registration.Name,
				Path: registration.Path,
				Reason: "this path is outside /usr/share/zenith/ and /etc/zenith/ " +
					"and is not on the sanctioned list in docs/architecture.md",
			})
			continue
		}

		registered := env.path(registration.Path)
		if _, err := os.Stat(registered); err != nil {
			findings = append(findings, RegistrationFinding{
				Name:   registration.Name,
				Path:   registration.Path,
				Reason: "not installed; the package that should write it did not",
			})
			continue
		}

		// A registration whose content is not written yet is checked for presence
		// alone. There is nothing to point at.
		if registration.Target == "" {
			continue
		}

		target := env.path(registration.Target)
		if _, err := os.Stat(target); err != nil {
			findings = append(findings, RegistrationFinding{
				Name:      registration.Name,
				Path:      registration.Path,
				Reason:    fmt.Sprintf("registered, but %s does not exist", registration.Target),
				Installed: true,
			})
		}
	}

	return findings
}

// Registrations returns the registrations, for a report that wants to show what
// was checked rather than only what failed.
func Registrations() []Registration {
	return append([]Registration(nil), registrations...)
}

// ExecStartTarget reads the ExecStart= target out of a unit file, if it has one.
//
// It exists for the check that catches the failure this milestone actually hit:
// a unit installed correctly, pointing at a binary that was never packaged. The
// unit is syntactically fine, systemd is happy, and the service exits 203/EXEC
// once per login.
func ExecStartTarget(root, unitPath string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(root, unitPath))
	if err != nil {
		return "", false
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		value, found := strings.CutPrefix(line, "ExecStart=")
		if !found {
			continue
		}
		// A prefixed ExecStart= carries an argument character; the executable is
		// still the first field.
		value = strings.TrimSpace(value)
		if fields := strings.Fields(value); len(fields) > 0 {
			return fields[0], true
		}
	}
	return "", false
}

// CheckUnits verifies that every unit this milestone ships points at an
// executable that the package actually installed.
//
// This is a separate check from CheckRegistrations because it catches a
// different fault: the unit is present and its registration target exists, but
// the binary the unit runs does not. Nothing warns about that at build time and
// the symptom is a service that fails once per login with a code nobody reads.
func CheckUnits(env RegistrationEnv) []RegistrationFinding {
	var findings []RegistrationFinding

	units := []struct{ name, path string }{
		{"zenith-shell.service", "/usr/share/zenith/units/zenith-shell.service"},
		{"zenith-portal.service", "/usr/share/zenith/units/zenith-portal.service"},
	}

	for _, unit := range units {
		target, found := ExecStartTarget(env.Root, unit.path)
		if !found {
			findings = append(findings, RegistrationFinding{
				Name:   unit.name,
				Path:   unit.path,
				Reason: "no ExecStart= line, so there is nothing to run",
			})
			continue
		}

		if _, err := os.Stat(env.path(target)); err != nil {
			findings = append(findings, RegistrationFinding{
				Name:      unit.name,
				Path:      unit.path,
				Reason:    fmt.Sprintf("ExecStart points at %s, which does not exist", target),
				Installed: true,
			})
		}
	}

	// The shell's own content. `quickshell -p /usr/share/zenith/shell` exits
	// cleanly and immediately when that directory is empty or missing, so a
	// missing config directory produces no error from the unit, no entry in the
	// journal beyond a line nobody reads, and a login with no shell on it. It is
	// checked here for the same reason the ExecStart targets are: a missing input
	// is only visible before the login, not during it.
	findings = append(findings, checkDirectory(env, "shell configuration",
		"/usr/share/zenith/shell")...)

	return findings
}

// checkDirectory asserts that a required directory exists and is a directory.
//
// Stat alone is not enough: a regular file where a directory belongs makes the
// reader fail later, with the same silence.
func checkDirectory(env RegistrationEnv, name, absolute string) []RegistrationFinding {
	info, err := os.Stat(env.path(absolute))
	if err != nil {
		return []RegistrationFinding{{
			Name:   name,
			Path:   absolute,
			Reason: "does not exist",
		}}
	}
	if !info.IsDir() {
		return []RegistrationFinding{{
			Name:   name,
			Path:   absolute,
			Reason: "exists but is not a directory",
		}}
	}
	return nil
}

func (e RegistrationEnv) path(absolute string) string {
	if e.Root == "" {
		return absolute
	}
	return filepath.Join(e.Root, absolute)
}
