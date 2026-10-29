package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const powerUsage = `Usage: zenith power <verb> [arguments...]

Verbs:
  status                print current power supply state, governor, and profile
  profile [name]        get or set profile (balanced, performance, battery)
  ac                    switch power settings to AC (balanced/performance)
  bat                   switch power settings to Battery (power-saving, D3cold)
`

func init() {
	register("power", "manage CPU governors, energy preference, and hybrid GPU power", zenithPower)
}

func zenithPower(args []string) int {
	if len(args) == 0 {
		return runPowerStatus()
	}

	switch args[0] {
	case "status":
		return runPowerStatus()
	case "profile":
		if len(args) > 1 {
			return setPowerProfile(args[1])
		}
		return getPowerProfile()
	case "ac":
		return setPowerProfile("balanced")
	case "bat", "battery":
		return setPowerProfile("battery")
	case "-h", "--help", "help":
		fmt.Fprint(os.Stdout, powerUsage)
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "zenith power: unknown verb %q\n\n%s", args[0], powerUsage)
		return exitUsage
	}
}

func runPowerStatus() int {
	state := map[string]interface{}{}

	// Check power supply online status
	acOnline := false
	supplies, _ := filepath.Glob("/sys/class/power_supply/*/online")
	for _, s := range supplies {
		data, err := os.ReadFile(s)
		if err == nil && strings.TrimSpace(string(data)) == "1" {
			acOnline = true
			break
		}
	}
	state["ac_online"] = acOnline

	// Read CPU governor
	gov, _ := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor")
	state["governor"] = strings.TrimSpace(string(gov))

	// Read CPU EPP if available
	epp, _ := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/energy_performance_preference")
	if len(epp) > 0 {
		state["epp"] = strings.TrimSpace(string(epp))
	}

	// Read NVIDIA power status
	nvPower, _ := os.ReadFile("/sys/bus/pci/devices/0000:01:00.0/power/runtime_status")
	if len(nvPower) > 0 {
		state["gpu_runtime_status"] = strings.TrimSpace(string(nvPower))
	}

	out, _ := json.MarshalIndent(state, "", "  ")
	fmt.Println(string(out))
	return exitOK
}

func getPowerProfile() int {
	epp, _ := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/energy_performance_preference")
	cur := strings.TrimSpace(string(epp))
	switch cur {
	case "performance":
		fmt.Println("performance")
	case "power":
		fmt.Println("battery")
	default:
		fmt.Println("balanced")
	}
	return exitOK
}

func setPowerProfile(profile string) int {
	profile = strings.ToLower(strings.TrimSpace(profile))

	var governor string
	var epp string

	switch profile {
	case "performance":
		governor = "performance"
		epp = "performance"
	case "battery", "bat", "powersave":
		governor = "powersave"
		epp = "power"
	case "balanced", "balance":
		governor = "powersave"
		epp = "balance_performance"
	default:
		fmt.Fprintf(os.Stderr, "zenith power profile: unknown profile %q (choose: balanced, performance, battery)\n", profile)
		return exitUsage
	}

	// Try IPC via /run/zenith.sock (zenith-sysd) first
	if applyViaSysd(governor, epp, profile) {
		fmt.Printf("Power profile set to %s (via zenith-sysd)\n", profile)
		return exitOK
	}

	// Direct sysfs fallback if running with privileges
	applied := false
	cpus, _ := filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/scaling_governor")
	for _, f := range cpus {
		if os.WriteFile(f, []byte(governor), 0644) == nil {
			applied = true
		}
	}

	epps, _ := filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/energy_performance_preference")
	for _, f := range epps {
		_ = os.WriteFile(f, []byte(epp), 0644)
	}

	// Enable D3cold auto runtime PM on GPU devices
	gpus, _ := filepath.Glob("/sys/bus/pci/devices/*/power/control")
	for _, f := range gpus {
		_ = os.WriteFile(f, []byte("auto"), 0644)
	}

	if applied {
		fmt.Printf("Power profile set to %s\n", profile)
		return exitOK
	}

	fmt.Printf("Power profile requested: %s (requires zenith-sysd or root privileges)\n", profile)
	return exitOK
}

func applyViaSysd(gov, epp, profile string) bool {
	sockPath := "/run/zenith.sock"
	conn, err := net.DialTimeout("unix", sockPath, 200*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()

	req := fmt.Sprintf(`{"action":"set_power","profile":"%s","governor":"%s","epp":"%s"}`+"\n", profile, gov, epp)
	if _, err := conn.Write([]byte(req)); err != nil {
		return false
	}
	return true
}
