package main

import (
	"testing"
)

func TestInstallSubcommand(t *testing.T) {
	cmd, taken := extraCommands["install"]
	if !taken {
		t.Fatalf("asura install command not registered")
	}

	if code := cmd.run([]string{"--help"}); code != exitOK {
		t.Errorf("asura install --help returned %d, want %d", code, exitOK)
	}
}
