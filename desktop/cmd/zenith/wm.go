package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"zenith/desktop/internal/wm"
)

// `zenith wm` is the front door to the compositor seam (internal/wm) for what
// is not the daemon. Ported from ryoku/cli/wm.go, minus the verbs nothing calls
// here: the switching and config-path verbs belong to a second compositor
// Zenith does not have, and `state` has no caller yet.
//
// What calls the verbs that remain:
//
//	status  the capture scripts read "Capabilities:", the store reads "Provider:"
//	caps    zenith-cmd-nightlight reads nightLightProcess
//	act     the Hub's touchpad switch, and the scripts that reload the compositor
//	        or start the night light
//
// The output of status and caps is read by scripts with sed, so the labels are
// not translated.

const wmUsage = `Usage: zenith wm <command>

  status            the compositor, its version, workspace model and capabilities
  caps              the capability manifest, as JSON
  act <id> [args]   run a compositor action, e.g. input.touchpad toggle

act exits 3 when no compositor answers and 4 when it cannot do the action.
`

func init() {
	register("wm", "the compositor seam: status, caps, act", zenithWm)
}

func zenithWm(args []string) int {
	return wmMain(args, wm.Open(), os.Stdout, os.Stderr)
}

// wmMain takes the client so a test never reaches the live compositor.
func wmMain(args []string, c *wm.Client, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(errOut, wmUsage)
		return exitUsage
	}
	switch args[0] {
	case "status":
		return wmStatus(c, out)
	case "caps":
		return wmCaps(c, out, errOut)
	case "act":
		return wmAct(c, args[1:], out, errOut)
	case "-h", "--help", "help":
		fmt.Fprint(out, wmUsage)
		return exitOK
	}
	fmt.Fprintf(errOut, "zenith wm: unknown command %q\n\n%s", args[0], wmUsage)
	return exitUsage
}

func wmStatus(c *wm.Client, out io.Writer) int {
	d := c.Detection()
	name := d.Name
	if name == "" {
		name = "(none)"
	}
	fmt.Fprintf(out, "Provider: %s\n", name)
	fmt.Fprintf(out, "Live: %t\n", d.Live)
	fmt.Fprintf(out, "Source: %s\n", d.Source)
	caps, err := c.Caps()
	if err != nil {
		fmt.Fprintf(out, "Capabilities: unavailable (%v)\n", err)
		return exitOK
	}
	if caps.Version != "" {
		fmt.Fprintf(out, "Version: %s\n", caps.Version)
	}
	fmt.Fprintf(out, "Workspace model: %s\n", caps.WorkspaceModel)
	supports := make([]string, 0, len(caps.Supports))
	for _, capability := range caps.Supports {
		supports = append(supports, string(capability))
	}
	sort.Strings(supports)
	fmt.Fprintf(out, "Capabilities: %s\n", strings.Join(supports, ", "))
	fmt.Fprintf(out, "Setting domains: %s\n", strings.Join(caps.SettingDomains, ", "))
	return exitOK
}

func wmCaps(c *wm.Client, out, errOut io.Writer) int {
	caps, err := c.Caps()
	if err != nil {
		fmt.Fprintln(errOut, "zenith wm:", err)
		return exitFail
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(caps); err != nil {
		fmt.Fprintln(errOut, "zenith wm:", err)
		return exitFail
	}
	return exitOK
}

// wmCallContext bounds an action. ZENITH_WM_CALL_TIMEOUT exists for the idle
// and lock paths, where a compositor that does not answer must not hold a
// script forever.
func wmCallContext() (context.Context, context.CancelFunc, error) {
	raw := strings.TrimSpace(os.Getenv("ZENITH_WM_CALL_TIMEOUT"))
	if raw == "" {
		ctx, cancel := context.WithCancel(context.Background())
		return ctx, cancel, nil
	}
	limit, err := time.ParseDuration(raw)
	if err != nil || limit <= 0 {
		return nil, nil, fmt.Errorf("invalid ZENITH_WM_CALL_TIMEOUT %q", raw)
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	return ctx, cancel, nil
}

// wmAct dispatches an action and maps the two seam errors to distinct exit
// codes, so a caller (a keybind, a script) can tell "no compositor" from "this
// compositor cannot". Quiet on success: it runs on the idle and lock paths.
func wmAct(c *wm.Client, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "usage: zenith wm act <action> [args...]")
		return exitUsage
	}
	ctx, cancel, err := wmCallContext()
	if err != nil {
		fmt.Fprintln(errOut, "zenith wm:", err)
		return exitUsage
	}
	defer cancel()

	// An action that answers with a value (input.touchpad status prints on|off)
	// reaches stdout; the silent ones return an empty string.
	result, err := c.ActOutputContext(ctx, wm.Action(args[0]), args[1:]...)
	if err == nil {
		if result != "" {
			fmt.Fprintln(out, result)
		}
		return exitOK
	}
	fmt.Fprintf(errOut, "zenith: %v\n", err)
	switch {
	case errors.Is(err, wm.ErrNoProvider):
		return 3
	case errors.Is(err, wm.ErrUnsupported):
		return 4
	}
	return exitFail
}
