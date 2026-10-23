package main

import (
	"regexp"
	"strings"
	"testing"
)

// TestDoctorContractIpcTargets checks that every `zenith ipc call <target>` in
// the compositor configuration names a target the shell actually exposes.
//
// The shell registers three IPC handlers: "zenith" (the legacy keybind bridge),
// "shell" (the daemon-facing surface channel), and "qsbar" (the quick-settings
// bar surface). A bind that calls anything else succeeds at the command level
// but reaches no listener, so the key does nothing.
//
// The test parses the live hyprland.conf, extracts every call, and fails with
// the offending line when a target is outside the allowlist.
func TestDoctorContractIpcTargets(t *testing.T) {
	root := repoRoot(t)
	config := compositorConfig(t, root)

	const pattern = `zenith ipc call (\S+)`
	re := regexp.MustCompile(pattern)

	validTargets := map[string]bool{
		"zenith": true,
		"shell":  true,
		"qsbar":  true,
	}

	lines := strings.Split(config, "\n")
	for i, line := range lines {
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		target := matches[1]
		if !validTargets[target] {
			t.Errorf("line %d: zenith ipc call target %q is not a registered handler; valid targets are zenith, shell, qsbar", i+1, target)
		}
	}
}

// TestDoctorContractFlagsInvalidTarget is the negative case: a configuration
// containing a call to an unknown target must be flagged.
//
// This uses a temporary config snippet rather than the live file so the test
// cannot pass vacuously if the live file happens to be clean.
func TestDoctorContractFlagsInvalidTarget(t *testing.T) {
	const badConfig = `
# Valid line
bind = SUPER, Space, exec, zenith ipc call zenith togglePopover launcher
# Invalid line — "nosuch" is not a registered handler
bind = SUPER, X, exec, zenith ipc call nosuch doSomething
`

	const pattern = `zenith ipc call (\S+)`
	re := regexp.MustCompile(pattern)

	validTargets := map[string]bool{
		"zenith": true,
		"shell":  true,
		"qsbar":  true,
	}

	lines := strings.Split(badConfig, "\n")
	foundInvalid := false
	for i, line := range lines {
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		target := matches[1]
		if !validTargets[target] {
			foundInvalid = true
			if target != "nosuch" {
				t.Errorf("line %d: expected invalid target \"nosuch\", got %q", i+1, target)
			}
		}
	}
	if !foundInvalid {
		t.Fatal("the test config contains an invalid target but the checker did not flag it")
	}
}

// TestDoctorContractLiveConfigHasExpectedCalls ensures the live configuration
// still contains the CLI binds the session depends on, all targeting "zenith".
// This is a regression guard against accidental removal of them.
func TestDoctorContractLiveConfigHasExpectedCalls(t *testing.T) {
	root := repoRoot(t)
	config := compositorConfig(t, root)

	for _, call := range []string{
		"zenith ipc call zenith togglePopover launcher",
		"zenith ipc call zenith toggleCheatsheet",
		"zenith ipc call zenith toggleOverview",
		"zenith ipc call zenith advanceSwitcher",
		"zenith ipc call zenith closeSwitcher",
	} {
		if !strings.Contains(config, call) {
			t.Errorf("the live config does not call %q", call)
		}
	}

	const pattern = `zenith ipc call (\S+) ?(\S*)`
	for _, m := range regexp.MustCompile(pattern).FindAllStringSubmatch(config, -1) {
		if m[1] != "zenith" {
			t.Errorf("bind calls target %q; the live config's chords all target \"zenith\"", m[1])
		}
	}
}
