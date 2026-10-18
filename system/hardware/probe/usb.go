package probe

import (
	"fmt"
	"strings"
)

// USBDevice is one USB device as sysfs reports it.
//
// Topology is reconstructed from the device directory name itself, which encodes
// it: "usb1" is a root hub, "1-0:1.0" is an interface of a device on bus 1 port
// 0, "3-10:1.0" is an interface of a device on bus 3 port 10. lsusb -t is the
// human rendering of exactly this tree and is never consulted for it.
type USBDevice struct {
	// Name is the sysfs directory name and the topology key.
	Name string `json:"name"`

	IDVendor     string `json:"id_vendor,omitempty"`
	IDProduct    string `json:"id_product,omitempty"`
	BCDevice     string `json:"bcd_device,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Product      string `json:"product,omitempty"`

	BusNum  string `json:"busnum,omitempty"`
	DevNum  string `json:"devnum,omitempty"`
	Speed   string `json:"speed,omitempty"`
	Version string `json:"version,omitempty"`

	Driver string `json:"driver,omitempty"`

	// Authorized is the kernel's verdict on whether this device may operate.
	// It is the signal that a machine has a device it cannot use, which is the
	// offline-risk condition, and it is deliberately visible rather than
	// smoothed over.
	Authorized bool `json:"authorized"`

	// Topology is the parsed position of this device.
	Topology USBTopology `json:"topology"`
}

// USBTopology is the bus/port path a device sits at.
type USBTopology struct {
	// Bus is the bus number.
	Bus int `json:"bus,omitempty"`
	// Port is the port number on that bus. 0 is the root hub itself.
	Port int `json:"port,omitempty"`
	// Interface is the interface number within the device; 0 is the device
	// itself rather than an interface.
	Interface int `json:"interface,omitempty"`
	// IsRootHub reports a "usbN" entry.
	IsRootHub bool `json:"is_root_hub"`
	// Depth is how many hubs deep the device sits. A device at depth 0 is
	// plugged straight into the machine.
	Depth int `json:"depth,omitempty"`
}

// USB reads /sys/bus/usb/devices/ relative to the reader's root.
//
// The `serial` attribute is deliberately not read. USB serial numbers are
// frequently per-device vendor garbage and are sometimes duplicated across an
// entire production run, so hashing them would publish a correlatable token for
// a value that carries no information. They are dropped unconditionally, which
// is a different decision from the one made for DMI serials and is deliberate.
func USB(fs *FS) ([]USBDevice, error) {
	names, err := fs.entries("bus/usb/devices")
	if err != nil {
		return nil, fmt.Errorf("usb: reading bus/usb/devices: %w", err)
	}

	devices := make([]USBDevice, 0, len(names))
	for _, name := range names {
		device, err := readUSBDevice(fs, name)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func readUSBDevice(fs *FS, name string) (USBDevice, error) {
	device := USBDevice{Name: name}
	base := "bus/usb/devices/" + name

	for attr, target := range map[string]*string{
		"idVendor":     &device.IDVendor,
		"idProduct":    &device.IDProduct,
		"bcdDevice":    &device.BCDevice,
		"manufacturer": &device.Manufacturer,
		"product":      &device.Product,
		"busnum":       &device.BusNum,
		"devnum":       &device.DevNum,
		"speed":        &device.Speed,
		"version":      &device.Version,
	} {
		value, present, err := fs.readString(base + "/" + attr)
		if err != nil {
			return device, fmt.Errorf("usb: reading %s of %s: %w", attr, name, err)
		}
		if present {
			*target = value
		}
	}

	if driver, err := fs.readLink(base + "/driver"); err != nil {
		return device, fmt.Errorf("usb: resolving driver of %s: %w", name, err)
	} else {
		device.Driver = driver
	}

	// authorized is "0" for a device the kernel has refused. Absent means an
	// older kernel or a device with no authorization concept, which is not the
	// same as being refused and must not be reported as though it were.
	if raw, present, err := fs.readString(base + "/authorized"); err != nil {
		return device, fmt.Errorf("usb: reading authorized of %s: %w", name, err)
	} else if present {
		device.Authorized = raw != "0"
	}

	device.Topology = ParseUSBName(name)
	return device, nil
}

// ParseUSBName reconstructs topology from a sysfs USB device directory name.
//
// The encoding is the kernel's, not Zenith's:
//
//	usb1           root hub on bus 1
//	1-0:1.0        device 1 on port 0 of bus 1, interface 1.0
//	3-10.2:1.0     device 3 on port 10 of bus 3, behind hub at port 10.2,
//	               interface 1.0
//
// Depth is the number of dot-separated port components, so a device on port 10
// directly is depth 1 and one behind a hub on port 10.2 is depth 2. That is the
// difference between a peripheral plugged into the laptop and one plugged into a
// dock, which a profile needs to know and which nothing else in sysfs states.
//
// A name that does not parse yields a zero Topology rather than an error: the
// directory listing is authoritative and a device with an unparseable name is
// still a device that exists.
func ParseUSBName(name string) USBTopology {
	topology := USBTopology{}

	if strings.HasPrefix(name, "usb") {
		topology.IsRootHub = true
		topology.Bus = atoiDefault(strings.TrimPrefix(name, "usb"), 0)
		return topology
	}

	devicePart, interfacePart, hasInterface := strings.Cut(name, ":")
	busPart, portPart, found := strings.Cut(devicePart, "-")
	if !found {
		return topology
	}

	topology.Bus = atoiDefault(busPart, 0)

	// portPart is "10" or "10.2" or "10.2.1"; each dot-separated component is
	// one hop away from the machine.
	ports := strings.Split(portPart, ".")
	topology.Depth = len(ports)
	if len(ports) > 0 {
		topology.Port = atoiDefault(ports[0], 0)
	}

	if hasInterface {
		dot := strings.Index(interfacePart, ".")
		if dot >= 0 {
			topology.Interface = atoiDefault(interfacePart[:dot], 0)
		} else {
			topology.Interface = atoiDefault(interfacePart, 0)
		}
	}

	return topology
}

// atoiDefault parses an unsigned decimal, returning def for anything malformed.
//
// The length check is not decoration. Accumulating digits without a bound
// overflows a long run of them and produces a negative number, which then
// travels into a topology field and out into a report — a signed number where
// every other number in the same struct is a port or a bus. sysfs values are
// short decimal strings; anything longer is firmware noise.
func atoiDefault(value string, def int) int {
	if value == "" || len(value) > maxDecimalDigits {
		return def
	}
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// maxDecimalDigits is the longest run of digits that cannot overflow an int on
// every platform this builds for. Nine digits keeps the arithmetic well inside
// the range of a 32-bit int, which is the smallest int this code has to work
// on.
const maxDecimalDigits = 9
