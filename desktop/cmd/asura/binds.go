package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"asura/desktop/internal/shell"
)

// The keybinding catalogue, printed as JSON.
//
// The catalogue is one file — session/binds.json — and this is the only thing
// that reads it for the shell. The shell could read the file itself with a
// FileView and a JSON.parse, and the reason it does not is that a file read
// twice is read by two pieces of code: the Go side would have a parser, the
// QML side would have another one, and the two would agree until the day one
// of them was changed.
//
// So the shell asks the tool, the way it already asks for the applications on
// the machine and the windows behind the switcher. The tool is the thing
// that knows the formats, and the catalogue is one more format it knows.
//
// The same answer is for a person: `asura binds` at a terminal is the
// catalogue, and it is the same catalogue the compositor's configuration is
// checked against by `asura doctor`.

// bindsPath is where the keybinding catalogue is installed.
//
// The session's own directory rather than the compositor's, because the
// catalogue is not the compositor's: it is the list of what the keyboard
// does, and the compositor's configuration is one of the two things that
// have to agree with it.
const bindsPath = "/usr/share/asura/session/binds.json"

// bindsCatalogue finds the catalogue, installed or in a checkout.
//
// The installed path is the truth on a real machine. ASURA_BINDS exists so
// the command also works in a checkout, for the same reason ASURA_SCHEMA_DIR
// does: a command that only works after installation is a command nobody
// runs before installing.
func bindsCatalogue(root string) (shell.Catalogue, error) {
	path := filepath.Join(root, bindsPath)
	if catalogue, err := shell.LoadCatalogue(path); err == nil {
		return catalogue, nil
	}

	override := os.Getenv("ASURA_BINDS")
	if override == "" {
		return shell.Catalogue{}, fmt.Errorf(
			"the keybinding catalogue is not installed in %s; install the "+
				"asura package, or set ASURA_BINDS to the catalogue's "+
				"path", path)
	}
	catalogue, err := shell.LoadCatalogue(override)
	if err != nil {
		return shell.Catalogue{}, fmt.Errorf("ASURA_BINDS=%s: %w", override, err)
	}
	return catalogue, nil
}

// asuraBinds prints the keybinding catalogue as JSON.
func asuraBinds(args []string) int {
	set := newFlags("binds", bindsUsage)
	var root = set.String("root", "", "operate on a tree under this directory")
	if err := set.Parse(args); err != nil {
		return exitUsage
	}

	catalogue, err := bindsCatalogue(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "asura binds:", err)
		return exitFail
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(catalogue); err != nil {
		fmt.Fprintln(os.Stderr, "asura binds:", err)
		return exitFail
	}
	return exitOK
}
