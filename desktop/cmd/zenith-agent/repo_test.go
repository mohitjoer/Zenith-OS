package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeCheckout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"desktop/cmd", "docs", "release"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(root, "desktop/cmd/main.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(root, "docs/agent.md"), []byte("# "+displayName+"\n\nbody"), 0o644)
	return root
}

func TestRepoIndexDocLayoutAndDocs(t *testing.T) {
	doc, err := RepoIndexDoc(makeCheckout(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc, "`desktop/`") {
		t.Fatalf("layout table missing desktop/ row:\n%s", doc)
	}
	if !strings.Contains(doc, "`docs/agent.md` "+displayName) {
		t.Fatalf("docs list missing first-heading title:\n%s", doc)
	}
}

func TestRepoIndexDocRejectsMissingRoot(t *testing.T) {
	if _, err := RepoIndexDoc(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error for missing root")
	}
}

func TestWriteRepoVaultDocPriority(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(envName("VAULT"), filepath.Join(t.TempDir(), "vault"))
	if err := EnsureVault(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(VaultDir(), "zenith-repo.md")

	// No checkout, no snapshot: stub.
	t.Setenv(envName("REPO"), "")
	t.Setenv(envName("SHIPPED"), filepath.Join(t.TempDir(), "absent.md"))
	if err := writeRepoVaultDoc(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFileOrEmpty(dest), "No repo snapshot found") {
		t.Fatal("expected stub body without snapshot")
	}

	// Shipped snapshot wins over stub.
	snap := filepath.Join(t.TempDir(), "snap.md")
	os.WriteFile(snap, []byte("## Layout\nsnapshot-body"), 0o644)
	t.Setenv(envName("SHIPPED"), snap)
	if err := writeRepoVaultDoc(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFileOrEmpty(dest), "snapshot-body") {
		t.Fatal("expected shipped snapshot body")
	}

	// A live checkout wins over the snapshot.
	t.Setenv(envName("REPO"), makeCheckout(t))
	if err := writeRepoVaultDoc(); err != nil {
		t.Fatal(err)
	}
	body := readFileOrEmpty(dest)
	if !strings.Contains(body, "`desktop/`") || strings.Contains(body, "snapshot-body") {
		t.Fatal("expected live checkout to replace snapshot body")
	}
}
