package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRollbackUsage(t *testing.T) {
	// Running with invalid flag returns exitUsage
	if code := asuraRollback([]string{"--invalid-flag"}); code != exitUsage {
		t.Errorf("asuraRollback(--invalid-flag) = %d, want %d", code, exitUsage)
	}
}

func TestRollbackListEmpty(t *testing.T) {
	temp := t.TempDir()
	code := asuraRollback([]string{"--root", temp, "--list"})
	if code != exitOK {
		t.Errorf("asuraRollback(--list) = %d, want %d", code, exitOK)
	}
}

func TestRollbackSystemList(t *testing.T) {
	code := asuraRollback([]string{"--system"})
	if code != exitOK {
		t.Errorf("asuraRollback(--system) = %d, want %d", code, exitOK)
	}
}

func TestRollbackNonExistentSystemSnapshot(t *testing.T) {
	code := asuraRollback([]string{"--system", "non-existent-snapshot-12345"})
	if code != exitFail {
		t.Errorf("asuraRollback(--system non-existent) = %d, want %d", code, exitFail)
	}
}

func TestIsBtrfsSnapshotHelper(t *testing.T) {
	temp := t.TempDir()
	snap := filepath.Join(temp, "snap1")
	if isBtrfsSnapshot(snap) {
		t.Errorf("isBtrfsSnapshot before mkdir reported true")
	}
	if err := os.Mkdir(snap, 0755); err != nil {
		t.Fatal(err)
	}
	if !isBtrfsSnapshot(snap) {
		t.Errorf("isBtrfsSnapshot after mkdir reported false")
	}
}
