// Command zenith is Zenith's control tool.
//
// It is a small binary on purpose. Every subcommand here does one thing, prints
// something a human or a script can act on, and exits non-zero when it could not
// do it. There is no daemon in this file and no background work: a control tool
// that keeps running is a control tool whose state nobody can reason about.
//
// Exit codes are shared with the rest of the suite so a script can tell the
// cases apart:
//
//	0  success
//	1  the command failed — a probe could not run, a report was refused
//	2  bad usage — an unknown flag, a missing argument
//	3  the machine does not match a profile, or two profiles conflict
//
// Code 3 is distinct from 1 on purpose. "Zenith does not know this machine" is
// not a failure of the tool; it is an answer, and a caller that treats it as an
// error will retry a machine that will never be recognised.
package main

import (
	"flag"
	"fmt"
	"os"
)

var version = "0.1-pre-alpha"

func main() {
	os.Exit(run(os.Args[1:]))
}

// commands is the dispatch table.
//
// It is a map rather than a switch because the subcommand list is also the usage
// text, and one source for it cannot drift from the other.
var commands = map[string]func(args []string) int{
	"adopt":       zenithAdopt,
	"hardware":    runHardware,
	"update":      zenithUpdate,
	"rollback":    zenithRollback,
	"materialize": zenithMaterialize,
	"reset":       zenithReset,
	"doctor":      zenithDoctor,
	"status":      zenithStatus,
	"version":     runVersion,
}

// Exit codes. Declared once so a caller reading this file sees the contract in
// one place rather than scattered through the subcommands.
const (
	exitOK      = 0
	exitFail    = 1
	exitUsage   = 2
	exitNoMatch = 3
)

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return exitUsage
	}

	switch args[0] {
	case "-h", "--help", "help":
		usage(os.Stdout)
		return exitOK
	case "-V", "--version":
		return runVersion(nil)
	}

	command, known := commands[args[0]]
	if !known {
		fmt.Fprintf(os.Stderr, "zenith: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return exitUsage
	}
	return command(args[1:])
}

func usage(w *os.File) {
	fmt.Fprintf(w, `zenith %s — Zenith OS control tool

Usage:
  zenith <command> [flags]

Commands:
  adopt         move this machine onto the Zenith session
  hardware      read this machine, and the profile database
  update        snapshot, fetch packages, re-materialize
  rollback      re-materialize the previous revision
  materialize   re-lay the base, clobbering shipped content
  reset PATH    drop one user edit back to the base
  doctor        report drift between base and overlay
  status        revision, paths, and how much drift
  version       print the version

Run "zenith <command>" with no arguments for that command's own usage.
`, version)
}

func runVersion(_ []string) int {
	fmt.Println(version)
	return exitOK
}

// hardwareSubcommands is the dispatch table for "zenith hardware".
var hardwareSubcommands = map[string]func(args []string) int{
	"report":        hardwareReport,
	"match":         hardwareMatch,
	"lint-profiles": hardwareLintProfiles,
	"capture":       hardwareCapture,
}

func runHardware(args []string) int {
	if len(args) == 0 {
		hardwareUsage(os.Stderr)
		return exitUsage
	}

	switch args[0] {
	case "-h", "--help", "help":
		hardwareUsage(os.Stdout)
		return exitOK
	}

	subcommand, known := hardwareSubcommands[args[0]]
	if !known {
		fmt.Fprintf(os.Stderr, "zenith hardware: unknown subcommand %q\n\n", args[0])
		hardwareUsage(os.Stderr)
		return exitUsage
	}
	return subcommand(args[1:])
}

func hardwareUsage(w *os.File) {
	fmt.Fprint(w, `Usage: zenith hardware <subcommand> [flags]

  report           this machine, redacted; safe to paste into an issue
  match            which profile claims this machine, and at what tier
  lint-profiles    check the profile database against the schema rules
  capture          write this machine's raw sysfs tree to a directory

Run a subcommand with --help for its own flags.

Exit codes:
  0  success
  1  the command failed
  2  bad usage
  3  no profile matches this machine, or two profiles conflict
`)
}

// newFlags builds a flag set that reports a usage error the way this tool does:
// the message on stderr and exit 2, never a panic.
func newFlags(name string, usageText string) *flag.FlagSet {
	set := flag.NewFlagSet("zenith "+name, flag.ContinueOnError)
	set.Usage = func() { fmt.Fprint(os.Stderr, usageText) }
	return set
}

// parseFlags parses arguments and returns the positional ones.
//
// The standard flag package stops at the first non-flag argument, which makes
// "zenith reset ghostty/config --root DIR" silently drop the --root flag: the
// path becomes the only positional and the flag is never seen. That is a trap in
// a tool a person types by hand, so flags after a positional are honoured too.
//
// The arguments are consumed one positional at a time and the remainder is
// re-parsed, which is what makes interspersed flags work. A single re-parse of
// the whole remainder does not: its own first argument is the positional that
// stopped the previous parse, so it stops there again.
func parseFlags(set *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	rest := args

	for {
		if err := set.Parse(rest); err != nil {
			return nil, err
		}
		rest = set.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		positional = append(positional, rest[0])
		rest = rest[1:]
	}
}
