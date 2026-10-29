// zpm forwards to `zenith pkg`, which is where the verbs live.
//
// The package layer's name was asked for before the control tool's verbs
// were, and starting TWO tools with two argv grammars for one package layer
// would make the second question "which tool owns the repo" — a question no
// answer to improves. So zpm is a keyboard surface: its argument list is the
// control tool's, its exit status is the manager's, and every specialised
// grammar (the repo flag, the channel, the update flow) gets a single place
// in the control tool's own explanation.
//
// Compile: desktop/go.mod builds it beside the others.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// forward runs `zenith pkg ...` against the found control tool and pipes it
// through: stdout and stderr for output, stdin for the password prompts, and
// the managers' exit statuses for the callers (scripts alike) that read them.
func forward(zenithPath string, argv []string) int {
	command := exec.Command(zenithPath, append([]string{"pkg"}, argv...)...)
	command.Stdout, command.Stderr, command.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := command.Run(); err != nil {
		var exited *exec.ExitError
		if errors.As(err, &exited) {
			// The control tool's own status IS the answer: zpm passing back a
			// different one is zpm telling its caller something else were
			// truth.
			return exited.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "zpm: %v\n", err)
		return 1
	}
	return 0
}

func main() {
	zenithPath, err := exec.LookPath("zenith")
	if err != nil {
		fmt.Fprintf(os.Stderr, "zpm: the zenith control tool is not installed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(forward(zenithPath, os.Args[1:]))
}
