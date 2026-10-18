package probe

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPCIClassifiesHybridLaptop covers the classification and the two values
// that matter most on a hybrid machine: boot_vga, which says which GPU the
// panel hangs off, and the candidate module list for a device with no driver.
func TestPCIClassifiesHybridLaptop(t *testing.T) {
	fs := fixture(t, "hybrid-laptop")

	rules, err := AliasRules(filepath.Join(fs.Root(), "modules.alias"))
	if err != nil {
		t.Fatalf("AliasRules: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("fixture has no modules.alias rules; the candidate module test " +
			"would pass vacuously")
	}

	devices, err := PCI(fs, rules)
	if err != nil {
		t.Fatalf("PCI: %v", err)
	}
	if len(devices) == 0 {
		t.Fatal("PCI returned no devices on a machine with six")
	}

	byAddress := map[string]Device{}
	for _, device := range devices {
		byAddress[device.Address] = device
	}

	integrated, ok := byAddress["0000:00:02.0"]
	if !ok {
		t.Fatal("integrated GPU 0000:00:02.0 not found")
	}
	if !integrated.IsDisplay() {
		t.Error("0000:00:02.0 is not classified as a display controller")
	}
	if !integrated.BootVGA {
		t.Error("BootVGA is false on the device the firmware boots the panel from; " +
			"this is the single most useful value on a hybrid machine")
	}
	if integrated.Driver != "xe" {
		t.Errorf("integrated driver = %q, want %q", integrated.Driver, "xe")
	}
	if integrated.Vendor != "0x8086" || integrated.Device != "0x46a6" {
		t.Errorf("integrated ids = %s/%s, want 0x8086/0x46a6",
			integrated.Vendor, integrated.Device)
	}

	// The discrete GPU has no driver bound. Its presence must not imply it
	// will accelerate anything, but its modalias must still resolve to the
	// modules that could bind it — that is what lets a profile record
	// `integrated: auto` on a basis rather than a hope.
	discrete, ok := byAddress["0000:01:00.0"]
	if !ok {
		t.Fatal("discrete GPU 0000:01:00.0 not found")
	}
	if discrete.Driver != "" {
		t.Errorf("discrete driver = %q, want empty; this fixture models an "+
			"unbound device", discrete.Driver)
	}
	if discrete.BootVGA {
		t.Error("BootVGA is true on the discrete GPU")
	}

	wireless, ok := byAddress["0000:00:14.3"]
	if !ok {
		t.Fatal("wireless device 0000:00:14.3 not found")
	}
	if !wireless.IsWireless() {
		t.Error("class 028000 is not classified as wireless")
	}
	if !wireless.IsNetwork() {
		t.Error("wireless should also satisfy IsNetwork")
	}

	audio, ok := byAddress["0000:00:1f.3"]
	if !ok {
		t.Fatal("audio device 0000:00:1f.3 not found")
	}
	if !audio.IsAudio() {
		t.Error("class 040300 is not classified as audio")
	}

	// The NVMe disk enumerates behind VMD at 01:00.1, which is the usual reason
	// a detection misses storage entirely.
	nvme, ok := byAddress["0000:01:00.1"]
	if !ok {
		t.Fatal("NVMe 0000:01:00.1 not found")
	}
	if !nvme.IsStorage() {
		t.Error("class 010802 is not classified as storage")
	}
	if nvme.Driver != "nvme" {
		t.Errorf("nvme driver = %q, want nvme", nvme.Driver)
	}
}

// TestCandidateModulesForUnboundDevice checks the promise that an unbound
// device still names what could bind it.
func TestCandidateModulesForUnboundDevice(t *testing.T) {
	rules := []AliasRule{
		{Pattern: "pci:v00008086d000046A6sv*sd*bc03sc00i00", Module: "i915"},
		{Pattern: "pci:v00008086d000046A6sv*sd*bc03sc00i00", Module: "xe"},
		{Pattern: "pci:v00008086d000051F0sv*sd*bc02sc00i00", Module: "iwlwifi"},
		{Pattern: "pci:v000010DED00002684sv*sd*bc03sc00i00", Module: "nvidia"},
	}

	// The exact case the design record quotes: an Intel display device with no
	// driver loaded still reports both modules that could bind it.
	got := CandidateModules("pci:v00008086d000046A6sv00001043sd00008762bc03sc00i00", rules)
	want := []string{"i915", "xe"}
	if len(got) != len(want) {
		t.Fatalf("CandidateModules = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CandidateModules = %v, want %v (sorted, deduplicated)", got, want)
		}
	}

	// A malformed pattern in a 40,000-line alias file must not cost the probe.
	withBad := append(rules, AliasRule{Pattern: "pci:v[[[bad", Module: "never"})
	if got := CandidateModules("pci:v00008086d000046A6sv00001043sd00008762bc03sc00i00", withBad); len(got) != 2 {
		t.Errorf("a malformed pattern changed the result: %v", got)
	}

	if got := CandidateModules("", rules); got != nil {
		t.Errorf("CandidateModules(\"\") = %v, want nil", got)
	}
	if got := CandidateModules("pci:v00008086d000046A6sv00001043sd00008762bc03sc00i00", nil); got != nil {
		t.Errorf("CandidateModules with no rules = %v, want nil", got)
	}
}

// TestPCIOnMachineWithoutBus checks that no PCI is a fact and not a failure.
func TestPCIOnMachineWithoutBus(t *testing.T) {
	devices, err := PCI(fixture(t, "arm-board"), nil)
	if err != nil {
		t.Fatalf("PCI on a machine with no PCI bus returned an error: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("PCI returned %d devices on an ARM board", len(devices))
	}
}

// TestPCIUnboundDevicesAreNotCountedAsWorking checks the negative fixture where
// neither GPU has a driver. Two GPUs must still be two devices, and neither
// may be described as working.
func TestPCIUnboundDevicesAreNotWorking(t *testing.T) {
	fs := fixture(t, "hybrid-no-drivers")

	rules, err := AliasRules(filepath.Join(fs.Root(), "modules.alias"))
	if err != nil {
		t.Fatalf("AliasRules: %v", err)
	}

	devices, err := PCI(fs, rules)
	if err != nil {
		t.Fatalf("PCI: %v", err)
	}

	var displays int
	for _, device := range devices {
		if !device.IsDisplay() {
			continue
		}
		displays++
		if device.Driver != "" {
			t.Errorf("%s reports driver %q; this fixture models both unbound",
				device.Address, device.Driver)
		}
	}

	if displays != 2 {
		t.Errorf("found %d display devices, want 2: an unbound device is still a "+
			"device, and losing it would describe a machine that does not exist", displays)
	}
}

// TestPCIReportsVirtualFunctions checks that a passed-through function is
// marked rather than silently counted as a second GPU.
func TestPCIReportsVirtualFunctions(t *testing.T) {
	dir := t.TempDir()
	dev := filepath.Join(dir, "bus/pci/devices/0000:03:00.1")
	if err := os.MkdirAll(dev, 0o755); err != nil {
		t.Fatal(err)
	}
	for attr, value := range map[string]string{
		"vendor": "0x10de", "device": "0x2684", "class": "0x030000",
	} {
		if err := os.WriteFile(filepath.Join(dev, attr), []byte(value+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// virtfn exists and is empty. Existence is the signal.
	if err := os.WriteFile(filepath.Join(dev, "virtfn"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	fs, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	devices, err := PCI(fs, nil)
	if err != nil {
		t.Fatalf("PCI: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("got %d devices, want 1", len(devices))
	}
	if !devices[0].IsVirtualFunction {
		t.Error("a device carrying virtfn was not marked as a virtual function")
	}
}

// TestUSBTopologyReconstruction checks that the bus/port path is parsed out of
// the directory name, which is what tells a peripheral plugged into the laptop
// from one plugged into a dock.
func TestUSBTopologyReconstruction(t *testing.T) {
	tests := []struct {
		name      string
		wantBus   int
		wantPort  int
		wantIface int
		wantDepth int
		root      bool
	}{
		{"usb1", 1, 0, 0, 0, true},
		{"1-0:1.0", 1, 0, 1, 1, false},
		{"1-1:1.0", 1, 1, 1, 1, false},
		// One hop further out: a device behind a hub on port 10.2 is depth 2.
		{"3-10.2:1.0", 3, 10, 1, 2, false},
		{"3-10.2.1:1.0", 3, 10, 1, 3, false},
		{"garbage", 0, 0, 0, 0, false},
	}

	for _, tc := range tests {
		got := ParseUSBName(tc.name)
		if got.IsRootHub != tc.root {
			t.Errorf("%s: IsRootHub = %v, want %v", tc.name, got.IsRootHub, tc.root)
		}
		if got.Bus != tc.wantBus {
			t.Errorf("%s: Bus = %d, want %d", tc.name, got.Bus, tc.wantBus)
		}
		if got.Port != tc.wantPort {
			t.Errorf("%s: Port = %d, want %d", tc.name, got.Port, tc.wantPort)
		}
		if got.Interface != tc.wantIface {
			t.Errorf("%s: Interface = %d, want %d", tc.name, got.Interface, tc.wantIface)
		}
		if got.Depth != tc.wantDepth {
			t.Errorf("%s: Depth = %d, want %d", tc.name, got.Depth, tc.wantDepth)
		}
	}
}

// TestUSBReadsDevices checks VID/PID mapping and that the serial attribute is
// never read, however present it is.
func TestUSBReadsDevices(t *testing.T) {
	devices, err := USB(fixture(t, "hybrid-laptop"))
	if err != nil {
		t.Fatalf("USB: %v", err)
	}
	if len(devices) == 0 {
		t.Fatal("USB returned no devices on a machine with a root hub and a reader")
	}

	byName := map[string]USBDevice{}
	for _, device := range devices {
		byName[device.Name] = device
	}

	fingerprint, ok := byName["1-1:1.0"]
	if !ok {
		t.Fatal("fingerprint reader 1-1:1.0 not found")
	}
	if fingerprint.IDVendor != "0c45" || fingerprint.IDProduct != "6366" {
		t.Errorf("fingerprint ids = %s/%s, want 0c45/6366",
			fingerprint.IDVendor, fingerprint.IDProduct)
	}
	if fingerprint.Manufacturer != "Goodix" {
		t.Errorf("fingerprint manufacturer = %q", fingerprint.Manufacturer)
	}
	if !fingerprint.Authorized {
		t.Error("an authorized device reported authorized=false")
	}

	// USBTopology must be populated from the directory name.
	if fingerprint.Topology.Bus != 1 || fingerprint.Topology.Port != 1 {
		t.Errorf("fingerprint topology = %+v, want bus 1 port 1", fingerprint.Topology)
	}

}
