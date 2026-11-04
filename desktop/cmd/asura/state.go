package main

import (
	"encoding/json"
	"fmt"
	"os"

	"asura/desktop/internal/desktopstate"
)

// asuraState reports the live desktop state as JSON.
//
// This is the entry point the sidebar reads through, and it is the reason the
// probing lives in Go: five parsers written in QML would have no tests.
func asuraState(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "asura state: takes no arguments")
		return exitUsage
	}

	body, err := json.MarshalIndent(desktopstate.Collect(), "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "asura state:", err)
		return exitFail
	}
	fmt.Println(string(body))
	return exitOK
}
