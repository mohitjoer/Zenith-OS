// asura-store is the Go data plane for the Asura store window: it normalizes
// the places software comes from into one JSON catalogue and installs items
// without activating them. The Quickshell front end and the Settings pages shell
// out to these subcommands.
//
//	asura-store catalog [--refresh] [--category <id>]   normalized catalogue, JSON
//	asura-store search <query>                          apt/xbps packages matching a query, JSON
//	asura-store check                                   upstream-changed probe, JSON
//	asura-store install <category> <id>                 install
//	asura-store remove <category> <id>                  removal
//	asura-store open <section>                          open the store window
//	asura-store settings <category> [id]                open Settings on its page
//
// install and remove print {"stage","percent"} lines on stdout while they work
// and a failure as one sentence on stderr, which is what the window shows.
//
// Ported from Ryoku's ryostore backend (GPL-3.0). Asura has no remote catalogue
// and a plugin is just a folder (`asura plugin` installs and removes them), so
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
	"time"
)

func main() {
	scrubQuickshellCrashEnv()
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "asura-store:", err)
		os.Exit(1)
	}
}

// dispatch routes one command. It returns an error for every bad invocation so
// the caller reports one useful line and exits nonzero.
func dispatch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command; expected catalog, search, check, install, remove, open, or settings")
	}
	switch args[0] {
	case "catalog":
		return runCatalog(os.Stdout, providers(), args[1:])
	case "search":
		return runSearch(args[1:])
	case "check":
		return runCheck(os.Stdout, providers())
	case "install":
		return runInstall(providers(), args[1:])
	case "remove":
		return runRemove(providers(), args[1:])
	case "open":
		return runOpen(args[1:])
	case "settings":
		return runSettings(args[1:])
	default:
		return fmt.Errorf("unknown command %q; expected catalog, search, check, install, remove, open, or settings", args[0])
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
		cat := BuildCatalog(context.Background(), provs, refresh)
		cat.Revision = catalogRevision(cat)
		return json.NewEncoder(w).Encode(cat)
	}
	cat := BuildCatalog(context.Background(), provs, refresh)
	cat.Revision = catalogRevision(cat)
	if err := json.NewEncoder(w).Encode(cat); err != nil {
		return err
	}
	// A freshly built catalogue is the baseline the user is now looking at,
	// so acknowledge it: the refresh dot stays clear until upstream advances.
	if len(cat.Items) > 0 {
		writeSeenRevision(cat.Revision)
	}
	return nil
}

const searchTimeout = 20 * time.Second

func runSearch(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("search needs one query")
	}
	ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
	defer cancel()
	var items []Item
	var err error
	if have("xbps-query") {
		items, err = xbpsProvider{}.Search(ctx, args[0])
	} else if have("apt-cache") {
		items, err = aptProvider{}.Search(ctx, args[0])
	} else {
		items, err = xbpsProvider{}.Search(ctx, args[0])
	}
	if err != nil {
		return err
	}
	if items == nil {
		items = []Item{}
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Items []Item `json:"items"`
	}{items})
}

// runCheck probes the live catalogue (no downloads) and reports whether it has
// advanced past the last acknowledged revision, so the store lights its refresh
// dot only on a real change. It never writes the seen revision: only viewing
// the catalogue (runCatalog) acknowledges it.
func runCheck(w io.Writer, provs []Provider) error {
	cat := BuildCatalog(context.Background(), provs, true)
	cat.Revision = catalogRevision(cat)
	revision := cat.Revision
	seen := readSeenRevision()
	out := struct {
		Revision        string `json:"revision"`
		UpdateAvailable bool   `json:"updateAvailable"`
		Offline         bool   `json:"offline"`
	}{
		Revision:        revision,
		UpdateAvailable: seen != "" && !cat.Offline && revision != seen,
		Offline:         cat.Offline,
	}
	return json.NewEncoder(w).Encode(out)
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
