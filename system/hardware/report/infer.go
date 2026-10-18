package report

import (
	"fmt"
	"strings"

	"zenith/hardware/probe"
)

// Inference is what a report can honestly derive from a detection result.
//
// Every field is a statement the machine supports, not a guess dressed up as
// one. Where detection cannot tell, the value is empty and the renderer says so
// in the output rather than filling it in — a profile that guesses is worse than
// a profile that admits it does not know, which is the rule the profile schema
// opens with.
type Inference struct {
	// KernelFlavour is one of generic, hwe, oem. Never a patch version.
	KernelFlavour string

	// GPUIntegrated and GPUDiscrete name what should be installed for each
	// device. Empty means detection had no basis to say.
	GPUIntegrated string
	GPUDiscrete   string

	// HybridMux is true when the machine has two display devices and one of them
	// carries the panel.
	HybridMux bool

	// BootVGAVendor is the vendor of the device the firmware drives the display
	// from, which on a hybrid machine is how the panel is attributed.
	BootVGAVendor string

	// WifiDrivers are the modules that could bind each wireless device, in
	// device order. More than one means detection could not choose and the
	// profile must say so rather than picking silently.
	WifiDrivers []string

	// AudioCodec is "sof" when a Sound Open Firmware codec is indicated,
	// "hda" for plain Intel HD Audio, or empty when neither is determinable.
	AudioCodec string

	// Microcode is the microcode package the host bridge vendor implies.
	Microcode string

	// InternalPanel names the panel the machine is driving, if one is internal.
	InternalPanel string
}

// kernelFlavours is the closed set the fleet carries.
//
// It is the `key:` list in system/kernel/fleet.yaml. A profile naming anything
// outside this set is a validation failure, and a report proposing one is a bug
// in this package rather than something to pass through.
var kernelFlavours = map[string]bool{"generic": true, "hwe": true, "oem": true}

// microcodeByVendor maps the PCI vendor of the host bridge to the microcode
// package name the fleet carries.
//
// This is derived from the host bridge rather than from a CPU string because
// sysfs has the former and reading the latter means reading /proc, which is
// outside the root every reader in this subsystem is given. An Intel CPU in an
// AMD machine is not a thing; the converse is not either.
var microcodeByVendor = map[string]string{
	"0x8086": "intel-microcode",
	"0x1022": "amd64-microcode",
}

// infer derives what the machine supports from the probe output.
func infer(input Input) Inference {
	out := Inference{}

	if !kernelFlavours[input.KernelFlavour] {
		// An unknown flavour is not passed through. A profile naming a kernel
		// that does not exist is a validation failure at install time, which is
		// a worse place to find out than here.
		out.KernelFlavour = ""
	} else {
		out.KernelFlavour = input.KernelFlavour
	}

	displays := 0
	for _, device := range input.PCI {
		switch {
		case device.IsDisplay():
			displays++
			driver := displayDriver(device)
			if device.BootVGA {
				// The device the firmware drives the display from is the
				// integrated one on every hybrid machine this has been run on,
				// and naming it is what lets a profile configure the mux.
				out.GPUIntegrated = driver
				out.BootVGAVendor = device.Vendor
			} else if driver != "" {
				out.GPUDiscrete = driver
			}

		case device.IsWireless():
			if drivers := usableDrivers(device); len(drivers) == 1 {
				out.WifiDrivers = append(out.WifiDrivers, drivers[0])
			} else if len(drivers) > 1 {
				// Detection cannot choose between these. Reporting both is the
				// honest answer; picking one would be a guess in a field a
				// profile is written from.
				out.WifiDrivers = append(out.WifiDrivers, drivers...)
			}

		case device.IsAudio():
			if codec := audioCodec(device); codec != "" && out.AudioCodec == "" {
				out.AudioCodec = codec
			}
		}
	}

	out.HybridMux = displays > 1

	// The host bridge is class 0600, the only class that names the CPU vendor
	// without leaving sysfs.
	for _, device := range input.PCI {
		if !isHostBridge(device.Class) {
			continue
		}
		if packageName, ok := microcodeByVendor[strings.ToLower(device.Vendor)]; ok {
			out.Microcode = packageName
			break
		}
	}

	for _, panel := range input.EDID {
		if panel.Present && panel.Internal {
			out.InternalPanel = panel.Connector
			break
		}
	}

	return out
}

// isHostBridge reports whether a class attribute is the PCI host bridge.
func isHostBridge(class string) bool {
	normalized := strings.TrimPrefix(strings.ToLower(class), "0x")
	return strings.HasPrefix(normalized, "0600")
}

// displayDriver names what should be installed for a display device.
//
// An unbound device is not a working device, so a device with no driver and no
// resolvable candidate has no answer and gets one. Where the modalias does
// resolve, the candidates are the honest statement: "either of these could
// bind", which is what a profile records as `auto`.
func displayDriver(device probe.Device) string {
	if device.Driver == "xe" {
		return "intel"
	}
	if device.Driver == "i915" {
		return "intel"
	}
	if device.Driver == "amdgpu" {
		return "amdgpu"
	}
	if device.Driver == "nvidia" {
		return "nvidia"
	}
	if device.Driver == "nouveau" {
		return "nouveau"
	}

	// No driver bound. The candidate list is still evidence.
	candidates := device.CandidateModules
	switch {
	case contains(candidates, "nvidia"):
		return "nvidia-open"
	case contains(candidates, "amdgpu"):
		return "amdgpu"
	case contains(candidates, "i915"), contains(candidates, "xe"):
		return "auto"
	}
	return ""
}

// usableDrivers returns the modules that could bind a device, preferring the
// bound driver and otherwise taking every candidate.
func usableDrivers(device probe.Device) []string {
	if device.Driver != "" {
		return []string{device.Driver}
	}
	if len(device.CandidateModules) == 0 {
		return nil
	}
	unique := make([]string, 0, len(device.CandidateModules))
	seen := map[string]bool{}
	for _, module := range device.CandidateModules {
		if seen[module] {
			continue
		}
		seen[module] = true
		unique = append(unique, module)
	}
	return unique
}

// audioCodec names the codec path an audio device implies.
//
// Sound Open Firmware is the interesting case: a machine with no sound and no
// visible cause is nearly always a missing SOF package, and detection can see
// the SND_SOF driver among the candidates before it is bound.
func audioCodec(device probe.Device) string {
	if contains(device.CandidateModules, "snd_sof") ||
		contains([]string{device.Driver}, "snd_sof") {
		return "sof"
	}
	if device.Driver != "" || len(device.CandidateModules) > 0 {
		return "hda"
	}
	return ""
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// validate checks that an inference is safe to write into a profile, and
// returns every problem rather than the first.
//
// A generated profile is a starting point for a human, not a finished
// artefact, but a starting point that violates the schema is worse than one
// that says where it is wrong.
func (in Inference) validate() []error {
	var problems []error

	if in.KernelFlavour == "" {
		problems = append(problems, fmt.Errorf("no kernel flavour could be "+
			"derived; the profile must name one of generic, hwe or oem"))
	}

	// A hybrid machine needs a mux decision, and an unmuxed decision is the
	// single most common cause of "the panel is black after switching GPUs".
	if in.HybridMux && in.BootVGAVendor == "" {
		problems = append(problems, fmt.Errorf("two display devices found but "+
			"neither carries boot_vga; the panel cannot be attributed to a GPU, "+
			"so the mux cannot be configured and a human must say which GPU "+
			"drives the panel"))
	}

	return problems
}
