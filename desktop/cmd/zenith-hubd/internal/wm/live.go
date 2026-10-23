package wm

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Hyprland's runtime layout: the instance signature, its socket, and the one way
// this package runs hyprctl.
//
// The signature rebind matters because a Hub started from a terminal, or a
// daemon that outlived its compositor, can carry a signature that names a
// session that exited days ago. hyprctl then dials a dead socket and every call
// silently does nothing. Resolving the live signature into this process fixes
// every later fork.

// ProviderHyprland is the one compositor Zenith runs. It names the settings
// domain (wm.hyprland.*) in desktop.json, so it is part of the on-disk contract.
const ProviderHyprland = "hyprland"

// ErrNoProvider is not fatal: the Hub must still open when no compositor is
// configured, so callers treat it as "no compositor features".
var ErrNoProvider = fmt.Errorf("no window manager provider available")

// ErrUnsupported tells a user something different from ErrNoProvider: the
// compositor is there, it just cannot do that.
var ErrUnsupported = fmt.Errorf("window manager does not support this action")

type Detection struct {
	Name string
	// Live means the compositor is running for this user, so actions and state
	// reads are meaningful. False means the box is configured for it and only caps
	// and apply are safe.
	Live bool
	// Source records what answered, for doctor output and bug reports.
	Source string
}

func runDir() string {
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		return filepath.Join(rt, "hypr")
	}
	return filepath.Join("/tmp", "hypr")
}

// aliveCheck is a var so tests can satisfy live() without a compositor.
var aliveCheck = socketAlive

// socketAlive dials the instance's command socket. The socket directory outlives
// the instance that made it, so the path existing proves nothing; only a dial
// does.
func socketAlive(sig string) bool {
	if sig == "" {
		return false
	}
	c, err := net.DialTimeout("unix", filepath.Join(runDir(), sig, ".socket.sock"), 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// liveSignature picks the newest instance by socket mtime, so the current login
// wins over stale leftovers. Empty when none answer.
func liveSignature() string {
	entries, err := os.ReadDir(runDir())
	if err != nil {
		return ""
	}
	best := ""
	var bestMod time.Time
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sig := e.Name()
		fi, err := os.Stat(filepath.Join(runDir(), sig, ".socket.sock"))
		if err != nil || !aliveCheck(sig) {
			continue
		}
		if best == "" || fi.ModTime().After(bestMod) {
			best, bestMod = sig, fi.ModTime()
		}
	}
	return best
}

func ensureLiveSignature() {
	if aliveCheck(os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")) {
		return
	}
	if sig := liveSignature(); sig != "" {
		_ = os.Setenv("HYPRLAND_INSTANCE_SIGNATURE", sig)
	}
}

// live lets verbs fail fast instead of hanging on a dead socket.
func live() bool {
	ensureLiveSignature()
	return aliveCheck(os.Getenv("HYPRLAND_INSTANCE_SIGNATURE"))
}

// Live reports whether a Hyprland session answers for this user.
func Live() bool { return live() }

// ctl is the single choke point for every compositor call. A var so tests can
// pin the emitted argv without a live compositor.
var ctl = runCtl

func runCtl(args ...string) ([]byte, error) {
	if _, err := exec.LookPath("hyprctl"); err != nil {
		return nil, fmt.Errorf("hyprctl is not installed")
	}
	if !live() {
		return nil, fmt.Errorf("no live Hyprland session")
	}
	cmd := exec.Command("hyprctl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("hyprctl %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return nil, fmt.Errorf("hyprctl %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

// Detect names the compositor this user runs. Zenith has one, so detection is
// only about whether it is live right now.
func Detect() Detection {
	if live() {
		return Detection{Name: ProviderHyprland, Live: true, Source: "HYPRLAND_INSTANCE_SIGNATURE"}
	}
	return Detection{Name: ProviderHyprland, Live: false, Source: "configured"}
}

// Providers lists the compositors the backend can author config for.
func Providers() []string { return []string{ProviderHyprland} }

// ConfigDir is a provider's config directory name inside ~/.config/zenith.
func ConfigDir(name string) string {
	if name == ProviderHyprland {
		return "hyprland"
	}
	return ""
}
