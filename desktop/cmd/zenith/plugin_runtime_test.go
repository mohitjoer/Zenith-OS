package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPluginRoundTrip is the one check of how the CLI and the shell's runtime
// fit: it installs a scaffold, turns it on, runs the real discover.sh and
// zenith-plugins-place, and takes the plugin off again. It needs bash, jq and
// flock, which the placement scripts need too, and skips without them.
func TestPluginRoundTrip(t *testing.T) {
	for _, tool := range []string{"bash", "jq", "flock"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	shell := filepath.Join("..", "..", "..", "shell")
	discover := filepath.Join(shell, "plugins", "discover.sh")
	place := filepath.Join(shell, "scripts", "zenith-plugins-place")
	for _, script := range []string{discover, place} {
		if !pluginFileThere(script) {
			t.Skipf("%s is not in this checkout", script)
		}
	}
	data, _ := pluginTestEnv(t)
	t.Setenv("ZENITH_PLUGINS_DIR", "")
	t.Setenv("ZENITH_PLUGINS_PLACE", place)

	source := filepath.Join(t.TempDir(), "demo")
	if err := pluginScaffold(source, "demo", "Demo", "QA <qa@example.com>", "topbarGlyph", true); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		var out, errOut bytes.Buffer
		if code := pluginDispatch(args, pluginIO{out: &out, err: &errOut}); code != exitOK {
			t.Fatalf("zenith plugin %v: exit %d: %s%s", args, code, out.String(), errOut.String())
		}
		return out.String()
	}
	discovered := func(flag ...string) []map[string]any {
		t.Helper()
		out, err := exec.Command("bash", append([]string{discover}, flag...)...).Output()
		if err != nil {
			t.Fatalf("discover.sh: %v", err)
		}
		var list []map[string]any
		if err := json.Unmarshal(out, &list); err != nil {
			t.Fatalf("discover.sh printed %q: %v", out, err)
		}
		return list
	}

	run("install", source, "--bar", "--yes")
	if !pluginFileThere(filepath.Join(data, "demo", "manifest.json")) {
		t.Fatal("install left no manifest in the plugin root")
	}
	list := discovered()
	if len(list) != 1 || list[0]["id"] != "demo" || list[0]["dir"] != filepath.Join(data, "demo") {
		t.Fatalf("an enabled plugin must be discovered: %v", list)
	}
	placement, _ := list[0]["placement"].(map[string]any)
	settings, _ := placement["settings"].(map[string]any)
	if placement["host"] != "topbarGlyph" || settings["showCount"] != true {
		t.Fatalf("placement = %v, want the bar host and the manifest's default setting", placement)
	}

	if out := run("list", "--json"); !strings.Contains(out, `"enabled":true`) {
		t.Fatalf("list --json = %s", out)
	}
	run("disable", "demo")
	if list := discovered(); len(list) != 0 {
		t.Fatalf("a disabled plugin must not be discovered: %v", list)
	}
	if list := discovered("--all"); len(list) != 1 {
		t.Fatalf("--all lists a disabled plugin: %v", list)
	}
	run("enable", "demo", "--bar")
	if list := discovered(); len(list) != 1 {
		t.Fatalf("an enabled plugin must be discovered again: %v", list)
	}

	var out, errOut bytes.Buffer
	if code := pluginDispatch([]string{"install", source, "--yes"}, pluginIO{out: &out, err: &errOut}); code == exitOK {
		t.Fatal("installing the same id twice must fail")
	}

	run("remove", "demo")
	if pluginFileThere(filepath.Join(data, "demo")) {
		t.Fatal("remove left the plugin folder")
	}
	if list := discovered("--all"); len(list) != 0 {
		t.Fatalf("a removed plugin must not be discovered: %v", list)
	}
	body, _ := os.ReadFile(pluginConfigFile())
	if strings.Contains(string(body), "demo") {
		t.Fatalf("remove left its placement behind: %s", body)
	}
}

// TestPluginRemoveKeepsALinkedCheckout: removing a dev plugin that is a link
// into a checkout removes the link and leaves the checkout.
func TestPluginRemoveKeepsALinkedCheckout(t *testing.T) {
	data, _ := pluginTestEnv(t)
	t.Setenv("ZENITH_PLUGINS_PLACE", "/bin/true")
	checkout := t.TempDir()
	pluginTestPutManifest(t, checkout, `{"id":"linked","name":"Linked","version":"1.0.0"}`)
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(checkout, filepath.Join(data, "linked")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
	var out, errOut bytes.Buffer
	if code := pluginDispatch([]string{"remove", "linked"}, pluginIO{out: &out, err: &errOut}); code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if pluginFileThere(filepath.Join(data, "linked")) {
		t.Error("the link is still there")
	}
	if !pluginFileThere(filepath.Join(checkout, "manifest.json")) {
		t.Error("remove followed the link and deleted the checkout")
	}
}
