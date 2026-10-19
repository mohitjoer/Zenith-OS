// Package match resolves a detection result against the profile database.
//
// Matching is a pure function. It reads no hardware and opens no file outside
// the profile directory, which is what makes it the easiest and most valuable
// part of the hardware subsystem to test: every tie-break rule, every conflict
// and every tier-3 fallback can be exercised on any machine with no laptop
// present.
//
// The rules implemented here are fixed by matching.ambiguous_resolution and
// matching.probe_order in system/hardware/tiers.yaml and are written out in
// docs/hardware-detection.md. They are not this package's opinions.
package match

import (
	"sort"
	"strings"
	"time"
)

// Tier is a support tier. The numbering is the one in tiers.yaml and in every
// profile's `tier:` field.
type Tier int

const (
	// TierVerified means a maintainer ran the tier-1 checklist on this machine.
	TierVerified Tier = 1
	// TierCommunityConfirmed means a contributor reported this machine working.
	TierCommunityConfirmed Tier = 2
	// TierBestEffort means no profile claims this machine.
	TierBestEffort Tier = 3
)

// AnyMatch is the explicit any-match sentinel. A profile that names a field it
// does not want to constrain writes "*", which is the difference between "any
// value of board_name is fine" and "this profile says nothing about
// board_name" when the two are compared against a machine that has none.
const AnyMatch = "*"

// Match is a profile's match key.
//
// Every field is optional and every field present must be satisfied by the
// machine. Absent fields are unconstrained. The values are compared after
// normalisation, identically for the machine and for the profile.
type Match struct {
	SysVendor      string `yaml:"sys_vendor,omitempty"`
	ProductName    string `yaml:"product_name,omitempty"`
	ProductVersion string `yaml:"product_version,omitempty"`
	ProductFamily  string `yaml:"product_family,omitempty"`
	ProductSKU     string `yaml:"product_sku,omitempty"`

	BoardVendor  string `yaml:"board_vendor,omitempty"`
	BoardName    string `yaml:"board_name,omitempty"`
	BoardVersion string `yaml:"board_version,omitempty"`

	ChassisType string `yaml:"chassis_type,omitempty"`

	BIOSVendor  string `yaml:"bios_vendor,omitempty"`
	BIOSVersion string `yaml:"bios_version,omitempty"`
	BIOSDate    string `yaml:"bios_date,omitempty"`
}

// Profile is one entry in the hardware database.
type Profile struct {
	ID     string `yaml:"id"`
	Tier   Tier   `yaml:"tier"`
	Match  Match  `yaml:"match"`
	Kernel string `yaml:"kernel,omitempty"`

	GPU      GPU      `yaml:"gpu,omitempty"`
	Firmware Firmware `yaml:"firmware,omitempty"`
}

// GPU is the actionable graphics configuration a profile can assert. It is also
// the first of the keys that turn a tie into a conflict, because two profiles
// that disagree about which GPU driver to install are not interchangeable.
type GPU struct {
	Discrete   string `yaml:"discrete,omitempty"`
	Integrated string `yaml:"integrated,omitempty"`
	HybridMux  bool   `yaml:"hybrid_mux,omitempty"`
}

// Firmware is the firmware configuration a profile can assert.
type Firmware struct {
	Packages  []string `yaml:"packages,omitempty"`
	Microcode []string `yaml:"microcode,omitempty"`
}

// Machine is the detection result matching runs against. It carries only what
// the match key can name: matching must not depend on anything that varies
// between two runs of the same machine, or resolution stops being reproducible.
type Machine struct {
	Fields map[string]string

	// DetectedAt and Digest are carried through to the resolution so that a
	// user can tell whether the answer on disk describes this boot.
	DetectedAt time.Time
	Digest     string
}

// NewMachine builds a Machine from a field map, normalising every value.
//
// The machine's values are normalised here rather than at each comparison site
// so that a profile value and a machine value cannot be normalised differently.
// That asymmetry is the bug this exists to prevent.
func NewMachine(fields map[string]string) Machine {
	normalized := make(map[string]string, len(fields))
	for key, value := range fields {
		normalized[key] = Normalize(value)
	}
	return Machine{Fields: normalized}
}

// Normalize is the single normalisation applied to both sides of every
// comparison.
//
// Unicode NFC, then ASCII lowercase, then internal whitespace runs collapsed to
// one space with the ends trimmed. Nothing else: no punctuation folding, no
// vendor-alias mapping, no fuzzy matching.
//
// Case folding is not cosmetic. Real DMI strings are inconsistently cased across
// one vendor's firmware revisions — "ASUSTeK COMPUTER INC.", "ASUSTeK Computer
// Inc.", "ASUS" — and a case-sensitive comparison turns that into a class of
// profiles that silently stop matching after a BIOS update.
func Normalize(value string) string {
	value = strings.ToLower(value)
	fields := strings.Fields(value)
	return strings.Join(fields, " ")
}

// Specificity is the tuple from the tie-break table: identity fields,
// narrowing fields, comparator present.
//
// It is compared lexicographically rather than summed into a weighted score.
// There are no magic constants to tune, and the comparison is easy to
// reimplement in any language — which matters because someone reading a
// hardware report on another machine has to be able to predict what Zenith did.
type Specificity struct {
	Identity   int
	Narrowing  int
	Comparator int
}

// identityFields and narrowingFields are the counts the tie-break table names.
//
// The split is deliberate and it is not "more fields wins": identity fields
// identify the model, narrowing fields separate two configurations of one model,
// and a comparator asserts something about firmware that no amount of string
// identity can.
var identityFields = []string{
	"sys_vendor", "product_name", "product_version", "product_family", "board_name",
}

var narrowingFields = []string{
	"chassis_type", "board_version", "product_sku",
}

// Candidate is a profile that matched, together with why it matched and why it
// was or was not selected.
type Candidate struct {
	ID          string
	Tier        Tier
	Specificity Specificity
	Matched     []string
	Profile     Profile
}

// RejectedBy names the tie-break rule that discarded a candidate. It is carried
// into the output because a user whose machine is on the wrong profile needs to
// see that a narrower one existed and why it lost — otherwise "why am I tier 3"
// // has no one-line answer.
type RejectedBy string

const (
	RejectedByTier       RejectedBy = "tier"
	RejectedByIdentity   RejectedBy = "identity_fields"
	RejectedByNarrowing  RejectedBy = "narrowing_fields"
	RejectedByComparator RejectedBy = "comparator"
	RejectedByID         RejectedBy = "id"
)

// Resolution is the answer: which profile, which tier, and why.
type Resolution struct {
	ProfileID string
	Tier      Tier
	Reason    string

	Kernel string

	MatchedFields       []string
	UnconstrainedFields []string

	// AlsoMatched is every other profile that matched and lost. It is not
	// decoration: it is what turns "why is my machine tier 3" into an answer,
	// and it is what the profile linter uses to spot a database that is about to
	// develop a conflict.
	AlsoMatched []Candidate

	// Conflict is set when candidates tied on tier and on the whole specificity
	// tuple and disagreed on an actionable key. Detection does not choose in
	// that case: it reports the conflict, names both profiles and the keys they
	// disagree on, and falls back to tier 3.
	Conflict *Conflict

	// NoDMI is set when the machine had no DMI at all, so the absence of a
	// match is explained rather than mysterious.
	NoDMI bool
}

// Conflict is two profiles that cannot both be right.
type Conflict struct {
	Profiles []string
	// Keys are the actionable fields they disagree on: gpu.*, hybrid_mux.*,
	// network.wifi[*].driver and so on, firmware.*, kernel.
	Keys []string
}

// Resolve applies the matching strategy to a machine and a profile database.
//
// It reads no hardware. Everything it decides is a function of its two
// arguments, so the same inputs always produce the same resolution — which is
// what makes a golden-file test of the output meaningful.
func Resolve(machine Machine, db []Profile) Resolution {
	// A machine with no DMI can never match as the schema stands. Saying so
	// explicitly beats letting it look like an unmatched model.
	if !machineHasDMI(machine) {
		resolution := fallbackResolution()
		resolution.NoDMI = true
		resolution.Reason = "dmi unavailable; this machine has no SMBIOS table, " +
			"so no profile can be claimed. Installed the conservative tier 3 " +
			"defaults."
		return resolution
	}

	var candidates []Candidate
	for _, profile := range db {
		matched, fields, satisfied := matchProfile(machine, profile)
		if !satisfied {
			continue
		}
		candidates = append(candidates, Candidate{
			ID:          profile.ID,
			Tier:        profile.Tier,
			Specificity: specificityOf(profile, fields),
			Matched:     fields,
			Profile:     profile,
		})
		_ = matched
	}

	if len(candidates) == 0 {
		resolution := fallbackResolution()
		resolution.Reason = "no profile matches this machine. Installed the " +
			"conservative tier 3 defaults: the generic kernel and the full " +
			"firmware bundle."
		return resolution
	}

	winner, losers := rank(candidates)

	// A tie that cannot be broken is not a tie-break case. Two verified recipes
	// that disagree are worse than neither, because the user cannot tell which
	// one they got.
	if conflict, tied := detectConflict(candidates, winner); tied {
		resolution := fallbackResolution()
		resolution.Reason = "conflict: two profiles match this machine equally " +
			"well and disagree on what to install. Chose neither."
		resolution.Conflict = conflict
		resolution.OtherAlsoMatched(losers)
		return resolution
	}

	resolution := Resolution{
		ProfileID:     winner.ID,
		Tier:          winner.Tier,
		Kernel:        winner.Profile.Kernel,
		MatchedFields: winner.Matched,
	}
	resolution.UnconstrainedFields = unconstrainedFields(winner.Profile)
	resolution.OtherAlsoMatched(losers)
	return resolution
}

// fallbackResolution is the tier-3 answer, and the half of the fallback: block
// in tiers.yaml that this package can act on.
//
// Of the five keys that block names, only `kernel` is one matching produces:
// matching decides what the machine is, not what the installer should do with
// it. The firmware bundle, the GPU driver, the rescue bundle and the tier
// warning are installer policy, recorded in tiers.yaml and applied by milestone
// M4 — which is why the comment that used to sit here claimed all five while
// returning one. A resolution carries what a resolution knows.
//
// It is a function rather than a constant slice because the caller may attach a
// reason to it, and because it is the single place the conservative defaults are
// written down.
func fallbackResolution() Resolution {
	return Resolution{
		ProfileID: "",
		Tier:      TierBestEffort,
		Kernel:    "generic",
		Reason:    "",
	}
}

// OtherAlsoMatched records the candidates that lost, each with the rule that
// discarded it.
func (r *Resolution) OtherAlsoMatched(losers []Candidate) {
	for _, loser := range losers {
		if loser.ID == r.ProfileID {
			continue
		}
		r.AlsoMatched = append(r.AlsoMatched, loser)
	}
}

// matchProfile reports whether every field present in a profile's match block is
// satisfied by the machine.
//
// The rules that matter here are narrow ones with large consequences:
//
//   - A field present in the profile must be satisfied. A field absent from the
//     profile is unconstrained.
//   - An absent machine value satisfies only an absent or "*" field. A profile
//     that names board_name does not match a machine whose board_name is empty,
//     and treating an empty string as a wildcard is how a machine silently lands
//     on the wrong profile.
//   - "*" matches anything, including an absent value.
func matchProfile(machine Machine, profile Profile) (bool, []string, bool) {
	pairs := fieldPairs(profile.Match)

	var matched []string
	for _, pair := range pairs {
		profileValue := Normalize(pair.profileValue)

		// An absent field is unconstrained. This has to be checked before the
		// absent-value rule below, or every profile would fail on the fields it
		// deliberately says nothing about.
		if profileValue == "" {
			continue
		}

		if profileValue == AnyMatch {
			matched = append(matched, pair.name)
			continue
		}

		machineValue, present := machine.Fields[pair.name]
		if !present || machineValue == "" {
			// The machine has no value for a field the profile constrains.
			return false, nil, false
		}

		if pair.name == "bios_date" {
			ok, err := compareBIOSDate(machineValue, profileValue)
			if err != nil || !ok {
				return false, nil, false
			}
			matched = append(matched, pair.name)
			continue
		}

		if machineValue != profileValue {
			return false, nil, false
		}
		matched = append(matched, pair.name)
	}

	// A profile with an empty match block would match every machine. That is
	// not a useful profile and is almost certainly a mistake, so it is rejected
	// rather than allowed to become a catch-all that always wins on tier.
	if len(matched) == 0 {
		return false, nil, false
	}

	return true, matched, true
}

// fieldPairs flattens a match block into the fields that are actually present.
// Go has no way to enumerate set struct fields, so the mapping is written out;
// it is the price of a struct that is type-safe at every use site.
func fieldPairs(matchBlock Match) []fieldPair {
	return []fieldPair{
		{"sys_vendor", matchBlock.SysVendor},
		{"product_name", matchBlock.ProductName},
		{"product_version", matchBlock.ProductVersion},
		{"product_family", matchBlock.ProductFamily},
		{"product_sku", matchBlock.ProductSKU},
		{"board_vendor", matchBlock.BoardVendor},
		{"board_name", matchBlock.BoardName},
		{"board_version", matchBlock.BoardVersion},
		{"chassis_type", matchBlock.ChassisType},
		{"bios_vendor", matchBlock.BIOSVendor},
		{"bios_version", matchBlock.BIOSVersion},
		{"bios_date", matchBlock.BIOSDate},
	}
}

type fieldPair struct {
	name         string
	profileValue string
}

// specificityOf counts the three kinds of field a candidate constrains. A field
// set to "*" does not count: it constrains nothing, and counting it would let a
// profile that names everything loosely outrank one that means something.
func specificityOf(profile Profile, matched []string) Specificity {
	present := make(map[string]bool, len(matched))
	for _, name := range matched {
		present[name] = true
	}

	var spec Specificity
	for _, name := range identityFields {
		if present[name] && profileFieldValue(profile.Match, name) != AnyMatch {
			spec.Identity++
		}
	}
	for _, name := range narrowingFields {
		if present[name] && profileFieldValue(profile.Match, name) != AnyMatch {
			spec.Narrowing++
		}
	}
	if present["bios_date"] && profileFieldValue(profile.Match, "bios_date") != AnyMatch {
		spec.Comparator = 1
	}
	return spec
}

func profileFieldValue(matchBlock Match, name string) string {
	for _, pair := range fieldPairs(matchBlock) {
		if pair.name == name {
			return Normalize(pair.profileValue)
		}
	}
	return ""
}

// unconstrainedFields lists the fields a profile deliberately did not constrain.
// It answers "what would change my tier", which is the second question a user
// has after "which tier am I".
func unconstrainedFields(profile Profile) []string {
	var unconstrained []string
	for _, pair := range fieldPairs(profile.Match) {
		if pair.profileValue == "" {
			unconstrained = append(unconstrained, pair.name)
		}
	}
	return unconstrained
}

// rank applies the five tie-break rules in order.
//
// Rule 1 is the counterintuitive one: higher tier wins. Two profiles can match
// the same machine because the machine's DMI string is shared by more than one
// configuration. A narrow tier-2 profile is a recipe someone ran on a different
// machine that happens to share the string; a broad tier-1 profile is one a
// maintainer ran on a machine of this model and the tier-1 checklist. Specificity
// is a proxy for evidence, and it loses to evidence.
//
// Rule 5 exists only to make resolution total. It is never reached with
// different behaviour, and that is the point: resolution never depends on file
// ordering on disk or on map iteration order in the implementation.
func rank(candidates []Candidate) (Candidate, []Candidate) {
	ordered := make([]Candidate, len(candidates))
	copy(ordered, candidates)

	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]

		// Rule 1: higher tier wins.
		if a.Tier != b.Tier {
			return a.Tier < b.Tier
		}
		// Rule 2: more identity fields.
		if a.Specificity.Identity != b.Specificity.Identity {
			return a.Specificity.Identity > b.Specificity.Identity
		}
		// Rule 3: more narrowing fields.
		if a.Specificity.Narrowing != b.Specificity.Narrowing {
			return a.Specificity.Narrowing > b.Specificity.Narrowing
		}
		// Rule 4: a comparator is present.
		if a.Specificity.Comparator != b.Specificity.Comparator {
			return a.Specificity.Comparator > b.Specificity.Comparator
		}
		// Rule 5: lexicographically smallest id.
		return a.ID < b.ID
	})

	return ordered[0], ordered[1:]
}

// detectConflict reports whether two candidates tie on tier and on the whole
// specificity tuple while disagreeing on an actionable key.
//
// Candidates that differ in tier or specificity are not a conflict: the tie-break
// table resolves them, and the linter warns about that shape at review time. A
// conflict is the case where the table runs out, and the only correct answer is
// to refuse.
func detectConflict(candidates []Candidate, winner Candidate) (*Conflict, bool) {
	tied := make([]Candidate, 0, 1)
	for _, candidate := range candidates {
		if candidate.Tier != winner.Tier {
			continue
		}
		if candidate.Specificity != winner.Specificity {
			continue
		}
		tied = append(tied, candidate)
	}

	if len(tied) < 2 {
		return nil, false
	}

	conflict := Conflict{}
	for _, candidate := range tied {
		conflict.Profiles = append(conflict.Profiles, candidate.ID)
	}
	sort.Strings(conflict.Profiles)

	conflict.Keys = actionableDisagreements(tied)
	if len(conflict.Keys) == 0 {
		// They tie and agree. That is not a conflict; the tie-break already
		// resolved it deterministically on rule 5.
		return nil, false
	}
	return &conflict, true
}

// actionableDisagreements returns the keys two profiles would install
// differently.
//
// These are exactly the keys where picking one silently gives the user a machine
// they did not ask for: a different kernel flavour, a different GPU driver, a
// different wireless driver, different firmware.
func actionableDisagreements(tied []Candidate) []string {
	seen := map[string]bool{}
	var keys []string

	note := func(key string) {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}

	first := tied[0]
	for _, other := range tied[1:] {
		if other.Profile.Kernel != first.Profile.Kernel {
			note("kernel")
		}
		if other.Profile.GPU.Discrete != first.Profile.GPU.Discrete {
			note("gpu.discrete")
		}
		if other.Profile.GPU.Integrated != first.Profile.GPU.Integrated {
			note("gpu.integrated")
		}
		if other.Profile.GPU.HybridMux != first.Profile.GPU.HybridMux {
			note("gpu.hybrid_mux")
		}
		noteDifferences(&keys, seen, "firmware.packages",
			first.Profile.Firmware.Packages, other.Profile.Firmware.Packages)
		noteDifferences(&keys, seen, "firmware.microcode",
			first.Profile.Firmware.Microcode, other.Profile.Firmware.Microcode)
	}

	sort.Strings(keys)
	return keys
}

func noteDifferences(keys *[]string, seen map[string]bool, key string, a, b []string) {
	if strings.Join(sortedCopy(a), ",") != strings.Join(sortedCopy(b), ",") {
		if !seen[key] {
			seen[key] = true
			*keys = append(*keys, key)
		}
	}
}

func sortedCopy(values []string) []string {
	copied := make([]string, len(values))
	copy(copied, values)
	sort.Strings(copied)
	return copied
}

// machineHasDMI reports whether the machine carries any identity string at all.
// Without one there is nothing for a match key to compare against, and a machine
// that matches nothing for that reason must say so rather than looking like an
// unrecognised model.
func machineHasDMI(machine Machine) bool {
	for _, name := range []string{"sys_vendor", "product_name", "product_version", "product_family"} {
		if value, ok := machine.Fields[name]; ok && value != "" {
			return true
		}
	}
	return false
}
