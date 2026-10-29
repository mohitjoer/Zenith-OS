package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const installUsage = `Usage: zenith install [target-disk]

Launches the Zenith OS system installer to partition and install
Zenith OS onto a specified drive (e.g. /dev/nvme0n1 or /dev/vda).
`

func init() {
	register("install", "launch guided Zenith OS system installer", zenithInstall)
}

func zenithInstall(args []string) int {
	installerPath := "/usr/bin/zenith-installer"
	if _, err := os.Stat(installerPath); err != nil {
		// Fallback to repository location if testing in dev
		installerPath = "installation/zenith-installer.sh"
		if _, err := os.Stat(installerPath); err != nil {
			fmt.Fprintf(os.Stderr, "zenith install: installer binary not found\n")
			return exitFail
		}
	}

	cmdArgs := []string{installerPath}
	cmdArgs = append(cmdArgs, args...)

	var cmd *exec.Cmd
	if os.Geteuid() != 0 {
		sudoPath, err := exec.LookPath("sudo")
		if err != nil {
			fmt.Fprintf(os.Stderr, "zenith install: root privileges required but sudo not found\n")
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
