package main

import (
	"encoding/json"
	"fmt"
	"os"

	"zenith/desktop/internal/hypr"
)

// zenithWindows lists the switchable windows, most recently focused first.
//
// The switcher reads this rather than `hyprctl` itself, for one reason: the
// ordering is the feature and it is the part that is easy to get backwards.
// See internal/hypr.
func zenithWindows(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "zenith windows: takes no arguments")
		return exitUsage
	}

	body, err := json.MarshalIndent(hypr.Clients(), "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith windows:", err)
		return exitFail
	}
	fmt.Println(string(body))
	return exitOK
}
