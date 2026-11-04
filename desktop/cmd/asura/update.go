package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"asura/desktop/internal/overlay"
	"asura/desktop/internal/runner"
)

// asuraUpdate runs the complete Void Linux system update lifecycle.
//
// Lifecycle sequence:
//  1. Snapshot ~/.config to git AND create an atomic Btrfs root snapshot
//     (/.snapshots/pre-update-<timestamp>) if / is on Btrfs.
//  2. Run system package updates via xbps-install -Su -y (routing through
//     asura-sysd socket /run/asura/sysd.sock or sudo xbps-install -Su -y).
//  3. Update installed Flatpaks via flatpak update -y (if flatpak is present).
//  4. Re-materialize the /usr/share/asura/ base overlay with ~/.config/asura/user_edits/
//     and trigger a live shell/daemon reload.
func asuraUpdate(args []string) int {
	set := newFlags("update", updateUsage)
	var (
		root       = set.String("root", "", "operate on a tree under this directory")
		dryRun     = set.Bool("dry-run", false, "do the snapshot and preserve steps, fetch nothing")
		noSnapshot = set.Bool("no-snapshot", false, "skip the config snapshot")
	)
	if err := set.Parse(args); err != nil {
		return exitUsage
	}

	paths, err := overlayPaths(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "asura update:", err)
		return exitFail
	}

	var snapshot func(overlay.Paths) error
	if !*noSnapshot {
		snapshot = func(p overlay.Paths) error {
			if err := gitSnapshot(p); err != nil {
				return err
			}
			if !*dryRun {
				if snap, err := createBtrfsSnapshot(); err != nil {
					fmt.Fprintf(os.Stderr, "asura update: btrfs snapshot: %v\n", err)
				} else if snap != "" {
					fmt.Printf("created system Btrfs snapshot: %s\n", snap)
				}
			}
			return nil
		}
	}

	var fetch func() error
	if !*dryRun {
		fetch = voidFetch
	} else {
		fetch = nil
	}

	// Step 1 & 2 & 4: snapshot, package update, and re-materialization
	if err := overlay.Update(paths, snapshot, fetch); err != nil {
		fmt.Fprintln(os.Stderr, "asura update:", err)
		return exitFail
	}

	// Step 3: Flatpak updates (if flatpak is present)
	if !*dryRun {
		updateFlatpaks()
	}

	status, err := overlay.StatusOf(paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "asura update:", err)
		return exitFail
	}
	fmt.Printf("updated to revision %s\n", status.Revision)
	fmt.Printf("config materialized; %d user edit(s), %d stale, %d missing\n",
		status.Edits, status.Stale, status.Missing)

	// Step 4b: Reconcile shell config and trigger live reload
	if msg, err := reconcileShellConfig(filepath.Join(paths.Config, "shell.json"), true); err != nil {
		fmt.Fprintln(os.Stderr, "asura update:", err)
	} else if msg != "" {
		fmt.Println(msg)
	}

	if !*dryRun {
		fmt.Println("triggering live shell and service reload...")
		reloadMain(nil, os.Stdout, os.Stderr, runner.Exec(15*time.Second))
	}

	return exitOK
}

// voidFetch executes the system package update via Void Linux XBPS.
// It first attempts to route through the privileged asura-sysd socket.
// If the daemon is not active, it falls back to direct xbps-install.
func voidFetch() error {
	fmt.Println("==> Step 2/4: Updating system packages (XBPS)...")

	// Try sysd socket first
	if err := runSysdPkgUpdate(); err == nil {
		fmt.Println("system packages updated via asura-sysd")
		return nil
	}

	// Fallback to local xbps-install command
	return runLocalXbpsUpdate()
}

// runSysdPkgUpdate communicates with asura-sysd over /run/asura/sysd.sock.
func runSysdPkgUpdate() error {
	conn, err := net.DialTimeout("unix", "/run/asura/sysd.sock", 2*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	req := struct {
		Action string `json:"action"`
	}{
		Action: "pkg_update",
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return err
	}

	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		var res struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &res); err != nil {
			return err
		}
		if res.Status != "ok" {
			return fmt.Errorf("sysd returned error: %s", res.Message)
		}
		return nil
	}
	return fmt.Errorf("no response from asura-sysd")
}

// runLocalXbpsUpdate executes xbps-install -Su -y locally with privilege elevation.
func runLocalXbpsUpdate() error {
	sudo := "sudo"
	if os.Geteuid() == 0 {
		sudo = ""
	}

	var syncCmd *exec.Cmd
	if sudo != "" {
		syncCmd = exec.Command(sudo, "xbps-install", "-Su", "-y")
	} else {
		syncCmd = exec.Command("xbps-install", "-Su", "-y")
	}
	syncCmd.Stdout, syncCmd.Stderr, syncCmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := syncCmd.Run(); err != nil {
		return fmt.Errorf("xbps-install: %w", err)
	}
	return nil
}

// updateFlatpaks executes flatpak update -y if Flatpak is installed.
func updateFlatpaks() {
	if _, err := exec.LookPath("flatpak"); err != nil {
		return
	}
	fmt.Println("==> Step 3/4: Updating installed Flatpak applications & runtimes...")
	cmd := exec.Command("flatpak", "update", "-y")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	_ = cmd.Run()
}

// isBtrfsRoot checks if the root filesystem is Btrfs.
func isBtrfsRoot() bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		out, err := exec.Command("stat", "-f", "-c", "%T", "/").Output()
		return err == nil && strings.TrimSpace(string(out)) == "btrfs"
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[1] == "/" && fields[2] == "btrfs" {
			return true
		}
	}
	return false
}

// createBtrfsSnapshot creates an atomic read-only snapshot of root in /.snapshots/.
func createBtrfsSnapshot() (string, error) {
	if !isBtrfsRoot() {
		return "", nil
	}
	if _, err := exec.LookPath("btrfs"); err != nil {
		return "", nil
	}

	snapDir := "/.snapshots"
	timestamp := time.Now().Format("20060102-150405")
	snapName := "pre-update-" + timestamp
	snapPath := filepath.Join(snapDir, snapName)

	sudo := "sudo"
	if os.Geteuid() == 0 {
		sudo = ""
	}

	// Ensure /.snapshots directory exists
	var mkdirCmd *exec.Cmd
	if sudo != "" {
		mkdirCmd = exec.Command(sudo, "mkdir", "-p", snapDir)
	} else {
		mkdirCmd = exec.Command("mkdir", "-p", snapDir)
	}
	_ = mkdirCmd.Run()

	var snapCmd *exec.Cmd
	if sudo != "" {
		snapCmd = exec.Command(sudo, "btrfs", "subvolume", "snapshot", "-r", "/", snapPath)
	} else {
		snapCmd = exec.Command("btrfs", "subvolume", "snapshot", "-r", "/", snapPath)
	}

	out, err := snapCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s (%s)", err, strings.TrimSpace(string(out)))
	}
	return snapPath, nil
}
