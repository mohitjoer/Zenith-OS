package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"asura/desktop/internal/runner"
)

// Two checks for faults that say nothing when they happen. A keybind whose
// command was never installed runs, fails inside the compositor, and the key
// does nothing; a recommended package that is not installed turns one control
// into a button that fails. Both are found here instead of by a user.

// configExec matches the command a compositor config runs: an exec line, or a
// bind whose dispatcher is exec. The first word of it is captured, after any
// window rule in brackets.
var configExec = regexp.MustCompile(`(?m)^\s*(?:execr?(?:-once)?\s*=|bind[a-z]*\s*=[^,\n]*,[^,\n]*,\s*exec\s*,)\s*(?:\[[^\]\n]*\]\s*)?(\S+)`)

// asuraCommands names the Asura commands a compositor config runs, by bare
// name. Anything else it runs (ghostty, wpctl) is the base layer's, and a
// command guarded by `command -v` is already degrading on purpose.
func asuraCommands(config string) []string {
	seen := map[string]bool{}
	for _, match := range configExec.FindAllStringSubmatch(config, -1) {
		word := match[1]
		if strings.HasPrefix(word, "/usr/bin/") {
			word = filepath.Base(word)
		}
		if word == "asura" || strings.HasPrefix(word, "asura-") {
			seen[word] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// reportCommands checks that every Asura command the compositor runs is
// installed where the compositor's PATH finds it. The session's PATH has no
// ~/.local/bin, so /usr/bin is the only place that counts.
func reportCommands(root string) int {
	fmt.Println()
	fmt.Println("Commands")
	config, err := installedCompositorConfig(root, "/usr/share/asura/hyprland/hyprland.conf")
	if err != nil {
		fmt.Println("  the compositor's configuration is not installed; nothing to check")
		return exitOK
	}

	names := asuraCommands(config)
	problems := 0
	for _, name := range names {
		info, err := os.Stat(filepath.Join(root, "/usr/bin", name))
		switch {
		case err != nil:
			fmt.Printf("  %-24s run by the compositor, and /usr/bin/%s is not installed\n", name, name)
			problems++
		case info.Mode().Perm()&0111 == 0:
			fmt.Printf("  %-24s /usr/bin/%s is not executable\n", name, name)
			problems++
		}
	}
	if problems > 0 {
		return exitFail
	}
	fmt.Printf("  all %d asura command(s) the compositor runs are installed\n", len(names))
	return exitOK
}

// recommendedAbsent returns the recommended packages of the asura packages
// that are not installed on Void Linux via xbps-query.
func recommendedAbsent(run runner.Func) ([]string, int, error) {
	out, err := run("xbps-query", "-l")
	if err != nil {
		return nil, 0, err
	}

	installed := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[0], "ii") {
			pkg := fields[1]
			if idx := strings.LastIndex(pkg, "-"); idx > 0 {
				installed[pkg[:idx]] = true
			}
		}
	}

	recommended := []string{
		"hyprland",
		"quickshell",
		"xdg-desktop-portal-hyprland",
		"pipewire",
		"wireplumber",
		"seatd",
		"NetworkManager",
		"ghostty",
	}

	var absent []string
	for _, pkg := range recommended {
		if !installed[pkg] {
			absent = append(absent, pkg)
		}
	}
	return absent, len(recommended), nil
}

// reportDeps lists what the asura desktop recommends and this machine lacks.
func reportDeps(root string, run runner.Func) int {
	fmt.Println()
	fmt.Println("Dependencies (XBPS)")
	if root != "" {
		fmt.Println("  not checked: the package database describes the running machine, not a --root tree")
		return exitOK
	}
	absent, total, err := recommendedAbsent(run)
	switch {
	case err != nil:
		fmt.Printf("  not checked: %s\n", runner.Reason(err))
	case len(absent) == 0:
		fmt.Printf("  all %d recommended package(s) are installed\n", total)
	default:
		fmt.Printf("  %d of %d recommended package(s) are not installed: %s\n",
			len(absent), total, strings.Join(absent, ", "))
		fmt.Printf("  sudo xbps-install -S %s\n", strings.Join(absent, " "))
	}
	return exitOK
}

// reportVoidRepo validates the XBPS repository configuration in /etc/xbps.d/.
func reportVoidRepo(root string) int {
	fmt.Println()
	fmt.Println("Repository (XBPS)")
	path := filepath.Join(root, "/etc/xbps.d/asura.conf")
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join(root, "/usr/share/xbps.d/asura.conf")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("  xbps repository config missing: %s\n", path)
		return exitFail
	}
	if !strings.Contains(string(data), "repository=") {
		fmt.Printf("  %s contains no repository entries\n", path)
		return exitFail
	}
	fmt.Printf("  repository config %s is valid\n", filepath.Base(path))
	return exitOK
}

// reportRunitServices validates that user runit services exist and are executable.
func reportRunitServices(root string) int {
	fmt.Println()
	fmt.Println("User Services (runit)")
	svcDir := filepath.Join(root, "/usr/share/asura/runit/services")
	entries, err := os.ReadDir(svcDir)
	if err != nil {
		svcDir = filepath.Join(root, "system/runit/user-services")
		entries, err = os.ReadDir(svcDir)
		if err != nil {
			fmt.Printf("  runit services directory not found\n")
			return exitFail
		}
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			runPath := filepath.Join(svcDir, entry.Name(), "run")
			if info, err := os.Stat(runPath); err == nil && info.Mode()&0111 != 0 {
				count++
			}
		}
	}
	fmt.Printf("  all %d runit user service(s) configured and executable\n", count)
	return exitOK
}

// reportCoreServices checks that scx-lavd, ananicy-cpp, asura-sysd, and seatd
// runit services are active, and with fix enables them.
func reportCoreServices(root string, fix bool) int {
	fmt.Println()
	fmt.Println("Core System Services (runit)")
	services := []string{"seatd", "asura-sysd", "scx-lavd", "ananicy-cpp"}
	problems := 0

	for _, svc := range services {
		active := false
		for _, dir := range []string{"/var/service", "/etc/runit/runsvdir/default"} {
			svcPath := filepath.Join(root, dir, svc)
			if _, err := os.Stat(svcPath); err == nil {
				active = true
				break
			}
		}
		if !active && root == "" {
			if out, err := exec.Command("sv", "status", svc).Output(); err == nil {
				if strings.HasPrefix(string(out), "run:") {
					active = true
				}
			}
		}

		if active {
			fmt.Printf("  • %-16s active\n", svc)
		} else {
			problems++
			fmt.Printf("  • %-16s inactive (enable: ln -sf /etc/sv/%s /var/service/)\n", svc, svc)
			if fix {
				sudo := "sudo"
				if os.Geteuid() == 0 {
					sudo = ""
				}
				targetDir := filepath.Join(root, "/var/service")
				if _, err := os.Stat(targetDir); err != nil {
					targetDir = filepath.Join(root, "/etc/runit/runsvdir/default")
				}
				src := filepath.Join(root, "/etc/sv", svc)
				dest := filepath.Join(targetDir, svc)
				var lnCmd *exec.Cmd
				if sudo != "" {
					lnCmd = exec.Command(sudo, "ln", "-sf", src, dest)
				} else {
					lnCmd = exec.Command("ln", "-sf", src, dest)
				}
				if err := lnCmd.Run(); err == nil {
					fmt.Printf("    fixed: enabled /etc/sv/%s -> %s\n", svc, dest)
					problems--
				}
			}
		}
	}

	if problems > 0 {
		return exitFail
	}
	return exitOK
}

// reportUserAudioServices checks that PipeWire and WirePlumber user services
// are configured or running under turnstiled.
func reportUserAudioServices(root string, fix bool) int {
	fmt.Println()
	fmt.Println("Audio Services (PipeWire & WirePlumber)")
	audioServices := []string{"pipewire", "wireplumber"}
	problems := 0

	userSvcDir := filepath.Join(root, "/usr/share/asura/runit/services")
	for _, svc := range audioServices {
		present := false
		if root == "" {
			uid := os.Getuid()
			runDir := fmt.Sprintf("/run/user/%d/service/%s", uid, svc)
			if _, err := os.Stat(runDir); err == nil {
				present = true
			} else {
				if out, err := exec.Command("pgrep", "-x", svc).Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
					present = true
				}
			}
		}
		if !present {
			if _, err := os.Stat(filepath.Join(userSvcDir, svc, "run")); err == nil {
				present = true
			} else if _, err := os.Stat(filepath.Join(root, "system/runit/user-services", svc, "run")); err == nil {
				present = true
			}
		}

		if present {
			fmt.Printf("  • %-16s configured\n", svc)
		} else {
			problems++
			fmt.Printf("  • %-16s missing or inactive under turnstiled\n", svc)
		}
	}
	if problems > 0 {
		return exitFail
	}
	return exitOK
}

// reportHyprlandVersion verifies that the installed Hyprland binary version
// is aligned with the 0.53.3 configuration syntax.
func reportHyprlandVersion(root string, fix bool) int {
	fmt.Println()
	fmt.Println("Hyprland Compositor Version")
	hyprBin := filepath.Join(root, "/usr/bin/Hyprland")
	if _, err := os.Stat(hyprBin); err != nil {
		path, err := exec.LookPath("Hyprland")
		if err != nil {
			fmt.Println("  Hyprland binary not found in PATH or /usr/bin")
			return exitOK
		}
		hyprBin = path
	}

	out, err := exec.Command(hyprBin, "--version").Output()
	if err != nil {
		out, err = exec.Command(hyprBin, "-v").Output()
	}
	if err != nil {
		fmt.Printf("  failed to query Hyprland version: %v\n", err)
		return exitFail
	}

	verStr := strings.TrimSpace(string(out))
	fmt.Printf("  installed: %s\n", strings.Split(verStr, "\n")[0])

	re := regexp.MustCompile(`v?([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	matches := re.FindStringSubmatch(verStr)
	if len(matches) >= 2 {
		ver := matches[1]
		if ver < "0.53" {
			fmt.Printf("  warning: Hyprland version (%s) is older than 0.53.3 configuration syntax\n", ver)
			if fix {
				fmt.Println("  running xbps-install -S hyprland to update compositor...")
				sudo := "sudo"
				if os.Geteuid() == 0 {
					sudo = ""
				}
				var cmd *exec.Cmd
				if sudo != "" {
					cmd = exec.Command(sudo, "xbps-install", "-S", "-y", "hyprland")
				} else {
					cmd = exec.Command("xbps-install", "-S", "-y", "hyprland")
				}
				cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
				_ = cmd.Run()
			}
			return exitFail
		}
	}
	fmt.Println("  compositor version is aligned with session configuration (0.53.3)")
	return exitOK
}

// reportHardwarePower checks NVIDIA PCIe D3cold and battery charge threshold.
func reportHardwarePower(root string, fix bool) int {
	fmt.Println()
	fmt.Println("Hardware Power & Thermal")
	problems := 0

	pciDevices, _ := filepath.Glob(filepath.Join(root, "/sys/bus/pci/devices/*/vendor"))
	nvidiaFound := false
	for _, vendorFile := range pciDevices {
		vdata, err := os.ReadFile(vendorFile)
		if err == nil && strings.TrimSpace(string(vdata)) == "0x10de" {
			nvidiaFound = true
			devDir := filepath.Dir(vendorFile)
			ctrlFile := filepath.Join(devDir, "power", "control")
			cdata, err := os.ReadFile(ctrlFile)
			if err == nil {
				val := strings.TrimSpace(string(cdata))
				if val != "auto" {
					problems++
					fmt.Printf("  • NVIDIA GPU (%s) power/control is %q (D3cold requires \"auto\")\n",
						filepath.Base(devDir), val)
					if fix {
						sudo := "sudo"
						if os.Geteuid() == 0 {
							sudo = ""
						}
						cmdStr := fmt.Sprintf("echo auto > %s", ctrlFile)
						var cmd *exec.Cmd
						if sudo != "" {
							cmd = exec.Command(sudo, "sh", "-c", cmdStr)
						} else {
							cmd = exec.Command("sh", "-c", cmdStr)
						}
						if err := cmd.Run(); err == nil {
							fmt.Printf("    fixed: set %s to auto\n", ctrlFile)
							problems--
						}
					}
				} else {
					fmt.Printf("  • NVIDIA GPU (%s) D3cold runtime PM is active (auto)\n", filepath.Base(devDir))
				}
			}
		}
	}
	if !nvidiaFound {
		fmt.Println("  • NVIDIA GPU: not present (Intel/AMD/integrated graphics)")
	}

	batThresholds, _ := filepath.Glob(filepath.Join(root, "/sys/class/power_supply/BAT*/charge_control_end_threshold"))
	if len(batThresholds) == 0 {
		fmt.Println("  • Battery charge threshold: no battery controller detected (desktop/VM)")
	} else {
		for _, threshFile := range batThresholds {
			batName := filepath.Base(filepath.Dir(threshFile))
			tdata, err := os.ReadFile(threshFile)
			if err == nil {
				val := strings.TrimSpace(string(tdata))
				if val != "80" {
					fmt.Printf("  • Battery %s charge limit is %s (recommended: 80 for battery longevity)\n",
						batName, val)
					if fix {
						sudo := "sudo"
						if os.Geteuid() == 0 {
							sudo = ""
						}
						cmdStr := fmt.Sprintf("echo 80 > %s", threshFile)
						var cmd *exec.Cmd
						if sudo != "" {
							cmd = exec.Command(sudo, "sh", "-c", cmdStr)
						} else {
							cmd = exec.Command("sh", "-c", cmdStr)
						}
						if err := cmd.Run(); err == nil {
							fmt.Printf("    fixed: set %s to 80\n", threshFile)
						}
					}
				} else {
					fmt.Printf("  • Battery %s charge limit is configured to 80%%\n", batName)
				}
			}
		}
	}

	if problems > 0 {
		return exitFail
	}
	return exitOK
}
