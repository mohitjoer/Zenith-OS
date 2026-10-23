package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The compositor's config is split across files the entry sources. The doctor
// asks whether the session starts and whether the binds reach the shell, and
// both facts now live beside the entry file, not in it.
func TestInstalledCompositorConfigReadsTheSplitConfig(t *testing.T) {
	const entry = "/usr/share/zenith/hyprland/hyprland.conf"
	root := t.TempDir()
	dir := filepath.Join(root, filepath.Dir(entry))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"hyprland.conf":  "source = autostart.conf\nsource = binds.conf\n",
		"autostart.conf": "exec-once = /usr/share/zenith/session/zenith-activate\n",
		"binds.conf":     "bind = SUPER, Space, exec, zenith ipc call zenith togglePopover launcher\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := installedCompositorConfig(root, entry)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"exec-once = /usr/share/zenith/session/zenith-activate", "zenith ipc call"} {
		if !strings.Contains(got, want) {
			t.Errorf("the split config lacks %q:\n%s", want, got)
		}
	}

	if _, err := installedCompositorConfig(t.TempDir(), entry); err == nil {
		t.Error("a machine with no compositor config should be an error")
	}
}
