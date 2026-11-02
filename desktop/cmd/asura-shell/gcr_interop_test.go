package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestSecretExchangeVsGcr proves the Go exchange is wire-compatible with the real
// gcr library in the exact keyring direction: the Go side begins (as the
// prompter), gcr (the reference client built from /tmp/gcrx.c) replies with its
// public key, the Go side seals a secret, and gcr must decrypt it byte-for-byte.
// Gated on GCRX_BIN (the compiled reference), so it skips in CI.
func TestSecretExchangeVsGcr(t *testing.T) {
	bin := os.Getenv("GCRX_BIN")
	if bin == "" {
		t.Skip("set GCRX_BIN to the compiled gcr reference to run")
	}
	dir := t.TempDir()
	beginIn := filepath.Join(dir, "begin")
	pubOut := filepath.Join(dir, "pub")
	sealedIn := filepath.Join(dir, "sealed")
	secretOut := filepath.Join(dir, "secret")

	ex := newSecretExchange()
	begin, err := ex.begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(beginIn, []byte(begin), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, beginIn, pubOut, sealedIn, secretOut)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	var pub []byte
	for i := 0; i < 200; i++ {
		if d, e := os.ReadFile(pubOut); e == nil && len(d) > 0 {
			pub = d
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if pub == nil {
		_ = cmd.Process.Kill()
		t.Fatal("gcr produced no public key")
	}
	if _, err := ex.receive(string(pub)); err != nil {
		t.Fatalf("receive gcr public: %v", err)
	}

	want := "GO-SECRET-\U0001F510-xyz"
	sealed, err := ex.send([]byte(want))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sealedIn, []byte(sealed), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("gcr reference failed: %v", err)
	}
	got, err := os.ReadFile(secretOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("gcr decrypted %q, want %q", got, want)
	}
}
