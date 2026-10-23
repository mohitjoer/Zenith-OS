package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAptChannel(t *testing.T) {
	tests := []struct {
		name    string
		sources string // empty: no file at all
		want    string
		wantErr string
	}{
		{"stable", "Types: deb\nURIs: https://x\nSuites: stable\nComponents: main\n", "stable", ""},
		{"a comment, odd case and spacing", "# Zenith OS\nsuites:   testing  \nComponents: main\n", "testing", ""},
		{"several suites: the first is the channel", "Suites: stable stable-updates\n", "stable", ""},
		{"no Suites line", "Types: deb\nURIs: https://x\n", "", "names no suite"},
		{"not configured", "", "", "no Zenith repository is configured"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.sources != "" {
				path := filepath.Join(root, aptSources)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tc.sources), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := aptChannel(root)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got %q, %v; want an error with %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

// `set` is not built; it must say so rather than succeed.
func TestChannelOnlyHasGet(t *testing.T) {
	if code := zenithChannel([]string{"set", "testing"}); code != exitUsage {
		t.Errorf("zenith channel set exits %d, want %d", code, exitUsage)
	}
	if code := zenithChannel(nil); code != exitUsage {
		t.Errorf("zenith channel with no verb exits %d, want %d", code, exitUsage)
	}
}
