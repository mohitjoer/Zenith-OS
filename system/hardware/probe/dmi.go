package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// dmiClass is the redaction class of every DMI field this reader emits.
//
// The rule from docs/hardware-detection.md is that there is no default class:
// an unclassified field is a validation error in the reader, not something the
// redactor is asked to catch later. Layer one of the redaction contract is that
// this reader copies named fields into a struct and never copies a directory,
// so a field that is not listed here has no path to a report at all.
//
// productUUID is the one hashed field the reader carries. The raw serial
// numbers, asset tags and product_uuid are never placed in Fields; the reader
// hashes product_uuid at the boundary and the redactor turns that hash into a
// keyed token. See package redact.
// DMIClass is the redaction class of a DMI field. It is exported because the
// report redactor derives its keep-set from the same policy.
type DMIClass string

const (
	// DMIClassKept is a field a report carries verbatim.
	DMIClassKept DMIClass = "kept"
	// DMIClassMapped is a field carried after mapping, such as the SMBIOS
	// chassis integer becoming an ACPI chassis string.
	DMIClassMapped DMIClass = "kept,mapped"
	// DMIClassHashed is a field the reader hashes and the redactor turns into a
	// keyed token.
	DMIClassHashed DMIClass = "hashed"
	// DMIClassNotRead is a field that exists in sysfs and is never copied. The
	// not-read fields are the serials and the asset tags.
	DMIClassNotRead DMIClass = "not-read"
)

// DMIPolicy returns the DMI allowlist: every sysfs attribute under
// /sys/class/dmi/id/, and its redaction class.
//
// It is exported because the report redactor's sweep has to know the same set.
// Two copies of this policy is two chances for them to disagree, and the failure
// mode when they do is the worst one available: a field added to the reader's
// allowlist but not to the redactor's makes every report containing it fail to
// render, with an error that names a rule nobody can find in the code.
//
// There is no import cycle in this: package redact already depends on what
// package probe produces.
func DMIPolicy() map[string]DMIClass { return dmiPolicy }

// dmiPolicy is the allowlist. Every entry is a sysfs attribute under
// /sys/class/dmi/id/, and its class says what happens to the value.
//
// The "not-read" entries are named on purpose. product_serial, board_serial,
// chassis_serial, chassis_asset_tag, board_asset_tag and modalias exist in that
// directory and several are world-readable, so listing them with their class is
// what makes the omission a decision rather than an oversight. chassis_asset_tag
// in particular is mode 0444 on many machines and holds a real asset tag that
// maps to a purchasing record; the kernel's permission bits are not a
// redaction policy.
var dmiPolicy = map[string]DMIClass{
	"sys_vendor":          DMIClassKept,
	"product_name":        DMIClassKept,
	"product_version":     DMIClassKept,
	"product_family":      DMIClassKept,
	"product_sku":         DMIClassKept,
	"board_vendor":        DMIClassKept,
	"board_name":          DMIClassKept,
	"board_version":       DMIClassKept,
	"chassis_type":        DMIClassMapped,
	"bios_vendor":         DMIClassKept,
	"bios_version":        DMIClassKept,
	"bios_date":           DMIClassKept,
	"bios_release":        DMIClassKept,
	"ec_firmware_release": DMIClassKept,

	"product_uuid": DMIClassHashed,

	"product_serial":    DMIClassNotRead,
	"board_serial":      DMIClassNotRead,
	"chassis_serial":    DMIClassNotRead,
	"chassis_asset_tag": DMIClassNotRead,
	"board_asset_tag":   DMIClassNotRead,
	"modalias":          DMIClassNotRead,
}

// Fields is what a DMI probe returns.
//
// Every string field is either the machine's value or empty. An empty field and
// a field the machine does not have are the same thing here, and that is
// deliberate: a profile that names board_name must not match a machine whose
// board_name is empty, and an absent value satisfies only an absent or "*"
// field. Carrying a separate Present bool for every string would invite a
// comparison somewhere that forgets to check it.
type Fields struct {
	SysVendor      string `json:"sys_vendor,omitempty"`
	ProductName    string `json:"product_name,omitempty"`
	ProductVersion string `json:"product_version,omitempty"`
	ProductFamily  string `json:"product_family,omitempty"`
	ProductSKU     string `json:"product_sku,omitempty"`

	BoardVendor  string `json:"board_vendor,omitempty"`
	BoardName    string `json:"board_name,omitempty"`
	BoardVersion string `json:"board_version,omitempty"`

	// ChassisType is the ACPI chassis type string ("notebook", "laptop",
	// "mini PC", ...), not the raw SMBIOS integer. It is mapped here rather than
	// at render time so that a profile's `chassis_type: notebook` compares
	// against the same word the report prints.
	ChassisType string `json:"chassis_type,omitempty"`

	BIOSVendor  string `json:"bios_vendor,omitempty"`
	BIOSVersion string `json:"bios_version,omitempty"`
	BIOSDate    string `json:"bios_date,omitempty"`

	BIOSRelease       string `json:"bios_release,omitempty"`
	ECFirmwareRelease string `json:"ec_firmware_release,omitempty"`

	// ProductUUIDHash is the reader's hash of product_uuid. It is not a token:
	// the token is a keyed HMAC built in package redact, and this value exists
	// so that the redactor has the material without this package ever holding
	// the raw UUID.
	ProductUUIDHash string `json:"-"`

	// Present is false when the machine has no readable DMI at all — an ARM
	// board with a device tree and no SMBIOS table, or a virtual machine whose
	// firmware exposes none. It is reported so that matching can say
	// "dmi unavailable" rather than treating an empty struct as a machine with
	// no model.
	Present bool `json:"present"`
}

// chassisTypes maps the raw SMBIOS chassis type integer to the ACPI chassis
// type string. The numeric type is meaningless to a human reading a report and
// to anyone writing a profile by hand, so it never reaches either.
var chassisTypes = map[int]string{
	3:  "desktop",
	4:  "low-profile desktop",
	5:  "pizza box",
	6:  "mini tower",
	7:  "tower",
	8:  "portable",
	9:  "laptop",
	10: "notebook",
	11: "hand held",
	12: "docking station",
	13: "all-in-one",
	14: "sub-notebook",
	30: "tablet",
	31: "convertible",
	32: "detachable",
	34: "embedded",
	35: "mini PC",
}

// ChassisTypeName returns the ACPI chassis type string for a raw SMBIOS integer.
// An integer outside the table is not an error and is not guessed: a firmware
// that reports 99 has not told us anything usable.
func ChassisTypeName(raw int) (string, bool) {
	name, ok := chassisTypes[raw]
	return name, ok
}

// DMI reads /sys/class/dmi/id/ relative to the reader's root.
//
// A machine with no DMI returns (Fields{Present: false}, nil) — not an error.
// That distinction is the difference between "this board has no SMBIOS table"
// and "the probe is broken", and only one of them is a fact about hardware.
func DMI(fs *FS) (Fields, error) {
	fields := Fields{}

	names, err := fs.entries("class/dmi/id")
	if err != nil {
		return fields, fmt.Errorf("dmi: reading class/dmi/id: %w", err)
	}
	if len(names) == 0 {
		return fields, nil
	}

	// The identity fields are what decide whether this machine can be claimed by
	// a profile at all. If none of them came back with a value there is no DMI
	// here worth reporting, even though the directory exists.
	identity := []string{"sys_vendor", "product_name", "product_version", "product_family"}
	anyIdentity := false

	for _, name := range names {
		class, listed := dmiPolicy[name]
		if !listed {
			// An unlisted attribute in the DMI directory is not an error: the
			// kernel adds attributes between releases and a new one is simply
			// not read. It is also not silently included, which is the property
			// that matters.
			continue
		}
		if class == DMIClassNotRead {
			continue
		}

		raw, present, err := fs.readString("class/dmi/id/" + name)
		if err != nil {
			return fields, fmt.Errorf("dmi: reading %s: %w", name, err)
		}
		if !present || raw == "" {
			continue
		}

		switch name {
		case "sys_vendor":
			fields.SysVendor, anyIdentity = raw, true
		case "product_name":
			fields.ProductName = raw
		case "product_version":
			fields.ProductVersion = raw
		case "product_family":
			fields.ProductFamily = raw
		case "product_sku":
			fields.ProductSKU = raw
		case "board_vendor":
			fields.BoardVendor = raw
		case "board_name":
			fields.BoardName = raw
		case "board_version":
			fields.BoardVersion = raw
		case "bios_vendor":
			fields.BIOSVendor = raw
		case "bios_version":
			fields.BIOSVersion = raw
		case "bios_date":
			fields.BIOSDate = raw
		case "bios_release":
			fields.BIOSRelease = raw
		case "ec_firmware_release":
			fields.ECFirmwareRelease = raw

		case "chassis_type":
			rawInt, ok, err := fs.readInt("class/dmi/id/chassis_type")
			if err != nil {
				return fields, fmt.Errorf("dmi: reading chassis_type: %w", err)
			}
			if ok {
				fields.ChassisType, _ = ChassisTypeName(rawInt)
			}

		case "product_uuid":
			fields.ProductUUIDHash = hashIdentifier("product_uuid", raw)
		}
	}

	for _, name := range identity {
		if fieldValue(&fields, name) != "" {
			anyIdentity = true
			break
		}
	}
	fields.Present = anyIdentity

	return fields, nil
}

// fieldValue returns one allowlisted field by its sysfs attribute name, so the
// presence check above does not need a second copy of the switch.
func fieldValue(fields *Fields, attr string) string {
	switch attr {
	case "sys_vendor":
		return fields.SysVendor
	case "product_name":
		return fields.ProductName
	case "product_version":
		return fields.ProductVersion
	case "product_family":
		return fields.ProductFamily
	}
	return ""
}

// hashIdentifier is the reader-side half of the token scheme.
//
// This is an unkeyed SHA-256 over the field name and the raw value, and it
// exists so that the raw product_uuid stops existing the moment it has been
// read. The keyed token that actually appears in a report is built in package
// redact, which holds the salt; this hash never reaches a report and cannot be
// correlated across machines, because the same UUID on two machines is not the
// same input.
func hashIdentifier(field, value string) string {
	sum := sha256.Sum256([]byte("zenith/hardware-reader/v1\x00" + field + "\x00" + value))
	return hex.EncodeToString(sum[:8])
}

// ParseBIOSDate parses the SMBIOS `bios_date` attribute.
//
// SMBIOS defines the field as MM/DD/YYYY, and the value on a real machine
// ("12/03/2024") is 3 December, not 12 March. The ambiguity is not a detail:
// getting it backwards silently accepts or rejects a profile's
// `bios_date: ">=2024-01-01"` comparator on some machines and not others.
//
// A value that is not exactly eight digits and two separators is a validation
// error, never a guess. A partially-parsed date would let a profile match on a
// firmware date nobody actually reported.
func ParseBIOSDate(value string) (time.Time, error) {
	const layout = "01/02/2006"
	if value == "" {
		return time.Time{}, fmt.Errorf("bios_date: empty")
	}
	if len(value) != len(layout) {
		return time.Time{}, fmt.Errorf("bios_date: %q is not MM/DD/YYYY", value)
	}
	parsed, err := time.Parse(layout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("bios_date: %q is not MM/DD/YYYY: %w", value, err)
	}
	return parsed, nil
}
