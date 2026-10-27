package portal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InterfaceFinding is one interface the configuration names a backend for, where
// the backend cannot actually serve it.
type InterfaceFinding struct {
	// Interface is the portal interface, in its impl form.
	Interface string
	// Backend is what the configuration named.
	Backend string
	// Reason is what is wrong, phrased so it says what to do.
	Reason string
}

// backendDir is where GLib reads portal backend descriptors from.
const backendDir = "/usr/share/xdg-desktop-portal/backends"

// backendDirs are the directories where xdg-desktop-portal searches for backend descriptors.
// Modern Linux distributions (Ubuntu, Debian, Arch) install .portal files into
// /usr/share/xdg-desktop-portal/portals.
var backendDirs = []string{
	"/usr/share/xdg-desktop-portal/portals",
	backendDir,
}

// CheckInterfaces verifies that every interface the configuration names a
// backend for is one that backend actually implements.
//
// It reads the backend's own .portal file rather than a table kept here, because
// the table would be a second thing to update and would drift from the packages
// that are actually installed. The .portal file is what the dispatcher reads, so
// it is the thing worth reading.
//
// Two failures, and they are not the same:
//
//   - the backend is not installed, which is a packaging fault and is found
//     before a login rather than during one;
//   - the backend is installed and does not implement the interface, which is a
//     configuration fault and is the one this catches by surprise.
//
// The second is the reason this exists. Secret named gtk for months. gtk is
// installed, gtk's .portal file is present and readable, and gtk does not
// implement Secret — so every other check passed and every Flatpak that stores a
// password still had nowhere to put one. A configuration that names a backend
// confidently is worse than one that names none, because a reader checking the
// file sees the interface handled.
func CheckInterfaces(env Env, config *Config) []InterfaceFinding {
	if config == nil {
		return nil
	}

	var findings []InterfaceFinding

	for section := range config.Sections {
		if section == "default" {
			continue
		}
		// Backend, not entries["use"]. The raw value still carries its trailing
		// semicolon — "use=hyprland;" — so reading the map directly looks for
		// "hyprland;.portal" and reports every backend as missing, which is
		// indistinguishable from none of them being installed.
		backend, ok := config.Backend(section)
		if !ok || backend == "" {
			continue
		}

		impl := canonicalInterface(section)

		var body []byte
		var err error
		for _, dir := range backendDirs {
			path := filepath.Join(env.Root, dir, backend+".portal")
			body, err = os.ReadFile(path)
			if err == nil {
				break
			}
		}
		if err != nil {
			findings = append(findings, InterfaceFinding{
				Interface: impl,
				Backend:   backend,
				Reason: fmt.Sprintf("named backend %q is not installed; "+
					"nothing implements %s", backend, impl),
			})
			continue
		}

		matched := declaresInterface(string(body), impl)
		if !matched && impl == "org.freedesktop.impl.portal.OpenURI" {
			matched = declaresInterface(string(body), "org.freedesktop.impl.portal.AppChooser")
		}

		if !matched {
			findings = append(findings, InterfaceFinding{
				Interface: impl,
				Backend:   backend,
				Reason: fmt.Sprintf("%s is installed but does not implement "+
					"%s; the dispatcher finds no backend for it and exports "+
					"nothing", backend, impl),
			})
		}
	}

	return findings
}

// canonicalInterface translates a portals.conf section name to the name a
// backend actually declares.
//
// portals.conf sections are named org.gnome.impl.portal.*, and backends declare
// org.freedesktop.impl.portal.*. The gnome namespace is a legacy alias the
// dispatcher resolves, and it exists because that is where the first portal
// interfaces were declared — it says nothing about whether GNOME is installed.
//
// Comparing the two as written would report every interface on every backend as
// unimplemented, which is a check that is always failing and therefore never
// read. This is the same trap the configuration file's own comment warns about,
// reached from the other end.
func canonicalInterface(section string) string {
	return strings.Replace(section,
		"org.gnome.impl.portal.",
		"org.freedesktop.impl.portal.", 1)
}

// declaresInterface reports whether a .portal file lists an interface.
//
// Matched on whole semicolon-separated elements rather than by substring,
// because `org.freedesktop.impl.portal.Notification` is a prefix of nothing but
// `org.freedesktop.impl.portal.NotificationFilter` is not a match for it, and a
// substring check would get that exactly backwards.
func declaresInterface(portalFile, impl string) bool {
	for _, line := range strings.Split(portalFile, "\n") {
		value, found := strings.CutPrefix(strings.TrimSpace(line), "Interfaces=")
		if !found {
			continue
		}
		for _, entry := range strings.Split(value, ";") {
			if strings.TrimSpace(entry) == impl {
				return true
			}
		}
	}
	return false
}
