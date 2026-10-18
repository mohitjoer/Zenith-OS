package main

import (
	"os"
	"path/filepath"
	"strings"

	"zenith/hardware/probe"
)

// captureAllow is the set of sysfs attributes a capture may write.
//
// It is an allowlist, and that is the whole design of this file rather than a
// detail of it. A capture is committed to a public repository, so anything it
// writes is published; and a deny-list leaks by construction, because every
// attribute nobody thought of is written by default. Two did exactly that before
// this was an allowlist: the DMI serial fields, and the device tree's
// serial-number property.
//
// The rule is the design record's own: a capture writes *exactly the files the
// readers consume*. Anything a reader does not read cannot reach a report, so it
// has no business in a fixture — and this way "cannot reach a report" and
// "cannot reach the disk" are the same statement, checkable by reading one
// table instead of auditing a filter.
//
// A path matches when it is globbed against these patterns relative to the
// capture root. `{...}` means any one of the listed names.
var captureAllow = []string{
	// Identity, filtered separately so the reader's own policy decides.
	"class/dmi/id/{sys_vendor,product_name,product_version,product_family," +
		"product_sku,board_vendor,board_name,board_version,chassis_type," +
		"bios_vendor,bios_version,bios_date,bios_release,ec_firmware_release," +
		"product_uuid}",

	// The device tree: model and compatible only. serial-number is a device
	// asset identifier exactly as a SMBIOS one is, and the probe deliberately
	// never reads it.
	"firmware/devicetree/base/{model,compatible}",

	// PCI. Every attribute the reader reads, and nothing else — in particular
	// no resource or config file, which carries BARs and interrupt state that
	// identify a machine's wiring.
	"bus/pci/devices/*/{vendor,device,class,subsystem_vendor,subsystem_device," +
		"revision,modalias,boot_vga,driver,virtfn}",

	// USB. The reader does not read `serial`, so a capture does not write it:
	// USB serials are frequently per-device vendor garbage and are sometimes
	// duplicated across an entire production run.
	"bus/usb/devices/*/{idVendor,idProduct,bcdDevice,manufacturer,product," +
		"busnum,devnum,speed,version,driver,authorized}",

	// Sleep capability.
	"power/{state,mem_sleep}",

	// ACPI: names and scalar facts. A table body is never written, because an
	// AML table embeds identifiers as _DSM GUIDs and as strings in arguments.
	"firmware/acpi/{platform_profile,platform_profile_choices,pm_profile,bgrt}",
	"firmware/acpi/tables/{APIC,APM,BDAT,BOOT,CSRT,DMAR,DSDT,ECAP,EINJ,ERST," +
		"FACP,FPDT,GTDT,HEST,HPET,IBFT,IORT,IVRS,LPIT,MCFG,MSDM,NHLT,PCCT,PMTT," +
		"RGRT,SBST,SDEI,SLIC,SLIT,SPCR,STAO,TCPA,TPML,UEFI,VIOT,WAET,WDAT," +
		"WDDT,WDRT,WSMT,XENV}",

	// Panel and backlight.
	"class/backlight/*/max_brightness",
	"class/drm/*/{edid,status,modes,enabled,dpms,connector_id}",

	// The module alias table the candidate-module lookup reads.
	"modules.alias",
}

// captureAllowed reports whether a capture may write one path.
//
// The match is on the path relative to the capture root, with glob matching
// that understands a single path segment per `*`. A path that does not match is
// not written, and that includes every identifier attribute a reader happens not
// to read today.
func captureAllowed(relative string) bool {
	// The DMI directory has its own filter, driven by the reader's policy, so a
	// field reclassified later moves here automatically.
	if strings.HasPrefix(relative, "class/dmi/id/") {
		return false
	}

	for _, pattern := range captureAllow {
		if globMatch(pattern, relative) {
			return true
		}
	}
	return false
}

// dmiCapturable reports whether the reader's policy permits this DMI field into
// a capture.
//
// Deriving it from the reader's own policy keeps a capture and a report in step:
// a field reclassified in package probe moves here with it, instead of becoming a
// second list that quietly disagrees.
//
// The hashed class is deliberately excluded even though the policy calls it
// carried. Hashed means the *reader* hashes it on the way into a struct — the
// raw value never reaches a report. A capture happens before the reader does: it
// copies sysfs files. Writing a hashed field to disk would therefore write the
// raw UUID, which is precisely the value the hash exists to protect.
func dmiCapturable(name string) bool {
	switch class, listed := probe.DMIPolicy()[name]; {
	case !listed:
		return false
	case class == probe.DMIClassKept, class == probe.DMIClassMapped:
		return true
	default:
		return false
	}
}

// globMatch matches a path against a pattern containing `*` and `{a,b}`
// alternations.
//
// The standard library has no path glob that does this, and pulling one in for
// a dozen static patterns would be a dependency bought for nothing.
func globMatch(pattern, value string) bool {
	alternatives := []string{pattern}
	if open := strings.IndexByte(pattern, '{'); open >= 0 {
		close := strings.IndexByte(pattern[open:], '}')
		if close < 0 {
			return false
		}
		close += open

		prefix, suffix := pattern[:open], pattern[close+1:]
		alternatives = nil
		for _, choice := range strings.Split(pattern[open+1:close], ",") {
			alternatives = append(alternatives, prefix+choice+suffix)
		}
	}

	for _, alternative := range alternatives {
		if segmentsMatch(strings.Split(alternative, "/"), strings.Split(value, "/")) {
			return true
		}
	}
	return false
}

// segmentsMatch matches path segments, where a segment of "*" matches one
// segment and nothing more.
//
// A `*` never spans a separator. That matters: `bus/pci/devices/*/driver` must
// not match a file two directories down.
func segmentsMatch(pattern, value []string) bool {
	if len(pattern) != len(value) {
		return false
	}
	for i := range pattern {
		if pattern[i] == "*" {
			continue
		}
		if pattern[i] != value[i] {
			return false
		}
	}
	return true
}

// capture copies the sysfs tree the readers consume, and nothing else.
//
// This is the whole reason the subsystem takes a root: a capture is a fixture,
// and the fixture is filtered as it is written rather than at review.
func capture(root, out string) (captured, skipped int, err error) {
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			// A directory the reader cannot read is skipped, not fatal: the
			// point of a capture is what the readers actually use.
			return nil
		}

		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}

		if strings.HasPrefix(relative, "class/dmi/id/") {
			if !dmiCapturable(filepath.Base(relative)) {
				skipped++
				return nil
			}
		} else if !captureAllowed(relative) {
			skipped++
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		target := filepath.Join(out, relative)
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
			return mkErr
		}
		// 0444 mirrors what sysfs exposes, and means a committed fixture cannot
		// be edited without noticing.
		if writeErr := os.WriteFile(target, data, 0o444); writeErr != nil {
			return writeErr
		}
		captured++
		return nil
	})

	return captured, skipped, err
}
