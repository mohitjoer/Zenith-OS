// Command zenith-sysd is the privileged system daemon for Zenith OS.
//
// It runs as root under /var/service/zenith-sysd and provides a Unix socket
// at /run/zenith.sock for the unprivileged user session (zenith-userd) and CLI.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	socketPath = "/run/zenith.sock"
	version    = "0.1-beta"
)

type Request struct {
	Action   string   `json:"action"`
	Profile  string   `json:"profile,omitempty"`
	Packages []string `json:"packages,omitempty"`
}

type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type HardwareInfo struct {
	CPU     string   `json:"cpu"`
	GPUs    []string `json:"gpus"`
	Battery string   `json:"battery,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run", "serve":
		runDaemon()
	case "scan":
		info := scanHardware()
		data, _ := json.MarshalIndent(info, "", "  ")
		fmt.Println(string(data))
	case "power":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: zenith-sysd power <performance|balanced|power>")
			os.Exit(2)
		}
		if err := applyPowerProfile(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "error applying power profile: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("power profile set to %s\n", os.Args[2])
	case "version", "-v", "--version":
		fmt.Printf("zenith-sysd %s\n", version)
	default:
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `zenith-sysd — Zenith OS Privileged System Daemon

Usage:
  zenith-sysd run           run the daemon listening on %s
  zenith-sysd scan          scan and display PCI hardware info
  zenith-sysd power <mode>  set CPU governor and power state (performance, balanced, power)
  zenith-sysd version       show daemon version
`, socketPath)
}

func runDaemon() {
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zenith-sysd: failed to listen on %s: %v\n", socketPath, err)
		os.Exit(1)
	}
	defer os.Remove(socketPath)
	defer listener.Close()

	// Set permissions: readable/writable by root and wheel group (0660)
	_ = os.Chmod(socketPath, 0660)
	// Try to set group to wheel if available
	setWheelGroup(socketPath)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		_ = listener.Close()
		_ = os.Remove(socketPath)
		os.Exit(0)
	}()

	fmt.Printf("zenith-sysd: listening on %s\n", socketPath)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go handleConnection(conn)
	}
}

func setWheelGroup(path string) {
	// Look up wheel group in /etc/group
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) >= 3 && parts[0] == "wheel" {
			if gid, err := strconv.Atoi(parts[2]); err == nil {
				_ = os.Chown(path, 0, gid)
				return
			}
		}
	}
}

func isBenignDisconnect(err error) bool {
	if err == nil || err == io.EOF {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "connection reset") ||
		strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "closed")
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if !isBenignDisconnect(err) {
				fmt.Fprintf(os.Stderr, "zenith-sysd: read error: %v\n", err)
			}
			return
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			sendResponse(conn, Response{Status: "error", Message: "malformed JSON request"})
			continue
		}

		res := dispatch(req)
		sendResponse(conn, res)
	}
}

func dispatch(req Request) Response {
	switch req.Action {
	case "ping":
		return Response{Status: "ok", Message: "pong"}
	case "hardware_scan":
		info := scanHardware()
		return Response{Status: "ok", Data: info}
	case "set_power_profile":
		if err := applyPowerProfile(req.Profile); err != nil {
			return Response{Status: "error", Message: err.Error()}
		}
		return Response{Status: "ok", Message: "applied " + req.Profile}
	case "pkg_install":
		if len(req.Packages) == 0 {
			return Response{Status: "error", Message: "no packages specified"}
		}
		if err := runXbpsInstall(req.Packages); err != nil {
			return Response{Status: "error", Message: err.Error()}
		}
		return Response{Status: "ok", Message: "packages installed"}
	case "pkg_update":
		if err := runXbpsUpdate(); err != nil {
			return Response{Status: "error", Message: err.Error()}
		}
		return Response{Status: "ok", Message: "system updated"}
	default:
		return Response{Status: "error", Message: "unknown action: " + req.Action}
	}
}

func sendResponse(conn net.Conn, res Response) {
	data, err := json.Marshal(res)
	if err == nil {
		_, _ = conn.Write(append(data, '\n'))
	}
}

func scanHardware() HardwareInfo {
	info := HardwareInfo{}

	// Read CPU info
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					info.CPU = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	}

	// Read PCI devices for GPUs
	pciDir := "/sys/bus/pci/devices"
	if entries, err := os.ReadDir(pciDir); err == nil {
		for _, entry := range entries {
			classPath := filepath.Join(pciDir, entry.Name(), "class")
			if classBytes, err := os.ReadFile(classPath); err == nil {
				class := strings.TrimSpace(string(classBytes))
				// VGA compatible controller is class 0x030000 or 0x030200 (3D controller)
				if strings.HasPrefix(class, "0x0300") || strings.HasPrefix(class, "0x0302") {
					vendorPath := filepath.Join(pciDir, entry.Name(), "vendor")
					vendorBytes, _ := os.ReadFile(vendorPath)
					vendor := strings.TrimSpace(string(vendorBytes))

					gpuName := entry.Name()
					switch vendor {
					case "0x8086":
						gpuName = "Intel Integrated Graphics (" + entry.Name() + ")"
					case "0x10de":
						gpuName = "NVIDIA Discrete Graphics (" + entry.Name() + ")"
					case "0x1002":
						gpuName = "AMD Radeon Graphics (" + entry.Name() + ")"
					}
					info.GPUs = append(info.GPUs, gpuName)
				}
			}
		}
	}

	// Read Battery info
	batPath := "/sys/class/power_supply/BAT0/capacity"
	if capBytes, err := os.ReadFile(batPath); err == nil {
		info.Battery = strings.TrimSpace(string(capBytes)) + "%"
	}

	return info
}

func applyPowerProfile(profile string) error {
	var governor string
	var epp string

	switch strings.ToLower(profile) {
	case "performance":
		governor = "performance"
		epp = "performance"
	case "power", "powersave", "battery":
		governor = "powersave"
		epp = "power"
	case "balanced", "balance", "auto", "ac":
		governor = "powersave"
		epp = "balance_performance"
	default:
		return fmt.Errorf("unknown profile: %s", profile)
	}

	// Apply cpufreq scaling governors
	matches, _ := filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/scaling_governor")
	for _, path := range matches {
		_ = os.WriteFile(path, []byte(governor), 0644)
	}

	// Apply energy_performance_preference if available
	eppMatches, _ := filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/energy_performance_preference")
	for _, path := range eppMatches {
		_ = os.WriteFile(path, []byte(epp), 0644)
	}

	// Trigger TLP if installed
	switch profile {
	case "performance", "balanced":
		_ = exec.Command("tlp", "ac").Run()
	case "power":
		_ = exec.Command("tlp", "bat").Run()
	}

	// Apply ASUS WMI thermal policy / platform profile
	switch profile {
	case "performance":
		_ = os.WriteFile("/sys/devices/platform/asus-nb-wmi/throttle_thermal_policy", []byte("1"), 0644)
		_ = os.WriteFile("/sys/firmware/acpi/platform_profile", []byte("performance"), 0644)
	case "power":
		_ = os.WriteFile("/sys/devices/platform/asus-nb-wmi/throttle_thermal_policy", []byte("2"), 0644)
		_ = os.WriteFile("/sys/firmware/acpi/platform_profile", []byte("low-power"), 0644)
	case "balanced":
		_ = os.WriteFile("/sys/devices/platform/asus-nb-wmi/throttle_thermal_policy", []byte("0"), 0644)
		_ = os.WriteFile("/sys/firmware/acpi/platform_profile", []byte("balanced"), 0644)
	}

	// Maintain 80% battery charge threshold for battery longevity
	bats, _ := filepath.Glob("/sys/class/power_supply/BAT*/charge_control_end_threshold")
	for _, b := range bats {
		_ = os.WriteFile(b, []byte("80"), 0644)
	}

	// Enable D3cold runtime PM for discrete PCIe devices (including NVIDIA dGPU)
	gpus, _ := filepath.Glob("/sys/bus/pci/devices/*/power/control")
	for _, f := range gpus {
		_ = os.WriteFile(f, []byte("auto"), 0644)
	}

	return nil
}

func runXbpsInstall(packages []string) error {
	if _, err := exec.LookPath("xbps-install"); err != nil {
		return fmt.Errorf("xbps-install not available on host")
	}
	args := append([]string{"-y"}, packages...)
	cmd := exec.Command("xbps-install", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runXbpsUpdate() error {
	if _, err := exec.LookPath("xbps-install"); err != nil {
		return fmt.Errorf("xbps-install not available on host")
	}
	cmd := exec.Command("xbps-install", "-Syu")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
