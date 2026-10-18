package probe

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestNoAbsoluteSysfsPath is the property the rest of this package is built on.
//
// Every reader takes a root and reads nothing outside it. That is what lets a
// test point a reader at a captured directory tree instead of at a laptop —
// and it decays on the first convenience, the one hard-coded "/sys/class/dmi/id"
// somebody adds because they already had it in their editor.
//
// So the rule is checked mechanically instead of being asked for in review. It
// walks the AST of every non-test file in this package and fails on any string
// literal beginning "/sys" or "/proc" except the RootDefault declaration. A new
// absolute path is a test failure the moment it is written.
//
// If this test ever needs loosening, the loosening belongs in the design, not
// here: a reader that must read a fixed path has a root parameter it is not
// honouring, and the fix is to give it one.
func TestNoAbsoluteSysfsPath(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package directory: %v", err)
	}

	fset := token.NewFileSet()

	var files []*ast.File
	var names []string
	allowed := map[string]bool{}

	// RootDefault is collected across the whole package before any file is
	// checked, because it is declared once in one file and permits literals in
	// all of them.
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		// The test files are allowed to name absolute paths: a test is
		// supposed to construct a fixture path, and that is the whole point of
		// the property being about non-test code.
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		files = append(files, file)
		names = append(names, name)
		for value := range allowedAbsolutePaths(file) {
			allowed[value] = true
		}
	}

	if len(allowed) == 0 {
		t.Fatal("no RootDefault declaration found in package probe; the root " +
			"constant must exist so this test has something to permit")
	}

	for i, file := range files {
		name := names[i]
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if !strings.HasPrefix(value, "/sys") && !strings.HasPrefix(value, "/proc") {
				return true
			}
			if allowed[value] {
				return true
			}
			t.Errorf("%s:%d: absolute sysfs path %q in a package whose readers "+
				"must take a root; add it to RootDefault only if every reader "+
				"needs it, or pass it in from the binary",
				name, fset.Position(lit.Pos()).Line, value)
			return true
		})
	}
}

// allowedAbsolutePaths finds the RootDefault declaration and returns the
// literal values it assigns.
//
// It deliberately does not simply allow the single string "/sys": if a future
// edit adds a second root, or renames the constant, this follows the constant
// rather than a hard-coded expectation, which is the only version of this that
// keeps working.
func allowedAbsolutePaths(file *ast.File) map[string]bool {
	allowed := map[string]bool{}

	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for _, ident := range spec.Names {
			if ident.Name != "RootDefault" {
				continue
			}
			for _, value := range spec.Values {
				lit, ok := value.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if unquoted, err := strconv.Unquote(lit.Value); err == nil {
					allowed[unquoted] = true
				}
			}
		}
		return true
	})

	return allowed
}

// TestRootRequired is the other half of the property. New(root) refuses an
// empty root rather than defaulting to RootDefault, because a reader that
// quietly ignored its argument would be reading the live machine inside a test
// that believed it was reading a fixture. That is the exact bug the root
// parameter exists to make impossible, so it is worth its own assertion rather
// than being folded into the AST test above.
func TestRootRequired(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("New(\"\") succeeded; an empty root must be an error so a " +
			"fixture test cannot silently read the live machine")
	}

	fs, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New on a real directory: %v", err)
	}
	if fs.Root() == "" {
		t.Fatal("Root() is empty after New succeeded")
	}
}

// TestDeterministicEntries pins the property that makes capture-based tests
// meaningful: the same tree must produce the same output on every run, so a
// golden-file test is testing detection rather than filesystem iteration order.
func TestDeterministicEntries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"zulu", "alpha", "mike", "0000:00:02.0", "0000:01:00.0"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatalf("creating fixture entry %s: %v", name, err)
		}
	}

	fs, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var first []string
	for run := 0; run < 5; run++ {
		got, err := fs.entries("")
		if err != nil {
			t.Fatalf("entries: %v", err)
		}
		if run == 0 {
			first = got
			continue
		}
		if strings.Join(got, ",") != strings.Join(first, ",") {
			t.Fatalf("entries not deterministic:\n run 0: %v\n run %d: %v", first, run, got)
		}
	}

	if first[0] != "0000:00:02.0" {
		t.Errorf("entries not sorted: got %v", first)
	}
}

// TestAbsentIsAResult checks the rule that keeps the subsystem from failing on
// hardware it has simply never seen. A missing attribute is a fact about the
// machine, and it is reported as absent rather than as an error.
func TestAbsentIsAResult(t *testing.T) {
	fs, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	value, present, err := fs.readString("class/dmi/id/sys_vendor")
	if err != nil {
		t.Fatalf("readString on a missing file returned an error: %v", err)
	}
	if present {
		t.Errorf("readString reported a missing file as present (%q)", value)
	}

	data, present, err := fs.readBytes("class/drm/card0-eDP-1/edid")
	if err != nil {
		t.Fatalf("readBytes on a missing file returned an error: %v", err)
	}
	if present || data != nil {
		t.Errorf("readBytes reported a missing file as present: %v", data)
	}

	names, err := fs.entries("firmware/acpi/tables")
	if err != nil {
		t.Fatalf("entries on a missing directory returned an error: %v", err)
	}
	if names != nil {
		t.Errorf("entries on a missing directory returned %v, want nil", names)
	}
}
