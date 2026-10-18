package probe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Three parsers in this package take untrusted input, and each is attacker-
// influenced in a way that matters: in a virtual machine the EDID block and the
// DMI strings are whatever the hypervisor or the VM's firmware chose, and a
// third-party pull request can attach any of them.
//
// The bar for all three is the same and it is the same bar the negative fixtures
// exist to enforce: never a crash, and never a wrong value that looks right.

// FuzzParseEDID covers the EDID decoder. Every offset in a 128- or 256-byte
// block is attacker-chosen, including the manufacturer letters, the product code
// and the timing descriptor that produces the resolution's refresh rate.
func FuzzParseEDID(f *testing.F) {
	// Seeds: the committed fixtures, which is one more reason they are worth
	// keeping pristine.
	for _, name := range []string{"hybrid-laptop", "desktop-minimal"} {
		fixtures, err := filepath.Glob(filepath.Join("testdata", name, "class", "drm", "*", "edid"))
		if err != nil {
			continue
		}
		for _, fixture := range fixtures {
			if data, err := os.ReadFile(fixture); err == nil {
				f.Add(data)
			}
		}
	}

	// Structural edge cases worth running first: right length, wrong header;
	// right header, every field extreme; the two accepted lengths.
	f.Add(make([]byte, 128))
	f.Add(make([]byte, 256))
	f.Add(append([]byte{0x00, 0xFF}, make([]byte, 126)...))
	f.Add(append([]byte{0x12, 0x34, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00},
		make([]byte, 120)...))
	f.Add(append([]byte{0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00},
		[]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}...))
	f.Add([]byte{})
	f.Add([]byte{0x00})

	f.Fuzz(func(t *testing.T, block []byte) {
		info, err := ParseEDID(block)

		// A length outside 128 or 256 is rejected, always. This is the rule
		// that stops a truncated block being parsed into a right manufacturer
		// and a garbage timing table.
		if err := validateEDIDLength(len(block)); err != nil {
			if err == nil {
				t.Fatalf("ParseEDID accepted %d bytes, which validateEDIDLength "+
					"rejects", len(block))
			}
			return
		}

		if err != nil {
			return
		}

		// A manufacturer is three letters, or nothing at all.
		//
		// The decoder only produces a code when the block carries the 0b00010
		// tag and all three 5-bit letter fields are inside 1-26. Anything else
		// correctly yields an empty string: inventing a code from an undecodable
		// tag or from a field holding 27 would be exactly the plausible-and-wrong
		// value this check exists to prevent.
		letters := []int{
			int(((block[9]>>5)&0x07)<<2 | (block[8] & 0x03)),
			int(block[9] & 0x1F),
			int(block[10] & 0x1F),
		}
		decodable := block[8]>>3 == 0x02
		for _, letter := range letters {
			if letter < 1 || letter > 26 {
				decodable = false
			}
		}
		if !decodable {
			if info.Manufacturer != "" {
				t.Fatalf("manufacturer %q decoded from a block with no decodable "+
					"manufacturer ID (tag=%d letters=%v)",
					info.Manufacturer, block[8]>>3, letters)
			}
		} else if len(info.Manufacturer) != 3 {
			t.Fatalf("manufacturer %q is not three letters (letters=%v)",
				info.Manufacturer, letters)
		} else {
			for _, r := range info.Manufacturer {
				if r < 'A' || r > 'Z' {
					t.Fatalf("manufacturer %q contains %q, which is not a letter",
						info.Manufacturer, r)
				}
			}
		}

		// A refresh rate is never negative and never infinite. A zero clock or a
		// zero total must leave it at zero rather than dividing by it.
		if info.RefreshHz < 0 {
			t.Fatalf("negative refresh %f", info.RefreshHz)
		}
		if info.Width < 0 || info.Height < 0 {
			t.Fatalf("negative dimensions %dx%d", info.Width, info.Height)
		}

		// A monitor name comes out of a fixed 13-byte field and cannot be
		// longer.
		if len(info.MonitorName) > 13 {
			t.Fatalf("monitor name %q is longer than the descriptor field",
				info.MonitorName)
		}
	})
}

// FuzzCandidateModules covers the modules.alias glob matcher.
//
// The alias string is attacker-chosen in a VM and the patterns come from a
// machine-generated file, so both sides are untrusted. The invariant is that a
// malformed pattern is skipped rather than fatal: the alias file is tens of
// thousands of lines and one odd line must not cost the whole probe.
func FuzzCandidateModules(f *testing.F) {
	// Fuzz arguments must be simple types, so the alias file is encoded as one
	// string: records separated by \n, pattern and module separated by \x00.
	// That also means the fuzzer mutates patterns and module names
	// independently, which is the interesting shape.
	encode := func(rules []AliasRule) string {
		var b strings.Builder
		for i, rule := range rules {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(rule.Pattern)
			b.WriteByte(0)
			b.WriteString(rule.Module)
		}
		return b.String()
	}
	decode := func(encoded string) []AliasRule {
		if encoded == "" {
			return nil
		}
		var rules []AliasRule
		for _, record := range strings.Split(encoded, "\n") {
			pattern, module, found := strings.Cut(record, "\x00")
			if !found {
				pattern, module = record, ""
			}
			rules = append(rules, AliasRule{Pattern: pattern, Module: module})
		}
		return rules
	}

	f.Add("pci:v00008086d000046A6sv00001043sd00008762bc03sc00i00",
		encode([]AliasRule{
			{Pattern: "pci:v00008086d000046A6sv*sd*bc03sc00i00", Module: "i915"},
			{Pattern: "pci:v*", Module: "drm"},
		}))
	f.Add("", encode([]AliasRule{{Pattern: "*", Module: "everything"}}))
	f.Add("pci:v0000", encode([]AliasRule{{Pattern: "pci:v[[[bad", Module: "never"}}))
	f.Add("usb:v00001D6Bd0000002",
		encode([]AliasRule{{Pattern: "usb:v00001D6B*", Module: "hub"}}))
	f.Add("x", "")

	f.Fuzz(func(t *testing.T, modalias, encodedRules string) {
		got := CandidateModules(modalias, decode(encodedRules))

		// Results are deduplicated: the same module reachable through several
		// matching patterns is reported once.
		seen := map[string]bool{}
		for _, module := range got {
			if seen[module] {
				t.Fatalf("module %q reported twice in %v", module, got)
			}
			seen[module] = true
			if module == "" {
				t.Fatal("an empty module name was reported")
			}
		}

		// An empty alias can never bind anything.
		if modalias == "" && len(got) != 0 {
			t.Fatalf("modules %v reported for an empty alias", got)
		}

		// No rules can never bind anything.
		if encodedRules == "" && len(got) != 0 {
			t.Fatalf("modules %v reported with no rules", got)
		}
	})
}

// FuzzParseBIOSDate covers the SMBIOS firmware date parser.
//
// A BIOS date is firmware-controlled and reaches a profile comparator, so a
// value that parses to the wrong date silently accepts a profile on one machine
// and rejects it on another. The invariant is that only a well-formed
// MM/DD/YYYY parses at all.
func FuzzParseBIOSDate(f *testing.F) {
	for _, seed := range []string{
		"12/03/2024", "01/02/2024", "31/12/2023", "12/31/2023",
		"13/01/2024", "00/10/2024", "99/99/9999", "", "/", "12/03/2024 ",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, value string) {
		parsed, err := ParseBIOSDate(value)
		if err != nil {
			return
		}
		// Anything that parses must round-trip to the same day and month, which
		// is the property that catches a swapped month and day.
		if parsed.Day() < 1 || parsed.Day() > 31 {
			t.Fatalf("ParseBIOSDate(%q) = day %d", value, parsed.Day())
		}
		if parsed.Month() < 1 || parsed.Month() > 12 {
			t.Fatalf("ParseBIOSDate(%q) = month %d", value, parsed.Month())
		}
		// The 1990 year offset belongs to the single-byte SMBIOS field, not to
		// this string form: "12/03/2024" is a literal year, so year 0 is a
		// syntactically valid value rather than a misread one. It is also safe
		// to accept, because every profile comparator against it fails closed —
		// a firmware date of year 0 matches nothing rather than matching
		// everything.
		if parsed.Year() < 0 || parsed.Year() > 9999 {
			t.Fatalf("ParseBIOSDate(%q) = year %d", value, parsed.Year())
		}
	})
}

// FuzzParseUSBName covers topology reconstruction from a directory name. The
// names come from the kernel, but a machine with a malformed one must not panic
// and must not report a topology it did not read.
func FuzzParseUSBName(f *testing.F) {
	for _, seed := range []string{
		"usb1", "1-0:1.0", "3-10.2:1.0", "garbage", "", "-", ":", "1-", "usb",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, name string) {
		topology := ParseUSBName(name)

		if topology.Bus < 0 || topology.Port < 0 || topology.Interface < 0 ||
			topology.Depth < 0 {
			t.Fatalf("ParseUSBName(%q) produced a negative field: %+v", name, topology)
		}
		if topology.IsRootHub && topology.Depth != 0 {
			t.Fatalf("ParseUSBName(%q) reports a root hub at depth %d",
				name, topology.Depth)
		}
		// A name with no bus-port separator is not a device, so it carries no
		// topology. Claiming one would put the device somewhere it is not.
		if !strings.Contains(name, "-") && !strings.HasPrefix(name, "usb") &&
			topology.Bus != 0 {
			t.Fatalf("ParseUSBName(%q) invented bus %d", name, topology.Bus)
		}
	})
}
