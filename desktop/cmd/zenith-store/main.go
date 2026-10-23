// zenith-store is the Go data plane for the Zenith store window: it normalizes
// the places software comes from into one JSON catalogue and installs items
// without activating them. The Quickshell front end and the Settings pages shell
// out to these subcommands.
//
//	zenith-store catalog [--refresh] [--category <id>]   normalized catalogue, JSON
//	zenith-store search <query>                          apt packages matching a query, JSON
//	zenith-store install <category> <id>                 install
//	zenith-store remove <category> <id>                  removal
//	zenith-store open <section>                          open the store window
//	zenith-store settings <category> [id]                open Settings on its page
//
// install and remove print {"stage","percent"} lines on stdout while they work
// and a failure as one sentence on stderr, which is what the window shows.
//
// Ported from Ryoku's ryostore backend (GPL-3.0). Zenith has no remote catalogue
// and a plugin is just a folder (`zenith plugin` installs and removes them), so
// the receipt transactions, registries and extras actuator did not come across.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	scrubQuickshellCrashEnv()
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "zenith-store:", err)
		os.Exit(1)
	}
}

// dispatch routes one command. It returns an error for every bad invocation so
// the caller reports one useful line and exits nonzero.
func dispatch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command; expected catalog, search, install, remove, open, or settings")
	}
	switch args[0] {
	case "catalog":
		return runCatalog(os.Stdout, providers(), args[1:])
	case "search":
		return runSearch(args[1:])
	case "install":
		return runInstall(providers(), args[1:])
	case "remove":
		return runRemove(providers(), args[1:])
	case "open":
		return runOpen(args[1:])
	case "settings":
		return runSettings(args[1:])
	default:
		return fmt.Errorf("unknown command %q; expected catalog, search, install, remove, open, or settings", args[0])
	}
}

func runCatalog(w io.Writer, provs []Provider, args []string) error {
	refresh := false
	category := ""
	for rest := args; len(rest) > 0; {
		switch rest[0] {
		case "--refresh":
			refresh = true
			rest = rest[1:]
		case "--category":
			if len(rest) < 2 || rest[1] == "" {
				return fmt.Errorf("--category needs a non-empty category id")
			}
			category = rest[1]
			rest = rest[2:]
		default:
			return fmt.Errorf("unknown catalog flag %q", rest[0])
		}
	}
	if category != "" {
		p, ok := providerFor(provs, category)
		if !ok {
			return fmt.Errorf("unknown category %q", category)
		}
		provs = []Provider{p}
	}
	return json.NewEncoder(w).Encode(BuildCatalog(context.Background(), provs, refresh))
}

func runSearch(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("search needs one query")
	}
	items, err := aptProvider{}.Search(context.Background(), args[0])
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Items []Item `json:"items"`
	}{items})
}

// transactionContext ends on SIGTERM or SIGINT, which is how the window cancels,
// and keeps going when the window is gone: a closed pipe or hangup must not kill
// a package transaction halfway.
func transactionContext() (context.Context, context.CancelFunc) {
	signal.Ignore(syscall.SIGPIPE, syscall.SIGHUP)
	return signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
}

func runInstall(provs []Provider, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("install needs <category> <id>")
	}
	p, ok := providerFor(provs, args[0])
	if !ok {
		return fmt.Errorf("unknown category %q", args[0])
	}
	ctx, stop := transactionContext()
	defer stop()
	return p.Install(ctx, args[1])
}

func runRemove(provs []Provider, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("remove needs <category> <id>")
	}
	p, ok := providerFor(provs, args[0])
	if !ok {
		return fmt.Errorf("unknown category %q", args[0])
	}
	ctx, stop := transactionContext()
	defer stop()
	return p.Remove(ctx, args[1])
}
