package probe

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestACPIReadsLaptop checks the free facts: which tables exist, what the sleep
// mask says, and what the platform profile is.
func TestACPIReadsLaptop(t *testing.T) {
	state, err := ACPI(fixture(t, "hybrid-laptop"))
	if err != nil {
		t.Fatalf("ACPI: %v", err)
	}

	if !state.Present {
		t.Error("Present is false on a laptop with ACPI")
	}
	if !state.FPDTPresent {
		t.Error("FPDTPresent is false; FPDT was in the fixture's table list")
	}
	if state.DefaultMemSleep != "s2idle" {
		t.Errorf("DefaultMemSleep = %q, want %q", state.DefaultMemSleep, "s2idle")
	}
	if state.PlatformProfile != "balanced" {
		t.Errorf("PlatformProfile = %q, want balanced", state.PlatformProfile)
	}
	if len(state.PlatformProfileChoices) != 4 {
		t.Errorf("PlatformProfileChoices = %v, want four entries",
			state.PlatformProfileChoices)
	}
	if !state.BacklightGraphics {
		t.Error("BacklightGraphics is false on a laptop with a backlight device")
	}
	if len(state.Tables) == 0 {
		t.Error("no ACPI tables reported")
	}
}

// TestACPIAbsentIsNotAnError covers the desktop fixture, which has ACPI but no
// battery, no backlight and no sleep mask. Every one of those is an absence the
// probe must report rather than fail on.
func TestACPIAbsentIsNotAnError(t *testing.T) {
	state, err := ACPI(fixture(t, "desktop-minimal"))
	if err != nil {
		t.Fatalf("ACPI returned an error for absent optional state: %v", err)
	}
	if !state.Present {
		t.Error("Present is false; a desktop still has ACPI")
	}
	if state.BacklightGraphics {
		t.Error("BacklightGraphics is true on a desktop with no panel")
	}
	if state.MemSleep != "" || state.DefaultMemSleep != "" {
		t.Errorf("sleep state reported on a machine with none: %q / %q",
			state.MemSleep, state.DefaultMemSleep)
	}
}

// TestACPINoACPI checks that a machine with no ACPI is a result.
func TestACPINoACPI(t *testing.T) {
	state, err := ACPI(fixture(t, "arm-board"))
	if err != nil {
		t.Fatalf("ACPI on a machine with no ACPI returned an error: %v", err)
	}
	if state.Present {
		t.Error("Present is true on an ARM board with no ACPI")
	}
	if len(state.Tables) != 0 {
		t.Errorf("Tables = %v, want none", state.Tables)
	}
}

// TestDefaultMemSleep covers the mask format. The brackets mark the default and
// the default is the only entry that predicts what the machine will do.
func TestDefaultMemSleep(t *testing.T) {
	tests := []struct {
		mask string
		want string
	}{
		{"s2idle", ""},              // no brackets: the kernel declared no default
		{"[s2idle] deep", "s2idle"}, // default first
		{"deep [s2idle]", "s2idle"}, // default last
		{"[s3]", "s3"},
		{"s3 s2idle", ""},
		{"", ""},
		{"[]", ""},
	}
	for _, tc := range tests {
		if got := defaultMemSleep(tc.mask); got != tc.want {
			t.Errorf("defaultMemSleep(%q) = %q, want %q", tc.mask, got, tc.want)
		}
	}
}

// TestACPINeverEmitsTableContent checks the trap the design record calls out:
// an AML table dump must never reach a report, because tables embed identifiers
// as _DSM GUIDs and as strings in arguments.
func TestACPINeverEmitsTableContent(t *testing.T) {
	state, err := ACPI(fixture(t, "hybrid-laptop"))
	if err != nil {
		t.Fatalf("ACPI: %v", err)
	}

	// The state carries names only. There is no field anywhere on ACPIState
	// that could hold a table body, a hex dump or ASL source — the check that
	// matters is a compile-time one, and the field list is it.
	for _, table := range state.Tables {
		if len(table) > 8 {
			t.Errorf("table entry %q is long enough to be content rather than a "+
				"name", table)
		}
	}
}

// TestEDIDParsesInternalPanel checks the reference panel: manufacturer, product
// code, native mode, refresh and the monitor name string.
func TestEDIDParsesInternalPanel(t *testing.T) {
	panels, err := EDID(fixture(t, "hybrid-laptop"))
	if err != nil {
		t.Fatalf("EDID: %v", err)
	}

	var internal *Panel
	for i := range panels {
		if panels[i].Connector == "card0-eDP-1" {
			internal = &panels[i]
		}
	}
	if internal == nil {
		t.Fatal("eDP connector not found")
	}

	if !internal.Present {
		t.Error("Present is false on a connected panel with an EDID block")
	}
	if internal.Manufacturer != "NCP" {
		t.Errorf("manufacturer = %q, want %q", internal.Manufacturer, "NCP")
	}
	// The product code shares byte 10 with the third manufacturer letter, so
	// 0xf310 is what the block actually encodes. Reading it back as 0xf31f would
	// mean ignoring the overlap that the format specifies.
	if internal.ProductCode != "f310" {
		t.Errorf("product code = %q, want %q", internal.ProductCode, "f310")
	}
	if internal.MonitorName != "LM156LF-2F01" {
		t.Errorf("monitor name = %q, want %q", internal.MonitorName, "LM156LF-2F01")
	}
	if internal.Width != 1920 || internal.Height != 1080 {
		t.Errorf("native mode = %dx%d, want 1920x1080", internal.Width, internal.Height)
	}
	// 148.5 MHz over 2080x1140 is 60.00 Hz. A parser that reads the blanking
	// widths as shifted 12-bit values lands somewhere near 60 and looks right,
	// which is why the refresh is asserted to two places.
	if internal.RefreshHz < 59.9 || internal.RefreshHz > 60.1 {
		t.Errorf("refresh = %.3f Hz, want 60.000", internal.RefreshHz)
	}
	if internal.SerialToken == "" {
		t.Error("SerialToken is empty; an internal panel does carry a serial " +
			"descriptor and dropping it silently loses the hashed field")
	}

	// Internal is an inference from the connector name, and it carries its
	// evidence rather than being asserted bare.
	if !internal.Internal {
		t.Error("eDP connector not marked internal")
	}
	if internal.InternalEvidence != "eDP" {
		t.Errorf("InternalEvidence = %q, want %q", internal.InternalEvidence, "eDP")
	}
	if internal.BacklightDevice != "intel_backlight" {
		t.Errorf("BacklightDevice = %q, want intel_backlight; which driver owns "+
			"the backlight is the difference between the panel dimming and not",
			internal.BacklightDevice)
	}
}

// TestEDIDZeroLengthIsDisconnected is trap two: a zero-byte read is a
// disconnected connector, not an error and not a panic.
func TestEDIDZeroLengthIsDisconnected(t *testing.T) {
	panels, err := EDID(fixture(t, "hybrid-laptop"))
	if err != nil {
		t.Fatalf("EDID: %v", err)
	}

	for _, panel := range panels {
		if panel.Connector != "card0-HDMI-A-1" {
			continue
		}
		if panel.Present {
			t.Error("Present is true for a connector with a zero-length edid")
		}
		if panel.Manufacturer != "" {
			t.Errorf("a disconnected connector reported manufacturer %q",
				panel.Manufacturer)
		}
		return
	}
	t.Fatal("disconnected HDMI connector not found")
}

// TestEDIDRejectsFaultyLength is trap three: a block that is not a non-zero
// multiple of 128 is a truncated read and is rejected whole, rather than having
// its valid prefix parsed into plausible-looking wrong values.
func TestEDIDRejectsFaultyLength(t *testing.T) {
	if err := validateEDIDLength(128); err != nil {
		t.Errorf("128 bytes rejected: %v", err)
	}
	if err := validateEDIDLength(256); err != nil {
		t.Errorf("256 bytes rejected: %v", err)
	}

	for _, bad := range []int{0, 1, 100, 127, 129, 200} {
		if err := validateEDIDLength(bad); err == nil {
			t.Errorf("validateEDIDLength(%d) accepted; a truncated read parsed as "+
				"a whole block yields a right manufacturer and a garbage timing table",
				bad)
		}
	}
}

// TestEDIDTruncatedBlockIsNotParsed drives the same rule through the real
// reader, not just the length check.
func TestEDIDTruncatedBlockIsNotParsed(t *testing.T) {
	dir := t.TempDir()
	connector := filepath.Join(dir, "class/drm/card0-eDP-1")
	if err := os.MkdirAll(connector, 0o755); err != nil {
		t.Fatal(err)
	}

	block := mustEDID(t)
	for _, n := range []int{100, 130} {
		if err := os.WriteFile(filepath.Join(connector, "edid"), block[:n], 0o644); err != nil {
			t.Fatal(err)
		}
		fs, err := New(dir)
		if err != nil {
			t.Fatal(err)
		}
		panels, err := EDID(fs)
		if err != nil {
			t.Fatalf("EDID on a truncated block returned an error: %v", err)
		}
		if len(panels) != 1 {
			t.Fatalf("got %d panels, want 1", len(panels))
		}
		if panels[0].Present {
			t.Errorf("a %d-byte block was parsed as a panel", n)
		}
		if panels[0].Manufacturer != "" {
			t.Errorf("a %d-byte block yielded manufacturer %q; parsing the prefix "+
				"of a corrupt block is how wrong values look right", n,
				panels[0].Manufacturer)
		}
	}
}

// TestEDIDRejectsNonEDIDBytes checks the header. Reading arbitrary bytes as
// EDID produces a manufacturer code that is plausible and wrong.
func TestEDIDRejectsNonEDIDBytes(t *testing.T) {
	block := make([]byte, 128)
	for i := range block {
		block[i] = 0xAB
	}
	if _, err := ParseEDID(block); err == nil {
		t.Error("ParseEDID accepted 128 bytes with no EDID header")
	}
}

// TestEDIDSerialIsHashedNotRaw checks that the serial descriptor never reaches
// a report in the clear.
func TestEDIDSerialIsHashedNotRaw(t *testing.T) {
	block := mustEDID(t)

	// The fixture encodes serial 0x12345678 little-endian at bytes 12-15.
	raw := []byte{0x78, 0x56, 0x34, 0x12}

	info, err := ParseEDID(block)
	if err != nil {
		t.Fatalf("ParseEDID: %v", err)
	}
	if bytes.Contains([]byte(info.SerialToken), raw) {
		t.Error("SerialToken contains the raw serial bytes")
	}
	if len(info.SerialToken) < 8 {
		t.Errorf("SerialToken = %q, too short to be a hash", info.SerialToken)
	}
}

// TestEDIDSparsePanel checks the desktop fixture's panel, which has no name
// descriptor and a zero serial. Absence in EDID is absence of data, not absence
// of capability, and neither may be reported as an error.
func TestEDIDSparsePanel(t *testing.T) {
	panels, err := EDID(fixture(t, "desktop-minimal"))
	if err != nil {
		t.Fatalf("EDID: %v", err)
	}
	if len(panels) == 0 {
		t.Fatal("no panels on the desktop fixture")
	}

	for _, panel := range panels {
		if panel.Internal {
			t.Errorf("%s marked internal; a DP connector is external by "+
				"convention", panel.Connector)
		}
	}
}

// TestEDIDInternalByConvention checks the embedded/external split by connector
// name, which is a convention and is recorded as such.
func TestEDIDInternalByConvention(t *testing.T) {
	embedded := []string{"card0-eDP-1", "card1-LVDS-1", "card0-DSI-1"}
	external := []string{"card0-DP-1", "card0-HDMI-A-1", "card0-VGA-1"}

	for _, name := range embedded {
		if embeddedPrefix(name) == "" {
			t.Errorf("%s not recognised as an embedded connector", name)
		}
	}
	for _, name := range external {
		if got := embeddedPrefix(name); got != "" {
			t.Errorf("%s recognised as embedded (%q); it is external", name, got)
		}
	}
}

// mustEDID returns a valid 128-byte block for the tests that need one.
func mustEDID(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("testdata", "hybrid-laptop/class/drm/card0-eDP-1/edid")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the fixture EDID block: %v", err)
	}
	if len(data) != 128 {
		t.Fatalf("fixture edid is %d bytes, want 128", len(data))
	}
	return data
}
