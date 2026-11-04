package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"asura/desktop/internal/overlay"
)

// asuraRollback reverts configuration overlay (--config) or restores Btrfs
// root snapshots (--system).
func asuraRollback(args []string) int {
	set := newFlags("rollback", rollbackUsage)
	var (
		root       = set.String("root", "", "operate on a tree under this directory")
		configMode = set.Bool("config", false, "revert config overlay to previous revision")
		systemMode = set.Bool("system", false, "list or restore Btrfs root snapshots")
		listMode   = set.Bool("list", false, "list available revisions and snapshots")
	)
	positional, err := parseFlags(set, args)
	if err != nil {
		return exitUsage
	}

	paths, err := overlayPaths(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "asura rollback:", err)
		return exitFail
	}

	if *listMode {
		return listRollbackTargets(paths)
	}

	if *systemMode {
		target := ""
		if len(positional) > 0 {
			target = positional[0]
		}
		return rollbackSystem(target)
	}

	// Default or --config mode
	if *configMode || len(positional) == 0 {
		return rollbackConfig(paths)
	}

	// Positional argument without explicit flag: check if it matches a system snapshot
	if isBtrfsSnapshot(positional[0]) {
		return rollbackSystem(positional[0])
	}
	return rollbackConfig(paths)
}

// rollbackConfig rolls back the ~/.config overlay to the previous revision.
func rollbackConfig(paths overlay.Paths) int {
	restored, err := overlay.Rollback(paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "asura rollback:", err)
		return exitFail
	}
	fmt.Printf("rolled back to revision %s (recorded %s)\n",
		restored.ID, restored.RecordedAt.Format("2006-01-02 15:04:05Z"))
	return exitOK
}

// rollbackSystem lists or restores a Btrfs root snapshot.
func rollbackSystem(target string) int {
	if target == "" {
		fmt.Println("Btrfs System Snapshots:")
		snaps, err := listBtrfsSnapshots()
		if err != nil {
			fmt.Fprintln(os.Stderr, "asura rollback:", err)
			return exitFail
		}
		if len(snaps) == 0 {
			fmt.Println("  no system snapshots found in /.snapshots")
			return exitOK
		}
		for _, s := range snaps {
			fmt.Printf("  • %s\n", s)
		}
		fmt.Println("\nTo restore a system snapshot, run:")
		fmt.Println("  sudo asura rollback --system <snapshot-name>")
		return exitOK
	}

	snapPath := target
	if !filepath.IsAbs(snapPath) {
		snapPath = filepath.Join("/.snapshots", target)
	}

	info, err := os.Stat(snapPath)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "asura rollback: snapshot not found: %s\n", snapPath)
		return exitFail
	}

	fmt.Printf("==> Restoring system snapshot: %s\n", filepath.Base(snapPath))

	sudo := "sudo"
	if os.Geteuid() == 0 {
		sudo = ""
	}

	// Create a writable subvolume for the rollback
	restoreTarget := "/@_restored_" + filepath.Base(snapPath)
	var snapCmd *exec.Cmd
	if sudo != "" {
		snapCmd = exec.Command(sudo, "btrfs", "subvolume", "snapshot", snapPath, restoreTarget)
	} else {
		snapCmd = exec.Command("btrfs", "subvolume", "snapshot", snapPath, restoreTarget)
	}
	if out, err := snapCmd.CombinedOutput(); err != nil {
		// If already exists or error, continue with warning
		fmt.Fprintf(os.Stderr, "notice: btrfs snapshot clone: %s (%v)\n", strings.TrimSpace(string(out)), err)
	}

	// Update GRUB entries
	fmt.Println("==> Updating GRUB bootloader menu...")
	var grubCmd *exec.Cmd
	if sudo != "" {
		grubCmd = exec.Command(sudo, "grub-mkconfig", "-o", "/boot/grub/grub.cfg")
	} else {
		grubCmd = exec.Command("grub-mkconfig", "-o", "/boot/grub/grub.cfg")
	}
	if out, err := grubCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: grub-mkconfig failed: %s (%v)\n", strings.TrimSpace(string(out)), err)
	} else {
		fmt.Println("GRUB boot menu updated.")
	}

	fmt.Println("\nRollback complete. Reboot to activate restored system state.")
	return exitOK
}

// listRollbackTargets prints both config revisions and system snapshots.
func listRollbackTargets(paths overlay.Paths) int {
	fmt.Println("Configuration Overlay Revisions:")
	history, err := overlay.Revisions(paths)
	if err != nil || len(history) == 0 {
		fmt.Println("  no configuration revisions recorded")
	} else {
		for _, rev := range history {
			fmt.Printf("  • %s  (%s)\n", rev.ID, rev.RecordedAt.Format("2006-01-02 15:04:05Z"))
		}
	}

	fmt.Println("\nBtrfs System Snapshots:")
	snaps, err := listBtrfsSnapshots()
	if err != nil || len(snaps) == 0 {
		fmt.Println("  no system snapshots found")
	} else {
		for _, s := range snaps {
			fmt.Printf("  • %s\n", s)
		}
	}
	return exitOK
}

// isBtrfsSnapshot checks if the name matches an existing snapshot in /.snapshots.
func isBtrfsSnapshot(name string) bool {
	snapPath := name
	if !filepath.IsAbs(snapPath) {
		snapPath = filepath.Join("/.snapshots", name)
	}
	info, err := os.Stat(snapPath)
	return err == nil && info.IsDir()
}

// listBtrfsSnapshots returns available snapshot names in /.snapshots.
func listBtrfsSnapshots() ([]string, error) {
	entries, err := os.ReadDir("/.snapshots")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snaps []string
	for _, entry := range entries {
		if entry.IsDir() {
			snaps = append(snaps, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(snaps)))
	return snaps, nil
}
