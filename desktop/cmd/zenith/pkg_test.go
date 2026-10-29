package main

import (
	"bytes"
	"reflect"
	"testing"

	"zenith/desktop/internal/runner"
	"zenith/desktop/internal/runner/runnertest"
)

// pkgArgv is the table the verbs stand on, so the tests are the table: every
// backend-verb pair this file claims, and the pieces that ride with them —
// the sudo prefix, the -R repository, the package list.

// withNoSudo takes the sudo decision out of the machine's hands: the test
// runs as the same user the operator does, and the argv tables are about the
// commands, not about who runs them.
func withNoSudo(t *testing.T) {
	t.Helper()
	original := pkgSudoPick
	pkgSudoPick = func() []string { return nil }
	t.Cleanup(func() { pkgSudoPick = original })
}

func TestPkgArgvXbps(t *testing.T) {
	withNoSudo(t)
	cases := []struct {
		name     string
		verb     string
		packages []string
		repo     string
		want     []string
	}{
		{"install", "install", []string{"cpu"}, "", []string{"xbps-install", "-S", "-y", "cpu"}},
		{"sync", "update", nil, "", []string{"xbps-install", "-S"}},
		{"upgrade everything", "upgrade", nil, "", []string{"xbps-install", "-Su"}},
		{"remove", "remove", []string{"pkg2"}, "", []string{"xbps-remove", "-y", "pkg2"}},
		{"search", "search", []string{"ghostty"}, "", []string{"xbps-query", "-Rs", "ghostty"}},
		{"the repository rides first", "install", []string{"cpu"},
			"https://zenithos.dev/xbps/unstable",
			[]string{"xbps-install", "-R", "https://zenithos.dev/xbps/unstable", "-S", "-y", "cpu"}},
	}
	for _, c := range cases {
		got, ok := pkgArgv("xbps", c.verb, c.packages, c.repo)
		if !ok {
			t.Errorf("%s: not composed", c.name)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestPkgArgvApt(t *testing.T) {
	withNoSudo(t)
	cases := []struct {
		name     string
		verb     string
		packages []string
		want     []string
	}{
		{"install", "install", []string{"ghostty"}, []string{"apt-get", "install", "--yes", "ghostty"}},
		{"update", "update", nil, []string{"apt-get", "update"}},
		{"upgrade", "upgrade", nil, []string{"apt-get", "full-upgrade", "--yes"}},
		{"remove", "remove", []string{"ghostty"}, []string{"apt-get", "remove", "--yes", "ghostty"}},
		{"search", "search", []string{"ghostty"}, []string{"apt", "search", "ghostty"}},
	}
	for _, c := range cases {
		got, ok := pkgArgv("apt", c.verb, c.packages, "")
		if !ok {
			t.Errorf("%s: not composed", c.name)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestPkgArgvFlatpak(t *testing.T) {
	withNoSudo(t)
	cases := []struct {
		name string
		verb string
		pkg  string
		want []string
	}{
		{"install", "install", "org.mozilla.firefox", []string{"flatpak", "install", "-y", "org.mozilla.firefox"}},
		{"uninstall", "remove", "org.mozilla.firefox", []string{"flatpak", "uninstall", "-y", "org.mozilla.firefox"}},
		{"search", "search", "zen browser", []string{"flatpak", "search", "zen browser"}},
	}
	for _, c := range cases {
		got, ok := pkgArgv("flatpak", c.verb, []string{c.pkg}, "")
		if !ok {
			t.Errorf("%s: not composed", c.name)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s = %s, want %s", c.name, got, c.want)
		}
	}
}

// A search runs without sudo — elevation is only ever in the way of the
// output the person is reading.
func TestPkgSearchNeedsNoSudo(t *testing.T) {
	original := pkgSudoPick
	pkgSudoPick = func() []string { return []string{"sudo"} }
	defer func() { pkgSudoPick = original }()

	want := []string{"apt", "search", "ghostty"}
	got, ok := pkgArgv("apt", "search", []string{"ghostty"}, "")
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("search argv = %v, want %v unchanged by the sudo pick", got, want)
	}
}

func TestPkgArgvNeedsPackages(t *testing.T) {
	if _, ok := pkgArgv("xbps", "install", nil, ""); ok {
		t.Error("install without packages composed — the manager would run without a package")
	}
	if _, ok := pkgArgv("xbps", "ghost", nil, ""); ok {
		t.Error("a verb that is not in the table composed")
	}
	if _, ok := pkgArgv("unknown", "install", []string{"cpu"}, ""); ok {
		t.Error("an unknown backend composed")
	}
	// update takes exactly none, and that is also what it gets.
	if _, ok := pkgArgv("apt", "update", nil, ""); !ok {
		t.Error("update with no packages is the normal form")
	}
}

// pkgMain runs through a delivered backend, because the machine's own answer
// is one of two possible systems and a test must not depend on which the
// test machine carries.
func TestPkgMainRoutesToFlatpak(t *testing.T) {
	var out, errOut bytes.Buffer
	fake := runnertest.New()
	fake.On("flatpak install -y org.mozilla.firefox", runnertest.Answer{Stdout: "installed\n"})

	code := pkgMain([]string{"install", "org.mozilla.firefox"}, &out, &errOut, fake.Run, "apt")
	if code != exitOK {
		t.Errorf("exit %d, want %d; stderr %q", code, exitOK, errOut.String())
	}
	if !fake.Ran("flatpak install -y org.mozilla.firefox") {
		t.Errorf("ran %v, want the flatpak route", fake.Calls())
	}
}

func TestPkgMainRunsTheDeliveredBackend(t *testing.T) {
	var out, errOut bytes.Buffer
	fake := runnertest.New()
	fake.On("xbps-remove -y cpu", runnertest.Answer{Stdout: "removed\n"})

	original := pkgSudoPick
	pkgSudoPick = func() []string { return nil }
	defer func() { pkgSudoPick = original }()

	code := pkgMain([]string{"remove", "cpu"}, &out, &errOut, fake.Run, "xbps")
	if code != exitOK {
		t.Errorf("exit %d, want %d; stderr %q", code, exitOK, errOut.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("removed")) {
		t.Errorf("stdout %q lacks the manager's own answer", out.String())
	}
}

func TestPkgMainPassesTheExitCodeThrough(t *testing.T) {
	var out, errOut bytes.Buffer
	fake := runnertest.New()
	fake.Fail("xbps-install -S -y cpu", &runner.ExitError{
		Name: "xbps-install", Args: []string{"-S", "-y", "cpu"}, Code: 1, Stderr: "mospheric"},
	)

	original := pkgSudoPick
	pkgSudoPick = func() []string { return nil }
	defer func() { pkgSudoPick = original }()
	// Ubuntu carries apt; deliver xbps directly means the fake answers anyway.
	code := pkgMain([]string{"install", "cpu"}, &out, &errOut, fake.Run, "xbps")
	if code != exitFail {
		t.Errorf("exit %d, want %d", code, exitFail)
	}
	if !bytes.Contains(errOut.Bytes(), []byte("xbps-install -S -y cpu")) {
		t.Errorf("stderr %q lacks the command that failed", errOut.String())
	}
}

func TestPkgMainUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := pkgMain([]string{}, &out, &errOut, runnertest.New().Run, "xbps"); code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
}
