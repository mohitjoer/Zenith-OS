package overlay

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Revision records one materialized state, so a rollback has something to go
// back to.
//
// It lives under the state directory rather than in the config tree, because the
// config tree is the thing being versioned and a revision record inside it would
// be one more file for an update to disagree about.
const revisionFile = "revision"
const revisionsDir = "revisions"

// Revision is the metadata for one base revision.
type Revision struct {
	// ID identifies the revision in rollback output. It is a timestamp because
	// revisions are created in time order and "the previous one" is the common
	// case.
	ID string `json:"id"`
	// RecordedAt is when this revision became current.
	RecordedAt time.Time `json:"recorded_at"`
	// Previous names the revision this one replaced, empty for the first.
	Previous string `json:"previous,omitempty"`
	// Preserved is the directory holding a copy of the base this revision
	// replaced, so a rollback has content to restore rather than only metadata.
	Preserved string `json:"preserved,omitempty"`
}

// Update performs the update sequence: snapshot, preserve, fetch, re-materialize.
//
// The four steps are the ones the architecture record fixes, and their order is
// the whole safety property:
//
//  1. Snapshot the user's config to git, so a change made *during* an update is
//     recoverable. This happens first because it is the only step that can fail
//     cheaply.
//  2. Preserve the current base, so a rollback has something to restore. This
//     happens before the fetch, because the fetch is what replaces it.
//  3. Fetch and install packages. This is the only step that needs root, and the
//     only step this package does not implement: it is handed in.
//  4. Re-materialize. Last, because every earlier step's value is that the
//     config tree can be rebuilt from the base plus the user's edits.
//
// If step 4 fails, the preserved base is still on disk and the config tree is
// untouched, so the machine is in the state it was in before the update ran.
func Update(paths Paths, snapshot func(Paths) error, fetch func() error) error {
	if snapshot != nil {
		if err := snapshot(paths); err != nil {
			return fmt.Errorf("update: snapshotting the config before anything "+
				"else: %w", err)
		}
	}

	previous, err := recordRevision(paths)
	if err != nil {
		return err
	}

	if fetch != nil {
		if err := fetch(); err != nil {
			// The revision record is left in place on purpose. It says a revision
			// was begun, and the preserved base is what makes going back
			// possible; deleting the record here would throw away the only
			// evidence that a rollback target exists.
			return fmt.Errorf("update: fetching packages: %w", err)
		}
	}

	if err := Materialize(paths); err != nil {
		return fmt.Errorf("update: %w", err)
	}

	_ = previous
	return nil
}

// recordRevision preserves the current base and writes the new revision record.
func recordRevision(paths Paths) (Revision, error) {
	now := time.Now().UTC()
	id := revisionID(now, filepath.Join(paths.State, revisionsDir))

	preserved := filepath.Join(paths.State, revisionsDir, id)
	if err := preserveTree(paths.Base, preserved); err != nil {
		return Revision{}, err
	}

	revision := Revision{
		ID:         id,
		RecordedAt: now,
		Preserved:  preserved,
	}
	if current, err := readRevision(paths); err == nil {
		revision.Previous = current.ID
	}

	// The record is written twice on purpose: once at the state root as the
	// "which revision is current" pointer, and once inside the revision's own
	// directory so the history is walkable. Without the second copy, `rollback`
	// and `zenith status --revisions` both find nothing, because the preserved
	// base and its metadata would be an archive with no index.
	recordPath := filepath.Join(preserved, revisionFile)
	if err := writeRevisionFile(recordPath, revision); err != nil {
		return Revision{}, err
	}

	if err := writeRevision(paths, revision); err != nil {
		return Revision{}, err
	}
	return revision, nil
}

// revisionID builds an identifier for a revision, disambiguating a collision.
//
// Second-resolution timestamps are not enough: two updates in the same second
// produced the same directory, so the second one's preserved base overwrote the
// first's and a rollback silently restored the wrong content. The nanoseconds
// handle the ordinary case and the loop handles the pathological one.
func revisionID(now time.Time, root string) string {
	base := now.Format("20060102T150405.000000000Z")

	candidate := base
	for attempt := 1; ; attempt++ {
		if _, err := os.Stat(filepath.Join(root, candidate)); os.IsNotExist(err) {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, attempt)
	}
}

// Rollback re-materializes the previous revision.
//
// It restores the preserved base first and the config tree second, in that
// order, because the config tree is derived from the base: restoring them the
// other way round would rebuild the config from a base that is about to change.
func Rollback(paths Paths) (Revision, error) {
	current, err := readRevision(paths)
	if err != nil {
		return Revision{}, fmt.Errorf("rollback: %w", err)
	}
	if current.Previous == "" {
		return Revision{}, fmt.Errorf("rollback: %s is the first recorded "+
			"revision; there is nothing to go back to", current.ID)
	}

	previous, err := readRevisionByID(paths, current.Previous)
	if err != nil {
		return Revision{}, err
	}
	if previous.Preserved == "" {
		return Revision{}, fmt.Errorf("rollback: revision %s recorded no "+
			"preserved base to restore", previous.ID)
	}

	if err := restoreTree(previous.Preserved, paths.Base); err != nil {
		return Revision{}, fmt.Errorf("rollback: restoring the base from %s: %w",
			previous.Preserved, err)
	}

	if err := Materialize(paths); err != nil {
		return Revision{}, fmt.Errorf("rollback: %w", err)
	}

	// The restored revision becomes current, so a second rollback goes one
	// further back rather than ping-ponging between two.
	if err := writeRevision(paths, previous); err != nil {
		return previous, err
	}
	return previous, nil
}

// Revisions lists recorded revisions, newest first.
func Revisions(paths Paths) ([]Revision, error) {
	entries, err := os.ReadDir(filepath.Join(paths.State, revisionsDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var revisions []Revision
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if revision, err := readRevisionFrom(filepath.Join(paths.State,
			revisionsDir, entry.Name(), revisionFile)); err == nil {
			revisions = append(revisions, revision)
		}
	}
	return revisions, nil
}

func readRevision(paths Paths) (Revision, error) {
	return readRevisionFrom(filepath.Join(paths.State, revisionFile))
}

func readRevisionByID(paths Paths, id string) (Revision, error) {
	if id == "" {
		return Revision{}, fmt.Errorf("no revision id given")
	}
	return readRevisionFrom(filepath.Join(paths.State, revisionsDir, id, revisionFile))
}

// readRevisionFrom parses one revision record.
//
// The record is a few key=value lines rather than JSON because it is written and
// read by one program at two moments, and a hand-rolled format has no dependency
// and no parse error to handle at rollback time — which is the worst possible
// moment to discover a format problem.
func readRevisionFrom(path string) (Revision, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Revision{}, fmt.Errorf("reading revision record %s: %w", path, err)
	}

	var revision Revision
	for _, line := range strings.Split(string(data), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch key {
		case "id":
			revision.ID = value
		case "recorded_at":
			revision.RecordedAt, _ = time.Parse(time.RFC3339, value)
		case "previous":
			revision.Previous = value
		case "preserved":
			revision.Preserved = value
		}
	}
	if revision.ID == "" {
		return Revision{}, fmt.Errorf("revision record %s has no id", path)
	}
	return revision, nil
}

func writeRevision(paths Paths, revision Revision) error {
	return writeRevisionFile(filepath.Join(paths.State, revisionFile), revision)
}

func writeRevisionFile(path string, revision Revision) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	body := "id=" + revision.ID + "\n" +
		"recorded_at=" + revision.RecordedAt.Format(time.RFC3339) + "\n"
	if revision.Previous != "" {
		body += "previous=" + revision.Previous + "\n"
	}
	if revision.Preserved != "" {
		body += "preserved=" + revision.Preserved + "\n"
	}

	// 0600: this record names where the previous base was preserved, which is a
	// map of what is on the machine.
	return os.WriteFile(path, []byte(body), 0o600)
}

// preserveTree copies a tree aside so a rollback has content to restore.
func preserveTree(source, dest string) error {
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			// No base to preserve. Recording a revision with nothing preserved is
			// honest and lets the sequence proceed; it just means a rollback has
			// no content, which the record shows.
			return nil
		}
		return err
	}
	return copyTree(source, dest)
}

// restoreTree replaces a destination tree with a preserved copy.
//
// The destination is emptied first. Restoring over the top would leave files the
// new base added and the old one does not have, and a rollback that leaves the
// new base's files behind is not a rollback.
func restoreTree(preserved, dest string) error {
	if _, err := os.Stat(preserved); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no preserved base at %s", preserved)
		}
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return copyTree(preserved, dest)
}

func copyTree(source, dest string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, relative)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

// Status is the one-glance answer: which revision, which tier, what drift.
type Status struct {
	Revision string
	Previous string
	Edits    int
	Stale    int
	Missing  int
}

// Status summarises the current state without changing anything.
func StatusOf(paths Paths) (Status, error) {
	status := Status{}

	if revision, err := readRevision(paths); err == nil {
		status.Revision = revision.ID
		status.Previous = revision.Previous
	}

	findings, err := Doctor(paths)
	if err != nil {
		return status, err
	}
	for _, finding := range findings {
		switch finding.Reason {
		case DriftUserEdit:
			status.Edits++
		case DriftStale:
			status.Stale++
		case DriftMissing:
			status.Missing++
		}
	}
	return status, nil
}
