package wm

import (
	"net"
	"os"
	"strings"
	"time"
)

// The only place allowed to care which compositor is running. It resolves an
// identity into a provider name once; every later decision reads Caps.

// Provider names name an executable (zenith-wm-<name>) for the providers that
// are not built in, so they are part of the on-disk contract.
const ProviderHyprland = "hyprland"

// forceEnv selects a provider by name regardless of the session, so a developer
// or a test can drive one without that compositor running.
const forceEnv = "ZENITH_WM"

type Detection struct {
	Name string
	// Live means the compositor is running for this user, so actions and watch
	// are meaningful. False means the box is configured for it and only caps
	// is safe: doctor can run in a chroot right after install, where the config
	// tree exists but no compositor does.
	Live bool
	// Source records what answered, for doctor output and bug reports.
	Source string
}

// Detect resolves the active provider. Zenith is a Hyprland desktop, so the
// answer is Hyprland unless ZENITH_WM names another provider; what varies is
// only whether the compositor is live, and that is probed rather than assumed.
//
// Presence of an instance handle is not proof of life. A terminal, a tmux
// server or a user service started under one compositor keeps that
// compositor's handle in its environment for as long as it lives, so after a
// restart a stale handle names a compositor that is gone and every later action
// would be dispatched at a dead socket and silently fail. The handle has to
// answer on its socket.
func Detect() Detection {
	if forced := strings.TrimSpace(os.Getenv(forceEnv)); forced != "" {
		return Detection{Name: forced, Live: forced == ProviderHyprland && hyprlandLive(), Source: forceEnv}
	}
	if sig := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE"); sig != "" && hyprlandSocketAlive(sig) {
		return Detection{Name: ProviderHyprland, Live: true, Source: "HYPRLAND_INSTANCE_SIGNATURE"}
	}
	if hyprlandLiveSignature() != "" {
		return Detection{Name: ProviderHyprland, Live: true, Source: "runtime dir"}
	}
	return Detection{Name: ProviderHyprland, Live: false, Source: "default"}
}

// handleAlive dials a compositor socket. A refused or missing socket means the
// session it names has exited; the path surviving the instance is why only a
// dial counts.
func handleAlive(path string) bool {
	if path == "" {
		return false
	}
	c, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}
