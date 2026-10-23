// Package runnertest is the fake program runner the tests use.
//
// It lives apart from package runner so that nothing in the shipped binary
// imports it.
package runnertest

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"zenith/desktop/internal/runner"
)

// Answer is what a fake program does when it is run.
type Answer struct {
	Stdout string
	// Err is returned as it stands. Use Exit for a program that ran and failed.
	Err error
}

// Exit builds the error a program that ran and failed returns.
func Exit(code int, stderr string) error {
	return &runner.ExitError{Name: "fake", Code: code, Stderr: stderr}
}

// Fake answers commands from a table and records every one it was asked.
//
// A command with no answer is an error, not an empty success: a test that
// reaches a command it did not expect has found a code path it did not know
// about, and silently succeeding would hide it.
type Fake struct {
	mu      sync.Mutex
	answers map[string][]Answer
	calls   []string
}

// New returns a fake with no answers.
func New() *Fake {
	return &Fake{answers: map[string][]Answer{}}
}

// On registers the answer for a command line, written as the words joined by
// single spaces. Registering the same line again queues another answer, taken
// in order; the last one repeats once the queue is down to it.
func (f *Fake) On(line string, answer Answer) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers[line] = append(f.answers[line], answer)
	return f
}

// Out registers a successful answer that prints stdout.
func (f *Fake) Out(line, stdout string) *Fake {
	return f.On(line, Answer{Stdout: stdout})
}

// Fail registers an answer that fails.
func (f *Fake) Fail(line string, err error) *Fake {
	return f.On(line, Answer{Err: err})
}

// Run is the runner.Func.
func (f *Fake) Run(name string, args ...string) (string, error) {
	line := strings.TrimSpace(name + " " + strings.Join(args, " "))

	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, line)

	queue, known := f.answers[line]
	if !known || len(queue) == 0 {
		return "", fmt.Errorf("runnertest: no answer for %q", line)
	}
	answer := queue[0]
	if len(queue) > 1 {
		f.answers[line] = queue[1:]
	}
	return answer.Stdout, answer.Err
}

// Calls returns every command line run so far, in order.
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// Ran reports whether a command line was run.
func (f *Fake) Ran(line string) bool {
	for _, call := range f.Calls() {
		if call == line {
			return true
		}
	}
	return false
}

// NotInstalled is the error a missing program produces, the one
// runner.NotFound recognises.
func NotInstalled(name string) error {
	return fmt.Errorf("%s: %w", name, exec.ErrNotFound)
}
