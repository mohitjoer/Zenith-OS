package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"zenith/desktop/internal/runner"
)

// `zenith reload` is called by the Hub after it changes the login shell, and
// named by the plugin installer as the way to see an updated plugin. Nothing
// else reloads the desktop: the shell is a systemd unit, so asking it to is
// systemd's job and not a signal this tool sends itself.

const (
	reloadShellUnit  = "zenith-shell.service"
	reloadDaemonUnit = "zenithd.service"
)

const reloadUsage = `Usage: zenith reload

Reloads the running shell (systemctl --user reload zenith-shell) and, when it
is running, the daemon. Use it after changing config that is already in place.
`

func init() {
	register("reload", "reload the running shell and daemon", zenithReload)
}

func zenithReload(args []string) int {
	return reloadMain(args, os.Stdout, os.Stderr, runner.Exec(30*time.Second))
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
	if _, err := run("systemctl", "--user", "reload", reloadShellUnit); err != nil {
		fmt.Fprintf(errOut, "shell:   not reloaded: %s\n", runner.Reason(err))
		code = exitFail
	} else {
		fmt.Fprintln(out, "shell:   reloaded")
	}

	// A daemon that is not running is nothing to reload, not a failure: it is
	// absent on a machine that has not installed it. is-active exits non-zero
	// for every state but active. try-reload-or-restart falls back to a restart
	// for a daemon whose unit has no reload action.
	if _, err := run("systemctl", "--user", "is-active", "--quiet", reloadDaemonUnit); err != nil {
		fmt.Fprintln(out, "daemon:  not running")
	} else if _, err := run("systemctl", "--user", "try-reload-or-restart", reloadDaemonUnit); err != nil {
		fmt.Fprintf(errOut, "daemon:  not reloaded: %s\n", runner.Reason(err))
		code = exitFail
	} else {
		fmt.Fprintln(out, "daemon:  reloaded")
	}
	return code
}
