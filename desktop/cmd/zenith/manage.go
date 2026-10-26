package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"zenith/desktop/internal/hub"
	"zenith/desktop/internal/overlay"
	"zenith/desktop/internal/portal"
	"zenith/desktop/internal/runner"
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

	// materialize never rewrites a user's shell.json, so the one value Zenith
	// renamed is fixed here. It is reported and never fails the update: the
	// packages are already in.
	if msg, err := reconcileShellConfig(filepath.Join(paths.Config, "shell.json"), true); err != nil {
		fmt.Fprintln(os.Stderr, "zenith update:", err)
	} else if msg != "" {
		fmt.Println(msg)
	}
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
// It changes nothing, unless it is asked to with --fix, and then only what the
// config section below names. A diagnostic that repairs on its own is not a
// diagnostic, and the moment it repairs, a user cannot tell what was wrong
// before they changed it.
func zenithDoctor(args []string) int {
	set := newFlags("doctor", doctorUsage)
	var (
		root = set.String("root", "", "operate on a tree under this directory")
		fix  = set.Bool("fix", false, "apply the fixes the config section offers")
	)
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
		return reportSessionChecks(*root, paths, *fix)
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
	return reportSessionChecks(*root, paths, *fix)
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
func reportSessionChecks(root string, paths overlay.Paths, fix bool) int {
	code := reportPortals(root)
	if registrationCode := reportRegistrations(root); registrationCode != exitOK {
		code = registrationCode
	}
	if sessionCode := reportSession(root); sessionCode != exitOK {
		code = sessionCode
	}
	if shellCode := reportShell(root); shellCode != exitOK {
		code = shellCode
	}
	if hubCode := reportHub(root); hubCode != exitOK {
		code = hubCode
	}
	if configCode := reportConfig(paths, fix); configCode != exitOK {
		code = configCode
	}
	if commandsCode := reportCommands(root); commandsCode != exitOK {
		code = commandsCode
	}
	reportDeps(root, runner.Exec(10*time.Second))
	return code
}

// reportConfig names what the user's own config carries that Zenith has since
// renamed, and with --fix renames it. See reconcile.go.
//
// A value left to be renamed is not a failure: the next `zenith update` renames
// it too, and a doctor that exited non-zero over it would train people to stop
// reading the exit code.
func reportConfig(paths overlay.Paths, fix bool) int {
	fmt.Println()
	fmt.Println("Config")
	msg, err := reconcileShellConfig(filepath.Join(paths.Config, "shell.json"), fix)
	switch {
	case err != nil:
		fmt.Printf("  unreadable        %v\n", err)
		return exitFail
	case msg == "":
		fmt.Println("  shell.json        nothing renamed since it was written")
	default:
		fmt.Printf("  %s\n", msg)
	}
	return exitOK
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
// It cannot run the shell. A behaviour check needs a compositor, a display and
// a logged-in session, and doctor is none of those — so what it can do is check
// the things that are silent at run time: a widget catalogue naming a file that
// is not there, a surface reading a property the settings wrapper does not
// declare, a QML file naming a path outside the boundary, a keybind calling a
// function the shell does not have. Each of those produces a shell that comes
// up slightly wrong, with nothing in the journal saying why.
//
// What it cannot do is check behaviour, and this does not pretend otherwise.
func reportShell(root string) int {
	fmt.Println()
	fmt.Println("Shell")

	// One tree and one pass. Both windows live in the shell's own directory: the
	// Hub is drawn with the same component kit as the desktop, and the kit reaches
	// itself through imports that only resolve when the configuration's root is
	// that directory. Two entry files in it — `shell.qml` for the desktop and
	// `hub.qml` for the Hub — are two configurations and one tree.
	//
	// The desktop shell owns the schemas and is checked against them. The Hub does
	// not: it renders what `zenith config export` hands it and never reads a
	// schema, so running the shell's settings checks over it would report every
	// property only the shell uses as an unused setting — true of the wrong tree.
	// A tree cannot be split by directory when both windows are one directory, and
	// a second pass over it would print every finding twice, so the Hub is checked
	// here and its own entry file is checked by shell/shell_test.sh, which loads
	// it — a stronger claim than the file being present.
	//
	// SchemaDir is not the shell's own directory: GLib installs schemas under
	// /usr/share/glib-2.0/schemas. Checking for them beside the QML reported
	// "declares no schema id" and then "no surface can reach it" for every key —
	// two failures that do not exist on the installed machine, which is the only
	// machine doctor is run on.
	// The paths are absolute, and they are still relative to the root: the
	// root is the tree being checked and the path is where the file is in
	// it, so a root of /usr and a path of /usr/share/zenith/shell are the
	// same directory. Written without the leading slash they are relative to
	// the working directory instead, and the checks read nothing at all on
	// the one machine they are meant for: a doctor that silently checks
	// nothing looks exactly like a doctor that found nothing.
	trees := []shell.Env{
		{Root: root, Dir: "/usr/share/zenith/shell", EntryPoint: "shell.qml",
			OwnsSettings: true, SchemaDir: settingsDir,
			HyprlandConfig: "/usr/share/zenith/hyprland/hyprland.conf",
			BindsCatalogue: "/usr/share/zenith/session/binds.json"},
	}

	problems := 0
	for _, env := range trees {
		for _, finding := range shell.Check(env) {
			fmt.Printf("  %-22s %s: %s\n", finding.Rule, finding.Path, finding.Detail)
			problems++
		}
	}

	// The vendored tree, reported on its own.
	//
	// The rules above are about Zenith's own code. The vendored tree is
	// upstream's: it is a shipped file that is never edited in place, so it
	// is checked against the manifest instead — every file byte for byte
	// against the commit it was pinned to — and its state is a line of its
	// own rather than another rule.
	//
	// The modules it imports are reported rather than failed for the same
	// reason. A vendored component that imports a module this image does
	// not carry is a fact about the image's package list, and the fix is a
	// package in the image rather than an edit to a file nobody may edit.
	vendor := shell.CheckVendored(trees[0])
	switch {
	case !vendor.Manifested:
		fmt.Printf("  %-22s not installed; there is no manifest at %s\n",
			"vendored tree", filepath.Join(trees[0].Dir, "vendor", "manifest.yaml"))
	case vendor.Drifted == 0:
		fmt.Printf("  %-22s %d file(s) byte-identical to their pinned "+
			"upstream commits, and %d declared override(s)\n",
			"vendored tree", vendor.Files, vendor.Declared)
	default:
		fmt.Printf("  %-22s %d file(s) checked against the manifest, %d of "+
			"them drifted, and %d declared override(s)\n",
			"vendored tree", vendor.Files, vendor.Drifted, vendor.Declared)
	}
	if len(vendor.Modules) > 0 {
		fmt.Printf("  %-22s %d QML module(s) the vendored tree needs: %s\n",
			"vendored imports", len(vendor.Modules),
			strings.Join(vendor.Modules, ", "))
		switch missing := vendor.Missing(); {
		case !vendor.Packages:
			fmt.Printf("  %-22s the image's package manifest is not in this "+
				"tree; this is what the tree needs, not a gap in the image\n",
				"vendored packages")
		case len(missing) == 0:
			fmt.Printf("  %-22s every one of them is carried by a package in "+
				"the image's manifest\n", "vendored packages")
		default:
			fmt.Printf("  %-22s %d of them are not in the image's package "+
				"manifest: %s\n", "vendored packages", len(missing),
				strings.Join(missing, ", "))
		}
	}

	if problems == 0 {
		fmt.Printf("  source checks pass for %d window(s)", len(trees))
		fmt.Println()
		// Whether quickshell is installed is a fact about the machine doctor is
		// run on, and the note used to assert one answer to it. Quickshell is
		// built from this project rather than taken from the archive, so "no
		// quickshell here" stopped being the default case; the line now says
		// which case it is looking at. Neither case claims to have run the
		// shell, because neither has.
		if _, err := exec.LookPath("quickshell"); err != nil {
			fmt.Println("  note    behaviour is unverified: no quickshell on this machine")
		} else {
			fmt.Println("  note    behaviour is unverified: quickshell is installed, and " +
				"nothing here runs it")
		}
		return exitOK
	}
	return exitFail
}

// installedCompositorConfig is the compositor's entry file and every .conf
// beside it. The binds and the autostart live in files the entry sources, and a
// check that only asks whether something is said anywhere is not worth a
// parser for source lines: reading the directory is what the shell's own bind
// checks do too.
func installedCompositorConfig(root, entry string) (string, error) {
	body, err := os.ReadFile(filepath.Join(root, entry))
	if err != nil {
		return "", err
	}
	siblings, _ := filepath.Glob(filepath.Join(root, filepath.Dir(entry), "*.conf"))
	for _, sibling := range siblings {
		if filepath.Base(sibling) == filepath.Base(entry) {
			continue
		}
		if more, err := os.ReadFile(sibling); err == nil {
			body = append(append(body, '\n'), more...)
		}
	}
	return string(body), nil
}

// reportSession asserts that the session can actually reach the shell.
//
// Everything checked here is one link in a chain that ends with a grey screen
// and a keyboard that does nothing: the compositor runs a script, the script
// starts a target, the target starts the shell, and the keybinds call the
// shell by the path it was installed at.
//
// Every one of those links has been wrong on this project, in a different way,
// and none of them said anything in a log. A script that was not executable
// runs nothing; a compositor configuration that does not mention it starts
// nothing; a keybind that names the wrong path calls nothing. The session
// looked identical in every case: a compositor, and no desktop.
//
// The checks are against the installed tree, because that is the tree the
// login manager runs. A session that works in the checkout and not on the
// machine is the failure this is for.
func reportSession(root string) int {
	fmt.Println()
	fmt.Println("Session")

	problems := 0

	// The script the compositor's configuration runs. It has to be there and
	// it has to be executable: a script that is not executable is a compositor
	// that runs nothing, and the compositor says nothing about it.
	const activate = "/usr/share/zenith/session/zenith-activate"
	activatePath := filepath.Join(root, activate)
	info, err := os.Stat(activatePath)
	switch {
	case err != nil:
		fmt.Printf("  %-22s %s is not installed; the session cannot start "+
			"the shell\n", "activation", activate)
		problems++
	case info.Mode().Perm()&0111 == 0:
		fmt.Printf("  %-22s %s is not executable; the session cannot start "+
			"the shell\n", "activation", activate)
		problems++
	default:
		fmt.Printf("  %-22s %s\n", "activation", activate)
	}

	// The compositor's configuration has to run it. A configuration that does
	// not mention the script is a session that starts a compositor and
	// nothing else.
	const compositorConfig = "/usr/share/zenith/hyprland/hyprland.conf"
	config, err := installedCompositorConfig(root, compositorConfig)
	if err != nil {
		fmt.Printf("  %-22s %s is not installed\n", "compositor", compositorConfig)
		problems++
	} else if !strings.Contains(config, "exec-once = "+activate) {
		fmt.Printf("  %-22s %s does not start the session; the shell will "+
			"not come up\n", "compositor", compositorConfig)
		problems++
	} else {
		fmt.Printf("  %-22s %s starts the session\n", "compositor", compositorConfig)
	}

	// The keybinds call the shell through the zenith CLI, which finds the
	// running instance rather than naming one, because the shell the login
	// manager runs and the shell a development tree runs are different trees
	// and the compositor cannot tell them apart. The older form of the call
	// names the installed path plainly; both of them are the keybinds
	// reaching a shell, and a configuration that reaches neither is the fault.
	const shellDir = "/usr/share/zenith/shell"
	shellPath := filepath.Join(root, shellDir)
	if _, err := os.Stat(filepath.Join(shellPath, "shell.qml")); err != nil {
		fmt.Printf("  %-22s %s is not installed\n", "keybinds", shellDir)
		problems++
	} else {
		ipc, err := installedCompositorConfig(root, compositorConfig)
		if err != nil {
			fmt.Printf("  %-22s the compositor's configuration is not installed\n",
				"keybinds")
			problems++
		} else {
			body := ipc
			switch {
			case strings.Contains(body, "zenith ipc call"):
				fmt.Printf("  %-22s call the shell through the zenith CLI, which "+
					"finds it running\n", "keybinds")
			case strings.Contains(body, "--path "+shellDir):
				fmt.Printf("  %-22s call %s\n", "keybinds", shellDir)
			default:
				fmt.Printf("  %-22s the keybinds reach no shell: they name "+
					"neither the installed path nor the CLI that finds the "+
					"running one\n", "keybinds")
				problems++
			}
		}
	}

	if problems == 0 {
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
	var (
		root   = set.String("root", "", "operate on a tree under this directory")
		asJSON = set.Bool("json", false, "print the update report the shell reads, as one JSON line")
	)
	if err := set.Parse(args); err != nil {
		return exitUsage
	}

	// The daemon calls this on a timer and publishes the line unread. It is
	// answered before the overlay is touched: the report is about packages, and
	// an update check that failed because of the config tree would be silent.
	if *asJSON {
		return statusJSON(*root, os.Stdout, runner.Exec(2*time.Minute))
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

	// The running machine is not part of a --root tree, so a fixture is not
	// asked about it: the answer would be about the wrong system.
	if *root == "" {
		printLiveStatus(os.Stdout, runner.Exec(5*time.Second))
	}
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
// We run under sudo so zenith update works without manual elevation.
func aptFetch() error {
	sudo := "sudo"
	// If already root, don't prefix sudo.
	if os.Geteuid() == 0 {
		sudo = ""
	}
	var updateCmd *exec.Cmd
	if sudo != "" {
		updateCmd = exec.Command(sudo, "apt-get", "update")
	} else {
		updateCmd = exec.Command("apt-get", "update")
	}
	updateCmd.Stdout, updateCmd.Stderr = os.Stderr, os.Stderr
	if err := updateCmd.Run(); err != nil {
		if sudo != "" {
			return fmt.Errorf("apt-get update: %w (run \"sudo zenith update\" in a terminal to enter your password)", err)
		}
		return fmt.Errorf("apt-get update: %w", err)
	}

	var installCmd *exec.Cmd
	if sudo != "" {
		installCmd = exec.Command(sudo, "apt-get", "install", "--yes", "zenith")
	} else {
		installCmd = exec.Command("apt-get", "install", "--yes", "zenith")
	}
	installCmd.Stdout, installCmd.Stderr = os.Stdout, os.Stderr
	if err := installCmd.Run(); err != nil {
		if sudo != "" {
			return fmt.Errorf("apt-get install zenith: %w (run \"sudo zenith update\" in a terminal to enter your password)", err)
		}
		return fmt.Errorf("apt-get install zenith: %w", err)
	}
	return nil
}
