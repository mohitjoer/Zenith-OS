package wm

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func newTestWatchState(emit func(Frame)) *watchState {
	return newWatchState(context.Background(), emit, func(FrameKind) bool { return true })
}

func TestMonitorOutputsUsesLiveModeForDisabledState(t *testing.T) {
	outputs := monitorOutputs([]hyprMonitor{
		{Name: "eDP-1", Width: 2560, Height: 1600, Disabled: true},
		{Name: "DP-1", Width: 0, Height: 0, Disabled: false},
	}, false)
	if outputs[0].Disabled {
		t.Fatal("live output inherited Hyprland's stale disabled flag")
	}
	if !outputs[1].Disabled {
		t.Fatal("output without a live mode was reported enabled")
	}
}

func TestFocusEventsDoNotQueryAndPreserveHistory(t *testing.T) {
	stubHyprctl(t, func(...string) ([]byte, error) {
		t.Fatal("focus queried compositor")
		return nil, nil
	})
	var frames []Frame
	s := newTestWatchState(func(f Frame) { frames = append(frames, f) })
	s.windows = []Window{{ID: "0xa", FocusOrder: 0}, {ID: "0xb", FocusOrder: 1}, {ID: "0xc", FocusOrder: 2}}
	s.publish(Frame{Kind: FrameWindows, Windows: s.windows})
	if s.event("activewindow>>kitty,a title, with commas") != 0 {
		t.Fatal("v1 refreshed")
	}
	if s.event("activewindowv2>>b") != 0 {
		t.Fatal("known focus refreshed")
	}
	if got := []int{s.windows[0].FocusOrder, s.windows[1].FocusOrder, s.windows[2].FocusOrder}; !reflect.DeepEqual(got, []int{1, 0, 2}) {
		t.Fatal(got)
	}
	if frames[0].Windows[0].FocusOrder != 0 {
		t.Fatal("mutated prior frame")
	}
	s.event("activewindowv2>>0xb")
	if len(frames) != 2 {
		t.Fatal("duplicate focus published")
	}
	s.event("activewindowv2>>")
	for _, w := range s.windows {
		if w.FocusOrder == 0 {
			t.Fatal("empty focus retained focused window")
		}
	}
	s.event("activewindowv2>>")
	if len(frames) != 3 {
		t.Fatal("duplicate empty focus published")
	}
	if s.event("activewindowv2>>dead") != refreshWindows {
		t.Fatal("unknown window needs refresh")
	}
}

func TestWindowOpenQueriesOnlyClientsAndWorkspaces(t *testing.T) {
	var calls []string
	stubHyprctl(t, func(args ...string) ([]byte, error) {
		calls = append(calls, args[0])
		switch args[0] {
		case "workspaces":
			return []byte(`[{"id":1,"name":"1","monitor":"DP-1","windows":1}]`), nil
		case "clients":
			return []byte(`[{"address":"0xa","class":"kitty","monitor":0,"workspace":{"id":1,"name":"1"},"focusHistoryID":0}]`), nil
		case "getoption":
			return []byte(`{"option":"general:layout","str":"dwindle","set":false}`), nil
		default:
			t.Fatalf("unexpected query %v", args)
			return nil, nil
		}
	})
	var frames []Frame
	s := newTestWatchState(func(f Frame) { frames = append(frames, f) })
	s.monitors = []hyprMonitor{{ID: 0, Name: "DP-1"}}
	s.refresh(s.event("openwindow>>a,1,kitty,title") | s.event("activewindowv2>>a"))
	if !reflect.DeepEqual(calls, []string{"workspaces", "getoption", "clients"}) {
		t.Fatal(calls)
	}
	if len(frames) != 2 || frames[1].Windows[0].Output != "DP-1" {
		t.Fatal(frames)
	}
	s.refresh(refreshWindows | refreshWorkspaces)
	if len(frames) != 2 {
		t.Fatal("unchanged state republished")
	}
}

func TestKeyboardAndTopologyRefresh(t *testing.T) {
	var calls []string
	stubHyprctl(t, func(args ...string) ([]byte, error) {
		calls = append(calls, args[0])
		if args[0] == "devices" {
			return []byte(`{"keyboards":[{"main":true,"active_keymap":"English (UK)","layout":"gb,us"}]}`), nil
		}
		return []byte(`[]`), nil
	})
	var frames []Frame
	s := newTestWatchState(func(f Frame) { frames = append(frames, f) })
	s.refresh(s.event("activelayout>>keyboard,English (UK)"))
	if !reflect.DeepEqual(calls, []string{"devices"}) || len(frames) != 1 || frames[0].KeyboardLayout != "English (UK)" {
		t.Fatal(calls, frames)
	}
	for _, event := range []string{"monitoraddedv2>>1,DP-1,test", "monitorremoved>>DP-1", "configreloaded>>"} {
		if s.event(event) != refreshAll {
			t.Fatal(event)
		}
	}
	if s.event("activewindowgarbage>>abc") != 0 {
		t.Fatal("prefix collision")
	}
}

// Hyprland reports the transient "ERROR" active_keymap while a keyboard's xkb
// state is mid-transition (a virtual device flapping), and no further
// activelayout event follows once it settles. Publishing it would latch the bar
// on "ERROR" forever, so a refresh that resolves only to the sentinel must
// publish nothing and leave the last-good layout standing.
func TestKeyboardErrorSentinelIsNotPublished(t *testing.T) {
	stubHyprctl(t, func(args ...string) ([]byte, error) {
		return []byte(`{"keyboards":[{"main":true,"active_keymap":"ERROR","layout":"us"}]}`), nil
	})
	var frames []Frame
	s := newTestWatchState(func(f Frame) { frames = append(frames, f) })
	s.refresh(s.event("activelayout>>keyboard,ERROR"))
	if len(frames) != 0 {
		t.Fatalf("ERROR sentinel published: %+v", frames)
	}
}

// With no keyboard flagged main the indicator still gets a layout from a device
// that has one, instead of staying blank for the session.
func TestKeyboardFallsBackWhenNoneIsMain(t *testing.T) {
	stubHyprctl(t, func(args ...string) ([]byte, error) {
		return []byte(`{"keyboards":[{"main":false,"active_keymap":"ERROR","layout":"us"},{"main":false,"active_keymap":"English (US)","layout":"us,de"}]}`), nil
	})
	active, all := readKeyboard(context.Background())
	if active != "English (US)" || !reflect.DeepEqual(all, []string{"us", "de"}) {
		t.Fatalf("readKeyboard = %q %v", active, all)
	}
}

// Real `hyprctl -j` output from Hyprland 0.53.3, trimmed to the keys read.
func TestSnapshotFromHyprland053(t *testing.T) {
	stubHyprctl(t, func(args ...string) ([]byte, error) {
		switch args[0] {
		case "monitors":
			return []byte(`[{"id":0,"name":"eDP-1","make":"Najing CEC Panda FPD Technology CO. ltd","model":"0x004D","width":1920,"height":1080,"physicalWidth":340,"refreshRate":144.00000,"x":0,"y":0,"activeWorkspace":{"id":1,"name":"1"},"scale":1.00,"transform":0,"focused":true,"vrr":false,"disabled":false,"mirrorOf":"none","availableModes":["1920x1080@144.00Hz","1920x1080@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.00}]`), nil
		case "workspaces":
			return []byte(`[{"id":1,"name":"1","monitor":"eDP-1","monitorID":0,"windows":1,"hasfullscreen":false},{"id":-99,"name":"special:scratch","monitor":"eDP-1","windows":0,"hasfullscreen":false}]`), nil
		case "clients":
			return []byte(`[{"address":"0x634f4be86b50","class":"firefox","title":"t","monitor":0,"floating":false,"at":[0,39],"size":[1920,987],"workspace":{"id":1,"name":"1"},"focusHistoryID":0}]`), nil
		case "devices":
			return []byte(`{"keyboards":[{"name":"at-translated-set-2-keyboard","main":true,"active_keymap":"English (US)","layout":"us"}]}`), nil
		case "getoption":
			return []byte(`{"option":"general:layout","str":"dwindle","set":false}`), nil
		}
		return []byte("ok"), nil
	})
	snap, err := hyprland{}.state(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.FocusedOutput != "eDP-1" || len(snap.Outputs) != 1 {
		t.Fatalf("outputs: %+v", snap)
	}
	o := snap.Outputs[0]
	if o.Mode != "1920x1080@144.00" || o.Mirror != "" || o.ColorMode != "srgb" || len(o.Modes) != 2 {
		t.Fatalf("output detail: %+v", o)
	}
	if len(snap.Workspaces) != 2 || !snap.Workspaces[0].Active || snap.Workspaces[0].Special || !snap.Workspaces[1].Special {
		t.Fatalf("workspaces: %+v", snap.Workspaces)
	}
	if snap.Workspaces[0].Layout != "dwindle" {
		t.Fatalf("layout: %q", snap.Workspaces[0].Layout)
	}
	if len(snap.Windows) != 1 || snap.Windows[0].Output != "eDP-1" || snap.Windows[0].Width != 1920 || snap.Windows[0].AppID != "firefox" {
		t.Fatalf("windows: %+v", snap.Windows)
	}
	if snap.KeyboardLayout != "English (US)" {
		t.Fatalf("keyboard: %q", snap.KeyboardLayout)
	}
}

func TestSpecialWorkspaceRange(t *testing.T) {
	for _, tc := range []struct {
		id   int
		name string
		want bool
	}{
		{1, "1", false},
		{-1, "", false},
		{-1337, "named", false}, // name-based workspaces are negative too
		{-99, "special:scratch", true},
		{-2, "", true},
		{5, "special:x", true},
	} {
		if got := isSpecialWorkspace(tc.id, tc.name); got != tc.want {
			t.Errorf("isSpecialWorkspace(%d, %q) = %v, want %v", tc.id, tc.name, got, tc.want)
		}
	}
}

func TestParseFocusedMonIsExact(t *testing.T) {
	if mon, ok := parseFocusedMon("focusedmon>>DP-1,2"); !ok || mon != "DP-1" {
		t.Fatalf("focusedmon: %q %v", mon, ok)
	}
	if _, ok := parseFocusedMon("focusedmonv2>>DP-1,2"); ok {
		t.Fatal("focusedmonv2 must not match: a Hyprland that drops v1 should fail loudly")
	}
	if name, ok := parseMonitorRemoved("monitorremoved>>HDMI-A-1"); !ok || name != "HDMI-A-1" {
		t.Fatalf("monitorremoved: %q %v", name, ok)
	}
}

// A watch that was asked for outputs only must not make the compositor answer
// for windows on every window event.
func TestNarrowedWatchSkipsUnwantedReads(t *testing.T) {
	var calls []string
	stubHyprctl(t, func(args ...string) ([]byte, error) {
		calls = append(calls, args[0])
		return []byte(`[]`), nil
	})
	var frames []Frame
	s := newWatchState(context.Background(), func(f Frame) { frames = append(frames, f) },
		func(k FrameKind) bool { return k == FrameOutputs })
	s.refresh(refreshAll)
	for _, c := range calls {
		if c != "monitors" {
			t.Errorf("narrowed watch queried %q", c)
		}
	}
}

// The signature rebind is what keeps a daemon restarted from a stale
// environment talking to the live compositor instead of a dead socket.
func TestEnsureSignatureRebindsToLiveInstance(t *testing.T) {
	rt := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", rt)
	for _, sig := range []string{"stale", "live"} {
		if err := os.MkdirAll(filepath.Join(rt, "hypr", sig), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rt, "hypr", sig, ".socket.sock"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prev := hyprlandSocketAlive
	hyprlandSocketAlive = func(sig string) bool { return sig == "live" }
	t.Cleanup(func() { hyprlandSocketAlive = prev })
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "stale")

	ensureHyprlandSignature()
	if got := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE"); got != "live" {
		t.Fatalf("signature = %q, want live", got)
	}
	if !strings.HasSuffix(hyprlandEventSocket(), "hypr/live/.socket2.sock") {
		t.Fatalf("event socket = %q", hyprlandEventSocket())
	}
}

// TestLiveHyprland reads the running compositor through the real provider:
// capabilities, one state snapshot and the first frames of a watch. Read-only,
// and gated so the default `go test` needs no session.
func TestLiveHyprland(t *testing.T) {
	if os.Getenv("ZENITH_LIVE_HYPRLAND") == "" {
		t.Skip("set ZENITH_LIVE_HYPRLAND=1 to read the running Hyprland")
	}
	c := Open()
	if !c.Detection().Live {
		t.Skip("no live Hyprland session")
	}
	caps, err := c.Caps()
	if err != nil || caps.Name != ProviderHyprland || caps.Instance == "" || caps.Version == "" {
		t.Fatalf("caps: %+v, %v", caps, err)
	}
	snap, err := c.State()
	if err != nil || snap.FocusedOutput == "" || len(snap.Outputs) == 0 {
		t.Fatalf("state: %+v, %v", snap, err)
	}
	t.Logf("hyprland %s: focused %s, %d output(s), %d workspace(s), %d window(s), keyboard %q",
		caps.Version, snap.FocusedOutput, len(snap.Outputs), len(snap.Workspaces), len(snap.Windows), snap.KeyboardLayout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ready := false
	kinds := map[FrameKind]bool{}
	_ = c.Watch(ctx, func(f Frame) {
		kinds[f.Kind] = true
		ready = ready || f.Kind == FrameReady
	})
	if !ready || !kinds[FrameOutputs] || !kinds[FrameWorkspaces] || !kinds[FrameWindows] {
		t.Fatalf("watch delivered %v, want ready plus outputs, workspaces and windows", kinds)
	}
}
