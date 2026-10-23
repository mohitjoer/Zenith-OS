package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pluginTestAudit lays files (and an optional manifest.json) into a fresh temp
// dir and runs the static audit over it.
func pluginTestAudit(t *testing.T, files map[string]string, manifest string) pluginAuditResult {
	t.Helper()
	dir := t.TempDir()
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for p, c := range files {
		fp := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fp, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return pluginAudit(dir, nil)
}

func pluginHasRule(res pluginAuditResult, rule string) bool {
	for _, f := range append(append([]pluginFinding{}, res.Blocking...), res.Warnings...) {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func pluginRuleBlocks(res pluginAuditResult, rule string) bool {
	for _, f := range res.Blocking {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

// TestPluginAuditRules is the table-driven audit test: one positive fixture (the
// rule fires) and one negative fixture (it does not) for every text-based rule.
func TestPluginAuditRules(t *testing.T) {
	const pkexecCmd = `Item { property var c: 0
  function act() { const p = Process; command: ["pkexec", "systemctl", "poweroff"] }
}`
	const pkexecManifest = `{"capabilities":{"privileged":["pkexec systemctl poweroff"]}}`
	const netManifest = `{"capabilities":{"network":["evil.example.com"]}}`

	cases := []struct {
		name     string
		rule     string
		blocking bool
		files    map[string]string
		manifest string
		want     bool
	}{
		{"escalation-sudo-pos", "escalation", true, map[string]string{"bin/x.sh": "#!/bin/sh\nsudo reboot\n"}, "", true},
		{"escalation-sudo-neg", "escalation", true, map[string]string{"bin/x.sh": "#!/bin/sh\necho hi\n"}, "", false},

		{"escalation-pkexec-pos", "escalation", true, map[string]string{"content/x.qml": pkexecCmd}, "", true},
		{"escalation-pkexec-neg", "escalation", true, map[string]string{"content/x.qml": pkexecCmd}, pkexecManifest, false},

		{"pipe-shell-pos", "pipe-shell", true, map[string]string{"bin/i.sh": "#!/bin/sh\ncurl https://x/i.sh | sh\n"}, "", true},
		{"pipe-shell-neg", "pipe-shell", true, map[string]string{"bin/i.sh": "#!/bin/sh\ncurl example > /tmp/x\n"}, "", false},

		{"internal-import-pos", "internal-import", true, map[string]string{"content/W.qml": "import shell.services\nItem {}\n"}, "", true},
		{"internal-import-qs-pos", "internal-import", true, map[string]string{"content/W.qml": "import qs.Services\nItem {}\n"}, "", true},
		{"internal-import-kit-neg", "internal-import", true, map[string]string{"content/W.qml": "import QtQuick\nimport qs.PluginKit\nimport qs.PluginKit.Singletons\nItem {}\n"}, "", false},
		{"internal-import-rel-pos", "internal-import", true, map[string]string{"content/W.qml": "import \"../../x\"\nItem {}\n"}, "", true},

		{"config-write-pos", "config-write", true, map[string]string{"content/W.qml": "property string p: \"shell.json\"\n"}, "", true},
		{"config-write-neg", "config-write", true, map[string]string{"content/W.qml": "property string p: \"data.json\"\n"}, "", false},

		{"secret-pos", "secret", true, map[string]string{"notes.txt": "token = ghp-abcdefghij0123456789\n"}, "", true},
		{"secret-neg", "secret", true, map[string]string{"notes.txt": "token = none\n"}, "", false},

		{"binary-pos", "binary", true, map[string]string{"bin/tool": "\x7fELF\x02\x01\x01\x00\x00\x00"}, "", true},
		{"binary-neg", "binary", true, map[string]string{"bin/tool": "#!/bin/sh\necho hi\n"}, "", false},

		{"undeclared-command-pos", "undeclared-command", false, map[string]string{"content/W.qml": "command: [\"foobar\", \"x\"]\n"}, "", true},
		{"undeclared-command-neg", "undeclared-command", false, map[string]string{"content/W.qml": "command: [\"nmcli\", \"x\"]\n"}, "", false},

		{"undeclared-host-pos", "undeclared-host", false, map[string]string{"content/W.qml": "x: \"https://evil.example.com/y\"\n"}, "", true},
		{"undeclared-host-neg", "undeclared-host", false, map[string]string{"content/W.qml": "x: \"https://evil.example.com/y\"\n"}, netManifest, false},

		{"dynamic-shell-pos", "dynamic-shell", false, map[string]string{"content/W.qml": "command: [\"sh\", \"-c\", \"echo \" + userInput]\n"}, "", true},
		{"dynamic-shell-neg", "dynamic-shell", false, map[string]string{"content/W.qml": "command: [\"sh\", \"-c\", \"echo hi\"]\n"}, "", false},

		{"outside-write-pos", "outside-write", false, map[string]string{"bin/x.sh": "#!/bin/sh\necho hi > ~/.config/x\n"}, "", true},
		{"outside-write-neg", "outside-write", false, map[string]string{"bin/x.sh": "#!/bin/sh\necho hi > \"$stateDir/x\"\n"}, "", false},

		{"removed-name-pos", "removed-name", false, map[string]string{"content/W.qml": "command: [\"ryoku-shell\", \"x\"]\n"}, "", true},
		{"removed-name-neg", "removed-name", false, map[string]string{"content/W.qml": "command: [\"zenith-shell\", \"x\"]\n"}, "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := pluginTestAudit(t, c.files, c.manifest)
			if got := pluginHasRule(res, c.rule); got != c.want {
				t.Fatalf("rule %q present=%v want %v (blocking=%v warnings=%v)", c.rule, got, c.want, res.Blocking, res.Warnings)
			}
			if c.want && c.blocking && !pluginRuleBlocks(res, c.rule) {
				t.Fatalf("rule %q should be a blocking finding", c.rule)
			}
			if c.want && !c.blocking && pluginRuleBlocks(res, c.rule) {
				t.Fatalf("rule %q should be a warning, not blocking", c.rule)
			}
		})
	}
}

// TestPluginAuditNamesTheKitRename pins the porting message: a plugin that
// imports Ryoku.PluginKit is told the import to use instead.
func TestPluginAuditNamesTheKitRename(t *testing.T) {
	res := pluginTestAudit(t, map[string]string{
		"content/W.qml": "import Ryoku.PluginKit\nimport Ryoku.PluginKit.Singletons\nItem {}\n",
	}, "")
	if len(res.Blocking) != 2 {
		t.Fatalf("want 2 blocking findings, got %+v", res)
	}
	for i, want := range []string{"import qs.PluginKit`", "import qs.PluginKit.Singletons`"} {
		if !strings.Contains(res.Blocking[i].Message, want) {
			t.Errorf("finding %d = %q, want it to say %q", i, res.Blocking[i].Message, want)
		}
	}
}

// TestPluginAuditSymlink flags a symlink anywhere in the tree as a blocking finding.
func TestPluginAuditSymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "real.qml"), []byte("import QtQuick\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.qml", filepath.Join(dir, "link.qml")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
	if res := pluginAudit(dir, nil); !pluginRuleBlocks(res, "symlink") {
		t.Fatalf("symlink not reported as blocking: %+v", res)
	}
}

// TestPluginAuditLargeTree warns when the tree exceeds 20 MB.
func TestPluginAuditLargeTree(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "big.dat"), bytes.Repeat([]byte("a"), 21*1024*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	res := pluginAudit(dir, nil)
	if !pluginHasRule(res, "large-tree") || pluginRuleBlocks(res, "large-tree") {
		t.Fatalf("large-tree should be reported as a warning: %+v", res)
	}
}

// TestPluginAuditAllowDowngrade checks --allow moves a blocking rule to warnings.
func TestPluginAuditAllowDowngrade(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.sh"), []byte("#!/bin/sh\nsudo reboot\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := pluginAudit(dir, map[string]bool{"escalation": true})
	if pluginRuleBlocks(res, "escalation") || !pluginHasRule(res, "escalation") {
		t.Fatalf("escalation should be downgraded to a warning: %+v", res)
	}
}

// TestPluginTemplateAuditsClean is the requirement that the scaffold passes the
// audit with zero findings.
func TestPluginTemplateAuditsClean(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo-x")
	if err := pluginScaffold(dir, "demo-x", "Demo X", "QA <qa@example.com>", "topbarGlyph", true); err != nil {
		t.Fatal(err)
	}
	res := pluginAudit(dir, nil)
	if len(res.Blocking) != 0 || len(res.Warnings) != 0 {
		t.Fatalf("template not clean: blocking=%+v warnings=%+v", res.Blocking, res.Warnings)
	}
	if _, err := pluginValidateManifest(dir, pluginReservedIDs()); err != nil {
		t.Fatalf("template manifest rejected: %v", err)
	}
}
