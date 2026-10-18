package portal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installedTree builds a machine where every registration and every target
// exists, which is what a correct package produces.
func installedTree(t *testing.T) RegistrationEnv {
	t.Helper()
	root := t.TempDir()

	// Registrations are files, and their targets are files, so they are created
	// as files rather than as the directories mkdirAll above would leave.
	for _, registration := range registrations {
		mkdirAll(t, filepath.Dir(filepath.Join(root, registration.Path)))
		writeFile(t, filepath.Join(root, registration.Path), "[Desktop Entry]\n")

		if registration.Target == "" {
			continue // nothing to point at yet
		}
		mkdirAll(t, filepath.Dir(filepath.Join(root, registration.Target)))
		writeFile(t, filepath.Join(root, registration.Target), unitBody)
	}

	// Each unit points at its own binary, so the check has to read each rather
	// than find the same target in both. The binaries are the base layer's, not
	// Zenith's, and they are absent from this tree: the check saying so is the
	// point.
	writeFile(t, filepath.Join(root, "/usr/share/zenith/units/zenith-shell.service"),
		"[Service]\nExecStart=/usr/bin/quickshell -p /usr/share/zenith/shell\n")
	writeFile(t, filepath.Join(root, "/usr/share/zenith/units/zenith-portal.service"),
		"[Service]\nExecStart=/usr/libexec/xdg-desktop-portal\n")
	mkdirAll(t, filepath.Join(root, "/usr/share/zenith/shell"))
	mkdirAll(t, filepath.Join(root, "/usr/share/zenith/portal"))

	return RegistrationEnv{Root: root}
}

// unitBody is a service file with an ExecStart, which is what CheckUnits needs
// to find.
const unitBody = `[Unit]
Description=test

[Service]
Type=exec
ExecStart=/usr/share/zenith/shell/zenith-shell
`

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestEveryRegistrationIsSanctioned is the boundary check on the registration
// set.
//
// A registration outside the closed list is a boundary violation even when
// everything else is right, and it is exactly the kind of thing that arrives by
// someone adding one line to a packaging script.
func TestEveryRegistrationIsSanctioned(t *testing.T) {
	allowed := map[string]bool{
		"/usr/share/wayland-sessions/zenith.desktop":                     true,
		"/usr/share/xdg-desktop-portal/zenith-portals.conf":              true,
		"/usr/lib/systemd/user/zenith-shell.service":                     true,
		"/usr/lib/systemd/user/zenith-portal.service":                    true,
		"/usr/lib/systemd/user/hyprland-session.target.d/10-zenith.conf": true,
		"/etc/sddm.conf.d/10-zenith.conf":                                true,
		"/usr/share/glib-2.0/schemas/org.zenith.shell.gschema.xml":       true,
		"/usr/share/glib-2.0/schemas/org.zenith.desktop.gschema.xml":     true,
	}

	for _, registration := range registrations {
		if !registration.Sanctioned {
			t.Errorf("%s is not marked sanctioned", registration.Path)
		}
		if !allowed[registration.Path] {
			t.Errorf("%s is not on the sanctioned list in architecture.md",
				registration.Path)
		}
		// An empty target is a registration whose content is not written yet.
		// When there is one, it still has to live under /usr/share/zenith/ —
		// a registration is a pointer and its content belongs there.
		if registration.Target != "" &&
			!strings.HasPrefix(registration.Target, "/usr/share/zenith/") {
			t.Errorf("%s points at %s; a registration is a pointer and its content "+
				"belongs under /usr/share/zenith/",
				registration.Path, registration.Target)
		}
	}
}

// TestCheckRegistrationsPassesOnACompleteTree is the baseline.
func TestCheckRegistrationsPassesOnACompleteTree(t *testing.T) {
	if findings := CheckRegistrations(installedTree(t)); len(findings) != 0 {
		t.Errorf("a complete tree reported findings: %v", findings)
	}
}

// TestMissingRegistrationIsNamed covers the packaging fault: the file the
// package should have written is not there, and the desktop the user expected
// simply does not appear.
func TestMissingRegistrationIsNamed(t *testing.T) {
	env := installedTree(t)
	if err := os.RemoveAll(filepath.Join(env.Root,
		"/usr/share/wayland-sessions/zenith.desktop")); err != nil {
		t.Fatal(err)
	}

	findings := CheckRegistrations(env)
	if len(findings) == 0 {
		t.Fatal("a missing registration was not reported")
	}
	if findings[0].Installed {
		t.Error("a missing registration was reported as installed; those are " +
			"different faults with different fixes")
	}
	if !strings.Contains(findings[0].Reason, "not installed") {
		t.Errorf("reason = %q", findings[0].Reason)
	}
}

// TestRegistrationWithMissingTargetIsNamed covers the other fault: registered,
// pointing at nothing.
func TestRegistrationWithMissingTargetIsNamed(t *testing.T) {
	env := installedTree(t)
	if err := os.RemoveAll(filepath.Join(env.Root,
		"/usr/share/zenith/session/zenith-portals.conf")); err != nil {
		t.Fatal(err)
	}

	findings := CheckRegistrations(env)
	found := false
	for _, finding := range findings {
		if finding.Path == "/usr/share/xdg-desktop-portal/zenith-portals.conf" {
			found = true
			if !finding.Installed {
				t.Error("a registration whose target is missing was reported as " +
					"not installed")
			}
		}
	}
	if !found {
		t.Errorf("a registration pointing at nothing was not reported: %v", findings)
	}
}

// TestExecStartTargetIsExtracted covers the unit check's first half.
func TestExecStartTargetIsExtracted(t *testing.T) {
	dir := t.TempDir()
	unit := filepath.Join(dir, "x.service")
	body := "[Unit]\nDescription=x\n\n[Service]\nType=exec\n" +
		"ExecStart=/usr/share/zenith/shell/zenith-shell\nRestart=on-failure\n"
	if err := os.WriteFile(unit, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	target, ok := ExecStartTarget("", unit)
	if !ok || target != "/usr/share/zenith/shell/zenith-shell" {
		t.Errorf("ExecStartTarget = %q, %v", target, ok)
	}

	// A unit with no ExecStart must say so rather than reporting a stale target.
	empty := filepath.Join(dir, "empty.service")
	if err := os.WriteFile(empty, []byte("[Unit]\nDescription=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := ExecStartTarget("", empty); ok {
		t.Error("a unit with no ExecStart reported one")
	}
}

// TestUnitPointingAtNothingIsFound covers the fault this milestone actually hit.
//
// The unit is packaged, the registration target exists, systemd is entirely
// happy with the syntax — and the service exits 203/EXEC on every login because
// the binary it runs was never installed. Nothing warns about it at build time.
func TestUnitPointingAtNothingIsFound(t *testing.T) {
	env := installedTree(t)
	findings := CheckUnits(env)

	if len(findings) == 0 {
		t.Fatal("units pointing at binaries that do not exist were not found")
	}
	for _, finding := range findings {
		if !strings.Contains(finding.Reason, "does not exist") {
			t.Errorf("%s: reason = %q", finding.Name, finding.Reason)
		}
	}
}

// TestUnitsPassWhenTheirBinariesExist is the other direction, so the check is
// not simply always reporting.
func TestUnitsPassWhenTheirBinariesExist(t *testing.T) {
	env := installedTree(t)
	for _, binary := range []string{
		"/usr/bin/quickshell",
		"/usr/libexec/xdg-desktop-portal",
	} {
		path := filepath.Join(env.Root, binary)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if findings := CheckUnits(env); len(findings) != 0 {
		t.Errorf("a complete unit tree reported findings: %v", findings)
	}
}
