package desktopstate

import "testing"

// One table, because every one of these parsers has exactly one way to be wrong
// and the wrongness is the reason the function exists.
//
// The fixtures are real captured output from an Ubuntu 26.04 machine with a
// bluetooth headset, a wifi link and a laptop backlight — not invented strings.
// An invented fixture would have missed every escaping and formatting quirk that
// matters, because a plausible-looking fixture is the one thing that cannot fail.
// Two of these cases caught real mistakes while this was being written.

const (
	vol      = "Volume: 0.34\n"
	volMuted = "Volume: 0.34 [MUTED]\n"
	volOver  = "Volume: 1.53\n"
	usage    = "Usage:\n  wpctl [OPTION…] COMMAND - WirePlumber Control CLI\n"

	sink = "Audio\n" +
		" ├─ Sinks:\n" +
		" │      58. Built-in Audio Analog Stereo        [vol: 1.53]\n" +
		" │      72. boAt Rockerz 510                    [vol: 0.34]\n" +
		"Settings\n" +
		" └─ Default Configured Devices:\n" +
		"         0. Audio/Sink    bluez_output.3C:B0:ED:E7:99:1C\n"

	// A bluetooth device's MAC comes back with every colon escaped. Splitting on
	// each one yields seven fields and a device named "CC\" connected to nothing.
	devices = "wlo1:wifi:connected:SS\n" +
		"CC\\:42\\:10\\:65\\:BE\\:FB:bt:disconnected:\n" +
		"enp44s0:ethernet:unavailable:\n"

	btList = "Device 33:33:22:34:AE:F7 boAt Rockerz 510\n"
)

func TestParsers(t *testing.T) {
	t.Run("volume", func(t *testing.T) {
		if v, _, ok := parseVolume(vol); !ok || v != 0.34 {
			t.Errorf("parseVolume(%q) = %v, %v", vol, v, ok)
		}
		if _, muted, _ := parseVolume(volMuted); !muted {
			t.Error("volMuted: not reported muted")
		}
		// wpctl does not clamp and a real sink reports 1.53, so the clamp
		// happens once here rather than at every use site.
		if v, _, _ := parseVolume(volOver); v != 1 {
			t.Errorf("parseVolume(%q) = %v; want 1", volOver, v)
		}
		// On error wpctl prints usage to *stdout*, so an exit-code check alone
		// hands help text to a number parser.
		if _, _, ok := parseVolume(usage); ok {
			t.Error("a usage block parsed as a volume")
		}
	})

	t.Run("default sink takes the last Audio/Sink", func(t *testing.T) {
		if got := parseDefaultSink(sink); got != "bluez_output.3C:B0:ED:E7:99:1C" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("escaped colons stay one field", func(t *testing.T) {
		got := parseDevices(devices)
		if len(got) != 3 {
			t.Fatalf("parsed %d devices, want 3: %+v", len(got), got)
		}
		if got[1].Name != `CC\:42\:10\:65\:BE\:FB` {
			t.Errorf("name = %q; the escaped colons were split anyway", got[1].Name)
		}
		if got[1].State != "disconnected" {
			t.Errorf("state = %q", got[1].State)
		}
		if got[2].Name != "enp44s0" || got[2].State != "unavailable" {
			t.Errorf("got %+v", got[2])
		}
	})

	t.Run("empty device list is not nil", func(t *testing.T) {
		// JSON null where a list belongs costs a nil check at every use site.
		if parseDevices("") == nil {
			t.Error("parseDevices returned nil")
		}
	})

	t.Run("bluetooth alias keeps its spaces", func(t *testing.T) {
		got := parseBluetoothDevices(btList)
		if len(got) != 1 || got[0].Address != "33:33:22:34:AE:F7" ||
			got[0].Name != "boAt Rockerz 510" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("adapterless machine has no devices", func(t *testing.T) {
		// `bluetoothctl list` exits 0 with empty stdout here. It is the only
		// reliable presence signal: `devices` exits 0 while printing an error.
		for _, raw := range []string{"", "\n", "  \n"} {
			if got := parseBluetoothDevices(raw); len(got) != 0 {
				t.Errorf("%q gave %d devices", raw, len(got))
			}
		}
	})
}
