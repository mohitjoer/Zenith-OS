package main

import (
	"testing"
	"time"
)

// The lock is the guard that keeps a second daemon from supervising a second
// desktop when the socket file is missing or was cleaned away.
func TestHoldDaemonLockIsExclusive(t *testing.T) {
	path := t.TempDir() + "/zenith-shell.sock.lock"
	first, err := holdDaemonLock(path)
	if err != nil {
		t.Fatalf("first daemon could not take the lock: %v", err)
	}
	start := time.Now()
	if _, err := holdDaemonLock(path); err == nil {
		t.Fatal("a second daemon took the lock while the first held it")
	}
	if waited := time.Since(start); waited < 2*time.Second {
		t.Fatalf("gave up after %v; it should retry for about 3s so a take-over can win", waited)
	}

	// releasing it (as process exit does) lets the next daemon in.
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := holdDaemonLock(path)
	if err != nil {
		t.Fatalf("lock not released with the file: %v", err)
	}
	second.Close()
}
