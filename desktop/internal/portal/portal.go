// Package portal resolves the portal backend configuration for a session.
//
// This exists because of one specific silent failure. `xdg-desktop-portal`
// derives the name of its per-desktop configuration file from
// `XDG_CURRENT_DESKTOP`, lower-cased. A session that does not declare itself
// correctly therefore finds no `zenith-portals.conf`, falls back to the system
// `portals.conf`, and quietly uses whatever that names — which on a machine
// adopted from GNOME means `use=gnome;` and a dispatcher trying to activate a
// backend that is not installed.
//
// Nothing is printed. No error, no warning, no fallback. The desktop comes up and
// the file chooser is wrong, and the user reports it as "Zenith's file chooser is
// broken" rather than as a session identity problem. `zenith doctor` asserts the
// configuration is actually being read, and that assertion is the whole reason
// this package exists.
package portal

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is a parsed portals.conf.
//
// The format is INI with one key per line in a section. It is parsed here rather
// than through a library because the file is four lines of `use=` per section and
// because the one thing worth being strict about — which section a key is in — is
// the thing a lenient parser gets wrong.
type Config struct {
	// Sections maps a section name to its key/value pairs.
	Sections map[string]map[string]string
}

// Backend returns the backend named in `use=` for a section, and whether the
// section said anything.
func (c Config) Backend(section string) (string, bool) {
	values, present := c.Sections[section]
	if !present {
		return "", false
	}
	use, present := values["use"]
	if !present {
		return "", false
	}
	// `use=hyprland;` is a single backend in a list. A trailing semicolon is
	// syntax, not part of the name — and the trim has to happen after the
	// semicolon is removed as well as before, because "use = hyprland ;" leaves
	// a space that would end up inside the name and match nothing.
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(use), ";")), true
}

// ConfigFile is the per-desktop configuration file name for a desktop identity.
//
// `xdg-desktop-portal` lower-cases XDG_CURRENT_DESKTOP and appends "-portals",
// so XDG_CURRENT_DESKTOP=Zenith reads "zenith-portals.conf". The desktop's
// capitalisation is therefore load-bearing: a session that exports
// XDG_CURRENT_DESKTOP=zenith and XDG_SESSION_DESKTOP=Zenith backwards — which
// reads as a plausible mistake, because the two are near-synonyms — resolves a
// different file name and silently gets the wrong configuration.
func ConfigFile(currentDesktop string) string {
	return strings.ToLower(strings.TrimSpace(currentDesktop)) + "-portals.conf"
}

// ExpectedDesktop is the session identity the session script sets.
//
// It is exported so `zenith doctor` can compare it against what a machine is
// actually running, which is the assertion the architecture record requires.
const ExpectedDesktop = "Zenith"

// Resolution is what the dispatcher will actually do on this machine.
type Resolution struct {
	// CurrentDesktop is the session identity the resolution was made against,
	// carried so a report can show what it saw rather than what it expected.
	CurrentDesktop string

	// ConfigFile is the name the dispatcher will look for.
	ConfigFile string

	// Source is where the configuration came from: the per-desktop file, the
	// system default, or nothing at all.
	Source string

	// Config is the parsed configuration, nil when none was found.
	Config *Config

	// Problems are reasons this machine will not get the backends it should.
	// Each is a fact the user can act on by editing a configuration.
	Problems []string

	// Notices are known limitations of the stack rather than of this machine's
	// configuration. They are reported because a silent gap is worse than a
	// stated one, and they are kept out of Problems because a user cannot fix
	// them by editing anything — `zenith doctor` failing for a gap the user has
	// no lever on teaches them to ignore its exit code.
	Notices []string
}

// Where a configuration can come from, in the order the dispatcher searches.
const (
	SourceUserConfig = "user config"
	SourceSystem     = "system default"
	SourceNone       = "none"
)

// Env is what the resolution reads.
type Env struct {
	// XDGConfigHome is $XDG_CONFIG_HOME; empty means ~/.config.
	XDGConfigHome string
	// Home is the user's home directory.
	Home string
	// CurrentDesktop is $XDG_CURRENT_DESKTOP as the running session sees it.
	CurrentDesktop string
}

// Resolve works out which configuration the dispatcher will read, and whether it
// is the one Zenith intends.
//
// The order of the problems is deliberate: the identity first, because every
// downstream failure is caused by it, and the backends last, because they are the
// consequence rather than the cause.
func Resolve(env Env) Resolution {
	resolution := Resolution{
		CurrentDesktop: env.CurrentDesktop,
		ConfigFile:     ConfigFile(env.CurrentDesktop),
		Source:         SourceNone,
	}

	configHome := env.XDGConfigHome
	if configHome == "" && env.Home != "" {
		configHome = filepath.Join(env.Home, ".config")
	}

	// 1. The session identity. A wrong one is the root cause of everything else.
	if env.CurrentDesktop == "" {
		resolution.Problems = append(resolution.Problems,
			"XDG_CURRENT_DESKTOP is not set, so the dispatcher cannot derive a "+
				"per-desktop configuration name and will use the system default")
	} else if env.CurrentDesktop != ExpectedDesktop {
		resolution.Problems = append(resolution.Problems, fmt.Sprintf(
			"XDG_CURRENT_DESKTOP is %q, not %q. The dispatcher looks for %q, so "+
				"it will not read this session's portal configuration",
			env.CurrentDesktop, ExpectedDesktop, resolution.ConfigFile))
	}

	// 2. The per-desktop user file.
	if configHome != "" {
		userPath := filepath.Join(configHome, "xdg-desktop-portal", resolution.ConfigFile)
		if config, err := parseFile(userPath); err == nil && config != nil {
			resolution.Source = SourceUserConfig
			resolution.Config = config
		}
	}

	// 3. The system default, which is the fallback the silent failure lands in.
	if resolution.Config == nil {
		if config, err := parseFile("/etc/xdg/xdg-desktop-portal/portals.conf"); err == nil && config != nil {
			resolution.Source = SourceSystem
			resolution.Config = config
		}
	}

	if resolution.Config == nil {
		resolution.Problems = append(resolution.Problems,
			"no portal configuration was found at all; the dispatcher will "+
				"use its compiled-in default")
		return resolution
	}

	resolution.Problems = append(resolution.Problems,
		checkBackends(*resolution.Config)...)

	// The Secret caveat is stated only when everything else is right. Mentioning
	// a known limitation alongside six real misconfigurations buries the six.
	if _, present := resolution.Config.Backend("org.gnome.impl.portal.Secret"); present &&
		len(resolution.Problems) == 0 {
		resolution.Notices = append(resolution.Notices, secretCaveat)
	}

	return resolution
}

// required maps the sections Zenith needs a backend for, to the backend it needs.
//
// Secret is listed because the configuration is what the architecture record
// specifies and omitting it silently would be worse than a stated limitation. It
// is kept separate from the rest because naming `use=gtk;` does not make
// passwords work: no backend in the stack implements the Secret interface, and
// gnome-keyring does not substitute for one.
var required = []struct {
	section string
	backend string
}{
	{"default", "hyprland"},
	{"org.gnome.impl.portal.ScreenCast", "hyprland"},
	{"org.gnome.impl.portal.Screenshot", "hyprland"},
	{"org.gnome.impl.portal.FileChooser", "gtk"},
	{"org.gnome.impl.portal.OpenURI", "gtk"},
	{"org.gnome.impl.portal.Print", "gtk"},
}

// secretCaveat is the one known limitation this configuration cannot fix.
//
// It is stated rather than omitted because a silently-absent interface is worse
// than a stated one, and kept out of Problems because no edit to any file on a
// user's machine changes it.
const secretCaveat = "the Secret interface does not work on this stack: no " +
	"backend implements org.freedesktop.portal.Secret and gnome-keyring does " +
	"not substitute for one, so every Flatpak that stores a password is " +
	"affected. The configuration line is present because the architecture record " +
	"specifies it, not because it works. Choosing a backend is an open question."

// checkBackends compares the configuration against what the session needs.
//
// A stale `use=gnome;` is reported by name. It is the single most likely thing to
// be wrong on an adopted machine, because it is what the previous desktop wrote,
// and it produces a dispatcher error rather than a useful fallback.
func checkBackends(config Config) []string {
	var problems []string

	for _, want := range required {
		got, present := config.Backend(want.section)
		if !present {
			problems = append(problems, fmt.Sprintf(
				"[%s] names no backend; %s is required",
				want.section, want.backend))
			continue
		}
		if got != want.backend {
			problems = append(problems, fmt.Sprintf(
				"[%s] selects %q, not %q. A stale preference from a previous "+
					"desktop produces a dispatcher error rather than a fallback",
				want.section, got, want.backend))
		}
	}
	return problems
}

// Parse reads a portals.conf from bytes.
func Parse(body string) (*Config, error) {
	config := &Config{Sections: map[string]map[string]string{}}
	section := "default"

	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if _, present := config.Sections[section]; !present {
				config.Sections[section] = map[string]string{}
			}
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		if _, present := config.Sections[section]; !present {
			config.Sections[section] = map[string]string{}
		}
		config.Sections[section][strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	return config, nil
}

func parseFile(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var body strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		body.WriteString(scanner.Text())
		body.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return Parse(body.String())
}
