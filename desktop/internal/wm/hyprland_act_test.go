package wm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// stubHyprctl replaces the hyprctl choke point with fn and makes the compositor
// read as live, so an action runs without a session. Each call is recorded as
// one space-joined line on the returned log.
func stubHyprctl(t *testing.T, fn func(args ...string) ([]byte, error)) *callLog {
	t.Helper()
	log := &callLog{}
	prevCtl, prevAlive := hyprctl, hyprlandSocketAlive
	hyprctl = func(_ context.Context, args ...string) ([]byte, error) {
		log.add(strings.Join(args, " "))
		if fn != nil {
			return fn(args...)
		}
		return []byte("ok"), nil
	}
	hyprlandSocketAlive = func(string) bool { return true }
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test-instance")
	t.Cleanup(func() { hyprctl, hyprlandSocketAlive = prevCtl, prevAlive })
	return log
}

type callLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *callLog) add(s string) {
	l.mu.Lock()
	l.lines = append(l.lines, s)
	l.mu.Unlock()
}

func (l *callLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines...)
}

func (l *callLog) reset() {
	l.mu.Lock()
	l.lines = nil
	l.mu.Unlock()
}

// Every action is pinned to the exact hyprctl argv it emits, because the dialect
// is the contract with the compositor and getting it wrong fails silently:
// hyprctl exits 0 on an invalid dispatcher. This is Hyprland 0.53, hyprlang:
// the classic "<dispatcher> <args>" form and `keyword` for live config. The Lua
// forms (hl.dsp.*, eval) of 0.55+ must never appear.
func TestActEmitsHyprlangDialect(t *testing.T) {
	const addr = "0xdeadbeef"
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"window focus", []string{"window.focus", addr},
			[]string{"dispatch focuswindow address:0xdeadbeef"}},
		{"window close", []string{"window.close", addr},
			[]string{"dispatch closewindow address:0xdeadbeef"}},
		{"window fullscreen focuses first", []string{"window.fullscreen", addr},
			[]string{"dispatch focuswindow address:0xdeadbeef", "dispatch fullscreen 0"}},
		{"window float", []string{"window.float", addr},
			[]string{"dispatch togglefloating address:0xdeadbeef"}},
		{"window to workspace", []string{"window.moveToWorkspace", addr, "3"},
			[]string{"dispatch movetoworkspace 3,address:0xdeadbeef"}},
		{"window to named workspace", []string{"window.moveToWorkspace", addr, "code"},
			[]string{"dispatch movetoworkspace name:code,address:0xdeadbeef"}},
		{"app focus", []string{"app.focus", "org.quickshell"},
			[]string{"dispatch focuswindow class:org.quickshell"}},
		{"workspace focus", []string{"workspace.focus", "9"},
			[]string{"dispatch workspace 9"}},
		{"workspace focus by name", []string{"workspace.focus", "web"},
			[]string{"dispatch workspace name:web"}},
		{"workspace focus special", []string{"workspace.focus", "special:magic"},
			[]string{"dispatch workspace special:magic"}},
		{"workspace cycle forward", []string{"workspace.cycle", "1"},
			[]string{"dispatch workspace r+1"}},
		{"workspace cycle back", []string{"workspace.cycle", "-2"},
			[]string{"dispatch workspace r-2"}},
		{"workspace to output", []string{"workspace.moveToOutput", "9", "eDP-2"},
			[]string{"dispatch moveworkspacetomonitor 9 eDP-2"}},
		{"special workspace", []string{"workspace.toggleSpecial", "sharebar"},
			[]string{"dispatch togglespecialworkspace sharebar"}},
		{"default special workspace", []string{"workspace.toggleSpecial"},
			[]string{"dispatch togglespecialworkspace"}},
		{"session exit", []string{"session.exit"},
			[]string{"dispatch exit"}},
		{"output power on", []string{"output.power", "on"},
			[]string{"dispatch dpms on"}},
		{"output power off", []string{"output.power", "off", "eDP-2"},
			[]string{"dispatch dpms off eDP-2"}},
		{"submap enter", []string{"submap.enter", "resize"},
			[]string{"dispatch submap resize"}},
		{"submap reset", []string{"submap.reset"},
			[]string{"dispatch submap reset"}},
		{"autoreload off", []string{"config.autoreload", "off"},
			[]string{"keyword misc:disable_autoreload true"}},
		{"autoreload on", []string{"config.autoreload", "on"},
			[]string{"keyword misc:disable_autoreload false"}},
		{"workspace layout", []string{"workspace.layout", "9", "master"},
			[]string{"keyword workspace 9,layout:master"}},
		{"border colours", []string{"decoration.borderColors", "#f3701e", "#0e0d0b"},
			[]string{
				"keyword general:col.active_border rgb(f3701e)",
				"keyword general:col.inactive_border rgb(0e0d0b)",
			}},
		{"border colour, one role", []string{"decoration.borderColors", "#f3701e", "not-a-colour"},
			[]string{"keyword general:col.active_border rgb(f3701e)"}},
		{"follow mouse", []string{"input.focusFollowsMouse", "2"},
			[]string{"getoption input:follow_mouse -j", "keyword input:follow_mouse 2"}},
		{"cursor", []string{"cursor.set", "Bibata-Modern-Ice", "24"},
			[]string{"setcursor Bibata-Modern-Ice 24"}},
		{"keyboard layout", []string{"keyboard.cycleLayout"},
			[]string{"switchxkblayout all next"}},
		{"reload config only", []string{"config.reload", "config-only"},
			[]string{"reload config-only"}},
		{"reload", []string{"config.reload"},
			[]string{"reload"}},
		{"game mode off reloads", []string{"decoration.gameMode", "off"},
			[]string{"reload"}},
		{"output disable", []string{"output.enable", "DP-1", "off"},
			[]string{"keyword monitor DP-1,disable"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := stubHyprctl(t, func(args ...string) ([]byte, error) {
				if args[0] == "getoption" {
					return []byte(`{"option":"input:follow_mouse","int":1,"set":false}`), nil
				}
				return []byte("ok"), nil
			})
			c := OpenNamed(ProviderHyprland)
			c.capsOK, c.caps = true, Caps{Supports: All()}
			if _, err := c.ActOutput(Action(tc.args[0]), tc.args[1:]...); err != nil {
				t.Fatalf("act %v: %v", tc.args, err)
			}
			got := log.all()
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("argv mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// No emitted command may be the Lua dialect; this is the regression Ryoku's
// provider would reintroduce if a case were copied across unconverted.
func TestActNeverEmitsLua(t *testing.T) {
	log := stubHyprctl(t, nil)
	c := OpenNamed(ProviderHyprland)
	c.capsOK, c.caps = true, Caps{Supports: All()}
	for _, args := range [][]string{
		{"window.focus", "0x1"}, {"workspace.focus", "2"}, {"output.power", "on"},
		{"submap.reset"}, {"decoration.gameMode", "on"}, {"workspace.layout", "1", "dwindle"},
		{"config.autoreload", "off"}, {"session.exit"},
	} {
		_, _ = c.ActOutput(Action(args[0]), args[1:]...)
	}
	for _, line := range log.all() {
		if strings.Contains(line, "hl.") || strings.HasPrefix(line, "eval ") {
			t.Errorf("Lua dialect reached hyprctl: %q", line)
		}
	}
}

// hyprctl exits 0 and prints the reason when the compositor refuses a request,
// so the exit status alone reports success for an action that did nothing.
func TestActSurfacesCompositorRefusal(t *testing.T) {
	stubHyprctl(t, func(...string) ([]byte, error) { return []byte("Invalid dispatcher\n"), nil })
	c := OpenNamed(ProviderHyprland)
	err := c.Act(ActionWindowFocus, "0xabc")
	if err == nil || !strings.Contains(err.Error(), "Invalid dispatcher") {
		t.Fatalf("a refused dispatch must be an error naming the reason, got %v", err)
	}
}

// An explicit on or off must be idempotent: repeating the same request emits the
// same literal, never a toggle. The resume path can re-issue an enable while the
// lock surface is still appearing, and a toggling dispatcher would flip the panel
// back off and flash the screen after wake.
func TestActOutputPowerIsIdempotent(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"on", []string{"on"}, "dispatch dpms on"},
		{"off", []string{"off"}, "dispatch dpms off"},
		{"on with monitor", []string{"on", "eDP-2"}, "dispatch dpms on eDP-2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := stubHyprctl(t, nil)
			c := OpenNamed(ProviderHyprland)
			c.capsOK, c.caps = true, Caps{Supports: All()}
			for range 2 {
				if err := c.Act(ActionOutputPower, tc.args...); err != nil {
					t.Fatal(err)
				}
			}
			got := log.all()
			if len(got) != 2 || got[0] != tc.want || got[1] != tc.want {
				t.Errorf("got %q, want two copies of %q", got, tc.want)
			}
		})
	}
}

// Re-enabling a connector must not rewrite its layout: a monitor rule names the
// whole mode, so the enable reads the current mode, position and scale back and
// writes them again. A bare "preferred, auto, 1" would snap the output to the
// default arrangement and throw away what the user set.
func TestOutputEnableKeepsLayout(t *testing.T) {
	log := stubHyprctl(t, func(args ...string) ([]byte, error) {
		if args[0] == "monitors" {
			return []byte(`[{"id":1,"name":"DP-1","width":2560,"height":1440,"refreshRate":143.912,"x":1920,"y":0,"scale":1.25}]`), nil
		}
		return []byte("ok"), nil
	})
	c := OpenNamed(ProviderHyprland)
	c.capsOK, c.caps = true, Caps{Supports: All()}
	if err := c.Act(ActionOutputEnable, "DP-1", "on"); err != nil {
		t.Fatal(err)
	}
	got := log.all()
	want := "keyword monitor DP-1,2560x1440@143.91,1920x0,1.25"
	if got[len(got)-1] != want {
		t.Errorf("enable rule = %q, want %q", got[len(got)-1], want)
	}

	// An output the compositor does not list comes up on its preferred mode.
	log.reset()
	if err := c.Act(ActionOutputEnable, "HDMI-A-1", "on"); err != nil {
		t.Fatal(err)
	}
	got = log.all()
	if got[len(got)-1] != "keyword monitor HDMI-A-1,preferred,auto,1" {
		t.Errorf("unknown output rule = %q", got[len(got)-1])
	}
}

// A missing argument must name the value, so a bad keybind is diagnosable, and
// must never reach the compositor with an empty selector (which Hyprland applies
// to the focused window).
func TestActRejectsMissingArgs(t *testing.T) {
	for _, args := range [][]string{
		{"window.focus"},
		{"window.moveToWorkspace", "0xabc"},
		{"workspace.focus"},
		{"workspace.moveToOutput", "9"},
		{"cursor.set", "Bibata"},
		{"output.power"},
		{"window.summon"},
		{"submap.enter"},
		{"app.focus", "  "},
	} {
		log := stubHyprctl(t, nil)
		c := OpenNamed(ProviderHyprland)
		c.capsOK, c.caps = true, Caps{Supports: All()}
		if err := c.Act(Action(args[0]), args[1:]...); err == nil {
			t.Errorf("act %v: expected an error", args)
		}
		for _, line := range log.all() {
			if !strings.HasPrefix(line, "clients") && !strings.HasPrefix(line, "monitors") {
				t.Errorf("act %v: reached the compositor despite a missing argument: %q", args, line)
			}
		}
	}
}

func TestActRejectsBadValues(t *testing.T) {
	for _, args := range [][]string{
		{"workspace.cycle", "next"},
		{"output.power", "maybe"},
		{"input.focusFollowsMouse", "7"},
		{"config.autoreload", ""},
		{"window.teleport", "0x1"},
	} {
		stubHyprctl(t, nil)
		c := OpenNamed(ProviderHyprland)
		c.capsOK, c.caps = true, Caps{Supports: All()}
		if err := c.Act(Action(args[0]), args[1:]...); err == nil {
			t.Errorf("act %v: expected an error", args)
		}
	}
}

// With no live compositor an action says so instead of dialing a dead socket.
func TestActWithoutSessionFailsFast(t *testing.T) {
	prev := hyprlandSocketAlive
	hyprlandSocketAlive = func(string) bool { return false }
	t.Cleanup(func() { hyprlandSocketAlive = prev })
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	called := false
	prevCtl := hyprctl
	hyprctl = func(context.Context, ...string) ([]byte, error) { called = true; return nil, nil }
	t.Cleanup(func() { hyprctl = prevCtl })

	c := OpenNamed(ProviderHyprland)
	c.capsOK, c.caps = true, Caps{Supports: All()}
	err := c.Act(ActionWindowFocus, "0x1")
	if err == nil || !strings.Contains(err.Error(), "no live Hyprland session") {
		t.Fatalf("err = %v, want a no-live-session error", err)
	}
	if called {
		t.Fatal("hyprctl ran without a session")
	}
}

// window.summon raises an already-open window to the current workspace and
// focuses it, matched by exact title. The live title wins over the initial title,
// and a title with no window is an error so the keybind launches the app instead
// of raising nothing.
func TestActSummonRaisesByTitle(t *testing.T) {
	const clients = `[
		{"address":"0xaaa","title":"Other","initialTitle":"Other"},
		{"address":"0xbbb","title":"Zenith Settings","initialTitle":"org.quickshell"},
		{"address":"0xccc","title":"stale","initialTitle":"Zenith Settings"}
	]`
	log := stubHyprctl(t, func(args ...string) ([]byte, error) {
		switch args[0] {
		case "clients":
			return []byte(clients), nil
		case "activeworkspace":
			return []byte(`{"id":5}`), nil
		}
		return []byte("ok"), nil
	})
	c := OpenNamed(ProviderHyprland)
	c.capsOK, c.caps = true, Caps{Supports: All()}

	if err := c.Act(ActionWindowSummon, "Zenith Settings"); err != nil {
		t.Fatalf("summon: %v", err)
	}
	var dispatched []string
	for _, l := range log.all() {
		if strings.HasPrefix(l, "dispatch ") {
			dispatched = append(dispatched, l)
		}
	}
	want := []string{
		"dispatch movetoworkspacesilent 5,address:0xbbb",
		"dispatch focuswindow address:0xbbb",
	}
	if strings.Join(dispatched, "\n") != strings.Join(want, "\n") {
		t.Errorf("dispatches\n got: %q\nwant: %q", dispatched, want)
	}

	log.reset()
	if err := c.Act(ActionWindowSummon, "Nonexistent"); err == nil {
		t.Error("summon of an absent title: expected an error")
	}
	for _, l := range log.all() {
		if strings.HasPrefix(l, "dispatch ") {
			t.Errorf("summon of an absent title dispatched %q", l)
		}
	}
}

// follow_mouse is a mode, not a flag: the previous value comes back so a caller
// restoring it does not overwrite a deliberate non-default.
func TestFollowMouseReportsPreviousMode(t *testing.T) {
	stubHyprctl(t, func(args ...string) ([]byte, error) {
		if args[0] == "getoption" {
			return []byte(`{"option":"input:follow_mouse","int":3,"set":true}`), nil
		}
		return []byte("ok"), nil
	})
	c := OpenNamed(ProviderHyprland)
	c.capsOK, c.caps = true, Caps{Supports: All()}
	prev, err := c.ActOutput(ActionFocusFollowsMouse, "0")
	if err != nil || prev != "3" {
		t.Fatalf("previous mode = %q, %v; want 3", prev, err)
	}
}

// A shader is a name; the file must exist, or the compositor would log a load
// failure on every frame.
func TestScreenShaderResolvesNamesToFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "zenith", "hyprland", "shaders")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "warm.glsl"), []byte("//"), 0o644); err != nil {
		t.Fatal(err)
	}
	log := stubHyprctl(t, nil)
	c := OpenNamed(ProviderHyprland)
	c.capsOK, c.caps = true, Caps{Supports: All()}

	if err := c.Act(ActionScreenShader, "warm"); err != nil {
		t.Fatal(err)
	}
	if got := log.all(); len(got) != 1 || got[0] != "keyword decoration:screen_shader "+filepath.Join(dir, "warm.glsl") {
		t.Errorf("shader keyword = %q", got)
	}

	log.reset()
	if err := c.Act(ActionScreenShader); err != nil {
		t.Fatal(err)
	}
	if got := log.all(); len(got) != 1 || got[0] != "keyword decoration:screen_shader [[EMPTY]]" {
		t.Errorf("clearing the shader must send hyprlang's empty sentinel, got %q", got)
	}

	log.reset()
	for _, bad := range []string{"missing", "../escape", "a/b"} {
		if err := c.Act(ActionScreenShader, bad); err == nil {
			t.Errorf("shader %q: expected an error", bad)
		}
	}
	if got := log.all(); len(got) != 0 {
		t.Errorf("an unknown shader reached the compositor: %q", got)
	}
}

func TestWorkspaceSelector(t *testing.T) {
	for in, want := range map[string]string{
		"3":                    "3",
		"-1337":                "-1337",
		"web":                  "name:web",
		"name:web":             "name:web",
		"special:magic":        "special:magic",
		"r+1":                  "r+1",
		"e-2":                  "e-2",
		"+1":                   "+1",
		"previous":             "previous",
		"":                     "",
		"my space":             "name:my space",
		"  4  ":                "4",
		"empty":                "empty",
		"previous_per_monitor": "previous_per_monitor",
	} {
		if got := workspaceSelector(in); got != want {
			t.Errorf("workspaceSelector(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHyprRGB(t *testing.T) {
	for in, want := range map[string]string{
		"#f3701e":  "rgb(f3701e)",
		"f3701e":   "rgb(f3701e)",
		" #ABCDEF": "rgb(ABCDEF)",
		"#fff":     "",
		"red":      "",
		"#gggggg":  "",
		"":         "",
	} {
		if got := hyprRGB(in); got != want {
			t.Errorf("hyprRGB(%q) = %q, want %q", in, got, want)
		}
	}
}

// The workspace layout the bar draws is the global one until this process
// applies a per-workspace rule, because 0.53 offers no readback for the rule.
func TestWorkspaceLayoutOverridesAreRemembered(t *testing.T) {
	layoutMu.Lock()
	layoutOverrides = map[string]string{}
	layoutMu.Unlock()
	stubHyprctl(t, func(args ...string) ([]byte, error) {
		switch args[0] {
		case "workspaces":
			return []byte(`[{"id":1,"name":"1","monitor":"DP-1","windows":1},{"id":2,"name":"2","monitor":"DP-1","windows":0}]`), nil
		case "getoption":
			return []byte(`{"option":"general:layout","str":"dwindle","set":false}`), nil
		}
		return []byte("ok"), nil
	})
	c := OpenNamed(ProviderHyprland)
	c.capsOK, c.caps = true, Caps{Supports: All()}

	ws := readWorkspaces(context.Background(), nil)
	if ws[0].Layout != "dwindle" || ws[1].Layout != "dwindle" {
		t.Fatalf("global layout not reported: %+v", ws)
	}
	if err := c.Act(ActionWorkspaceLayout, "2", "master"); err != nil {
		t.Fatal(err)
	}
	ws = readWorkspaces(context.Background(), nil)
	if ws[0].Layout != "dwindle" || ws[1].Layout != "master" {
		t.Fatalf("override not remembered per workspace: %+v", ws)
	}
}

// Helper-backed capabilities exist only while their tool does, so a control is
// hidden rather than offered and failing.
func TestCapsHelperBackedEntriesFollowTheTool(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	caps, err := hyprland{}.caps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if caps.Has(CapNightLight) || caps.Has(CapTouchpadToggle) {
		t.Fatalf("capabilities offered without their tools: %v", caps.Supports)
	}
	for _, tool := range []string{"gammastep", "zenith-touchpad"} {
		if err := os.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	caps, err = hyprland{}.caps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !caps.Has(CapNightLight) || !caps.Has(CapTouchpadToggle) {
		t.Fatalf("capabilities missing with their tools present: %v", caps.Supports)
	}
	if caps.NightLightProcess != "gammastep" {
		t.Fatalf("night light process = %q", caps.NightLightProcess)
	}
}

// Every capability the manifest claims must be one the action table can map or
// that a lane owns; and the manifest must not claim what the bridge modules or
// plugin manager this build lacks would be needed for.
func TestCapsDoNotClaimWhatIsAbsent(t *testing.T) {
	caps, err := hyprland{}.caps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []Capability{
		CapGlobalShortcuts, CapFocusGrab, CapPlugins, CapNativeOverview, CapOverviewBackdrop,
	} {
		if caps.Has(absent) {
			t.Errorf("capability %s claimed, but nothing here can honour it", absent)
		}
	}
	seen := map[Capability]bool{}
	for _, c := range caps.Supports {
		if seen[c] {
			t.Errorf("capability %s listed twice", c)
		}
		seen[c] = true
	}
	for _, f := range caps.GeneratedFiles {
		if !strings.HasPrefix(f, "zenith/hyprland/") || !strings.HasSuffix(f, ".conf") {
			t.Errorf("generated file %q is outside the Zenith compositor overlay", f)
		}
	}
}

func TestNightlightTempIsClamped(t *testing.T) {
	for in, want := range map[string]int{"": 4000, "3500": 3500, "10": 1000, "99999": 25000, "warm": 4000} {
		var args []string
		if in != "" {
			args = []string{in}
		}
		if got := nightlightTemp(args); got != want {
			t.Errorf("nightlightTemp(%q) = %d, want %d", in, got, want)
		}
	}
}
