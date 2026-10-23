package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The plugin tool's surroundings: where plugins live, how output is coloured,
// how an external helper is found. Ported from Ryoku's CLI (GPL-3.0), where
// these were the internal/sys helpers; here they sit beside the command that
// uses them because nothing else in this tool wants them, and every name carries
// a "plugin" prefix because this is package main and the other subcommands
// declare their helpers in the same namespace.
//
// Every directory is read from the environment at the moment it is needed, not
// once at start-up. The tests point XDG_* at a temp directory per test, and a
// path cached at init would send the second test to the first test's tree.

// pluginIO is what a subcommand talks to: its output, its errors, and whether
// anybody is there to answer a question. A subcommand takes one so a test can
// hand it buffers and a scripted answer instead of the process's own streams.
type pluginIO struct {
	out    io.Writer
	err    io.Writer
	in     io.Reader
	tty    bool // stdin is a terminal, so a y/N question can be asked
	colour bool // stdout is a terminal and the user has not asked for none
}

// pluginStdIO is the process's own streams. Colour is for a person at a
// terminal: the shell runs this tool with a pipe and strips escapes it finds,
// but a script that captures the output should not have to.
func pluginStdIO() pluginIO {
	return pluginIO{
		out:    os.Stdout,
		err:    os.Stderr,
		in:     os.Stdin,
		tty:    pluginIsTerminal(os.Stdin),
		colour: pluginIsTerminal(os.Stdout) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb",
	}
}

func pluginIsTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// paint wraps text in an SGR code when colour is on. 32 is green, 33 amber.
func (p pluginIO) paint(code, text string) string {
	if !p.colour {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (p pluginIO) green(text string) string { return p.paint("32", text) }
func (p pluginIO) amber(text string) string { return p.paint("33", text) }

// confirm asks a y/N question. It declines when nobody can answer: a command
// run from the shell's own UI has no terminal, and guessing "yes" there would
// install code the user never saw a prompt for.
func (p pluginIO) confirm(prompt string) bool {
	if !p.tty {
		return false
	}
	fmt.Fprintf(p.out, "%s [y/N] ", prompt)
	var answer string
	_, _ = fmt.Fscanln(p.in, &answer)
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func pluginHome() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	return os.Getenv("HOME")
}

func pluginXDG(env, fallback string) string {
	if value := os.Getenv(env); value != "" {
		return value
	}
	return filepath.Join(pluginHome(), fallback)
}

// pluginDataRoot is where `install` puts a plugin: ~/.local/share/zenith/plugins,
// the directory the ownership table in agents/agents.md gives to "the plugin".
func pluginDataRoot() string {
	return filepath.Join(pluginXDG("XDG_DATA_HOME", ".local/share"), "zenith", "plugins")
}

// pluginConfigFile is the placement file: which plugins are on, where, and with
// what settings. The shell watches it and retunes live. Only zenith-plugins-place
// writes it.
func pluginConfigFile() string {
	return filepath.Join(pluginXDG("XDG_CONFIG_HOME", ".config"), "zenith", "plugins.json")
}

// pluginDevDirs are the directories in ZENITH_PLUGINS_DIR, colon-separated: a
// folder of plugin folders that shadows the installed ones, so an author's
// checkout is what loads while they work on it. discover.sh reads the same
// variable the same way.
func pluginDevDirs() []string {
	var dirs []string
	for _, dir := range strings.Split(os.Getenv("ZENITH_PLUGINS_DIR"), ":") {
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func pluginExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func pluginHas(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// pluginOutput runs a command and returns what it printed.
func pluginOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}

// pluginCopyTree copies every regular file under src into dest, keeping the
// permission bits (a script's exec bit is the difference between running and
// not). It skips git metadata and refuses a symlink: a link inside a plugin is
// how a plugin would read or overwrite something outside its own folder.
func pluginCopyTree(src, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dest, rel), 0o755)
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlink in plugin tree: %s", rel)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("not a regular file in plugin tree: %s", rel)
		}
		return pluginCopyFile(path, filepath.Join(dest, rel))
	})
}

func pluginCopyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// pluginFirstSymlink returns the relative path of the first symlink under dir,
// or "" when there is none. The .git directory is not part of the plugin.
func pluginFirstSymlink(dir string) (string, error) {
	found := ""
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			found, _ = filepath.Rel(dir, path)
			return filepath.SkipAll
		}
		return nil
	})
	return found, err
}

// pluginPlaceTool finds zenith-plugins-place, the one writer of plugins.json.
// The CLI goes through it for the same reason the shell does: two writers of a
// file the shell watches is how a placement gets lost.
//
// Order: an explicit override (the tests', and a developer's), the shell tree a
// dev run points at, PATH (the packaged /usr/bin copy), then the copy inside
// the installed shell tree, which ships whole and so is there even on a build
// that has not yet put the script on PATH.
func pluginPlaceTool() (string, error) {
	if path := os.Getenv("ZENITH_PLUGINS_PLACE"); path != "" {
		return path, nil
	}
	if dir := os.Getenv("ZENITH_SHELL_DIR"); dir != "" {
		if path := filepath.Join(dir, "scripts", "zenith-plugins-place"); pluginExists(path) {
			return path, nil
		}
	}
	if path, err := exec.LookPath("zenith-plugins-place"); err == nil {
		return path, nil
	}
	if path := "/usr/share/zenith/shell/scripts/zenith-plugins-place"; pluginExists(path) {
		return path, nil
	}
	return "", errors.New("zenith-plugins-place is not installed; it ships with the zenith package")
}

// pluginPlace runs the placement tool and returns its complaint, not its
// chatter: the tool prints nothing on success.
func pluginPlace(args ...string) error {
	tool, err := pluginPlaceTool()
	if err != nil {
		return err
	}
	var output bytes.Buffer
	command := exec.Command(tool, args...)
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err != nil {
		return fmt.Errorf("zenith-plugins-place %s: %s", strings.Join(args, " "), pluginTail(output.String(), err))
	}
	return nil
}

// pluginTail is the last non-empty line of a tool's output, or the error when
// it said nothing.
func pluginTail(output string, fallback error) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "" {
		return last
	}
	return fallback.Error()
}

// pluginGit runs git in dir. GIT_TERMINAL_PROMPT=0 because a clone started from
// the shell's UI has no terminal to ask for a password on, and a prompt nobody
// can answer is a hang, not a failure.
func pluginGit(dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// pluginCommitAs commits with the plugin author as the identity when git has
// none configured, which a fresh desktop rarely does; a configured identity
// wins. Signing is off because a scaffold commit must not wait on a key.
func pluginCommitAs(dir, author, subject string) error {
	args := []string{"-c", "commit.gpgsign=false"}
	if _, err := pluginGit(dir, "config", "user.name"); err != nil {
		name, mail := pluginSplitAuthor(author)
		args = append(args, "-c", "user.name="+name, "-c", "user.email="+mail)
	}
	_, err := pluginGit(dir, append(args, "commit", "-q", "-m", subject)...)
	return err
}

// pluginSplitAuthor reads the manifest's `Name <mail>` form. Either half may be
// missing and both fall back to something git accepts.
func pluginSplitAuthor(author string) (string, string) {
	name, mail := strings.TrimSpace(author), ""
	if i := strings.Index(name, "<"); i >= 0 {
		mail = strings.Trim(strings.TrimSpace(name[i:]), "<>")
		name = strings.TrimSpace(name[:i])
	}
	if name == "" {
		name = "zenith"
	}
	if mail == "" {
		mail = strings.ToLower(strings.ReplaceAll(name, " ", "-")) + "@users.noreply.github.com"
	}
	return name, mail
}
