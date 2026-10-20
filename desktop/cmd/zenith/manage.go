package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"zenith/desktop/internal/hub"
	"zenith/desktop/internal/overlay"
	"zenith/desktop/internal/portal"
	"zenith/desktop/internal/shell"
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
	// The portal assertion runs whatever this finds. An early return here would
	// skip it on every healthy machine, which is exactly the machine where
	// nothing else has gone wrong and the one where a silently-misread portal
	// configuration would go unnoticed.
	if len(findings) == 0 {
		fmt.Println("no drift: the config tree matches the base")
		return reportSessionChecks(*root)
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

	// The portal assertion, which the architecture record requires doctor to
	// make. It is here rather than in the overlay report because it is a
	// different kind of check: not drift between files, but whether the session
	// is declaring itself so that the portal dispatcher reads the right
	// configuration at all.
	//
	// That failure is silent. A session that does not declare itself correctly
	// finds no per-desktop configuration, falls back to the system default, and
	// quietly uses whatever that names — on a machine adopted from GNOME that
	// is a dispatcher trying to activate a backend that is not installed. Nothing
	// is printed and nothing fails, and the symptom is a file chooser that is
	// wrong.
	return reportSessionChecks(*root)
}

// reportSessionChecks runs the checks that are not about drift between the base
// and the overlay. Both are here rather than in the overlay report because they
// answer a different question: not "what changed" but "is this machine wired
// up correctly at all".
//
// It is one function called from both return paths on purpose. An early return
// on the clean case would skip every one of these checks on exactly the machine
// where nothing else has gone wrong, which is the machine where a silently
// misread portal configuration or a registration pointing at nothing would go
// unnoticed.
func reportSessionChecks(root string) int {
	code := reportPortals(root)
	if registrationCode := reportRegistrations(root); registrationCode != exitOK {
		code = registrationCode
	}
	if shellCode := reportShell(root); shellCode != exitOK {
		code = shellCode
	}
	if hubCode := reportHub(root); hubCode != exitOK {
		code = hubCode
	}
	return code
}

// reportHub checks the claim that every setting has a Hub page and every page
// has a command.
//
// That sentence in the architecture record is a claim about two lists — the keys
// the control tool can reach and the pages the Hub draws — and nothing else
// compares them. Both directions fail silently: a key with no page is a setting
// only a command line can change, and a page with no command is a control the
// user can see and use that either does nothing or writes dconf behind the
// tool's back.
func reportHub(root string) int {
	fmt.Println()
	fmt.Println("Settings and Hub pages")

	catalogue, err := settingsCatalogue(root)
	if err != nil {
		fmt.Printf("  problem           %v\n", err)
		return exitFail
	}

	// Two claims, both of which only this compares. hub.Check covers pages
	// against commands; CheckControls covers types against widgets. The second
	// was written and tested and never called, so a setting type with no
	// control passed doctor and then rendered as a row nobody can change —
	// the failure the rule exists to name, found by a function nothing invoked.
	findings := append(hub.Check(catalogue), hub.CheckControls(catalogue)...)
	if len(findings) == 0 {
		fmt.Printf("  all %d setting(s) have a page, and all %d page(s) have "+
			"commands", len(catalogue.Refs()), len(hub.Pages))
		return exitOK
	}
	for _, finding := range findings {
		fmt.Printf("  %s\n", finding)
	}
	return exitFail
}

// reportShell checks the shell's source.
//
// It cannot run the shell: Quickshell is built from source and is not in the
// archive, so there is no quickshell here to run and no qmllint to check the
// QML with. What it can do is check the things that are silent at run time — a
// widget catalogue naming a file that is not there, a surface reading a
// property the settings wrapper does not declare, a QML file naming a path
// outside the boundary. Each of those produces a shell that comes up slightly
// wrong, with nothing in the journal saying why.
//
// What it cannot do is check behaviour, and this does not pretend otherwise.
func reportShell(root string) int {
	fmt.Println()
	fmt.Println("Shell")

	// Two windows, two trees, and they are checked differently on purpose.
	//
	// The desktop shell owns the schemas and is checked against them. The Hub
	// does not: it renders what `zenith config export` hands it and never reads a
	// schema, so running the shell's settings checks over it would report every
	// property only the shell uses as an unused setting — true of the wrong tree.
	// SchemaDir is set on the shell's tree only, and it is not the shell's own
	// directory: GLib installs schemas under /usr/share/glib-2.0/schemas. Checking
	// for them beside the QML reported "declares no schema id" and then "no
	// surface can reach it" for every key — two failures that do not exist on
	// the installed machine, which is the only machine doctor is run on.
	trees := []shell.Env{
		{Root: root, Dir: "/usr/share/zenith/shell", EntryPoint: "shell.qml",
			OwnsSettings: true, SchemaDir: settingsDir,
			HyprlandConfig: "usr/share/zenith/hyprland/hyprland.conf"},
		{Root: root, Dir: "/usr/share/zenith/hub", EntryPoint: "hub.qml"},
	}

	problems := 0
	for _, env := range trees {
		for _, finding := range shell.Check(env) {
			fmt.Printf("  %-22s %s: %s\n", finding.Rule, finding.Path, finding.Detail)
			problems++
		}
	}
	if problems == 0 {
		fmt.Printf("  source checks pass for %d window(s)", len(trees))
		fmt.Println()
		fmt.Println("  note    behaviour is unverified: no quickshell on this machine")
		return exitOK
	}
	return exitFail
}

// reportRegistrations asserts that every registration is installed and that what
// it points at exists.
//
// The architecture record requires this of doctor, and the reason is that both
// failures are silent in the same way: a registration that is missing means the
// desktop does not appear, and a registration pointing at nothing means a unit
// fails 203/EXEC once per login. Neither warns at build time.
func reportRegistrations(root string) int {
	fmt.Println()
	fmt.Println("Registrations")

	// Root is a field the package has always had and doctor has never set, so
	// every path in the report was the live machine's even when --root named
	// another tree. A check that reads the wrong tree is worse than one that
	// reads none: it reports "not installed" about files that are installed,
	// and the reader concludes the package is broken.
	env := portal.RegistrationEnv{Root: root}
	findings := append(portal.CheckRegistrations(env), portal.CheckUnits(env)...)

	if len(findings) == 0 {
		fmt.Printf("  all %d registration(s) installed and pointing at content",
			len(portal.Registrations()))
		return exitOK
	}
	for _, finding := range findings {
		fmt.Printf("  %-22s %s\n", finding.Name, finding.Reason)
	}
	return exitFail
}

// reportPortals asserts that the portal configuration is actually being read.
func reportPortals(root string) int {
	fmt.Println()
	fmt.Println("Portals")

	// The session identity is the running session, not a file, so it is read
	// from the environment either way. Everything else the resolver looks for is
	// a path, and under --root those paths live in the tree.
	xdgConfigHome, home := os.Getenv("XDG_CONFIG_HOME"), homeDir()
	if root != "" {
		// An unset XDG_CONFIG_HOME means ~/.config of the home directory, and
		// joining that before the root would otherwise leave config home as the
		// root itself — a directory nobody keeps portal configuration in, which
		// is how an empty search and a broken one look the same.
		if xdgConfigHome == "" {
			xdgConfigHome = filepath.Join(home, ".config")
		}
		xdgConfigHome = filepath.Join(root, xdgConfigHome)
		home = filepath.Join(root, home)
	}
	resolution := portal.Resolve(portal.Env{
		XDGConfigHome:  xdgConfigHome,
		Home:           home,
		CurrentDesktop: os.Getenv("XDG_CURRENT_DESKTOP"),
		Root:           root,
	})

	fmt.Printf("  session identity  %s (looks for %s)\n",
		orNone(resolution.CurrentDesktop), resolution.ConfigFile)

	if resolution.Source == portal.SourceNone {
		fmt.Println("  configuration     none found")
	} else {
		fmt.Printf("  configuration     %s (%s)\n", resolution.Source,
			resolution.ConfigPath)
	}

	for _, notice := range resolution.Notices {
		fmt.Printf("  known limit       %s\n", notice)
	}

	if len(resolution.Problems) == 0 {
		if len(resolution.Notices) == 0 {
			fmt.Println("  backends          every interface has the backend it needs")
		}
		// A known limit is not a misconfiguration. Failing here would train
		// users to ignore the exit code, and they have no lever on it.
		return exitOK
	}
	for _, problem := range resolution.Problems {
		fmt.Printf("  problem           %s\n", problem)
	}
	return exitFail
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
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

	command = exec.Command("apt-get", "install", "--yes", "zenith")
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("apt-get install zenith: %w", err)
	}
	return nil
}
