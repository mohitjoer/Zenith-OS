package main

import "testing"

func TestZenithRoleUsage(t *testing.T) {
	code := zenithRole([]string{"--help"})
	if code != exitOK {
		t.Fatalf("zenith role --help returned code %d, want %d", code, exitOK)
	}

	code = zenithRole([]string{"status"})
	if code != exitOK {
		t.Fatalf("zenith role status returned code %d, want %d", code, exitOK)
	}

	code = zenithRole([]string{"invalid-role"})
	if code != exitUsage {
		t.Fatalf("zenith role invalid-role returned code %d, want %d", code, exitUsage)
	}
}
