package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zenith/hardware/probe"
	"zenith/hardware/redact"
)

// machineFromFixture runs the real probes against a captured machine, so these
// tests exercise the same path `zenith hardware report` does.
func machineFromFixture(t *testing.T, name string) Input {
	t.Helper()

	root := filepath.Join("..", "probe", "testdata", name)
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("fixture %q missing: %v", name, err)
	}

	fs, err := probe.New(root)
	if err != nil {
		t.Fatalf("probe.New: %v", err)
	}

	dmi, err := probe.DMI(fs)
	if err != nil {
		t.Fatalf("DMI: %v", err)
	}
	rules, err := probe.AliasRules(filepath.Join(fs.Root(), "modules.alias"))
	if err != nil {
		t.Fatalf("AliasRules: %v", err)
	}
	pci, err := probe.PCI(fs, rules)
	if err != nil {
		t.Fatalf("PCI: %v", err)
	}
	usb, err := probe.USB(fs)
	if err != nil {
		t.Fatalf("USB: %v", err)
	}
	acpi, err := probe.ACPI(fs)
	if err != nil {
		t.Fatalf("ACPI: %v", err)
	}
	edid, err := probe.EDID(fs)
	if err != nil {
		t.Fatalf("EDID: %v", err)
	}

	// The raw DMI dump, as it stands on disk, for the sweep.
	raw := map[string]string{}
	entries, err := os.ReadDir(filepath.Join(root, "class", "dmi", "id"))
	if err == nil {
		for _, entry := range entries {
			data, err := os.ReadFile(filepath.Join(root, "class", "dmi", "id", entry.Name()))
			if err != nil {
				continue
			}
			raw[entry.Name()] = strings.TrimRight(string(data), "\n")
		}
	}

	allowed := make([]string, 0, len(pci)*2+len(usb)*2)
	for _, device := range pci {
		allowed = append(allowed, device.Vendor+":"+device.Device)
	}
	for _, device := range usb {
		allowed = append(allowed, device.IDVendor+":"+device.IDProduct)
	}

	var salt redact.Salt
	for i := range salt {
		salt[i] = byte(i)
	}

	return Input{
		DMI:           dmi,
		PCI:           pci,
		USB:           usb,
		ACPI:          acpi,
		EDID:          edid,
		KernelFlavour: "hwe",
		RawDMI:        raw,
		Salt:          salt,
		Host: redact.Context{
			RawDMI:             raw,
			AllowedIdentifiers: allowed,
			// A machine that has these set in its environment. The report must
			// not carry them.
			Hostname:   "mohit-thinkpad",
			User:       "mohit",
			Logname:    "mohit",
			MachineIDs: []string{"4d7e2a1b9c8f0e3d5a6b7c8d9e0f1a2b"},
		},
	}
}

// canaries are the identifying values present in the hybrid-laptop fixture. No
// rendered shape may contain any of them.
var canaries = []string{
	"N0ABCDE12345",                         // product_serial
	"BOARDSERIAL9",                         // board_serial
	"CHASSISSERIAL7",                       // chassis_serial
	"ASSET-TAG-778899",                     // chassis_asset_tag, mode 0444
	"BOARD-ASSET-1122",                     // board_asset_tag
	"4C4C4544-0051-3810-8043-B9C04F503632", // product_uuid
}

// TestReportCarriesNoCanary is the test that matters most in this package. The
// output of this tool is attached to public pull requests.
func TestReportCarriesNoCanary(t *testing.T) {
	input := machineFromFixture(t, "hybrid-laptop")

	for _, format := range []Format{FormatText, FormatJSON, FormatProfile} {
		for _, deep := range []bool{false, true} {
			rendered, err := Render(input, Options{Format: format, Deep: deep})
			if err != nil {
				t.Fatalf("%s (deep=%t): %v", format, deep, err)
			}
			for _, canary := range canaries {
				if strings.Contains(string(rendered), canary) {
					t.Errorf("%s (deep=%t) leaked %q", format, deep, canary)
				}
			}
			for _, host := range []string{"mohit-thinkpad", "mohit",
				"4d7e2a1b9c8f0e3d5a6b7c8d9e0f1a2b"} {
				if strings.Contains(string(rendered), host) {
					t.Errorf("%s (deep=%t) leaked the host value %q", format, deep, host)
				}
			}
		}
	}
}

// TestProfileCarriesNoTokensAtAll checks the stronger rule for the profile
// shape: a profile is a committed artefact, and a token in it is a correlatable
// value with no reason to be there.
func TestProfileCarriesNoTokensAtAll(t *testing.T) {
	input := machineFromFixture(t, "hybrid-laptop")

	rendered, err := Render(input, Options{Format: FormatProfile})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if strings.Contains(string(rendered), "sha256:") {
		t.Errorf("the candidate profile carries a token:\n%s", rendered)
	}
	if strings.Contains(string(rendered), "token") {
		t.Errorf("the candidate profile mentions a token:\n%s", rendered)
	}
}

// TestProfileHasNoKernelPatchVersion checks the prohibition that keeps a profile
// matching after the next security update.
func TestProfileHasNoKernelPatchVersion(t *testing.T) {
	for _, flavour := range []string{"generic", "hwe", "oem"} {
		input := machineFromFixture(t, "hybrid-laptop")
		input.KernelFlavour = flavour

		rendered, err := Render(input, Options{Format: FormatProfile})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}

		text := string(rendered)
		if !strings.Contains(text, "kernel: "+flavour+"\n") {
			t.Errorf("profile does not carry kernel: %s\n%s", flavour, text)
		}
		// A patch version looks like a version with a dash. If one ever appears
		// on the kernel line, the profile stops matching at the next update.
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "kernel:") && strings.Count(line, "-") > 1 {
				t.Errorf("kernel line looks like a pin: %q", line)
			}
		}
	}
}

// TestProfileNamesNoFirmwareFile checks the other prohibition: firmware is named
// as packages, never as a blob or a path.
func TestProfileNamesNoFirmwareFile(t *testing.T) {
	input := machineFromFixture(t, "hybrid-laptop")

	rendered, err := Render(input, Options{Format: FormatProfile})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	text := string(rendered)
	for _, forbidden := range []string{".ko", ".fw", "/lib/firmware", ".hex", ".bin"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("profile names something that looks like a firmware file (%q):\n%s",
				forbidden, text)
		}
	}
}

// TestProfileQuotesVersionFields is the regression the whole YAML emitter
// exists for.
func TestProfileQuotesVersionFields(t *testing.T) {
	input := machineFromFixture(t, "hybrid-laptop")

	rendered, err := Render(input, Options{Format: FormatProfile})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, line := range []string{
		`product_version: "1.0"`,
		`board_version: "1.0"`,
	} {
		if !strings.Contains(string(rendered), line) {
			t.Errorf("profile does not contain %s; an unquoted 1.0 parses as a "+
				"float and stops matching the string the machine reports.\n%s",
				line, rendered)
		}
	}
}

// TestProfileIsDeterministic is what makes a golden-file diff a review artefact
// rather than noise.
func TestProfileIsDeterministic(t *testing.T) {
	input := machineFromFixture(t, "hybrid-laptop")

	first, err := Render(input, Options{Format: FormatProfile})
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 20; run++ {
		got, err := Render(input, Options{Format: FormatProfile})
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d differs from run 0", run)
		}
	}
}

// TestProfileOnARMLaptop checks the reach path end to end: a machine with no
// DMI still produces a usable profile.
func TestProfileOnARMLaptop(t *testing.T) {
	root := filepath.Join("..", "probe", "testdata", "arm-laptop")
	fs, err := probe.New(root)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := probe.DeviceTree(fs)
	if err != nil {
		t.Fatal(err)
	}

	// A device tree projected onto the match-key fields, as the probe layer
	// would hand it to the renderer.
	dmiFields := probe.Fields{
		Present:     true,
		SysVendor:   tree.IdentityFields()["sys_vendor"],
		ProductName: tree.IdentityFields()["product_name"],
		ChassisType: "notebook",
	}

	input := Input{DMI: dmiFields, KernelFlavour: "hwe"}

	rendered, err := Render(input, Options{Format: FormatProfile})
	if err != nil {
		t.Fatalf("Render on an ARM laptop: %v", err)
	}

	text := string(rendered)
	if !strings.Contains(text, "Snapdragon X Elite Laptop") {
		t.Errorf("the ARM profile does not carry the model:\n%s", text)
	}
	if !strings.Contains(text, `sys_vendor: qcom,x1e-generic`) {
		t.Errorf("the ARM profile does not carry the vendor:\n%s", text)
	}
}

// TestReportRefusesToEmitALeak is the contract: a renderer that would produce
// something containing an identifier must return an error rather than bytes.
func TestReportRefusesToEmitALeak(t *testing.T) {
	input := machineFromFixture(t, "hybrid-laptop")

	// Plant a serial in a kept field. The sweep compares the raw dump against
	// the rendered bytes, so a value that is present in both must appear in
	// neither — which means the render must fail.
	input.Host.RawDMI["bios_version"] = "N0ABCDE12345"
	input.DMI.BIOSVersion = "N0ABCDE12345"

	if _, err := Render(input, Options{Format: FormatText}); err == nil {
		t.Fatal("a report carrying a serial was emitted; the sweep is not " +
			"running or the canary is not reaching it")
	}
}

// TestRawRefusesUnderCI checks that a pipeline cannot produce a file intended
// for humans to keep local.
func TestRawRefusesUnderCI(t *testing.T) {
	for _, env := range []string{"true", "1", "yes", "anything at all"} {
		if !RawPathRefusesUnderCI(env) {
			t.Errorf("RawPathRefusesUnderCI(%q) = false", env)
		}
	}
	if RawPathRefusesUnderCI("") {
		t.Error("RawPathRefusesUnderCI(\"\") = true; an empty environment is a " +
			"terminal, not a pipeline")
	}
	if !strings.Contains(RawPathWarning, "Do not attach") {
		t.Error("the raw warning does not say the file must not be attached")
	}
}

// TestJSONIsValidAndCarriesNoSerial checks the machine-readable shape.
func TestJSONIsValidAndCarriesNoSerial(t *testing.T) {
	input := machineFromFixture(t, "hybrid-laptop")

	rendered, err := Render(input, Options{Format: FormatJSON})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(rendered, &doc); err != nil {
		t.Fatalf("the JSON shape is not valid JSON: %v", err)
	}

	notes, ok := doc["notes"].(map[string]any)
	if !ok {
		t.Fatal("the JSON shape has no notes block")
	}
	// The report must state its own properties rather than leave a reader to
	// infer them: tokens are tokens, they rotate, they do not identify, and
	// detection says nothing about whether the machine works.
	if notes["serials_are_tokens"] != true {
		t.Error("notes does not say serials are tokens")
	}
	if notes["tokens_identify_the_machine"] != false {
		t.Error("notes does not say tokens do not identify the machine")
	}
	if notes["this_does_not_say_the_machine_works"] != false {
		t.Error("notes conflates detection with a working machine")
	}
}

// TestDeepOnlyAddsTableNames checks that --deep adds facts and never content. An
// AML table embeds identifiers as _DSM GUIDs and as strings in arguments.
func TestDeepOnlyAddsTableNames(t *testing.T) {
	input := machineFromFixture(t, "hybrid-laptop")

	shallow, err := Render(input, Options{Format: FormatText})
	if err != nil {
		t.Fatal(err)
	}
	deep, err := Render(input, Options{Format: FormatText, Deep: true})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(shallow), "ACPI tables:") {
		t.Error("the shallow report lists ACPI tables")
	}
	if !strings.Contains(string(deep), "ACPI tables:") {
		t.Error("the deep report does not list ACPI tables")
	}
	// Names only: an entry that looks like a hex dump or a path is content.
	for _, line := range strings.Split(string(deep), "\n") {
		if strings.HasPrefix(line, "ACPI tables:") &&
			strings.Contains(line, "0x") {
			t.Errorf("the deep report's table line looks like content: %q", line)
		}
	}
}

// TestUnknownFormatIsAnError checks the renderer refuses rather than defaulting.
func TestUnknownFormatIsAnError(t *testing.T) {
	if _, err := Render(Input{}, Options{Format: "toml"}); err == nil {
		t.Error("an unknown format was accepted")
	}
}

// TestVMWithPlaceholderFirmwareDoesNotGetAProfileNameFromIt checks that a
// machine whose firmware says nothing usable still gets an honest id.
func TestVMWithPlaceholderFirmwareDoesNotGetAProfileNameFromIt(t *testing.T) {
	input := machineFromFixture(t, "vm-oem")

	rendered, err := Render(input, Options{Format: FormatProfile})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	text := string(rendered)
	if strings.Contains(text, "id: to-be-filled") {
		t.Errorf("the profile id is derived from firmware filler:\n%s", text)
	}
	if !strings.Contains(text, "unknown-vendor") {
		t.Errorf("the profile id does not say the vendor is unknown:\n%s", text)
	}
}

// TestProfileDoesNotInventAKernelFlavour covers the case the CLI hit first.
//
// osrelease lives in /proc, which a captured machine does not carry, so
// detection from a fixture cannot know the kernel. Defaulting to "generic" there
// is a silent claim: a reader cannot tell it apart from a machine that genuinely
// reports generic, and the wrong flavour installs a kernel that does not drive
// the hardware.
func TestProfileDoesNotInventAKernelFlavour(t *testing.T) {
	input := machineFromFixture(t, "hybrid-laptop")
	input.KernelFlavour = ""

	rendered, err := Render(input, Options{Format: FormatProfile})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	text := string(rendered)
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "kernel:") {
			t.Errorf("profile names a kernel flavour detection could not "+
				"determine: %q", line)
		}
	}
	if !strings.Contains(text, "could not determine") {
		t.Errorf("profile does not say the flavour is undetermined:\n%s", text)
	}
}

// TestProfileNeverNamesAPatchVersion is the belt to the above braces: whatever
// the caller passes, a patch version must not reach a profile.
func TestProfileNeverNamesAPatchVersion(t *testing.T) {
	for _, flavour := range []string{
		"6.8.0-31-generic", "6.8.0-31", "generic-6.8", "hwe-6.8.0-31",
	} {
		input := machineFromFixture(t, "hybrid-laptop")
		input.KernelFlavour = flavour

		rendered, err := Render(input, Options{Format: FormatProfile})
		if err != nil {
			t.Fatalf("Render(%q): %v", flavour, err)
		}
		for _, line := range strings.Split(string(rendered), "\n") {
			if strings.HasPrefix(line, "kernel:") &&
				strings.Count(line, "-") > 1 {
				t.Errorf("flavour %q produced a pin: %q", flavour, line)
			}
		}
	}
}
