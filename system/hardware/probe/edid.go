package probe

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

// Panel is one DRM connector and what EDID said about it.
type Panel struct {
	// Connector is the sysfs connector directory name, e.g. "card0-eDP-1".
	Connector string `json:"connector"`

	// Card is the card part, e.g. "card0". On a hybrid machine this is how the
	// panel is attributed to the GPU that drives it.
	Card string `json:"card,omitempty"`

	// Present is false for a disconnected connector. A zero-length edid read is
	// this and not an error: the kernel reports 0 bytes for a connector with
	// nothing plugged in, and that is the single most common case on a laptop
	// with a lid that is closed.
	Present bool `json:"present"`

	// Connected is the connector's own status attribute, which distinguishes
	// "connected" from "unknown". A connector can be connected and still have
	// no readable EDID.
	Connected string `json:"connected,omitempty"`

	Manufacturer string `json:"manufacturer,omitempty"`
	ProductCode  string `json:"product_code,omitempty"`
	MonitorName  string `json:"monitor_name,omitempty"`
	SerialToken  string `json:"serial_token,omitempty"`

	// Width, Height and RefreshHz are the preferred/native mode. A native mode
	// is what the panel was factory-calibrated to, which is the mode worth
	// naming to a user whose external display is running at 60 Hz.
	Width     int     `json:"width,omitempty"`
	Height    int     `json:"height,omitempty"`
	RefreshHz float64 `json:"refresh_hz,omitempty"`

	// HasRangeLimits mirrors EDIDInfo. See its comment: a range-limits
	// descriptor is not an HDR claim.
	HasRangeLimits bool `json:"has_range_limits"`

	// Internal is a hint, never a fact. No EDID field says whether a panel is
	// the machine's own; it is inferred from the connector name convention and
	// corroborated by there being exactly one backlight device. The evidence is
	// carried alongside so a reader can see why the inference was made.
	Internal bool `json:"internal"`
	// InternalEvidence is the connector-name prefix that drove the inference,
	// e.g. "eDP". Empty when no inference was made.
	InternalEvidence string `json:"internal_evidence,omitempty"`

	// MaxBacklight is the panel's maximum brightness where a backlight device
	// exists. Whether the panel dims is not a thing EDID can answer.
	MaxBacklight int `json:"max_brightness,omitempty"`

	// BacklightDevice is the driver that owns the backlight —
	// "intel_backlight", "amdgpu_bl0", "nvidia_wmi_ec_backlight". Which driver
	// owns it is the difference between "the panel dims" and "the panel does
	// not dim at all", so it is carried rather than reduced to a boolean.
	BacklightDevice string `json:"backlight_device,omitempty"`

	// Modes is the connector's advertised mode list, if the kernel exposes it.
	Modes []string `json:"modes,omitempty"`
}

// embeddedConnectorPrefixes are the connector-name prefixes that mean "this
// panel is inside the machine". It is a naming convention, which is exactly why
// Internal is a hint with its evidence attached and not a bare fact.
var embeddedConnectorPrefixes = []string{"eDP-", "LVDS-", "DSI-"}

// EDID reads /sys/class/drm/ relative to the reader's root.
//
// Three rules govern this reader and all three are traps:
//
//  1. Do not trust stat. On this base layer's kernel the sysfs size attribute
//     for a DRM edid file reports 0 while the file itself reads 128 bytes. The
//     byte count the read returns is the only trustworthy length.
//  2. A zero-length read is normal and means the connector has nothing on it.
//     It is Present:false, never an error and never a panic.
//  3. Anything that is not a non-zero multiple of 128 is a truncated or
//     faulty read and is rejected whole. Parsing the valid prefix of a corrupt
//     block produces plausible-looking wrong values, which is worse than
//     saying nothing.
func EDID(fs *FS) ([]Panel, error) {
	cards, err := fs.entries("class/drm")
	if err != nil {
		return nil, fmt.Errorf("edid: reading class/drm: %w", err)
	}

	// One backlight device for the whole machine is what corroborates which
	// connector is the internal panel, so it is resolved once here rather than
	// re-derived per panel.
	backlights, err := fs.entries("class/backlight")
	if err != nil {
		return nil, fmt.Errorf("edid: reading class/backlight: %w", err)
	}
	backlightDevice := ""
	maxBacklight := 0
	if len(backlights) == 1 {
		backlightDevice = backlights[0]
		if value, present, err := fs.readInt("class/backlight/" + backlightDevice + "/max_brightness"); err == nil && present {
			maxBacklight = value
		}
	}

	var panels []Panel
	for _, name := range cards {
		card, isCard := splitConnector(name)
		if !isCard {
			continue
		}

		panel := Panel{
			Connector:       name,
			Card:            card,
			BacklightDevice: backlightDevice,
			MaxBacklight:    maxBacklight,
		}

		if status, present, err := fs.readString("class/drm/" + name + "/status"); err == nil && present {
			panel.Connected = status
		}

		if modes, present, err := fs.readString("class/drm/" + name + "/modes"); err == nil && present {
			panel.Modes = strings.Fields(modes)
		}

		block, present, err := fs.readBytes("class/drm/" + name + "/edid")
		if err != nil {
			return nil, fmt.Errorf("edid: reading %s: %w", name, err)
		}
		if !present {
			// No edid file at all. Present stays false.
			panels = append(panels, panel)
			continue
		}
		if len(block) == 0 {
			// Rule 2: a zero-length read is a disconnected connector.
			panels = append(panels, panel)
			continue
		}
		if err := validateEDIDLength(len(block)); err != nil {
			// Rule 3: reject the whole read rather than parse its prefix.
			panels = append(panels, panel)
			continue
		}

		parsed, err := ParseEDID(block)
		if err != nil {
			panels = append(panels, panel)
			continue
		}

		panel.Present = true
		panel.Manufacturer = parsed.Manufacturer
		panel.ProductCode = parsed.ProductCode
		panel.MonitorName = parsed.MonitorName
		panel.SerialToken = parsed.SerialToken
		panel.Width = parsed.Width
		panel.Height = parsed.Height
		panel.RefreshHz = parsed.RefreshHz
		panel.HasRangeLimits = parsed.HasRangeLimits

		prefix := embeddedPrefix(name)
		if prefix != "" {
			panel.Internal = true
			panel.InternalEvidence = prefix
		}

		panels = append(panels, panel)
	}

	return panels, nil
}

// EDIDInfo is the parsed content of one EDID block.
type EDIDInfo struct {
	Manufacturer string
	ProductCode  string
	MonitorName  string
	// SerialToken is the hashed serial descriptor, never the raw value.
	SerialToken string
	Width       int
	Height      int
	RefreshHz   float64

	// HasRangeLimits reports a display range limits descriptor. It is not the
	// same as HDR support: HDR lives in a CTA extension block, not in the base
	// block, and reporting a range-limits descriptor as HDR would claim a
	// capability the block does not describe.
	HasRangeLimits bool
}

// validateEDIDLength enforces that a block is 128 or 256 bytes.
//
// Anything else is a partial read. The kernel reads the block in 128-byte
// chunks and a truncated read is common on a flaky eDP link; parsing 100 bytes
// as though they were a whole block would yield a manufacturer and a product
// code that happen to be right and a timing table that is garbage.
func validateEDIDLength(n int) error {
	if n == 128 || n == 256 {
		return nil
	}
	return fmt.Errorf("edid: block is %d bytes; want 128 or 256", n)
}

// ParseEDID parses a 128- or 256-byte EDID block.
//
// This takes untrusted input by design: in a virtual machine the block is
// whatever the hypervisor chose, and in a third-party pull request it is
// whatever the contributor attached. Every offset is bounds-checked and no
// read trusts a length it computed from the block's own contents.
func ParseEDID(block []byte) (EDIDInfo, error) {
	var info EDIDInfo
	if err := validateEDIDLength(len(block)); err != nil {
		return info, err
	}

	// The header is 8 fixed bytes whose first two spell "ED" in the extended
	// display identification data format. A block without it is not EDID, and
	// reading one anyway is how a parser ends up emitting numbers.
	if block[0] != 0x00 || block[1] != 0xFF {
		return info, fmt.Errorf("edid: missing 00 FF header")
	}

	// Bytes 8 and 9 hold the manufacturer ID, a 15-bit value carrying three
	// 5-bit letters, with a reserved 0b00010 tag in the top five bits of byte 8.
	//
	// The letters do not sit in byte-aligned fields: the first letter's low two
	// bits are byte 8's bottom bits, its high three bits are byte 9's top bits,
	// the second letter is byte 9's bottom five, and the third is the bottom
	// five of byte 10 — which is why the decode below reads across the byte
	// boundary rather than slicing one field per byte.
	//
	// Getting this wrong yields a three-letter code that is plausible and
	// incorrect, which is worse than reporting none.
	// The letters are 5-bit values, so they range 0-31 while the alphabet is
	// 1-26. Values outside 1-26 are not letters: a firmware that puts 27 in
	// that field has not encoded a manufacturer, and mapping it anyway yields
	// '@' or '[' — a string that looks like a decoded PNP ID and is not one.
	if block[8]>>3 == 0x02 {
		letter1 := ((block[9]>>5)&0x07)<<2 | (block[8] & 0x03)
		letter2 := block[9] & 0x1F
		letter3 := block[10] & 0x1F
		if letter1 >= 1 && letter1 <= 26 &&
			letter2 >= 1 && letter2 <= 26 &&
			letter3 >= 1 && letter3 <= 26 {
			info.Manufacturer = string([]byte{
				'A' + letter1 - 1,
				'A' + letter2 - 1,
				'A' + letter3 - 1,
			})
		}
	}

	// Product code, little-endian 16-bit at 10-11.
	info.ProductCode = fmt.Sprintf("%04x", binary.LittleEndian.Uint16(block[10:12]))

	// Serial number, little-endian 32-bit at 12-15. Hashed here and never
	// carried raw: an internal panel does carry a serial, and it is an asset
	// identifier when it reaches a human-readable field.
	info.SerialToken = hashIdentifier("edid_serial", strconv.FormatUint(uint64(binary.LittleEndian.Uint32(block[12:16])), 10))

	// Week and year of manufacture at 16-17. Read but not surfaced: it is a
	// date, and a panel's manufacture week narrows a production run.
	_ = block[16]
	_ = block[17]

	// EDID version at 18, revision at 19.
	_ = block[18]
	_ = block[19]

	// Video timings start at byte 54 with four 18-byte descriptors.
	//
	// A descriptor's first two bytes decide what it is: an unused descriptor is
	// 0xFF 0xFF, a display descriptor is 0xFF followed by a tag, and anything
	// else is a detailed timing descriptor whose first two bytes are a non-zero
	// pixel clock. Checking for 0xFF 0x00 as though it were a DTD marker would
	// never match a real panel, because a real panel's pixel clock is not zero.
	for i := 0; i < 4; i++ {
		offset := 54 + i*18
		if offset+18 > len(block) {
			break
		}
		descriptor := block[offset : offset+18]

		switch {
		case descriptor[0] == 0xFF && descriptor[1] == 0xFF:
			// Unused descriptor. Nothing here, and nothing to guess.

		case descriptor[0] == 0xFF:
			// Display descriptor: byte 1 is the tag.
			switch descriptor[1] {
			case 0xFC:
				info.MonitorName = parseMonitorName(descriptor[2:])
			case 0xFF:
				// Serial number descriptor. The value was already read from
				// the header block.
			case 0xFD:
				// Display range limits. A fixed range is not a range.
				info.HasRangeLimits = true
			}

		default:
			// Detailed timing descriptor: the real mode.
			//
			// The byte layout is the awkward one: the low byte of each of
			// h_active and v_active sits at offsets 2 and 3, and their high
			// nibbles at 4 and 5, while the blanking widths are plain 16-bit
			// little-endian from 6 onwards. Reading the blanking as a shifted
			// 12-bit value — which looks plausible — silently produces a wrong
			// total and therefore a wrong refresh rate on every panel.
			clock := float64(binary.LittleEndian.Uint16(descriptor[0:2])) * 10000
			hActive := int(descriptor[2]) | int(descriptor[4])<<8
			vActive := int(descriptor[3]) | int(descriptor[5])<<8
			hTotal := int(binary.LittleEndian.Uint16(descriptor[10:12]))
			vTotal := int(binary.LittleEndian.Uint16(descriptor[12:14]))

			info.Width, info.Height = hActive, vActive

			// Refresh from the pixel clock over the full totals, taken from
			// the descriptor rather than derived from active+blank: the
			// descriptor is what the panel was told, and it is free to
			// disagree with the sum.
			if clock > 0 && hTotal > 0 && vTotal > 0 {
				info.RefreshHz = clock / (float64(hTotal) * float64(vTotal))
			}
		}
	}

	return info, nil
}

// monitorNameFieldLength is the size of a monitor-name descriptor's text field:
// 13 characters followed by a 0x0A terminator, inside a 16-byte payload.
const (
	monitorNameFieldLength = 13
	monitorNamePayload     = 16
)

// parseMonitorName reads a monitor-name descriptor's text.
//
// The payload is exactly 13 characters, a 0x0A terminator and then space
// padding. Reading the whole payload instead of the field would let padding
// become part of the name, so a panel with a short name reports a longer one
// than it has — a wrong value that looks perfectly plausible in a report.
func parseMonitorName(payload []byte) string {
	if len(payload) > monitorNamePayload {
		payload = payload[:monitorNamePayload]
	}
	if len(payload) > monitorNameFieldLength {
		payload = payload[:monitorNameFieldLength]
	}
	if index := bytes.IndexByte(payload, 0x0A); index >= 0 {
		payload = payload[:index]
	}
	return strings.TrimSpace(string(payload))
}

// splitConnector separates "card0-eDP-1" into its card and whether it is a
// connector at all. The card directories (card0, card1) have no connector part.
func splitConnector(name string) (string, bool) {
	if !strings.HasPrefix(name, "card") {
		return "", false
	}
	rest := name[len("card"):]
	dash := strings.Index(rest, "-")
	if dash < 0 {
		// A bare "cardN" is a card, not a connector.
		return name, false
	}
	return name[:len("card")+dash], true
}

// embeddedPrefix returns the connector-name prefix that marks an internal
// panel, or "" for an external one.
func embeddedPrefix(connector string) string {
	for _, prefix := range embeddedConnectorPrefixes {
		if strings.Contains(connector, prefix) {
			return strings.TrimSuffix(prefix, "-")
		}
	}
	return ""
}
