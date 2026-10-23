package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"zenith/desktop/internal/runner"
)

// Two checks for faults that say nothing when they happen. A keybind whose
// command was never installed runs, fails inside the compositor, and the key
// does nothing; a recommended package that is not installed turns one control
// into a button that fails. Both are found here instead of by a user.

// configExec matches the command a compositor config runs: an exec line, or a
// bind whose dispatcher is exec. The first word of it is captured, after any
// window rule in brackets.
var configExec = regexp.MustCompile(`(?m)^\s*(?:execr?(?:-once)?\s*=|bind[a-z]*\s*=[^,\n]*,[^,\n]*,\s*exec\s*,)\s*(?:\[[^\]\n]*\]\s*)?(\S+)`)

// zenithCommands names the Zenith commands a compositor config runs, by bare
// name. Anything else it runs (ghostty, wpctl) is the base layer's, and a
// command guarded by `command -v` is already degrading on purpose.
func zenithCommands(config string) []string {
	seen := map[string]bool{}
	for _, match := range configExec.FindAllStringSubmatch(config, -1) {
		word := match[1]
		if strings.HasPrefix(word, "/usr/bin/") {
			word = filepath.Base(word)
		}
		if word == "zenith" || strings.HasPrefix(word, "zenith-") {
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

// reportCommands checks that every Zenith command the compositor runs is
// installed where the compositor's PATH finds it. The session's PATH has no
// ~/.local/bin, so /usr/bin is the only place that counts.
func reportCommands(root string) int {
	fmt.Println()
	fmt.Println("Commands")
	config, err := installedCompositorConfig(root, "/usr/share/zenith/hyprland/hyprland.conf")
	if err != nil {
		fmt.Println("  the compositor's configuration is not installed; nothing to check")
		return exitOK
	}

	names := zenithCommands(config)
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
	fmt.Printf("  all %d zenith command(s) the compositor runs are installed\n", len(names))
	return exitOK
}

// recommendedAbsent returns the recommended packages of the zenith packages
// that are not installed. A recommendation written `a | b` is satisfied by
// either.
func recommendedAbsent(run runner.Func) ([]string, int, error) {
	// zenith-desktop may not be installed, which makes dpkg-query exit non-zero
	// while still printing for the one that is; the output is what counts.
	recommends, err := run("dpkg-query", "-W", "-f=${Recommends}\n", "zenith", "zenith-desktop")
	if strings.TrimSpace(recommends) == "" && err != nil {
		return nil, 0, err
	}
	status, err := run("dpkg-query", "-W", "-f=${Package} ${db:Status-Abbrev}\n")
	if err != nil {
		return nil, 0, err
	}

	installed := map[string]bool{}
	for _, line := range strings.Split(status, "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && fields[1] == "ii" {
			installed[fields[0]] = true
		}
	}

	var absent []string
	total := 0
	for _, entry := range strings.FieldsFunc(recommends, func(r rune) bool { return r == ',' || r == '\n' }) {
		satisfied, first := false, ""
		for _, alternative := range strings.Split(entry, "|") {
			name := strings.Fields(alternative)
			if len(name) == 0 {
				continue
			}
			// "pkg:any" and "pkg (>= 1)" are both the package pkg.
			pkg := strings.SplitN(name[0], ":", 2)[0]
			if first == "" {
				first = pkg
			}
			satisfied = satisfied || installed[pkg]
		}
		if first == "" {
			continue
		}
		total++
		if !satisfied {
			absent = append(absent, first)
		}
	}
	return absent, total, nil
}

// reportDeps lists what the zenith packages recommend and this machine lacks.
// A recommendation is optional by definition, so it never fails the doctor; it
// says what to install.
func reportDeps(root string, run runner.Func) int {
	fmt.Println()
	fmt.Println("Dependencies")
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
		fmt.Printf("  sudo apt install %s\n", strings.Join(absent, " "))
	}
	return exitOK
}
