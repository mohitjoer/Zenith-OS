package overlay

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestUpdatePreservesAUserEdit is M2's exit criterion, first half: an update lays
// down the base, and what the user wrote survives it.
func TestUpdatePreservesAUserEdit(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base-a-v1\n")
	m.baseFile("b.conf", "base-b-v1\n")
	m.edit("a.conf", "mine\n")

	var fetched bool
	err := Update(m.paths, nil, func() error {
		fetched = true
		// The fetch is where a real update would replace the base; here it does
		// so explicitly so the preserved copy is meaningful.
		m.baseFile("a.conf", "base-a-v2\n")
		m.baseFile("b.conf", "base-b-v2\n")
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !fetched {
		t.Fatal("the fetch step did not run")
	}

	if got := m.read(filepath.Join(m.paths.Config, "a.conf")); got != "mine\n" {
		t.Errorf("the user edit did not survive the update: %q", got)
	}
	// The untouched file picked up the new base.
	if got := m.read(filepath.Join(m.paths.Config, "b.conf")); got != "base-b-v2\n" {
		t.Errorf("the untouched file is %q, want the new base", got)
	}
}

// TestUpdateSnapshotsBeforeAnythingElse checks the order. The snapshot is the
// only step that can fail cheaply, and a change made during an update has to be
// recoverable.
func TestUpdateSnapshotsBeforeAnythingElse(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base\n")

	var order []string
	err := Update(m.paths,
		func(Paths) error {
			order = append(order, "snapshot")
			return nil
		},
		func() error {
			order = append(order, "fetch")
			return nil
		})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(order) != 2 || order[0] != "snapshot" || order[1] != "fetch" {
		t.Errorf("step order was %v, want snapshot then fetch", order)
	}
}

// TestAFailedFetchLeavesTheMachineRecoverable is the important failure case: a
// failed fetch must not leave a revision record pointing nowhere.
func TestAFailedFetchLeavesTheMachineRecoverable(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base-v1\n")

	// One successful update so there is a preserved base to go back to.
	if err := Update(m.paths, nil, func() error { return nil }); err != nil {
		t.Fatalf("first Update: %v", err)
	}

	err := Update(m.paths, nil, func() error { return errors.New("apt failed") })
	if err == nil {
		t.Fatal("a failed fetch was reported as success")
	}

	// The config tree still holds v1, because materialization never ran.
	if got := m.read(filepath.Join(m.paths.Config, "a.conf")); got != "base-v1\n" {
		t.Errorf("after a failed update the config holds %q", got)
	}
}

// TestRollbackRestoresThePreviousBase is the second half of the exit criterion.
func TestRollbackRestoresThePreviousBase(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base-v1\n")
	m.baseFile("gone-in-v2.conf", "only-in-v1\n")
	m.edit("a.conf", "mine\n")

	if err := Update(m.paths, nil, func() error { return nil }); err != nil {
		t.Fatalf("first Update: %v", err)
	}

	// The second update replaces the base wholesale.
	if err := Update(m.paths, nil, func() error {
		if err := os.RemoveAll(m.paths.Base); err != nil {
			return err
		}
		m.baseFile("a.conf", "base-v2\n")
		m.baseFile("new-in-v2.conf", "only-in-v2\n")
		return nil
	}); err != nil {
		t.Fatalf("second Update: %v", err)
	}

	if got := m.read(filepath.Join(m.paths.Config, "a.conf")); got != "mine\n" {
		t.Fatalf("setup: the edit did not survive: %q", got)
	}

	restored, err := Rollback(m.paths)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if restored.ID == "" {
		t.Error("Rollback restored nothing and reported no revision")
	}

	if got := m.read(filepath.Join(m.paths.Base, "a.conf")); got != "base-v1\n" {
		t.Errorf("the base is %q, want the v1 content", got)
	}
	// Restoring has to remove what the newer base added. Restoring over the top
	// would leave new-in-v2.conf behind, and that is not a rollback.
	if _, err := os.Stat(filepath.Join(m.paths.Base, "new-in-v2.conf")); err == nil {
		t.Error("rollback left a file from the newer base behind")
	}
	if _, err := os.Stat(filepath.Join(m.paths.Base, "gone-in-v2.conf")); err != nil {
		t.Error("rollback did not restore a file the older base shipped")
	}

	if got := m.read(filepath.Join(m.paths.Config, "a.conf")); got != "mine\n" {
		t.Errorf("the user edit did not survive the rollback: %q", got)
	}
}

// TestRollbackWithNoHistoryRefuses checks it does not invent one.
func TestRollbackWithNoHistoryRefuses(t *testing.T) {
	m := newMachine(t)
	if _, err := Rollback(m.paths); err == nil {
		t.Error("rollback succeeded with no recorded revision")
	}
}

func TestRollbackOfTheFirstRevisionRefuses(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base\n")
	if err := Update(m.paths, nil, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := Rollback(m.paths); err == nil {
		t.Error("rollback succeeded with only one recorded revision")
	}
}

// TestRevisionsAreListed checks the history a rollback walks.
func TestRevisionsAreListed(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base\n")
	for i := 0; i < 3; i++ {
		if err := Update(m.paths, nil, func() error { return nil }); err != nil {
			t.Fatalf("update %d: %v", i, err)
		}
	}
	revisions, err := Revisions(m.paths)
	if err != nil {
		t.Fatalf("Revisions: %v", err)
	}
	if len(revisions) != 3 {
		t.Fatalf("Revisions returned %d, want 3", len(revisions))
	}
}

// TestStatusSummarisesWithoutChangingAnything covers the one-glance command.
func TestStatusSummarisesWithoutChangingAnything(t *testing.T) {
	m := newMachine(t)
	m.baseFile("a.conf", "base\n")
	m.baseFile("b.conf", "base\n")
	m.edit("a.conf", "mine\n")

	if err := Update(m.paths, nil, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	m.write(filepath.Join(m.paths.Config, "b.conf"), "tampered\n")

	status, err := StatusOf(m.paths)
	if err != nil {
		t.Fatalf("StatusOf: %v", err)
	}
	if status.Revision == "" {
		t.Error("StatusOf reports no revision after an update")
	}
	if status.Edits != 1 {
		t.Errorf("StatusOf reports %d edits, want 1", status.Edits)
	}
	if status.Stale != 1 {
		t.Errorf("StatusOf reports %d stale files, want 1", status.Stale)
	}
}
