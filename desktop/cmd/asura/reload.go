package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"asura/desktop/internal/runner"
)

// `asura reload` is called by the Hub after it changes the login shell, and
// named by the plugin installer as the way to see an updated plugin.
// It signals the user runit services under ~/.config/service.

const (
	reloadShellService  = "quickshell"
	reloadDaemonService = "asurad"
)

const reloadUsage = `Usage: asura reload

Reloads the running shell and daemon via user runit supervision.
Use it after changing config that is already in place.
`

func init() {
	register("reload", "reload the running shell and daemon", asuraReload)
}

func asuraReload(args []string) int {
	return reloadMain(args, os.Stdout, os.Stderr, runner.Exec(30*time.Second))
}

func userServiceDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "service")
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	return filepath.Join(home, ".config", "service")
}

// reloadMain takes the program runner so a test never reaches the live session.
func reloadMain(args []string, out, errOut io.Writer, run runner.Func) int {
	if len(args) != 0 {
		if args[0] == "-h" || args[0] == "--help" {
			fmt.Fprint(out, reloadUsage)
			return exitOK
		}
		fmt.Fprint(errOut, reloadUsage)
		return exitUsage
	}

	code := exitOK
	svcDir := userServiceDir()
	shellPath := filepath.Join(svcDir, reloadShellService)
	daemonPath := filepath.Join(svcDir, reloadDaemonService)

	if _, err := run("sv", "restart", shellPath); err != nil {
		fmt.Fprintf(errOut, "shell:   not reloaded: %s\n", runner.Reason(err))
		code = exitFail
	} else {
		fmt.Fprintln(out, "shell:   reloaded")
	}

	// A daemon that is not running is nothing to reload, not a failure: it is
	// absent on a machine that has not installed it.
	if _, err := run("sv", "status", daemonPath); err != nil {
		fmt.Fprintln(out, "daemon:  not running")
	} else if _, err := run("sv", "restart", daemonPath); err != nil {
		fmt.Fprintf(errOut, "daemon:  not reloaded: %s\n", runner.Reason(err))
		code = exitFail
	} else {
		fmt.Fprintln(out, "daemon:  reloaded")
	}
	return code
}
