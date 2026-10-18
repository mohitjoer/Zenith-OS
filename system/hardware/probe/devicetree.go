package probe

import (
	"bytes"
	"strings"
)

// DeviceTree is what the flattened device tree reports about the board.
//
// This exists because DMI cannot reach a large and growing class of machines.
// DMI is an SMBIOS structure, and an ARM board with a device tree and no SMBIOS
// table has no DMI at all — not a broken one, not an empty one, none. Before
// this probe, such a machine could never match a profile as the schema stands,
// so a perfectly working ARM laptop could never be anything but tier 3. That is
// not a documented limitation; it is a ceiling on reach, and reach is the
// product.
type Tree struct {
	// Present is false on an x86 machine with no device tree, which is most of
	// them. It is a fact, not a failure.
	Present bool `json:"present"`

	// Model is the board's model string: "Raspberry Pi 5 Model B",
	// "Jetson AGX Orin Developer Kit", "Qualcomm Snapdragon X Elite Laptop".
	Model string `json:"model,omitempty"`

	// Compatible is the ordered list of compatible strings, most specific first.
	// A board typically lists a machine string then a SoC family string, e.g.
	// "raspberrypi,5-model-b", "brcm,bcm2712".
	Compatible []string `json:"compatible,omitempty"`

	// SerialNumber is read and deliberately not carried. Device-tree serial
	// numbers are asset identifiers exactly as SMBIOS ones are, and this probe's
	// whole job is to reach a board without collecting it.
	//
	// It is named here only so the omission is visible in the source rather than
	// looking like an oversight.
	_ struct{}
}

// DeviceTree reads /sys/firmware/devicetree/base/ relative to the reader's root.
//
// Device-tree properties are a sequence of NUL-terminated strings, not a single
// line, so they cannot be read the way a sysfs attribute can. A property written
// as one string still ends in a NUL, and a list property has one NUL per entry —
// a parser that trims only the trailing newline would leave the separators in
// and produce a "compatible" value like "board\0soc" that no profile could name.
func DeviceTree(fs *FS) (Tree, error) {
	var tree Tree

	if fs.stat("firmware/devicetree/base") != nil {
		return tree, nil
	}
	tree.Present = true

	model, present, err := fs.readBytes("firmware/devicetree/base/model")
	if err != nil {
		return tree, err
	}
	if present {
		tree.Model = firstDeviceTreeString(model)
	}

	compatible, present, err := fs.readBytes("firmware/devicetree/base/compatible")
	if err != nil {
		return tree, err
	}
	if present {
		tree.Compatible = deviceTreeStrings(compatible)
	}

	return tree, nil
}

// firstDeviceTreeString returns the first NUL-terminated string in a property.
func firstDeviceTreeString(raw []byte) string {
	values := deviceTreeStrings(raw)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// deviceTreeStrings splits a property into its NUL-terminated strings.
//
// Padding NULs at the end of a property are dropped rather than yielding a run
// of empty strings, and the result is empty rather than a slice of one blank
// string when the property is entirely padding.
func deviceTreeStrings(raw []byte) []string {
	parts := bytes.Split(raw, []byte{0})

	var values []string
	for _, part := range parts {
		value := strings.TrimSpace(string(part))
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

// IdentityFields renders a device tree as match-key values.
//
// The mapping is deliberate: a profile that claims a machine should be written
// once, in the same vocabulary, and work whether the board answers in SMBIOS or
// in a device tree. Introducing a parallel `devicetree_model:` key for ARM
// machines would mean every profile is written twice and neither copy is checked
// against the other, and the reach problem would come back wearing a different
// hat.
//
// The keys are the existing match-key fields, so:
//
//   - model       becomes product_name, which is what a reader expects to find
//   - compatible  becomes sys_vendor, taking the most specific entry
//
// An empty field is omitted rather than emitted as an empty string. Matching
// treats an absent field as unconstrained and an empty one as a value the
// machine must have, and the difference between them is the difference between
// "this profile says nothing about it" and "this profile requires it to be
// blank".
func (t Tree) IdentityFields() map[string]string {
	fields := map[string]string{}

	if t.Model != "" {
		fields["product_name"] = t.Model
	}
	if len(t.Compatible) > 0 {
		// The compatible list is ordered most specific first, so the head is the
		// closest thing a device tree has to a vendor.
		fields["sys_vendor"] = t.Compatible[0]
	}

	return fields
}
