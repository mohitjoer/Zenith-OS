package adopt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// machine builds a synthetic machine the preflight can be run against.
//
// Every path the checks read is created here and every command they shell out to
// is answered from a table, so the eleven checks are exercised against machines
// nobody has — including the ones that are impossible to reproduce on demand.
type machine struct {
	t        *testing.T
	root     string
	home     string
	desktop  string
	commands map[string]string
	// failures lists commands that should return an error, by "name arg arg".
	failures map[string]bool
}

func newMachine(t *testing.T) *machine {
	t.Helper()
	root := t.TempDir()
	home := t.TempDir()
	return &machine{
		t:        t,
		root:     root,
		home:     home,
		desktop:  "GNOME",
		commands: map[string]string{},
		failures: map[string]bool{},
	}
}

// defaultCommands is what a healthy machine answers.
func (m *machine) defaultCommands() {
	m.commands["dpkg-query -W -f=${Version} gnome-shell"] = "48.2\n"
	for _, name := range []string{"xdg-desktop-portal-hyprland", "xdg-desktop-portal-gtk"} {
		m.commands["apt-cache policy "+name] = name + ":\n  Installed: (none)\n  Candidate: 1.2.3\n"
	}
	m.commands["df -Pk /usr"] = "Filesystem 1024-blocks Used Available Capacity Mounted on\n" +
		"/dev/nvme0n1p3 103078920 61892336 41186584 60% /\n"
	m.commands["dpkg --audit"] = ""
}

// write creates a file under the machine's root.
func (m *machine) write(absolute, content string) {
	m.t.Helper()
	path := filepath.Join(m.root, absolute)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		m.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		m.t.Fatal(err)
	}
}

func (m *machine) mkdir(absolute string) {
	m.t.Helper()
	if err := os.MkdirAll(filepath.Join(m.root, absolute), 0o755); err != nil {
		m.t.Fatal(err)
	}
}

// env builds the Env this machine answers to.
func (m *machine) env() Env {
	machine := m
	return Env{
		Root:    m.root,
		Desktop: m.desktop,
		Home:    m.home,
		Run: func(name string, args ...string) (string, error) {
			key := strings.TrimSpace(name + " " + strings.Join(args, " "))
			if machine.failures[key] {
				return "", fmt.Errorf("%s: not available", key)
			}
			if output, ok := machine.commands[key]; ok {
				return output, nil
			}
			return "", fmt.Errorf("%s: not answered", key)
		},
	}
}

// ready builds a machine that passes every check.
func ready(t *testing.T) *machine {
	t.Helper()
	m := newMachine(t)
	m.defaultCommands()
	m.write("/etc/os-release", "NAME=\"Ubuntu\"\nID=ubuntu\nVERSION_ID=\"26.04\"\nPRETTY_NAME=\"Ubuntu 26.04 LTS\"\n")
	m.write("/etc/X11/default-display-manager", "/usr/sbin/gdm3\n")
	m.mkdir("sys/class/drm/card0/device")
	if err := os.Symlink("../../../bus/pci/drivers/i915",
		filepath.Join(m.root, "sys/class/drm/card0/device/driver")); err != nil {
		t.Fatal(err)
	}
	m.mkdir("dev/dri")
	m.write("dev/dri/renderD128", "")
	m.mkdir("sys/firmware/efi/efivars")
	return m
}

func (m *machine) check(t *testing.T) Report {
	t.Helper()
	report, err := Check(m.env())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return report
}

func findingNamed(report Report, name string) Finding {
	for _, finding := range report.Findings {
		if finding.Check == name {
			return finding
		}
	}
	return Finding{Check: name, Severity: "missing"}
}

// TestReadyMachineHasNoBlockers is the baseline every other test departs from.
func TestReadyMachineHasNoBlockers(t *testing.T) {
	report := ready(t).check(t)

	if !report.Ready() {
		t.Errorf("a ready machine is blocked: %v", report.Blockers())
	}
	if len(report.Findings) != 11 {
		t.Errorf("got %d findings, want 11; the preflight has eleven checks and "+
			"a skipped one is a check that cannot report a problem", len(report.Findings))
	}
	if code := report.ExitCode(); code != 0 {
		t.Errorf("ExitCode = %d, want 0", code)
	}
}

// TestBaseReleaseIsCheckedFirst checks the one thing that decides whether the
// rest is worth reading.
func TestBaseReleaseIsCheckedFirst(t *testing.T) {
	tests := []struct {
		name        string
		osRelease   string
		wantBlocked bool
	}{
		{"correct", "ID=ubuntu\nVERSION_ID=\"26.04\"\n", false},
		{"wrong release", "ID=ubuntu\nVERSION_ID=\"24.04\"\n", true},
		{"unquoted version", "ID=ubuntu\nVERSION_ID=26.04\n", false},
		{"not ubuntu", "ID=fedora\nVERSION_ID=\"42\"\n", true},
		{"missing", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := ready(t)
			if tc.osRelease == "" {
				if err := os.Remove(filepath.Join(m.root, "etc/os-release")); err != nil {
					t.Fatal(err)
				}
			} else {
				m.write("/etc/os-release", tc.osRelease)
			}
			report := m.check(t)
			finding := findingNamed(report, "Base")

			blocked := finding.Severity == Blocker
			if blocked != tc.wantBlocked {
				t.Errorf("Base = %q (%s), want blocked=%v",
					finding.Detail, finding.Severity, tc.wantBlocked)
			}
		})
	}
}

// TestOSReleaseWithoutFileIsABlocker covers the machine where nothing can be
// established at all.
func TestOSReleaseWithoutFileIsABlocker(t *testing.T) {
	m := ready(t)
	// Remove it rather than not writing it, so the check sees a machine where
	// something else removed it.
	if err := os.Remove(filepath.Join(m.root, "etc/os-release")); err != nil {
		t.Fatal(err)
	}
	if finding := findingNamed(m.check(t), "Base"); finding.Severity != Blocker {
		t.Errorf("Base = %q, want a blocker", finding.Detail)
	}
}

// TestNonGNOMESessionIsBlocked covers the machine that is already something else.
func TestNonGNOMESessionIsBlocked(t *testing.T) {
	for _, desktop := range []string{"KDE", "XFCE", "Hyprland", "", "sway"} {
		m := ready(t)
		m.desktop = desktop
		finding := findingNamed(m.check(t), "Session")
		if finding.Severity != Blocker {
			t.Errorf("desktop %q = %q, want a blocker", desktop, finding.Detail)
		}
	}
	// The case-insensitive match matters: the variable has been spelled
	// "GNOME", "X-GNOME" and "gnome" in the wild.
	for _, desktop := range []string{"GNOME", "gnome", "X-GNOME"} {
		m := ready(t)
		m.desktop = desktop
		if finding := findingNamed(m.check(t), "Session"); finding.Severity == Blocker {
			t.Errorf("desktop %q was blocked", desktop)
		}
	}
}

// TestDisplayManagerAcceptsOnlyTwo covers the login managers this project has
// actually considered.
func TestDisplayManagerAcceptsOnlyTwo(t *testing.T) {
	for _, dm := range []string{"/usr/sbin/gdm3", "/usr/bin/sddm"} {
		m := ready(t)
		m.write("/etc/X11/default-display-manager", dm+"\n")
		finding := findingNamed(m.check(t), "Display manager")
		if finding.Severity == Blocker {
			t.Errorf("%s was blocked", dm)
		}
		if finding.Note != "left untouched" {
			t.Errorf("%s does not say the display manager is untouched: %q", dm, finding.Note)
		}
	}

	for _, dm := range []string{"/usr/sbin/lightdm", "/usr/bin/anything"} {
		m := ready(t)
		m.write("/etc/X11/default-display-manager", dm+"\n")
		if finding := findingNamed(m.check(t), "Display manager"); finding.Severity != Blocker {
			t.Errorf("%s was accepted", dm)
		}
	}
}

// TestMissingGNOMEIsABlocker covers the machine with nothing to switch away from.
func TestMissingGNOMEIsABlocker(t *testing.T) {
	m := ready(t)
	m.failures["dpkg-query -W -f=${Version} gnome-shell"] = true
	if finding := findingNamed(m.check(t), "GNOME"); finding.Severity != Blocker {
		t.Errorf("a machine without gnome-shell is %q", finding.Detail)
	}
}

// TestGPUNeverBlocks covers the decision that keeps a ceiling off reach.
//
// Refusing adoption for an unusual GPU would put a limit on the exact hardware
// this project exists to support, and adoption installs no driver — so the only
// honest response to a GPU it does not recognise is to say so.
func TestGPUNeverBlocks(t *testing.T) {
	machines := map[string]string{
		"nvidia": "nvidia",
		"hybrid": "amdgpu",
		"weird":  "some_unknown_driver",
	}

	for name, driver := range machines {
		m := ready(t)
		// Rebuild the drm tree with one driver.
		os.RemoveAll(filepath.Join(m.root, "sys/class/drm"))
		m.mkdir("sys/class/drm/card0/device")
		os.Symlink("../../../bus/pci/drivers/"+driver,
			filepath.Join(m.root, "sys/class/drm/card0/device/driver"))

		finding := findingNamed(m.check(t), "GPU")
		if finding.Severity == Blocker {
			t.Errorf("%s GPU blocked adoption; it must only ever warn", name)
		}
		if finding.Detail == "" {
			t.Errorf("%s GPU reported nothing", name)
		}
	}
}

// TestNVIDIAWarnsByName covers the warning that has to be specific to be useful.
func TestNVIDIAWarnsByName(t *testing.T) {
	m := ready(t)
	os.RemoveAll(filepath.Join(m.root, "sys/class/drm"))
	m.mkdir("sys/class/drm/card0/device")
	os.Symlink("../../../bus/pci/drivers/nvidia",
		filepath.Join(m.root, "sys/class/drm/card0/device/driver"))

	finding := findingNamed(m.check(t), "GPU")
	if finding.Severity != Warning {
		t.Errorf("NVIDIA = %q, want a warning", finding.Detail)
	}
	if !strings.Contains(finding.Note, "mux") {
		t.Errorf("the NVIDIA warning does not mention the mux: %q", finding.Note)
	}
}

// TestMissingRenderNodeIsABlocker covers the check the spec calls a blocker.
func TestMissingRenderNodeIsABlocker(t *testing.T) {
	m := ready(t)
	os.RemoveAll(filepath.Join(m.root, "dev/dri"))
	if finding := findingNamed(m.check(t), "Composite"); finding.Severity != Blocker {
		t.Errorf("a machine with no render node is %q", finding.Detail)
	}
}

// TestPortalsMustBeInstallable covers the only check about the future rather
// than the present, and the only one that blocks on it.
func TestPortalsMustBeInstallable(t *testing.T) {
	// A candidate of "(none)" means the archive has dropped the package. An
	// installed version with no candidate would still fail on the next upgrade,
	// so it is caught here rather than months later.
	for _, candidate := range []string{"(none)", ""} {
		m := ready(t)
		m.commands["apt-cache policy xdg-desktop-portal-gtk"] =
			"xdg-desktop-portal-gtk:\n  Installed: 4.0.1\n  Candidate: " + candidate + "\n"
		finding := findingNamed(m.check(t), "Portal apt")
		if finding.Severity != Blocker {
			t.Errorf("candidate %q = %q, want a blocker", candidate, finding.Detail)
		}
	}

	// apt-cache failing entirely is the same conclusion by another route.
	m := ready(t)
	m.failures["apt-cache policy xdg-desktop-portal-hyprland"] = true
	if finding := findingNamed(m.check(t), "Portal apt"); finding.Severity != Blocker {
		t.Error("an unanswerable apt-cache was not a blocker")
	}
}

// TestDiskFloor checks the 2 GiB rule at its boundary.
func TestDiskFloor(t *testing.T) {
	dfLine := func(gib int) string {
		return "Filesystem 1024-blocks Used Available Capacity Mounted on\n" +
			fmt.Sprintf("/dev/nvme0n1p3 103078920 61892336 %d 60%% /\n", gib*1024*1024)
	}

	m := ready(t)
	m.commands["df -Pk /usr"] = dfLine(41)
	if finding := findingNamed(m.check(t), "Disk"); finding.Severity == Blocker {
		t.Errorf("41 GiB free blocked: %q", finding.Detail)
	}

	// Just under 2 GiB blocks; exactly 2 does not.
	m = ready(t)
	m.commands["df -Pk /usr"] = dfLine(1)
	if finding := findingNamed(m.check(t), "Disk"); finding.Severity != Blocker {
		t.Errorf("1 GiB free was accepted: %q", finding.Detail)
	}
}

// TestAPTLockIsAWarningNotABlocker covers the distinction that is easy to get
// backwards.
//
// A lock file existing is normal — apt creates one per operation. What cannot be
// established without fuser is *who* holds it, so the honest answer is a warning
// saying so. A half-configured package is different: it is a blocker, because
// running apt in that state is unsafe.
func TestAPTLockIsAWarningNotABlocker(t *testing.T) {
	m := ready(t)
	m.write("var/lib/dpkg/lock", "")
	m.write("var/lib/dpkg/lock-frontend", "")

	finding := findingNamed(m.check(t), "APT")
	if finding.Severity != Warning {
		t.Errorf("a lock file produced %q, want a warning", finding.Detail)
	}
	if !strings.Contains(finding.Note, "fuser") {
		t.Errorf("the warning does not say why it cannot attribute the lock: %q",
			finding.Note)
	}

	m = ready(t)
	m.commands["dpkg --audit"] = "zenith: half-configured\n"
	if finding := findingNamed(m.check(t), "APT"); finding.Severity != Blocker {
		t.Errorf("dpkg --audit output produced %q, want a blocker", finding.Detail)
	}
}

// TestExtensionsAreReportedNotRemoved covers the promise that they are archived.
func TestExtensionsAreReportedNotRemoved(t *testing.T) {
	m := ready(t)
	for _, name := range []string{"gnome-shell-extension-foo", "gnome-shell-extension-bar"} {
		if err := os.MkdirAll(filepath.Join(m.home, ".local/share/gnome-shell/extensions", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	finding := findingNamed(m.check(t), "Extensions")
	if !strings.Contains(finding.Detail, "2 third-party") {
		t.Errorf("Extensions = %q, want two counted", finding.Detail)
	}
	if finding.Note != "all will be archived" {
		t.Errorf("Extensions note = %q; it must say archived, not removed", finding.Note)
	}

	// And the directory is still there, because the preflight writes nothing.
	if _, err := os.Stat(filepath.Join(m.home,
		".local/share/gnome-shell/extensions/gnome-shell-extension-foo")); err != nil {
		t.Errorf("the preflight touched the extension directory: %v", err)
	}
}

// TestBootChainIsReportedOnly covers the check that must never block: the boot
// chain is on the never-write list.
func TestBootChainIsReportedOnly(t *testing.T) {
	legacy := newMachine(t)
	legacy.defaultCommands()
	legacy.write("/etc/os-release", "ID=ubuntu\nVERSION_ID=\"26.04\"\n")
	legacy.write("/etc/X11/default-display-manager", "/usr/sbin/gdm3\n")
	legacy.mkdir("dev/dri")

	finding := findingNamed(legacy.check(t), "Boot chain")
	if finding.Severity == Blocker {
		t.Errorf("a legacy-BIOS machine was blocked: %q", finding.Detail)
	}
	if finding.Detail != "legacy BIOS" {
		t.Errorf("Boot chain = %q, want it to report legacy BIOS", finding.Detail)
	}
}

// TestExitCodesAreDistinct covers the three-way contract a script depends on.
func TestExitCodesAreDistinct(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*machine)
		want    int
	}{
		{"ready", func(*machine) {}, 0},
		{"ready with a warning", func(m *machine) {
			m.write("var/lib/dpkg/lock", "")
		}, 2},
		{"blocked", func(m *machine) {
			m.write("/etc/os-release", "ID=fedora\nVERSION_ID=\"42\"\n")
		}, 1},
		{"blocked beats warning", func(m *machine) {
			m.write("/etc/os-release", "ID=fedora\nVERSION_ID=\"42\"\n")
			m.write("var/lib/dpkg/lock", "")
		}, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := ready(t)
			tc.prepare(m)
			if got := m.check(t).ExitCode(); got != tc.want {
				t.Errorf("ExitCode = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestPreflightWritesNothing is the property the whole command is trusted for.
//
// It is checked by comparing the entire tree before and after, rather than by
// inspecting the code for writes, because the code looks correct either way.
func TestPreflightWritesNothing(t *testing.T) {
	m := ready(t)
	m.mkdir("home/.local/share/gnome-shell/extensions/example")

	before := snapshotTree(t, m.root)
	if _, err := Check(m.env()); err != nil {
		t.Fatal(err)
	}
	after := snapshotTree(t, m.root)

	if len(before) != len(after) {
		t.Fatalf("the preflight added or removed %d path(s)", len(after)-len(before))
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("the preflight modified %s", path)
		}
	}
}

// TestNeverWritePromises pins the paths adoption may never write.
//
// These are load-bearing for booting or for the way back, and a promise in
// prose is not a promise. The list is data so this can assert against it.
func TestNeverWritePromises(t *testing.T) {
	forbidden := []string{
		"/boot/vmlinuz",
		"/boot/grub/grub.cfg",
		"/etc/fstab",
		"/etc/default/grub",
		"/etc/gdm3/daemon.conf",
		"/etc/kernel/cmdline",
		"/usr/share/gnome-shell/theme.css",
		"/usr/share/wayland-sessions/gnome.desktop",
		"/var/lib/dpkg/lock",
		"/var/lib/dpkg/info/zenith.postinst",
		// A full install writes this one; adoption does not. Adoption is not an
		// installer, and putting a boot configuration on its never-write list is
		// the point rather than an accident.
		"/boot/limine.conf",
	}

	for _, path := range forbidden {
		if _, blocked := ForbiddenWrite(path); !blocked {
			t.Errorf("ForbiddenWrite(%q) = false; adoption promises never to "+
				"write it", path)
		}
	}

	// The paths adoption does own, and the one exception the design names.
	for _, path := range []string{
		"/usr/share/zenith/session/zenith-session",
		"/etc/zenith/config.toml",
		"/usr/share/wayland-sessions/zenith.desktop",
		"/usr/share/xdg-desktop-portal/zenith-portals.conf",
		"/etc/sddm.conf.d/10-zenith.conf",
	} {
		if prefix, blocked := ForbiddenWrite(path); blocked {
			t.Errorf("ForbiddenWrite(%q) = %q; it is on the sanctioned list", path, prefix)
		}
	}
}

// TestParseOSRelease checks the quoting, because a quoted VERSION_ID is the norm
// and a parser that leaves the quotes on compares against "26.04" and fails.
func TestParseOSRelease(t *testing.T) {
	fields := parseOSRelease(
		"NAME=\"Ubuntu\"\n# a comment\nID=ubuntu\nVERSION_ID=\"26.04\"\nEMPTY=\nSINGLE='x'\n")

	if fields["NAME"] != "Ubuntu" {
		t.Errorf("NAME = %q", fields["NAME"])
	}
	if fields["ID"] != "ubuntu" {
		t.Errorf("ID = %q", fields["ID"])
	}
	if fields["VERSION_ID"] != "26.04" {
		t.Errorf("VERSION_ID = %q", fields["VERSION_ID"])
	}
	if fields["EMPTY"] != "" {
		t.Errorf("EMPTY = %q, want empty", fields["EMPTY"])
	}
	if fields["SINGLE"] != "x" {
		t.Errorf("SINGLE = %q", fields["SINGLE"])
	}
}

// TestParseFreeFromDF checks the column count, since df's filesystem name is
// last and its width is not fixed.
func TestParseFreeFromDF(t *testing.T) {
	output := "Filesystem 1024-blocks Used Available Capacity Mounted on\n" +
		"/dev/mapper/very-long-volume-name 100 40 60 40% /usr\n"

	free, err := parseFreeFromDF(output)
	if err != nil {
		t.Fatalf("parseFreeFromDF: %v", err)
	}
	if free != 60*1024 {
		t.Errorf("free = %d, want %d", free, 60*1024)
	}

	if _, err := parseFreeFromDF("only one line\n"); err == nil {
		t.Error("a one-line df output was accepted")
	}
}

// TestCandidateVersion covers the drop case, which is the one that matters.
func TestCandidateVersion(t *testing.T) {
	if got := candidateVersion("x:\n  Installed: (none)\n  Candidate: 1.2.3\n"); got != "1.2.3" {
		t.Errorf("candidate = %q", got)
	}
	if got := candidateVersion("x:\n  Installed: 4.0\n  Candidate: (none)\n"); got != "(none)" {
		t.Errorf("a dropped package's candidate = %q, want (none)", got)
	}
	if got := candidateVersion("N: Unable to locate package x\n"); got != "" {
		t.Errorf("an unknown package returned candidate %q, want empty", got)
	}
}

// snapshotTree reads every regular file under a root into a path→content map.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		files[relative] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return files
}
