package svcmgr

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"asura/desktop/internal/runner/runnertest"
)

func fakeManager(t *testing.T) (*runnertest.Fake, *Manager, string) {
	t.Helper()
	config := filepath.Join(t.TempDir(), "xdg")
	t.Setenv("XDG_CONFIG_HOME", config)
	svdir := filepath.Join(config, "service")
	if err := os.MkdirAll(svdir, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := runnertest.New()
	return fake, New(Runit, fake), svdir
}

func TestRunitServiceName(t *testing.T) {
	cases := []struct {
		unit string
		want string
	}{
		{"asura-shell.service", "quickshell"},
		{"asurad.service", "asurad"},
		{"asura-palette-bridge.service", "asura-palette-bridge"},
		{"asura-session.target", "asura-session"},
		{"asura-ai-usage.timer", "asura-ai-usage"},
		{"voxtype.service", "voxtype"},
	}
	for _, c := range cases {
		if got := RunitServiceName(c.unit); got != c.want {
			t.Errorf("RunitServiceName(%q) = %s, want %s", c.unit, got, c.want)
		}
	}
}

func TestRunitVerbs(t *testing.T) {
	fake, m, svdir := fakeManager(t)
	dir := filepath.Join(svdir, "asurad")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run := "sv status " + dir

	// ── the answer table, unchanged order = the call order below ───────────
	up := runnertest.Answer{Stdout: "run: asurad: (pid 901) 33102s\n"}
	down := runnertest.Answer{Stdout: "down: asurad: normally up, got TERM\n"}
	fake.On(run, up)
	fake.On(run, up)
	fake.On(run, up)
	fake.On(run, down)

	// ── the up run: no down marker, sv says run ────────────────────────────
	if !m.Active("asurad.service") {
		t.Error("a run: status line is running")
	}
	if !m.Enabled("asurad.service") {
		t.Error("no down marker is enabled")
	}
	if got := m.MainPID("asurad.service"); got != "901" {
		t.Errorf("MainPID = %q, want 901", got)
	}
	if got := m.Summary("asurad.service"); got != "active (running)" {
		t.Errorf("Summary = %q, want active (running)", got)
	}

	// ── the down run: the marker is laid, sv says down ────────────────────
	if err := os.WriteFile(filepath.Join(dir, "down"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if m.Active("asurad.service") {
		t.Error("a down: status line is not running")
	}
	if m.Enabled("asurad.service") {
		t.Error("a down marker is disabled")
	}
	if got := m.Summary("asurad.service"); got != "inactive" {
		t.Errorf("Summary = %q, want inactive", got)
	}

	// EnableNow removes the marker and runs sv up.
	fake.On("sv up "+dir, runnertest.Answer{})
	if err := m.EnableNow("asurad.service"); err != nil {
		t.Errorf("EnableNow: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "down")); err == nil {
		t.Error("EnableNow left the down marker in place")
	}

	// DisableNow runs sv down and lays the marker back.
	fake.On("sv down "+dir, runnertest.Answer{})
	if err := m.DisableNow("asurad.service"); err != nil {
		t.Errorf("DisableNow: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "down")); err != nil {
		t.Error("DisableNow did not lay the down marker")
	}

	if err := os.Remove(filepath.Join(dir, "down")); err != nil {
		t.Fatal(err)
	}

	// Every reload-adjacent verb is the same sv restart
	fake.On("sv restart "+dir, runnertest.Answer{})
	fake.On("sv restart "+dir, runnertest.Answer{})
	fake.On("sv restart "+dir, runnertest.Answer{})
	for name, op := range map[string]func() error{
		"Restart":            func() error { return m.Restart("asurad.service") },
		"Reload":             func() error { return m.Reload("asurad.service") },
		"TryReloadOrRestart": func() error { return m.TryReloadOrRestart("asurad.service") },
	} {
		if err := op(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	// An unseeded service is an error to enable and "not installed" to describe
	if err := m.EnableNow("ghost.service"); err == nil {
		t.Error("EnableNow of an unseeded service should be an error")
	}
	if got := m.Summary("ghost.service"); got != "not installed" {
		t.Errorf("Summary of an unseeded service = %q, want not installed", got)
	}

	// A supervised dir that sv could not answer still reads from its run marker
	ghostDir := filepath.Join(svdir, "ghost")
	if err := os.Mkdir(ghostDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ghostDir, "run"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := m.Summary("ghost.service"); got != "active (running)" {
		t.Errorf("Summary with a live run file = %q, want active (running)", got)
	}

	// The envdir helper carries the variable under the runit backend.
	fake.On("turnstile-update-runit-env SHELL=/usr/bin/zsh", runnertest.Answer{})
	if err := m.SetUserEnv("SHELL=/usr/bin/zsh"); err != nil {
		t.Errorf("SetUserEnv: %v", err)
	}

	fake.On("sv status /var/service/asusd", runnertest.Answer{Stdout: "run: asusd: (pid 611) 902s\n"})
	if !m.SystemActive("asusd.service") {
		t.Error("a run: /var/service line is system-active")
	}
}

func TestRunitPowerArgv(t *testing.T) {
	if argv, ok := PowerArgv(Runit, "reboot"); !ok || !reflect.DeepEqual(argv, []string{"loginctl", "reboot"}) {
		t.Errorf("PowerArgv(Runit, reboot) = %v, %t", argv, ok)
	}
	if argv, ok := PowerArgv(Runit, "poweroff"); !ok || !reflect.DeepEqual(argv, []string{"loginctl", "poweroff"}) {
		t.Errorf("PowerArgv(Runit, poweroff) = %v, %t", argv, ok)
	}
	if _, ok := PowerArgv(Runit, "suspend"); ok {
		t.Error("suspend is not an argv action")
	}
}

func TestNewNeedsRunner(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New without a runner should panic loudly, not turn into os/exec")
		}
	}()
	New(Runit, nil)
}
