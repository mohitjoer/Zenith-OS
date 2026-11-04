// Package runner is how the control tool starts other programs.
//
// The commands that manage the running desktop — reload, restart, the
// compositor wrapper, the doctor's probes — all end in someone else's
// program: systemctl, hyprctl, dpkg-query. Those programs act on the live
// session of whoever runs the test, so a test that reached the real one would
// restart a shell or move a window on the machine it was written on.
//
// So none of them call os/exec directly. They take a Func, the real one is
// built only by the thin entry point of each command, and a test hands in a
// table of canned answers. A test that forgets to inject anything has nothing
// to run: the zero Func is nil and panics at the call, loudly, before any
// process starts.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Func runs a program and returns what it wrote to standard output.
//
// A program that ran and failed returns an *ExitError carrying its standard
// error; a program that could not be started returns the start error, which
// NotFound recognises.
type Func func(name string, args ...string) (string, error)

// ExitError is a program that ran and exited non-zero.
type ExitError struct {
	Name   string
	Args   []string
	Code   int
	Stderr string
}

func (e *ExitError) Error() string {
	detail := firstLine(e.Stderr)
	if detail == "" {
		detail = fmt.Sprintf("exit status %d", e.Code)
	}
	return fmt.Sprintf("%s %s: %s", e.Name, strings.Join(e.Args, " "), detail)
}

// Message is the program's own words, without the command line. It is what a
// person wants to read after "could not restart the shell:".
func (e *ExitError) Message() string {
	if detail := firstLine(e.Stderr); detail != "" {
		return detail
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

// NotFound reports whether the program is not installed.
func NotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}

// Exec returns a Func that runs the real program, with a deadline.
//
// The deadline is there because every program this tool leans on can hang: a
// compositor that is busy answers hyprctl in five seconds or never, and
// systemctl waits on a bus that may have gone. A control command that blocks
// forever is one the user can only stop with a signal.
//
// LC_ALL=C because the output is parsed, and a translated "active" is not
// "active".
func Exec(timeout time.Duration) Func {
	return func(name string, args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		command := exec.CommandContext(ctx, name, args...)
		command.Env = append(command.Environ(), "LC_ALL=C")
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr

		err := command.Run()
		if ctx.Err() != nil {
			return stdout.String(), fmt.Errorf("%s: no answer after %s", name, timeout)
		}
		if err != nil {
			var exited *exec.ExitError
			if errors.As(err, &exited) {
				return stdout.String(), &ExitError{
					Name: name, Args: args,
					Code: exited.ExitCode(), Stderr: stderr.String(),
				}
			}
			return stdout.String(), err
		}
		return stdout.String(), nil
	}
}

// Reason is the short human form of an error from a Func: the program's own
// message when it ran and failed, the error text otherwise.
func Reason(err error) string {
	var exited *ExitError
	if errors.As(err, &exited) {
		return exited.Message()
	}
	return err.Error()
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if before, _, found := strings.Cut(text, "\n"); found {
		return strings.TrimSpace(before)
	}
	return text
}
