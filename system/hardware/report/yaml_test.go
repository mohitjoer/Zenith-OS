package report

import "testing"

// TestYAMLStringQuotesAnythingThatWouldChangeType is the single most important
// test in this package.
//
// `product_version: 1.0` parses as a float and then compares unequal to the
// string "1.0" that the machine reports, so a correct profile silently fails
// to select its own machine. This is called out in the design record as the
// most common way that happens.
func TestYAMLStringQuotesAnythingThatWouldChangeType(t *testing.T) {
	quotes := []string{
		"1.0",        // float
		"1",          // int
		"0x10",       // int in another base
		"0o17",       // octal
		"0b1010",     // binary
		"1_000",      // YAML permits underscore separators in numbers
		".inf",       // infinity
		".nan",       // not a number
		"true",       // bool in every YAML version
		"False",      // bool, different case
		"yes",        // bool in YAML 1.1
		"no",         // bool in YAML 1.1, string in 1.2
		"on",         // ditto
		"OFF",        // ditto
		"null",       // null
		"~",          // null
		"",           // empty
		"  padded",   // leading space is not preserved unquoted
		"trailing  ", // trailing space
		"-dash",      // indicator character
		"a: b",       // introduces a mapping
		"a #b",       // introduces a comment
		"key:",       // trailing colon
		"2024-01-01", // timestamp
		"12/03/2024", // date-like
		"[bracket",   // flow indicator
		"*anchor",    // anchor
	}

	for _, value := range quotes {
		got := YAMLString(value)
		if got == value {
			t.Errorf("YAMLString(%q) = %q, unquoted; it would not read back as "+
				"that string", value, got)
		}
		if len(got) < 2 || got[0] != '"' || got[len(got)-1] != '"' {
			t.Errorf("YAMLString(%q) = %q, want double-quoted", value, got)
		}
	}
}

// TestYAMLStringLeavesPlainWordsBare checks the emitter is not simply quoting
// everything, because a profile where every scalar is quoted is one nobody
// reviews comfortably.
func TestYAMLStringLeavesPlainWordsBare(t *testing.T) {
	bare := []string{
		"notebook",
		"ASUSTeK COMPUTER INC.",
		"ASUS TUF Gaming F15 FX507ZC4_FX507ZC4",
		"hwe",
		"intel-microcode",
		"linux-firmware-intel-wireless",
		"qcom,x1e-generic",
		"FX507ZC4",
		"To Be Filled By O.E.M.",
	}

	for _, value := range bare {
		if got := YAMLString(value); got != value {
			t.Errorf("YAMLString(%q) = %q, want it bare", value, got)
		}
	}
}

// TestYAMLQuoteEscapes checks that a value containing a quote or a newline
// cannot break out of the scalar.
func TestYAMLQuoteEscapes(t *testing.T) {
	got := YAMLString(`say "hi"` + "\n")
	if got != `"say \"hi\"\n"` {
		t.Errorf("YAMLString = %s, want escaped", got)
	}
}

// TestSlug checks the id form the schema requires: lowercase and hyphenated, and
// safe as a filename stem.
func TestSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ASUSTeK COMPUTER INC. ASUS TUF Gaming F15", "asustek-computer-inc-asus-tuf-gaming-f15"},
		{"  leading and trailing  ", "leading-and-trailing"},
		{"multiple---hyphens", "multiple-hyphens"},
		{"!!!", ""},
		{"Raspberry Pi 5 Model B", "raspberry-pi-5-model-b"},
	}

	for _, tc := range tests {
		if got := slug(tc.in); got != tc.want {
			t.Errorf("slug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestIsPlaceholder covers firmware filler. A VM whose DMI says
// "To Be Filled By O.E.M." in every field must not produce a profile named
// after the placeholder.
func TestIsPlaceholder(t *testing.T) {
	for _, value := range []string{
		"To Be Filled By O.E.M.",
		"to be filled by o.e.m.",
		"Default string",
		"System Product Name",
		"unknown",
		"none",
	} {
		if !isPlaceholder(value) {
			t.Errorf("isPlaceholder(%q) = false", value)
		}
	}
	for _, value := range []string{"ASUSTeK COMPUTER INC.", "Dell Inc.", "Example Corp"} {
		if isPlaceholder(value) {
			t.Errorf("isPlaceholder(%q) = true; that is a real vendor", value)
		}
	}
}
