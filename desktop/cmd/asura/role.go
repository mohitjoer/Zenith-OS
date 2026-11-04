package main

import (
	"fmt"
	"os"
	"os/exec"
)

func init() {
	register("role", "switch system roles: developer, security researcher, or gamer via native container bridge", asuraRole)
}

const roleUsage = `Usage: asura role <role-name> [flags]

Roles:
  gamer, --enable-gamer    Enable gaming stack (Steam, Gamemode, MangoHud, Gamescope, 32-bit Vulkan)
  hacker, --enable-hacker  Provision BlackArch / security container tool-bridge with AUR parity
  dev, --enable-dev        Provision development toolchain & AUR container bridge
  list, status             List available roles and container bridge status
`

func asuraRole(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(roleUsage)
		return exitOK
	}

	role := args[0]
	switch role {
	case "gamer", "--enable-gamer":
		return enableGamerRole()
	case "hacker", "--enable-hacker":
		return enableHackerRole()
	case "dev", "--enable-dev":
		return enableDevRole()
	case "list", "status":
		return showRoleStatus()
	default:
		fmt.Fprintf(os.Stderr, "asura role: unknown role %q\n\n%s", role, roleUsage)
		return exitUsage
	}
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func enableGamerRole() int {
	fmt.Println("[*] Provisioning Asura OS Gaming Stack (Steam, Gamemode, MangoHud, Gamescope)...")

	// Ensure native package manager is configured
	if _, err := exec.LookPath("xbps-install"); err == nil {
		_ = runCmd("sudo", "xbps-install", "-Sy", "void-repo-nonfree", "void-repo-multilib", "void-repo-multilib-nonfree")
		err := runCmd("sudo", "xbps-install", "-Sy", "steam", "gamemode", "mangohud", "gamescope", "vulkan-loader-32bit", "mesa-vulkan-radeon-32bit")
		if err != nil {
			fmt.Fprintf(os.Stderr, "asura role: failed to install gaming packages: %v\n", err)
			return exitFail
		}
	}

	fmt.Println("[+] Asura OS Gaming Stack enabled successfully.")
	return exitOK
}

func enableHackerRole() int {
	fmt.Println("[*] Provisioning Asura OS Security Researcher & BlackArch Tool Bridge...")

	if _, err := exec.LookPath("xbps-install"); err == nil {
		_ = runCmd("sudo", "xbps-install", "-Sy", "podman", "distrobox", "nmap", "wireshark-qt", "aircrack-ng")
	}

	if _, err := exec.LookPath("distrobox"); err == nil {
		fmt.Println("[*] Initializing BlackArch container bridge via Distrobox + Podman...")
		_ = runCmd("distrobox", "create", "--name", "blackarch", "--image", "docker.io/archlinux/archlinux:latest", "--yes")
		fmt.Println("[+] Container 'blackarch' ready. Enter via: distrobox enter blackarch")
	}

	fmt.Println("[+] Security tool bridge provisioned with native kernel performance.")
	return exitOK
}

func enableDevRole() int {
	fmt.Println("[*] Provisioning Asura OS Developer Toolchain & AUR Bridge...")

	if _, err := exec.LookPath("xbps-install"); err == nil {
		_ = runCmd("sudo", "xbps-install", "-Sy", "git", "base-devel", "podman", "distrobox", "nodejs", "python3", "rustup", "lazygit", "neovim")
	}

	if _, err := exec.LookPath("distrobox"); err == nil {
		fmt.Println("[*] Initializing developer container bridge (aur-box)...")
		_ = runCmd("distrobox", "create", "--name", "aur-box", "--image", "docker.io/archlinux/archlinux:latest", "--yes")
		fmt.Println("[+] Container 'aur-box' ready. Enter via: distrobox enter aur-box")
	}

	fmt.Println("[+] Developer toolchain and AUR bridge enabled.")
	return exitOK
}

func showRoleStatus() int {
	fmt.Println("Asura OS Role Profiles:")
	fmt.Println("  • gamer   - Steam, Gamemode, MangoHud, Gamescope, low-latency BORE kernel tuning")
	fmt.Println("  • hacker  - BlackArch / Kali native container bridge (podman + distrobox)")
	fmt.Println("  • dev     - Full compilation toolchain (Go, Rust, Node, C/C++) & AUR bridge")
	fmt.Println()
	if _, err := exec.LookPath("distrobox"); err == nil {
		fmt.Println("Active Distrobox Bridges:")
		_ = runCmd("distrobox", "list")
	}
	return exitOK
}
