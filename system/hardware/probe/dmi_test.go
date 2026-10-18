package probe

import (
	"os"
	"path/filepath"
	"testing"
)

// fixture opens a captured machine.
//
// Every test in this file reads a directory under testdata rather than the live
// machine. That is the payoff of the root-per-probe rule: detection is tested
// on any machine, on any laptop, for free, and a contributor's laptop becomes
// an end-to-end test forever by contributing one directory.
func fixture(t *testing.T, name string) *FS {
	t.Helper()
	root := filepath.Join("testdata", name)
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("fixture %q missing: %v", name, err)
	}
	fs, err := New(root)
	if err != nil {
		t.Fatalf("opening fixture %q: %v", name, err)
	}
	return fs
}

// TestDMICapturesReferenceMachine pins the field-by-field parse of a real
// machine's DMI directory.
func TestDMICapturesReferenceMachine(t *testing.T) {
	fields, err := DMI(fixture(t, "hybrid-laptop"))
	if err != nil {
		t.Fatalf("DMI: %v", err)
	}

	if !fields.Present {
		t.Error("Present is false on a machine with a full DMI table")
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"sys_vendor", fields.SysVendor, "ASUSTeK COMPUTER INC."},
		{"product_name", fields.ProductName, "ASUS TUF Gaming F15 FX507ZC4_FX507ZC4"},
		{"product_version", fields.ProductVersion, "1.0"},
		{"product_family", fields.ProductFamily, "ASUS TUF Gaming F15"},
		{"product_sku", fields.ProductSKU, "FX507ZC4"},
		{"board_vendor", fields.BoardVendor, "ASUSTeK COMPUTER INC."},
		{"board_name", fields.BoardName, "FX507ZC4"},
		{"board_version", fields.BoardVersion, "1.0"},
		{"bios_vendor", fields.BIOSVendor, "American Megatrends Inc."},
		{"bios_version", fields.BIOSVersion, "3303"},
		{"bios_date", fields.BIOSDate, "12/03/2024"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}

	// chassis_type is a raw SMBIOS integer in the directory and an ACPI
	// chassis string in the struct. 10 is "notebook".
	if fields.ChassisType != "notebook" {
		t.Errorf("chassis_type = %q, want %q", fields.ChassisType, "notebook")
	}
}

// TestDMIEmitsNoSerial is the reader half of the redaction contract.
//
// Layer one is that the reader copies named fields into a struct and never
// copies a directory, so a field nobody classified has no path to a report. The
// fixture carries a product serial, a board serial, a chassis serial and two
// asset tags precisely so this can be asserted rather than assumed — and so the
// redaction canary test has real canaries to hunt.
func TestDMIEmitsNoSerial(t *testing.T) {
	fields, err := DMI(fixture(t, "hybrid-laptop"))
	if err != nil {
		t.Fatalf("DMI: %v", err)
	}

	// The only place a UUID-derived value may appear is the reader-side hash,
	// which is not a token and never reaches a report.
	carriers := map[string]string{
		"sys_vendor":     fields.SysVendor,
		"product_name":   fields.ProductName,
		"product_family": fields.ProductFamily,
		"board_name":     fields.BoardName,
		"board_version":  fields.BoardVersion,
		"bios_version":   fields.BIOSVersion,
		"chassis_type":   fields.ChassisType,
	}

	canaries := []string{
		"N0ABCDE12345",                         // product_serial
		"BOARDSERIAL9",                         // board_serial
		"CHASSISSERIAL7",                       // chassis_serial
		"ASSET-TAG-778899",                     // chassis_asset_tag, mode 0444
		"BOARD-ASSET-1122",                     // board_asset_tag
		"4C4C4544-0051-3810-8043-B9C04F503632", // product_uuid
	}

	for field, value := range carriers {
		for _, canary := range canaries {
			if contains(value, canary) {
				t.Errorf("field %s leaked %q: %q", field, canary, value)
			}
		}
	}

	if fields.ProductUUIDHash == "" {
		t.Error("ProductUUIDHash is empty; the reader must hash product_uuid " +
			"rather than dropping or carrying it")
	}
	if len(fields.ProductUUIDHash) == len("4C4C4544-0051-3810-8043-B9C04F503632") {
		t.Error("ProductUUIDHash appears to be the raw UUID")
	}
}

// TestDMIOnARMBoard checks that a machine with no SMBIOS is a result, not a
// failure. This is the negative fixture that nobody can capture from real
// hardware by accident, because it is the absence of hardware.
func TestDMIOnARMBoard(t *testing.T) {
	fields, err := DMI(fixture(t, "arm-board"))
	if err != nil {
		t.Fatalf("DMI on a machine with no DMI returned an error: %v", err)
	}
	if fields.Present {
		t.Error("Present is true on an ARM board with no SMBIOS table")
	}
	if fields.SysVendor != "" || fields.ProductName != "" {
		t.Errorf("ARM board reported identity strings: %+v", fields)
	}
}

// TestDMIOnVMWithOEMFirmware checks the case the spec calls out: firmware that
// says nothing usable. These strings are present and non-empty, so a naive
// presence check would treat them as a real machine.
func TestDMIOnVMWithOEMFirmware(t *testing.T) {
	fields, err := DMI(fixture(t, "vm-oem"))
	if err != nil {
		t.Fatalf("DMI: %v", err)
	}
	if !fields.Present {
		t.Error("Present is false; the directory exists and reports strings")
	}
	if fields.SysVendor != "To Be Filled By O.E.M." {
		t.Errorf("sys_vendor = %q; the fixture should carry the OEM placeholder "+
			"verbatim so matching can be tested against it", fields.SysVendor)
	}
}

// TestParseBIOSDate covers the MM/DD/YYYY trap. A machine reporting 12/03/2024
// means 3 December. Parsing it as 12 March would silently accept or reject a
// profile's date comparator on some machines and not others.
func TestParseBIOSDate(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"12/03/2024", "2024-12-03", false},
		{"01/02/2024", "2024-01-02", false}, // 1 February, not 12 January
		{"31/12/2023", "", true},            // DD/MM: 31 is not a month
		{"12/31/2023", "2023-12-31", false}, // 31 December, unambiguously
		{"13/01/2024", "", true},            // 13 is not a month
		{"00/10/2024", "", true},            // day zero
		{"2024-12-03", "", true},            // ISO is not the SMBIOS form
		{"12/3/2024", "", true},             // not zero-padded
		{"12/03/24", "", true},              // two-digit year
		{"", "", true},
		{"garbage", "", true},
	}

	for _, tc := range tests {
		got, err := ParseBIOSDate(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseBIOSDate(%q) = %v, want an error; a partially "+
					"parsed date would let a profile match on a firmware date "+
					"nobody reported", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseBIOSDate(%q): %v", tc.in, err)
			continue
		}
		if got.Format("2006-01-02") != tc.want {
			t.Errorf("ParseBIOSDate(%q) = %s, want %s", tc.in,
				got.Format("2006-01-02"), tc.want)
		}
	}
}

// TestChassisTypeNameUnknown checks that an unmapped integer is not guessed.
func TestChassisTypeNameUnknown(t *testing.T) {
	if _, ok := ChassisTypeName(99); ok {
		t.Error("ChassisTypeName(99) reported a mapping; a firmware that reports " +
			"an undefined chassis type has not told us anything usable")
	}
	if name, ok := ChassisTypeName(35); !ok || name != "mini PC" {
		t.Errorf("ChassisTypeName(35) = %q, %v; want \"mini PC\", true", name, ok)
	}
}

func contains(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	return len(haystack) >= len(needle) &&
		func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}()
}
