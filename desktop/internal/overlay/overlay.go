// Package overlay implements Zenith's materialization model.
//
// The whole model is one rule: a user's override always wins at its mirrored
// path, and no update can ever clobber it. Everything else here exists to make
// that rule true by construction rather than by remembering.
//
// The paths are the ones the architecture record fixes:
//
//	/usr/share/zenith/            the packaged base, replaced on update
//	/etc/zenith/                  machine-level config, replaced on update
//	~/.config/zenith/             the materialized base, the user's copy
//	~/.config/zenith/user_edits/  the user's overlay; wins at the mirrored path
//
// Every path this package touches is taken from a struct rather than read from
// the process environment, so a test can run the real code against a temporary
// directory and an installed binary resolves the same paths it does.
package overlay

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Paths is the set of roots the model is built on.
type Paths struct {
	// Base is the packaged base, /usr/share/zenith by default.
	Base string
	// Etc is machine-level config, /etc/zenith.
	Etc string
	// Config is the materialized copy the user reads, ~/.config/zenith.
	Config string
	// Edits is the user's overlay inside Config.
	Edits string
	// State holds revisions and snapshots, ~/.local/state/zenith.
	State string
}

// DefaultPaths returns the installed layout, rooted at a user's home directory.
//
// The home directory is passed in rather than read from the environment so a
// caller — a test, or a root invocation with a different home — controls it.
func DefaultPaths(home string) Paths {
	return Paths{
		Base:   "/usr/share/zenith",
		Etc:    "/etc/zenith",
		Config: filepath.Join(home, ".config", "zenith"),
		Edits:  filepath.Join(home, ".config", "zenith", "user_edits"),
		State:  filepath.Join(home, ".local", "state", "zenith"),
	}
}

// Source says where a materialized file's content came from.
type Source string

const (
	// SourceUserEdit is the user's overlay. It wins, always.
	SourceUserEdit Source = "user_edit"
	// SourceMaterialized is the base as previously materialized.
	SourceMaterialized Source = "materialized"
	// SourceBase is the packaged base.
	SourceBase Source = "base"
	// SourceNone means no managed file exists at that path.
	SourceNone Source = "none"
)

// Resolution is what a path resolves to.
type Resolution struct {
	Path   string
	Source Source
}

// Entry is one file in the overlay model.
type Entry struct {
	// Path is the mirrored path relative to the base root, e.g. "ghostty/config".
	Path string
	// Source is where the live content comes from.
	Source Source
}

// EditsDir is the overlay's directory name inside the materialized config.
//
// It is a constant rather than a computed path so that the exclusion is one
// value: a materialization that walked into user_edits/ would write the user's
// overlay with base content, which is the single clobber the model forbids.
const EditsDir = "user_edits"

// materializeInto lays one source tree into a destination, honouring the
// overlay.
//
// A path with a user edit is skipped entirely — not copied, not truncated, not
// opened for writing. Opening it "just to check" is how a tool that promises not
// to clobber ends up clobbering on a full disk.
func materializeInto(sourceRoot, destRoot, editsRoot string) error {
	return filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}

		// The overlay lives inside the destination. Never write through it.
		if isEditPath(relative, editsRoot, destRoot) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if entry.IsDir() {
			return os.MkdirAll(path, 0o755)
		}

		if !entry.Type().IsRegular() {
			// A symlink or device node in the base is not content this model
			// materializes. Skipping it is better than following it, which could
			// write outside the tree entirely.
			return nil
		}

		// A user edit wins *at the mirrored path*, which means its content is
		// written there — not that the path is left alone. Applications read
		// ~/.config/zenith/<path>; they do not know user_edits/ exists, so a
		// model that left the mirrored path absent would resolve the user's edit
		// in the one place nothing reads. user_edits/ is the record of who
		// changed it and when, and it is what `reset` and `doctor` consult.
		source := path
		if _, err := os.Lstat(filepath.Join(editsRoot, relative)); err == nil {
			source = filepath.Join(editsRoot, relative)
		}

		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		target := filepath.Join(destRoot, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// isEditPath reports whether a relative path lies inside the user's overlay.
func isEditPath(relative, editsRoot, destRoot string) bool {
	if relative == EditsDir || strings.HasPrefix(relative, EditsDir+string(filepath.Separator)) {
		return true
	}
	_ = editsRoot
	_ = destRoot
	return false
}

// Materialize lays the base into the user's config, preserving every user edit.
//
// This is the operation an update performs on the config tree, and the one whose
// correctness the whole model rests on: after it, the user sees the base plus
// their own changes, and nothing they wrote has been read for writing.
func Materialize(paths Paths) error {
	if err := os.MkdirAll(paths.Edits, 0o755); err != nil {
		return fmt.Errorf("overlay: creating the overlay directory: %w", err)
	}
	if err := materializeInto(paths.Base, paths.Config, paths.Edits); err != nil {
		return fmt.Errorf("overlay: materializing %s into %s: %w",
			paths.Base, paths.Config, err)
	}
	if err := pruneOrphans(paths); err != nil {
		return fmt.Errorf("overlay: pruning orphans: %w", err)
	}
	return nil
}

// pruneOrphans removes materialized files that the base no longer ships.
func pruneOrphans(paths Paths) error {
	findings, err := Doctor(paths)
	if err != nil {
		return err
	}
	for _, finding := range findings {
		if finding.Reason == DriftOrphan {
			target := filepath.Join(paths.Config, finding.Path)
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return err
			}
			parent := filepath.Dir(target)
			for parent != paths.Config && parent != "." && parent != "/" {
				if err := os.Remove(parent); err != nil {
					break
				}
				parent = filepath.Dir(parent)
			}
		}
	}
	return nil
}

// Resolve reports where a mirrored path's live content comes from.
func Resolve(paths Paths, relative string) Resolution {
	if _, err := os.Lstat(filepath.Join(paths.Edits, relative)); err == nil {
		return Resolution{Path: relative, Source: SourceUserEdit}
	}
	if _, err := os.Lstat(filepath.Join(paths.Config, relative)); err == nil {
		return Resolution{Path: relative, Source: SourceMaterialized}
	}
	if _, err := os.Lstat(filepath.Join(paths.Base, relative)); err == nil {
		return Resolution{Path: relative, Source: SourceBase}
	}
	return Resolution{Path: relative, Source: SourceNone}
}

// Reset drops one overlay file back to the base.
//
// It removes the user edit and re-lays the base at that path. That is the only
// command in the set that destroys a user's change, which is why it takes one
// explicit path and never a directory.
func Reset(paths Paths, relative string) error {
	clean, err := cleanRelative(relative)
	if err != nil {
		return err
	}

	edit := filepath.Join(paths.Edits, clean)
	if _, err := os.Lstat(edit); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("overlay: %s has no user edit to reset", relative)
		}
		return err
	}
	if err := os.Remove(edit); err != nil {
		return err
	}

	// Re-lay the base at that one path, which now has nothing overriding it.
	base := filepath.Join(paths.Base, clean)
	data, err := os.ReadFile(base)
	if err != nil {
		if os.IsNotExist(err) {
			// The user removed an override for something the base no longer
			// ships. That is a legitimate state: the materialized copy stays and
			// the file is now unmanaged.
			return nil
		}
		return err
	}
	target := filepath.Join(paths.Config, clean)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}

// Drift is one difference between the base and what is materialized.
type Drift struct {
	Path   string
	Reason string
}

// Reasons a materialized file can differ from the base.
const (
	// DriftUserEdit is not drift at all: it is the model working. It is reported
	// so `doctor` can show what would be lost by a reset, not so it can be
	// treated as a problem.
	DriftUserEdit = "user edit"
	// DriftStale means the materialized copy no longer matches the base.
	DriftStale = "stale"
	// DriftMissing means the base ships a file the config tree lacks.
	DriftMissing = "missing"
	// DriftOrphan means the config tree holds a file the base no longer ships.
	DriftOrphan = "orphan"
)

// Doctor reports how the config tree differs from the base.
//
// It opens nothing for writing. A diagnostic that repairs is not a diagnostic,
// and the moment it repairs, a user cannot tell what was wrong before they
// changed it.
func Doctor(paths Paths) ([]Drift, error) {
	var findings []Drift

	seen := map[string]bool{}

	// Everything the base ships.
	err := filepath.WalkDir(paths.Base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(paths.Base, path)
		if err != nil {
			return err
		}
		seen[relative] = true

		switch Resolve(paths, relative).Source {
		case SourceUserEdit:
			findings = append(findings, Drift{Path: relative, Reason: DriftUserEdit})
		case SourceMaterialized:
			same, err := sameContent(path, filepath.Join(paths.Config, relative))
			if err != nil {
				return err
			}
			if !same {
				findings = append(findings, Drift{Path: relative, Reason: DriftStale})
			}
		default:
			findings = append(findings, Drift{Path: relative, Reason: DriftMissing})
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Everything the config tree holds that the base does not.
	err = filepath.WalkDir(paths.Config, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") || entry.Name() == EditsDir ||
				entry.Name() == "current" || entry.Name() == "agent" {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(paths.Config, path)
		if err != nil {
			return err
		}
		if isEditPath(relative, paths.Edits, paths.Config) || isUserConfig(relative) {
			return nil
		}
		if seen[relative] {
			return nil
		}
		findings = append(findings, Drift{Path: relative, Reason: DriftOrphan})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Reason < findings[j].Reason
	})
	return findings, nil
}

func sameContent(a, b string) (bool, error) {
	first, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	second, err := os.ReadFile(b)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return string(first) == string(second), nil
}

// cleanRelative validates a path the user typed.
//
// A path that escapes the tree is rejected rather than normalized. An update
// tool that will follow "../" is a tool that can be told to rewrite a file
// outside the directory it owns, and the check costs one function.
func cleanRelative(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("overlay: empty path")
	}
	cleaned := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(cleaned) || cleaned == ".." ||
		strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("overlay: %q is outside the overlay; paths are "+
			"relative to %s", path, EditsDir)
	}
	if cleaned == EditsDir || strings.HasPrefix(cleaned, EditsDir+string(filepath.Separator)) {
		return "", fmt.Errorf("overlay: %q names the overlay itself", path)
	}
	return cleaned, nil
}

// isUserConfig reports whether a path in paths.Config is user state / config rather than a materialized base file.
func isUserConfig(relative string) bool {
	parts := strings.Split(relative, string(filepath.Separator))
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	if parts[0] == "current" || parts[0] == "agent" {
		return true
	}
	if len(parts) == 1 {
		ext := filepath.Ext(relative)
		if ext == ".json" || ext == ".toml" || ext == ".lock" {
			return true
		}
	}
	return false
}
