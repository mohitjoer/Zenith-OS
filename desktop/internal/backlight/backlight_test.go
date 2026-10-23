package backlight

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

// tree builds a fake /sys under a temp dir. Every helper returns the directory
// it made so a test can link things to it.
type tree struct {
	t    *testing.T
	root string
}

func newTree(t *testing.T) tree {
	t.Helper()
	return tree{t: t, root: t.TempDir()}
}

func (f tree) sys() Sysfs { return Sysfs{Root: f.root} }

func (f tree) write(path, content string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f tree) device(class, name, kind string, cur, top int) string {
	f.t.Helper()
	dir := filepath.Join(f.root, "class", class, name)
	f.write(filepath.Join(dir, "brightness"), strconv.Itoa(cur))
	f.write(filepath.Join(dir, "max_brightness"), strconv.Itoa(top))
	if kind != "" {
		f.write(filepath.Join(dir, "type"), kind)
	}
	return dir
}

func (f tree) backlight(name string, cur, top int) string {
	return f.device(ClassBacklight, name, "raw", cur, top)
}

func (f tree) led(name string, cur, top int) string {
	return f.device(ClassLEDs, name, "", cur, top)
}

// connector makes class/drm/<name>/status.
func (f tree) connector(name, status string) string {
	f.t.Helper()
	dir := filepath.Join(f.root, "class", "drm", name)
	f.write(filepath.Join(dir, "status"), status)
	return dir
}

func (f tree) link(target, from string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(from), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Symlink(target, from); err != nil {
		f.t.Fatal(err)
	}
}

func names(devs []Device) []string {
	var out []string
	for _, d := range devs {
		out = append(out, d.Name)
	}
	return out
}

func TestDevicesAreScreensAndKeyboardBacklightsOnly(t *testing.T) {
	f := newTree(t)
	f.backlight("intel_backlight", 31944, 96000)
	f.led("asus::kbd_backlight", 1, 3)
	// Status lights live in the same class and are not brightnesses anyone wants.
	f.led("input3::capslock", 0, 1)
	f.led("phy0-led", 1, 1)
	// A device with no usable range cannot be stepped, so it is not listed.
	f.backlight("broken", 5, 0)

	got := f.sys().Devices()
	if want := []string{"intel_backlight", "asus::kbd_backlight"}; !reflect.DeepEqual(names(got), want) {
		t.Fatalf("devices = %v, want %v", names(got), want)
	}
	if got[0].Kind != KindPanel || got[0].Class != ClassBacklight || got[0].Type != "raw" {
		t.Errorf("screen = %+v", got[0])
	}
	if got[1].Kind != KindKeyboard || got[1].Class != ClassLEDs || got[1].Max != 3 {
		t.Errorf("keyboard = %+v", got[1])
	}
	if got[0].Percent() < 33.2 || got[0].Percent() > 33.3 {
		t.Errorf("percent = %v, want 33.3", got[0].Percent())
	}
}

func TestNoDevicesIsNotAnError(t *testing.T) {
	f := newTree(t)
	if got := f.sys().Devices(); len(got) != 0 {
		t.Fatalf("devices = %v, want none", names(got))
	}
	if _, ok := f.sys().Panel(); ok {
		t.Error("a desktop reported a panel")
	}
	if _, ok := f.sys().Keyboard(); ok {
		t.Error("a desktop reported a keyboard backlight")
	}
}

// The panel selection cases are the regression suite Ryoku wrote for its
// ryoku-hw-backlight, ported: each is a laptop shape that made a name-ordered
// pick land on a backlight that changes nothing.
func TestPanelIsTheBacklightOnTheConnectedConnector(t *testing.T) {
	t.Run("two native backlights, only one panel connected", func(t *testing.T) {
		f := newTree(t)
		off := f.connector("card2-eDP-0", "disconnected")
		on := f.connector("card2-eDP-2", "connected")
		f.link(off, filepath.Join(f.backlight("amdgpu_bl0", 10, 255), "device"))
		f.link(on, filepath.Join(f.backlight("amdgpu_bl2", 10, 255), "device"))

		got, ok := f.sys().Panel()
		if !ok || got.Name != "amdgpu_bl2" {
			t.Fatalf("panel = %q, want amdgpu_bl2", got.Name)
		}
	})

	t.Run("backlight on the GPU's PCI node", func(t *testing.T) {
		f := newTree(t)
		pci := filepath.Join(f.root, "devices", "pci", "gpu")
		if err := os.MkdirAll(pci, 0o755); err != nil {
			t.Fatal(err)
		}
		card := filepath.Join(f.root, "class", "drm", "card0")
		f.write(filepath.Join(card, "dev"), "226:0")
		f.link(pci, filepath.Join(card, "device"))
		f.connector("card0-eDP-1", "connected")
		f.link(pci, filepath.Join(f.backlight("nvidia_0", 10, 100), "device"))
		f.backlight("amdgpu_bl0", 10, 255)

		got, _ := f.sys().Panel()
		if got.Name != "nvidia_0" {
			t.Fatalf("panel = %q, want nvidia_0", got.Name)
		}
	})

	t.Run("no connected panel falls back to the name list", func(t *testing.T) {
		f := newTree(t)
		f.link(f.connector("card0-eDP-0", "disconnected"), filepath.Join(f.backlight("amdgpu_bl0", 10, 255), "device"))
		f.backlight("nvidia_wmi_ec_backlight", 10, 100)

		got, _ := f.sys().Panel()
		if got.Name != "amdgpu_bl0" {
			t.Fatalf("panel = %q, want amdgpu_bl0", got.Name)
		}
	})

	t.Run("no DRM tree at all", func(t *testing.T) {
		f := newTree(t)
		f.backlight("nvidia_wmi_ec_backlight", 10, 100)
		f.backlight("amdgpu_bl0", 10, 255)

		got, _ := f.sys().Panel()
		if got.Name != "amdgpu_bl0" {
			t.Fatalf("panel = %q, want amdgpu_bl0", got.Name)
		}
	})

	t.Run("the ASUS TUF F15: one Intel backlight on eDP-1", func(t *testing.T) {
		f := newTree(t)
		f.link(f.connector("card1-eDP-1", "connected"), filepath.Join(f.backlight("intel_backlight", 31944, 96000), "device"))
		f.connector("card0-HDMI-A-1", "disconnected")

		got, ok := f.sys().Panel()
		if !ok || got.Name != "intel_backlight" || got.Max != 96000 {
			t.Fatalf("panel = %+v", got)
		}
	})

	t.Run("an unlinked device with a name nobody knows still gets picked", func(t *testing.T) {
		f := newTree(t)
		f.backlight("zz_vendor", 1, 10)
		got, ok := f.sys().Panel()
		if !ok || got.Name != "zz_vendor" {
			t.Fatalf("panel = %+v", got)
		}
	})
}

func TestKeyboardPrefersTheWholeKeyboardLevel(t *testing.T) {
	f := newTree(t)
	f.led("rgb:kbd_backlight_1", 0, 255)
	f.led("rgb:kbd_backlight", 1, 255)

	got, ok := f.sys().Keyboard()
	if !ok || got.Name != "rgb:kbd_backlight" {
		t.Fatalf("keyboard = %+v", got)
	}

	// With only a zone to go on, a zone is better than nothing.
	g := newTree(t)
	g.led("rgb:kbd_backlight_1", 0, 255)
	if got, ok := g.sys().Keyboard(); !ok || got.Name != "rgb:kbd_backlight_1" {
		t.Fatalf("zone-only keyboard = %+v", got)
	}
}

func TestFindAcceptsAClassQualifiedName(t *testing.T) {
	f := newTree(t)
	f.backlight("intel_backlight", 5, 10)
	f.led("asus::kbd_backlight", 1, 3)

	for _, name := range []string{"intel_backlight", "backlight/intel_backlight"} {
		if d, ok := f.sys().Find(name); !ok || d.Name != "intel_backlight" {
			t.Errorf("Find(%q) = %+v, %v", name, d, ok)
		}
	}
	if _, ok := f.sys().Find("leds/intel_backlight"); ok {
		t.Error("a class-qualified name matched the wrong class")
	}
	if d, ok := f.sys().Find("asus::kbd_backlight"); !ok || d.Class != ClassLEDs {
		t.Errorf("keyboard by name = %+v", d)
	}
	if _, ok := f.sys().Find("nope"); ok {
		t.Error("found a device that does not exist")
	}
}

func TestExternalsAreConnectedConnectorsThatAreNotThePanel(t *testing.T) {
	f := newTree(t)
	f.connector("card1-eDP-1", "connected")
	f.connector("card1-DP-2", "connected")
	f.connector("card1-HDMI-A-1", "disconnected")
	f.connector("card1-Writeback-1", "connected")
	f.connector("card0-HDMI-A-2", "connected")
	f.write(filepath.Join(f.root, "class", "drm", "version"), "1.1")

	want := []string{"card0-HDMI-A-2", "card1-DP-2"}
	if got := f.sys().Externals(); !reflect.DeepEqual(got, want) {
		t.Fatalf("externals = %v, want %v", got, want)
	}

	g := newTree(t)
	g.connector("card1-eDP-1", "connected")
	if got := g.sys().Externals(); len(got) != 0 {
		t.Fatalf("a laptop on its own has externals %v", got)
	}
}

func TestMinRawNeverReachesZeroOnAScreen(t *testing.T) {
	cases := []struct {
		kind string
		top  int
		want int
	}{
		{KindPanel, 96000, 960},
		{KindPanel, 255, 3},
		{KindPanel, 100, 1},
		{KindPanel, 7, 1},
		{KindPanel, 1, 1},
		{KindKeyboard, 3, 0},
	}
	for _, c := range cases {
		d := Device{Kind: c.kind, Max: c.top}
		if got := d.MinRaw(); got != c.want {
			t.Errorf("%s max %d: MinRaw = %d, want %d", c.kind, c.top, got, c.want)
		}
	}
}

func TestFromPercentClampsToTheSafeRange(t *testing.T) {
	d := Device{Kind: KindPanel, Max: 96000, Brightness: 31944}
	cases := []struct {
		pct  float64
		want int
	}{
		{50, 48000},
		{100, 96000},
		{150, 96000},
		{0, 960},
		{-20, 960},
		{0.2, 960},
	}
	for _, c := range cases {
		if got := d.FromPercent(c.pct); got != c.want {
			t.Errorf("FromPercent(%v) = %d, want %d", c.pct, got, c.want)
		}
	}

	// A keyboard may be switched off, and a percentage of three levels rounds.
	k := Device{Kind: KindKeyboard, Max: 3}
	if got := k.FromPercent(0); got != 0 {
		t.Errorf("keyboard off = %d, want 0", got)
	}
	if got := k.FromPercent(50); got != 2 {
		t.Errorf("keyboard 50%% = %d, want 2", got)
	}
}

func TestSteppingIsEvenAndAlwaysMoves(t *testing.T) {
	d := Device{Kind: KindPanel, Max: 96000, Brightness: 96000}

	var path []int
	for i := 0; i < 40; i++ {
		next := d.Stepped(-5)
		if next > d.Brightness {
			t.Fatalf("a down step went up: %d -> %d", d.Brightness, next)
		}
		if next == d.Brightness {
			break
		}
		path = append(path, next)
		d.Brightness = next
	}
	if d.Brightness != d.MinRaw() {
		t.Fatalf("stepping down stopped at %d, want the floor %d", d.Brightness, d.MinRaw())
	}
	// Even perceived steps: about seventeen presses from full to the floor.
	if len(path) < 14 || len(path) > 22 {
		t.Errorf("full to floor took %d presses (%v)", len(path), path)
	}

	// The floor is a floor: another press stays put.
	if got := d.Stepped(-5); got != d.MinRaw() {
		t.Errorf("below the floor: %d", got)
	}

	var up int
	for ; up < 40; up++ {
		next := d.Stepped(5)
		if next == d.Brightness {
			break
		}
		if next < d.Brightness {
			t.Fatalf("an up step went down: %d -> %d", d.Brightness, next)
		}
		d.Brightness = next
	}
	if d.Brightness != d.Max {
		t.Fatalf("stepping up stopped at %d, want %d", d.Brightness, d.Max)
	}
	if up < 14 || up > 22 {
		t.Errorf("floor to full took %d presses", up)
	}
}

func TestSteppingOnTheCurrentLevelOfTheF15(t *testing.T) {
	// 31944/96000 is where this laptop was when the numbers were taken.
	d := Device{Kind: KindPanel, Max: 96000, Brightness: 31944}
	up, down := d.Stepped(5), d.Stepped(-5)
	if !(down < d.Brightness && d.Brightness < up) {
		t.Fatalf("steps from %d: down %d up %d", d.Brightness, down, up)
	}
	// Five perceived points is a visible step and not a lurch.
	if up > 45000 || down < 24000 {
		t.Errorf("steps are too coarse: down %d up %d", down, up)
	}
}

func TestASmallRangeStillMovesOnEveryPress(t *testing.T) {
	// acpi_video0 is often 0..7: five percent of that rounds to nothing.
	d := Device{Kind: KindPanel, Max: 7, Brightness: 7}
	seen := []int{d.Brightness}
	for i := 0; i < 20; i++ {
		next := d.Stepped(-5)
		if next == d.Brightness {
			break
		}
		d.Brightness = next
		seen = append(seen, next)
	}
	want := []int{7, 6, 5, 4, 3, 2, 1}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("path = %v, want %v", seen, want)
	}
}

func TestAKeyboardStepsByWholeLevels(t *testing.T) {
	d := Device{Kind: KindKeyboard, Max: 3, Brightness: 1}
	var seen []int
	for _, step := range []float64{1, 1, 1, -1, -1, -1, -1} {
		d.Brightness = d.Stepped(step)
		seen = append(seen, d.Brightness)
	}
	if want := []int{2, 3, 3, 2, 1, 0, 0}; !reflect.DeepEqual(seen, want) {
		t.Fatalf("levels = %v, want %v", seen, want)
	}
}

func TestParseRequest(t *testing.T) {
	good := []struct {
		in   string
		want Request
	}{
		{"40", Request{Value: 40}},
		{"40%", Request{Value: 40}},
		{" 7 ", Request{Value: 7}},
		{"0", Request{Value: 0}},
		{"100%", Request{Value: 100}},
		{"12.5%", Request{Value: 12.5}},
		{"+5", Request{Relative: true, Value: 5}},
		{"-5", Request{Relative: true, Value: -5}},
		{"+5%", Request{Relative: true, Value: 5}},
		{"-5%", Request{Relative: true, Value: -5}},
		{"5%+", Request{Relative: true, Value: 5}},
		{"5%-", Request{Relative: true, Value: -5}},
	}
	for _, c := range good {
		got, err := ParseRequest(c.in)
		if err != nil || got != c.want {
			t.Errorf("ParseRequest(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}

	for _, in := range []string{"", " ", "abc", "101", "-101", "5%%", "NaN", "inf", "+", "--5", "5%+%", "1,5"} {
		if got, err := ParseRequest(in); err == nil {
			t.Errorf("ParseRequest(%q) = %+v, want an error", in, got)
		}
	}
}

func TestTargetRoutesAbsoluteAndRelative(t *testing.T) {
	d := Device{Kind: KindPanel, Max: 1000, Brightness: 500}
	if got := d.Target(Request{Value: 25}); got != 250 {
		t.Errorf("absolute target = %d, want 250", got)
	}
	if got := d.Target(Request{Relative: true, Value: 5}); got <= 500 {
		t.Errorf("relative target = %d, want above 500", got)
	}
}
