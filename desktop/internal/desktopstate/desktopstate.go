// Package desktopstate reads the desktop's live state for the sidebar.
//
// It exists as a Go package rather than as QML for one reason: parsing these
// five tools' output is the hard part, and this is where the tests are. Five
// untestable QML parsers would be strictly worse than one tested command the
// shell renders.
//
// Every command here has a documented trap, and each one is a field below rather
// than a comment:
//
//   - wpctl's volume is 0..1+ and is NOT clamped. A sink can report 1.53, so a
//     control that renders value*100 draws "153" on a slider whose maximum is
//     100.
//   - nmcli -g escapes a literal colon inside a value as \:. A MAC address comes
//     back as "CC\:42\:10\:65\:BE\:FB" and a naive split on ":" produces six
//     fields and a wrong answer that looks plausible.
//   - bluetoothctl prints its errors to stdout and exits 0. `devices` on a
//     machine with no adapter exits 0, prints "No default controller available",
//     and looks like success to anything watching the exit code.
//   - bluetoothctl blocks forever and silently when bluetoothd is not running.
//     Every probe here has a deadline; there is no way to ask for one from the
//     tool itself, because its --timeout flag is a floor, not a ceiling.
//   - playerctl has no JSON and no -p ALL. Its two output modes format the same
//     float differently, so exactly one form is used here.
//
// Nothing is a hard failure. A machine with no bluetooth adapter reports no
// adapter; it does not report an error, because a sidebar full of error panels
// is a sidebar nobody turns on.
package desktopstate

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// probeTimeout is the deadline for every command here.
//
// Two seconds is generous for nmcli (measured at ~20ms warm, ~15ms to report a
// dead NetworkManager) and short enough that a hung bluetoothctl is a missing
// section rather than a frozen shell. The tools cannot be asked for a ceiling,
// so this is the only ceiling there is.
const probeTimeout = 2 * time.Second

// State is everything the sidebar draws.
type State struct {
	Audio     Audio     `json:"audio"`
	Network   Network   `json:"network"`
	Bluetooth Bluetooth `json:"bluetooth"`
	Display   Display   `json:"display"`
	Media     Media     `json:"media"`
}

// Audio is the sound state.
type Audio struct {
	// Available is false when PipeWire is not reachable, which is the only
	// reason the section is hidden.
	Available bool `json:"available"`
	// Volume is clamped to 0..1 even though wpctl does not clamp it.
	Volume float64 `json:"volume"`
	// Percent is Volume as a whole number, for display.
	Percent int  `json:"percent"`
	Muted   bool `json:"muted"`
	// Sink is the default sink's description, for the label.
	Sink string `json:"sink"`
}

// Network is the connection state.
type Network struct {
	// Available is false when NetworkManager is not running, which is exit 8.
	Available bool `json:"available"`
	// WifiOn reflects the radio, which is independent of whether a connection is
	// up: a machine with the radio on and no network is a normal state and not an
	// error.
	WifiOn bool `json:"wifi_on"`
	// Devices is the device list, already parsed.
	Devices []Device `json:"devices"`
}

// Device is one network interface.
type Device struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	State string `json:"state"`
}

// Bluetooth is the adapter state.
type Bluetooth struct {
	// Present is false when the machine has no adapter at all.
	//
	// It is detected with `bluetoothctl list`, which exits 0 with completely
	// empty stdout in that case. Nothing else about this tool's behaviour is
	// reliable: `devices` exits 0 while printing an error, and the whole tool
	// aborts with SIGABRT when the bus is unreachable.
	Present     bool              `json:"present"`
	Powered     bool              `json:"powered"`
	Discovering bool              `json:"discovering"`
	Devices     []BluetoothDevice `json:"devices"`
}

// BluetoothDevice is one paired device.
type BluetoothDevice struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	// Connected reflects the Connected: line from `bluetoothctl info`.
	Connected bool `json:"connected"`
}

// Display is the backlight state.
type Display struct {
	// Available is false when there is no backlight device, which is a desktop
	// machine and not a fault.
	Available bool `json:"available"`
	// Percent is 0..100.
	Percent int `json:"percent"`
	// Writable is false on a stock image, and the sidebar shows the level without
	// a slider. Writing /sys/class/backlight/*/brightness needs a udev rule
	// Zenith has not laid down; see docs/architecture.md.
	Writable bool `json:"writable"`
}

// Media is the MPRIS player state.
type Media struct {
	Available bool    `json:"available"`
	Player    string  `json:"player"`
	Status    string  `json:"status"`
	Title     string  `json:"title"`
	Artist    string  `json:"artist"`
	Volume    float64 `json:"volume"`
}

// Collect reads everything.
//
// The probes are independent and none of them aborts the others: a machine with
// no bluetooth adapter still gets its network state. That is the whole reason
// this returns a State and not an error.
func Collect() State {
	var state State
	state.Audio = audio()
	state.Network = network()
	state.Bluetooth = bluetooth()
	state.Display = display()
	state.Media = media()
	return state
}

// run executes a command with a deadline and returns its stdout.
//
// exit is the process exit status, or -1 when the command was killed for
// running too long or could not be started at all. Both of those are failures
// and neither needs telling apart: a caller that wants to know why a tool
// failed has to read stderr, and the sidebar shows the state, not the errors.
//
// -1 rather than 0 is the whole design of this function. Every caller asks
// `exit != 0`, and a timed-out bluetoothctl that returned 0 with empty stdout
// is the exact failure this deadline exists to stop.
func run(name string, args ...string) (out string, exit int) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	// LC_ALL=C on every command. Three of these tools are gettext-translated,
	// and a parser that works on the developer's machine and not the user's is
	// the bug this line prevents. TERM=dumb stops anything deciding to page.
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(cmd.Environ(), "LC_ALL=C", "LANG=C", "TERM=dumb")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", -1
	}

	if err == nil {
		return stdout.String(), 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		// The binary is missing, which is the same as the feature being absent.
		// Treating it as a distinct error would make a machine without
		// bluetoothctl permanently broken.
		return "", -1
	}
	return stdout.String(), exitErr.ExitCode()
}

// audio reads the volume and the default sink.
//
// `get-volume` is the only terse form wpctl has and it is not localised —
// verified byte-identical under LC_ALL=C and under a deliberately absent
// locale. The output is one of two lines and nothing else.
func audio() Audio {
	raw, exit := run("wpctl", "get-volume", "@DEFAULT_AUDIO_SINK@")
	if exit != 0 {
		return Audio{}
	}

	volume, muted, ok := parseVolume(raw)
	if !ok {
		return Audio{}
	}
	state := Audio{
		Available: true,
		Volume:    volume,
		Percent:   int(volume*100 + 0.5),
		Muted:     muted,
	}

	// The sink name is one line from status; failure here is not worth a second
	// error path, because an unlabelled volume control is still correct.
	if status, exit := run("wpctl", "status"); exit == 0 {
		state.Sink = parseDefaultSink(status)
	}
	return state
}

// parseVolume reads `wpctl get-volume` output.
//
// The clamp is the reason this is a function. wpctl does not clamp, and a sink
// can legitimately report 1.53 — measured, not assumed. Every consumer treats
// the number as 0..1, so it is clamped once here rather than in each of them.
func parseVolume(raw string) (volume float64, muted, ok bool) {
	fields := strings.Fields(raw)
	if len(fields) < 2 || fields[0] != "Volume:" {
		return 0, false, false
	}

	parsed, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, false, false
	}
	if parsed < 0 {
		parsed = 0
	}
	if parsed > 1 {
		parsed = 1
	}
	return parsed, strings.Contains(raw, "[MUTED]"), true
}

// parseDefaultSink pulls the default sink's node name out of `wpctl status`.
//
// The tree is box-drawing characters and right-aligned columns. It is parsed
// here only because wpctl offers no alternative, and only for the one line that
// has a stable shape: the last "Audio/Sink" in the Default Configured Devices
// block. A failure costs a label, which is why this returns "" rather than an
// error.
func parseDefaultSink(status string) string {
	lines := strings.Split(status, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "Audio/Sink") {
			fields := strings.Fields(lines[i])
			if len(fields) >= 2 {
				return fields[len(fields)-1]
			}
		}
	}
	return ""
}

// network reads the device list and the wifi radio.
//
// nmcli is gettext-translated, so -g and LC_ALL=C are both required. "No
// connection" is not an error: a machine with nothing connected exits 0 and the
// sidebar shows every device as disconnected.
func network() Network {
	state := Network{}

	// Exit 8 is NetworkManager not running, in about 15ms. That is the whole
	// availability test.
	raw, exit := run("nmcli", "-g", "DEVICE,TYPE,STATE", "device", "status")
	// Any failure means NetworkManager is not usable, including exit 8 (not
	// running) and exit 127 (no nmcli). Reporting "available" with no devices
	// because a probe failed is how a sidebar starts lying.
	if exit != 0 {
		return state
	}
	state.Available = true

	state.Devices = parseDevices(raw)

	// "enabled:enabled:missing:disabled" — WIFI-HW:WIFI:WWAN-HW:WWAN
	if radio, exit := run("nmcli", "-g", "WIFI-HW,WIFI", "radio", "all"); exit == 0 {
		state.WifiOn = strings.HasPrefix(strings.TrimSpace(radio), "enabled:enabled")
	}
	return state
}

// parseDevices reads `nmcli -g DEVICE,TYPE,STATE device status`.
//
// It always returns a non-nil slice: JSON `null` where a list belongs is a
// different thing to a caller than an empty list, and one that costs a check at
// every use site.
func parseDevices(raw string) []Device {
	devices := []Device{}

	for _, line := range strings.Split(raw, "\n") {
		parts := splitUnescaped(line, ':')
		if len(parts) < 3 || parts[0] == "" {
			continue
		}
		devices = append(devices, Device{
			Name:  parts[0],
			Type:  parts[1],
			State: parts[2],
		})
	}
	return devices
}

// splitUnescaped splits on sep, ignoring an escaped one.
//
// This is the single most important function in the file. nmcli escapes a
// literal colon inside a value as "\:", so a bluetooth device's MAC comes back
// as "CC\:42\:10\:65\:BE\:FB". Splitting on every colon gives seven fields
// instead of three and produces a device named "CC\" connected to nothing —
// wrong, and wrong in a way that looks like a data problem rather than a parser.
func splitUnescaped(line string, sep byte) []string {
	var (
		parts   []string
		current strings.Builder
	)

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\\' && i+1 < len(line):
			// Keep the escape and the character; the caller wants the value as
			// nmcli wrote it, and un-escaping is the caller's business only if
			// it wants it.
			current.WriteByte(c)
			i++
			current.WriteByte(line[i])
		case c == sep:
			parts = append(parts, current.String())
			current.Reset()
		default:
			current.WriteByte(c)
		}
	}
	parts = append(parts, current.String())
	return parts
}

// bluetooth reads the adapter and its paired devices.
//
// Every failure here is "no bluetooth", never an error. The tool blocks forever
// when bluetoothd is down, which is why run() has a deadline, and it prints its
// errors to stdout while exiting 0, which is why presence is tested with `list`
// — the one subcommand whose empty output means what it says.
func bluetooth() Bluetooth {
	// list: exit 0 and empty stdout means no adapter. Exit 0 and a MAC means an
	// adapter exists.
	listing, exit := run("bluetoothctl", "list")
	if exit != 0 || strings.TrimSpace(listing) == "" {
		return Bluetooth{}
	}
	state := Bluetooth{Present: true, Devices: []BluetoothDevice{}}

	// show: tab-indented "Key: value" lines. Only two lines matter and they use
	// one vocabulary, "yes"/"no" — PowerState: on is a second, redundant field
	// with a different one, so it is not read.
	if show, exit := run("bluetoothctl", "show"); exit == 0 {
		state.Powered = strings.Contains(show, "Powered: yes")
	}

	// devices: "Device <MAC> <alias>". A fixed four-token prefix makes this the
	// one bluetoothctl output that is genuinely safe to split, and the alias may
	// contain spaces, so only the first two fields are taken structurally.
	state.Devices = parseBluetoothDevices(listing)
	return state
}

// parseBluetoothDevices reads `bluetoothctl list`.
//
// A fixed "Device <MAC> <alias>" prefix makes this the one bluetoothctl output
// that is safe to split, and the alias may contain spaces, so only the MAC is
// taken structurally and the rest of the line is the name.
func parseBluetoothDevices(listing string) []BluetoothDevice {
	devices := []BluetoothDevice{}

	for _, line := range strings.Split(listing, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Device ") {
			continue
		}
		mac, alias, found := strings.Cut(strings.TrimPrefix(line, "Device "), " ")
		if !found {
			continue
		}
		devices = append(devices, BluetoothDevice{Address: mac, Name: alias})
	}
	return devices
}

// display reads the backlight.
//
// brightnessctl -m is the safest of the five to parse: pure ASCII, no human
// words, no locale-sensitive formatting. The device is pinned rather than left
// to "whichever backlight comes first", because a machine can have more than one
// and the bare form silently follows the first.
//
// Writable is false and stays false until Zenith lays down a udev rule; see
// docs/architecture.md. It is reported rather than assumed so the sidebar can
// show the level without offering a slider that fails.
func display() Display {
	raw, exit := run("brightnessctl", "-m", "-c", "backlight")
	if exit != 0 {
		return Display{}
	}

	// "name,class,current,percent,max" — five fields, and the fourth is the
	// percentage with its sign attached.
	// "name,class,current,percent,max" — the fourth field is the percentage with
	// its sign attached.
	fields := strings.Split(strings.TrimSpace(raw), ",")
	if len(fields) != 5 {
		return Display{}
	}
	percent, err := strconv.Atoi(strings.TrimSuffix(fields[3], "%"))
	if err != nil {
		return Display{}
	}
	return Display{Available: true, Percent: percent}
}

// media reads the MPRIS player.
//
// playerctl has no JSON. It has a format template, and the template form is used
// for everything because the two output modes format the same float differently
// — `playerctl volume` gives "1.000000" and `{{volume}}` gives "1.0". One form,
// one parser.
//
// -p ALL does not work in playerctl 2.4, despite being a magic value in a
// dozen other tools; the correct forms are -a and -p NAME. -l is used to find a
// player at all, and an empty list is exit 0.
func media() Media {
	listing, exit := run("playerctl", "-l")
	if exit != 0 {
		return Media{}
	}

	player := strings.Trim(strings.TrimSpace(listing), "[]")
	if player == "" {
		return Media{}
	}

	// metadata fails against a stopped player with the same exit code as "no
	// player", so it is not allowed to decide availability — that was decided
	// above, from -l. A player that answers with nothing still reports
	// available, which is the truth: the player exists.
	format := "{{status}}|{{title}}|{{artist}}"
	raw, exit := run("playerctl", "-p", player, "-f", format, "metadata")
	if exit != 0 {
		return Media{Available: true, Player: player}
	}

	parts := strings.SplitN(strings.TrimSpace(raw), "|", 3)
	state := Media{Available: true, Player: player}
	if len(parts) > 0 {
		state.Status = parts[0]
	}
	if len(parts) > 1 {
		state.Title = parts[1]
	}
	if len(parts) > 2 {
		state.Artist = parts[2]
	}
	return state
}
