package wm

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// sandbox keeps a test off the real config tree and off the real compositor.
func sandbox(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	old := aliveCheck
	aliveCheck = func(string) bool { return false }
	t.Cleanup(func() { aliveCheck = old })
	return dir
}

// verify runs the compositor's own parser over a generated file, which is the
// only check that the file is one Hyprland 0.53 accepts. Skipped where Hyprland
// is not installed.
func verify(t *testing.T, path string) {
	t.Helper()
	bin, err := exec.LookPath("Hyprland")
	if err != nil {
		t.Skip("Hyprland is not installed; cannot verify the generated config")
	}
	out, _ := exec.Command(bin, "--verify-config", "-c", path).CombinedOutput()
	if !strings.Contains(string(out), "config ok") {
		body, _ := os.ReadFile(path)
		idx := strings.Index(string(out), "Config parsing result")
		t.Fatalf("Hyprland rejected %s:\n%s\n--- file ---\n%s", path, string(out)[max(idx, 0):], body)
	}
}

// perturb moves every bool, int and float in a struct off its default, so the
// generator has to emit every option it knows.
func perturb(v reflect.Value) {
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		switch f.Kind() {
		case reflect.Bool:
			f.SetBool(!f.Bool())
		case reflect.Int:
			f.SetInt(f.Int() + 1)
		case reflect.Float64:
			f.SetFloat(f.Float() + 0.1)
		}
	}
}

func everything() Overrides {
	o := defaultOverrides()
	perturb(reflect.ValueOf(&o.Appearance).Elem())
	perturb(reflect.ValueOf(&o.Dwindle).Elem())
	perturb(reflect.ValueOf(&o.Master).Elem())
	perturb(reflect.ValueOf(&o.Input).Elem())
	perturb(reflect.ValueOf(&o.Cursor).Elem())
	o.inputSaved = true
	o.Appearance.Layout = "master"
	o.Appearance.GlowEnabled = false // Hyprland 0.53 has no glow; the store keeps the field
	o.Appearance.WindowStyle = "slide"
	o.Appearance.WobblyWindows = true
	o.Appearance.AnimatedBorder = true
	o.Appearance.BorderFollowsPalette = false
	o.Dwindle.ForceSplit = "left/top"
	o.Master.NewStatus, o.Master.Orientation = "master", "top"
	o.Input.AccelProfile, o.Input.KbLayout, o.Input.KbOptions = "flat", "de", "caps:escape"
	o.Input.WorkspaceSwipe = true
	o.Cursor.Theme = "DMZ-White"
	o.Env = []EnvVar{{Key: "FOO", Value: "a#b"}, {Key: "BAD KEY", Value: "x"}}
	o.Apps = map[string]string{"browser": "firefox", "terminal": "ghostty"}
	o.Autostart = []Autostart{{Command: "nm-applet --indicator"}}
	o.WindowRules = []WindowRule{
		{Class: "^(a|b){1,2}$", Title: "^(x, y)$", Action: "float"},
		{Class: "mpv", Action: "idleinhibit", Value: "fullscreen"},
		{Class: "kitty", Action: "size", Value: "800x600"},
		{Class: "kitty", Action: "opacity", Value: "0.9"},
		{Class: "kitty", Action: "norounding"},
		{Class: "kitty", Action: "noblur"},
		{Class: "kitty", Action: "workspace", Value: "3"},
	}
	o.LayerRules = []LayerRule{{Namespace: "quickshell:overview", Action: "blur"}, {Namespace: "x", Action: "ignorealpha", Value: "0.3"}}
	o.AppOverrides = []AppOverride{{Class: "firefox", Opacity: 0.95, Rounding: 6, BorderSize: -1, Blur: "off"}}
	o.Keybinds = []Keybind{
		{Keys: "SUPER + SHIFT + X", Action: "exec", Value: "notify-send, hi"},
		{Keys: "SUPER + KP_1", Action: "close"},
		{Keys: "SUPER + Y", Action: "exec", Value: "true", Release: true},
	}
	o.Unbinds = []string{"SUPER + Z"}
	o.Anim = Anim{
		Curves: []AnimCurve{{Name: "mine", X0: 0.1, Y0: 0.2, X1: 0.3, Y1: 0.4}},
		Items:  []AnimItem{{Leaf: "windows", Enabled: true, Speed: 4, Bezier: "mine", Style: "popin 80%"}, {Leaf: "fade", Enabled: false}},
	}
	return o
}

func TestGeneratedSettingsAreValidHyprlang(t *testing.T) {
	dir := sandbox(t)
	if err := os.MkdirAll(filepath.Join(dir, "zenith"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(AnimPresetPath(), []byte("bounce\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	layout := `[{"name":"eDP-1","enabled":true,"mode":"1920x1080@144","scale":1.25,"x":0,"y":0,"vrr":true},
	            {"name":"HDMI-A-1","enabled":true,"scale":1,"x":1920,"y":0,"mirror":"eDP-1","colorMode":"hdr","sdrBrightness":1.2},
	            {"name":"DP-1","enabled":false}]`
	if err := os.WriteFile(AppliedLayoutPath(), []byte(layout), 0o644); err != nil {
		t.Fatal(err)
	}

	o := everything()
	got := genSettings(o, false)
	file := filepath.Join(dir, "settings.conf")
	if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"gaps_in = 13", "tap-to-click", "env = FOO,a##b", "windowrule {", "name = zenith-user-1",
		"match:title = ^(x, y)$", "layerrule {", "bezier = mine, 0.1, 0.2, 0.3, 0.4",
		"bindr = SUPER, Y, exec, true", "bind = SUPER, KP_End, killactive,", "unbind = SUPER, Z",
		"exec-once = nm-applet --indicator", "monitor = eDP-1, 1920x1080@144, 0x0, 1.25, vrr, 1",
		"monitor = DP-1, disable", "animation = windowsIn, 1, 6, jelly, popin 60%",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("settings.conf lacks %q", want)
		}
	}
	if strings.Contains(got, "glow") || strings.Contains(got, "BAD KEY") {
		t.Errorf("settings.conf carries a line Hyprland 0.53 rejects:\n%s", got)
	}
	verify(t, file)
}

func TestScaleSnapsToWhatHyprlandAccepts(t *testing.T) {
	// 1.3 is not a multiple of 1/120 that divides 1920x1080 into whole pixels;
	// 1.3333 (160/120) is, and is the nearest.
	if s := snapScale(1920, 1080, 1.3); s != 1.3333 {
		t.Errorf("snapScale(1920,1080,1.3) = %v, want 1.3333", s)
	}
	if s := snapScale(1920, 1080, 1.25); s != 1.25 {
		t.Errorf("snapScale(1920,1080,1.25) = %v, want 1.25", s)
	}
}

// A rebind is an unbind of the shipped chord plus the same bind on the new one,
// read from the packaged config; a family keeps its ten members.
func TestRebindsUnbindAndRebind(t *testing.T) {
	dir := sandbox(t)
	base := filepath.Join(dir, "hyprland.conf")
	conf := "$mod = SUPER\n" +
		"bind = $mod, Q, killactive,\n" +
		"bind = SUPER, Tab, exec, zenith ipc call zenith advanceSwitcher\n" +
		"bindr = SUPER, Tab, exec, zenith ipc call zenith closeSwitcher\n" +
		"source = ~/.config/zenith/hyprland/rebinds.conf\n"
	for n := 1; n <= 10; n++ {
		conf += "bind = SUPER, " + string(rune('0'+n%10)) + ", workspace, " + itoa(n) + "\n"
	}
	if err := os.WriteFile(base, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := filepath.Join(dir, "binds.json")
	catalogue := `{"categories":[{"id":"w","label":"Windows"}],"binds":[
	 {"id":"close","category":"w","label":"Close","chord":"SUPER + Q","kind":"wm","action":"killactive"},
	 {"id":"switcher","category":"w","label":"Switch","chord":"SUPER + Tab","kind":"shell","action":"advanceSwitcher","releaseAction":"closeSwitcher"},
	 {"id":"ws","category":"w","label":"Workspace","chord":"SUPER + {n}","kind":"wm","action":"workspace"}]}`
	if err := os.WriteFile(cat, []byte(catalogue), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZENITH_BINDS_JSON", cat)
	t.Setenv("ZENITH_HYPRLAND_CONFIG", base)

	o := defaultOverrides()
	o.KeybindRebinds = map[string]string{
		"SUPER + Q":   "SUPER + W",
		"SUPER + Tab": "ALT + Tab",
		"SUPER + {n}": "SUPER + CTRL + {n}",
	}
	got := string(renderRebinds(o))
	for _, want := range []string{
		"unbind = SUPER, Q\nbind = SUPER, W, killactive,",
		"unbind = SUPER, Tab\nbind = ALT, Tab, exec, zenith ipc call zenith advanceSwitcher\nbindr = ALT, Tab, exec, zenith ipc call zenith closeSwitcher",
		"unbind = SUPER, 0\nbind = SUPER CTRL, 0, workspace, 10",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rebinds.conf lacks:\n%s\n--- got ---\n%s", want, got)
		}
	}
	if n := strings.Count(got, "unbind = SUPER, "); n != 12 {
		t.Errorf("want 12 unbinds (Q, Tab, ten workspaces), got %d", n)
	}
	file := filepath.Join(dir, "rebinds.conf")
	if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
		t.Fatal(err)
	}
	verify(t, file)
}

func itoa(n int) string {
	if n == 10 {
		return "10"
	}
	return string(rune('0' + n))
}
