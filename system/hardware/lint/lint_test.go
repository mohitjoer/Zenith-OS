package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write drops a profile into a temporary directory and loads it.
func write(t *testing.T, name, body string) Profile {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	profile, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%s): %v", name, err)
	}
	return profile
}

// rules returns the set of rules that fired.
func rules(findings []Finding) map[string]Severity {
	set := map[string]Severity{}
	for _, finding := range findings {
		set[finding.Rule] = finding.Severity
	}
	return set
}

// validProfile is a profile that must lint clean, and every negative test below
// breaks exactly one thing about it.
const validProfile = `version: 1
id: asus-rog-strix-g16-2024
license: GPL-3.0
tier: 2
match:
  sys_vendor: ASUSTeK
  product_name: ROG Strix G16
  board_name: G614
kernel: hwe
gpu:
  discrete: nvidia-open
network:
  wifi:
    - driver: mt7921e
firmware:
  packages: [linux-firmware, linux-firmware-mediatek]
  microcode: [intel-microcode]
`

func TestValidProfileLintsClean(t *testing.T) {
	findings := Lint(write(t, "asus-rog-strix-g16-2024.yaml", validProfile))
	for _, finding := range findings {
		t.Errorf("unexpected finding: %s", finding)
	}
}

// TestExampleProfileIsExactlyAsCleanAsATemplateCanBe checks the repository's own
// annotated example.
//
// example.yaml is documentation: its id is the placeholder "vendor-model-
// variant" and its filename is "example", so the id-mismatch rule fires on it
// by construction. LoadDir skips it by name for exactly that reason, and this
// test pins the consequence: the example produces that one finding and no
// others. A tool that reports the repository's own example teaches
// contributors to ignore it, so anything more than this is a defect.
func TestExampleProfileIsExactlyAsCleanAsATemplateCanBe(t *testing.T) {
	path := filepath.Join("..", "profiles", "example.yaml")
	example, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	findings := Lint(example)
	if len(findings) != 1 || findings[0].Rule != "profile/id-mismatch" {
		for _, finding := range findings {
			t.Errorf("unexpected finding on the example profile: %s", finding)
		}
		if len(findings) == 0 {
			t.Error("expected exactly the id-mismatch a template necessarily has")
		}
	}
}

// TestLoadDirSkipsTheExample checks the mechanism that keeps it out of the way.
func TestLoadDirSkipsTheExample(t *testing.T) {
	profiles, err := LoadDir(filepath.Join("..", "profiles"))
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	// The directory currently holds only the example, so a correct loader
	// returns nothing — and Run must then refuse rather than report success.
	if len(profiles) != 0 {
		t.Errorf("LoadDir returned %d profiles, want 0 (only example.yaml is present)",
			len(profiles))
	}
	if _, err := Run(profiles); err == nil {
		t.Error("Run over a directory of no real profiles reported clean")
	}
}

// TestKernelPatchPinIsRefused covers the rule that keeps a profile matching
// after the next security update.
func TestKernelPatchPinIsRefused(t *testing.T) {
	body := strings.Replace(validProfile, "kernel: hwe", "kernel: 6.8.0-31-generic", 1)
	findings := Lint(write(t, "asus-rog-strix-g16-2024.yaml", body))

	if _, fired := rules(findings)["profile/kernel-patch-pin"]; !fired {
		t.Errorf("a patch-level kernel pin was accepted: %v", findings)
	}
}

func TestUnknownKernelFlavourIsRefused(t *testing.T) {
	for _, kernel := range []string{"6.1", "stable", "hwe-fixes", "HWE"} {
		body := strings.Replace(validProfile, "kernel: hwe", "kernel: "+kernel, 1)
		findings := Lint(write(t, "asus-rog-strix-g16-2024.yaml", body))
		if _, fired := rules(findings)["profile/kernel-unknown"]; !fired {
			t.Errorf("kernel %q was accepted: %v", kernel, findings)
		}
	}
}

func TestMissingKernelIsRefused(t *testing.T) {
	body := strings.Replace(validProfile, "kernel: hwe\n", "", 1)
	findings := Lint(write(t, "asus-rog-strix-g16-2024.yaml", body))
	if _, fired := rules(findings)["profile/kernel-missing"]; !fired {
		t.Errorf("a profile with no kernel flavour was accepted: %v", findings)
	}
}

// TestIDMustMatchFilename checks the id is findable from a bug report.
func TestIDMustMatchFilename(t *testing.T) {
	profile := write(t, "asus-rog-strix-g16-2024.yaml", validProfile)
	profile.ID = "some-other-name"
	if _, fired := rules(Lint(profile))["profile/id-mismatch"]; !fired {
		t.Error("an id disagreeing with the filename was accepted")
	}
}

func TestIDShapeIsEnforced(t *testing.T) {
	for _, id := range []string{"Asus_ROG", "asus rog", "asus--rog", "-asus"} {
		body := strings.Replace(validProfile,
			"id: asus-rog-strix-g16-2024", "id: "+id, 1)
		profile := write(t, "asus-rog-strix-g16-2024.yaml", body)
		if _, fired := rules(Lint(profile))["profile/id-shape"]; !fired {
			t.Errorf("id %q was accepted", id)
		}
	}
}

// TestTierOneCannotBeClaimed covers the rule that a contributor cannot self-
// award the tier that means a maintainer ran a checklist.
func TestTierOneCannotBeClaimed(t *testing.T) {
	body := strings.Replace(validProfile, "tier: 2", "tier: 1", 1)
	findings := Lint(write(t, "asus-rog-strix-g16-2024.yaml", body))
	if _, fired := rules(findings)["profile/tier-claimed"]; !fired {
		t.Errorf("a submitted profile claimed tier 1: %v", findings)
	}
}

func TestInvalidTierIsRefused(t *testing.T) {
	for _, tier := range []string{"0", "4", "two", "-1"} {
		body := strings.Replace(validProfile, "tier: 2", "tier: "+tier, 1)
		findings := Lint(write(t, "asus-rog-strix-g16-2024.yaml", body))
		if _, fired := rules(findings)["profile/tier-invalid"]; !fired {
			t.Errorf("tier %q was accepted", tier)
		}
	}
}

// TestEmptyMatchBlockIsRefused covers the catch-all. A profile that constrains
// nothing would match every machine, and the matcher rejects it, so it can never
// be selected — a silently dead file.
func TestEmptyMatchBlockIsRefused(t *testing.T) {
	body := "version: 1\nid: catch-all\nlicense: GPL-3.0\ntier: 2\nkernel: hwe\n"
	findings := Lint(write(t, "catch-all.yaml", body))
	if _, fired := rules(findings)["profile/match-empty"]; !fired {
		t.Errorf("a profile with no match block was accepted: %v", findings)
	}
}

// TestMatchFieldOutsideTheVocabulary covers the silent failure: a field the
// probes never produce can never be satisfied, so the profile is dead and says
// nothing about why.
func TestMatchFieldOutsideTheVocabulary(t *testing.T) {
	body := strings.Replace(validProfile,
		"  board_name: G614",
		"  board_name: G614\n  dmi_chassis_asset: G614", 1)
	findings := Lint(write(t, "asus-rog-strix-g16-2024.yaml", body))

	set := rules(findings)
	if _, fired := set["profile/match-unknown-field"]; !fired {
		t.Errorf("a match field outside the detection vocabulary was accepted: %v", findings)
	}
}

// TestEmptyMatchValueIsRefused covers the difference between a field a profile
// says nothing about and a field it requires to be blank.
func TestEmptyMatchValueIsRefused(t *testing.T) {
	body := strings.Replace(validProfile, "  board_name: G614", "  board_name:", 1)
	findings := Lint(write(t, "asus-rog-strix-g16-2024.yaml", body))
	if _, fired := rules(findings)["profile/match-empty-value"]; !fired {
		t.Errorf("an empty match value was accepted: %v", findings)
	}
}

// TestFirmwareMustBeAPackageNotABlob covers both halves: a wrong package name,
// and a filename or path.
func TestFirmwareMustBeAPackageNotABlob(t *testing.T) {
	body := strings.Replace(validProfile,
		"packages: [linux-firmware, linux-firmware-mediatek]",
		"packages: [firmware-mediatek]", 1)
	findings := Lint(write(t, "asus-rog-strix-g16-2024.yaml", body))
	if _, fired := rules(findings)["profile/firmware-package-shape"]; !fired {
		t.Errorf("a pre-split firmware name was accepted: %v", findings)
	}

	blob := validProfile + "\n# see /lib/firmware/mediatek/mt7921e.bin\n"
	findings = Lint(write(t, "asus-rog-strix-g16-2024.yaml", blob))
	if _, fired := rules(findings)["profile/firmware-file"]; !fired {
		t.Errorf("a firmware path was accepted: %v", findings)
	}
}

// TestIdentifyingContentIsRefused covers the security-relevant half. A profile is
// committed to a public repository, so a serial or a home path in one is there
// permanently.
func TestIdentifyingContentIsRefused(t *testing.T) {
	tests := []struct {
		name string
		rule string
		body string
	}{
		{"mac", "profile/mac-address",
			validProfile + "notes: the nic is a4:bb:6d:11:22:33\n"},
		{"home", "profile/home-path",
			validProfile + "notes: copy from /home/mohit/config\n"},
		{"root", "profile/home-path",
			validProfile + "notes: config lives under /root/.config\n"},
		{"linklocal", "profile/ipv6-link-local",
			validProfile + "notes: gateway fe80::1\n"},
	}

	for _, tc := range tests {
		findings := Lint(write(t, "asus-rog-strix-g16-2024.yaml", tc.body))
		if _, fired := rules(findings)[tc.rule]; !fired {
			t.Errorf("%s: not caught, findings were %v", tc.name, findings)
		}
	}
}

// TestFindingCarriesLineNumbers checks a finding points at something, because a
// merge review that has to search a 100-line profile will not.
func TestFindingCarriesLineNumbers(t *testing.T) {
	findings := Lint(write(t, "asus-rog-strix-g16-2024.yaml",
		validProfile+"notes: nic a4:bb:6d:11:22:33\n"))
	for _, finding := range findings {
		if finding.Rule == "profile/mac-address" && finding.Line == 0 {
			t.Error("a line-based finding carries no line number")
		}
	}
}

// TestOverlappingProfilesWarn covers the database check the design record asks
// for: a narrow tier 2 and a broad tier 1 covering the same machines is legal
// today and is the shape that becomes a conflict once a third arrives.
func TestOverlappingProfilesWarn(t *testing.T) {
	narrow := write(t, "asus-rog-strix-g16-2024.yaml", validProfile)
	broad := write(t, "asus-rog-strix-g16.yaml", strings.Replace(
		validProfile, "id: asus-rog-strix-g16-2024", "id: asus-rog-strix-g16", 1))

	findings := LintDatabase([]Profile{narrow, broad})
	if _, fired := rules(findings)["database/overlapping-profiles"]; !fired {
		t.Errorf("two overlapping profiles produced no warning: %v", findings)
	}
	for _, finding := range findings {
		if finding.Severity != Warning {
			t.Errorf("overlap should be a warning, got %s", finding.Severity)
		}
	}
}

// TestContradictoryProfilesDoNotOverlap checks the other direction: two profiles
// that cannot both match anything are not reported, or the warning becomes noise
// nobody reads.
func TestContradictoryProfilesDoNotOverlap(t *testing.T) {
	a := write(t, "asus-a.yaml", strings.Replace(
		validProfile, "board_name: G614", "board_name: G614", 1))
	b := write(t, "asus-b.yaml", strings.Replace(
		validProfile, "board_name: G614", "board_name: G615", 1))

	if findings := LintDatabase([]Profile{a, b}); len(findings) != 0 {
		t.Errorf("contradictory profiles were reported as overlapping: %v", findings)
	}
}

// TestNonOverlappingProfilesAreQuiet keeps the warning meaningful.
func TestNonOverlappingProfilesAreQuiet(t *testing.T) {
	a := write(t, "asus.yaml", validProfile)
	b := write(t, "lenovo.yaml", strings.NewReplacer(
		"id: asus-rog-strix-g16-2024", "id: lenovo-thinkpad-t14",
		"sys_vendor: ASUSTeK", "sys_vendor: LENOVO",
		"product_name: ROG Strix G16", "product_name: ThinkPad T14",
		"board_name: G614", "board_name: 21J5",
	).Replace(validProfile))

	if findings := LintDatabase([]Profile{a, b}); len(findings) != 0 {
		t.Errorf("unrelated profiles were reported as overlapping: %v", findings)
	}
}

// TestEmptyDatabaseDoesNotLintClean checks Run refuses an empty result.
//
// A linter that reports "no findings" over zero files is indistinguishable from
// a linter that found nothing wrong, and the first is a bug report.
func TestEmptyDatabaseDoesNotLintClean(t *testing.T) {
	if _, err := Run(nil); err == nil {
		t.Error("Run(nil) returned no findings and no error")
	}
}

// TestStripCommentRespectsQuotes checks a value containing '#' survives.
func TestStripCommentRespectsQuotes(t *testing.T) {
	if got := stripComment(`model: "GPU#1"`); strings.Contains(got, "model") == false ||
		!strings.Contains(got, "GPU#1") {
		t.Errorf("stripComment truncated a quoted value: %q", got)
	}
	if got := stripComment("model: GPU  # a comment"); strings.Contains(got, "comment") {
		t.Errorf("stripComment kept a comment: %q", got)
	}
	if got := stripComment("model: a#b"); !strings.Contains(got, "a#b") {
		t.Errorf("stripComment truncated mid-word: %q", got)
	}
}
