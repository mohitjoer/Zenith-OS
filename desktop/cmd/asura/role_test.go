package main

import "testing"

func TestAsuraRoleUsage(t *testing.T) {
	code := asuraRole([]string{"--help"})
	if code != exitOK {
		t.Fatalf("asura role --help returned code %d, want %d", code, exitOK)
	}

	code = asuraRole([]string{"status"})
	if code != exitOK {
		t.Fatalf("asura role status returned code %d, want %d", code, exitOK)
	}

	code = asuraRole([]string{"invalid-role"})
	if code != exitUsage {
		t.Fatalf("asura role invalid-role returned code %d, want %d", code, exitUsage)
	}
}
