package svcmgr

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"zenith/desktop/internal/runner/runnertest"
)

// The two families are two tables of verbs over two notions of state, and
// both columns are asserted here — not just the systemd one the development
// machine answers, which is the test that passes forever and fails at the
// first Void boot.
//
// The fake's answers queue per command line and the last one repeats once the
// queue runs out, so every answer for a line is registered before the calls
// that consume them, in order — an answer registered after a call would sit
// behind the repeating one and never be seen.

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

func TestKindFromComm(t *testing.T) {
	cases := []struct {
		comm string
		want Kind
	}{
		{"systemd\n", Systemd},
		{"(systemd)\n", Systemd},
		{"runit\n", Runit},
		{"init\n", Unknown},
		{"", Unknown},
	}
	for _, c := range cases {
		if got := kindFromComm(c.comm); got != c.want {
			t.Errorf("kindFromComm(%q) = %s, want %s", c.comm, got, c.want)
		}
	}
}

func TestRunitServiceName(t *testing.T) {
	cases := []struct {
		unit string
		want string
	}{
		{"zenith-shell.service", "quickshell"},
		{"zenithd.service", "zenithd"},
		{"zenith-palette-bridge.service", "zenith-palette-bridge"},
		{"zenith-session.target", "zenith-session"},
		{"zenith-ai-usage.timer", "zenith-ai-usage"},
		{"voxtype.service", "voxtype"},
	}
	for _, c := range cases {
		if got := RunitServiceName(c.unit); got != c.want {
			t.Errorf("RunitServiceName(%q) = %s, want %s", c.unit, got, c.want)
		}
	}
}

func TestSystemdVerbs(t *testing.T) {
	fake := runnertest.New()
	m := New(Systemd, fake)
	unit := "zenithd.service"
	user := "systemctl --user "

	// ── the whole run's answer table, registered first ────────────────────
	fake.On(user+"is-active --quiet "+unit, runnertest.Answer{})
	fake.On(user+"is-enabled --quiet "+unit, runnertest.Answer{})
	fake.On(user+"enable --now "+unit, runnertest.Answer{})
	fake.On(user+"disable --now "+unit, runnertest.Answer{})
	fake.On(user+"restart "+unit, runnertest.Answer{})
	fake.On(user+"reload "+unit, runnertest.Answer{})
	fake.On(user+"try-reload-or-restart "+unit, runnertest.Answer{})
	fake.On(user+"show -p MainPID --value "+unit, runnertest.Answer{Stdout: "4242\n"})
	fake.On(user+"set-environment SHELL=/usr/bin/zsh", runnertest.Answer{})
	fake.On("systemctl is-active --quiet asusd.service", runnertest.Answer{})

	// The four unit-state answers, in the order the four Summary calls below
	// consume them: running, exited, not-found, unreachable.
	show := user + "show " + unit + " -p LoadState -p ActiveState -p SubState --no-pager"
	fake.On(show, runnertest.Answer{Stdout: "LoadState=loaded\nActiveState=active\nSubState=running\n"})
	fake.On(show, runnertest.Answer{Stdout: "LoadState=loaded\nActiveState=active\nSubState=exited\n"})
	fake.On(show, runnertest.Answer{Stdout: "LoadState=not-found\nActiveState=\nSubState=\n"})
	fake.Fail(show, runnertest.NotInstalled("systemctl"))

	// ── the verbs ──────────────────────────────────────────────────────────
	if !m.Active(unit) {
		t.Error("is-active success should read as active")
	}
	if !m.Enabled(unit) {
		t.Error("is-enabled success should read as enabled")
	}
	if err := m.EnableNow(unit); err != nil {
		t.Errorf("EnableNow: %v", err)
	}
	if err := m.DisableNow(unit); err != nil {
		t.Errorf("DisableNow: %v", err)
	}
	if err := m.Restart(unit); err != nil {
		t.Errorf("Restart: %v", err)
	}
	if err := m.Reload(unit); err != nil {
		t.Errorf("Reload: %v", err)
	}
	if err := m.TryReloadOrRestart(unit); err != nil {
		t.Errorf("TryReloadOrRestart: %v", err)
	}
	if got := m.MainPID(unit); got != "4242" {
		t.Errorf("MainPID = %q, want 4242", got)
	}
	if err := m.SetUserEnv("SHELL=/usr/bin/zsh"); err != nil {
		t.Errorf("SetUserEnv: %v", err)
	}
	if !m.SystemActive("asusd.service") {
		t.Error("systemctl is-active success should read as system-active")
	}

	// ── the state lines Summary composes the phrase from ───────────────────
	if got := m.Summary(unit); got != "active (running)" {
		t.Errorf("Summary = %q, want active (running)", got)
	}
	if got := m.Summary(unit); got != "active (exited)" {
		t.Errorf("Summary = %q, want active (exited)", got)
	}
	if got := m.Summary(unit); got != "not installed" {
		t.Errorf("Summary = %q, want not installed", got)
	}
	if got := m.Summary(unit); got != "unknown (the user service manager is not reachable)" {
		t.Errorf("Summary = %q, want the unreachable phrase", got)
	}
}

func TestSystemdPowerArgv(t *testing.T) {
	if argv, ok := PowerArgv(Systemd, "reboot"); !ok || !reflect.DeepEqual(argv, []string{"systemctl", "reboot"}) {
		t.Errorf("PowerArgv(Systemd, reboot) = %v, %t", argv, ok)
	}
	if argv, ok := PowerArgv(Systemd, "poweroff"); !ok || !reflect.DeepEqual(argv, []string{"systemctl", "poweroff"}) {
		t.Errorf("PowerArgv(Systemd, poweroff) = %v, %t", argv, ok)
	}
	if _, ok := PowerArgv(Systemd, "suspend"); ok {
		t.Error("suspend is not an argv action; the dialog has none either")
	}
	if _, ok := PowerArgv(Unknown, "reboot"); !ok {
		t.Error("an unknown kind answers with the systemd table, the base the dev machines talk")
	}
}

func TestRunitVerbs(t *testing.T) {
	fake, m, svdir := fakeManager(t)
	dir := filepath.Join(svdir, "zenithd")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run := "sv status " + dir

	// ── the answer table, unchanged order = the call order below ───────────
	up := runnertest.Answer{Stdout: "run: zenithd: (pid 901) 33102s\n"}
	down := runnertest.Answer{Stdout: "down: zenithd: normally up, got TERM\n"}
	// status calls in order: Active, MainPID, Summary (up), Active, Summary
	// (down), Summary (exhausted-repeats-down).
	fake.On(run, up)
	fake.On(run, up)
	fake.On(run, up)
	fake.On(run, down)

	// ── the up run: no down marker, sv says run ────────────────────────────
	if !m.Active("zenithd.service") {
		t.Error("a run: status line is running")
	}
	if !m.Enabled("zenithd.service") {
		t.Error("no down marker is enabled")
	}
	if got := m.MainPID("zenithd.service"); got != "901" {
		t.Errorf("MainPID = %q, want 901", got)
	}
	if got := m.Summary("zenithd.service"); got != "active (running)" {
		t.Errorf("Summary = %q, want active (running)", got)
	}

	// ── the down run: the marker is laid, sv says down ────────────────────
	if err := os.WriteFile(filepath.Join(dir, "down"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if m.Active("zenithd.service") {
		t.Error("a down: status line is not running")
	}
	if m.Enabled("zenithd.service") {
		t.Error("a down marker is disabled")
	}
	if got := m.Summary("zenithd.service"); got != "inactive" {
		t.Errorf("Summary = %q, want inactive", got)
	}

	// EnableNow removes the marker and runs sv up.
	fake.On("sv up "+dir, runnertest.Answer{})
	if err := m.EnableNow("zenithd.service"); err != nil {
		t.Errorf("EnableNow: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "down")); err == nil {
		t.Error("EnableNow left the down marker in place")
	}

	// DisableNow runs sv down and lays the marker back.
	fake.On("sv down "+dir, runnertest.Answer{})
	if err := m.DisableNow("zenithd.service"); err != nil {
		t.Errorf("DisableNow: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "down")); err != nil {
		t.Error("DisableNow did not lay the down marker")
	}

	if err := os.Remove(filepath.Join(dir, "down")); err != nil {
		t.Fatal(err)
	}

	// Every reload-adjacent verb is the same sv restart: three verbs, three
	// queued restarts, no fourth.
	fake.On("sv restart "+dir, runnertest.Answer{})
	fake.On("sv restart "+dir, runnertest.Answer{})
	fake.On("sv restart "+dir, runnertest.Answer{})
	for name, op := range map[string]func() error{
		"Restart":            func() error { return m.Restart("zenithd.service") },
		"Reload":             func() error { return m.Reload("zenithd.service") },
		"TryReloadOrRestart": func() error { return m.TryReloadOrRestart("zenithd.service") },
	} {
		if err := op(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	// An unseeded service is an error to enable and "not installed" to
	// describe — sv is asked, sv cannot answer (the fake has none), and the
	// directory is absent.
	if err := m.EnableNow("ghost.service"); err == nil {
		t.Error("EnableNow of an unseeded service should be an error")
	}
	if got := m.Summary("ghost.service"); got != "not installed" {
		t.Errorf("Summary of an unseeded service = %q, want not installed", got)
	}

	// A supervised dir that sv could not answer still reads from its run
	// marker. The ghost dir takes the run marker instead, to keep the zenithd
	// status queue's repeat answer from being the case being tested.
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
	New(Systemd, nil)
}
