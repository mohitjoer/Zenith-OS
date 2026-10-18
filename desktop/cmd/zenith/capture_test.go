package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCaptureNeverWritesAnIdentifier is the test that has to exist.
//
// A capture is committed to a public repository, so anything it writes is
// published. Two attributes leaked before this was an allowlist — the DMI serial
// fields and the device tree's serial-number property — and both leaked the same
// way: a deny-list writes everything nobody thought of.
//
// This walks every fixture through a real capture and asserts none of the
// identifying values survives.
func TestCaptureNeverWritesAnIdentifier(t *testing.T) {
	canaries := []string{
		"N0ABCDE12345",                         // DMI product_serial
		"BOARDSERIAL9",                         // DMI board_serial
		"CHASSISSERIAL7",                       // DMI chassis_serial
		"ASSET-TAG-778899",                     // DMI chassis_asset_tag
		"BOARD-ASSET-1122",                     // DMI board_asset_tag
		"4C4C4544-0051-3810-8043-B9C04F503632", // DMI product_uuid
		"SN-ARM-00012345",                      // device tree serial-number
	}

	fixtures, err := filepath.Glob(filepath.Join(
		"..", "..", "..", "system", "hardware", "probe", "testdata", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no fixtures found")
	}

	for _, fixture := range fixtures {
		info, err := os.Stat(fixture)
		if err != nil || !info.IsDir() {
			continue
		}
		// testdata/fuzz holds fuzzing corpus files, not a machine.
		if filepath.Base(fixture) == "fuzz" {
			continue
		}

		out := t.TempDir()
		captured, _, err := capture(fixture, out)
		if err != nil {
			t.Fatalf("capture(%s): %v", filepath.Base(fixture), err)
		}
		if captured == 0 {
			t.Errorf("capture(%s) wrote nothing", filepath.Base(fixture))
			continue
		}

		// The canaries are checked against the *source* tree, so a fixture that
		// does not contain a given value cannot pass vacuously.
		source := readAll(t, fixture)
		for _, canary := range canaries {
			if !strings.Contains(source, canary) {
				continue
			}
			if written := readAll(t, out); strings.Contains(written, canary) {
				t.Errorf("%s: capture wrote %q to disk; a committed fixture is "+
					"published", filepath.Base(fixture), canary)
			}
		}
	}
}

// TestCaptureIsUsableAsAFixture checks the allowlist did not filter so hard that
// the result is useless. A capture that writes nothing a reader needs is a
// capture that cannot be tested against.
func TestCaptureIsUsableAsAFixture(t *testing.T) {
	fixture := filepath.Join("..", "..", "..", "system", "hardware", "probe",
		"testdata", "hybrid-laptop")
	out := t.TempDir()

	captured, _, err := capture(fixture, out)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}

	// The attributes the reference machine's report depends on.
	for _, relative := range []string{
		"class/dmi/id/sys_vendor",
		"class/dmi/id/product_name",
		"class/dmi/id/chassis_type",
		"class/drm/card0-eDP-1/edid",
		"bus/pci/devices/0000:00:02.0/vendor",
		"bus/pci/devices/0000:00:02.0/boot_vga",
		"power/mem_sleep",
		"firmware/acpi/tables/DSDT",
		"modules.alias",
	} {
		if _, err := os.Stat(filepath.Join(out, relative)); err != nil {
			t.Errorf("capture dropped %s, which a reader needs; the allowlist "+
				"filters too hard (%d files captured)", relative, captured)
		}
	}
}

// TestCaptureKeepsTheARMIdentity checks the reach path survives a capture: a
// device tree that lost its model would make an ARM laptop unmatchable.
func TestCaptureKeepsTheARMIdentity(t *testing.T) {
	fixture := filepath.Join("..", "..", "..", "system", "hardware", "probe",
		"testdata", "arm-laptop")
	out := t.TempDir()

	if _, _, err := capture(fixture, out); err != nil {
		t.Fatalf("capture: %v", err)
	}

	model, err := os.ReadFile(filepath.Join(out, "firmware/devicetree/base/model"))
	if err != nil {
		t.Fatalf("capture dropped the device-tree model: %v", err)
	}
	if !strings.Contains(string(model), "Snapdragon") {
		t.Errorf("the captured model is %q", model)
	}

	if _, err := os.Stat(filepath.Join(out,
		"firmware/devicetree/base/serial-number")); err == nil {
		t.Error("capture wrote the device-tree serial-number")
	}
}

// TestGlobMatch checks the matcher, because a permissive glob is a leak and a
// strict one is a broken fixture.
func TestGlobMatch(t *testing.T) {
	tests := []struct {
		pattern, value string
		want           bool
	}{
		{"power/state", "power/state", true},
		{"power/state", "power/mem_sleep", false},
		{"bus/pci/devices/*/vendor", "bus/pci/devices/0000:00:02.0/vendor", true},
		{"bus/pci/devices/*/vendor", "bus/pci/devices/0000:00:02.0/config", false},
		// A * must never span a separator.
		{"bus/pci/devices/*/vendor", "bus/pci/devices/a/b/vendor", false},
		{"class/dmi/id/{a,b}", "class/dmi/id/a", true},
		{"class/dmi/id/{a,b}", "class/dmi/id/c", false},
		{"firmware/acpi/tables/{DSDT,FACP}", "firmware/acpi/tables/FACP", true},
		{"firmware/acpi/tables/{DSDT,FACP}", "firmware/acpi/tables/DSDT.orig", false},
	}
	for _, tc := range tests {
		if got := globMatch(tc.pattern, tc.value); got != tc.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v",
				tc.pattern, tc.value, got, tc.want)
		}
	}
}

// TestSerialNumberIsNeverCapturable is the direct assertion for the leak that
// was found: the device tree's serial-number property.
func TestSerialNumberIsNeverCapturable(t *testing.T) {
	if captureAllowed("firmware/devicetree/base/serial-number") {
		t.Error("the device-tree serial-number is capturable")
	}
	if !captureAllowed("firmware/devicetree/base/model") {
		t.Error("the device-tree model is not capturable, so an ARM machine " +
			"would be unmatchable from its own capture")
	}
}

// readAll concatenates every regular file under a tree.
func readAll(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			b.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return b.String()
}
