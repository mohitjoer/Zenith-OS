// Package backlight finds the screen and keyboard backlights a machine exposes,
// chooses the right panel, and works out where a brightness change should land.
//
// It exists because the obvious tool does not work on Ubuntu. The sysfs
// brightness file of a backlight is group video, the user is not in video, and
// the brightnessctl in the archive is built without logind support, so
// `brightnessctl set` is refused on every laptop. systemd-logind will write the
// value on behalf of the active session's owner, and set.go makes that call. The
// rules that decide *what* to write live here instead of in shell scripts so
// they can be tested against a fake sysfs tree.
package backlight

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The two sysfs classes a brightness lives in. They are also the subsystem names
// logind's SetBrightness accepts, and nothing else is accepted by it.
const (
	ClassBacklight = "backlight"
	ClassLEDs      = "leds"
)

// What a device lights.
const (
	KindPanel    = "panel"
	KindKeyboard = "keyboard"
)

// Gamma is the exponent of the curve relative steps are taken on.
//
// Panels are driven by a duty cycle, so equal raw steps are not equal steps to
// the eye: five percent of the range is a lurch near the bottom and invisible
// near the top. A step is therefore five points of *perceived* brightness,
// perceived = (raw/max)^(1/Gamma), which is what `brightnessctl -e` does with
// its exponent. Three is the cube-root lightness curve CIE L* approximates; it
// gives a panel about seventeen presses from full to the floor, evenly spaced.
const Gamma = 3.0

// MinPercent is the dimmest a screen is ever driven. A backlight at zero is a
// black screen with no way to see the next keypress land, and a panel with a
// small range can round one percent down to nothing, so MinRaw never returns
// less than one.
const MinPercent = 1.0

// Device is one backlight or keyboard backlight.
type Device struct {
	Class      string `json:"class"`          // ClassBacklight or ClassLEDs
	Name       string `json:"name"`           // intel_backlight, asus::kbd_backlight
	Kind       string `json:"kind"`           // KindPanel or KindKeyboard
	Type       string `json:"type,omitempty"` // raw, firmware or platform (screens only)
	Brightness int    `json:"brightness"`
	Max        int    `json:"max"`
	Dir        string `json:"-"` // the sysfs directory the values were read from
}

// File is the attribute a sysfs write goes to.
func (d Device) File() string { return filepath.Join(d.Dir, "brightness") }

// Percent is the linear level, 0..100: what a slider and the OSD show.
func (d Device) Percent() float64 {
	if d.Max <= 0 {
		return 0
	}
	return float64(d.Brightness) / float64(d.Max) * 100
}

// MinRaw is the lowest raw value this device is driven to. A keyboard may be
// switched off; a screen may not.
func (d Device) MinRaw() int {
	if d.Kind == KindKeyboard {
		return 0
	}
	return max(1, int(math.Ceil(float64(d.Max)*MinPercent/100)))
}

// Clamp keeps a raw value inside what the device accepts.
func (d Device) Clamp(raw int) int { return min(max(raw, d.MinRaw()), d.Max) }

// FromPercent turns a linear percentage into a raw value. The percentage is
// clamped first, so 150 means full and 0 means the floor rather than an error: a
// slider that overshoots by a pixel should not fail.
func (d Device) FromPercent(p float64) int {
	p = min(max(p, 0), 100)
	return d.Clamp(int(math.Round(p / 100 * float64(d.Max))))
}

// Stepped is the raw value points steps away from the current one.
//
// For a screen the points are perceived percentage points on the Gamma curve and
// the result always moves by at least one raw unit, because a panel with a small
// range (acpi_video0 is often 0..7) would otherwise swallow a five percent step
// and the key would appear dead. A keyboard backlight has a handful of levels, so
// there the points are whole levels.
func (d Device) Stepped(points float64) int {
	if d.Kind == KindKeyboard {
		return d.Clamp(d.Brightness + int(math.Round(points)))
	}
	if d.Max <= 0 {
		return d.Brightness
	}
	perceived := math.Pow(float64(d.Brightness)/float64(d.Max), 1/Gamma)*100 + points
	perceived = min(max(perceived, 0), 100)
	raw := int(math.Round(math.Pow(perceived/100, Gamma) * float64(d.Max)))
	switch {
	case points > 0 && raw <= d.Brightness:
		raw = d.Brightness + 1
	case points < 0 && raw >= d.Brightness:
		raw = d.Brightness - 1
	}
	return d.Clamp(raw)
}

// Request is a parsed brightness change.
type Request struct {
	// Relative requests are steps from the current level; absolute ones name the
	// level.
	Relative bool
	// Value is a linear percentage when absolute, and signed step points (levels,
	// for a keyboard) when relative.
	Value float64
}

// ParseRequest reads the forms the keys, the shell and brightnessctl users write:
//
//	40  40%     an absolute level
//	+5  -5      a relative step
//	5%+ 5%-     the same, in brightnessctl's spelling
//
// A bare number is absolute and a signed one is relative, because "-5" meaning
// "set to minus five" has no use and "5" meaning "step up five" would make the
// keys and the slider disagree about what a plain number is.
func ParseRequest(s string) (Request, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Request{}, errors.New("empty brightness value")
	}

	sign := 0
	switch {
	case strings.HasSuffix(s, "%+"):
		sign, s = 1, strings.TrimSuffix(s, "%+")
	case strings.HasSuffix(s, "%-"):
		sign, s = -1, strings.TrimSuffix(s, "%-")
	case strings.HasPrefix(s, "+"):
		sign, s = 1, strings.TrimPrefix(s, "+")
	case strings.HasPrefix(s, "-"):
		sign, s = -1, strings.TrimPrefix(s, "-")
	}
	s = strings.TrimSuffix(s, "%")

	n, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 100 {
		return Request{}, fmt.Errorf("%q is not a brightness: use 40, 40%%, +5 or -5", s)
	}
	if sign == 0 {
		return Request{Value: n}, nil
	}
	return Request{Relative: true, Value: float64(sign) * n}, nil
}

// Target is the raw value a request asks of this device.
func (d Device) Target(r Request) int {
	if r.Relative {
		return d.Stepped(r.Value)
	}
	return d.FromPercent(r.Value)
}

// Sysfs is a view of a /sys tree. Root is "/sys" on a machine and a fixture
// directory in a test, which is the only reason it is a field.
type Sysfs struct{ Root string }

// System is the real /sys.
func System() Sysfs { return Sysfs{Root: "/sys"} }

// Devices lists every screen backlight and keyboard backlight, screens first,
// each group in name order.
//
// Only /sys/class/leds entries named *kbd_backlight* are keyboards. The rest of
// the class is status lights (caps lock, the network port LEDs), none of which
// is a brightness anyone wants a key for.
func (s Sysfs) Devices() []Device {
	var out []Device
	out = append(out, s.read(ClassBacklight, "*", KindPanel)...)
	out = append(out, s.read(ClassLEDs, "*kbd_backlight*", KindKeyboard)...)
	return out
}

func (s Sysfs) read(class, pattern, kind string) []Device {
	dirs, _ := filepath.Glob(filepath.Join(s.Root, "class", class, pattern))
	sort.Strings(dirs)

	var out []Device
	for _, dir := range dirs {
		cur, okCur := readInt(filepath.Join(dir, "brightness"))
		top, okMax := readInt(filepath.Join(dir, "max_brightness"))
		// A device with no readable range cannot be stepped or clamped. It is
		// skipped rather than reported with a guessed maximum.
		if !okCur || !okMax || top <= 0 {
			continue
		}
		out = append(out, Device{
			Class:      class,
			Name:       filepath.Base(dir),
			Kind:       kind,
			Type:       readString(filepath.Join(dir, "type")),
			Brightness: cur,
			Max:        top,
			Dir:        dir,
		})
	}
	return out
}

// Find returns the device with this name. A name of the form class/name, such as
// backlight/intel_backlight, is accepted for the rare machine where a screen and
// a LED share one.
func (s Sysfs) Find(name string) (Device, bool) {
	class, bare, qualified := strings.Cut(name, "/")
	if !qualified {
		class, bare = "", name
	}
	for _, d := range s.Devices() {
		if d.Name == bare && (class == "" || d.Class == class) {
			return d, true
		}
	}
	return Device{}, false
}

// Panel is the backlight that drives the laptop's own screen.
//
// The first device by name is not it. A laptop with a discrete GPU registers
// several: a native one for the panel, a firmware one that changes nothing, and
// on some ASUS models an nvidia_wmi_ec_backlight beside the real one. Writes to
// the wrong one succeed and the screen does not move, which looks exactly like
// dead keys. So the choice is made on the one fact that cannot be wrong: the
// backlight whose connector is a *connected* internal panel (eDP, LVDS, DSI).
// Only when the DRM tree cannot answer does a name list decide.
func (s Sysfs) Panel() (Device, bool) {
	var panels []Device
	for _, d := range s.Devices() {
		if d.Kind == KindPanel {
			panels = append(panels, d)
		}
	}
	if len(panels) == 0 {
		return Device{}, false
	}

	for _, d := range panels {
		if s.drivesConnectedPanel(d) {
			return d, true
		}
	}

	for _, prefix := range []string{"amdgpu_bl", "intel_backlight", "acpi_video", "nvidia_wmi_ec_backlight"} {
		for _, d := range panels {
			if strings.HasPrefix(d.Name, prefix) {
				return d, true
			}
		}
	}
	return panels[0], true
}

// Keyboard is the keyboard backlight. A name that ends in kbd_backlight wins
// over one with a zone suffix (kbd_backlight_1), which is one of several RGB
// zones and not the whole-keyboard level.
func (s Sysfs) Keyboard() (Device, bool) {
	var first *Device
	devices := s.Devices()
	for i := range devices {
		d := devices[i]
		if d.Kind != KindKeyboard {
			continue
		}
		if strings.HasSuffix(d.Name, "kbd_backlight") {
			return d, true
		}
		if first == nil {
			first = &devices[i]
		}
	}
	if first == nil {
		return Device{}, false
	}
	return *first, true
}

var (
	panelConnector = regexp.MustCompile(`-(eDP|LVDS|DSI)-`)
	drmCard        = regexp.MustCompile(`^card[0-9]+$`)
)

// drivesConnectedPanel follows a backlight's device link. It takes one of two
// shapes: the link is the DRM connector itself (amdgpu and Intel, cardN-eDP-N),
// or it is the GPU's PCI node (nvidia_0), in which case the answer is whether
// the DRM card on that same PCI node has a connected panel connector.
func (s Sysfs) drivesConnectedPanel(d Device) bool {
	link, err := filepath.EvalSymlinks(filepath.Join(d.Dir, "device"))
	if err != nil {
		return false
	}
	if panelConnector.MatchString(filepath.Base(link)) {
		return readString(filepath.Join(link, "status")) == "connected"
	}

	cards, _ := filepath.Glob(filepath.Join(s.Root, "class", "drm", "card[0-9]*"))
	for _, card := range cards {
		name := filepath.Base(card)
		if !drmCard.MatchString(name) {
			continue
		}
		pci, err := filepath.EvalSymlinks(filepath.Join(card, "device"))
		if err != nil || pci != link {
			continue
		}
		if s.cardHasConnectedPanel(name) {
			return true
		}
	}
	return false
}

func (s Sysfs) cardHasConnectedPanel(card string) bool {
	for _, kind := range []string{"eDP", "LVDS", "DSI"} {
		conns, _ := filepath.Glob(filepath.Join(s.Root, "class", "drm", card+"-"+kind+"-*"))
		for _, conn := range conns {
			if readString(filepath.Join(conn, "status")) == "connected" {
				return true
			}
		}
	}
	return false
}

// Externals lists the connected DRM connectors that are not the laptop's panel,
// sorted. Whether an external monitor is plugged in is a sysfs read, and it is
// the gate in front of everything that talks to a monitor over i2c.
func (s Sysfs) Externals() []string {
	conns, _ := filepath.Glob(filepath.Join(s.Root, "class", "drm", "*-*"))
	var out []string
	for _, conn := range conns {
		name := filepath.Base(conn)
		if !strings.HasPrefix(name, "card") || strings.Contains(name, "Writeback") || panelConnector.MatchString(name) {
			continue
		}
		if readString(filepath.Join(conn, "status")) == "connected" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func readString(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func readInt(path string) (int, bool) {
	n, err := strconv.Atoi(readString(path))
	return n, err == nil
}
