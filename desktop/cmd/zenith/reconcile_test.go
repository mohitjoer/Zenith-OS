package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReconcileShellConfig(t *testing.T) {
	const retired = `{"barStyle":"qsbar","fontScale":1.30,"frameBars":{"style":"ryoku-frame","rails":{"left":{"size":48}}}}`

	write := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "shell.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("a dry run reports and changes nothing", func(t *testing.T) {
		path := write(t, retired)
		msg, err := reconcileShellConfig(path, false)
		if err != nil || !strings.Contains(msg, "zenith doctor --fix") {
			t.Fatalf("got %q, %v", msg, err)
		}
		if got, _ := os.ReadFile(path); string(got) != retired {
			t.Errorf("a dry run rewrote the file: %s", got)
		}
	})

	t.Run("the rename keeps every other key and the file mode, and is idempotent", func(t *testing.T) {
		path := write(t, retired)
		if _, err := reconcileShellConfig(path, true); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		for _, want := range []string{`"style": "zenith-frame"`, `"fontScale": 1.30`, `"barStyle": "qsbar"`, `"size": 48`} {
			if !strings.Contains(string(got), want) {
				t.Errorf("result lacks %s:\n%s", want, got)
			}
		}
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
			t.Errorf("mode is %v, want 0600", info.Mode().Perm())
		}
		if msg, err := reconcileShellConfig(path, true); msg != "" || err != nil {
			t.Errorf("a second run reported %q, %v", msg, err)
		}
	})

	t.Run("nothing to do", func(t *testing.T) {
		for _, body := range []string{
			`{"frameBars":{"style":"zenith-frame"}}`,
			`{"frameBars":{"style":"slate-frame"}}`,
			`{"frameBars":{}}`,
			`{"barStyle":"qsbar"}`,
		} {
			if msg, err := reconcileShellConfig(write(t, body), true); msg != "" || err != nil {
				t.Errorf("%s: reported %q, %v", body, msg, err)
			}
		}
		if msg, err := reconcileShellConfig(filepath.Join(t.TempDir(), "absent.json"), true); msg != "" || err != nil {
			t.Errorf("no file: reported %q, %v", msg, err)
		}
	})

	t.Run("a file that does not parse is left alone", func(t *testing.T) {
		path := write(t, `{"frameBars": `)
		if _, err := reconcileShellConfig(path, true); err == nil {
			t.Error("expected an error")
		}
		if got, _ := os.ReadFile(path); string(got) != `{"frameBars": ` {
			t.Errorf("the file was touched: %s", got)
		}
	})
}
