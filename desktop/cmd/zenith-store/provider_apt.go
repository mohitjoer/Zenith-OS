package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
)

// The apps category: packages from the Ubuntu archive and Zenith's own
// repository. Browsing reads dpkg and apt-cache and never touches the network;
// changes go through zenith-pkg-add and zenith-pkg-remove, which ask polkit for
// authority, so this process never holds a privilege and never runs sudo.

// seedApps is the Discover shelf: a few packages worth finding without
// searching. Everything else about them (summary, description, version, home
// page) is read from apt, so the list is only names and a display title.
var seedApps = []struct{ pkg, title string }{
	{"vlc", "VLC"}, {"mpv", "mpv"}, {"gimp", "GIMP"}, {"inkscape", "Inkscape"},
	{"krita", "Krita"}, {"blender", "Blender"}, {"darktable", "darktable"},
	{"obs-studio", "OBS Studio"}, {"kdenlive", "Kdenlive"}, {"audacity", "Audacity"},
	{"handbrake", "HandBrake"}, {"easyeffects", "EasyEffects"}, {"pavucontrol", "Volume Control"},
	{"libreoffice", "LibreOffice"}, {"keepassxc", "KeePassXC"}, {"syncthing", "Syncthing"},
	{"transmission-gtk", "Transmission"}, {"qutebrowser", "qutebrowser"},
	{"neovim", "Neovim"}, {"git", "Git"}, {"lazygit", "lazygit"}, {"tmux", "tmux"},
	{"btop", "btop"}, {"htop", "htop"}, {"ripgrep", "ripgrep"}, {"fzf", "fzf"},
	{"gparted", "GParted"}, {"steam-installer", "Steam"}, {"lutris", "Lutris"},
}

// aptName is Debian policy for package names, tightened to refuse a leading
// dash so a name can never be read as an option by the helpers.
var aptName = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]+$`)

type aptProvider struct{}

func (aptProvider) Category() Category {
	return Category{
		ID:          "apps",
		Name:        "Apps",
		Group:       "find",
		Description: "Software from the Ubuntu archive and Zenith's repository, installed with apt.",
	}
}

func (aptProvider) Load(ctx context.Context, _ bool) ([]Item, SourceState, error) {
	if !have("dpkg-query") || !have("apt-cache") {
		return nil, SourceState{}, fmt.Errorf("apt is not available on this machine")
	}
	installed, err := dpkgInstalled(ctx)
	if err != nil {
		return nil, SourceState{}, err
	}
	names := make([]string, len(seedApps))
	for i, s := range seedApps {
		names[i] = s.pkg
	}
	info, err := aptShow(ctx, names)
	if err != nil {
		return nil, SourceState{}, err
	}
	items := make([]Item, 0, len(seedApps))
	for _, s := range seedApps {
		i, known := info[s.pkg]
		if !known && installed[s.pkg] == "" {
			continue // not in the enabled repositories
		}
		item := aptItem(s.pkg, i, installed)
		item.Name = s.title
		items = append(items, item)
	}
	return items, SourceState{}, nil
}

// Search answers a typed query from the whole archive. apt-cache matches the
// long description too, which for a common word is half the archive, so the
// hits are kept only when every term is in the name or the one-line summary,
// and libraries, headers and debug packages are left out.
func (aptProvider) Search(ctx context.Context, query string) ([]Item, error) {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return []Item{}, nil
	}
	if len(terms) > 6 {
		terms = terms[:6]
	}
	args := []string{"search", "--"}
	for _, t := range terms {
		args = append(args, regexp.QuoteMeta(t))
	}
	out, err := runProbe(ctx, "apt-cache", args...)
	if err != nil && len(out) == 0 {
		return nil, err
	}
	installed, err := dpkgInstalled(ctx)
	if err != nil {
		return nil, err
	}
	items := rankSearch(parseSearch(out, terms), terms, installed)
	if len(items) > 60 {
		items = items[:60]
	}
	return items, nil
}

func (aptProvider) Install(ctx context.Context, id string) error {
	return runPackageHelper(ctx, "zenith-pkg-add", "INSTALLING", id)
}

func (aptProvider) Remove(ctx context.Context, id string) error {
	return runPackageHelper(ctx, "zenith-pkg-remove", "REMOVING", id)
}

func aptItem(pkg string, info aptRecord, installed map[string]string) Item {
	item := Item{
		ID: pkg, Category: "apps", Name: pkg,
		Summary: info.summary, Description: info.description,
		Version: info.version, Upstream: info.homepage,
	}
	if v := installed[pkg]; v != "" {
		item.Installed = true
		item.InstalledVersion = v
	}
	if size := info.size; size > 0 {
		item.Metadata = map[string]any{"size": size, "installedSize": info.installedSize}
	}
	return item
}

// aptRecord is the part of an apt-cache stanza the store shows.
type aptRecord struct {
	version, homepage, summary, description string
	size, installedSize                     int64
}

// aptShow reads the candidate stanza of each named package. A name apt does not
// know is absent from the result: apt-cache prints what it can and complains on
// stderr, so an unknown name is not an error.
func aptShow(ctx context.Context, names []string) (map[string]aptRecord, error) {
	out, err := runProbe(ctx, "apt-cache", append([]string{"show", "--no-all-versions", "--"}, names...)...)
	if err != nil && len(out) == 0 && !strings.Contains(err.Error(), "No packages found") {
		return nil, err
	}
	return parseShow(out), nil
}

// parseShow reads RFC 822 stanzas: continuation lines start with a space, and a
// lone " ." is a paragraph break in a description.
func parseShow(out []byte) map[string]aptRecord {
	records := map[string]aptRecord{}
	for _, stanza := range strings.Split(string(out), "\n\n") {
		fields := map[string]string{}
		key := ""
		for _, line := range strings.Split(strings.Trim(stanza, "\n"), "\n") {
			switch {
			case line == "":
			case line[0] == ' ' || line[0] == '\t':
				if key != "" {
					fields[key] += "\n" + strings.TrimSpace(line)
				}
			default:
				k, v, _ := strings.Cut(line, ":")
				key, fields[k] = k, strings.TrimSpace(v)
			}
		}
		name := fields["Package"]
		if _, dup := records[name]; name == "" || dup {
			continue
		}
		desc := fields["Description-en"]
		if desc == "" {
			desc = fields["Description"]
		}
		summary, long, _ := strings.Cut(desc, "\n")
		rec := aptRecord{
			version: fields["Version"], homepage: fields["Homepage"], summary: summary,
			description: strings.ReplaceAll(strings.TrimSpace(long), "\n.\n", "\n\n"),
		}
		fmt.Sscan(fields["Size"], &rec.size)
		fmt.Sscan(fields["Installed-Size"], &rec.installedSize)
		rec.installedSize *= 1024
		records[name] = rec
	}
	return records
}

// dpkgInstalled maps every installed package to its version in one call. The
// status abbreviation's second letter is the state; "i" is installed whatever
// the want flag says, so a held package still counts.
func dpkgInstalled(ctx context.Context) (map[string]string, error) {
	out, err := runProbe(ctx, "dpkg-query", "-W", "-f", "${Package}\t${db:Status-Abbrev}\t${Version}\n")
	if err != nil {
		return nil, err
	}
	return parseInstalled(out), nil
}

func parseInstalled(out []byte) map[string]string {
	set := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if f := strings.Split(sc.Text(), "\t"); len(f) == 3 && len(f[1]) > 1 && f[1][1] == 'i' {
			set[f[0]] = f[2]
		}
	}
	return set
}

type searchHit struct{ name, summary string }

func parseSearch(out []byte, terms []string) []searchHit {
	var hits []searchHit
	for _, line := range strings.Split(string(out), "\n") {
		name, summary, ok := strings.Cut(line, " - ")
		if !ok || !aptName.MatchString(name) {
			continue
		}
		hay := strings.ToLower(name + " " + summary)
		all := true
		for _, t := range terms {
			all = all && strings.Contains(hay, t)
		}
		if all {
			hits = append(hits, searchHit{name, summary})
		}
	}
	return hits
}

// libraryish are the packages a person searching for software never wants.
var libraryish = regexp.MustCompile(`^(lib|linux-|python3-|python-|ruby-|node-|golang-|php-|perl-|r-cran-|gir1\.2-|xserver-)|(-dev|-dbg|-dbgsym|-doc|-data|-common|-l10n|-headers|-dkms)$|-locale-`)

func rankSearch(hits []searchHit, terms []string, installed map[string]string) []Item {
	joined := strings.Join(terms, " ")
	score := func(name string) int {
		switch {
		case name == joined:
			return 3
		case strings.HasPrefix(name, joined):
			return 2
		case strings.Contains(name, joined):
			return 1
		}
		return 0
	}
	kept := hits[:0:0]
	for _, h := range hits {
		if h.name == joined || !libraryish.MatchString(h.name) {
			kept = append(kept, h)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool {
		if a, b := score(kept[i].name), score(kept[j].name); a != b {
			return a > b
		}
		return len(kept[i].name) < len(kept[j].name)
	})
	items := make([]Item, len(kept))
	for i, h := range kept {
		items[i] = aptItem(h.name, aptRecord{summary: h.summary}, installed)
	}
	return items
}

func have(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// runProbe runs a read-only tool with the locale pinned, because its output is
// parsed and apt translates its field names.
func runProbe(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8", "LANGUAGE=")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			err = fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(ee.Stderr)))
		}
	}
	return out, err
}

// runPackageHelper runs zenith-pkg-add or zenith-pkg-remove and relays what
// apt reports as JSON lines on stdout, one {"stage","percent"} object each, for
// the store window. The helper escalates through pkexec, so the polkit agent
// shows the prompt; declining it ends the helper with pkexec's own 126 or 127.
//
// A cancel (ctx done) is honoured until dpkg starts, and after that the
// transaction runs to its end, because stopping apt-get mid-install leaves the
// system half-configured. The user cannot signal a root process, so "cancel" is
// written to the helper's stdin for it to act on; SIGTERM goes to pkexec for the
// case where the prompt is still open and nothing runs as root yet.
func runPackageHelper(ctx context.Context, helper, working, pkg string) error {
	if !aptName.MatchString(pkg) {
		return fmt.Errorf("%q is not a package name", pkg)
	}
	cmd := exec.Command(helper, "--status", pkg)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s is not installed: %w", helper, err)
	}
	emitStage("AUTHORIZING", -1)

	done := make(chan struct{})
	var cancelled, installing atomic.Bool
	go func() {
		select {
		case <-ctx.Done():
			if !installing.Load() {
				cancelled.Store(true)
				fmt.Fprintln(stdin, "cancel")
				_ = cmd.Process.Signal(syscall.SIGTERM)
			}
		case <-done:
		}
	}()
	relayAptStatus(stdout, working, &installing)
	err = cmd.Wait()
	close(done)

	var ee *exec.ExitError
	switch {
	case err == nil:
		return nil
	case cancelled.Load():
		return errCancelled
	case errors.As(err, &ee) && (ee.ExitCode() == 126 || ee.ExitCode() == 127):
		return fmt.Errorf("authorization was declined, or no authentication agent is running to ask for it")
	}
	if msg := strings.TrimSpace(stderr.String()); msg != "" {
		// The helper names itself on stderr for a terminal; the window shows
		// this as a sentence, and zenith-pkg-remove delegates to zenith-pkg-add.
		return errors.New(strings.TrimPrefix(msg, "zenith-pkg-add: "))
	}
	return err
}

var errCancelled = errors.New("cancelled")

// relayAptStatus turns apt's machine status (APT::Status-Fd) into stage lines.
// "dlstatus:id:percent:text" is the download and "pmstatus:pkg:percent:text"
// the dpkg run; anything else is the tools' ordinary output and is dropped.
func relayAptStatus(r io.Reader, working string, installing *atomic.Bool) {
	sc := bufio.NewScanner(r)
	last := ""
	for sc.Scan() {
		f := strings.SplitN(sc.Text(), ":", 4)
		if len(f) < 3 {
			continue
		}
		var stage string
		switch f[0] {
		case "dlstatus":
			stage = "DOWNLOADING"
		case "pmstatus":
			stage = working
			installing.Store(true)
		default:
			continue
		}
		var p float64
		fmt.Sscan(f[2], &p)
		if line := fmt.Sprintf("%s %d", stage, int(p)); line != last {
			last = line
			emitStage(stage, int(p))
		}
	}
	_ = sc.Err()
}

func emitStage(stage string, percent int) {
	fmt.Printf("{\"stage\":%q,\"percent\":%d}\n", stage, percent)
}
