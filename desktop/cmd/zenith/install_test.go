package main

import (
	"testing"
)

func TestInstallSubcommand(t *testing.T) {
	// Running without arguments in test environment where disk isn't provided
	// should attempt to run the script or print usage
	if _, taken := extraCommands["install"]; !taken {
		t.Fatalf("zenith install command not registered")
	}
}
