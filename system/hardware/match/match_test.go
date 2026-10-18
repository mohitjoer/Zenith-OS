package match

import (
	"strings"
	"testing"
)

// f15 is the reference machine from the design record, as the match key sees it:
// normalised at construction so the tests assert matching rather than spelling.
func f15() Machine {
	return NewMachine(map[string]string{
		"sys_vendor":      "ASUSTeK COMPUTER INC.",
		"product_name":    "ASUS TUF Gaming F15 FX507ZC4_FX507ZC4",
		"product_version": "1.0",
		"product_family":  "ASUS TUF Gaming F15",
		"product_sku":     "FX507ZC4",
		"board_vendor":    "ASUSTeK COMPUTER INC.",
		"board_name":      "FX507ZC4",
		"board_version":   "1.0",
		"chassis_type":    "notebook",
		"bios_version":    "3303",
		"bios_date":       "12/03/2024",
	})
}

// TestNormalize covers the single normalisation applied to both sides.
//
// Case folding is not cosmetic: the same vendor's firmware reports its name
// three different ways across revisions, and a case-sensitive comparison turns
// that into profiles that silently stop matching after a BIOS update.
func TestNormalize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ASUSTeK COMPUTER INC.", "asustek computer inc."},
		{"ASUSTeK Computer Inc.", "asustek computer inc."},
		{"ASUS", "asus"},
		{"  spaced   out  ", "spaced out"},
		{"tabs\tand\nnewlines", "tabs and newlines"},
		{"", ""},
		// Normalisation is deliberately not punctuation folding.
		{"ASUS-TUF", "asus-tuf"},
		{"To Be Filled By O.E.M.", "to be filled by o.e.m."},
	}
	for _, tc := range tests {
		if got := Normalize(tc.in); got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestMatchIsCaseInsensitive is the regression the whole normalisation exists to
// prevent: three firmware revisions, one profile, still matching.
func TestMatchIsCaseInsensitive(t *testing.T) {
	for _, spelling := range []string{
		"ASUSTeK COMPUTER INC.",
		"ASUSTeK Computer Inc.",
		"Asustek Computer Inc.",
	} {
		machine := NewMachine(map[string]string{
			"sys_vendor":   spelling,
			"product_name": "ASUS TUF Gaming F15",
		})
		profile := Profile{
			ID:   "asus-tuf-gaming-f15",
			Tier: TierVerified,
			Match: Match{
				SysVendor:   "ASUSTeK COMPUTER INC.",
				ProductName: "ASUS TUF Gaming F15",
			},
		}

		resolution := Resolve(machine, []Profile{profile})
		if resolution.ProfileID != profile.ID {
			t.Errorf("sys_vendor spelled %q did not match the profile: %+v",
				spelling, resolution)
		}
	}
}

// TestAbsentValueDoesNotSatisfyANamedField is the rule that keeps a machine from
// landing on a profile that never claimed it.
func TestAbsentValueDoesNotSatisfyANamedField(t *testing.T) {
	machine := NewMachine(map[string]string{
		"sys_vendor":   "Example Corp",
		"product_name": "Example Pro 16",
		// board_name absent entirely.
	})

	profile := Profile{
		ID:   "naming-a-field-the-machine-lacks",
		Tier: TierCommunityConfirmed,
		Match: Match{
			SysVendor: "Example Corp",
			BoardName: "EX16-1A",
		},
	}

	resolution := Resolve(machine, []Profile{profile})
	if resolution.ProfileID != "" {
		t.Errorf("a profile naming board_name matched a machine that has none: %+v",
			resolution)
	}
	if resolution.Tier != TierBestEffort {
		t.Errorf("tier = %d, want 3", resolution.Tier)
	}
}

// TestAnyMatchSatisfiesAnything checks the explicit any-match sentinel.
func TestAnyMatchSatisfiesAnything(t *testing.T) {
	machine := NewMachine(map[string]string{
		"sys_vendor":   "Example Corp",
		"product_name": "Example Pro 16",
	})

	profile := Profile{
		ID:   "wildcard-board",
		Tier: TierCommunityConfirmed,
		Match: Match{
			SysVendor: "Example Corp",
			BoardName: AnyMatch,
		},
	}

	resolution := Resolve(machine, []Profile{profile})
	if resolution.ProfileID != profile.ID {
		t.Errorf("a wildcard field did not match an absent value: %+v", resolution)
	}
}

// TestProfileWithNoMatchBlockMatchesNothing checks that an empty match block
// does not become a catch-all.
func TestProfileWithNoMatchBlockMatchesNothing(t *testing.T) {
	machine := f15()
	resolution := Resolve(machine, []Profile{{ID: "empty", Tier: TierVerified}})
	if resolution.ProfileID != "" {
		t.Errorf("a profile with an empty match block claimed the machine: %+v",
			resolution)
	}
}

// TestRule1HigherTierWins is the counterintuitive rule, stated explicitly
// because it is the one a reader will expect to be the other way round.
//
// A narrow tier-2 profile is a recipe someone ran on a different machine that
// happens to share a DMI string. A broad tier-1 profile is one a maintainer ran
// on a machine of this model. Specificity is a proxy for evidence and loses to
// evidence.
func TestRule1HigherTierWins(t *testing.T) {
	machine := f15()

	db := []Profile{
		{
			// Narrower: three identity fields, one narrowing field.
			ID:   "narrow-tier-2",
			Tier: TierCommunityConfirmed,
			Match: Match{
				SysVendor:     "ASUSTeK COMPUTER INC.",
				ProductName:   "ASUS TUF Gaming F15 FX507ZC4_FX507ZC4",
				ProductFamily: "ASUS TUF Gaming F15",
				ChassisType:   "notebook",
				BoardVersion:  "1.0",
				ProductSKU:    "FX507ZC4",
				BIOSVersion:   "3303",
			},
			Kernel: "generic",
		},
		{
			// Broader: one identity field, but verified.
			ID:   "broad-tier-1",
			Tier: TierVerified,
			Match: Match{
				SysVendor: "ASUSTeK COMPUTER INC.",
			},
			Kernel: "hwe",
		},
	}

	resolution := Resolve(machine, db)
	if resolution.ProfileID != "broad-tier-1" {
		t.Fatalf("winner = %q, want broad-tier-1; a verified recipe outranks a "+
			"more specific guess from an unverified machine", resolution.ProfileID)
	}
	if resolution.Tier != TierVerified {
		t.Errorf("tier = %d, want 1", resolution.Tier)
	}
}

// TestRule2MoreIdentityFields checks that within a tier, identity fields decide.
func TestRule2MoreIdentityFields(t *testing.T) {
	machine := f15()

	db := []Profile{
		{
			ID:   "one-identity-field",
			Tier: TierVerified,
			Match: Match{
				SysVendor: "ASUSTeK COMPUTER INC.",
			},
		},
		{
			ID:   "three-identity-fields",
			Tier: TierVerified,
			Match: Match{
				SysVendor:     "ASUSTeK COMPUTER INC.",
				ProductName:   "ASUS TUF Gaming F15 FX507ZC4_FX507ZC4",
				ProductFamily: "ASUS TUF Gaming F15",
			},
		},
	}

	resolution := Resolve(machine, db)
	if resolution.ProfileID != "three-identity-fields" {
		t.Errorf("winner = %q, want three-identity-fields", resolution.ProfileID)
	}
}

// TestRule3MoreNarrowingFields checks that narrowing fields break a tie on
// identity alone.
func TestRule3MoreNarrowingFields(t *testing.T) {
	machine := f15()

	db := []Profile{
		{
			ID:   "identity-only",
			Tier: TierVerified,
			Match: Match{
				SysVendor:   "ASUSTeK COMPUTER INC.",
				ProductName: "ASUS TUF Gaming F15 FX507ZC4_FX507ZC4",
			},
		},
		{
			ID:   "identity-plus-narrowing",
			Tier: TierVerified,
			Match: Match{
				SysVendor:   "ASUSTeK COMPUTER INC.",
				ProductName: "ASUS TUF Gaming F15 FX507ZC4_FX507ZC4",
				BoardName:   "FX507ZC4",
				ChassisType: "notebook",
			},
		},
	}

	resolution := Resolve(machine, db)
	if resolution.ProfileID != "identity-plus-narrowing" {
		t.Errorf("winner = %q, want identity-plus-narrowing", resolution.ProfileID)
	}
}

// TestRule4ComparatorPresent checks that a firmware-date comparator is more
// specific than none.
func TestRule4ComparatorPresent(t *testing.T) {
	machine := f15()

	db := []Profile{
		{
			ID:   "no-comparator",
			Tier: TierVerified,
			Match: Match{
				SysVendor: "ASUSTeK COMPUTER INC.",
				BoardName: "FX507ZC4",
			},
		},
		{
			ID:   "with-comparator",
			Tier: TierVerified,
			Match: Match{
				SysVendor: "ASUSTeK COMPUTER INC.",
				BoardName: "FX507ZC4",
				BIOSDate:  ">=2024-01-01",
			},
		},
	}

	resolution := Resolve(machine, db)
	if resolution.ProfileID != "with-comparator" {
		t.Errorf("winner = %q, want with-comparator", resolution.ProfileID)
	}
}

// TestRule5LexicographicallySmallestID checks that resolution is total and
// reproducible even when nothing else separates two profiles.
func TestRule5LexicographicallySmallestID(t *testing.T) {
	machine := f15()

	build := func() []Profile {
		return []Profile{
			{
				ID:   "zebra-profile",
				Tier: TierVerified,
				Match: Match{
					SysVendor: "ASUSTeK COMPUTER INC.",
				},
			},
			{
				ID:   "alpha-profile",
				Tier: TierVerified,
				Match: Match{
					SysVendor: "ASUSTeK COMPUTER INC.",
				},
			},
		}
	}

	// Reversed input order must not change the answer.
	forward := Resolve(machine, build())
	backward := Resolve(machine, func() []Profile {
		profiles := build()
		profiles[0], profiles[1] = profiles[1], profiles[0]
		return profiles
	}())

	if forward.ProfileID != "alpha-profile" {
		t.Errorf("forward winner = %q, want alpha-profile", forward.ProfileID)
	}
	if backward.ProfileID != forward.ProfileID {
		t.Errorf("input order changed the winner: %q vs %q; resolution must "+
			"not depend on file ordering on disk", backward.ProfileID, forward.ProfileID)
	}
}

// TestConflictIsRefusedNotGuessed is the case the tie-break table cannot reach.
//
// Two candidates that tie on tier and on the whole specificity tuple and
// disagree on an actionable key are not a tie-break case. Detection reports the
// conflict, names both profiles and the keys, and falls back to tier 3 —
// because silently picking one of two verified recipes is worse than picking
// neither, since the user cannot tell which one they got.
func TestConflictIsRefusedNotGuessed(t *testing.T) {
	machine := f15()

	db := []Profile{
		{
			ID:   "profile-a",
			Tier: TierVerified,
			Match: Match{
				SysVendor: "ASUSTeK COMPUTER INC.",
			},
			Kernel: "hwe",
			GPU:    GPU{Discrete: "nvidia-open"},
		},
		{
			ID:   "profile-b",
			Tier: TierVerified,
			Match: Match{
				SysVendor: "ASUSTeK COMPUTER INC.",
			},
			Kernel: "generic",
			GPU:    GPU{Discrete: "nouveau"},
		},
	}

	resolution := Resolve(machine, db)

	if resolution.Conflict == nil {
		t.Fatal("two verified profiles disagreeing on the GPU driver were " +
			"resolved to one; that is the exact case the table cannot decide")
	}
	if resolution.Tier != TierBestEffort {
		t.Errorf("tier = %d, want 3; a conflict falls back to the conservative "+
			"defaults", resolution.Tier)
	}
	if resolution.ProfileID != "" {
		t.Errorf("ProfileID = %q; a conflict must not select a profile",
			resolution.ProfileID)
	}

	names := strings.Join(resolution.Conflict.Profiles, ",")
	if !strings.Contains(names, "profile-a") || !strings.Contains(names, "profile-b") {
		t.Errorf("conflict does not name both profiles: %v", resolution.Conflict.Profiles)
	}
	if !containsString(resolution.Conflict.Keys, "gpu.discrete") {
		t.Errorf("conflict keys = %v, want gpu.discrete among them",
			resolution.Conflict.Keys)
	}
	if !containsString(resolution.Conflict.Keys, "kernel") {
		t.Errorf("conflict keys = %v, want kernel among them", resolution.Conflict.Keys)
	}
}

// TestTieThatAgreesIsNotAConflict checks the other side of the same rule: two
// profiles that tie and agree are resolved deterministically, not refused.
func TestTieThatAgreesIsNotAConflict(t *testing.T) {
	machine := f15()

	db := []Profile{
		{
			ID: "duplicate-a", Tier: TierVerified,
			Match:  Match{SysVendor: "ASUSTeK COMPUTER INC."},
			Kernel: "hwe", GPU: GPU{Discrete: "nvidia-open"},
		},
		{
			ID: "duplicate-b", Tier: TierVerified,
			Match:  Match{SysVendor: "ASUSTeK COMPUTER INC."},
			Kernel: "hwe", GPU: GPU{Discrete: "nvidia-open"},
		},
	}

	resolution := Resolve(machine, db)
	if resolution.Conflict != nil {
		t.Errorf("identical profiles reported as a conflict: %+v", resolution.Conflict)
	}
	if resolution.ProfileID != "duplicate-a" {
		t.Errorf("winner = %q, want duplicate-a", resolution.ProfileID)
	}
}

// TestFirmwareDisagreementIsAConflict checks that firmware counts as actionable.
func TestFirmwareDisagreementIsAConflict(t *testing.T) {
	machine := f15()

	db := []Profile{
		{
			ID: "fw-a", Tier: TierCommunityConfirmed,
			Match:    Match{SysVendor: "ASUSTeK COMPUTER INC."},
			Firmware: Firmware{Packages: []string{"linux-firmware"}},
		},
		{
			ID: "fw-b", Tier: TierCommunityConfirmed,
			Match:    Match{SysVendor: "ASUSTeK COMPUTER INC."},
			Firmware: Firmware{Packages: []string{"linux-firmware-intel-wireless"}},
		},
	}

	resolution := Resolve(machine, db)
	if resolution.Conflict == nil {
		t.Fatal("two profiles naming different firmware were not reported as a conflict")
	}
	if !containsString(resolution.Conflict.Keys, "firmware.packages") {
		t.Errorf("conflict keys = %v, want firmware.packages",
			resolution.Conflict.Keys)
	}
}

// TestNoMatchIsTier3WithFallback checks the third pass of the strategy.
func TestNoMatchIsTier3WithFallback(t *testing.T) {
	machine := NewMachine(map[string]string{
		"sys_vendor":   "Unknown Vendor Ltd",
		"product_name": "Unknown Model 9000",
	})

	resolution := Resolve(machine, []Profile{
		{ID: "asus", Tier: TierVerified, Match: Match{SysVendor: "ASUSTeK COMPUTER INC."}},
	})

	if resolution.Tier != TierBestEffort {
		t.Errorf("tier = %d, want 3", resolution.Tier)
	}
	if resolution.Kernel != "generic" {
		t.Errorf("kernel = %q, want %q; an unmatched machine takes the fallback "+
			"block's kernel", resolution.Kernel, "generic")
	}
	if resolution.ProfileID != "" {
		t.Errorf("ProfileID = %q, want empty", resolution.ProfileID)
	}
	if resolution.Reason == "" {
		t.Error("a tier-3 resolution must carry a reason; \"why is my machine " +
			"tier 3\" has to have an answer")
	}
}

// TestARMBoardWithNoDMI is the negative fixture that must never crash and never
// produce a profile.
func TestARMBoardWithNoDMI(t *testing.T) {
	machine := Machine{Fields: map[string]string{}}

	resolution := Resolve(machine, []Profile{
		{ID: "asus", Tier: TierVerified, Match: Match{SysVendor: "ASUSTeK COMPUTER INC."}},
	})

	if !resolution.NoDMI {
		t.Error("NoDMI is false on a machine with no DMI; the absence of a match " +
			"must be explained rather than looking like an unrecognised model")
	}
	if resolution.Tier != TierBestEffort {
		t.Errorf("tier = %d, want 3", resolution.Tier)
	}
	if resolution.ProfileID != "" {
		t.Errorf("ProfileID = %q; a machine with no DMI can never match",
			resolution.ProfileID)
	}
	if !strings.Contains(resolution.Reason, "dmi unavailable") {
		t.Errorf("reason = %q, want it to name dmi unavailable", resolution.Reason)
	}
}

// TestBIOSDateComparator covers the only non-equality in the match key, and the
// day/month ambiguity that makes it worth writing down.
func TestBIOSDateComparator(t *testing.T) {
	tests := []struct {
		machineDate string
		comparator  string
		want        bool
	}{
		// 12/03/2024 is 3 December, not 12 March. Under a December threshold it
		// passes; under a March threshold it must not, which is the whole point.
		{"12/03/2024", ">=2024-12-01", true},
		{"12/03/2024", ">=2025-01-01", false},
		{"12/03/2024", "<2024-12-01", false},
		{"12/03/2024", "==2024-12-03", true},
		{"01/02/2024", ">=2024-01-01", true},
		{"06/15/2022", ">=2024-01-01", false},
		{"08/15/2023", ">2023-01-01", true},
	}

	for _, tc := range tests {
		ok, err := compareBIOSDate(tc.machineDate, tc.comparator)
		if err != nil {
			t.Errorf("compareBIOSDate(%q, %q): %v", tc.machineDate, tc.comparator, err)
			continue
		}
		if ok != tc.want {
			t.Errorf("compareBIOSDate(%q, %q) = %v, want %v",
				tc.machineDate, tc.comparator, ok, tc.want)
		}
	}
}

// TestBIOSDateRejectsMalformed checks that a firmware date nobody reported is
// rejected rather than guessed.
func TestBIOSDateRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"2024-12-03", "12/3/2024", "garbage", "", "31/12/2023"} {
		if _, err := compareBIOSDate(bad, ">=2024-01-01"); err == nil {
			t.Errorf("compareBIOSDate(%q, ...) accepted a malformed firmware date", bad)
		}
	}
}

// TestAlsoMatchedRecordsLosers checks the field that turns "why am I tier 3"
// into a one-line answer.
func TestAlsoMatchedRecordsLosers(t *testing.T) {
	machine := f15()

	resolution := Resolve(machine, []Profile{
		{ID: "winner", Tier: TierVerified, Match: Match{SysVendor: "ASUSTeK COMPUTER INC."}},
		{ID: "loser", Tier: TierCommunityConfirmed,
			Match: Match{SysVendor: "ASUSTeK COMPUTER INC.", BoardName: "FX507ZC4"}},
	})

	if resolution.ProfileID != "winner" {
		t.Fatalf("winner = %q, want winner", resolution.ProfileID)
	}
	if len(resolution.AlsoMatched) != 1 {
		t.Fatalf("AlsoMatched = %+v, want the one losing candidate",
			resolution.AlsoMatched)
	}
	if resolution.AlsoMatched[0].ID != "loser" {
		t.Errorf("AlsoMatched[0].ID = %q, want loser", resolution.AlsoMatched[0].ID)
	}
}

// TestMatchedAndUnconstrainedFields checks the two lists that answer the two
// questions a user actually has: what claimed this machine, and what would
// change the answer.
func TestMatchedAndUnconstrainedFields(t *testing.T) {
	machine := f15()

	resolution := Resolve(machine, []Profile{{
		ID:   "partial",
		Tier: TierVerified,
		Match: Match{
			SysVendor: "ASUSTeK COMPUTER INC.",
			BoardName: "FX507ZC4",
		},
	}})

	if !containsString(resolution.MatchedFields, "sys_vendor") ||
		!containsString(resolution.MatchedFields, "board_name") {
		t.Errorf("MatchedFields = %v, want sys_vendor and board_name",
			resolution.MatchedFields)
	}
	if containsString(resolution.MatchedFields, "product_sku") {
		t.Errorf("MatchedFields = %v, must not contain an unconstrained field",
			resolution.MatchedFields)
	}
	if !containsString(resolution.UnconstrainedFields, "product_sku") {
		t.Errorf("UnconstrainedFields = %v, want product_sku",
			resolution.UnconstrainedFields)
	}
}

// TestResolutionIsDeterministic runs the whole thing repeatedly, because a
// resolution that varied between runs would be untestable and untrustworthy.
func TestResolutionIsDeterministic(t *testing.T) {
	machine := f15()
	db := []Profile{
		{ID: "a", Tier: TierVerified, Match: Match{SysVendor: "ASUSTeK COMPUTER INC."}, Kernel: "hwe"},
		{ID: "b", Tier: TierCommunityConfirmed, Match: Match{SysVendor: "ASUSTeK COMPUTER INC.", BoardName: "FX507ZC4"}},
		{ID: "c", Tier: TierCommunityConfirmed, Match: Match{ProductFamily: "ASUS TUF Gaming F15", ChassisType: "notebook"}},
	}

	first := Resolve(machine, db)
	for run := 0; run < 50; run++ {
		got := Resolve(machine, db)
		if got.ProfileID != first.ProfileID || got.Tier != first.Tier {
			t.Fatalf("run %d resolved differently: %q/%d vs %q/%d",
				run, got.ProfileID, got.Tier, first.ProfileID, first.Tier)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// TestARMLaptopCanReachTier2 is the reach test, and it is the one that matters.
//
// The machine has no SMBIOS table, so DMI reports nothing and the only identity
// available is in its device tree. Before the device-tree probe this machine
// could not match a profile at all: it was capped at tier 3 forever, however
// well it worked and however many people reported it. That is a ceiling on
// support, not a limitation of it — and support is the product.
//
// A profile written in the ordinary match vocabulary must now claim it.
func TestARMLaptopCanReachTier2(t *testing.T) {
	// Identity as the device tree reports it, projected onto the shared
	// match-key fields.
	machine := NewMachine(map[string]string{
		"sys_vendor":   "qcom,x1e-generic",
		"product_name": "Snapdragon X Elite Laptop",
		"chassis_type": "notebook",
	})

	db := []Profile{
		{
			ID:   "generic-snapdragon-laptop",
			Tier: TierCommunityConfirmed,
			Match: Match{
				SysVendor:   "qcom,x1e-generic",
				ProductName: "Snapdragon X Elite Laptop",
			},
			Kernel: "hwe",
			GPU:    GPU{Integrated: "auto"},
		},
	}

	resolution := Resolve(machine, db)

	if resolution.NoDMI {
		t.Error("NoDMI is true even though the device tree supplied an identity")
	}
	if resolution.ProfileID != "generic-snapdragon-laptop" {
		t.Fatalf("winner = %q; a machine whose only identity is its device tree "+
			"must still be able to match a profile", resolution.ProfileID)
	}
	if resolution.Tier != TierCommunityConfirmed {
		t.Errorf("tier = %d, want 2", resolution.Tier)
	}
	if resolution.Kernel != "hwe" {
		t.Errorf("kernel = %q, want hwe", resolution.Kernel)
	}
}

// TestMachineWithNeitherDMIorDeviceTree is the other half.
//
// A board with no SMBIOS and no device tree has told detection nothing at all.
// It must degrade to the conservative defaults and say why, and it must never
// be handed a profile that never claimed it.
func TestMachineWithNeitherDMIorDeviceTree(t *testing.T) {
	machine := Machine{Fields: map[string]string{}}

	resolution := Resolve(machine, []Profile{
		{
			ID:   "broad",
			Tier: TierVerified,
			Match: Match{
				SysVendor: AnyMatch,
			},
		},
	})

	if resolution.ProfileID != "" {
		t.Errorf("ProfileID = %q; a wildcard profile must not become a catch-all "+
			"that claims machines nothing was known about", resolution.ProfileID)
	}
	if resolution.Tier != TierBestEffort {
		t.Errorf("tier = %d, want 3", resolution.Tier)
	}
	if resolution.Kernel != "generic" {
		t.Errorf("kernel = %q, want generic", resolution.Kernel)
	}
}
