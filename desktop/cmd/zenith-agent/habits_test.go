package main

import (
	"os"
	"path/filepath"
	"testing"
)

func tokSet(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

// TestCommandKey pins the history-line reduction: argv0 base, with a bareword
// subcommand for the tools where that is the story, sudo stripped, env
// assignments stripped. Flag-style operations (pacman -Syu) collapse to argv0
// by design -- only bareword subcommands are captured.
func TestCommandKey(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"git push origin", "git push"},
		{"ls -la", "ls"},
		// sudo stripped; pacman's op is a flag, so it collapses to argv0.
		{"sudo pacman -Syu", "pacman"},
		{"FOO=1 make build", "make build"},
		// sudo stripped AND a bareword subcommand kept together.
		{"sudo systemctl restart nginx", "systemctl restart"},
		{"cargo build --release", "cargo build"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := commandKey(tc.in); got != tc.want {
				t.Errorf("commandKey(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestJaccard checks the similarity metric at its boundaries and interior.
func TestJaccard(t *testing.T) {
	cases := []struct {
		name string
		a, b map[string]bool
		want float64
	}{
		{"identical", tokSet("list", "files"), tokSet("list", "files"), 1.0},
		{"disjoint", tokSet("a", "b"), tokSet("c", "d"), 0.0},
		{"half overlap", tokSet("a", "b"), tokSet("a", "b", "c", "d"), 0.5},
		{"one third", tokSet("a", "b"), tokSet("b", "c"), 1.0 / 3.0},
		{"empty operand", tokSet(), tokSet("a"), 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jaccard(tc.a, tc.b); got != tc.want {
				t.Errorf("jaccard = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestAskTokens: words are lowercased and stopwords dropped.
func TestAskTokens(t *testing.T) {
	// "How", "to", "ALL" are stopwords; "Files" must survive lowercased.
	toks := askTokens("How to list ALL Files")

	if len(toks) != 2 {
		t.Fatalf("askTokens = %v, want exactly {list, files}", toks)
	}
	if !toks["list"] || !toks["files"] {
		t.Errorf("askTokens dropped a content word: %v", toks)
	}
	for _, stop := range []string{"how", "to", "all"} {
		if toks[stop] {
			t.Errorf("askTokens kept stopword %q: %v", stop, toks)
		}
	}
	// Lowercasing: the capitalized input must not leak through.
	if toks["Files"] || toks["ALL"] {
		t.Errorf("askTokens did not lowercase: %v", toks)
	}
}

// TestXdgUserDirs parses user-dirs.dirs from XDG_CONFIG_HOME: names are
// title-cased, $HOME becomes ~, and a dir pointing at $HOME itself is skipped.
func TestXdgUserDirs(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	content := "# written by xdg-user-dirs-update\n" +
		`XDG_PICTURES_DIR="$HOME/Pictures"` + "\n" +
		`XDG_DOCUMENTS_DIR="$HOME/Dokumente"` + "\n" +
		`XDG_DOWNLOAD_DIR="$HOME/"` + "\n" + // points at $HOME: skipped
		`XDG_DESKTOP_DIR="$HOME"` + "\n" // points at $HOME: skipped
	if err := os.WriteFile(filepath.Join(cfg, "user-dirs.dirs"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	dirs := xdgUserDirs()
	got := map[string]string{}
	for _, d := range dirs {
		got[d.name] = d.path
	}
	if len(got) != 2 {
		t.Fatalf("xdgUserDirs = %+v, want 2 entries (home-pointing dirs skipped)", dirs)
	}
	if got["Pictures"] != "~/Pictures" {
		t.Errorf("Pictures = %q, want ~/Pictures", got["Pictures"])
	}
	// Localized name, still keyed by the canonical XDG name.
	if got["Documents"] != "~/Dokumente" {
		t.Errorf("Documents = %q, want ~/Dokumente", got["Documents"])
	}
}

// TestHistoryCounts parses `- cmd:` lines, filters secret-ish lines before
// counting, and aggregates by commandKey.
func TestHistoryCounts(t *testing.T) {
	dir := t.TempDir()
	hist := filepath.Join(dir, "fish_history")
	content := "" +
		"- cmd: git status\n  when: 1700000000\n" +
		"- cmd: git status\n  when: 1700000001\n" +
		"- cmd: eza -la\n  when: 1700000002\n" +
		"- cmd: cargo build --release\n  when: 1700000003\n" +
		"- cmd: mysql -u root -ppassword mydb\n  when: 1700000004\n"
	if err := os.WriteFile(hist, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	counts := historyCounts(hist, 1<<20)

	if counts["git status"] != 2 {
		t.Errorf("git status = %d, want 2", counts["git status"])
	}
	if counts["eza"] != 1 {
		t.Errorf("eza = %d, want 1", counts["eza"])
	}
	if counts["cargo build"] != 1 {
		t.Errorf("cargo build = %d, want 1", counts["cargo build"])
	}
	// The password line is dropped before counting, so its argv0 never lands.
	if _, ok := counts["mysql"]; ok {
		t.Errorf("secret line was counted: %v", counts)
	}
	if len(counts) != 3 {
		t.Errorf("counts = %v, want exactly 3 keys", counts)
	}

	// zsh, the default shell here, writes `: <time>:<elapsed>;<command>`.
	zsh := filepath.Join(dir, "zsh_history")
	if err := os.WriteFile(zsh, []byte(": 1700000000:0;git status\n: 1700000001:2;git status\n: 1700000002:0;ls\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if zc := historyCounts(zsh, 1<<20); zc["git status"] != 2 || zc["ls"] != 1 {
		t.Errorf("zsh counts = %v", zc)
	}
}

// TestEditorName pins the $EDITOR reduction: filepath.Base of the value, except
// the no-op sentinels ("" -> ".", "true", "false") fall back to the Zenith
// default. "true"/"false" surfaced during live testing (automation and editor
// suppression set EDITOR to them, and they are never real editors).
func TestEditorName(t *testing.T) {
	const dflt = "nvim (Zenith default)"
	cases := []struct {
		name   string
		editor string
		want   string
	}{
		{"true sentinel falls back", "true", dflt},
		{"false sentinel falls back", "false", dflt},
		{"empty falls back", "", dflt},
		{"absolute path reduced to base", "/usr/bin/nvim", "nvim"},
		{"bare name kept", "hx", "hx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EDITOR", tc.editor)
			if got := editorName(); got != tc.want {
				t.Errorf("editorName() with EDITOR=%q = %q, want %q", tc.editor, got, tc.want)
			}
		})
	}
}
