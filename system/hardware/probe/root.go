// Package probe reads machine hardware out of sysfs.
//
// Every reader in this package takes a root path as its first argument and
// reads nothing outside it. That single property is what makes the whole
// subsystem testable without a hardware lab: a test points a reader at a
// captured directory tree instead of at a laptop, and the reader cannot tell
// the difference.
//
// The rule is mechanically enforced rather than trusted. TestNoAbsoluteSysfsPath
// in this package walks the AST of every file here and fails if a string
// literal beginning "/sys" or "/proc" appears anywhere except the RootDefault
// constant below. A new hard-coded path is a test failure, not something a
// reviewer has to notice.
//
// Reading sysfs directly, rather than shelling out to lspci, lsusb, acpidump or
// edid-decode, is a deliberate choice recorded in docs/hardware-detection.md.
// A shell-out costs a process spawn and a pci.ids parse to obtain data that is
// two read() calls away, and it makes detection depend on four packages being
// present in an image that is a reviewed artefact. The ISO's package set can
// change without warning; this code cannot.
//
// Four rules apply to every reader here, and they are what keeps the subsystem
// maintainable:
//
//   - Absent is a result. No DMI on an ARM board, no ACPI tables, a
//     disconnected connector's empty EDID block — all recorded as unknown or
//     present:false, never as an error, and never as an empty string that later
//     matches an empty field in a profile.
//   - One filesystem root per probe. See above.
//   - Never trust the source. DMI strings, EDID blocks and ACPI tables are all
//     writable by firmware and trivially attacker-chosen in a virtual machine.
//     They select a profile; nothing derived from them may introduce a package
//     outside the reviewed set.
//   - Redact at the reader. See package redact. A reader copies named fields
//     into a struct and never copies a directory, so there is no code path that
//     can emit a field nobody classified.
package probe

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// RootDefault is the one absolute sysfs path in this package. The binary passes
// it in; every reader in this package takes a root and nothing else.
//
// It is a variable rather than a constant only so that tests can point a reader
// at a fixture without a helper rewriting paths. Production code never assigns
// to it.
var RootDefault = "/sys"

// FS is a reader rooted at one filesystem tree. It is the type every exported
// reader accepts, and it exists so that the root is passed once rather than
// being re-derived from a global at every call site.
type FS struct {
	root string
}

// New returns a reader for the tree at root. An empty root is an error rather
// than a silent fallback to RootDefault: a reader that quietly ignored its
// argument would be reading the live machine in a test that believed it was
// reading a fixture, which is exactly the bug this package is arranged to
// prevent.
func New(root string) (*FS, error) {
	if root == "" {
		return nil, fmt.Errorf("probe: empty root")
	}
	return &FS{root: filepath.Clean(root)}, nil
}

// Root returns the tree this reader was opened on.
func (fs *FS) Root() string { return fs.root }

// path joins a sysfs-relative path onto the root. Callers pass sysfs-relative
// paths only; nothing here may name an absolute path, and the test in
// root_test.go is what keeps that true.
func (fs *FS) path(rel string) string {
	return filepath.Join(fs.root, filepath.FromSlash(rel))
}

// readString reads a sysfs attribute and trims the trailing newline sysfs
// always appends. A missing file is ( "", false, nil ) — absent is a result, not
// an error, and the caller decides what absent means for its field.
func (fs *FS) readString(rel string) (value string, present bool, err error) {
	data, err := os.ReadFile(fs.path(rel))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		// A sysfs attribute can be unreadable without being absent, most often
		// because it is mode 0400 and we are unprivileged. That is a fact about
		// the machine, not a failure of the probe, so it is reported as "not
		// readable" rather than as an error that aborts the whole probe.
		if os.IsPermission(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimRight(string(data), "\n"), true, nil
}

// readInt reads a sysfs attribute as an integer. An attribute that is present
// but unparseable is ( 0, false, nil ): firmware writes malformed values often
// enough that treating one as a fatal error would lose the whole machine.
func (fs *FS) readInt(rel string) (value int, present bool, err error) {
	raw, present, err := fs.readString(rel)
	if err != nil || !present {
		return 0, present, err
	}
	n, convErr := strconv.Atoi(strings.TrimSpace(raw))
	if convErr != nil {
		return 0, false, nil
	}
	return n, true, nil
}

// readBytes reads a raw attribute. The byte count returned by ReadFile is the
// read length and is authoritative; the size sysfs reports in its own size
// attribute is not, and on this base layer's kernel `stat -c %s` on a DRM edid
// file returns 0 while the file itself reads 128 bytes.
func (fs *FS) readBytes(rel string) (data []byte, present bool, err error) {
	data, err = os.ReadFile(fs.path(rel))
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

// readLink resolves a sysfs symlink and returns its basename.
//
// The kernel exposes a bound driver as /sys/bus/pci/devices/<addr>/driver,
// which is a symlink to ../../../bus/pci/drivers/<module>. The module name is
// the basename; the path above it is a fact about the sysfs layout and is
// deliberately discarded rather than carried into a report.
func (fs *FS) readLink(rel string) (string, error) {
	target, err := os.Readlink(fs.path(rel))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return filepath.Base(target), nil
}

// stat reports whether a sysfs entry exists.
//
// It is used for presence checks where the value does not matter: a device
// carries a "virtfn" file when it is a passed-through function, and the file's
// existence is the entire signal. Reading it as text would give an empty string
// and lose the distinction between "not a virtual function" and "the file
// exists and is empty".
func (fs *FS) stat(rel string) error {
	_, err := os.Lstat(fs.path(rel))
	return err
}

// entries lists the directory names at a sysfs-relative path, sorted, so that
// every reader produces the same output for the same tree regardless of
// filesystem iteration order. Determinism is not cosmetic: detection that
// resolved differently on two runs of the same machine would be untestable and
// untrustworthy.
func (fs *FS) entries(rel string) ([]string, error) {
	dirEntries, err := os.ReadDir(fs.path(rel))
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(dirEntries))
	for _, e := range dirEntries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}
