// Command "zenith ipc" calls the running shell's IPC surface.
//
// The compositor's keybinds round the loop through here, and the reason is
// that only the session knows which instance of the shell is running: the one
// the package installed, or one a development tree started. A call that dials
// a path dials it whether or not anything is listening there, and the failure
// is the quietest one on the desktop — the key is pressed, the command
// succeeds, and nothing opens.
//
// So this command does not take a path. It finds the running quickshell by
// reading the processes, and forwards the call to it.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ipcUsage is the command's own usage text.
const ipcUsage = `Usage: zenith ipc call TARGET [FUNCTION [ARGS...]]

Calls the running shell's IPC surface. The shell is found, not named: the
instance that is running is the one the call goes to, whatever tree it was
started from.

  TARGET       the shell's IPC handler, e.g. zenith
  FUNCTION     the function to call, e.g. togglePopover
  ARGS...      the function's arguments, in order
`

// zenithIpc forwards one IPC call to the running shell.
//
// The call is spelled exactly as quickshell spells it — target, then
// function, then the function's arguments — and is handed to quickshell's
// own CLI after this one has found the instance. A call with no function is
// refused here rather than by quickshell, because the compositor's
// configuration is read by the bind checks and a call whose shape is wrong
// should be called out the same way its target and function are.
func zenithIpc(args []string) int {
	if len(args) == 0 || args[0] != "call" {
		fmt.Fprint(os.Stderr, ipcUsage)
		return exitUsage
	}
	rest := args[1:]
	if len(rest) < 2 {
		fmt.Fprintln(os.Stderr,
			"zenith ipc call: needs TARGET and FUNCTION")
		return exitUsage
	}

	path, err := findShellConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith ipc:", err)
		return exitFail
	}

	cmdline := []string{"quickshell", "ipc", "--path", path,
		"call", rest[0], rest[1]}
	cmdline = append(cmdline, rest[2:]...)
	cmd := exec.Command(cmdline[0], cmdline[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "zenith ipc:",
			"the call did not reach the shell:", err)
		return exitFail
	}
	return exitOK
}

// findShellConfig returns the configuration the running shell was started
// from, which is what quickshell's own CLI needs in order to reach it: it is
// the value of the instance's -p, read out of the process table rather than
// remembered here.
func findShellConfig() (string, error) {
	return pickShellInstance(scanShellInstances("/proc"))
}

// scanShellInstances walks a process table and returns the configuration
// paths of the quickshell processes it finds.
//
// Read errors are not findings. A process that exits between the listing and
// the read is a process that no longer needs calling, and a directory the
// kernel invents without a pid in it does not belong to a number the walk
// asked for.
func scanShellInstances(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid := entry.Name()
		for _, char := range pid {
			if char < '0' || char > '9' {
				pid = ""
				break
			}
		}
		if pid == "" {
			continue
		}

		// The command line is a NUL-separated list ending in a NUL, and
		// quickshell's own name is the field before its arguments: a command
		// line that merely mentions quickshell in a flag is somebody else's
		// process.
		data, err := readCmdline(root, pid)
		if err != nil {
			continue
		}
		fields := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
		if len(fields) == 0 || !strings.HasSuffix(fields[0], "quickshell") {
			continue
		}
		if path, ok := quickshellPathArg(fields); ok {
			paths = append(paths, path)
		}
	}
	return paths
}

// quickshellPathArg reads -p or --path out of one command line, in both of
// quickshell's spellings of it.
func quickshellPathArg(fields []string) (string, bool) {
	for i, field := range fields {
		switch {
		case field == "-p" || field == "--path":
			if i+1 < len(fields) && fields[i+1] != "" {
				return fields[i+1], true
			}
		case strings.HasPrefix(field, "--path="):
			if value := strings.TrimPrefix(field, "--path="); value != "" {
				return value, true
			}
		}
	}
	return "", false
}

// pickShellInstance chooses which instance a call is for.
//
// A desktop with one shell is the case every other branch is for. When there
// is more than one the one whose tree is named after the desktop itself is
// the one the key was pressed on, and what is left is an ambiguity to report
// rather than a coin to toss.
//
// Only an instance started on a directory is the shell. The unit runs
// `quickshell -p /usr/share/zenith/shell`; every other window the desktop
// opens (the Hub, the screenshot tool, the wallpaper picker ...) is a
// quickshell of its own started on one entry file, `-p .../hub.qml`, and has
// no `zenith` IPC target. Counting those made one open window look like a
// second shell, so every keybind refused to fire until it was closed.
func pickShellInstance(paths []string) (string, error) {
	shells := make([]string, 0, len(paths))
	for _, path := range paths {
		if !strings.HasSuffix(strings.ToLower(path), ".qml") {
			shells = append(shells, path)
		}
	}
	paths = shells

	switch len(paths) {
	case 0:
		return "", fmt.Errorf("no instance of the shell is running; the call " +
			"would reach nothing, so it is not made")
	case 1:
		return paths[0], nil
	}

	var preferred []string
	for _, path := range paths {
		if strings.Contains(strings.ToLower(path), "/zenith") {
			preferred = append(preferred, path)
		}
	}
	switch len(preferred) {
	case 1:
		return preferred[0], nil
	case 0:
		return "", fmt.Errorf("several shells are running, and none of them is "+
			"under a Zenith tree; refusing to guess: %s",
			strings.Join(paths, ", "))
	default:
		return "", fmt.Errorf("several Zenith trees are running, and refusing "+
			"to guess: %s", strings.Join(preferred, ", "))
	}
}

// The three segments are joined with filepath.Join rather than by hand, so
// that the separator is the platform's and not anybody's habit.
func readCmdline(root, pid string) ([]byte, error) {
	return os.ReadFile(filepath.Join(root, pid, "cmdline"))
}
