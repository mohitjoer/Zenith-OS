package main

import (
	"os"
	"path/filepath"
	"testing"
)

// skillEnv sets a hermetic HOME plus a temp skill source (the SKILLS override),
// and neutralises the dev-checkout resolution so only the override resolves. It
// returns the home dir and the resolved `zenith` skill dir.
func skillEnv(t *testing.T) (string, string) {
	t.Helper()
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(h, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(h, ".local", "state"))
	t.Setenv(envName("VAULT"), filepath.Join(h, "vault"))
	t.Setenv(envName("REPO"), "") // no dev checkout in resolution
	skills := t.TempDir()
	zenith := filepath.Join(skills, "zenith")
	if err := os.MkdirAll(zenith, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zenith, "SKILL.md"), []byte("# zenith\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envName("SKILLS"), skills)
	return h, zenith
}

func assertSkillLink(t *testing.T, link, want string) {
	t.Helper()
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("expected skill link %s: %v", link, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink", link)
	}
	dest, _ := os.Readlink(link)
	if dest != want {
		t.Fatalf("%s -> %q, want %q", link, dest, want)
	}
}

func TestWireSkillLinksApplicableLocations(t *testing.T) {
	h, zenith := skillEnv(t)
	// claude and codex present; omp absent (its link must be skipped).
	if err := os.MkdirAll(filepath.Join(h, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(h, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	n, err := WireSkill()
	if err != nil {
		t.Fatalf("WireSkill: %v", err)
	}
	if n < 3 {
		t.Fatalf("WireSkill wrote %d links; want at least 3", n)
	}
	// The always-created ~/.agents and the two present agents are linked.
	assertSkillLink(t, filepath.Join(h, ".agents", "skills", "zenith"), zenith)
	assertSkillLink(t, filepath.Join(h, ".claude", "skills", "zenith"), zenith)
	assertSkillLink(t, filepath.Join(h, ".codex", "skills", "zenith"), zenith)
	// omp absent, so no omp link was created.
	if _, err := os.Lstat(filepath.Join(h, ".omp", "agent", "skills", "zenith")); !os.IsNotExist(err) {
		t.Fatal("omp skill link created without ~/.omp")
	}
	// Idempotent: a second wire changes nothing on disk.
	if _, err := WireSkill(); err != nil {
		t.Fatalf("second WireSkill: %v", err)
	}
	assertSkillLink(t, filepath.Join(h, ".agents", "skills", "zenith"), zenith)
}

func TestWireSkillNoOpWhenSkillDirMissing(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(h, ".local", "state"))
	t.Setenv(envName("REPO"), "")
	// Override points at a dir with no SKILL.md, so nothing resolves.
	t.Setenv(envName("SKILLS"), t.TempDir())
	n, err := WireSkill()
	if err != nil || n != 0 {
		t.Fatalf("WireSkill on a box with no skill dir: n=%d err=%v", n, err)
	}
	if _, err := os.Lstat(filepath.Join(h, ".agents", "skills", "zenith")); !os.IsNotExist(err) {
		t.Fatal("WireSkill created a link with no skill source")
	}
}

func TestSymlinkForceKeepsRealDir(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "zenith")
	if err := os.MkdirAll(link, 0o755); err != nil { // a real dir sits here
		t.Fatal(err)
	}
	wrote, err := symlinkForce(link, t.TempDir())
	if err != nil {
		t.Fatalf("symlinkForce: %v", err)
	}
	if wrote {
		t.Fatal("symlinkForce reported writing over a real directory")
	}
	fi, _ := os.Lstat(link)
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Fatal("symlinkForce clobbered a real directory")
	}
}

func TestUnwireSkillRemovesOnlyOurs(t *testing.T) {
	h, zenith := skillEnv(t)
	if _, err := WireSkill(); err != nil {
		t.Fatal(err)
	}
	// A foreign symlink at a skills/zenith path must be left alone.
	foreign := filepath.Join(h, ".claude", "skills")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	if err := os.Symlink(other, filepath.Join(foreign, "zenith")); err != nil {
		t.Fatal(err)
	}
	UnwireSkill()
	// Ours is gone.
	if _, err := os.Lstat(filepath.Join(h, ".agents", "skills", "zenith")); !os.IsNotExist(err) {
		t.Fatal("UnwireSkill left our agents link")
	}
	// The foreign link survives.
	if _, err := os.Lstat(filepath.Join(foreign, "zenith")); err != nil {
		t.Fatal("UnwireSkill removed a foreign symlink")
	}
	_ = zenith
}

func TestDetectAgentsReportsSkillWired(t *testing.T) {
	h, _ := skillEnv(t)
	if err := os.MkdirAll(filepath.Join(h, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Wire("claude"); err != nil {
		t.Fatal(err)
	}
	var claude, opencode Agent
	for _, a := range DetectAgents() {
		switch a.ID {
		case "claude":
			claude = a
		case "opencode":
			opencode = a
		}
	}
	if !claude.SkillWired {
		t.Fatal("claude should report skillWired after Wire")
	}
	if opencode.SkillWired {
		t.Fatal("opencode has no skills dir and must report skillWired=false")
	}
}

// A link laid by another checkout's deploy still points at a directory carrying
// our SKILL.md (frontmatter name: zenith), so it counts as wired and unwire may
// remove it; a foreign skill dir without the marker stays foreign.
func TestSkillLinkRecognisedByMarker(t *testing.T) {
	h, _ := skillEnv(t)
	other := filepath.Join(t.TempDir(), "elsewhere", "zenith")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "SKILL.md"), []byte("---\nname: zenith\ndescription: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(h, ".agents", "skills", "zenith")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, link); err != nil {
		t.Fatal(err)
	}
	if !isOurSkillLink(link) {
		t.Fatal("a link to a dir with our SKILL.md marker must count as ours")
	}
	foreign := filepath.Join(t.TempDir(), "zenith")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "SKILL.md"), []byte("---\nname: someone-else\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, link); err != nil {
		t.Fatal(err)
	}
	if isOurSkillLink(link) {
		t.Fatal("a foreign skill without the marker must not count as ours")
	}
}
