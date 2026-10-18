package probe

import "testing"

// TestDeviceTreeOnARMLaptop is the reach test.
//
// This machine has no SMBIOS table at all. Before the device-tree probe it
// could never match a profile, so a perfectly working ARM laptop was capped at
// tier 3 for good — which is a ceiling on "works on everyone's laptop", not a
// documented limitation of it.
func TestDeviceTreeOnARMLaptop(t *testing.T) {
	tree, err := DeviceTree(fixture(t, "arm-laptop"))
	if err != nil {
		t.Fatalf("DeviceTree: %v", err)
	}

	if !tree.Present {
		t.Fatal("Present is false on a board with a device tree")
	}
	if tree.Model != "Snapdragon X Elite Laptop" {
		t.Errorf("model = %q, want %q", tree.Model, "Snapdragon X Elite Laptop")
	}
	if len(tree.Compatible) != 2 {
		t.Fatalf("compatible = %v, want two entries", tree.Compatible)
	}
	if tree.Compatible[0] != "qcom,x1e-generic" {
		t.Errorf("compatible[0] = %q, want qcom,x1e-generic; the list is ordered "+
			"most specific first", tree.Compatible[0])
	}
}

// TestDeviceTreeStringsAreNULSeparated checks the parsing of a property that is
// a sequence of NUL-terminated strings rather than a line.
//
// Trimming only a trailing newline would leave the separators in and produce
// "qcom,x1e-generic\x00qcom,sc8280" as a single value that no profile could
// name — a value that looks parsed and is not.
func TestDeviceTreeStringsAreNULSeparated(t *testing.T) {
	tree, err := DeviceTree(fixture(t, "arm-laptop"))
	if err != nil {
		t.Fatalf("DeviceTree: %v", err)
	}
	for _, value := range append(tree.Compatible, tree.Model) {
		if containsNUL(value) {
			t.Errorf("value %q still contains a NUL separator", value)
		}
	}
}

// TestIdentityFieldsUseTheSharedMatchVocabulary checks that a device tree is
// projected into the existing match-key fields rather than into a parallel
// vocabulary.
//
// A separate devicetree_model: key would mean every profile is written twice
// and neither copy is checked against the other, and the reach problem would
// return in a different shape.
func TestIdentityFieldsUseTheSharedMatchVocabulary(t *testing.T) {
	tree, err := DeviceTree(fixture(t, "arm-laptop"))
	if err != nil {
		t.Fatalf("DeviceTree: %v", err)
	}

	fields := tree.IdentityFields()

	if fields["product_name"] != "Snapdragon X Elite Laptop" {
		t.Errorf("product_name = %q", fields["product_name"])
	}
	if fields["sys_vendor"] != "qcom,x1e-generic" {
		t.Errorf("sys_vendor = %q, want the most specific compatible entry",
			fields["sys_vendor"])
	}
	// Absent values are omitted, not emitted as empty strings. Matching treats
	// an absent field as unconstrained and an empty one as a value the machine
	// must have, so the difference is the difference between "this profile says
	// nothing about it" and "this profile requires it to be blank".
	if _, present := fields["product_version"]; present {
		t.Error("product_version present in the identity fields; absent values " +
			"must be omitted rather than emitted empty")
	}
}

// TestDeviceTreeCarriesNoSerial checks the omission is real.
func TestDeviceTreeCarriesNoSerial(t *testing.T) {
	tree, err := DeviceTree(fixture(t, "arm-laptop"))
	if err != nil {
		t.Fatalf("DeviceTree: %v", err)
	}
	// The fixture carries "SN-ARM-00012345" on the board. Nothing here can
	// reach it: the struct has no field for it and the reader never copies it.
	fields := tree.IdentityFields()
	for key, value := range fields {
		if contains(value, "SN-ARM-00012345") {
			t.Errorf("field %s leaked the device-tree serial: %q", key, value)
		}
	}
}

// TestDeviceTreeAbsentIsNotAnError checks that an x86 machine with no device
// tree — most of them — is a fact rather than a failure.
func TestDeviceTreeAbsentIsNotAnError(t *testing.T) {
	tree, err := DeviceTree(fixture(t, "hybrid-laptop"))
	if err != nil {
		t.Fatalf("DeviceTree returned an error on a machine with no device tree: %v", err)
	}
	if tree.Present {
		t.Error("Present is true on an x86 laptop with no device tree")
	}
	if len(tree.IdentityFields()) != 0 {
		t.Errorf("IdentityFields = %v, want empty", tree.IdentityFields())
	}
}

// TestDeviceTreeOnBoardWithNeither is the machine no probe can name at all. It
// must not crash, and it must not invent an identity.
func TestDeviceTreeOnBoardWithNeither(t *testing.T) {
	tree, err := DeviceTree(fixture(t, "arm-board"))
	if err != nil {
		t.Fatalf("DeviceTree: %v", err)
	}
	if tree.Present {
		t.Error("Present is true on a board with neither SMBIOS nor a device tree")
	}
	if len(tree.IdentityFields()) != 0 {
		t.Errorf("IdentityFields = %v, want empty", tree.IdentityFields())
	}
}

// TestDeviceTreeStringParsing covers the parsing rules directly, including the
// all-padding case that would otherwise yield a slice of one blank string.
func TestDeviceTreeStringParsing(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want []string
	}{
		{"single", []byte("board\x00"), []string{"board"}},
		{"list", []byte("a\x00b\x00c\x00"), []string{"a", "b", "c"}},
		{"padded", []byte("a\x00\x00\x00"), []string{"a"}},
		{"all padding", []byte("\x00\x00\x00"), nil},
		{"empty", []byte{}, nil},
		{"unterminated", []byte("board"), []string{"board"}},
		{"with newline", []byte("board\x00\n"), []string{"board"}},
	}

	for _, tc := range tests {
		got := deviceTreeStrings(tc.raw)
		if len(got) != len(tc.want) {
			t.Errorf("%s: deviceTreeStrings(%q) = %v, want %v",
				tc.name, tc.raw, got, tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
				break
			}
		}
	}
}

func containsNUL(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] == 0 {
			return true
		}
	}
	return false
}
