package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The session names the shell in three places, and they have to name the same
// shell.
//
// The compositor's configuration says where the shell lives, because that is
// what the keybinds call: the IPC call has to know which instance it is
// talking to, and it finds it by path. The packaging says where the shell is
// installed. The control tool says where the shell is, because it tells the
// shell to read its settings again after a write.
//
// None of the three can check the others, because none of them can see the
// others' files: one is a QML-adjacent configuration file, one is a shell
// script, and one is a Go constant. So a rename in one is invisible to the
// other two, and the symptom is the same every time: a session that starts a
// compositor, a shell that never starts, and keybinds that call nothing.
//
// Each of these has been wrong on this project, one at a time.

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
// the keyboard does and what the session starts.
func compositorConfig(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, "session", "hyprland", "hyprland.conf")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("the compositor's configuration is not in the checkout: %v", err)
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

// TestTheKeybindsCallTheInstalledShell checks that the keybinds, the
// packaging and the control tool all name the same directory.
//
// The keybinds call the shell by path, and the path the control tool uses to
// tell the shell to re-read its settings has to be the same one: a shell
// reached at one path and updated at another is a shell whose settings
// change when nothing else does.
func TestTheKeybindsCallTheInstalledShell(t *testing.T) {
	root := repoRoot(t)
	config := compositorConfig(t, root)

	match := regexp.MustCompile(`quickshell ipc --path (\S+) call`).FindStringSubmatch(config)
	if match == nil {
		t.Fatal("no keybind names the shell's path; an IPC call without one " +
			"looks for a default configuration, finds none, and does nothing")
	}
	path := match[1]

	if path != shellConfigPath {
		t.Errorf("the keybinds call %q, and the control tool tells %q to "+
			"re-read its settings; they have to be the same shell",
			path, shellConfigPath)
	}

	// And that is where the packaging puts it.
	if !strings.Contains(packaging(t, root), path) {
		t.Errorf("the packaging does not install the shell at %q, which is "+
			"where the keybinds look for it", path)
	}
}
