package main

import (
	"testing"
)

func TestPowerSubcommand(t *testing.T) {
	if code := zenithPower([]string{"--help"}); code != exitOK {
		t.Fatalf("zenith power --help returned code %d, want %d", code, exitOK)
	}

	if code := zenithPower([]string{"status"}); code != exitOK {
		t.Fatalf("zenith power status returned code %d, want %d", code, exitOK)
	}

	if code := zenithPower([]string{"profile"}); code != exitOK {
		t.Fatalf("zenith power profile returned code %d, want %d", code, exitOK)
	}

	if code := zenithPower([]string{"profile", "balanced"}); code != exitOK {
		t.Fatalf("zenith power profile balanced returned code %d, want %d", code, exitOK)
	}

	if code := zenithPower([]string{"profile", "invalid"}); code != exitUsage {
		t.Fatalf("zenith power profile invalid returned code %d, want %d", code, exitUsage)
	}

	if code := zenithPower([]string{"unknown"}); code != exitUsage {
		t.Fatalf("zenith power unknown returned code %d, want %d", code, exitUsage)
	}
}
