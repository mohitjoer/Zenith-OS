package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

const version = "1.0.0"

func printUsage() {
	fmt.Fprintf(os.Stderr, `asura-sysmon %s — Asura OS Resource Monitor

Usage:
  asura-sysmon [command] [options]

Commands:
  stats                Output full system vitals (CPU, RAM, Swap, Disks, Net) as JSON
  proc                 Output list of active processes as JSON
  kill <pid> [--force] Terminate a process safely
  version              Show version and exit
  ui, open, (default)  Launch the Quickshell Resource Monitor window

Options for 'proc':
  --sort <field>       Sort by: mem (default), pid, name
  --limit <n>          Cap results to n entries (default 100, 0 for all)

Options for 'stats':
  --interval <ms>      Sample interval for CPU & Network deltas in ms (default 200)
`, version)
}

func main() {
	if len(os.Args) < 2 {
		launchUI()
		return
	}

	cmd := os.Args[1]
	switch cmd {
	case "help", "-h", "--help":
		printUsage()
		return

	case "version", "-v", "--version":
		fmt.Printf("asura-sysmon %s (Asura OS)\n", version)
		return

	case "stats":
		statsCmd := flag.NewFlagSet("stats", flag.ExitOnError)
		intervalMs := statsCmd.Int("interval", 200, "Sample interval in ms")
		statsCmd.Parse(os.Args[2:])

		st, err := CollectAllStats(time.Duration(*intervalMs) * time.Millisecond)
		if err != nil {
			fmt.Fprintf(os.Stderr, "asura-sysmon: stats error: %v\n", err)
			os.Exit(1)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(st); err != nil {
			os.Exit(1)
		}

	case "proc":
		procCmd := flag.NewFlagSet("proc", flag.ExitOnError)
		sortBy := procCmd.String("sort", "mem", "Sort by: mem, pid, name")
		limit := procCmd.Int("limit", 100, "Limit number of processes")
		procCmd.Parse(os.Args[2:])

		procs, err := ListProcesses(*sortBy, *limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "asura-sysmon: proc error: %v\n", err)
			os.Exit(1)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			Processes []ProcessInfo `json:"processes"`
		}{procs}); err != nil {
			os.Exit(1)
		}

	case "kill":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "asura-sysmon kill: requires a PID\n")
			os.Exit(2)
		}
		pid, err := strconv.Atoi(os.Args[2])
		if err != nil || pid <= 0 {
			fmt.Fprintf(os.Stderr, "asura-sysmon kill: invalid PID %q\n", os.Args[2])
			os.Exit(2)
		}
		force := false
		for _, arg := range os.Args[3:] {
			if arg == "--force" || arg == "-f" || arg == "-9" {
				force = true
			}
		}
		if err := KillProcess(pid, force); err != nil {
			fmt.Fprintf(os.Stderr, "asura-sysmon kill: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("{\"status\":\"ok\",\"killed\":%d}\n", pid)

	case "open", "ui", "window":
		launchUI()

	default:
		// If first arg starts with dash or unknown, try launching UI or print error
		if os.Args[1] == "--open" {
			launchUI()
			return
		}
		fmt.Fprintf(os.Stderr, "asura-sysmon: unknown command %q\n", cmd)
		printUsage()
		os.Exit(2)
	}
}

// findQmlEntry locates sysmon.qml in development or installed locations
func findQmlEntry() string {
	candidates := []string{
		// Relative to binary in dev tree
		filepath.Join(".", "ui", "sysmon.qml"),
		filepath.Join("..", "ui", "sysmon.qml"),
		// Standard Asura OS installed locations
		"/usr/share/asura/sysmon/sysmon.qml",
		"/usr/share/asura/shell/sysmon.qml",
		filepath.Join(os.Getenv("HOME"), ".local/share/asura/sysmon/sysmon.qml"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			abs, err := filepath.Abs(c)
			if err == nil {
				return abs
			}
			return c
		}
	}
	return "/usr/share/asura/sysmon/sysmon.qml"
}

func launchUI() {
	entry := findQmlEntry()

	// Use flock on $XDG_RUNTIME_DIR/asura-sysmon.lock for single-instance
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = os.TempDir()
	}
	lockFile := filepath.Join(runtimeDir, "asura-sysmon.lock")

	cmd := exec.Command("flock", "-n", "-o", lockFile, "quickshell", "-p", entry)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		// If flock failed because already running, that's clean single-instance behaviour
		os.Exit(0)
	}
}
