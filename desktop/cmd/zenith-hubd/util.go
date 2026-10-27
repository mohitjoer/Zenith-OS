package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// Small helpers that the passthrough, voxtype and rice-publish files used to
// define; the verbs that stay still need them.

func configHome() string {
	if b := os.Getenv("XDG_CONFIG_HOME"); b != "" {
		return b
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func ttyRun(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func selfExe() string {
	if e, err := os.Executable(); err == nil {
		return e
	}
	return "zenith-hubd"
}

// escalateSelf re-runs this binary as root via pkexec, preserving the invoking
// user's id (PKEXEC_UID) and HOME so the privileged half can access the
// caller's config (e.g. gpu-presets.json). Without HOME pkexec clears the
// environment and defaults HOME to /root, which breaks custom preset loading.
func escalateSelf(args ...string) error {
	uid := strconv.Itoa(os.Getuid())
	home := os.Getenv("HOME")
	full := append([]string{"env", "HOME=" + home, "PKEXEC_UID=" + uid, selfExe()}, args...)
	cmd := exec.Command("pkexec", full...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return cmd.Run()
}

// run is best-effort: one missing tool never aborts the rest of a reset.
func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	_ = cmd.Run()
}

// unavailable answers a verb whose backend Zenith does not have. A read prints
// {"available":false,"reason":...} and succeeds, so the page can show why; a
// verb that would change something fails, so nothing reads it as done.
func unavailable(verb, reason string, args []string) error {
	for _, a := range args[:min(len(args), 2)] {
		if a == "get" || a == "caps" || a == "list" || a == "status" {
			return printJSON(map[string]any{"available": false, "reason": reason})
		}
	}
	if len(args) == 0 {
		return printJSON(map[string]any{"available": false, "reason": reason})
	}
	return fmt.Errorf("%s: not available: %s", verb, reason)
}

func dataHome() string {
	if b := os.Getenv("XDG_DATA_HOME"); b != "" {
		return b
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
