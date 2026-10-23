package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The shell's QML and this daemon are two halves of one protocol, written in
// different languages and loaded at different times, so nothing but a test
// notices when one half asks for something the other never learned. This walks
// the real shell tree beside the daemon and checks the three things QML names
// literally: the topics it subscribes to, the control calls it makes, and the
// zenith-shell verbs it spawns.

const shellTree = "../../../shell"

func qmlSources(t *testing.T) map[string]string {
	t.Helper()
	if _, err := os.Stat(shellTree); err != nil {
		t.Skip("no shell tree beside the daemon")
	}
	out := map[string]string{}
	_ = filepath.WalkDir(shellTree, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == "vendor" || d.Name() == "i18n") {
			return filepath.SkipDir
		}
		if ext := filepath.Ext(path); !d.IsDir() && (ext == ".qml" || ext == ".js") {
			if b, err := os.ReadFile(path); err == nil {
				out[path] = string(b)
			}
		}
		return nil
	})
	return out
}

func goSource(t *testing.T, names ...string) string {
	t.Helper()
	var sb strings.Builder
	for _, n := range names {
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(b)
	}
	return sb.String()
}

// findAll maps each captured name to the file it came from; a pattern with
// several groups contributes whichever one matched.
func findAll(re *regexp.Regexp, srcs map[string]string) map[string]string {
	found := map[string]string{}
	for path, src := range srcs {
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			for _, g := range m[1:] {
				if g != "" {
					found[g] = path
				}
			}
		}
	}
	return found
}

func TestQMLTopicsAndCallsExistInTheDaemon(t *testing.T) {
	srcs := qmlSources(t)
	files, _ := filepath.Glob("*.go")
	var prod []string
	for _, f := range files {
		if !strings.HasSuffix(f, "_test.go") {
			prod = append(prod, f)
		}
	}
	daemon := goSource(t, prod...)

	// "wallpaper" and the playlist.* calls belong to zenith-wallpaper's own socket.
	for topic, from := range findAll(regexp.MustCompile(`subscribe ([a-z]+)\\n`), srcs) {
		if topic == "wallpaper" || topic == "to" {
			continue
		}
		if !regexp.MustCompile(`register(Event)?Topic\("` + topic + `"`).MatchString(daemon) {
			t.Errorf("%s subscribes to %q, which the daemon never registers", from, topic)
		}
	}

	// Only the call syntaxes: a queued "call x.y" line, or a helper taking the
	// method name. A bare "settings.conf" or "wm.caps" is a path or a property.
	calls := regexp.MustCompile(`call ((?:clipboard|network|nightlight|powerprofiles|settings|tray|weather|music|calendar|updates|keypress|polkit|wm)\.[A-Za-z]+)|(?:_call|\.call|send)\("((?:clipboard|network|nightlight|powerprofiles|settings|tray|weather|music|calendar|updates|keypress|polkit|wm)\.[A-Za-z]+)"`)
	for call, from := range findAll(calls, srcs) {
		if strings.Contains(from, "/wallpaper-ui/") {
			continue // the picker's DaemonClient speaks to zenith-wallpaper's socket
		}
		if !strings.Contains(daemon, `registerCall("`+call+`"`) {
			t.Errorf("%s calls %q, which the daemon never registers", from, call)
		}
	}
}

func TestQMLShellVerbsAreKnown(t *testing.T) {
	srcs := qmlSources(t)
	known := goSource(t, "daemon.go", "main.go", "actions.go")
	verbs := regexp.MustCompile(`"zenith-shell",\s*"([a-z-]+)"|"zenith-shell ([a-z-]+)[ "]`)
	for path, src := range srcs {
		for _, m := range verbs.FindAllStringSubmatch(src, -1) {
			verb := m[1] + m[2]
			if !strings.Contains(known, `"`+verb) {
				t.Errorf("%s runs `zenith-shell %s`, which the daemon does not know", path, verb)
			}
		}
	}
}
