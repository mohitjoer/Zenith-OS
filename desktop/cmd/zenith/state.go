package main

import (
	"encoding/json"
	"fmt"
	"os"

	"zenith/desktop/internal/desktopstate"
)

// zenithState reports the live desktop state as JSON.
//
// It exists because parsing wpctl, nmcli, bluetoothctl, brightnessctl and
// playerctl output is the hard part of the sidebar, and this is where the tests
// are. Five parsers written in QML would have no tests and five places to be
// wrong; one tested command the shell renders is smaller and checkable.
//
// Nothing here fails. A machine with no bluetooth adapter reports no adapter, and
// a sidebar that shows an error panel for hardware that does not exist is a
// sidebar nobody turns on.
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
