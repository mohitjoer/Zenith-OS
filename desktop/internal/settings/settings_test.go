package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSchema = `<?xml version="1.0"?>
<schemalist>
  <enum id="org.zenith.test.Edge">
    <value nick="top" value="0"/>
    <value nick="left" value="2"/>
  </enum>
  <schema id="org.zenith.test" path="/org/zenith/test/">
    <key name="name" type="s"><default>'x'</default></key>
    <key name="count" type="i"><default>3</default><range min="1" max="10"/></key>
    <key name="enabled" type="b"><default>false</default></key>
    <key name="items" type="as"><default>[]</default></key>
    <key name="edge" enum="org.zenith.test.Edge"><default>'top'</default></key>
  </schema>
</schemalist>`

func testCatalogue(t *testing.T) *Catalogue {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "org.zenith.test.gschema.xml"),
		[]byte(testSchema), 0o644); err != nil {
		t.Fatal(err)
	}
	catalogue, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return catalogue
}

// TestOnlyZenithSchemasAreLoaded covers the filter, which is not a convenience.
//
// /usr/share/glib-2.0/schemas on a real machine holds every schema the desktop
// has. Reading all of it gives a catalogue of several hundred GNOME settings,
// and `zenith config list` then lists GNOME's preferences — and the Hub's page
// registry gets checked against them, which says nothing about Zenith.
func TestOnlyZenithSchemasAreLoaded(t *testing.T) {
	dir := t.TempDir()
	write := func(name, id string) {
		body := `<?xml version="1.0"?><schemalist><schema id="` + id +
			`" path="/x/"><key name="k" type="s"><default>''</default></key>` +
			`</schema></schemalist>`
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("org.zenith.test.gschema.xml", "org.zenith.test")
	write("org.gnome.desktop.interface.gschema.xml", "org.gnome.desktop.interface")
	write("org.gtk.Settings.gschema.xml", "org.gtk.Settings.FileChooser")

	catalogue, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if got := len(catalogue.Schemas()); got != 1 {
		t.Errorf("loaded %d schema(s), want 1: %v", got, catalogue.Schemas())
	}
	if got := len(catalogue.Refs()); got != 1 {
		t.Errorf("loaded %d setting(s), want 1", got)
	}
}

// TestLoadFailsWhenNothingIsOurs covers the case that would otherwise be an
// empty list, which reads as "Zenith has no settings".
func TestLoadFailsWhenNothingIsOurs(t *testing.T) {
	dir := t.TempDir()
	body := `<?xml version="1.0"?><schemalist><schema id="org.gnome.desktop.interface" path="/x/">
		<key name="k" type="s"><default>''</default></key></schema></schemalist>`
	if err := os.WriteFile(filepath.Join(dir, "gnome.gschema.xml"),
		[]byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("loading a directory with no Zenith schemas succeeded")
	}
}

// TestRangeIsEnforcedBeforeTheWrite covers the reason validation exists here.
//
// A value that is well-formed GVariant and outside a declared range is accepted
// by GSettings, committed to dconf, and then read by the shell, which does
// something nobody asked for. Nothing complains at any of those steps.
func TestRangeIsEnforcedBeforeTheWrite(t *testing.T) {
	catalogue := testCatalogue(t)
	ref := Ref{Schema: "org.zenith.test", Key: "count"}

	if err := catalogue.ValidateValue(ref, "5"); err != nil {
		t.Errorf("a value inside the range was rejected: %v", err)
	}
	if err := catalogue.ValidateValue(ref, "0"); err == nil {
		t.Error("a value below the range was accepted")
	}
	if err := catalogue.ValidateValue(ref, "11"); err == nil {
		t.Error("a value above the range was accepted")
	}
	if err := catalogue.ValidateValue(ref, "five"); err == nil {
		t.Error("a non-integer was accepted for an integer key")
	}
}

// TestEnumAcceptsOnlyItsNicks covers the other type with a domain.
func TestEnumAcceptsOnlyItsNicks(t *testing.T) {
	catalogue := testCatalogue(t)
	ref := Ref{Schema: "org.zenith.test", Key: "edge"}

	if err := catalogue.ValidateValue(ref, "left"); err != nil {
		t.Errorf("a valid nick was rejected: %v", err)
	}
	if err := catalogue.ValidateValue(ref, "diagonal"); err == nil {
		t.Error("a value outside the enum was accepted")
	}

	// A message that lists the options is the difference between a user who
	// fixes it and a user who goes looking it up.
	err := catalogue.ValidateValue(ref, "diagonal")
	if err == nil || !strings.Contains(err.Error(), "top, left") {
		t.Errorf("error = %v; it should list the permitted values", err)
	}
}

// TestStringListMustLookLikeAList covers `as`.
//
// Accepted by GSettings as anything that parses as a GVariant tuple, so a bare
// word is a different setting than a list and means nothing to the shell.
func TestStringListMustLookLikeAList(t *testing.T) {
	catalogue := testCatalogue(t)
	ref := Ref{Schema: "org.zenith.test", Key: "items"}

	if err := catalogue.ValidateValue(ref, "[]"); err != nil {
		t.Errorf("an empty list was rejected: %v", err)
	}
	if err := catalogue.ValidateValue(ref, "['a', 'b']"); err != nil {
		t.Errorf("a list was rejected: %v", err)
	}
	if err := catalogue.ValidateValue(ref, "a"); err == nil {
		t.Error("a bare word was accepted for a string-list key")
	}
}

// TestBooleanMustBeTrueOrFalse covers the last type.
func TestBooleanMustBeTrueOrFalse(t *testing.T) {
	catalogue := testCatalogue(t)
	ref := Ref{Schema: "org.zenith.test", Key: "enabled"}

	if err := catalogue.ValidateValue(ref, "true"); err != nil {
		t.Errorf("true was rejected: %v", err)
	}
	if err := catalogue.ValidateValue(ref, "1"); err == nil {
		t.Error("1 was accepted for a boolean key")
	}
}

// TestUnknownSchemaNamesTheOnesWeShip covers the error a user can act on.
//
// "no schema org.zenith.shl" is not a message anyone can do anything with.
func TestUnknownSchemaNamesTheOnesWeShip(t *testing.T) {
	catalogue := testCatalogue(t)

	_, _, err := catalogue.Lookup(Ref{Schema: "org.zenith.shl", Key: "bar-height"})
	if err == nil {
		t.Fatal("a typo'd schema was accepted")
	}
	if !strings.Contains(err.Error(), "org.zenith.test") {
		t.Errorf("error = %q; it should list the schemas Zenith ships", err)
	}
}
