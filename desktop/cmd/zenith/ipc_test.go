package main

// The discovery is what the keybinds lean on, so it is a pure function of the
// process table it is given and is tested the way the rest of the tool's
// claims are: against a table that was written down, not against the machine
// running the tests, whose contents are whoever logged in.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCmdline lays down one process in a fake /proc: a directory named for
// the pid, and inside it the NUL-separated command line the kernel reports.
func writeCmdline(t *testing.T, root, pid, argv string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, pid), 0o755); err != nil {
		t.Fatalf("making /proc/%s: %v", pid, err)
	}
	body := strings.ReplaceAll(argv, " ", "\x00") + "\x00"
	if err := os.WriteFile(filepath.Join(root, pid, "cmdline"),
		[]byte(body), 0o644); err != nil {
		t.Fatalf("writing /proc/%s/cmdline: %v", pid, err)
	}
}

// TestScanShellInstancesFindsTheShell checks that the instance whose command
// line says where it is running is the one the scanner reports, and the
// processes that are not shells — a process merely holding the word
// "quickshell" in an argument, and one without a command line at all — are
// passed over rather than reported.
func TestScanShellInstancesFindsTheShell(t *testing.T) {
	root := t.TempDir()

	// The live shell: quickshell, started from a project path.
	writeCmdline(t, root, "21420", "quickshell -p /home/mohit/code/Zenith-OS/shell --no-color")
	// A process whose argument mentions quickshell, which is not one.
	writeCmdline(t, root, "12104", "journalctl --unit quickshell-shell.service --no-pager")
	// A process the table has but whose command line it will not report;
	// reading its record succeeds and answers nothing.
	if err := os.MkdirAll(filepath.Join(root, "4983"), 0o755); err != nil {
		t.Fatalf("making /proc/4983: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "4983", "cmdline"), []byte(""), 0o644); err != nil {
		t.Fatalf("writing an empty command line: %v", err)
	}
	// Not a pid: the table names its own directories, and none of them
	// belongs to a process.
	writeCmdline(t, root, "self", "quickshell -p /over/run")

	got := scanShellInstances(root)
	if len(got) != 1 {
		t.Fatalf("the scan reports %d instance(s), %v, and the table says one",
			len(got), got)
	}
	if want := "/home/mohit/code/Zenith-OS/shell"; got[0] != want {
		t.Errorf("the scan reports the instance running from %q, and its "+
			"command line says %q", got[0], want)
	}
}

// TestQuickshellPathArgSpellsThePath checks both spellings of quickshell's
// own option, and that a flag with nothing after it is not a path.
func TestQuickshellPathArgSpellsThePath(t *testing.T) {
	cases := []struct {
		name   string
		fields []string
		path   string
		found  bool
	}{
		{"the short flag", []string{"quickshell", "-p", "/x/shell", "--no-color"}, "/x/shell", true},
		{"the long flag", []string{"quickshell", "--path", "/x/shell"}, "/x/shell", true},
		{"the long flag with a value", []string{"quickshell", "--path=/x/shell"}, "/x/shell", true},
		{"nothing after the flag", []string{"quickshell", "--path"}, "", false},
		{"no flag at all", []string{"quickshell", "ipc"}, "", false},
		{"an empty value", []string{"quickshell", "--path=", "/x/shell"}, "", false},
	}
	for _, test := range cases {
		path, found := quickshellPathArg(test.fields)
		if found != test.found {
			t.Errorf("%s: found %v; want %v", test.name, found, test.found)
		} else if found && path != test.path {
			t.Errorf("%s: reports %q; want %q", test.name, path, test.path)
		}
	}
}

// TestPickShellInstance prefers the instance that is unambiguous and refuses
// to guess when two zenith trees are both running.
func TestPickShellInstance(t *testing.T) {
	installed := "/usr/share/zenith/shell"
	scoped := []struct {
		name    string
		paths   []string
		want    string
		wantErr string
	}{
		{"nothing is running", nil, "", "no instance"},
		{
			"one shell",
			[]string{installed},
			installed,
			"",
		},
		{
			"one shell, and something else",
			[]string{installed, "/home/somewhere/other/shell"},
			installed,
			"",
		},
		{
			"the foreign shell alone among others",
			[]string{"/home/somewhere/other/shell", "/tmp/another/quickshell"},
			"",
			"refusing to guess",
		},
		{
			"two zenith trees",
			[]string{installed, "/home/mohit/code/Zenith-OS/shell"},
			"",
			"refusing to guess",
		},
	}

	for _, test := range scoped {
		got, err := pickShellInstance(test.paths)
		switch {
		case err != nil:
			if test.wantErr == "" {
				t.Errorf("%s: an error it did not expect: %v", test.name, err)
			} else if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("%s: the error %q does not mention %q",
					test.name, err.Error(), test.wantErr)
			}
		case got != test.want:
			t.Errorf("%s: picks %q; want %q", test.name, got, test.want)
		}
	}
}

// TestZenithIpcRefusesABareCall checks that the command's own answer to a
// call it cannot make is a usage exit, not a dispatch that goes nowhere.
func TestZenithIpcRefusesABareCall(t *testing.T) {
	if code := zenithIpc(nil); code != exitUsage {
		t.Errorf("zenith ipc with no subcommand exits %d; want %d", code, exitUsage)
	}
	if code := zenithIpc([]string{"inform"}); code != exitUsage {
		t.Errorf("zenith ipc with a subcommand it does not have exits %d; want %d", code, exitUsage)
	}
	if code := zenithIpc([]string{"call", "zenith"}); code != exitUsage {
		t.Errorf("zenith ipc call without a function exits %d; want %d", code, exitUsage)
	}
}
