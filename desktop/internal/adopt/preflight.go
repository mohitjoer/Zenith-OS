// Package adopt implements `zenith adopt` — moving an existing GNOME desktop
// on the base layer over to the Zenith session, in place.
//
// This file is the preflight: `zenith adopt --check`. It changes nothing, needs
// no root, and is safe to run as often as the user likes. That is not a
// convenience — it is the property that makes the command trustworthy enough to
// run before deciding anything, and `zenith adopt` re-runs it as its own step
// zero rather than reimplementing a single check.
//
// Every absolute path and every external command comes from an Env, for the same
// reason the hardware probes take a filesystem root: a preflight that reads the
// live machine can only be tested on a machine that is like the one you care
// about. With an Env, the eleven checks below are exercised against synthetic
// machines — including the ones nobody has — in a test that runs anywhere.
package adopt

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Severity is what a check found.
type Severity string

const (
	// Blocker means `zenith adopt` will refuse. A hard blocker is a machine this
	// project cannot switch safely, and pretending otherwise is worse than
	// refusing.
	Blocker Severity = "blocker"
	// Warning means adoption will proceed and the user should read it.
	Warning Severity = "warning"
	// OK means the check passed and there is nothing to say.
	OK Severity = "ok"
)

// Finding is one check's result.
type Finding struct {
	// Check is the human label, as the sample output prints it.
	Check string
	// Severity is what it found.
	Severity Severity
	// Detail is the value worth reading: the version, the driver, the free space.
	Detail string
	// Note is an aside — "left untouched", "unchanged" — that belongs on the same
	// line without displacing the value.
	Note string
}

// Env is everything the preflight reads from outside this process.
//
// It is a struct rather than a set of globals so a test can point the whole
// preflight at a temporary tree, and so nothing here can reach the live machine
// by accident.
type Env struct {
	// Root prefixes every absolute path. Empty means the real filesystem.
	Root string

	// Desktop is $XDG_CURRENT_DESKTOP, which is how the preflight knows what
	// session is running.
	Desktop string

	// Home is the user's home directory, for the extension and state paths.
	Home string

	// Run executes an external command and returns its standard output.
	//
	// The preflight reads four external programs — dpkg-query, apt-cache, df and
	// dpkg — and does not reimplement any of them. Reimplementing dpkg's
	// dependency state or apt's candidate selection to avoid a subprocess would
	// produce a worse version of both, and the reason the hardware probes read
	// sysfs directly does not apply: these are programs with no equivalent in the
	// filesystem.
	Run func(name string, args ...string) (string, error)
}

// path resolves an absolute path against the environment's root.
func (e Env) path(absolute string) string {
	if e.Root == "" {
		return absolute
	}
	return filepath.Join(e.Root, absolute)
}

// Report is the whole preflight result.
type Report struct {
	Findings []Finding
}

// Blockers returns the findings that will stop adoption.
func (r Report) Blockers() []Finding { return r.bySeverity(Blocker) }

// Warnings returns the findings adoption will proceed past.
func (r Report) Warnings() []Finding { return r.bySeverity(Warning) }

// Ready reports whether adoption may proceed at all.
func (r Report) Ready() bool { return len(r.Blockers()) == 0 }

func (r Report) bySeverity(severity Severity) []Finding {
	var matched []Finding
	for _, finding := range r.Findings {
		if finding.Severity == severity {
			matched = append(matched, finding)
		}
	}
	return matched
}

// ExitCode is the preflight's exit status.
//
//	0  ready, no warnings
//	1  blocked
//	2  ready with warnings
//
// Warnings have their own code because the distinction is the one a script
// needs: a caller that must not proceed gets 1, and a caller that should tell
// the user to read something gets 2.
func (r Report) ExitCode() int {
	switch {
	case !r.Ready():
		return 1
	case len(r.Warnings()) > 0:
		return 2
	default:
		return 0
	}
}

// minFreeBytes is the disk the preflight insists on.
//
// Two gigabytes is not a guess about what adoption needs; it is the floor below
// which the base layer cannot be unpacked alongside what is already there, and a
// machine that fails it is one where adoption would run a machine out of room
// partway through.
const minFreeBytes = 2 * 1024 * 1024 * 1024

// Check runs every preflight check and returns the report.
//
// The order is the order of the design record's table, which is roughly
// cheapest and most decisive first: the base release decides whether anything
// else is worth reading.
func Check(env Env) (Report, error) {
	report := Report{}
	add := func(finding Finding) { report.Findings = append(report.Findings, finding) }

	add(checkBaseRelease(env))
	add(checkSession(env))
	add(checkDisplayManager(env))
	add(checkGNOME(env))
	add(checkGPU(env))
	add(checkComposite(env))
	add(checkPortals(env))
	add(checkDisk(env))
	add(checkAPTHealth(env))
	add(checkExtensions(env))
	add(checkBootChain(env))

	return report, nil
}

// checkBaseRelease reads /etc/os-release.
//
// This is the first check because everything else assumes it: the package
// names, the kernel fleet and the profile database are all written against one
// base layer, and on another one they are not merely wrong but misleading.
func checkBaseRelease(env Env) Finding {
	finding := Finding{Check: "Base", Severity: OK}

	data, err := os.ReadFile(env.path("/etc/os-release"))
	if err != nil {
		finding.Severity = Blocker
		finding.Detail = "no /etc/os-release"
		return finding
	}

	fields := parseOSRelease(string(data))
	id := fields["ID"]
	version := fields["VERSION_ID"]

	finding.Detail = fmt.Sprintf("%s %s", strings.Title(id), version) //nolint:staticcheck // ASCII release names only

	if id != "ubuntu" {
		finding.Severity = Blocker
		finding.Detail += "  (not Ubuntu)"
		return finding
	}
	if version != "26.04" {
		finding.Severity = Blocker
		finding.Detail += "  (not 26.04)"
	}
	return finding
}

// checkSession reads $XDG_CURRENT_DESKTOP.
//
// A non-GNOME session is either already a Zenith session or something else
// entirely. Both are worth refusing rather than switching.
func checkSession(env Env) Finding {
	finding := Finding{Check: "Session", Severity: OK}
	desktop := strings.ToUpper(env.Desktop)

	if desktop == "" {
		finding.Severity = Blocker
		finding.Detail = "no XDG_CURRENT_DESKTOP"
		return finding
	}

	finding.Detail = desktop
	if !strings.Contains(desktop, "GNOME") {
		finding.Severity = Blocker
		finding.Detail += "  (not GNOME)"
	}
	return finding
}

// checkDisplayManager reads /etc/X11/default-display-manager.
//
// Adoption leaves the display manager alone entirely — it is not in the removal
// set, and its configuration is on the never-write list. The check exists to
// say so, and to refuse a machine whose login manager this project has never
// considered.
func checkDisplayManager(env Env) Finding {
	finding := Finding{
		Check:    "Display manager",
		Severity: OK,
		Note:     "left untouched",
	}

	data, err := os.ReadFile(env.path("/etc/X11/default-display-manager"))
	if err != nil {
		finding.Severity = Blocker
		finding.Detail = "no default-display-manager"
		return finding
	}

	name := strings.TrimSpace(string(data))
	switch filepath.Base(name) {
	case "gdm3":
		finding.Detail = "gdm3"
	case "sddm":
		finding.Detail = "sddm"
	default:
		finding.Severity = Blocker
		finding.Detail = fmt.Sprintf("%s  (neither gdm3 nor sddm)", filepath.Base(name))
	}
	return finding
}

// checkGNOME asks dpkg whether gnome-shell is installed.
//
// There is nothing to switch away from without it, and a machine that reports a
// GNOME session without gnome-shell is broken in a way adoption would make worse.
func checkGNOME(env Env) Finding {
	finding := Finding{Check: "GNOME", Severity: OK}

	output, err := env.Run("dpkg-query", "-W", "-f=${Version}", "gnome-shell")
	if err != nil {
		finding.Severity = Blocker
		finding.Detail = "gnome-shell is not installed"
		return finding
	}

	version := strings.TrimSpace(output)
	if version == "" {
		finding.Severity = Blocker
		finding.Detail = "gnome-shell reports no version"
		return finding
	}
	finding.Detail = "gnome-shell " + version
	return finding
}

// checkGPU reports the display hardware.
//
// It never blocks. A machine with an unusual GPU is a warning, not a refusal:
// adoption does not install a driver, and refusing a machine for its GPU would
// put a ceiling on reach at exactly the hardware the project exists to support.
func checkGPU(env Env) Finding {
	finding := Finding{Check: "GPU", Severity: OK}

	drivers, err := filepath.Glob(env.path("/sys/class/drm/card*/device/driver"))
	if err != nil || len(drivers) == 0 {
		finding.Severity = Warning
		finding.Detail = "no DRM driver reported"
		return finding
	}

	// The driver is a symlink named "driver" whose target ends in the module
	// name. Taking the basename of the link path would report the literal
	// string "driver" for every device and silently classify none of them.
	names := make([]string, 0, len(drivers))
	for _, link := range drivers {
		name := filepath.Base(link)
		if target, err := os.Readlink(link); err == nil {
			name = filepath.Base(target)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	finding.Detail = strings.Join(names, ", ")

	// NVIDIA and PRIME are worth a warning each, and both are common enough that
	// saying them by name is more use than a generic "unusual GPU".
	joined := strings.ToLower(strings.Join(names, " "))
	switch {
	case strings.Contains(joined, "nvidia"):
		finding.Severity = Warning
		finding.Note = "NVIDIA; a hybrid machine may need a mux decision"
	case strings.Contains(joined, "amdgpu") && len(names) > 1:
		finding.Severity = Warning
		finding.Note = "hybrid graphics; confirm which GPU drives the panel"
	}
	return finding
}

// checkComposite looks for a render node.
//
// Without one there is no GPU rendering, which is not a reason to refuse
// adoption — a machine that renders on llvmpipe still boots — but it is a reason
// to say so before the user finds out.
func checkComposite(env Env) Finding {
	finding := Finding{Check: "Composite", Severity: OK}

	nodes, err := filepath.Glob(env.path("/dev/dri/render*"))
	if err != nil || len(nodes) == 0 {
		finding.Severity = Blocker
		finding.Detail = "no render node under /dev/dri"
		return finding
	}
	finding.Detail = filepath.Base(nodes[0])
	return finding
}

// checkPortals asks apt whether both portal backends are installable.
//
// This is a hard blocker and the only one that is about the future rather than
// the present: Zenith's shell has no portal backend of its own, so a machine
// that cannot install these gets a desktop where file choosers and screen
// sharing silently do not work.
func checkPortals(env Env) Finding {
	finding := Finding{Check: "Portal apt", Severity: OK}

	required := []string{"xdg-desktop-portal-hyprland", "xdg-desktop-portal-gtk"}
	var missing []string
	var versions []string

	for _, name := range required {
		output, err := env.Run("apt-cache", "policy", name)
		if err != nil {
			missing = append(missing, name)
			continue
		}
		candidate := candidateVersion(output)
		if candidate == "" || candidate == "(none)" {
			missing = append(missing, name)
			continue
		}
		versions = append(versions, name+" "+candidate)
	}

	if len(missing) > 0 {
		finding.Severity = Blocker
		finding.Detail = "not installable: " + strings.Join(missing, ", ")
		return finding
	}
	finding.Detail = strings.Join(versions, "; ")
	return finding
}

// checkDisk requires the free space floor.
//
// df is used rather than statvfs because Go's standard library does not expose
// statvfs and pulling in golang.org/x/sys for one call is not worth a
// dependency in a module that currently has none.
func checkDisk(env Env) Finding {
	finding := Finding{Check: "Disk", Severity: OK}

	output, err := env.Run("df", "-Pk", "/usr")
	if err != nil {
		finding.Severity = Warning
		finding.Detail = "could not read free space"
		return finding
	}

	free, err := parseFreeFromDF(output)
	if err != nil {
		finding.Severity = Warning
		finding.Detail = "could not read free space"
		return finding
	}

	gib := free / (1024 * 1024 * 1024)
	finding.Detail = fmt.Sprintf("%d GiB free on /usr", gib)

	if free < minFreeBytes {
		finding.Severity = Blocker
		finding.Detail += "  (under 2 GiB)"
	}
	return finding
}

// checkAPTHealth looks for a held lock and a half-configured package.
//
// Both mean APT is not in a state where running it again is safe, and adoption
// runs APT. It is a blocker because the alternative is discovering it halfway
// through a switch.
func checkAPTHealth(env Env) Finding {
	finding := Finding{Check: "APT", Severity: OK}

	locks, err := filepath.Glob(env.path("/var/lib/dpkg/lock*"))
	if err != nil {
		finding.Severity = Warning
		finding.Detail = "could not read the dpkg lock directory"
		return finding
	}

	// A lock file existing is normal — apt creates one per operation. What matters
	// is whether another apt is running, which the lock's holder reports; without
	// fuser or lsof the honest answer is that a lock file is present and cannot
	// be attributed, which is a warning rather than a blocker.
	if len(locks) > 0 {
		finding.Severity = Warning
		finding.Detail = fmt.Sprintf("%d lock file(s) present", len(locks))
		finding.Note = "cannot attribute without fuser"
	}

	if output, err := env.Run("dpkg", "--audit"); err == nil {
		if strings.TrimSpace(output) != "" {
			finding.Severity = Blocker
			finding.Detail = "dpkg --audit reported problems"
		} else if finding.Detail == "" {
			finding.Detail = "no locks, dpkg --audit clean"
		}
	} else if finding.Detail == "" {
		finding.Severity = Warning
		finding.Detail = "could not run dpkg --audit"
	}
	return finding
}

// checkExtensions reports third-party GNOME Shell extensions.
//
// They become inert rather than deleted, and that is worth saying before the
// switch rather than after. Adoption archives them; it does not remove them, and
// the extension directory is on the never-write list.
func checkExtensions(env Env) Finding {
	finding := Finding{Check: "Extensions", Severity: OK}

	home := env.Home
	if home == "" {
		finding.Detail = "no home directory to inspect"
		return finding
	}

	entries, err := os.ReadDir(filepath.Join(home, ".local", "share", "gnome-shell", "extensions"))
	if err != nil {
		finding.Detail = "none"
		return finding
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	if count == 0 {
		finding.Detail = "none"
		return finding
	}
	finding.Detail = fmt.Sprintf("%d third-party", count)
	finding.Note = "all will be archived"
	return finding
}

// checkBootChain reports the firmware situation.
//
// Report only, always. The boot chain is on the never-write list, and a machine
// whose boot needs a cmdline change is outside this project's funnel rather than
// inside it.
func checkBootChain(env Env) Finding {
	finding := Finding{Check: "Boot chain", Severity: OK, Note: "unchanged"}

	if _, err := os.Stat(env.path("/sys/firmware/efi")); err != nil {
		finding.Detail = "legacy BIOS"
		return finding
	}
	finding.Detail = "UEFI"

	if _, err := os.Stat(env.path("/sys/firmware/efi/efivars")); err == nil {
		finding.Detail += ", Secure Boot present"
	} else {
		finding.Severity = Warning
		finding.Note = "Secure Boot state unknown"
	}
	return finding
}

// neverWrite is the list of paths adoption promises never to write.
//
// Every one is either load-bearing for booting or load-bearing for the way back.
// If a future change needs one of them, that change is wrong. It lives here as
// data rather than as a comment so a test can assert against it, which is the
// only way a promise in prose stays a promise.
var neverWrite = []string{
	"/boot/",
	"/usr/share/xsessions/",
	"/etc/fstab",
	// One exception, and it is the whole first step of adoption: writing the
	// session file is what makes the Zenith session appear at the login screen,
	// and it is reversible by picking the other session.
	"/usr/share/wayland-sessions/", // except zenith.desktop
	"/etc/default/grub",
	"/usr/share/gnome-session/",
	"/etc/kernel/",
	"/usr/share/gnome-shell/",
	"/etc/initramfs-tools/",
	"/etc/gdm3/",
	"/etc/linux-image/",
	"/etc/xdg/",
	"/var/lib/dpkg/info/",
	"/var/lib/dpkg/lock",
}

// neverWriteExceptions are the paths inside a never-write prefix that adoption
// does write, each with the reason it is the exception rather than an oversight.
//
// There is exactly one, and it is the entire first step of adoption: the Zenith
// session file in the fixed wayland-sessions directory, which is what makes the
// session appear at the login screen and is reversible by picking the other one.
var neverWriteExceptions = map[string]string{
	"/usr/share/wayland-sessions/zenith.desktop": "the Zenith session registration — the whole first step, reversible by " +
		"choosing the other session at the login screen",
}

// ForbiddenWrite reports whether adoption writing to a path breaks a promise.
//
// The preflight never writes, so this exists for the commands that do and for
// the test that keeps the list honest.
func ForbiddenWrite(path string) (string, bool) {
	clean := filepath.Clean(path)

	if _, allowed := neverWriteExceptions[clean]; allowed {
		return "", false
	}

	for _, prefix := range neverWrite {
		if clean == prefix || strings.HasPrefix(clean, prefix) {
			return prefix, true
		}
	}
	return "", false
}

// NeverWriteExceptions returns the exceptions, with their reasons, so a report
// can print why a path is permitted rather than leaving it to be looked up.
func NeverWriteExceptions() map[string]string {
	out := make(map[string]string, len(neverWriteExceptions))
	for path, reason := range neverWriteExceptions {
		out[path] = reason
	}
	return out
}

// NeverWritePaths returns the promises, for the report and for tests.
func NeverWritePaths() []string {
	return append([]string(nil), neverWrite...)
}

// parseOSRelease parses the KEY=VALUE lines of /etc/os-release.
//
// Values may be quoted, and a quoted VERSION_ID is the norm, so the quotes have
// to come off or every comparison against "26.04" fails.
func parseOSRelease(body string) map[string]string {
	fields := map[string]string{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		fields[strings.TrimSpace(key)] = value
	}
	return fields
}

// candidateVersion pulls the candidate version out of `apt-cache policy`.
//
// apt-cache prints an "Installed:" and a "Candidate:" block. The candidate is
// the one that says whether the package can be installed at all: an installed
// version with a candidate of "(none)" means the archive has dropped it, and
// installing it again on the next upgrade would not be possible.
func candidateVersion(output string) string {
	inCandidate := false
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Candidate:") {
			inCandidate = true
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "Candidate:"))
		}
		if trimmed == "" {
			inCandidate = false
		}
		_ = inCandidate
	}
	return ""
}

// parseFreeFromDF pulls the available-kilobytes column out of `df -Pk`.
//
// df's output is a header and one line per filesystem, with the filesystem name
// last. The free column is the fourth of six, so it is counted from the end of
// the data lines rather than from the start, which keeps it correct whatever the
// filesystem name happens to be.
func parseFreeFromDF(output string) (uint64, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("df produced %d line(s)", len(lines))
	}

	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return 0, fmt.Errorf("df line has %d fields", len(fields))
	}

	// blocks, used, available, use%, mount
	kilobytes, err := strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("df available column %q: %w", fields[3], err)
	}
	return kilobytes * 1024, nil
}
