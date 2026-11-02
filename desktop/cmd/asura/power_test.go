package main

import (
	"testing"
)

func TestPowerSubcommand(t *testing.T) {
	if code := asuraPower([]string{"--help"}); code != exitOK {
		t.Fatalf("asura power --help returned code %d, want %d", code, exitOK)
	}

	if code := asuraPower([]string{"status"}); code != exitOK {
		t.Fatalf("asura power status returned code %d, want %d", code, exitOK)
	}

	if code := asuraPower([]string{"profile"}); code != exitOK {
		t.Fatalf("asura power profile returned code %d, want %d", code, exitOK)
	}

	if code := asuraPower([]string{"profile", "balanced"}); code != exitOK {
		t.Fatalf("asura power profile balanced returned code %d, want %d", code, exitOK)
	}

	if code := asuraPower([]string{"profile", "invalid"}); code != exitUsage {
		t.Fatalf("asura power profile invalid returned code %d, want %d", code, exitUsage)
	}

	if code := asuraPower([]string{"unknown"}); code != exitUsage {
		t.Fatalf("asura power unknown returned code %d, want %d", code, exitUsage)
	}
}
