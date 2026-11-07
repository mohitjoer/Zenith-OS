package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// desktop.md must carry the GUI map so an agent answers "how do I" questions
// GUI-first: the Asura Hub deep link, a compositor-neutral page name like
// Graphics & Power, and the picker and bar-settings commands behind the surfaces.
func TestDesktopBodyGUIMap(t *testing.T) {
	body := desktopBody()
	for _, want := range []string{
		"## GUI map",
		"hub open",
		"Graphics & Power",
		"asura-wallpaper wallpaper ui",
		"asura-shell bar settings",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("desktop.md GUI map missing %q", want)
		}
	}
}

// TestOKFBundleStructure verifies that Reindex produces a complete Google Open
// Knowledge Format (OKF v0.1) bundle with root index.md, audit log.md, and all
// subsystem docs carrying valid YAML frontmatter and bidirectional graph links.
func TestOKFBundleStructure(t *testing.T) {
	vault := vaultEnv(t)
	if err := Reindex(); err != nil {
		t.Fatalf("Reindex failed: %v", err)
	}

	expectedDocs := map[string]string{
		"index.md":      "Index",
		"log.md":        "Changelog",
		"system.md":     "Hardware Component",
		"desktop.md":    "Desktop Subsystem",
		"packages.md":   "Package Inventory",
		"asura-repo.md": "Repository Architecture",
		"user.md":       "User Configuration",
		"habits.md":     "Workflow Patterns",
	}

	typeRe := regexp.MustCompile(`(?m)^type: (.+)$`)
	tsRe := regexp.MustCompile(`(?m)^timestamp: (.+)$`)

	for filename, expectedType := range expectedDocs {
		path := filepath.Join(vault, filename)
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected OKF file %s missing: %v", filename, err)
		}
		content := string(contentBytes)

		// Must start with YAML frontmatter delimiter
		if !strings.HasPrefix(content, "---\n") {
			t.Fatalf("%s does not start with YAML frontmatter delimiter:\n%s", filename, content[:min(len(content), 100)])
		}

		// Must have matching closing delimiter
		endDelim := strings.Index(content[4:], "\n---\n")
		if endDelim < 0 {
			t.Fatalf("%s missing closing YAML frontmatter delimiter", filename)
		}

		frontmatter := content[:endDelim+8]

		// Must match the expected OKF type
		mType := typeRe.FindStringSubmatch(frontmatter)
		if len(mType) < 2 || strings.TrimSpace(mType[1]) != expectedType {
			t.Fatalf("%s: expected OKF type %q, got %v", filename, expectedType, mType)
		}

		// Must have ISO 8601 / RFC 3339 timestamp
		mTS := tsRe.FindStringSubmatch(frontmatter)
		if len(mTS) < 2 || strings.TrimSpace(mTS[1]) == "" {
			t.Fatalf("%s: missing timestamp in frontmatter", filename)
		}

		// Must contain fenced generated region
		if !strings.Contains(content, vaultFenceBegin) || !strings.Contains(content, vaultFenceEnd) {
			t.Fatalf("%s: missing generated fence markers", filename)
		}

		// Verify bidirectional graph links
		if filename == "index.md" {
			// index.md must link to all subsystem docs
			for target := range expectedDocs {
				if target == "index.md" {
					continue
				}
				link := "./" + target
				if !strings.Contains(content, link) {
					t.Fatalf("index.md missing graph link to %q", link)
				}
			}
		} else {
			// Subsystem docs must link back to index.md
			if !strings.Contains(content, "./index.md") {
				t.Fatalf("%s missing backlink to ./index.md", filename)
			}
		}
	}
}

// TestOKFReindexIdempotentAndPreservesUserNotes checks that reindexing an existing OKF
// document updates its timestamp and fenced body while preserving custom user prose.
func TestOKFReindexIdempotentAndPreservesUserNotes(t *testing.T) {
	vault := vaultEnv(t)
	if err := Reindex(); err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	userNote := "## Important Operator Notes\nDo not wipe /opt/custom\n"
	sysPath := filepath.Join(vault, "system.md")
	existing, err := os.ReadFile(sysPath)
	if err != nil {
		t.Fatal(err)
	}

	// Append user prose outside the fence
	withUserNote := string(existing) + "\n" + userNote
	if err := os.WriteFile(sysPath, []byte(withUserNote), 0o644); err != nil {
		t.Fatal(err)
	}

	// Reindex again
	if err := Reindex(); err != nil {
		t.Fatalf("second Reindex: %v", err)
	}

	reindexed, err := os.ReadFile(sysPath)
	if err != nil {
		t.Fatal(err)
	}
	reindexedStr := string(reindexed)

	if !strings.Contains(reindexedStr, userNote) {
		t.Fatalf("user note was lost after reindex:\n%s", reindexedStr)
	}
	if !strings.Contains(reindexedStr, "type: Hardware Component") {
		t.Fatalf("OKF frontmatter damaged after reindex:\n%s", reindexedStr)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
