package runner

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// The real Exec is exercised against programs that cannot touch anything: sh
// and a name that is not installed.
func TestExecReportsWhatTheProgramDid(t *testing.T) {
	run := Exec(5 * time.Second)

	t.Run("stdout is returned", func(t *testing.T) {
		out, err := run("sh", "-c", "printf hello")
		if err != nil || out != "hello" {
			t.Fatalf("got %q, %v; want hello, nil", out, err)
		}
	})

	t.Run("a failing program carries its own words", func(t *testing.T) {
		_, err := run("sh", "-c", "echo broken >&2; exit 3")
		var exited *ExitError
		if !errors.As(err, &exited) {
			t.Fatalf("error is %T, want *ExitError", err)
		}
		if exited.Code != 3 || exited.Message() != "broken" {
			t.Errorf("got code %d message %q, want 3 and broken", exited.Code, exited.Message())
		}
		if Reason(err) != "broken" {
			t.Errorf("Reason = %q, want the program's message", Reason(err))
		}
	})

	t.Run("a program that is not installed is recognised", func(t *testing.T) {
		_, err := run("zenith-no-such-program-w17")
		if !NotFound(err) {
			t.Fatalf("NotFound(%v) = false, want true", err)
		}
	})

	t.Run("a program that never answers is stopped", func(t *testing.T) {
		started := time.Now()
		_, err := Exec(150*time.Millisecond)("sleep", "5")
		if err == nil || !strings.Contains(err.Error(), "no answer after") {
			t.Fatalf("got %v, want a timeout", err)
		}
		if time.Since(started) > 3*time.Second {
			t.Errorf("the deadline did not stop the program: %s", time.Since(started))
		}
	})
}

func TestExitErrorFallsBackToTheStatus(t *testing.T) {
	err := &ExitError{Name: "x", Args: []string{"y"}, Code: 7}
	if err.Message() != "exit status 7" {
		t.Errorf("Message = %q", err.Message())
	}
	if !strings.Contains(err.Error(), "x y") {
		t.Errorf("Error = %q, want the command line in it", err.Error())
	}
}
