package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"zenith/desktop/internal/overlay"
)

// overlayPaths resolves the overlay model against the running user.
//
// --root overrides the base and the state directory so the whole command surface
// can be exercised against a fixture without root and without touching the
// machine. It is the same escape hatch the hardware commands have, for the same
// reason: the code that matters is the code a test can reach.
func overlayPaths(rootOverride string) (overlay.Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return overlay.Paths{}, fmt.Errorf("cannot determine the home directory: %w", err)
	}

	paths := overlay.DefaultPaths(home)
	if rootOverride != "" {
		paths.Base = filepath.Join(rootOverride, "usr", "share", "zenith")
		paths.Etc = filepath.Join(rootOverride, "etc", "zenith")
		paths.Config = filepath.Join(rootOverride, "home", ".config", "zenith")
		paths.Edits = filepath.Join(paths.Config, overlay.EditsDir)
		paths.State = filepath.Join(rootOverride, "home", ".local", "state", "zenith")
	}
	return paths, nil
}

// zenithUpdate runs the update sequence.
//
// The fetch step shells out to apt, which is the one thing here that needs root
// and the one thing that cannot be tested without it. Everything around it — the
// snapshot, the base preservation, the re-materialization — is ordinary code
// with tests, and it is the part where a mistake destroys somebody's work.
func zenithUpdate(args []string) int {
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
		fmt.Fprintln(os.Stderr, "zenith update:", err)
		return exitFail
	}

	var snapshot func(overlay.Paths) error
	if !*noSnapshot {
		snapshot = gitSnapshot
	}

	var fetch func() error
	if !*dryRun {
		fetch = aptFetch
	} else {
		fetch = nil
	}

	if err := overlay.Update(paths, snapshot, fetch); err != nil {
		fmt.Fprintln(os.Stderr, "zenith update:", err)
		return exitFail
	}

	status, err := overlay.StatusOf(paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith update:", err)
		return exitFail
	}
	fmt.Printf("updated to revision %s\n", status.Revision)
	fmt.Printf("config materialized; %d user edit(s), %d stale, %d missing\n",
		status.Edits, status.Stale, status.Missing)
	return exitOK
}

// zenithRollback reverts to the previous revision.
func zenithRollback(args []string) int {
	set := newFlags("rollback", rollbackUsage)
	var root = set.String("root", "", "operate on a tree under this directory")
	if err := set.Parse(args); err != nil {
		return exitUsage
	}

	paths, err := overlayPaths(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith rollback:", err)
		return exitFail
	}

	restored, err := overlay.Rollback(paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith rollback:", err)
		return exitFail
	}
	fmt.Printf("rolled back to revision %s (recorded %s)\n",
		restored.ID, restored.RecordedAt.Format("2006-01-02 15:04:05Z"))
	return exitOK
}

// zenithMaterialize re-lays the base, clobbering shipped content.
func zenithMaterialize(args []string) int {
	set := newFlags("materialize", materializeUsage)
	var root = set.String("root", "", "operate on a tree under this directory")
	positional, err := parseFlags(set, args)
	if err != nil {
		return exitUsage
	}
	if len(positional) > 0 {
		fmt.Fprintln(os.Stderr, "zenith materialize: takes no path; it re-lays "+
			"the whole base. Use `zenith reset <path>` for one file.")
		return exitUsage
	}

	paths, err := overlayPaths(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith materialize:", err)
		return exitFail
	}
	if err := overlay.Materialize(paths); err != nil {
		fmt.Fprintln(os.Stderr, "zenith materialize:", err)
		return exitFail
	}
	fmt.Println("base materialized; user edits preserved")
	return exitOK
}

// zenithReset drops one overlay file back to the base.
func zenithReset(args []string) int {
	set := newFlags("reset", resetUsage)
	var root = set.String("root", "", "operate on a tree under this directory")
	positional, err := parseFlags(set, args)
	if err != nil {
		return exitUsage
	}
	if len(positional) != 1 {
		fmt.Fprintln(os.Stderr, "zenith reset: needs exactly one path")
		return exitUsage
	}

	paths, err := overlayPaths(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith reset:", err)
		return exitFail
	}
	if err := overlay.Reset(paths, positional[0]); err != nil {
		fmt.Fprintln(os.Stderr, "zenith reset:", err)
		return exitFail
	}
	fmt.Printf("%s is back to the base\n", positional[0])
	return exitOK
}

// zenithDoctor reports drift between base and overlay.
//
// It changes nothing. A diagnostic that repairs is not a diagnostic, and the
// moment it repairs, a user cannot tell what was wrong before they changed it.
func zenithDoctor(args []string) int {
	set := newFlags("doctor", doctorUsage)
	var root = set.String("root", "", "operate on a tree under this directory")
	if err := set.Parse(args); err != nil {
		return exitUsage
	}

	paths, err := overlayPaths(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith doctor:", err)
		return exitFail
	}

	findings, err := overlay.Doctor(paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith doctor:", err)
		return exitFail
	}
	if len(findings) == 0 {
		fmt.Println("no drift: the config tree matches the base")
		return exitOK
	}

	counts := map[string]int{}
	for _, finding := range findings {
		fmt.Printf("%-10s %s\n", finding.Reason, finding.Path)
		counts[finding.Reason]++
	}

	fmt.Println()
	for _, reason := range []string{overlay.DriftStale, overlay.DriftMissing, overlay.DriftOrphan} {
		if counts[reason] > 0 {
			fmt.Printf("%d %s file(s) — `zenith materialize` fixes them, and "+
				"your edits are preserved\n", counts[reason], reason)
		}
	}
	if counts[overlay.DriftUserEdit] > 0 {
		fmt.Printf("%d user edit(s) — these are yours and survive every update\n",
			counts[overlay.DriftUserEdit])
	}
	return exitOK
}

// zenithStatus is the one-glance answer.
func zenithStatus(args []string) int {
	set := newFlags("status", statusUsage)
	var root = set.String("root", "", "operate on a tree under this directory")
	if err := set.Parse(args); err != nil {
		return exitUsage
	}

	paths, err := overlayPaths(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith status:", err)
		return exitFail
	}

	status, err := overlay.StatusOf(paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith status:", err)
		return exitFail
	}

	fmt.Printf("revision:  %s\n", orNone(status.Revision))
	fmt.Printf("previous:  %s\n", orNone(status.Previous))
	fmt.Printf("base:      %s\n", paths.Base)
	fmt.Printf("config:    %s\n", paths.Config)
	fmt.Printf("overlay:   %s\n", paths.Edits)
	fmt.Printf("edits:     %d\n", status.Edits)
	fmt.Printf("drift:     %d stale, %d missing\n", status.Stale, status.Missing)
	return exitOK
}

// gitSnapshot commits the user's config tree before an update.
//
// It runs git rather than reimplementing it because git already does this
// correctly, it is a reviewed package in the manifest, and a hand-rolled
// snapshot would have to grow conflict handling the moment two updates ran close
// together.
func gitSnapshot(paths overlay.Paths) error {
	if _, err := os.Stat(paths.Config); err != nil {
		if os.IsNotExist(err) {
			// Nothing to snapshot on a machine that has never materialized.
			return nil
		}
		return err
	}

	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.email", "zenith@localhost"},
		{"config", "user.name", "zenith"},
		{"add", "--all", "."},
		{"commit", "--allow-empty", "--quiet", "-m", "zenith: snapshot before update"},
	} {
		command := exec.Command("git", args...)
		command.Dir = paths.Config
		// The snapshot must never stop an update: a failure here is worth
		// reporting but it is not worth refusing to update over, and the base
		// preservation that follows is what makes an update recoverable anyway.
		command.Env = append(os.Environ(),
			"GIT_DIR="+filepath.Join(paths.Config, ".git"))
		if output, err := command.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "zenith update: git %v: %v\n%s\n",
				args, err, output)
			return nil
		}
	}
	return nil
}

// aptFetch fetches and installs the first-party packages.
//
// This is the one step that needs root and the one this project does not
// reimplement: apt already resolves dependencies, verifies signatures and
// records what it installed, and a second implementation would be a smaller,
// worse version of it.
func aptFetch() error {
	command := exec.Command("apt-get", "update")
	command.Stdout, command.Stderr = os.Stderr, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("apt-get update: %w", err)
	}

	command = exec.Command("apt-get", "install", "--yes", "zenith-desktop")
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("apt-get install zenith-desktop: %w", err)
	}
	return nil
}
