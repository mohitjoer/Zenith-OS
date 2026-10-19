package main

import (
	"encoding/json"
	"fmt"
	"os"

	"zenith/desktop/internal/desktopstate"
)

// zenithState reports the live desktop state as JSON.
//
// This is the entry point the sidebar reads through, and it is the reason the
// probing lives in Go: five parsers written in QML would have no tests.
func zenithState(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "zenith state: takes no arguments")
		return exitUsage
	}

	body, err := json.MarshalIndent(desktopstate.Collect(), "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith state:", err)
		return exitFail
	}
	fmt.Println(string(body))
	return exitOK
}
