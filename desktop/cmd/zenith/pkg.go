package main

// pkg.go carries `zenith pkg`, the engine zpm stands on. The two names are
// one tool split by audience: zpm is the six letters a person types on an
// install line, and `zenith pkg` is the same verbs inside the control tool,
// where every other desktop action already lives.
//
// Routing, from the base layer out: a package name goes through the base
// system's package manager — xbps on the Void base, apt where the development
// machines live — and a name that carries an application id (which is a
// Flatpak ref, org.mozilla.firefox) goes to Flatpak. The routing is a prefix
// rule because it is the only split that survives contact with a
// distribution's own package set; what it will not do is guess a remote for
// one ref and not another.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"zenith/desktop/internal/runner"
)

const pkgUsage = `Usage: zenith pkg [-flatpak] [-repo URL] verb [package...]

Verbs: install, remove, search, update, upgrade.

The package manager is picked by what the machine runs: xbps when the Void
base is on, apt otherwise. A package name that carries an application id
(org.mozilla.firefox) is routed to flatpak, and --flatpak forces that; the
route must already have its remote enabled, because nothing here chooses one.

update and upgrade take no packages; the other verbs take one or more. The
verbs install, remove and update run the system managers behind sudo, and the
password is asked for in the terminal that runs the command.
`

func init() {
	register("pkg", "install, remove, search or move packages (the verbs zpm calls)", zenithPkg)
}

// pkgSudoPick decides the privilege prefix a system package manager runs
// behind. It is a variable because the tests run as the same user the
// operator does, and a check right here would make the test table a
// statement about whoever runs it.
var pkgSudoPick = func() []string {
	if os.Geteuid() == 0 {
		return nil
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return nil
	}
	return []string{"sudo"}
}

// pkgBackend answers the system package manager this machine runs: "xbps"
// under the Void base, "apt" under the development base, "" when neither's
// tools are here. It reads what is installed rather than a compiled-in
// answer, because a machine can carry both tool families without running
// either's base.
func pkgBackend() string {
	if _, err := exec.LookPath("xbps-install"); err == nil {
		return "xbps"
	}
	if _, err := exec.LookPath("apt-get"); err == nil {
		return "apt"
	}
	return ""
}

// pkgVerbs is the verb table per backend, WITHOUT the sudo prefix and
// without the package names. The flags are the manager's own — xbps installs
// with -S (sync) -y (assume-yes), asks about a repo with -R, searches with
// xbps-query -Rs; apt splits search into `apt search` because apt-get has no
// search verb; flatpak's three mutating verbs take -y to stay out of the way.
//
// Verbs that carry no packages live in pkgNeedsPackages, which is how the
// table's shape — one row per backend-verb pair, no special cases inside —
// stays reviewable.
var pkgVerbs = map[string]map[string][]string{
	"xbps": {
		"install": {"xbps-install", "-S", "-y"},
		"update":  {"xbps-install", "-S"},
		"upgrade": {"xbps-install", "-Su"},
		"remove":  {"xbps-remove", "-y"},
		"search":  {"xbps-query", "-Rs"},
	},
	"apt": {
		"install": {"apt-get", "install", "--yes"},
		"update":  {"apt-get", "update"},
		"upgrade": {"apt-get", "full-upgrade", "--yes"},
		"remove":  {"apt-get", "remove", "--yes"},
		"search":  {"apt", "search"},
	},
	"flatpak": {
		"install": {"flatpak", "install", "-y"},
		"update":  {"flatpak", "update", "-y"},
		"upgrade": {"flatpak", "update", "-y"},
		"remove":  {"flatpak", "uninstall", "-y"},
		"search":  {"flatpak", "search"},
	},
}

// pkgNeedsPackages says which verbs refuse to run without at least one.
var pkgVerbsNeedPackages = map[string]bool{"install": true, "remove": true, "search": true}

// pkgArgv builds the argv one verb runs, or false when the backend has no
// such verb, the verb needs packages and got none, or the backend is unknown
// — and a false with the usage text is what a caller does with it.
func pkgArgv(backend, verb string, packages []string, repo string) ([]string, bool) {
	verbs, known := pkgVerbs[backend]
	if !known {
		return nil, false
	}
	base, known := verbs[verb]
	if !known {
		return nil, false
	}
	if pkgVerbsNeedPackages[verb] && len(packages) == 0 {
		return nil, false
	}

	argv := append([]string{}, base...)
	if repo != "" && backend == "xbps" {
		// The repository is the first thing after xbps's own name, so the
		// install reads xbps-install -R URL -S -y: the manual's own shape.
		argv = append([]string{argv[0], "-R", repo}, argv[1:]...)
	}

	switch {
	case backend == "flatpak":
		// Flatpak prompts for its own confirmation on system-wide
		// operations; -y is the answer. sudo is not folded in: unprivileged
		// installs and searches are the normal case.
	case verb == "search":
		// A search mutates nothing; elevation only makes it wait on a prompt
		// the person running it did not expect.
	default:
		argv = append(pkgSudoPick(), argv...)
	}

	return append(argv, packages...), true
}

// pkgMain runs one verb against one delivered backend. The backend is a
// parameter because the tests run on machines of both bases and the
// detection helper answers about the real one; the command entry delivers
// "" and lets pkgBackend speak for this machine.
func pkgMain(args []string, out, errOut io.Writer, run runner.Func, backend string) int {
	set := newFlags("pkg", pkgUsage)
	var (
		forceFlatpak = set.Bool("flatpak", false, "route the whole command to flatpak")
		repoURL      = set.String("repo", "", "restrict the fetch to this repository (xbps only)")
	)
	positional, err := parseFlags(set, args)
	if err != nil {
		fmt.Fprint(errOut, pkgUsage)
		return exitUsage
	}
	if len(positional) == 0 {
		fmt.Fprint(errOut, pkgUsage)
		return exitUsage
	}
	verb, packages := positional[0], positional[1:]

	chosen := backend
	if chosen == "" {
		chosen = pkgBackend()
	}
	if *forceFlatpak {
		chosen = "flatpak"
	} else {
		for _, pkg := range packages {
			// A carrying `/` or reverse-DNS application ID (e.g. org.mozilla.firefox)
			// is the shape of a Flatpak ref.
			if strings.Contains(pkg, "/") || isFlatpakRef(pkg) {
				chosen = "flatpak"
				break
			}
		}
	}

	argv, ok := pkgArgv(chosen, verb, packages, *repoURL)
	if !ok {
		if chosen == "" {
			fmt.Fprintln(errOut, "zenith pkg: neither xbps nor apt is on this machine; "+
				"install its packages from your own package manager")
		} else {
			fmt.Fprintf(errOut, "zenith pkg: the %s backend cannot perform %s\n", chosen, verb)
		}
		fmt.Fprint(errOut, pkgUsage)
		return exitUsage
	}

	// Exit codes are the managers', passed through: an install that fails
	// with XBPS's exit 1 is reported with the reason that produced it.
	outText, err := run(argv[0], argv[1:]...)
	if err != nil {
		fmt.Fprintf(errOut, "zenith pkg: %s: %s\n", strings.Join(argv, " "), runner.Reason(err))
		return exitFail
	}
	fmt.Fprint(out, outText)
	return exitOK
}

func zenithPkg(args []string) int {
	return pkgMain(args, os.Stdout, os.Stderr, runner.Exec(30*time.Minute), "")
}

// zenFetch is the fetch inside `zenith update`: one package, the control
// tool's own, by the locked manager the base runs. The apt lane is the code
// the update flow has carried since it was the only lane and it is imported
// from manage.go; the xbps lane is the same promise over the same package,
// and its own update step syncs repositories as it installs, which is what
// apt-get needed two commands for.
func zenFetch() error {
	switch pkgBackend() {
	case "xbps":
		argv, ok := pkgArgv("xbps", "install", []string{"zenith"}, "")
		if !ok {
			return fmt.Errorf("cannot build the xbps command for zenith")
		}
		command := exec.Command(argv[0], argv[1:]...)
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("xbps-install: %w (run the update command again in a terminal to enter the password)", err)
		}
		return nil
	case "apt":
		return aptFetch()
	default:
		return fmt.Errorf("neither xbps nor apt is installed; place Zenith's packages by the means your base has")
	}
}

func isFlatpakRef(pkg string) bool {
	if strings.Contains(pkg, "/") {
		return true
	}
	parts := strings.Split(pkg, ".")
	return len(parts) >= 3 && parts[0] != "" && parts[1] != "" && parts[2] != ""
}
