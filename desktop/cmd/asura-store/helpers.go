package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func have(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// runProbe runs a read-only tool with the locale pinned.
func runProbe(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8", "LANGUAGE=")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			err = fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(ee.Stderr)))
		}
	}
	return out, err
}

func emitStage(stage string, percent int) {
	fmt.Printf("{\"stage\":%q,\"percent\":%d}\n", stage, percent)
}
