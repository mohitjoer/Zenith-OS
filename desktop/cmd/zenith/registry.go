package main

import (
	"fmt"
	"os"
	"sort"
)

// extraCommands are the subcommands that live in their own files and announce
// themselves from an init function, so adding one is a new file and not an edit
// to the dispatch table in main.go. Two people adding two commands at once then
// touch two files, and the table cannot be left half-edited between them.
//
// The built-in commands keep their place in the table in main.go: the usage
// text lists them in a deliberate order, and a registry sorted by name would
// bury "update" under "theme".
var extraCommands = map[string]extraCommand{}

type extraCommand struct {
	summary string
	run     func(args []string) int
}

// register adds a subcommand. It panics on a duplicate or a name the table
// already has, because a command that silently replaced another would be found
// by a user and not by a test.
func register(name, summary string, run func(args []string) int) {
	if _, taken := commands[name]; taken {
		panic("zenith: command " + name + " is already built in")
	}
	if _, taken := extraCommands[name]; taken {
		panic("zenith: command " + name + " is registered twice")
	}
	extraCommands[name] = extraCommand{summary: summary, run: run}
}

// printExtraUsage lists the registered commands after the built-in ones.
func printExtraUsage(w *os.File) {
	if len(extraCommands) == 0 {
		return
	}
	names := make([]string, 0, len(extraCommands))
	for name := range extraCommands {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Fprintln(w)
	for _, name := range names {
		fmt.Fprintf(w, "  %-13s %s\n", name, extraCommands[name].summary)
	}
}
