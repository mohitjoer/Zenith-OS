package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// defaultsFakeMachine answers for gsettings and update-alternatives and records what
// was asked of them, so the test never touches the real settings or the real system.
// GHOSTTY in the canned alternatives output stands for the fake binary's path.
func defaultsFakeMachine(t *testing.T, alternative string) *[]string {
	t.Helper()
	oldExec, oldBin := defaultsExec, defaultsGhostty
	t.Cleanup(func() { defaultsExec, defaultsGhostty = oldExec, oldBin })

	bin := filepath.Join(t.TempDir(), "ghostty")
	if err := os.WriteFile(bin, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	defaultsGhostty = bin

	var calls []string
	defaultsExec = func(name string, args ...string) (string, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		switch {
		case name == "gsettings" && args[0] == "get" && args[2] == "exec":
			return "'xdg-terminal-exec'", nil
		case name == "gsettings" && args[0] == "get":
			return "'--'", nil
		case name == "update-alternatives":
			return strings.ReplaceAll(alternative, "GHOSTTY", defaultsGhostty), nil
		}
		return "", nil
	}
	return &calls
}

func TestAppsDefaultsAppliesWhatTheUserOwnsAndOnlyPrintsTheRest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("ZDOTDIR", "")
	// Ptyxis is the current choice and Ghostty is registered, so only --set is needed.
	calls := defaultsFakeMachine(t, "Value: /usr/bin/ptyxis\n\nAlternative: GHOSTTY\nPriority: 50")

	// "zenith apps defaults" must reach this command through the built-in "apps".
	if code := run([]string{"apps", "defaults", "--check"}); code != exitOK {
		t.Fatalf("zenith apps defaults --check exited %d", code)
	}
	var check bytes.Buffer
	runAppsDefaults(&check, true)
	list := filepath.Join(home, ".config", "zenith-xdg-terminals.list")
	if _, err := os.Stat(list); err == nil {
		t.Error("--check wrote the terminal list")
	}
	for _, call := range *calls {
		if strings.HasPrefix(call, "gsettings set") {
			t.Errorf("--check ran %q", call)
		}
	}

	*calls = nil
	var out bytes.Buffer
	if code := runAppsDefaults(&out, false); code != exitOK {
		t.Fatalf("exit %d:\n%s", code, out.String())
	}
	joined := strings.Join(*calls, "\n")
	for _, want := range []string{
		"gsettings set org.gnome.desktop.default-applications.terminal exec 'ghostty'",
		"gsettings set org.gnome.desktop.default-applications.terminal exec-arg '-e'",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing call %q in\n%s", want, joined)
		}
	}
	for _, call := range *calls {
		if strings.HasPrefix(call, "update-alternatives") && !strings.Contains(call, "--query") {
			t.Errorf("ran root command %q", call)
		}
	}
	if body, _ := os.ReadFile(list); string(body) != defaultsDesktopID+"\n" {
		t.Errorf("terminal list is %q", body)
	}
	shown := out.String()
	for _, want := range []string{
		"sudo update-alternatives --set x-terminal-emulator " + defaultsGhostty,
		"config-file = ?~/.config/zenith/apps/ghostty/config",
		`. "$HOME/.config/zenith/apps/zsh/zenith.zsh"`,
	} {
		if !strings.Contains(shown, want) {
			t.Errorf("output lacks %q:\n%s", want, shown)
		}
	}
	if strings.Contains(shown, "--install") {
		t.Errorf("asked to register an alternative that already exists:\n%s", shown)
	}

	// A user who followed the printed lines is not asked again.
	write := func(path, body string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(home, ".config", "ghostty", "config"), "config-file = ?~/.config/zenith/apps/ghostty/config\n")
	write(filepath.Join(home, ".zshrc"), `. "$HOME/.config/zenith/apps/zsh/zenith.zsh"`+"\n")
	out.Reset()
	runAppsDefaults(&out, false)
	if strings.Contains(out.String(), "add this line") {
		t.Errorf("asked again for lines that are there:\n%s", out.String())
	}
}

// Strict monochrome: every colour in what this lane ships is a grey, or one of the
// design system's error and warning exceptions (shell/Theme.qml), and the text
// colours clear the 4.5:1 floor on the terminal background.
func TestShippedAppsStayMonochrome(t *testing.T) {
	exceptions := map[string]bool{"E5484D": true, "F98080": true, "F5A524": true}
	hex := regexp.MustCompile(`#([0-9A-Fa-f]{6})\b`)
	luminance := func(h string) float64 {
		var l float64
		for i, w := range []float64{0.2126, 0.7152, 0.0722} {
			v, _ := strconv.ParseUint(h[2*i:2*i+2], 16, 8)
			c := float64(v) / 255
			if c <= 0.04045 {
				c /= 12.92
			} else {
				c = math.Pow((c+0.055)/1.055, 2.4)
			}
			l += w * c
		}
		return l
	}
	bg := luminance("101010")

	for _, file := range []string{"apps/ghostty/colors", "apps/starship/starship.toml", "apps/zsh/zenith.zsh"} {
		body, err := os.ReadFile(filepath.Join(repoRoot(t), file))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range hex.FindAllStringSubmatch(string(body), -1) {
			h := strings.ToUpper(m[1])
			if h[0:2] != h[2:4] || h[2:4] != h[4:6] {
				if !exceptions[h] {
					t.Errorf("%s: #%s is neither grey nor an allowed exception", file, h)
				}
			}
		}
	}

	// Text slots of the terminal palette (0 is the ANSI black: a fill, not text).
	body, _ := os.ReadFile(filepath.Join(repoRoot(t), "apps/ghostty/colors"))
	for _, m := range regexp.MustCompile(`palette = (\d+)=#([0-9A-F]{6})`).FindAllStringSubmatch(string(body), -1) {
		if m[1] == "0" {
			continue
		}
		if ratio := (luminance(m[2]) + 0.05) / (bg + 0.05); ratio < 4.5 {
			t.Errorf("palette %s #%s has contrast %.2f:1 on the background, want 4.5", m[1], m[2], ratio)
		}
	}
}
