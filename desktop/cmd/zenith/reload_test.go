package main

import (
	"bytes"
	"strings"
	"testing"

	"zenith/desktop/internal/runner/runnertest"
)

// TestReload covers what the Hub's caller depends on: the shell is reloaded
// through its unit, the daemon only when it is running, and a unit that will
// not reload is a failure the caller can see. The fake runner is the only
// thing that answers, so the live session is never reached.
func TestReload(t *testing.T) {
	shell := "systemctl --user reload zenith-shell.service"
	active := "systemctl --user is-active --quiet zenithd.service"
	daemon := "systemctl --user try-reload-or-restart zenithd.service"

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
				f.Out(shell, "").Out(active, "").Out(daemon, "")
			},
			wantCode: exitOK,
			wantOut:  []string{"shell:   reloaded", "daemon:  reloaded"},
			wantRan:  []string{shell, active, daemon},
		},
		{
			name: "no daemon to reload",
			setup: func(f *runnertest.Fake) {
				f.Out(shell, "").Fail(active, runnertest.Exit(3, ""))
			},
			wantCode: exitOK,
			wantOut:  []string{"shell:   reloaded", "daemon:  not running"},
			wantNot:  []string{daemon},
		},
		{
			name: "the shell unit refuses",
			setup: func(f *runnertest.Fake) {
				f.Fail(shell, runnertest.Exit(1, "Unit zenith-shell.service is not loaded.")).
					Fail(active, runnertest.Exit(3, ""))
			},
			wantCode: exitFail,
			wantErr:  []string{"shell:   not reloaded: Unit zenith-shell.service is not loaded."},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := runnertest.New()
			tc.setup(fake)
			var out, errOut bytes.Buffer
			if code := reloadMain(nil, &out, &errOut, fake.Run); code != tc.wantCode {
				t.Errorf("exit %d, want %d", code, tc.wantCode)
			}
			for _, want := range tc.wantOut {
				if !strings.Contains(out.String(), want) {
					t.Errorf("stdout %q lacks %q", out.String(), want)
				}
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(errOut.String(), want) {
					t.Errorf("stderr %q lacks %q", errOut.String(), want)
				}
			}
			for _, want := range tc.wantRan {
				if !fake.Ran(want) {
					t.Errorf("did not run %q; ran %v", want, fake.Calls())
				}
			}
			for _, not := range tc.wantNot {
				if fake.Ran(not) {
					t.Errorf("ran %q, which it should not have", not)
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
