package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The session names the shell in the places that have to agree, and none of
// them can check the others: the compositor's configuration is a text file,
// the shell's IPC target is a string in QML, and the control tool's command
// table is Go. One is not QML-adjacent to another, so a rename in one of them
// is invisible to the rest, and the symptom is the same every time: a session
// that starts a compositor, keybinds that call nothing, and a shell that
// never hears about a settings write.
//
// The keybinds no longer name a path, because a name for the instance points
// at one tree and the machine can be running another: the installed shell,
// which the package owns, is not the shell a development checkout starts, and
// both are legitimate. The calls say who to call — the target "zenith", the
// shell's own IPC handler — and leave which instance is running to `zenith
// ipc`, which finds it in the process table.

// repoRoot is the checkout, from the package directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	return root
}

// compositorConfig is the compositor's configuration: the file that says what
// the session starts, then every other .conf beside it, which is where the
// keyboard is described.
func compositorConfig(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "session", "hyprland")
	body, err := os.ReadFile(filepath.Join(dir, "hyprland.conf"))
	if err != nil {
		t.Skipf("the compositor's configuration is not in the checkout: %v", err)
	}
	siblings, _ := filepath.Glob(filepath.Join(dir, "*.conf"))
	for _, sibling := range siblings {
		if filepath.Base(sibling) == "hyprland.conf" {
			continue
		}
		if more, err := os.ReadFile(sibling); err == nil {
			body = append(append(body, '\n'), more...)
		}
	}
	return string(body)
}

// packaging is the build script: the thing that decides what is installed and
// where.
func packaging(t *testing.T, root string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "release", "build-debs.sh"))
	if err != nil {
		t.Fatalf("reading the packaging script: %v", err)
	}
	return string(body)
}

// TestTheCompositorStartsTheActivationScript checks that the script the
// compositor runs is one the package installs and one it can run.
//
// A script that is not installed is a compositor that runs nothing. A script
// that is installed but not executable is the same thing, and quieter: it was
// 0644 in the package while being executable in the checkout, so the checkout
// agreed and the machine did not.
func TestTheCompositorStartsTheActivationScript(t *testing.T) {
	root := repoRoot(t)
	config := compositorConfig(t, root)

	match := regexp.MustCompile(`(?m)^exec-once = (\S+)$`).FindStringSubmatch(config)
	if match == nil {
		t.Fatal("the compositor's configuration starts nothing; the session " +
			"has no anchor and the shell never comes up")
	}
	script := match[1]

	if !strings.HasPrefix(script, "/usr/share/zenith/") {
		t.Errorf("the activation script is %q, which is not in the package's "+
			"own tree; the package can only install what it owns", script)
	}

	// It exists in the checkout, and it is executable there.
	local := filepath.Join(root, "session", filepath.Base(script))
	info, err := os.Stat(local)
	if err != nil {
		t.Errorf("the activation script %s is not in the checkout: %v", script, err)
	} else if info.Mode().Perm()&0111 == 0 {
		t.Errorf("the activation script %s is not executable in the checkout", script)
	}

	// And the packaging installs it, by name.
	if !strings.Contains(packaging(t, root), filepath.Base(script)) {
		t.Errorf("the packaging does not mention %s, so it is not installed; "+
			"the compositor runs a file that is not there", filepath.Base(script))
	}
}

// TestTheKeybindsReachTheShell checks that the keybinds, the compositor's
// configuration and the shell's IPC surface agree.
//
// Every IPC bind dispatches through `zenith ipc call`, and every one of those
// calls names the target the shell's entry point registers. A call whose
// target is anything else is a bind that runs a command that succeeds and
// opens nothing.
func TestTheKeybindsReachTheShell(t *testing.T) {
	root := repoRoot(t)
	config := compositorConfig(t, root)

	const pattern = `zenith ipc call (\S+) ?(\S*)`
	matches := regexp.MustCompile(pattern).FindAllStringSubmatch(config, -1)
	if len(matches) == 0 {
		t.Fatal("no keybind dispatches through `zenith ipc call`; either the " +
			"keybinds run the shell's surfaces themselves, which is a second " +
			"shell, or they call quickshell directly by a path that only names " +
			"one of the trees it can run from")
	}
	for _, match := range matches {
		if match[1] != "zenith" {
			t.Errorf("a keybind calls the IPC target %q, and the shell's "+
				"handler is \"zenith\"", match[1])
		}
	}

	// The command exists: a bind that dispatches through a subcommand the
	// control tool does not have fails before it reaches any IPC surface.
	if _, ok := commands["ipc"]; !ok {
		t.Error("the keybinds dispatch through `zenith ipc`, and the control " +
			"tool has no ipc command")
	}

	// And the shell registers the target the binds name, in the checkout's
	// own entry point: the one file that is both sides of the seam.
	entry, err := os.ReadFile(filepath.Join(root, "shell", "shell.qml"))
	if err != nil {
		t.Skipf("the shell's entry point is not in the checkout: %v", err)
	}
	if !strings.Contains(string(entry), `target: "zenith"`) {
		t.Error("the keybinds call the target \"zenith\", and the shell's " +
			"entry point registers no such handler")
	}
}
