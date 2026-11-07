package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	candidates := []string{
		"/usr/bin/asura-installer",
		"/usr/share/asura/installation/asura-installer.sh",
		"installation/asura-installer.sh",
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "asura-installer"),
			filepath.Join(exeDir, "..", "installation", "asura-installer.sh"),
			filepath.Join(exeDir, "..", "..", "installation", "asura-installer.sh"),
		)
	}

	var installerPath string
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			installerPath = p
			break
		}
	}

	if installerPath == "" {
		fmt.Fprintf(os.Stderr, "asura install: installer binary not found\n")
		return exitFail
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
