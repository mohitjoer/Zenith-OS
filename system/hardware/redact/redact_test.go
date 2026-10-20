package redact

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zenith/hardware/probe"
)

// testSalt is a fixed key. Tests need determinism, not entropy.
func testSalt() Salt {
	var salt Salt
	for i := range salt {
		salt[i] = byte(i)
	}
	return salt
}

// TestTokenIsStablePerMachine checks the property a maintainer depends on: two
// reports from the same machine produce the same token, so they can tell that
// two pull requests came from the same laptop.
func TestTokenIsStablePerMachine(t *testing.T) {
	salt := testSalt()
	first := Token(salt, "product_uuid", "4C4C4544-0051-3810-8043-B9C04F503632")
	second := Token(salt, "product_uuid", "4C4C4544-0051-3810-8043-B9C04F503632")

	if first != second {
		t.Errorf("token is not stable: %q vs %q", first, second)
	}
	if !strings.HasPrefix(first, "sha256:") {
		t.Errorf("token %q lacks the sha256: prefix", first)
	}
	if strings.Contains(first, "4C4C4544") {
		t.Error("token contains the raw value")
	}
}

// TestTokenIsFieldScoped checks that a token cannot be lifted from one field into
// another. The DMI UUID token and the EDID serial token for the same machine
// must differ, because they are different fields that happen to hold equal
// values.
func TestTokenIsFieldScoped(t *testing.T) {
	salt := testSalt()
	const value = "4C4C4544-0051-3810-8043-B9C04F503632"

	uuid := Token(salt, "product_uuid", value)
	edid := Token(salt, "edid_serial", value)

	if uuid == edid {
		t.Errorf("the same value in two fields produced one token: %q", uuid)
	}
}

// TestRotatingSaltChangesEveryToken checks the other property: tokens are not a
// durable machine fingerprint, so no external index of them is meaningful.
func TestRotatingSaltChangesEveryToken(t *testing.T) {
	first := Token(testSalt(), "product_uuid", "value")
	second := Token(Salt{}, "product_uuid", "value")
	if first == second {
		t.Error("rotating the salt produced an identical token")
	}
}

// TestTokenOfEmptyValueIsEmpty keeps "absent" from becoming a token, which would
// otherwise let every machine with no value in a field share a token.
func TestTokenOfEmptyValueIsEmpty(t *testing.T) {
	if got := Token(testSalt(), "product_uuid", ""); got != "" {
		t.Errorf("Token of an empty value = %q, want empty", got)
	}
}

// TestSweepRefusesASerial is the test that matters most: a serial that must not
// be published stops the report.
func TestSweepRefusesASerial(t *testing.T) {
	rendered := []byte("sys_vendor: ASUSTeK COMPUTER INC.\nproduct_serial: N0ABCDE12345\n")

	findings := Verify(rendered, Context{
		RawDMI:     map[string]string{"product_serial": "N0ABCDE12345"},
		RawSerials: []string{"N0ABCDE12345"},
	})

	if len(findings) == 0 {
		t.Fatal("a report carrying a raw serial was allowed through")
	}
	if Refusal(findings) == "" {
		t.Error("Refusal is empty for a non-empty finding set")
	}
}

// TestSweepAllowsAValueAKeptFieldCarries is the other half of the rule.
//
// Chassis vendor and board vendor are the same manufacturer on most machines,
// and chassis version and product version are often the same string. A field
// the report does not carry, whose value the report carries anyway under
// another name, is not a leak — and treating it as one stops the report dead
// on a machine that has two DMI attributes saying the same thing.
//
// This is what a real machine does. The report was refused on an ASUS laptop
// for exactly this reason: chassis_vendor held the same string as
// board_vendor, and chassis_version held the same string as product_version.
//
// The identifier is still caught: a serial number is not a kept field, and if
// its value reaches the report this rule fires on it. TestSweepRefusesASerial
// above is that case.
func TestSweepAllowsAValueAKeptFieldCarries(t *testing.T) {
	const vendor = "ASUSTeK COMPUTER INC."
	rendered := []byte("board_vendor: " + vendor + "\nproduct_version: 1.0\n")

	findings := Verify(rendered, Context{
		RawDMI: map[string]string{
			// Kept, and therefore legitimately in the report.
			"board_vendor":    vendor,
			"product_version": "1.0",
			// Not kept, and not read. Their values are the same strings.
			"chassis_vendor":  vendor,
			"chassis_version": "1.0",
		},
	})

	for _, finding := range findings {
		if finding.Rule == "unclassified-dmi-field" {
			t.Errorf("findings = %v, want no unclassified-dmi-field finding: the "+
				"value is carried by a kept field under another name", findings)
		}
	}
}

// TestSweepStillRefusesAnIdentifierBesideAKeptOne is the same machine, with a
// serial in it.
//
// The exemption above is for a duplicate, not for an identifier: a serial
// number that reaches the report stops it, even when a kept field happens to
// carry the same string.
func TestSweepStillRefusesAnIdentifierBesideAKeptOne(t *testing.T) {
	const serial = "N0ABCDE12345"
	rendered := []byte("board_name: FX507ZC4\nproduct_serial: " + serial + "\n")

	findings := Verify(rendered, Context{
		RawDMI: map[string]string{
			"board_name":     "FX507ZC4",
			"product_serial": serial,
		},
		RawSerials: []string{serial},
	})

	if len(findings) == 0 {
		t.Fatal("a report carrying a raw serial was allowed through")
	}
}

// TestSweepRefusesAssetTag checks the field that is dropped rather than hashed.
// Asset tags are world-readable, stable, low-entropy and map to a purchasing
// record, so they are the field most likely to be enumerable — which is exactly
// why hashing them is worse than dropping them.
func TestSweepRefusesAssetTag(t *testing.T) {
	const tag = "ASSET-TAG-778899"
	rendered := []byte("chassis_asset_tag: " + tag + "\n")

	findings := Verify(rendered, Context{
		RawDMI: map[string]string{"chassis_asset_tag": tag},
	})

	if len(findings) == 0 {
		t.Fatal("a report carrying an asset tag was allowed through")
	}
	found := false
	for _, finding := range findings {
		if finding.Rule == "unclassified-dmi-field" {
			found = true
		}
	}
	if !found {
		t.Errorf("findings = %v, want an unclassified-dmi-field finding", findings)
	}
	// The finding must not republish the tag.
	for _, finding := range findings {
		if strings.Contains(finding.Detail, tag) {
			t.Errorf("finding detail echoes the secret: %q", finding.Detail)
		}
	}
}

// TestSweepRefusesMACAddress covers the stable cross-network identifier.
func TestSweepRefusesMACAddress(t *testing.T) {
	rendered := []byte("interface: wlan0\naddress: a4:bb:6d:11:22:33\n")

	findings := Verify(rendered, Context{})
	if len(findings) == 0 {
		t.Fatal("a report carrying a MAC address was allowed through")
	}
	if findings[0].Rule != "mac-address" {
		t.Errorf("rule = %q, want mac-address", findings[0].Rule)
	}
}

// TestSweepRefusesIPv6LinkLocal covers the address form that a MAC pattern
// would miss entirely.
func TestSweepRefusesIPv6LinkLocal(t *testing.T) {
	rendered := []byte("gateway: fe80::a4bb:6dff:fe11:2233\n")

	findings := Verify(rendered, Context{})
	if len(findings) == 0 {
		t.Fatal("a link-local IPv6 address was allowed through")
	}
	if findings[0].Rule != "ipv6-link-local" {
		t.Errorf("rule = %q, want ipv6-link-local", findings[0].Rule)
	}
}

// TestSweepRefusesHomeDirectory covers the three path shapes that carry a
// username.
func TestSweepRefusesHomeDirectory(t *testing.T) {
	for _, path := range []string{
		"/home/alice/.config/zenith",
		"/root/.cache",
		"/Users/alice/Library",
	} {
		findings := Verify([]byte("path: "+path+"\n"), Context{})
		if len(findings) == 0 {
			t.Errorf("a report carrying %q was allowed through", path)
		}
	}
}

// TestSweepRefusesMachineIdentity covers the literal values the sweep cannot
// recognise by shape: a machine-id, a hostname, a username.
func TestSweepRefusesMachineIdentity(t *testing.T) {
	context := Context{
		Hostname:   "mohit-thinkpad",
		User:       "mohit",
		Logname:    "mohit",
		MachineIDs: []string{"4d7e2a1b9c8f0e3d5a6b7c8d9e0f1a2b"},
	}

	tests := []struct {
		name string
		text string
	}{
		{"hostname", "host: mohit-thinkpad\n"},
		{"username", "owner: mohit\n"},
		{"machine-id", "machine_id: 4d7e2a1b9c8f0e3d5a6b7c8d9e0f1a2b\n"},
	}

	for _, tc := range tests {
		findings := Verify([]byte(tc.text), context)
		if len(findings) == 0 {
			t.Errorf("%s: report allowed through\n%s", tc.name, tc.text)
		}
	}
}

// TestSweepAllowsHexIdentifiers is the false positive the design record calls
// out as something to design around rather than to weaken.
//
// A report's whole value is hardware identifiers. If rule 2 fires on them, the
// check is disabled by exactly the data it exists to protect. Four hex digits a
// side does not match a six-pair MAC, but "0011:2233" sits close enough that the
// sweep has to be explicitly allowlist-aware.
func TestSweepAllowsHexIdentifiers(t *testing.T) {
	allowed := []string{"8086:46a6", "0c45:6366", "0011:2233", "10de:2684"}

	rendered := []byte(
		"pci:\n" +
			"  - vendor_device: 8086:46a6\n" +
			"  - vendor_device: 0c45:6366\n" +
			"  - vendor_device: 0011:2233\n" +
			"  - vendor_device: 10de:2684\n")

	findings := Verify(rendered, Context{AllowedIdentifiers: allowed})
	for _, finding := range findings {
		if finding.Rule == "mac-address" {
			t.Errorf("a legitimate PCI/USB identifier was flagged as a MAC: %q",
				finding.Detail)
		}
	}
}

// TestSweepStillRefusesARealMACAmongIdentifiers checks that allowlisting does not
// become a blanket exemption for anything hex-shaped in the report.
func TestSweepStillRefusesARealMACAmongIdentifiers(t *testing.T) {
	rendered := []byte(
		"pci:\n  - vendor_device: 8086:46a6\n" +
			"net:\n  - address: de:ad:be:ef:00:01\n")

	findings := Verify(rendered, Context{AllowedIdentifiers: []string{"8086:46a6"}})
	found := false
	for _, finding := range findings {
		if finding.Rule == "mac-address" && strings.Contains(finding.Detail, "de:ad:be:ef") {
			found = true
		}
	}
	if !found {
		t.Errorf("a real MAC passed alongside an allowlisted identifier: %v", findings)
	}
}

// TestSweepAllowsACleanReport is the positive case: a report built the way the
// policy says must pass.
func TestSweepAllowsACleanReport(t *testing.T) {
	rendered := []byte(strings.Join([]string{
		"# Zenith hardware report",
		"dmi:",
		"  sys_vendor: ASUSTeK COMPUTER INC.",
		"  product_name: ASUS TUF Gaming F15 FX507ZC4_FX507ZC4",
		"  product_uuid_token: " + Token(testSalt(), "product_uuid", "4C4C4544-0051") + "",
		"pci:",
		"  - vendor: 0x8086",
		"  - device: 0x46a6",
		"  - class: 0x030000",
		"edid:",
		"  - manufacturer: NCP",
		"  - monitor_name: LM156LF-2F01",
	}, "\n"))

	context := Context{
		RawDMI: map[string]string{
			"sys_vendor":     "ASUSTeK COMPUTER INC.",
			"product_name":   "ASUS TUF Gaming F15 FX507ZC4_FX507ZC4",
			"product_uuid":   "4C4C4544-0051",
			"product_serial": "N0ABCDE12345",
		},
		AllowedIdentifiers: []string{"8086:46a6"},
		Hostname:           "mohit-thinkpad",
		User:               "mohit",
	}

	if findings := Verify(rendered, context); len(findings) != 0 {
		t.Errorf("a clean report was refused: %v", findings)
	}
}

// TestSweepReportsEveryFinding checks that one pass reports all of them. A
// report that leaked three identifiers should not need three round trips.
func TestSweepReportsEveryFinding(t *testing.T) {
	rendered := []byte("a4:bb:6d:11:22:33\n/home/alice/x\nfe80::1\n")
	if findings := Verify(rendered, Context{}); len(findings) < 3 {
		t.Errorf("findings = %d, want at least 3", len(findings))
	}
}

// TestRefusalNeverEchoesTheSecret checks the message an emitter exits with,
// because that message ends up in a CI log.
func TestRefusalNeverEchoesTheSecret(t *testing.T) {
	const serial = "N0ABCDE12345"
	findings := Verify([]byte("product_serial: "+serial+"\n"), Context{
		RawDMI:     map[string]string{"product_serial": serial},
		RawSerials: []string{serial},
	})
	refusal := Refusal(findings)

	if strings.Contains(refusal, serial) {
		t.Errorf("the refusal message echoes the serial: %q", refusal)
	}
	if !strings.Contains(refusal, "refused") {
		t.Errorf("refusal = %q, want it to say the report was refused", refusal)
	}
	if Refusal(nil) != "" {
		t.Error("Refusal(nil) is not empty")
	}
}

// TestSaltRoundTrip checks the salt file, including its permissions.
func TestSaltRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "zenith", "hardware", "report-salt")

	created, err := LoadOrCreateSalt(path)
	if err != nil {
		t.Fatalf("LoadOrCreateSalt: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat salt: %v", err)
	}
	// The salt is the only thing standing between a published token and the
	// serial behind it.
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("salt mode = %o, want 600", perm)
	}

	reloaded, err := LoadOrCreateSalt(path)
	if err != nil {
		t.Fatalf("reloading salt: %v", err)
	}
	if reloaded != created {
		t.Error("reloaded salt differs from the created one")
	}
	if Token(reloaded, "f", "v") != Token(created, "f", "v") {
		t.Error("tokens differ across a salt round trip")
	}
}

// TestSaltIsRandom checks that two machines do not get the same key.
func TestSaltIsRandom(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreateSalt(filepath.Join(dir, "a"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateSalt(filepath.Join(dir, "b"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Error("two salts are identical; the key must be random per machine")
	}
}

// TestCorruptSaltIsRejected checks that a damaged salt file is an error rather
// than a silently different key, which would change every token on the machine.
func TestCorruptSaltIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report-salt")
	if err := os.WriteFile(path, []byte("not hex\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateSalt(path); err == nil {
		t.Error("a corrupt salt file was accepted")
	}
}

// TestRandomSaltIsUsable guards the fixture helper against a silent zero key.
func TestRandomSaltIsUsable(t *testing.T) {
	var salt Salt
	if _, err := rand.Read(salt[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	if Token(salt, "f", "v") == Token(Salt{}, "f", "v") {
		t.Error("a random salt token equals the zero salt token")
	}
}

// TestKeepSetMatchesTheReaderPolicy pins the invariant that the redactor's
// keep-set is derived from the reader's allowlist rather than restated.
//
// These two used to be separate hand-written lists. They cannot drift silently
// any more, and if this test is ever deleted the reason should be that the
// derivation was replaced by something better — not that somebody found it
// redundant.
func TestKeepSetMatchesTheReaderPolicy(t *testing.T) {
	policy := probe.DMIPolicy()
	if len(policy) == 0 {
		t.Fatal("the reader policy is empty")
	}

	for name, class := range policy {
		switch class {
		case probe.DMIClassKept, probe.DMIClassMapped, probe.DMIClassHashed:
			if !dmiKeptFields[name] {
				t.Errorf("%q is %q in the reader policy but absent from the "+
					"redactor's keep-set; every report containing it would fail "+
					"to render", name, class)
			}
		case probe.DMIClassNotRead:
			if dmiKeptFields[name] {
				t.Errorf("%q is %q in the reader policy but present in the "+
					"redactor's keep-set", name, class)
			}
		case probe.DMIClassDuplicate:
			// A duplicate is not carried by the report under its own name, so
			// it is not in the keep-set. The sweep exempts it on its own
			// terms: a kept field carrying the same string.
			if dmiKeptFields[name] {
				t.Errorf("%q is %q in the reader policy but present in the "+
					"redactor's keep-set", name, class)
			}
		default:
			t.Errorf("%q has an unclassified policy value %q", name, class)
		}
	}
}

// TestReaderPolicyCoversTheClassifiedFields checks the reader side of the same
// invariant: every sysfs attribute the reader knows about is classified, so no
// field can slip through unclassified and into a report.
func TestReaderPolicyCoversTheClassifiedFields(t *testing.T) {
	// The attributes the reference machine's DMI directory carries. Each must be
	// classified, and the serial and asset-tag ones must be classified not-read.
	mustNotRead := []string{
		"product_serial", "board_serial", "chassis_serial",
		"chassis_asset_tag", "board_asset_tag",
	}
	policy := probe.DMIPolicy()
	for _, name := range mustNotRead {
		class, listed := policy[name]
		if !listed {
			t.Errorf("%q is not in the reader policy at all; its omission is "+
				"then an oversight rather than a decision", name)
			continue
		}
		if class != probe.DMIClassNotRead {
			t.Errorf("%q is %q, want %q", name, class, probe.DMIClassNotRead)
		}
	}

	// The attributes a real machine's DMI directory carries that the report
	// does not carry under their own name. A machine with two attributes
	// saying the same thing is the ordinary case, and the policy has to say
	// so: an unclassified attribute is one the sweep cannot reason about, and
	// the sweep refuses the report rather than guess.
	mustBeDuplicate := []string{"chassis_vendor", "chassis_version", "uevent"}
	for _, name := range mustBeDuplicate {
		class, listed := policy[name]
		if !listed {
			t.Errorf("%q is not in the reader policy at all; a machine that "+
				"carries it has an attribute the sweep cannot reason about", name)
			continue
		}
		if class != probe.DMIClassDuplicate {
			t.Errorf("%q is %q, want %q", name, class, probe.DMIClassDuplicate)
		}
	}
}
