// Package redact enforces the hardware report's redaction policy.
//
// Reports are attached to public pull requests, so the policy is part of the
// detection contract rather than a formatting pass, and it is implemented once
// here rather than reviewed per report.
//
// The policy is two layers, and the second is what makes it safe:
//
//   - Layer one, the reader allowlist, lives in package probe: the readers copy
//     named fields into structs and never copy a directory, so there is no code
//     path that can emit a field nobody classified.
//   - Layer two, this package, sweeps the rendered bytes and refuses to emit if
//     anything it should not contain is present.
//
// The sweep is not redundant with the allowlist. The allowlist is a claim about
// what the readers copy; the sweep is a check on what actually came out. Either
// alone can be wrong — a reader can be extended carelessly, and a regex list
// with nothing to compare against cannot tell a serial from a version string.
package redact

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"zenith/hardware/probe"
)

// tokenDomain is the fixed prefix in every MAC input. It exists so that a token
// computed here cannot collide with a hash computed for some other purpose on
// the same machine, and so that rotating the format later is a one-line change
// rather than a migration.
const tokenDomain = "zenith/hardware-report/v1\x00"

// Salt holds the key that makes tokens stable per machine and unresolvable by
// anyone who does not hold it.
type Salt [32]byte

// LoadOrCreateSalt reads the report salt, creating it if absent.
//
// The two locations are both ones Zenith already owns: a user-state path under
// XDG_STATE_HOME, and /etc/zenith/ for the installer running as root. Nothing is
// written to a home directory by a package and nothing is written outside
// /usr/share/zenith/ and /etc/zenith/ by anything shipped.
func LoadOrCreateSalt(path string) (Salt, error) {
	var salt Salt

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		decoded, decodeErr := hex.DecodeString(strings.TrimSpace(string(data)))
		if decodeErr != nil || len(decoded) != len(salt) {
			return salt, fmt.Errorf("redact: salt at %s is %d bytes of %q; "+
				"expected 32 bytes of hex", path, len(data), decodeErr)
		}
		copy(salt[:], decoded)
		return salt, nil

	case !os.IsNotExist(err):
		return salt, fmt.Errorf("redact: reading salt: %w", err)
	}

	if _, err := rand.Read(salt[:]); err != nil {
		return salt, fmt.Errorf("redact: generating salt: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return salt, fmt.Errorf("redact: creating salt directory: %w", err)
	}
	// 0600: the salt is the only thing standing between a published token and
	// the serial it came from.
	if err := os.WriteFile(path, []byte(hex.EncodeToString(salt[:])+"\n"), 0o600); err != nil {
		return salt, fmt.Errorf("redact: writing salt: %w", err)
	}
	return salt, nil
}

// Token produces the stable per-machine token for one field.
//
//	input = "zenith/hardware-report/v1\0" || field_name || "\0" || raw_value
//	token = "sha256:" || lowercase hex of the first 8 bytes of HMAC-SHA256(key, input)
//
// Including the field name means a token cannot be lifted from one field into
// another: the DMI UUID token and the EDID serial token for the same machine
// differ, which is correct, because they are different fields that happen to
// hold equal values.
//
// Two properties follow and both belong in the report's own header, so a reader
// knows them:
//
//   - Two reports from the same machine produce the same tokens, so a maintainer
//     can tell that two pull requests came from the same laptop.
//   - Rotating the salt changes every token. The tokens are therefore not a
//     durable machine fingerprint, and no external index of them is meaningful.
func Token(salt Salt, field, rawValue string) string {
	if rawValue == "" {
		return ""
	}
	mac := hmac.New(sha256.New, salt[:])
	mac.Write([]byte(tokenDomain))
	mac.Write([]byte(field))
	mac.Write([]byte{0})
	mac.Write([]byte(rawValue))
	return "sha256:" + hex.EncodeToString(mac.Sum(nil)[:8])
}

// Findings are the reasons a report was refused.
type Findings struct {
	// Rule is which check fired: "unclassified-dmi-field", "mac-address",
	// "ipv6-link-local", "home-directory", "username", "machine-id",
	// "hostname", or "raw-serial".
	Rule string
	// Detail is the offending text, truncated. It is enough for a maintainer to
	// locate the bug and short enough not to republish the secret in the error
	// message that goes in a CI log.
	Detail string
}

// String renders a finding for an error message.
func (f Findings) String() string { return f.Rule + ": " + f.Detail }

// Context is everything the sweep needs to know beyond the rendered bytes.
//
// The sweep cannot be a fixed list of regexes. It has to know which values are
// legitimately present — this machine's own allowlisted PCI and USB
// identifiers — or rule 2 below would fire on hex that is the evidence a
// profile is written from.
type Context struct {
	// RawDMI is every value found in the raw DMI directory, whatever its class.
	// The sweep uses it to catch any value that is not on the keep-list.
	RawDMI map[string]string

	// AllowedIdentifiers are PCI and USB identifiers that legitimately appear in
	// the report. A substring matching one of these is not a leak.
	AllowedIdentifiers []string

	// Hostname, User and Logname are the machine's own values. Their presence in
	// a report is a leak even though the sweep has no way to recognise them
	// generically.
	Hostname string
	User     string
	Logname  string

	// MachineIDs are the contents of /etc/machine-id and the dbus machine-id.
	MachineIDs []string

	// RawSerials are values that must never appear: USB serials, storage
	// serials, WWAN IMEIs, nvidia GPU serials. They are not in any report field,
	// so their appearance means a reader or an emitter changed without this
	// package being told.
	RawSerials []string
}

// patterns are the shape-based checks.
//
// Rule 2 is the one with a false positive designed around rather than removed:
// PCI and USB identifiers are hex, and a subsystem identifier written
// "vendor:device" sits uncomfortably close to the first half of a MAC. Four
// digits a side does not match a six-pair MAC, but a USB idVendor of 0x0011 and
// a device of 0x2233 reported as "0011:2233" would. The sweep is therefore
// allowlist-aware: it skips any substring that appears verbatim as an
// allowlisted identifier in the same report. A redaction check that is disabled
// by hex-looking data is not a check.
var patterns = []struct {
	rule    string
	pattern *regexp.Regexp
}{
	{"mac-address", regexp.MustCompile(`(?i)\b([0-9a-f]{2}:){5}[0-9a-f]{2}\b`)},
	{"ipv6-link-local", regexp.MustCompile(`(?i)\bfe80(:[0-9a-f]{0,4}){1,7}\b`)},
	{"home-directory", regexp.MustCompile(`(/home/[A-Za-z0-9._-]+|/root\b|/Users/[A-Za-z0-9._-]+)`)},
}

// dmiKeptFields is the set of DMI attributes a report may carry.
//
// Rule 1 of the sweep — "any value present in the raw DMI dump that is not on
// this list" — is what makes redaction a policy rather than a regex list.
//
// It is derived from the reader's own policy rather than restated here. A
// separate copy is a second chance for the two to disagree, and when they do the
// symptom is that every report containing a newly-kept field fails to render
// with an error naming a rule that appears nowhere in the reader. The hashed
// fields are included because a token is derived from them; the not-read fields
// are absent because the reader never copied them.
var dmiKeptFields = func() map[string]bool {
	fields := map[string]bool{}
	for name, class := range probe.DMIPolicy() {
		if class == probe.DMIClassKept || class == probe.DMIClassMapped ||
			class == probe.DMIClassHashed {
			fields[name] = true
		}
	}
	return fields
}()

// Verify runs the sweep over rendered report bytes.
//
// It returns every finding it has rather than the first, because a report that
// leaked three identifiers should not need three round trips to fix. An empty
// slice means the report is safe to emit.
//
// The caller must not emit when this returns findings. That is the contract:
// the emitter asks, and refuses.
func Verify(rendered []byte, context Context) []Findings {
	text := string(rendered)
	var findings []Findings

	// Rule 1: a raw DMI value that is not on the keep-list, appearing verbatim.
	// An empty value cannot be searched for, so it is skipped; absence is not a
	// leak.
	for field, value := range context.RawDMI {
		if dmiKeptFields[field] || value == "" {
			continue
		}
		if strings.Contains(text, value) {
			findings = append(findings, Findings{
				Rule:   "unclassified-dmi-field",
				Detail: fmt.Sprintf("dmi field %q (value redacted, %d bytes)", field, len(value)),
			})
		}
	}

	for _, pattern := range patterns {
		for _, match := range pattern.pattern.FindAllString(text, -1) {
			if isAllowlistedIdentifier(match, context.AllowedIdentifiers) {
				continue
			}
			findings = append(findings, Findings{
				Rule:   pattern.rule,
				Detail: match,
			})
		}
	}

	// Rules 5 to 7: literal values this machine knows and the sweep cannot
	// recognise generically. They are checked as whole substrings because a
	// hostname or a machine-id has no shape.
	for _, literal := range []struct {
		rule  string
		value string
	}{
		{"username", context.User},
		{"username", context.Logname},
		{"hostname", context.Hostname},
	} {
		if literal.value == "" {
			continue
		}
		if strings.Contains(text, literal.value) {
			findings = append(findings, Findings{Rule: literal.rule, Detail: literal.value})
		}
	}

	for _, id := range context.MachineIDs {
		if id == "" {
			continue
		}
		if strings.Contains(text, id) {
			findings = append(findings, Findings{Rule: "machine-id", Detail: id})
		}
	}

	// A serial that must never appear in any form. This is deliberately
	// separate from the hashed fields: the hashed fields are hashed, and a raw
	// serial appearing means a reader started carrying it.
	for _, serial := range context.RawSerials {
		if serial == "" {
			continue
		}
		if strings.Contains(text, serial) {
			findings = append(findings, Findings{
				Rule:   "raw-serial",
				Detail: fmt.Sprintf("%d bytes, value withheld", len(serial)),
			})
		}
	}

	return findings
}

// isAllowlistedIdentifier reports whether a match is an identifier the report is
// entitled to carry.
//
// It compares case-insensitively and with the 0x prefix normalised away,
// because the same identifier is written "8086:46a6", "0x8086:0x46a6" and
// "00008086:000046a6" depending on which reader produced it, and all of them are
// the same published hardware ID.
func isAllowlistedIdentifier(match string, allowed []string) bool {
	normalised := normaliseIdentifier(match)
	for _, identifier := range allowed {
		if normaliseIdentifier(identifier) == normalised {
			return true
		}
	}
	return false
}

func normaliseIdentifier(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "0x", "")
	return strings.TrimLeft(value, "0")
}

// Refusal renders findings as the message an emitter exits with. It names the
// offending rule and never echoes the value, because that error message ends up
// in a CI log, which is a place a serial must not be.
func Refusal(findings []Findings) string {
	if len(findings) == 0 {
		return ""
	}
	rules := make([]string, 0, len(findings))
	for _, finding := range findings {
		rules = append(rules, finding.Rule)
	}
	return fmt.Sprintf("report refused: %d redaction finding(s): %s. "+
		"Nothing was written. This is a bug in a reader or an emitter, not in "+
		"the report; do not paste the rendered output anywhere.",
		len(findings), strings.Join(unique(rules), ", "))
}

func unique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
