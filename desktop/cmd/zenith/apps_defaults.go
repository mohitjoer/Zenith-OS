package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// zenith apps defaults makes Ghostty the default terminal everywhere the desktop
// asks, for this user.
//
// docs/architecture.md names four places. Two ship as content: the shell's
// launcher (zenith-app terminal) and the zsh integration (apps/zsh/zenith.zsh).
// This command covers the rest: the default-applications setting, the
// x-terminal-emulator alternative (root, so it is printed and never run), and a
// fifth the architecture predates. The shell starts terminal applications through
// xdg-terminal-exec, which takes the first terminal it finds unless a list names
// one; with Ptyxis installed that is Ptyxis.
//
// The two lines the user's own dotfiles need are printed, never written: a
// user's ~/.zshrc and ghostty config are not ours to edit.

const appsDefaultsUsage = `Usage: zenith apps defaults [--check]

Make Ghostty the default terminal for this user. What this user owns is applied;
the root-only registration and the lines your own dotfiles need are printed and
never run or written.

  --check   say what would change and change nothing
`

const (
	defaultsDesktopID  = "com.mitchellh.ghostty.desktop"
	defaultsTermSchema = "org.gnome.desktop.default-applications.terminal"
)

// Variables so the test can stand in for the machine.
var (
	defaultsGhostty = "/usr/bin/ghostty"
	defaultsExec    = func(name string, args ...string) (string, error) {
		out, err := exec.Command(name, args...).CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
)

// "apps" is a built-in, so register() would refuse it and "defaults" cannot be a
// command of its own; the built-in is wrapped to hand "defaults" over.
func init() {
	list := commands["apps"]
	if list == nil {
		return
	}
	commands["apps"] = func(args []string) int {
		if len(args) > 0 && args[0] == "defaults" {
			return appsDefaults(args[1:])
		}
		return list(args)
	}
}

func appsDefaults(args []string) int {
	set := newFlags("apps defaults", appsDefaultsUsage)
	check := set.Bool("check", false, "say what would change and change nothing")
	if err := set.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if set.NArg() != 0 {
		fmt.Fprint(os.Stderr, appsDefaultsUsage)
		return exitUsage
	}
	return runAppsDefaults(os.Stdout, *check)
}

func runAppsDefaults(w io.Writer, check bool) int {
	say := func(status, format string, a ...any) {
		fmt.Fprintf(w, "%-6s %s\n", status, fmt.Sprintf(format, a...))
	}
	if _, err := os.Stat(defaultsGhostty); err != nil {
		say("fail", "not available: install ghostty (apt install ghostty)")
		return exitFail
	}
	failed := false

	for _, kv := range [][2]string{{"exec", "ghostty"}, {"exec-arg", "-e"}} {
		key, want := kv[0], kv[1]
		got, err := defaultsExec("gsettings", "get", defaultsTermSchema, key)
		if err != nil {
			say("fail", "gsettings %s: %s", key, defaultsWhy(got, err))
			failed = true
			continue
		}
		switch {
		case strings.Trim(got, "'") == want:
			say("ok", "gsettings %s is %s", key, want)
		case check:
			say("would", "gsettings %s: %s -> %s", key, got, want)
		default:
			// Quoted, so a value that starts with a dash is never an option.
			if out, err := defaultsExec("gsettings", "set", defaultsTermSchema, key, "'"+want+"'"); err != nil {
				say("fail", "gsettings set %s: %s", key, defaultsWhy(out, err))
				failed = true
				continue
			}
			say("set", "gsettings %s: %s -> %s", key, got, want)
		}
	}

	list := filepath.Join(defaultsConfigHome(), "zenith-xdg-terminals.list")
	body, err := os.ReadFile(list)
	switch first := defaultsFirstEntry(string(body)); {
	case err == nil && first == defaultsDesktopID:
		say("ok", "%s names Ghostty", list)
	case err == nil:
		say("kept", "%s names %q first; it is yours, left alone", list, first)
	case check:
		say("would", "write %s", list)
	default:
		err := os.MkdirAll(filepath.Dir(list), 0o755)
		if err == nil {
			err = os.WriteFile(list, []byte(defaultsDesktopID+"\n"), 0o644)
		}
		if err != nil {
			say("fail", "%s: %v", list, err)
			failed = true
		} else {
			say("set", "%s names Ghostty, so xdg-terminal-exec opens it", list)
		}
	}

	query, err := defaultsExec("update-alternatives", "--query", "x-terminal-emulator")
	current, registered := defaultsAlternative(query)
	if err == nil && current == defaultsGhostty {
		say("ok", "x-terminal-emulator is Ghostty")
	} else {
		say("root", "x-terminal-emulator is not Ghostty; as root:")
		if !registered {
			fmt.Fprintf(w, "         sudo update-alternatives --install /usr/bin/x-terminal-emulator x-terminal-emulator %s 30\n", defaultsGhostty)
		}
		fmt.Fprintf(w, "         sudo update-alternatives --set x-terminal-emulator %s\n", defaultsGhostty)
	}

	cfg := defaultsConfigHome()
	zshrc := filepath.Join(defaultsHome(), ".zshrc")
	if dir := os.Getenv("ZDOTDIR"); dir != "" {
		zshrc = filepath.Join(dir, ".zshrc")
	}
	defaultsHook(say, w, []string{filepath.Join(cfg, "ghostty", "config"), filepath.Join(cfg, "ghostty", "config.ghostty")},
		"zenith/apps/ghostty/config", "config-file = ?~/.config/zenith/apps/ghostty/config")
	defaultsHook(say, w, []string{zshrc}, "zenith/apps/zsh/zenith.zsh",
		`[ -r "$HOME/.config/zenith/apps/zsh/zenith.zsh" ] && . "$HOME/.config/zenith/apps/zsh/zenith.zsh"`)

	if failed {
		return exitFail
	}
	return exitOK
}

// defaultsHook says whether a dotfile reads Zenith's base, and prints the line
// when it does not. The first candidate that exists is the one named.
func defaultsHook(say func(string, string, ...any), w io.Writer, files []string, marker, line string) {
	target := files[0]
	for i := len(files) - 1; i >= 0; i-- {
		body, err := os.ReadFile(files[i])
		if err != nil {
			continue
		}
		if strings.Contains(string(body), marker) {
			say("ok", "%s reads Zenith's base", files[i])
			return
		}
		target = files[i]
	}
	say("you", "add this line to %s:", target)
	fmt.Fprintf(w, "         %s\n", line)
}

func defaultsFirstEntry(list string) string {
	for _, line := range strings.Split(list, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}

// defaultsAlternative reads `update-alternatives --query`: the current choice,
// and whether Ghostty is registered at all.
func defaultsAlternative(query string) (current string, registered bool) {
	for _, line := range strings.Split(query, "\n") {
		switch {
		case strings.HasPrefix(line, "Value: "):
			current = strings.TrimPrefix(line, "Value: ")
		case line == "Alternative: "+defaultsGhostty:
			registered = true
		}
	}
	return current, registered
}

func defaultsWhy(out string, err error) string {
	if out != "" {
		return out
	}
	return err.Error()
}

func defaultsHome() string {
	home, _ := os.UserHomeDir()
	return home
}

func defaultsConfigHome() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(defaultsHome(), ".config")
}
