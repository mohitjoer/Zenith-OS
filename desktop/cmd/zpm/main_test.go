package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fake zenith is a shell script that echoes its argv, because what the
// test asks zpm is: does the forward pass through name, verbs, and statuses —
// and nothing else.

func fakeZenith(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "zenith")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestForwardPassesEverythingThrough(t *testing.T) {
	zenith := fakeZenith(t, `echo "pkg $*"`)

	out, exit := capture(t, zenith, []string{"install", "-y", "cpu"})
	if exit != 0 {
		t.Errorf("exit %d", exit)
	}
	if !strings.Contains(out, "pkg install -y cpu") {
		t.Errorf("stdout %q lacks the forwarded argv", out)
	}
}

func TestForwardCarriesTheExitStatus(t *testing.T) {
	zenith := fakeZenith(t, `exit 3`)

	_, exit := capture(t, zenith, []string{"update"})
	if exit != 3 {
		t.Errorf("exit %d, want the manager's own 3", exit)
	}
}

// capture runs forward against a fake zenith, with a pipe standing in for
// the process's real stdout so the output can be read back.
func capture(t *testing.T, zenithPath string, argv []string) (string, int) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	exit := forward(zenithPath, argv)

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	return string(buf[:n]), exit
}
