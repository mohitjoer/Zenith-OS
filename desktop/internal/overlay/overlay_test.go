package overlay

import (
	"os"
	"path/filepath"
	"testing"
)

// machine builds a temp tree with a base, an optional set of user edits, and an
// already-materialized config.
//
// It is the whole fixture vocabulary this package needs: a base to lay down, an
// overlay to protect, and a config to rebuild. Nothing here reaches outside the
// temporary directory, which is what lets the real code run unmodified.
type machine struct {
	t     *testing.T
	paths Paths
}

func newMachine(t *testing.T) *machine {
	t.Helper()
	root := t.TempDir()
	paths := Paths{
		Base:   filepath.Join(root, "usr", "share", "zenith"),
		Etc:    filepath.Join(root, "etc", "zenith"),
		Config: filepath.Join(root, "home", ".config", "zenith"),
		Edits:  filepath.Join(root, "home", ".config", "zenith", "user_edits"),
		State:  filepath.Join(root, "home", ".local", "state", "zenith"),
	}
	return &machine{t: t, paths: paths}
}

func (m *machine) baseFile(relative, content string) {
	m.t.Helper()
	m.write(filepath.Join(m.paths.Base, relative), content)
}

func (m *machine) edit(relative, content string) {
	m.t.Helper()
	m.write(filepath.Join(m.paths.Edits, relative), content)
}

func (m *machine) write(path, content string) {
	m.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		m.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		m.t.Fatal(err)
	}
}

func (m *machine) read(path string) string {
	m.t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		m.t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

// TestUserEditSurvivesMaterialize is the invariant the whole model exists for:
// a user's override always wins at its mirrored path, and no update can clobber
// it.
func TestUserEditSurvivesMaterialize(t *testing.T) {
	m := newMachine(t)
	m.baseFile("ghostty/config", "font-family = JetBrains Mono\n")
	m.baseFile("shell/bar.json", "{}\n")
	m.edit("ghostty/config", "font-family = Iosevka\n")

	if err := Materialize(m.paths); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got := m.read(filepath.Join(m.paths.Config, "ghostty/config"))
	if got != "font-family = Iosevka\n" {
		t.Errorf("the user edit was clobbered: %q", got)
	}

	// And a file with no edit is laid down from the base.
	bar := m.read(filepath.Join(m.paths.Config, "shell/bar.json"))
	if bar != "{}\n" {
		t.Errorf("the base file was not materialized: %q", bar)
	}
}

// TestMaterializeIsIdempotent covers a second update: laying the base again must
// change nothing, because every path is either already correct or owned by the
// user.
func TestMaterializeIsIdempotent(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base\n")
	m.baseFile("nested/b.conf", "base\n")
	m.edit("a.conf", "mine\n")

	for round := 0; round < 3; round++ {
		if err := Materialize(m.paths); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
	}
	if got := m.read(filepath.Join(m.paths.Config, "a.conf")); got != "mine\n" {
		t.Errorf("after three materializations the edit is %q", got)
	}
	if got := m.read(filepath.Join(m.paths.Config, "nested/b.conf")); got != "base\n" {
		t.Errorf("the nested base file is %q", got)
	}
}

// TestMaterializeNeverWritesThroughTheOverlay is the sharp form of the same
// invariant: the base must not even be laid into user_edits/, which a
// materialize that walked its own destination would do.
func TestMaterializeNeverWritesThroughTheOverlay(t *testing.T) {
	m := newMachine(t)
	m.baseFile("secret.conf", "base\n")

	if err := Materialize(m.paths); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	if _, err := os.Stat(filepath.Join(m.paths.Edits, "secret.conf")); err == nil {
		t.Error("materialization wrote into user_edits/, which is the one " +
			"clobber the model forbids")
	}
}

// TestResolveReportsThePrecedence checks the three-way lookup.
func TestResolveReportsThePrecedence(t *testing.T) {
	m := newMachine(t)
	m.baseFile("edited.conf", "base\n")
	m.baseFile("materialized.conf", "base\n")
	m.baseFile("base-only.conf", "base\n")
	m.edit("edited.conf", "mine\n")

	if err := os.MkdirAll(m.paths.Config, 0o755); err != nil {
		t.Fatal(err)
	}
	m.write(filepath.Join(m.paths.Config, "materialized.conf"), "base\n")

	tests := []struct {
		relative string
		want     Source
	}{
		{"edited.conf", SourceUserEdit},
		{"materialized.conf", SourceMaterialized},
		{"base-only.conf", SourceBase},
		{"nothing.conf", SourceNone},
	}
	for _, tc := range tests {
		if got := Resolve(m.paths, tc.relative); got.Source != tc.want {
			t.Errorf("Resolve(%q) = %s, want %s", tc.relative, got.Source, tc.want)
		}
	}
}

// TestResetDropsOneEditAndRestoresTheBase covers the only command that
// deliberately destroys a user's change.
func TestResetDropsOneEditAndRestoresTheBase(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base-a\n")
	m.baseFile("b.conf", "base-b\n")
	m.edit("a.conf", "mine-a\n")
	m.edit("b.conf", "mine-b\n")

	if err := Materialize(m.paths); err != nil {
		t.Fatal(err)
	}
	if err := Reset(m.paths, "a.conf"); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if got := m.read(filepath.Join(m.paths.Config, "a.conf")); got != "base-a\n" {
		t.Errorf("the reset file is %q, want the base", got)
	}
	if got := m.read(filepath.Join(m.paths.Config, "b.conf")); got != "mine-b\n" {
		t.Errorf("the reset touched a second file: %q", got)
	}
	if Resolve(m.paths, "a.conf").Source != SourceMaterialized {
		t.Error("the reset file still resolves as a user edit")
	}
}

func TestResetWithNoEditIsAnError(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base\n")
	if err := Reset(m.paths, "a.conf"); err == nil {
		t.Error("resetting a file with no user edit succeeded")
	}
}

// TestResetRefusesPathsOutsideTheOverlay checks the traversal guard.
//
// An update tool that will follow "../" can be told to rewrite a file outside
// the directory it owns.
func TestResetRefusesPathsOutsideTheOverlay(t *testing.T) {
	m := newMachine(t)
	for _, path := range []string{
		"", "/etc/passwd", "../outside", "../../etc/shadow",
		"user_edits", "user_edits/a.conf",
	} {
		if err := Reset(m.paths, path); err == nil {
			t.Errorf("Reset accepted %q", path)
		}
	}
}

// TestDoctorSeparatesEditsFromDrift checks that a user edit is reported as the
// model working, not as a problem, and that a genuinely stale file is.
func TestDoctorSeparatesEditsFromDrift(t *testing.T) {
	m := newMachine(t)
	m.baseFile("kept.conf", "base\n")
	m.baseFile("changed.conf", "base\n")
	m.baseFile("missing.conf", "base\n")
	m.baseFile("elsewhere.conf", "base\n")
	m.edit("kept.conf", "mine\n")

	if err := Materialize(m.paths); err != nil {
		t.Fatal(err)
	}
	// Someone changed the materialized copy behind the model's back.
	m.write(filepath.Join(m.paths.Config, "changed.conf"), "tampered\n")
	// Someone deleted a file the base still ships.
	if err := os.Remove(filepath.Join(m.paths.Config, "missing.conf")); err != nil {
		t.Fatal(err)
	}
	// A file the base does not ship, left behind by a downgrade. This one is
	// never in the base at all, so it cannot be "missing"; it is an orphan.
	m.write(filepath.Join(m.paths.Config, "deleted.conf"), "leftover\n")

	findings, err := Doctor(m.paths)
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}

	byPath := map[string]string{}
	for _, finding := range findings {
		byPath[finding.Path] = finding.Reason
	}

	want := map[string]string{
		"kept.conf":    DriftUserEdit,
		"changed.conf": DriftStale,
		"missing.conf": DriftMissing,
		"deleted.conf": DriftOrphan,
	}
	// A file the base ships and the config has, matching, is not drift and must
	// not appear at all.
	if _, reported := byPath["elsewhere.conf"]; reported {
		t.Errorf("elsewhere.conf is reported as %q; a matching file is not drift",
			byPath["elsewhere.conf"])
	}
	for path, reason := range want {
		if byPath[path] != reason {
			t.Errorf("%s reported as %q, want %q", path, byPath[path], reason)
		}
	}
}

// TestDoctorNeverWrites is the property that keeps a diagnostic trustworthy.
func TestDoctorNeverWrites(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base\n")
	if err := Materialize(m.paths); err != nil {
		t.Fatal(err)
	}

	before := m.read(filepath.Join(m.paths.Config, "a.conf"))
	if _, err := Doctor(m.paths); err != nil {
		t.Fatal(err)
	}
	if after := m.read(filepath.Join(m.paths.Config, "a.conf")); after != before {
		t.Errorf("doctor changed a file: %q -> %q", before, after)
	}
}

// TestMaterializePrunesOrphansPreservingUserConfig verifies that Materialize removes
// stale orphan files while preserving user config and edits.
func TestMaterializePrunesOrphansPreservingUserConfig(t *testing.T) {
	m := newMachine(t)
	m.baseFile("shipped.conf", "base\n")
	m.edit("shipped.conf", "customized\n")

	// Write user config files and state
	m.write(filepath.Join(m.paths.Config, "shell.json"), "{\"theme\":\"dark\"}\n")
	m.write(filepath.Join(m.paths.Config, ".git", "HEAD"), "ref: refs/heads/main\n")
	m.write(filepath.Join(m.paths.Config, "orphan.conf"), "leftover\n")

	// Doctor should see orphan.conf as DriftOrphan, but NOT shell.json or .git
	findings, err := Doctor(m.paths)
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	orphans := 0
	for _, f := range findings {
		if f.Reason == DriftOrphan {
			orphans++
			if f.Path != "orphan.conf" {
				t.Errorf("unexpected orphan reported: %s", f.Path)
			}
		}
	}
	if orphans != 1 {
		t.Errorf("expected 1 orphan, got %d", orphans)
	}

	// Materialize should prune orphan.conf
	if err := Materialize(m.paths); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	if _, err := os.Stat(filepath.Join(m.paths.Config, "orphan.conf")); !os.IsNotExist(err) {
		t.Errorf("orphan.conf should have been pruned by Materialize")
	}

	// User edit and user config must be preserved
	if got := m.read(filepath.Join(m.paths.Config, "shipped.conf")); got != "customized\n" {
		t.Errorf("user edit clobbered: %s", got)
	}
	if got := m.read(filepath.Join(m.paths.Config, "shell.json")); got != "{\"theme\":\"dark\"}\n" {
		t.Errorf("user config clobbered: %s", got)
	}
	if got := m.read(filepath.Join(m.paths.Config, ".git", "HEAD")); got != "ref: refs/heads/main\n" {
		t.Errorf(".git clobbered: %s", got)
	}
}
