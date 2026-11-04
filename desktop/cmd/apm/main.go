// apm forwards to `asura pkg`, which is where the verbs live.
//
// The package layer's name was asked for before the control tool's verbs
// were, and starting TWO tools with two argv grammars for one package layer
// would make the second question "which tool owns the repo" — a question no
// answer to improves. So apm is a keyboard surface: its argument list is the
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

// forward runs `asura pkg ...` against the found control tool and pipes it
// through: stdout and stderr for output, stdin for the password prompts, and
// the managers' exit statuses for the callers (scripts alike) that read them.
func forward(asuraPath string, argv []string) int {
	command := exec.Command(asuraPath, append([]string{"pkg"}, argv...)...)
	command.Stdout, command.Stderr, command.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := command.Run(); err != nil {
		var exited *exec.ExitError
		if errors.As(err, &exited) {
			// The control tool's own status IS the answer: apm passing back a
			// different one is apm telling its caller something else were
			// truth.
			return exited.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "apm: %v\n", err)
		return 1
	}
	return 0
}

func main() {
	asuraPath, err := exec.LookPath("asura")
	if err != nil {
		fmt.Fprintf(os.Stderr, "apm: the asura control tool is not installed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(forward(asuraPath, os.Args[1:]))
}
