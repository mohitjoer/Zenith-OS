package portal

import "testing"

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
