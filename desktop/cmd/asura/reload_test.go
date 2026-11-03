package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"asura/desktop/internal/runner/runnertest"
)

// TestReload covers what the Hub's caller depends on: the shell is reloaded
// through sv restart on its service directory, the daemon only when it is running,
// and a service that will not reload is a failure the caller can see.
func TestReload(t *testing.T) {
	svcDir := filepath.Join(t.TempDir(), "service")
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(svcDir))

	shell := "sv restart " + filepath.Join(svcDir, "quickshell")
	status := "sv status " + filepath.Join(svcDir, "asurad")
	daemon := "sv restart " + filepath.Join(svcDir, "asurad")

	tests := []struct {
		name     string
		setup    func(*runnertest.Fake)
		wantCode int
		wantOut  []string
		wantErr  []string
		wantRan  []string
		wantNot  []string
	}{
		{
			name: "both reload",
			setup: func(f *runnertest.Fake) {
				f.Out(shell, "").Out(status, "run: asurad: (pid 1234) 10s\n").Out(daemon, "")
			},
			wantCode: exitOK,
			wantOut:  []string{"shell:   reloaded", "daemon:  reloaded"},
			wantRan:  []string{shell, status, daemon},
		},
		{
			name: "no daemon to reload",
			setup: func(f *runnertest.Fake) {
				f.Out(shell, "").Fail(status, runnertest.Exit(1, "fail: asurad: unable to change to service directory: file does not exist\n"))
			},
			wantCode: exitOK,
			wantOut:  []string{"shell:   reloaded", "daemon:  not running"},
			wantNot:  []string{daemon},
		},
		{
			name: "the shell refuses",
			setup: func(f *runnertest.Fake) {
				f.Fail(shell, runnertest.Exit(1, "fail: quickshell: unable to change to service directory\n")).
					Fail(status, runnertest.Exit(1, "fail: asurad: unable to change to service directory\n"))
			},
			wantCode: exitFail,
			wantErr:  []string{"shell:   not reloaded: fail: quickshell: unable to change to service directory"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := runnertest.New()
			tc.setup(fake)

			var out, errOut bytes.Buffer
			code := reloadMain(nil, &out, &errOut, fake.Run)
			if code != tc.wantCode {
				t.Fatalf("exit %d, want %d (stderr %q)", code, tc.wantCode, errOut.String())
			}
			for _, w := range tc.wantOut {
				if !strings.Contains(out.String(), w) {
					t.Errorf("stdout lacks %q; got:\n%s", w, out.String())
				}
			}
			for _, w := range tc.wantErr {
				if !strings.Contains(errOut.String(), w) {
					t.Errorf("stderr lacks %q; got:\n%s", w, errOut.String())
				}
			}
			for _, r := range tc.wantRan {
				if !fake.Ran(r) {
					t.Errorf("did not run %q; calls were %v", r, fake.Calls())
				}
			}
			for _, n := range tc.wantNot {
				if fake.Ran(n) {
					t.Errorf("ran %q, should not have", n)
				}
			}
		})
	}
}

func TestReloadTakesNoArguments(t *testing.T) {
	var out, errOut bytes.Buffer
	fake := runnertest.New()
	if code := reloadMain([]string{"shell"}, &out, &errOut, fake.Run); code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
	if len(fake.Calls()) != 0 {
		t.Errorf("a usage error ran %v", fake.Calls())
	}
}
