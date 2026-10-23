package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const repoRoot = "../../.."

func TestProductNameDerivations(t *testing.T) {
	if binName != "zenith-"+productName || unitName != binName+".service" || socketName != binName+".sock" {
		t.Fatalf("names do not derive from productName: %s %s %s", binName, unitName, socketName)
	}
	if displayName != "Zenith Agent" {
		t.Fatalf("displayName = %q", displayName)
	}
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	if SocketPath() != "/run/user/1000/zenith-agent.sock" {
		t.Fatalf("socket = %s", SocketPath())
	}
	t.Setenv("HOME", "/home/u")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv(envName("VAULT"), "")
	if ConfigPath() != "/home/u/.config/zenith/agent/config.json" || VaultDir() != "/home/u/.local/share/zenith/agent" {
		t.Fatalf("paths: %s %s", ConfigPath(), VaultDir())
	}
}

// A rename is the productName line plus the names a compiler cannot compute:
// these are the ones that must follow it.
func TestProductNameFilesFollow(t *testing.T) {
	if wd, _ := os.Getwd(); filepath.Base(wd) != binName {
		t.Errorf("the cmd directory is %s, want %s", filepath.Base(wd), binName)
	}
	unit, err := os.ReadFile(filepath.Join(repoRoot, "session/units", unitName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(unit), "ExecStart=/usr/bin/"+binName+" serve --if-enabled") {
		t.Errorf("%s does not exec /usr/bin/%s serve --if-enabled", unitName, binName)
	}
	for _, p := range []string{"docs/" + productName + ".md", "shell/hub/pages/" + displayNameNoSpace() + "Page.qml", "shell/" + productName + "/skills/zenith/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(repoRoot, p)); err != nil {
			t.Errorf("missing %s", p)
		}
	}
}

func displayNameNoSpace() string { return strings.ToUpper(productName[:1]) + productName[1:] }

// The product name is spelled once. A literal in code would survive a rename.
func TestNoProductLiteralOutsideProductGo(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	lit := regexp.MustCompile(`zenith-agent|Zenith Agent|ZENITH_AGENT`)
	for _, f := range files {
		if f == "product.go" || strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		for i, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if lit.MatchString(line) {
				t.Errorf("%s:%d spells the product name: %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// Every `zenith-agent` call in the shell must reach a verb and flag this binary
// implements. The check is textual on purpose: it reads the QML as written and
// the dispatch as written, so neither can drift without this failing.
func TestShellCallSitesAreImplemented(t *testing.T) {
	var src strings.Builder
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if !strings.HasSuffix(f, "_test.go") {
			b, _ := os.ReadFile(f)
			src.Write(b)
		}
	}
	main, _ := os.ReadFile("main.go")

	quoted := regexp.MustCompile(`"([^"\\]*)"`)
	calls := 0
	_ = filepath.WalkDir(filepath.Join(repoRoot, "shell"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".qml") {
			return nil
		}
		b, _ := os.ReadFile(p)
		for i, line := range strings.Split(string(b), "\n") {
			// Comments and visible strings mention the binary; only calls count.
			if strings.HasPrefix(strings.TrimSpace(line), "//") || strings.Contains(line, "I18n.tr(") || !strings.Contains(line, binName) {
				continue
			}
			var words []string
			for _, m := range quoted.FindAllStringSubmatch(line, -1) {
				words = append(words, strings.Fields(m[1])...)
			}
			at := -1
			for j, w := range words {
				if w == binName {
					at = j
				}
			}
			if at < 0 || at+1 >= len(words) {
				continue
			}
			calls++
			verb := words[at+1]
			if !strings.Contains(string(main), `case "`+verb+`"`) {
				t.Errorf("%s:%d calls %s %s, which main.go does not dispatch", p, i+1, binName, verb)
			}
			for _, w := range words[at+2:] {
				if strings.HasPrefix(w, "--") && !strings.Contains(src.String(), `"`+w+`"`) {
					t.Errorf("%s:%d passes %s %s %s, which no code parses", p, i+1, binName, verb, w)
				}
			}
		}
		return nil
	})
	if calls < 10 {
		t.Fatalf("found only %d call sites in shell/; the scan is not reading the QML", calls)
	}
}

func TestShellHasNoOldName(t *testing.T) {
	_ = filepath.WalkDir(filepath.Join(repoRoot, "shell"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".qml") {
			return nil
		}
		b, _ := os.ReadFile(p)
		if strings.Contains(string(b), "zenith-rashin") {
			t.Errorf("%s still calls zenith-rashin", p)
		}
		return nil
	})
}
