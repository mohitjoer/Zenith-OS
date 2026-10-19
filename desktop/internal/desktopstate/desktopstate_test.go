package desktopstate

import "testing"

// The fixtures in this file are real captured output, not invented strings.
//
// They were taken from an actual Ubuntu 26.04 machine with a bluetooth headset,
// a wifi connection, a laptop backlight and a stopped MPRIS player. Invented
// fixtures would have missed every escaping and formatting quirk that matters,
// because a plausible-looking fixture is the one thing that cannot fail.

const realVolume = `Volume: 0.34
`

const realMutedVolume = `Volume: 0.34 [MUTED]
`

const realWPStatus = `PipeWire 'pipewire-0' [1.6.2, mohit@mohit-ASUS-TUF-Gaming-F15, cookie:1617339268]
 └─ Clients:
        33. WirePlumber                         [1.6.2, mohit@..., pid:4455]
 122. wpctl                               [1.6.2, mohit@..., pid:96818]

Audio
 ├─ Devices:
 │      48. GA107 High Definition Audio Controller [alsa]
 │      52. Built-in Audio                      [alsa]
 │      67. boAt Rockerz 510                    [bluez5]
 │
 ├─ Sinks:
 │      58. Built-in Audio Analog Stereo        [vol: 1.53]
 │      72. boAt Rockerz 510                    [vol: 0.34]
 │
 ├─ Sources:
 │  *   59. Built-in Audio Analog Stereo        [vol: 0.15]

Video
 └─ Devices:
        53. USB2.0 HD UVC WebCam                [v4l2]

Settings
 └─ Default Configured Devices:
         0. Audio/Sink    bluez_output.3C:B0:ED:E7:99:1C
         1. Audio/Source  alsa_input.pci-0000_00_1f.3.analog-stereo
`

const realNmcliDevices = `wlo1:wifi:connected:SS
lo:loopback:connected (externally):lo
docker0:bridge:connected (externally):docker0
CC\:42\:10\:65\:BE\:FB:bt:disconnected:
p2p-dev-wlo1:wifi-p2p:disconnected:
enp44s0:ethernet:unavailable:
`

const realBtList = `Device AC:EF:92:D3:11:40 Xiaomi Sound Outdoor
Device 33:33:22:34:AE:F7 boAt Rockerz 510
Device 3C:B0:ED:E7:99:1C CMF Buds 2a
`

const realBtShow = `Controller 04:EC:D8:C3:D7:59 (public)
	Manufacturer: 0x0002 (2)
	Name: mohit-ASUS-TUF-Gaming-F15
	Powered: yes
	PowerState: on
	Discoverable: no
	Pairable: no
	UUID: A/V Remote Control        (0000110e-0000-1000-8000-00805f9b34fb)
`

// TestParseVolumeClampsAboveOne covers the trap that started this.
//
// wpctl does not clamp, and a real sink on this machine reports [vol: 1.53]. A
// consumer that renders value*100 draws "153" on a slider whose maximum is 100.
func TestParseVolumeClampsAboveOne(t *testing.T) {
	volume, muted, ok := parseVolume("Volume: 1.53\n")
	if !ok {
		t.Fatal("a real over-range volume was not parsed")
	}
	if volume != 1 {
		t.Errorf("volume = %v; wpctl reports above 1.0 and it must be clamped "+
			"here, once, rather than at every use site", volume)
	}
	if muted {
		t.Error("an unmuted sink was reported muted")
	}
}

func TestParseVolumeNormal(t *testing.T) {
	volume, muted, ok := parseVolume(realVolume)
	if !ok || volume != 0.34 || muted {
		t.Errorf("parseVolume(%q) = %v, %v, %v", realVolume, volume, muted, ok)
	}
}

func TestParseVolumeMuted(t *testing.T) {
	volume, muted, ok := parseVolume(realMutedVolume)
	if !ok || volume != 0.34 || !muted {
		t.Errorf("parseVolume(%q) = %v, %v, %v", realMutedVolume, volume, muted, ok)
	}
}

// TestParseVolumeRejectsUsageText covers the other thing wpctl does.
//
// On an error it prints a usage block to *stdout* and the message to stderr, so a
// parser that only checks the exit code hands the help text to a number parser.
func TestParseVolumeRejectsUsageText(t *testing.T) {
	usage := `Usage:
  wpctl [OPTION…] COMMAND [COMMAND_OPTIONS] - WirePlumber Control CLI

Options:
  -h, --help                  Show help options
`
	if _, _, ok := parseVolume(usage); ok {
		t.Error("a usage block was parsed as a volume")
	}
}

// TestParseDefaultSink picks the *last* Audio/Sink, not the first.
//
// `wpctl status` mentions Audio/Sink in more than one place, and the default
// configured device is the one in the trailing Settings block.
func TestParseDefaultSink(t *testing.T) {
	got := parseDefaultSink(realWPStatus)
	if got != "bluez_output.3C:B0:ED:E7:99:1C" {
		t.Errorf("parseDefaultSink = %q", got)
	}
}

// TestParseDevicesKeepsEscapedColons is the single most important test in this
// file.
//
// nmcli escapes a literal colon inside a value as "\:", so a bluetooth device's
// MAC comes back as "CC\:42\:10\:65\:BE\:FB". Splitting on every colon gives
// seven fields instead of three and produces a device named "CC\" that appears
// connected to nothing — wrong, and wrong in a way that looks like a data
// problem rather than a parser.
func TestParseDevicesKeepsEscapedColons(t *testing.T) {
	devices := parseDevices(realNmcliDevices)

	var bt *Device
	for i := range devices {
		if devices[i].Type == "bt" {
			bt = &devices[i]
		}
	}
	if bt == nil {
		t.Fatal("the bluetooth device vanished from the device list")
	}
	// Equality, not a search for a colon: the name legitimately contains six of
	// them, all escaped. What must not happen is it becoming six fields.
	if bt.Name != `CC\:42\:10\:65\:BE\:FB` {
		t.Errorf("bluetooth device name = %q; the escaped colons were split "+
			"anyway", bt.Name)
	}
	if bt.State != "disconnected" {
		t.Errorf("state = %q", bt.State)
	}
	if len(devices) != 6 {
		t.Errorf("parsed %d devices, want 6: %+v", len(devices), devices)
	}
}

// TestParseDevicesHandlesATrailingEmptyField covers the last device's absent
// CONNECTION column, which arrives as a trailing colon.
func TestParseDevicesHandlesATrailingEmptyField(t *testing.T) {
	devices := parseDevices("p2p-dev-wlo1:wifi-p2p:disconnected:\n")
	if len(devices) != 1 {
		t.Fatalf("parsed %d devices, want 1", len(devices))
	}
	if devices[0].Name != "p2p-dev-wlo1" || devices[0].State != "disconnected" {
		t.Errorf("got %+v", devices[0])
	}
}

// TestParseDevicesIsNeverNil covers the JSON shape.
//
// A `null` where a list belongs costs a nil check at every use site, and the
// sidebar is exactly the place that would forget one.
func TestParseDevicesIsNeverNil(t *testing.T) {
	if devices := parseDevices(""); devices == nil {
		t.Error("parseDevices returned nil; it must return an empty slice")
	}
}

// TestDeviceConnected covers the prefix rule, since "connected (externally)"
// is a connected device.
func TestDeviceConnected(t *testing.T) {
	if !(Device{State: "connected"}).Connected() {
		t.Error("a plain connected device reported disconnected")
	}
	if !(Device{State: "connected (externally)"}).Connected() {
		t.Error("an externally connected device reported disconnected")
	}
	if (Device{State: "unavailable"}).Connected() {
		t.Error("an unavailable device reported connected")
	}
}

func TestParseAdapterShow(t *testing.T) {
	powered, discovering := parseAdapterShow(realBtShow)
	if !powered {
		t.Error("Powered: yes was not read")
	}
	if discovering {
		t.Error("Discoverable: no was read as Discovering: yes")
	}
}

// TestParseBluetoothDevicesKeepsSpacesInNames covers the alias, which may
// contain spaces and is the rest of the line.
func TestParseBluetoothDevicesKeepsSpacesInNames(t *testing.T) {
	devices := parseBluetoothDevices(realBtList)
	if len(devices) != 3 {
		t.Fatalf("parsed %d devices, want 3: %+v", len(devices), devices)
	}
	if devices[1].Address != "33:33:22:34:AE:F7" {
		t.Errorf("address = %q", devices[1].Address)
	}
	if devices[1].Name != "boAt Rockerz 510" {
		t.Errorf("name = %q", devices[1].Name)
	}
}

// TestParseBluetoothDevicesIsEmptyOnAnAdapterlessMachine covers the presence
// probe.
//
// `bluetoothctl list` exits 0 with completely empty stdout when there is no
// adapter. That is the only reliable signal this tool gives: `devices` exits 0
// while printing "No default controller available", and the tool aborts with
// SIGABRT when the bus is unreachable.
func TestParseBluetoothDevicesIsEmptyOnAnAdapterlessMachine(t *testing.T) {
	for _, raw := range []string{"", "\n", "  \n"} {
		if devices := parseBluetoothDevices(raw); len(devices) != 0 {
			t.Errorf("%q gave %d devices, want 0", raw, len(devices))
		}
	}
}

// TestSplitUnescaped is the unit behind the MAC test.
func TestSplitUnescaped(t *testing.T) {
	parts := splitUnescaped(`a:b\:c:d`, ':')
	if len(parts) != 3 {
		t.Fatalf("got %d parts %q, want 3", len(parts), parts)
	}
	if parts[1] != `b\:c` {
		t.Errorf("middle part = %q", parts[1])
	}
}
