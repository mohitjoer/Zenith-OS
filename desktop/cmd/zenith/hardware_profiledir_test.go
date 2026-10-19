package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProfileCandidatesAreNotOneDirectoryTooHigh is the regression for a bug
// that made an installed zenith unable to find the profile database it shipped.
//
// filepath.Dir(filepath.Dir("/usr/bin/zenith")) is "/" and not "/usr", so the
// installed candidate used to be "/share/zenith/hardware/profiles". The search
// then fell through to a path relative to the working directory, which resolved
// to something that did not exist or, worse, to the checkout — and the failure
// presented as "an empty database", indistinguishable from a machine with no
// profiles at all. That message is the one result the linter refuses to print,
// because it looks like there is nothing to do when the tool is actually broken.
func TestProfileCandidatesAreNotOneDirectoryTooHigh(t *testing.T) {
	candidates := profileCandidates()
	if len(candidates) == 0 {
		t.Fatal("no candidates")
	}

	installed := candidates[0]
	want := filepath.Join("share", "zenith", "hardware", "profiles")
	if !strings.HasSuffix(installed, want) {
		t.Errorf("installed candidate = %q, want it to end in %q", installed, want)
	}

	// The whole bug in one line: exactly one hop up from the executable's own
	// directory. Two hops — Dir(Dir(exe)) — lands one level too high, which for
	// /usr/bin/zenith is /share rather than /usr/share.
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wantOneHop := filepath.Join(filepath.Dir(executable), "..", want)
	if installed != wantOneHop {
		t.Errorf("installed candidate = %q, want %q; an installed zenith looks "+
			"in the wrong place", installed, wantOneHop)
	}

	// The last candidate is always the checkout layout, so the tool runs from a
	// repository with nothing installed.
	if candidates[len(candidates)-1] != filepath.Join("system", "hardware", "profiles") {
		t.Errorf("last candidate = %q, want the repository layout", candidates[len(candidates)-1])
	}
}
