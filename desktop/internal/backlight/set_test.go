package backlight

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

type call struct {
	dest   string
	path   dbus.ObjectPath
	method string
	args   []any
}

type fakeBus struct {
	calls  []call
	err    error
	closed bool
}

func (b *fakeBus) Call(_ context.Context, dest string, path dbus.ObjectPath, method string, args ...any) error {
	b.calls = append(b.calls, call{dest, path, method, args})
	return b.err
}

func (b *fakeBus) Close() error { b.closed = true; return nil }

func dialer(b *fakeBus) func() (Bus, error) { return func() (Bus, error) { return b, nil } }

var f15 = Device{Class: ClassBacklight, Name: "intel_backlight", Kind: KindPanel, Brightness: 31944, Max: 96000}

func TestLogindIsAskedToWriteForTheSession(t *testing.T) {
	bus := &fakeBus{}
	if err := (Logind{Dial: dialer(bus)}).Write(context.Background(), f15, 40000); err != nil {
		t.Fatal(err)
	}

	want := []call{{
		dest:   "org.freedesktop.login1",
		path:   "/org/freedesktop/login1/session/auto",
		method: "org.freedesktop.login1.Session.SetBrightness",
		// The value must be a uint32: logind's signature is "ssu", and godbus
		// would send an int as "i" and be rejected.
		args: []any{"backlight", "intel_backlight", uint32(40000)},
	}}
	if !reflect.DeepEqual(bus.calls, want) {
		t.Fatalf("calls = %#v\nwant   %#v", bus.calls, want)
	}
	if !bus.closed {
		t.Error("the bus connection was left open")
	}
}

func TestLogindWritesAKeyboardThroughTheLEDsSubsystem(t *testing.T) {
	bus := &fakeBus{}
	kbd := Device{Class: ClassLEDs, Name: "asus::kbd_backlight", Kind: KindKeyboard, Brightness: 1, Max: 3}
	if err := (Logind{Dial: dialer(bus)}).Write(context.Background(), kbd, 2); err != nil {
		t.Fatal(err)
	}
	if got := bus.calls[0].args; !reflect.DeepEqual(got, []any{"leds", "asus::kbd_backlight", uint32(2)}) {
		t.Fatalf("args = %#v", got)
	}
}

func TestLogindErrorsSaySomethingUseful(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		unavailable bool
		contains    string
	}{
		{"logind is not running", dbus.Error{Name: "org.freedesktop.DBus.Error.ServiceUnknown"}, true, "logind is not running"},
		{"no owner", dbus.Error{Name: "org.freedesktop.DBus.Error.NameHasNoOwner"}, true, "logind is not running"},
		{"not in a session", dbus.Error{Name: "org.freedesktop.login1.NoSessionForPID"}, true, "not in a login session"},
		{"wrong seat", dbus.Error{Name: "org.freedesktop.DBus.Error.AccessDenied", Body: []any{"Only owner of session may change brightness."}}, false, "refused"},
		{"anything else", errors.New("boom"), false, "boom"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (Logind{Dial: dialer(&fakeBus{err: c.err})}).Write(context.Background(), f15, 1000)
			if err == nil || !strings.Contains(err.Error(), c.contains) {
				t.Fatalf("error = %v, want it to contain %q", err, c.contains)
			}
			if errors.Is(err, ErrUnavailable) != c.unavailable {
				t.Errorf("Is(ErrUnavailable) = %v, want %v", !c.unavailable, c.unavailable)
			}
		})
	}
}

func TestLogindWithNoBusIsUnavailable(t *testing.T) {
	err := (Logind{Dial: func() (Bus, error) { return nil, errors.New("no socket") }}).Write(context.Background(), f15, 1)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

func TestDirectWritesAWritableAttribute(t *testing.T) {
	f := newTree(t)
	dir := f.backlight("intel_backlight", 100, 1000)
	d := Device{Class: ClassBacklight, Name: "intel_backlight", Kind: KindPanel, Max: 1000, Dir: dir}

	if err := (Direct{}).Write(context.Background(), d, 640); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "brightness"))
	if strings.TrimSpace(string(raw)) != "640" {
		t.Fatalf("brightness = %q, want 640", raw)
	}
}

func TestDirectOnAReadOnlyAttributeIsUnavailable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write a read-only file")
	}
	f := newTree(t)
	dir := f.backlight("intel_backlight", 100, 1000)
	if err := os.Chmod(filepath.Join(dir, "brightness"), 0o444); err != nil {
		t.Fatal(err)
	}
	d := Device{Class: ClassBacklight, Name: "intel_backlight", Kind: KindPanel, Max: 1000, Dir: dir}

	err := (Direct{}).Write(context.Background(), d, 640)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
	if !strings.Contains(err.Error(), "brightness") {
		t.Errorf("the error does not name the file: %v", err)
	}
}

// fakeBrightnessctl installs a brightnessctl that records its arguments, one per
// line, and returns the path of the record.
func fakeBrightnessctl(t *testing.T, exit int) string {
	t.Helper()
	dir := t.TempDir()
	record := filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + record + "\n"
	if exit != 0 {
		script += "echo 'Can'\"'\"'t modify brightness: Permission denied' >&2\nexit " + string(rune('0'+exit)) + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "brightnessctl"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return record
}

func TestBrightnessctlGetsAnAbsoluteRawValue(t *testing.T) {
	record := fakeBrightnessctl(t, 0)
	if err := (Brightnessctl{}).Write(context.Background(), f15, 31944); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(record)
	want := []string{"-q", "-c", "backlight", "-d", "intel_backlight", "set", "31944"}
	if got := strings.Fields(string(raw)); !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}
}

func TestBrightnessctlRefusalCarriesItsOwnMessage(t *testing.T) {
	fakeBrightnessctl(t, 1)
	err := (Brightnessctl{}).Write(context.Background(), f15, 100)
	if err == nil || !strings.Contains(err.Error(), "Permission denied") {
		t.Fatalf("error = %v", err)
	}
	if errors.Is(err, ErrUnavailable) {
		t.Error("a refusal is not unavailability: brightnessctl ran and was told no")
	}
}

func TestBrightnessctlMissingIsUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := (Brightnessctl{}).Write(context.Background(), f15, 100); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

type stub struct {
	name string
	err  error
	hits *[]string
}

func (s stub) Name() string { return s.name }
func (s stub) Write(context.Context, Device, int) error {
	*s.hits = append(*s.hits, s.name)
	return s.err
}

func TestChainStopsAtTheFirstRouteThatWorks(t *testing.T) {
	var hits []string
	chain := Chain{
		stub{"logind", errors.New("no session"), &hits},
		stub{"brightnessctl", nil, &hits},
		stub{"sysfs", nil, &hits},
	}
	route, err := chain.Write(context.Background(), f15, 1)
	if err != nil || route != "brightnessctl" {
		t.Fatalf("route = %q, err = %v", route, err)
	}
	if want := []string{"logind", "brightnessctl"}; !reflect.DeepEqual(hits, want) {
		t.Fatalf("tried %v, want %v", hits, want)
	}
}

func TestChainFailureNamesEveryRoute(t *testing.T) {
	var hits []string
	chain := Chain{
		stub{"logind", errors.New("not in a login session"), &hits},
		stub{"sysfs", errors.New("not writable"), &hits},
	}
	_, err := chain.Write(context.Background(), f15, 1)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"logind: not in a login session", "sysfs: not writable"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	if _, err := (Chain{}).Write(context.Background(), f15, 1); err == nil {
		t.Error("an empty chain reported success")
	}
}

func TestWritableByNamesTheRouteWithoutWriting(t *testing.T) {
	f := newTree(t)
	dir := f.backlight("intel_backlight", 100, 1000)
	d := Device{Class: ClassBacklight, Name: "intel_backlight", Kind: KindPanel, Max: 1000, Dir: dir}
	t.Setenv("PATH", t.TempDir()) // no brightnessctl

	if got := WritableBy(d, Chain{Logind{Dial: dialer(&fakeBus{})}, Direct{}}); got != "logind" {
		t.Errorf("with a bus: %q, want logind", got)
	}
	if got := WritableBy(d, Chain{Brightnessctl{}, Direct{}}); got != "sysfs" {
		t.Errorf("sysfs only: %q, want sysfs", got)
	}

	if os.Geteuid() != 0 {
		if err := os.Chmod(filepath.Join(dir, "brightness"), 0o444); err != nil {
			t.Fatal(err)
		}
		if got := WritableBy(d, Chain{Brightnessctl{}, Direct{}}); got != "" {
			t.Errorf("read-only and nothing else: %q, want none", got)
		}
	}

	raw, _ := os.ReadFile(filepath.Join(dir, "brightness"))
	if strings.TrimSpace(string(raw)) != "100" {
		t.Errorf("a prediction changed the brightness to %q", raw)
	}
}
