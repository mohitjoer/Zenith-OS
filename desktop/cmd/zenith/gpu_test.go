package main

import (
	"testing"
)

func TestGpuSubcommand(t *testing.T) {
	if code := zenithGpu([]string{"--help"}); code != exitOK {
		t.Fatalf("zenith gpu --help returned code %d, want %d", code, exitOK)
	}

	if code := zenithGpu([]string{"status"}); code != exitOK {
		t.Fatalf("zenith gpu status returned code %d, want %d", code, exitOK)
	}

	if code := zenithGpu([]string{"status", "--json"}); code != exitOK {
		t.Fatalf("zenith gpu status --json returned code %d, want %d", code, exitOK)
	}

	if code := zenithGpu([]string{"nonexistent-verb"}); code != exitUsage {
		t.Fatalf("zenith gpu nonexistent-verb returned code %d, want %d", code, exitUsage)
	}
}
