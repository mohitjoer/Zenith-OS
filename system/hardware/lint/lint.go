// Package lint enforces the rules a hardware profile must satisfy before it is
// merged.
//
// The rules live in system/hardware/README.md under "what a profile must not
// contain", and they are enforced here rather than asked for in review because
// two of them are security-relevant: a profile is committed to a public
// repository and installed on other people's machines.
//
// This package reads profile YAML without a YAML parser. That is a deliberate
// trade and it is the right one for a linter:
//
//   - The identifying-content rules are substring checks over the whole file.
//     They must fire even on a file this package cannot parse, because a profile
//     that is malformed *and* carries a serial is exactly the case a review
//     skims past.
//   - Adding a YAML dependency to the hardware module — the only module in the
//     repository with no external dependencies — to enforce a known schema is a
//     poor trade.
//
// ponytail: the structured rules read a fixed, flat schema by section and key.
// That is correct for the documented profile shape and for nothing else. A
// profile that uses flow mappings or anchors is silently skipped by those rules
// rather than mis-parsed; upgrade to a real parser if profiles ever do that.
package lint

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"zenith/hardware/match"
)

// Severity distinguishes a refusal from an advice.
type Severity string

const (
	// Error means the profile must not be merged as it stands.
	Error Severity = "error"
	// Warning means the profile is valid but will probably cause trouble later.
	Warning Severity = "warning"
)

// Finding is one problem with one profile, or with a pair of them.
type Finding struct {
	Severity Severity
	Rule     string
	Path     string
	Line     int
	Message  string
}

func (f Finding) String() string {
	location := f.Path
	if f.Line > 0 {
		location = fmt.Sprintf("%s:%d", f.Path, f.Line)
	}
	return fmt.Sprintf("%s: %s: %s: %s", location, f.Severity, f.Rule, f.Message)
}

// Profile is the parsed-enough view of a profile file.
//
// Scalar fields are strings because every one of them is compared against a
// vocabulary, and a linter that has to reason about "this is an int" for a field
// whose YAML type is already wrong is doing the wrong job.
type Profile struct {
	Path string

	Version string
	ID      string
	License string
	Tier    int
	Kernel  string

	// Match is the constraint set, by detection field name. An empty slice means
	// the profile constrains nothing, which is itself an error.
	Match map[string]string

	FirmwarePackages []string
	Microcode        []string

	// Raw is the file as read, for the parser-independent rules.
	Raw []byte

	// lines is Raw split for line numbers.
	lines []string
}

// identifyingPatterns are shapes that must never appear in a committed profile.
//
// These are the same shapes the report redactor's sweep refuses to emit. A
// profile is generated from a report and then edited by a human, so a value can
// survive the step between the two — and a profile carrying a serial is a
// serial in a public repository forever.
var identifyingPatterns = []struct {
	rule    string
	pattern *regexp.Regexp
	message string
}{
	{"profile/mac-address", regexp.MustCompile(`(?i)\b([0-9a-f]{2}:){5}[0-9a-f]{2}\b`),
		"a MAC address is a stable cross-network and cross-machine identifier"},
	{"profile/ipv6-link-local", regexp.MustCompile(`(?i)\bfe80(:[0-9a-f]{0,4}){1,7}\b`),
		"a link-local IPv6 address identifies a machine"},
	{"profile/home-path", regexp.MustCompile(`(/home/[A-Za-z0-9._-]+|/root\b|/Users/[A-Za-z0-9._-]+)`),
		"a home directory path carries a username"},
}

// detectionVocabulary is every field name the probes can produce.
//
// A profile naming anything else can never match, which is a silent failure: the
// installer reads the profile, finds no candidate, and falls back to tier 3 with
// no message about which field was wrong.
var detectionVocabulary = map[string]bool{
	"sys_vendor": true, "product_name": true, "product_version": true,
	"product_family": true, "product_sku": true,
	"board_vendor": true, "board_name": true, "board_version": true,
	"chassis_type": true,
	"bios_vendor":  true, "bios_version": true, "bios_date": true,
	"bios_release": true, "ec_firmware_release": true,
}

// kernelFlavours is the closed set in system/kernel/fleet.yaml.
//
// It is written out rather than read from the fleet because a linter that
// cannot run is a linter that does not run, and three values will not drift
// quietly. A profile naming a fourth installs no kernel at all, which is worse
// than a profile naming a patch version.
var kernelFlavours = map[string]bool{"generic": true, "hwe": true, "oem": true}

// firmwarePackageShape matches the only two firmware package name shapes this
// base layer carries.
var firmwarePackageShape = regexp.MustCompile(`^linux-firmware(-[a-z0-9]+(-[a-z0-9]+)*)?$`)

// idShape is the documented profile id form: lowercase words joined by hyphens.
var idShape = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// patchPinPattern matches a kernel value carrying a patch number.
//
// The rule is "a digit, a hyphen, a digit" anywhere in the value: 6.8.0-31-generic,
// 6.8.0-31, and a value like hwe-6.8 all fail it, while the three flavour names
// pass because none of them contains a digit at all.
var patchPinPattern = regexp.MustCompile(`[0-9]-[0-9]`)

// firmwareFilePattern matches anything that looks like a firmware blob or a path
// to one.
var firmwareFilePattern = regexp.MustCompile(`(?i)\.(ko|fw|hex|bin|elf)\b|/lib/firmware`)

// Lint checks one profile.
func Lint(p Profile) []Finding {
	var findings []Finding
	add := func(severity Severity, rule string, line int, format string, args ...any) {
		findings = append(findings, Finding{
			Severity: severity, Rule: rule, Path: p.Path, Line: line,
			Message: fmt.Sprintf(format, args...),
		})
	}

	// Parser-independent rules first, because they must fire on a file that the
	// structured rules could not read.
	for _, pattern := range identifyingPatterns {
		for i, line := range p.lines {
			if pattern.pattern.MatchString(line) {
				add(Error, pattern.rule, i+1,
					"line contains %s: %s", pattern.rule, pattern.message)
			}
		}
	}

	// id must be the filename stem, because the id is the key used in bug
	// reports and a profile whose id disagrees with its filename is reported
	// under a name nobody can find on disk.
	stem := strings.TrimSuffix(p.Path, ".yaml")
	if stem != "" && p.ID != "" && p.ID != stem {
		add(Error, "profile/id-mismatch", 0,
			"id is %q but the file is named %q.yaml; the id is the key used in "+
				"bug reports and must be findable", p.ID, stem)
	}
	if p.ID != "" && !idShape.MatchString(p.ID) {
		add(Error, "profile/id-shape", 0,
			"id %q is not lowercase words joined by hyphens", p.ID)
	}

	if p.License == "" {
		add(Error, "profile/license-missing", 0,
			"no license field; a profile names base-layer packages and a package "+
				"name is not a licence grant, so the profile has to say which "+
				"terms it carries")
	}

	// The tier rules.
	switch {
	case p.Tier < 1 || p.Tier > 3:
		add(Error, "profile/tier-invalid", 0,
			"tier is %d; the valid values are 1, 2 and 3", p.Tier)
	case p.Tier == 1:
		add(Error, "profile/tier-claimed", 0,
			"tier 1 means a maintainer ran the tier-1 checklist on this machine. "+
				"A submitted profile cannot claim it; open the issue form and say "+
				"so, and a maintainer will run the checklist")
	}

	// The kernel rules, in the order they bite: an unknown flavour installs
	// nothing at all, which is worse than a pin that merely stops matching.
	switch {
	case p.Kernel == "":
		add(Error, "profile/kernel-missing", 0,
			"no kernel flavour; one of generic, hwe or oem must be named")
	case !kernelFlavours[p.Kernel]:
		if patchPinPattern.MatchString(p.Kernel) {
			add(Error, "profile/kernel-patch-pin", 0,
				"kernel is %q; pin the flavour, never a patch version. A profile "+
					"pinning %s stops matching on the next security update",
				p.Kernel, p.Kernel)
		} else {
			add(Error, "profile/kernel-unknown", 0,
				"kernel is %q, which is not a fleet key; the valid set is "+
					"generic, hwe, oem and a flavour that is not a key installs no "+
					"kernel at all", p.Kernel)
		}
	}

	// The match block.
	if len(p.Match) == 0 {
		add(Error, "profile/match-empty", 0,
			"match constrains nothing; a profile with no constraints would match "+
				"every machine, so the matcher rejects it and this profile can "+
				"never be selected")
	}
	for _, field := range sortedKeys(p.Match) {
		value := p.Match[field]
		if !detectionVocabulary[field] {
			add(Error, "profile/match-unknown-field", 0,
				"match field %q is not in the detection vocabulary; the probes "+
					"never produce it, so this constraint can never be satisfied",
				field)
			continue
		}
		if value == "*" {
			continue
		}
		if value == "" {
			add(Error, "profile/match-empty-value", 0,
				"match field %q is empty; an empty value is a value the machine "+
					"must have, not a field the profile says nothing about", field)
		}
	}

	// Firmware.
	for _, pkg := range p.FirmwarePackages {
		if !firmwarePackageShape.MatchString(pkg) {
			add(Error, "profile/firmware-package-shape", 0,
				"firmware package %q is not linux-firmware or "+
					"linux-firmware-<vendor>; this base layer splits firmware by "+
					"vendor and a bare firmware-<vendor> spelling resolves to "+
					"nothing", pkg)
		}
	}
	for i, line := range p.lines {
		if firmwareFilePattern.MatchString(line) {
			add(Error, "profile/firmware-file", i+1,
				"line names something that looks like a firmware blob or path; a "+
					"profile names the package that ships a blob, never the blob")
		}
	}

	return findings
}

// LintDatabase runs the checks that need more than one profile.
//
// It is the check the design record specifically asks for: "a narrow tier 2
// profile and a broad tier 1 profile covering the same machines is the shape
// that most often turns into a conflict once someone adds a third."
//
// Two profiles that can select the same machine are reported even though the
// tie-break table resolves them today. They are a warning rather than an error
// because they are legal — the table is total — but they are the shape that
// stops being legal the moment a third profile arrives and two of them tie on
// tier and specificity while disagreeing on what to install.
func LintDatabase(profiles []Profile) []Finding {
	var findings []Finding

	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			a, b := profiles[i], profiles[j]
			if !overlaps(a, b) {
				continue
			}
			findings = append(findings, Finding{
				Severity: Warning,
				Rule:     "database/overlapping-profiles",
				Path:     a.Path,
				Message: fmt.Sprintf(
					"this profile and %s can select the same machine (%s). "+
						"That is legal today and the tie-break table resolves it, "+
						"but a third profile overlapping the same pair is how this "+
						"database develops a conflict — two profiles tying on tier "+
						"and specificity while disagreeing on what to install. "+
						"Consider narrowing one of them or making the tiers differ.",
					b.Path, describeOverlap(a, b)),
			})
		}
	}

	return findings
}

// overlaps reports whether two profiles' match blocks can be satisfied by one
// machine.
//
// The test is constraint-set containment: every field one profile constrains,
// the other either constrains to the same value or leaves unconstrained. Two
// identical blocks overlap. So do a narrow block and a broad one that does not
// contradict it. Two blocks that contradict each other — the same field, two
// different values — cannot, and are correctly not reported.
func overlaps(a, b Profile) bool {
	if len(a.Match) == 0 || len(b.Match) == 0 {
		return false
	}

	contained := func(narrow, broad Profile) bool {
		for field, value := range narrow.Match {
			if value == "*" {
				// An explicit any-match constrains nothing, so it cannot make
				// the other profile fail to overlap.
				continue
			}
			other, present := broad.Match[field]
			if !present || other == "*" {
				continue
			}
			if other != value {
				return false
			}
		}
		return true
	}

	return contained(a, b) || contained(b, a)
}

// describeOverlap renders why two profiles overlap, for the warning message.
func describeOverlap(a, b Profile) string {
	shared := make([]string, 0, len(a.Match))
	for field, value := range a.Match {
		if value == "*" {
			continue
		}
		if other, present := b.Match[field]; present && other != "*" && other == value {
			shared = append(shared, field)
		}
	}
	if len(shared) == 0 {
		// One block is a strict superset of the other's constraints.
		return "one match block is contained in the other"
	}
	sort.Strings(shared)
	return "shared on " + strings.Join(shared, ", ")
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// ToMatchProfiles projects linted profiles onto the shape matching consumes.
//
// The two packages both care about the same file and neither should depend on
// the other's internals: lint answers "is this profile well formed", match
// answers "which profile claims this machine". The projection lives here, next
// to the reader that produced the source, so adding a schema field means editing
// one function rather than two packages.
func ToMatchProfiles(profiles []Profile) []match.Profile {
	converted := make([]match.Profile, 0, len(profiles))
	for _, profile := range profiles {
		converted = append(converted, match.Profile{
			ID:   profile.ID,
			Tier: match.Tier(profile.Tier),
			Match: match.Match{
				SysVendor:      profile.Match["sys_vendor"],
				ProductName:    profile.Match["product_name"],
				ProductVersion: profile.Match["product_version"],
				ProductFamily:  profile.Match["product_family"],
				ProductSKU:     profile.Match["product_sku"],
				BoardVendor:    profile.Match["board_vendor"],
				BoardName:      profile.Match["board_name"],
				BoardVersion:   profile.Match["board_version"],
				ChassisType:    profile.Match["chassis_type"],
				BIOSVendor:     profile.Match["bios_vendor"],
				BIOSVersion:    profile.Match["bios_version"],
				BIOSDate:       profile.Match["bios_date"],
			},
			Kernel: profile.Kernel,
			Firmware: match.Firmware{
				Packages:  profile.FirmwarePackages,
				Microcode: profile.Microcode,
			},
		})
	}
	return converted
}
