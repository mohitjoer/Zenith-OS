package main

import (
	"bytes"
	"strings"
	"testing"

	"zenith/desktop/internal/wm"
)

// The wiring is checked against a client with no provider, so nothing reaches
// the live compositor: what the actions do is internal/wm's own tests. This
// pins what the scripts depend on, which is the output labels and the exit
// codes a caller can tell apart.
func TestWmWithNoProvider(t *testing.T) {
	none := wm.OpenNamed("")

	t.Run("status still prints the labels the scripts read", func(t *testing.T) {
		var out, errOut bytes.Buffer
		if code := wmMain([]string{"status"}, none, &out, &errOut); code != exitOK {
			t.Fatalf("exit %d: %s", code, errOut.String())
		}
		for _, label := range []string{"Provider: (none)", "Live: false", "Capabilities: unavailable"} {
			if !strings.Contains(out.String(), label) {
				t.Errorf("status lacks %q:\n%s", label, out.String())
			}
		}
	})

	t.Run("act with nothing to act on is exit 3", func(t *testing.T) {
		var out, errOut bytes.Buffer
		if code := wmMain([]string{"act", "config.reload"}, none, &out, &errOut); code != 3 {
			t.Errorf("exit %d, want 3: %s", code, errOut.String())
		}
	})

	t.Run("caps cannot be read", func(t *testing.T) {
		var out, errOut bytes.Buffer
		if code := wmMain([]string{"caps"}, none, &out, &errOut); code != exitFail {
			t.Errorf("exit %d, want %d", code, exitFail)
		}
	})

	t.Run("usage errors", func(t *testing.T) {
		for _, args := range [][]string{nil, {"state"}, {"act"}} {
			var out, errOut bytes.Buffer
			if code := wmMain(args, none, &out, &errOut); code != exitUsage {
				t.Errorf("zenith wm %v exits %d, want %d", args, code, exitUsage)
			}
		}
	})
}

func TestWmCallTimeoutIsChecked(t *testing.T) {
	t.Setenv("ZENITH_WM_CALL_TIMEOUT", "soon")
	var out, errOut bytes.Buffer
	if code := wmMain([]string{"act", "config.reload"}, wm.OpenNamed(""), &out, &errOut); code != exitUsage {
		t.Errorf("a bad timeout exits %d, want %d", code, exitUsage)
	}
}
