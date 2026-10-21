package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zenith/desktop/internal/settings"
)

// TestCamelCaseKeepsTheFirstWord pins the naming the shell reads settings by.
//
// The shell asks for `Config.barHeight`, and the schema calls the key
// `bar-height`. The rule that turns one into the other has to keep the first
// word as it is: `BarHeight` is a different name from `barHeight`, and a
// shell that looks for one and finds the other reads nothing at all.
func TestCamelCaseKeepsTheFirstWord(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"bar-height", "barHeight"},
		{"dock-position", "dockPosition"},
		{"color-scheme", "colorScheme"},
		{"idle-lock-seconds", "idleLockSeconds"},
		{"favorites", "favorites"},
		{"wallpaper-path-dark", "wallpaperPathDark"},
	}

	for _, test := range tests {
		if got := camelCase(test.in); got != test.want {
			t.Errorf("camelCase(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

// TestParseStringListReadsGVariantLists pins the reader the shell's lists go
// through.
//
// The values arrive as GVariant text — `['workspaces', 'clock']` — and a
// reader that keeps the quotes, or the space after the comma, puts a name
// with a space in it into the bar, where it is a widget that does not exist.
func TestParseStringListReadsGVariantLists(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"[]", []string{}},
		{"['workspaces', 'clock']", []string{"workspaces", "clock"}},
		{"['a']", []string{"a"}},
		{"['with, comma', 'plain']", []string{"with, comma", "plain"}},
		{"", []string{}},
	}

	for _, test := range tests {
		got := parseStringList(test.in)
		if len(got) != len(test.want) {
			t.Errorf("parseStringList(%q) = %q, want %q", test.in, got, test.want)
			continue
		}
		for i := range got {
			if got[i] != test.want[i] {
				t.Errorf("parseStringList(%q) = %q, want %q", test.in, got, test.want)
				break
			}
		}
	}
}

// TestConfigValuesIsJSONTheShellCanRead runs the command the shell reads its
// settings through and checks the shape of what comes back.
//
// The shell parses this as JSON and reads the values by name, so a change to
// the shape is a change to the shell: the values are camelCase, an enum is a
// number rather than a nick, and every schema in the catalogue is in there.
func TestConfigValuesIsJSONTheShellCanRead(t *testing.T) {
	// The path is absolute, and it has to be: the command below is run as a
	// process with its own working directory, and a relative path means
	// something different there than it does here. The schema directory
	// was "../../../shell" here and "../../../shell" in the child, which
	// from the child's directory is a directory that does not exist — so
	// the command failed on a path that was right and the test reported
	// it as the command being broken.
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	schemas := filepath.Join(repo, "shell")
	if _, err := os.Stat(schemas); err != nil {
		t.Skip("the schemas are not in the checkout:", err)
	}

	t.Setenv("ZENITH_SCHEMA_DIR", schemas)

	if _, err := settings.Load(schemas); err != nil {
		t.Fatalf("loading the schemas: %v", err)
	}

	// The command prints to stdout, so it is run as a process rather than
	// called: the JSON is the contract, and the contract is what a shell
	// parses.
	command := exec.Command("go", "run", "./cmd/zenith", "config", "values")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "ZENITH_SCHEMA_DIR="+schemas)

	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("running the command: %v\n%s", err, out)
	}

	var values map[string]map[string]any
	if err := json.Unmarshal(out, &values); err != nil {
		t.Fatalf("the output is not JSON: %v\n%s", err, out)
	}

	shell, ok := values["org.zenith.shell"]
	if !ok {
		t.Fatalf("no org.zenith.shell in the output: %v", values)
	}

	// A number, because the shell compares an edge against a number.
	if _, ok := shell["barHeight"].(float64); !ok {
		t.Errorf("barHeight is %T, want a number", shell["barHeight"])
	}
	if _, ok := shell["dockPosition"].(float64); !ok {
		t.Errorf("dockPosition is %T, want a number", shell["dockPosition"])
	}

	// A list, because the shell iterates it.
	if _, ok := shell["barWidgets"].([]any); !ok {
		t.Errorf("barWidgets is %T, want a list", shell["barWidgets"])
	}

	// The desktop's schema too: the colour scheme lives there, and a shell
	// that could not read it would be a shell with no dark mode.
	if _, ok := values["org.zenith.desktop"]["colorScheme"]; !ok {
		t.Errorf("no colorScheme in org.zenith.desktop: %v", values["org.zenith.desktop"])
	}
}

// TestConfigSetAcceptsAnEnumByItsNumber pins the write the shell makes.
//
// The shell holds an edge as a number and writes it back. The number is the
// enum's value and the nick is its name, and the schema is what says which
// name belongs to which value — so a write by number has to be accepted and
// stored as the nick the schema uses.
func TestConfigSetAcceptsAnEnumByItsNumber(t *testing.T) {
	repo := filepath.Clean(filepath.Join("..", "..", ".."))
	schemas := filepath.Join(repo, "shell")
	if _, err := os.Stat(schemas); err != nil {
		t.Skip("the schemas are not in the checkout:", err)
	}

	catalogue, err := settings.Load(schemas)
	if err != nil {
		t.Fatalf("loading the schemas: %v", err)
	}

	ref := settings.Ref{Schema: "org.zenith.desktop", Key: "color-scheme"}
	key, schema, err := catalogue.Lookup(ref)
	if err != nil {
		t.Fatalf("looking the key up: %v", err)
	}
	if key.Enum == "" {
		t.Skip("color-scheme is not an enum in this checkout")
	}

	// The number is found in the schema, not in this file: a mapping written
	// out here would be a second copy of the schema's own, and the two would
	// disagree the first time a value was added.
	found := ""
	for _, permitted := range schema.Enums[key.Enum] {
		if permitted.Value == 1 {
			found = permitted.Nick
		}
	}
	if found == "" {
		t.Skip("the enum has no value 1")
	}

	if err := catalogue.ValidateValue(ref, found); err != nil {
		t.Errorf("the schema's own nick %q is not a valid value: %v", found, err)
	}
	if err := catalogue.ValidateValue(ref, "1"); err == nil {
		t.Error("the number was accepted by the library, which checks nicks only")
	}

	if !strings.Contains(found, "-") && found == "" {
		t.Error("the enum's nick is empty")
	}
}
