package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/sys/unix"

	"zenith/desktop/internal/backlight"
)

func init() {
	register("brightness", "screen and keyboard backlight, with no group to join", zenithBrightness)
}

const brightnessUsage = `Usage: zenith brightness <command> [--device panel|kbd|NAME]

  get                 the level, as a percentage
  set VALUE           40 or 40% to go there; +5, -5, 5%+ or 5%- to step
  up [N], down [N]    step N points (default 5; a keyboard steps whole levels)
  name                the backlight the device resolves to
  list                every backlight, and the route a change would take

--device is panel (the laptop's screen, the default), kbd (the keyboard
backlight), or the name of a /sys/class/backlight or /sys/class/leds entry.

A screen never goes below 1%. Steps are even to the eye rather than to the duty
cycle: five points of perceived brightness, so a key feels the same at 90% and at
5%. A number with no sign is a level and a number with one is a step.

The change is written through systemd-logind, which needs no privileges for the
person at the machine, then brightnessctl, then the sysfs file if it is writable.
`

// brightnessEnv is what the command touches besides its arguments, so a test can
// run it against a fake /sys with no bus.
type brightnessEnv struct {
	Sys   backlight.Sysfs
	Chain backlight.Chain
	Out   io.Writer
	Err   io.Writer
	// State holds the lock that serialises key repeat and the stamps that keep a
	// failing key from filling the journal.
	State string
}

func zenithBrightness(args []string) int {
	sys := backlight.System()
	if root := os.Getenv("ZENITH_SYSFS_ROOT"); root != "" {
		sys = backlight.Sysfs{Root: root}
	}
	state := filepath.Join(os.TempDir(), fmt.Sprintf("zenith-%d", os.Getuid()))
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		state = filepath.Join(rt, "zenith")
	}
	return runBrightness(args, brightnessEnv{
		Sys: sys, Chain: backlight.DefaultChain(), Out: os.Stdout, Err: os.Stderr, State: state,
	})
}

func runBrightness(args []string, e brightnessEnv) int {
	// The standard flag package cannot be taught that "-5" is a value: it sees a
	// flag, and the dim key is the one that has to work.
	var verb, device string
	var values []string
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "-h" || arg == "--help" || arg == "help":
			fmt.Fprint(e.Out, brightnessUsage)
			return exitOK
		case arg == "--device" || arg == "-d":
			if i++; i >= len(args) {
				fmt.Fprintln(e.Err, "zenith brightness: --device needs a name")
				return exitUsage
			}
			device = args[i]
		case strings.HasPrefix(arg, "--device="):
			device = strings.TrimPrefix(arg, "--device=")
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(e.Err, "zenith brightness: unknown flag %s\n", arg)
			return exitUsage
		case verb == "":
			verb = arg
		default:
			values = append(values, arg)
		}
	}
	if device == "keyboard" {
		device = "kbd"
	}

	switch verb {
	case "get", "name":
		d, ok := e.pick(device)
		if !ok {
			return exitFail
		}
		if verb == "name" {
			fmt.Fprintln(e.Out, d.Name)
		} else {
			fmt.Fprintln(e.Out, percentLabel(d))
		}
		return exitOK
	case "list":
		return e.list()
	case "set":
		if len(values) != 1 {
			fmt.Fprintln(e.Err, "zenith brightness set: needs one value, such as 40% or +5")
			return exitUsage
		}
		req, err := backlight.ParseRequest(values[0])
		if err != nil {
			fmt.Fprintln(e.Err, "zenith brightness set:", err)
			return exitUsage
		}
		return e.change(device, req, false)
	case "up", "down":
		sign := 1.0
		if verb == "down" {
			sign = -1
		}
		switch len(values) {
		case 0:
			// The size is the device's usual one, which is not known until the
			// device is: five points for a screen, one level for a keyboard.
			return e.change(device, backlight.Request{Relative: true, Value: sign}, true)
		case 1:
			n, err := strconv.ParseFloat(strings.TrimSuffix(values[0], "%"), 64)
			if err == nil && n > 0 && n <= 100 {
				return e.change(device, backlight.Request{Relative: true, Value: sign * n}, false)
			}
		}
		fmt.Fprintf(e.Err, "zenith brightness %s: takes one step size, such as 10\n", verb)
		return exitUsage
	case "":
		fmt.Fprint(e.Err, brightnessUsage)
	default:
		fmt.Fprintf(e.Err, "zenith brightness: unknown command %q\n\n%s", verb, brightnessUsage)
	}
	return exitUsage
}

// pick finds the one device a --device value names, and says why when it cannot.
func (e brightnessEnv) pick(name string) (backlight.Device, bool) {
	var (
		d    backlight.Device
		ok   bool
		what = "no screen backlight on this machine"
	)
	switch name {
	case "", "panel":
		d, ok = e.Sys.Panel()
	case "kbd":
		d, ok = e.Sys.Keyboard()
		what = "no keyboard backlight on this machine"
	default:
		d, ok = e.Sys.Find(name)
		what = fmt.Sprintf("no backlight named %q (`zenith brightness list` shows them)", name)
	}
	if !ok {
		fmt.Fprintln(e.Err, "zenith brightness:", what)
	}
	return d, ok
}

// percentLabel is the whole-number level. A keyboard backlight with three levels
// reads 33, 67 and 100, which is what a slider wants.
func percentLabel(d backlight.Device) string {
	return strconv.Itoa(int(math.Round(d.Percent())))
}

// change applies a request to one device and prints the level it ended at.
// byDefault means the request carries only a direction.
func (e brightnessEnv) change(device string, req backlight.Request, byDefault bool) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// A held brightness key fires faster than a process starts, so without this
	// two presses read the same level and both write one step from it. It waits a
	// second at most and then carries on: a lost step is a smaller fault than a
	// key that does nothing.
	defer e.serialise()()

	d, ok := e.pick(device)
	if !ok {
		return exitFail
	}
	if byDefault {
		step := 5.0
		if d.Kind == backlight.KindKeyboard {
			step = 1
		}
		req.Value = math.Copysign(step, req.Value)
	}

	raw := d.Target(req)
	if _, err := e.Chain.Write(ctx, d, raw); err != nil {
		e.report(d.Name, "could not set %s to %d of %d: %v\n  `zenith brightness list` shows which route each device has", d.Name, raw, d.Max, err)
		return exitFail
	}
	if fresh, ok := e.Sys.Find(d.Class + "/" + d.Name); ok {
		d = fresh
	}
	fmt.Fprintln(e.Out, percentLabel(d))
	return exitOK
}

func (e brightnessEnv) list() int {
	devices := e.Sys.Devices()
	if len(devices) == 0 {
		fmt.Fprintln(e.Out, "no backlight on this machine")
		return exitOK
	}
	panel, _ := e.Sys.Panel()
	kbd, _ := e.Sys.Keyboard()

	w := tabwriter.NewWriter(e.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "\tNAME\tKIND\tLEVEL\tWRITE")
	for _, d := range devices {
		mark := " "
		if d.Name == panel.Name || d.Name == kbd.Name {
			mark = "*"
		}
		level := fmt.Sprintf("%d%%", int(math.Round(d.Percent())))
		if d.Kind == backlight.KindKeyboard {
			level = fmt.Sprintf("%d of %d", d.Brightness, d.Max)
		}
		route := backlight.WritableBy(d, e.Chain)
		if route == "" {
			route = "no route: not in a login session, and the sysfs file is not writable"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", mark, d.Name, d.Kind, level, route)
	}
	w.Flush()
	return exitOK
}

// serialise takes the key-repeat lock and returns the function that drops it.
func (e brightnessEnv) serialise() func() {
	if e.State == "" || os.MkdirAll(e.State, 0o700) != nil {
		return func() {}
	}
	f, err := os.OpenFile(filepath.Join(e.State, "brightness.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return func() {}
	}
	for deadline := time.Now().Add(time.Second); unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil; {
		if time.Now().After(deadline) {
			f.Close()
			return func() {}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return func() { f.Close() }
}

// report prints a failure to stderr, at most once every half minute per device.
// The stderr of a keybind lands in the compositor's log, and a held key that
// cannot work would otherwise write a line per repeat; the exit status still
// says it every time.
func (e brightnessEnv) report(device, format string, args ...any) {
	if e.State != "" && os.MkdirAll(e.State, 0o700) == nil {
		stamp := filepath.Join(e.State, "brightness-"+strings.NewReplacer("/", "_", ":", "_").Replace(device)+".stamp")
		now := time.Now().Unix()
		if raw, err := os.ReadFile(stamp); err == nil {
			if last, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64); err == nil && now-last < 30 {
				return
			}
		}
		_ = os.WriteFile(stamp, []byte(strconv.FormatInt(now, 10)), 0o600)
	}
	fmt.Fprintf(e.Err, "zenith brightness: "+format+"\n", args...)
}
