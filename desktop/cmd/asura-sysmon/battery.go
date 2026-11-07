package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type BatteryStats struct {
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	Percentage float64 `json:"percentage"`
	HealthPct  float64 `json:"healthPct"`
	Watts      float64 `json:"watts"`
	VoltageV   float64 `json:"voltageV"`
	Technology string  `json:"technology"`
	Present    bool    `json:"present"`
}

func readTrimmed(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func readUint(path string) uint64 {
	s := readTrimmed(path)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

func readFloat(path string) float64 {
	s := readTrimmed(path)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// CollectBatteryStats detects all system batteries across laptops and portables.
// On desktops, servers, and VMs, it safely returns an empty slice.
func CollectBatteryStats() []BatteryStats {
	batteries := []BatteryStats{}

	entries, err := filepath.Glob("/sys/class/power_supply/*")
	if err != nil || len(entries) == 0 {
		return batteries
	}

	for _, entry := range entries {
		typ := strings.ToLower(readTrimmed(filepath.Join(entry, "type")))
		base := filepath.Base(entry)

		// Include power supplies identified as Battery or named BAT* / battery_*
		if typ != "battery" && !strings.HasPrefix(strings.ToUpper(base), "BAT") && !strings.HasPrefix(strings.ToLower(base), "battery") {
			continue
		}

		// Skip peripheral device batteries (e.g. wireless mouse/keyboard with scope == "Device")
		scope := readTrimmed(filepath.Join(entry, "scope"))
		if strings.EqualFold(scope, "device") {
			continue
		}

		// Check if battery is present
		presentStr := readTrimmed(filepath.Join(entry, "present"))
		present := true
		if presentStr == "0" {
			present = false
		}

		status := readTrimmed(filepath.Join(entry, "status"))
		if status == "" {
			status = "Unknown"
		}

		technology := readTrimmed(filepath.Join(entry, "technology"))
		if technology == "" {
			technology = "Li-ion"
		}

		// Capacity %: try capacity file, or compute from charge/energy
		capacity := readFloat(filepath.Join(entry, "capacity"))

		// Voltage: microvolts -> Volts
		voltageUV := readFloat(filepath.Join(entry, "voltage_now"))
		if voltageUV == 0 {
			voltageUV = readFloat(filepath.Join(entry, "voltage_min_design"))
		}
		voltageV := voltageUV / 1e6

		// Power / Watts
		// Some kernels report power_now in microwatts, others report current_now in microamps
		var watts float64
		powerUW := readFloat(filepath.Join(entry, "power_now"))
		if powerUW > 0 {
			watts = powerUW / 1e6
		} else {
			currentUA := readFloat(filepath.Join(entry, "current_now"))
			if currentUA > 0 && voltageV > 0 {
				watts = (currentUA / 1e6) * voltageV
			}
		}

		// Health calculation: charge_full / charge_full_design OR energy_full / energy_full_design
		var healthPct float64
		fullCharge := readFloat(filepath.Join(entry, "charge_full"))
		designCharge := readFloat(filepath.Join(entry, "charge_full_design"))
		if fullCharge == 0 || designCharge == 0 {
			fullCharge = readFloat(filepath.Join(entry, "energy_full"))
			designCharge = readFloat(filepath.Join(entry, "energy_full_design"))
		}

		if designCharge > 0 && fullCharge > 0 {
			healthPct = (fullCharge / designCharge) * 100.0
			if healthPct > 100.0 {
				healthPct = 100.0
			}
		}

		// If capacity wasn't directly in capacity file, compute it
		if capacity == 0 {
			nowCharge := readFloat(filepath.Join(entry, "charge_now"))
			if nowCharge == 0 {
				nowCharge = readFloat(filepath.Join(entry, "energy_now"))
			}
			if fullCharge > 0 && nowCharge > 0 {
				capacity = (nowCharge / fullCharge) * 100.0
			}
		}

		// Clamp numerical metrics to safe ranges
		if capacity < 0 {
			capacity = 0
		} else if capacity > 100 {
			capacity = 100
		}

		if healthPct < 0 {
			healthPct = 0
		} else if healthPct > 100 {
			healthPct = 100
		}

		if watts < 0 {
			watts = 0
		}
		if voltageV < 0 {
			voltageV = 0
		}

		batteries = append(batteries, BatteryStats{
			Name:       base,
			Status:     status,
			Percentage: capacity,
			HealthPct:  healthPct,
			Watts:      watts,
			VoltageV:   voltageV,
			Technology: technology,
			Present:    present,
		})
	}

	if batteries == nil {
		batteries = []BatteryStats{}
	}
	return batteries
}
