package overlay

import (
	"path/filepath"
	"testing"
)

// TestTwoUpdatesInTheSameSecondGetDistinctRevisions is a bug this package
// actually had.
//
// Revision identifiers were second-resolution timestamps, so two updates inside
// one second produced the same directory. The second one's preserved base
// overwrote the first's, and a rollback then restored the wrong content while
// reporting success — the worst failure shape available: a rollback that claims
// to have worked and did not.
func TestTwoUpdatesInTheSameSecondGetDistinctRevisions(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base\n")

	for i := 0; i < 5; i++ {
		if err := Update(m.paths, nil, func() error { return nil }); err != nil {
			t.Fatalf("update %d: %v", i, err)
		}
		revisions, err := Revisions(m.paths)
		if err != nil {
			t.Fatalf("Revisions: %v", err)
		}
		if len(revisions) != i+1 {
			t.Fatalf("after %d updates there are %d revisions; two collided",
				i+1, len(revisions))
		}
		// Within one result, ids must be distinct. Across calls they are not:
		// every call returns the whole history, so an earlier revision appears
		// in each later one by design.
		seen := map[string]bool{}
		for _, revision := range revisions {
			if seen[revision.ID] {
				t.Fatalf("revision id %q appeared twice in one history: %v",
					revision.ID, revisions)
			}
			seen[revision.ID] = true
		}
	}
}

// TestEachRevisionKeepsItsOwnRecord checks the history is walkable.
//
// The record is written twice on purpose: once as the "current revision" pointer
// and once inside the revision's own directory. Without the second copy the
// preserved base is an archive with no index, and rollback finds nothing.
func TestEachRevisionKeepsItsOwnRecord(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base\n")

	if err := Update(m.paths, nil, func() error { return nil }); err != nil {
		t.Fatal(err)
	}

	revisions, err := Revisions(m.paths)
	if err != nil {
		t.Fatalf("Revisions: %v", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("got %d revisions, want 1", len(revisions))
	}

	// The walkable copy has to resolve on its own, not through the current
	// pointer, or reading history depends on which revision happens to be live.
	direct := filepath.Join(m.paths.State, revisionsDir, revisions[0].ID, revisionFile)
	if _, err := readRevisionFrom(direct); err != nil {
		t.Errorf("the revision's own record is not readable: %v", err)
	}
}
