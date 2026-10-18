package probe

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
)

// Device is one PCI function as sysfs reports it.
//
// Every field here is a published hardware identifier. Nothing in this struct
// identifies a person or an asset, which is why the whole struct is classified
// "kept" in the redaction policy — PCI vendor, device, subsystem and revision
// IDs are printed on the box.
type Device struct {
	// Address is the sysfs directory name: "0000:01:00.0". It is the stable key
	// for a function and is also what the display wiring hangs off.
	Address string `json:"address"`

	Vendor string `json:"vendor,omitempty"`
	Device string `json:"device,omitempty"`
	Class  string `json:"class,omitempty"`

	SubsystemVendor string `json:"subsystem_vendor,omitempty"`
	SubsystemDevice string `json:"subsystem_device,omitempty"`
	Revision        string `json:"revision,omitempty"`

	// Driver is the basename of the /sys/bus/pci/drivers symlink, empty when
	// nothing is bound.
	//
	// An unbound device is emphatically not a working device. A `nvidia`
	// device with no driver is a device, not a GPU, and its presence implies
	// nothing about acceleration — so Driver is carried separately from the
	// device's existence rather than being folded into a single "GPU: yes".
	Driver string `json:"driver,omitempty"`

	// CandidateModules are the modules the kernel says could bind this device,
	// parsed from modalias. On a machine whose GPU has no driver loaded this
	// still reports "i915, xe", which is what lets a profile record
	// `integrated: auto` with a basis rather than a hope.
	CandidateModules []string `json:"candidate_modules,omitempty"`

	// BootVGA marks the device the firmware drives the display from. On a
	// hybrid machine this is the single most useful value in the whole probe:
	// it is how detection knows the panel hangs off the integrated GPU rather
	// than the discrete one, which is the fact a mux profile depends on.
	BootVGA bool `json:"boot_vga"`

	// IsVirtualFunction marks a passed-through VF. One must never be counted as
	// a second GPU: on a vGPU or in a VM an honest report says so rather than
	// describing a machine that does not exist.
	IsVirtualFunction bool `json:"is_virtual_function"`
}

// Class helpers. The class attribute is 6 hex digits — class, subclass,
// interface — and the ones that matter to a profile are named here so that
// classification is one comparison rather than a substring test at every call
// site.
//
// They are the 4-digit class portions. The class attribute as sysfs writes it
// is "0x030000" for a VGA controller, so normalisePCIClass strips the prefix and
// takes the leading class digit pair before any of these comparisons happen.
const (
	classVGA               = "0300"
	class3DController      = "0302"
	classEthernet          = "0200"
	classOtherNetwork      = "0280"
	classAudio             = "0403"
	classSATAController    = "0106"
	classNVMeController    = "0108"
	classUSBXHCI           = "0c03"
	classUSBXHCIWithPaging = "0c04"
)

// normalizePCIClass reduces a sysfs class attribute to its 4-digit class code.
//
// sysfs writes "0x030000"; the subclass and interface live in the trailing four
// digits and are not what "is this a GPU" means. Comparing the raw attribute
// against a 4-digit constant would silently classify nothing, which is the kind
// of failure that looks like "this machine has no GPU".
func normalizePCIClass(class string) string {
	class = strings.TrimSpace(class)
	class = strings.TrimPrefix(class, "0x")
	class = strings.TrimPrefix(class, "0X")
	if len(class) < 4 {
		return class
	}
	return class[:4]
}

// IsDisplay reports whether this is a display controller.
func (d Device) IsDisplay() bool { return d.Class == classVGA || d.Class == class3DController }

// IsNetwork reports whether this is any network controller, wired or wireless.
func (d Device) IsNetwork() bool { return d.Class == classEthernet || d.Class == classOtherNetwork }

// IsWireless reports whether this is the wireless class specifically. A laptop
// whose only wireless chip is this device is the offline-risk signal, which is
// why it is separated from IsNetwork.
func (d Device) IsWireless() bool { return d.Class == classOtherNetwork }

// IsAudio reports whether this is an audio device, which includes the HDMI
// audio function on a discrete GPU.
func (d Device) IsAudio() bool { return d.Class == classAudio }

// IsStorage reports whether this is a storage controller. Note that a
// VMD-managed NVMe disk enumerates as class 0108 behind 00:1e.0 rather than
// under the PCH, which is the usual reason a detection misses it.
func (d Device) IsStorage() bool {
	return d.Class == classSATAController || d.Class == classNVMeController
}

// IsUSBHost reports whether this is a USB host controller.
func (d Device) IsUSBHost() bool { return d.Class == classUSBXHCI || d.Class == classUSBXHCIWithPaging }

// PCI reads /sys/bus/pci/devices/ relative to the reader's root.
//
// rules is the kernel's modules.alias, passed in so that the candidate module
// list can be resolved without this function knowing where the kernel keeps it.
// A nil rules slice leaves every device's CandidateModules empty, which is a
// degraded but correct result rather than an error.
//
// A machine with no PCI bus — an ARM board, some virtual machines — returns an
// empty slice and no error. The distinction matters: "no PCI" is a fact about
// the machine, and a profile written for such a board must be able to match
// against its absence rather than against a probe failure.
func PCI(fs *FS, rules []AliasRule) ([]Device, error) {
	names, err := fs.entries("bus/pci/devices")
	if err != nil {
		return nil, fmt.Errorf("pci: reading bus/pci/devices: %w", err)
	}

	devices := make([]Device, 0, len(names))
	for _, name := range names {
		device, err := readPCIDevice(fs, name, rules)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func readPCIDevice(fs *FS, address string, rules []AliasRule) (Device, error) {
	device := Device{Address: address}
	base := "bus/pci/devices/" + address

	for attr, target := range map[string]*string{
		"vendor":           &device.Vendor,
		"device":           &device.Device,
		"class":            &device.Class,
		"subsystem_vendor": &device.SubsystemVendor,
		"subsystem_device": &device.SubsystemDevice,
		"revision":         &device.Revision,
	} {
		value, present, err := fs.readString(base + "/" + attr)
		if err != nil {
			return device, fmt.Errorf("pci: reading %s of %s: %w", attr, address, err)
		}
		if present {
			*target = value
		}
	}
	device.Class = normalizePCIClass(device.Class)
	// driver is a symlink into /sys/bus/pci/drivers/<name>. Resolving the link
	// and taking the basename gives the module name without the probe ever
	// listing the drivers directory, which would be a second thing to go wrong.
	driver, err := fs.readLink(base + "/driver")
	if err != nil {
		return device, fmt.Errorf("pci: resolving driver of %s: %w", address, err)
	}
	device.Driver = driver

	modalias, present, err := fs.readString(base + "/modalias")
	if err != nil {
		return device, fmt.Errorf("pci: reading modalias of %s: %w", address, err)
	}
	if present {
		device.CandidateModules = CandidateModules(modalias, rules)
	}

	bootVGA, present, err := fs.readInt(base + "/boot_vga")
	if err != nil {
		return device, fmt.Errorf("pci: reading boot_vga of %s: %w", address, err)
	}
	device.BootVGA = present && bootVGA == 1

	// virtfn marks a device passed through from a hypervisor. Counting one as a
	// real GPU would describe a machine with two graphics adapters on hardware
	// that has one, which is the kind of wrong that produces a confident,
	// incorrect profile.
	device.IsVirtualFunction = fs.stat(base+"/virtfn") == nil

	return device, nil
}

// AliasRule is one line of the kernel's modules.alias: a glob pattern for a
// device alias and the module that handles it.
type AliasRule struct {
	Pattern string
	Module  string
}

// AliasRules loads a kernel modules.alias file.
//
// The path is injected rather than derived, for the same reason sysfs is: the
// file's location moves between releases (/lib/modules, /usr/lib/modules) and a
// probe that hard-codes one loses every module name on the other layout. It is
// also the only file outside sysfs this subsystem reads, and it is data the
// kernel ships rather than hardware.
//
// An absent file is not an error. A machine with no modules.alias for the
// running kernel has told us that nothing we could name will bind, and that is
// a result.
func AliasRules(path string) ([]AliasRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("pci: reading modules.alias: %w", err)
	}

	var rules []AliasRule
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pattern, module, found := strings.Cut(line, " ")
		if !found {
			continue
		}
		module = strings.TrimSpace(module)
		// A line with no module is not a rule. Handing back an empty name would
		// put an empty string into the candidate list, which a report would then
		// print as though something could bind.
		if module == "" {
			continue
		}
		rules = append(rules, AliasRule{
			Pattern: pattern,
			Module:  module,
		})
	}
	return rules, nil
}

// CandidateModules returns every module that could bind a device alias.
//
// This is what modprobe does when no driver is loaded yet, done here rather
// than by shelling out — the rule this whole package is built on. It is also
// what lets a report name the modules that *would* bind a device that has none:
// on a hybrid laptop the GPU with no driver loaded still reports "i915, xe",
// which is what allows a profile to record `integrated: auto` on a basis rather
// than a hope.
//
// Every matching rule is returned, deduplicated and ordered. Returning the whole
// set is deliberate. A single name would be a claim; the honest statement about
// an unbound device is that either of these could bind.
//
// A malformed pattern is skipped rather than fatal. The alias file is
// machine-generated and tens of thousands of lines long, and one odd line must
// not cost the whole probe.
func CandidateModules(modalias string, rules []AliasRule) []string {
	if modalias == "" || len(rules) == 0 {
		return nil
	}

	seen := map[string]bool{}
	var candidates []string
	for _, rule := range rules {
		if rule.Module == "" || seen[rule.Module] {
			continue
		}
		matched, err := path.Match(rule.Pattern, modalias)
		if err != nil {
			continue
		}
		if matched {
			seen[rule.Module] = true
			candidates = append(candidates, rule.Module)
		}
	}
	sort.Strings(candidates)
	return candidates
}
