package wm

import (
	"os"
	"testing"
)

func TestDetectDefaultsToHyprland(t *testing.T) {
	t.Setenv("ZENITH_WM", "")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	d := Detect()
	if d.Name != ProviderHyprland || d.Live {
		t.Fatalf("no session: %+v, want hyprland, not live", d)
	}
}

func TestDetectForcedProviderIsNotLive(t *testing.T) {
	t.Setenv("ZENITH_WM", "testwm")
	d := Detect()
	if d.Name != "testwm" || d.Live || d.Source != "ZENITH_WM" {
		t.Fatalf("forced: %+v", d)
	}
}

// A handle that names a dead instance is not proof of life: only a dial is.
func TestDetectIgnoresStaleHandle(t *testing.T) {
	t.Setenv("ZENITH_WM", "")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "long-gone")
	if d := Detect(); d.Live {
		t.Fatalf("a stale signature read as live: %+v", d)
	}
}

func TestDetectFindsLiveInstanceInRuntimeDir(t *testing.T) {
	t.Setenv("ZENITH_WM", "")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	rt := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", rt)
	if err := os.MkdirAll(rt+"/hypr/abc", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rt+"/hypr/abc/.socket.sock", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	prev := hyprlandSocketAlive
	hyprlandSocketAlive = func(sig string) bool { return sig == "abc" }
	t.Cleanup(func() { hyprlandSocketAlive = prev })
	if d := Detect(); !d.Live || d.Source != "runtime dir" {
		t.Fatalf("detect: %+v", d)
	}
}

func TestOpenNamedUnknownHasNoProviderBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if OpenNamed("does-not-exist").Available() {
		t.Fatal("a provider with no executable read as available")
	}
	if _, err := OpenNamed("").Caps(); err != ErrNoProvider {
		t.Fatalf("empty name: %v, want ErrNoProvider", err)
	}
}
