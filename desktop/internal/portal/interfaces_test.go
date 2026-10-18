package portal

import (
	"strings"
	"testing"
)

// TestCanonicalInterfaceTranslation covers the alias that makes every other
// check in this file useless if it is missed.
//
// portals.conf names sections org.gnome.impl.portal.* and backends declare
// org.freedesktop.impl.portal.*. Compared as written, every interface on every
// backend reads as unimplemented — a check that is always failing and therefore
// never read.
func TestCanonicalInterfaceTranslation(t *testing.T) {
	got := canonicalInterface("org.gnome.impl.portal.Secret")
	if got != "org.freedesktop.impl.portal.Secret" {
		t.Errorf("canonicalInterface = %q", got)
	}

	// An already-freedesktop name must survive unchanged.
	got = canonicalInterface("org.freedesktop.impl.portal.FileChooser")
	if got != "org.freedesktop.impl.portal.FileChooser" {
		t.Errorf("an already-canonical name was changed: %q", got)
	}
}

// TestCameraIsNotABackendInterface covers the misreading that made this look
// unfixable.
//
// No backend implements org.freedesktop.portal.Camera, and the natural reading of
// that is "Camera is unsupported and someone has to write a backend". It is not:
// desktop-portal/camera.c is part of the xdg-desktop-portal binary itself. The
// dispatcher implements Camera over PipeWire and asks an Access backend for
// consent before opening a device.
//
// So Camera must never appear in the required-backend list. Naming a backend for
// it would be demanding something no backend has, and the resulting report would
// read as "this desktop has no Camera backend" — which is true, and not the
// problem.
func TestCameraIsNotABackendInterface(t *testing.T) {
	for _, want := range required {
		if strings.Contains(want.section, "Camera") {
			t.Fatalf("%s is in the required-backend list; Camera is "+
				"implemented by the dispatcher, not by a backend", want.section)
		}
	}

	// The thing that is actually required is Access, because that is what the
	// dispatcher's Camera portal asks for.
	found := false
	for _, want := range required {
		if want.section == "org.gnome.impl.portal.Access" {
			found = true
			if want.backend != "gtk" {
				t.Errorf("Access names %q; only gtk is required to supply it",
					want.backend)
			}
		}
	}
	if !found {
		t.Error("Access is not required; without it the dispatcher's Camera " +
			"portal has nothing to ask for consent")
	}
}
