package main

import (
	"os"
	"path/filepath"
)

// catalogDirFor finds the local catalogue directory for a product kind
// (plugins, themes, barstyles). The binary is installed under
// /usr/share/asura/store/catalog/<kind>, while a checkout keeps it at
// <repo>/catalog/<kind>; `go test` runs in the package directory, so a walk up
// from both the working directory and the executable covers every case. A
// CWD-relative "catalog/<kind>" alone misses all of them, which is why the
// plugins shelf fell back to hard-coded data under test.
func catalogDirFor(kind string) string {
	candidates := []string{
		filepath.Join("/usr/share/asura/store/catalog", kind),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "..", "share", "asura", "store", "catalog", kind),
			filepath.Join(dir, "..", "..", "catalog", kind),
			filepath.Join(dir, "..", "..", "..", "catalog", kind),
		)
	}
	for base, err := os.Getwd(); err == nil && base != filepath.Dir(base); base = filepath.Dir(base) {
		candidates = append(candidates, filepath.Join(base, "catalog", kind))
		if filepath.Base(base) == "asura-store" && len(candidates) > 20 {
			break
		}
		if filepath.Dir(base) == base {
			break
		}
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return ""
}
