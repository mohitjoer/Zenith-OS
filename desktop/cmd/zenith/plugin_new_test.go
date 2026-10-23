package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pluginFileThere(p string) bool { _, err := os.Stat(p); return err == nil }

// TestPluginNewScaffold checks the bar scaffold lays every required file,
// renames gitignore to .gitignore, creates assets/, ships no symlink, and
// templates the author, name and id into the docs.
func TestPluginNewScaffold(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo-x")
	if err := pluginScaffold(dir, "demo-x", "Demo X", "QA <qa@example.com>", "topbarGlyph", true); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{
		"manifest.json", "service/Main.qml", "content/Widget.qml", "content/Panel.qml",
		"README.md", "LICENSE", "AGENTS.md", "CLAUDE.md", ".gitignore",
	} {
		if !pluginFileThere(filepath.Join(dir, f)) {
			t.Errorf("scaffold missing %s", f)
		}
	}
	if pluginFileThere(filepath.Join(dir, "gitignore")) {
		t.Errorf("gitignore should be renamed to .gitignore")
	}
	if fi, err := os.Stat(filepath.Join(dir, "assets")); err != nil || !fi.IsDir() {
		t.Errorf("assets/ dir not created")
	}
	if link, _ := pluginFirstSymlink(dir); link != "" {
		t.Errorf("scaffold contains a symlink: %s", link)
	}
	lic, _ := os.ReadFile(filepath.Join(dir, "LICENSE"))
	if !strings.Contains(string(lic), "QA") {
		t.Errorf("LICENSE not templated with the author name")
	}
	readme, _ := os.ReadFile(filepath.Join(dir, "README.md"))
	if !strings.Contains(string(readme), "Demo X") || !strings.Contains(string(readme), "demo-x") {
		t.Errorf("README not templated with name/id")
	}
	claude, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if !strings.Contains(string(claude), "AGENTS.md") {
		t.Errorf("CLAUDE.md should point at AGENTS.md")
	}
}

// TestPluginNewBarManifest pins the bar manifest: a panel entry point, a panel
// width, honest official/author, and the topbarGlyph host.
func TestPluginNewBarManifest(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bary")
	if err := pluginScaffold(dir, "bary", "Bary", "QA <qa@example.com>", "topbarGlyph", true); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	body, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("manifest not valid JSON: %v", err)
	}
	entry, _ := m["entryPoints"].(map[string]any)
	if entry["panel"] != "content/Panel.qml" {
		t.Errorf("bar manifest missing panel entry point: %v", entry)
	}
	panel, ok := m["panel"].(map[string]any)
	if !ok || int(panel["width"].(float64)) != 320 {
		t.Errorf("bar manifest missing panel width 320: %v", m["panel"])
	}
	if m["official"] != false {
		t.Errorf("official must be false for a community plugin")
	}
	if m["author"] != "QA <qa@example.com>" {
		t.Errorf("author not written: %v", m["author"])
	}
	hosts, _ := m["hosts"].([]any)
	if len(hosts) != 1 || hosts[0] != "topbarGlyph" {
		t.Errorf("host wrong: %v", hosts)
	}
}

// TestPluginNewDesktopNoPanel checks the desktop host omits the panel file and
// the panel manifest fields.
func TestPluginNewDesktopNoPanel(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "desky")
	if err := pluginScaffold(dir, "desky", "Desky", "QA <qa@example.com>", "desktopWidget", false); err != nil {
		t.Fatal(err)
	}
	if pluginFileThere(filepath.Join(dir, "content/Panel.qml")) {
		t.Errorf("desktop scaffold should not ship content/Panel.qml")
	}
	var m map[string]any
	body, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	entry, _ := m["entryPoints"].(map[string]any)
	if _, ok := entry["panel"]; ok {
		t.Errorf("desktop manifest should not declare a panel entry point")
	}
	if _, ok := m["panel"]; ok {
		t.Errorf("desktop manifest should not declare a panel width")
	}
	hosts, _ := m["hosts"].([]any)
	if len(hosts) != 1 || hosts[0] != "desktopWidget" {
		t.Errorf("host wrong: %v", hosts)
	}
}

func TestPluginTitleFromID(t *testing.T) {
	for in, want := range map[string]string{"demo-x": "Demo X", "vpn": "Vpn", "a-b-c": "A B C"} {
		if got := pluginTitleFromID(in); got != want {
			t.Errorf("pluginTitleFromID(%q)=%q want %q", in, got, want)
		}
	}
}

// TestPluginNewDryRun is the `zenith plugin new demo --dry-run` check: it lists
// the files a real run writes and writes none of them.
func TestPluginNewDryRun(t *testing.T) {
	pluginTestEnv(t)
	target := filepath.Join(t.TempDir(), "demo")
	var out, errOut bytes.Buffer
	code := pluginDispatch([]string{"new", "demo", "--dry-run", "--to", target, "--author", "QA <qa@example.com>"},
		pluginIO{out: &out, err: &errOut})
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	for _, want := range []string{"manifest.json", "service/Main.qml", "content/Widget.qml", "content/Panel.qml", "would scaffold demo"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("dry run output lacks %q:\n%s", want, out.String())
		}
	}
	if pluginFileThere(target) {
		t.Errorf("--dry-run wrote %s", target)
	}
}

// TestPluginNewRefusesReservedID: a built-in widget id is not a plugin id.
func TestPluginNewRefusesReservedID(t *testing.T) {
	pluginTestEnv(t)
	var out, errOut bytes.Buffer
	code := pluginDispatch([]string{"new", "clock", "--dry-run"}, pluginIO{out: &out, err: &errOut})
	if code != exitUsage || !strings.Contains(errOut.String(), "reserved") {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
}
