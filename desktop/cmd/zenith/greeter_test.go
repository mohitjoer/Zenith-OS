package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// greeterMachine lays out a root with everything the login screen needs and
// returns it, with the command's output captured.
func greeterMachine(t *testing.T) (root string, out, errOut *bytes.Buffer) {
	t.Helper()
	root = t.TempDir()
	write := func(path string, mode os.FileMode) {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("#!/bin/sh\n"), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(greeterBinary, 0o755)
	write("/usr/bin/weston", 0o755)
	write(greeterTheme+"/Main.qml", 0o644)
	write(greeterWrapper, 0o755)
	write(greeterDropIn, 0o644)

	out, errOut = &bytes.Buffer{}, &bytes.Buffer{}
	oldRoot, oldOut, oldErr := greeterRoot, greeterOut, greeterErr
	greeterRoot, greeterOut, greeterErr = root, out, errOut
	t.Cleanup(func() { greeterRoot, greeterOut, greeterErr = oldRoot, oldOut, oldErr })
	return root, out, errOut
}

// TestGreeterEnableChecksThenOnlyPrints pins the safety property of the
// command: it refuses while anything the login screen needs is missing, and
// when it is satisfied it still writes nothing, only prints the command.
func TestGreeterEnableChecksThenOnlyPrints(t *testing.T) {
	root, out, errOut := greeterMachine(t)

	if err := os.Remove(filepath.Join(root, "/usr/bin/weston")); err != nil {
		t.Fatal(err)
	}
	if code := zenithGreeter([]string{"enable"}); code != exitFail {
		t.Fatalf("enable without weston exited %d, want %d", code, exitFail)
	}
	if !strings.Contains(errOut.String(), "sudo apt install weston") || out.Len() != 0 {
		t.Errorf("a missing weston must be named with its fix and no command printed: %q %q", errOut, out)
	}

	if err := os.WriteFile(filepath.Join(root, "/usr/bin/weston"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := zenithGreeter([]string{"enable"}); code != exitOK {
		t.Fatalf("enable on a ready machine exited %d, want %d", code, exitOK)
	}
	want := "sudo install -D -m 0644 " + greeterDropIn + " " + greeterInstalled
	if !strings.Contains(out.String(), want) {
		t.Errorf("the printed command is missing %q:\n%s", want, out)
	}
	if _, err := os.Stat(filepath.Join(root, greeterInstalled)); err == nil {
		t.Error("enable wrote the drop-in itself; only the user may do that")
	}
}

func TestGreeterDisablePrintsRemovalOnlyWhenEnabled(t *testing.T) {
	root, out, _ := greeterMachine(t)

	zenithGreeter([]string{"disable"})
	if strings.Contains(out.String(), "sudo rm") {
		t.Errorf("nothing is enabled, yet disable printed a removal:\n%s", out)
	}

	installed := filepath.Join(root, greeterInstalled)
	if err := os.MkdirAll(filepath.Dir(installed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	zenithGreeter([]string{"disable"})
	if !strings.Contains(out.String(), "sudo rm "+greeterInstalled) {
		t.Errorf("disable did not print the removal:\n%s", out)
	}
}

// TestGreeterTestRunsTheThemeInTestMode checks the arguments, because a greeter
// started without --test-mode would try to reach a daemon that is not there.
func TestGreeterTestRunsTheThemeInTestMode(t *testing.T) {
	root, _, _ := greeterMachine(t)
	t.Setenv("WAYLAND_DISPLAY", "wayland-test")
	args := filepath.Join(root, "args")
	script := "#!/bin/sh\necho \"$@\" > " + args + "\n"
	if err := os.WriteFile(filepath.Join(root, greeterBinary), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	old := greeterTestTime
	greeterTestTime = 10 * time.Second
	t.Cleanup(func() { greeterTestTime = old })

	if code := zenithGreeter([]string{"test"}); code != exitOK {
		t.Fatalf("test exited %d, want %d", code, exitOK)
	}
	got, err := os.ReadFile(args)
	if err != nil {
		t.Fatal(err)
	}
	if want := "--test-mode --theme " + root + greeterTheme; strings.TrimSpace(string(got)) != want {
		t.Errorf("greeter ran with %q, want %q", got, want)
	}
}
