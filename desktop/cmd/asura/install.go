package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const installUsage = `Usage: asura install [target-disk]

Launches the Asura OS system installer to partition and install
Asura OS onto a specified drive (e.g. /dev/nvme0n1 or /dev/vda).
`

func init() {
	register("install", "launch guided Asura OS system installer", asuraInstall)
}

func asuraInstall(args []string) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(os.Stdout, installUsage)
		return exitOK
	}

	installerPath := "/usr/bin/asura-installer"
	if _, err := os.Stat(installerPath); err != nil {
		// Fallback to repository location if testing in dev
		installerPath = "installation/asura-installer.sh"
		if _, err := os.Stat(installerPath); err != nil {
			fmt.Fprintf(os.Stderr, "asura install: installer binary not found\n")
			return exitFail
		}
	}

	cmdArgs := []string{installerPath}
	cmdArgs = append(cmdArgs, args...)

	var cmd *exec.Cmd
	if os.Geteuid() != 0 {
		sudoPath, err := exec.LookPath("sudo")
		if err != nil {
			fmt.Fprintf(os.Stderr, "asura install: root privileges required but sudo not found\n")
			return exitFail
		}
		cmd = exec.Command(sudoPath, cmdArgs...)
	} else {
		cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus()
			}
			return exitFail
		}
		return exitFail
	}
	return exitOK
}
