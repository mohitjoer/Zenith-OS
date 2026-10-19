// Package hypr reads the window list out of the compositor.
//
// The switcher is a most-recently-used list, and the whole feature is one
// question: which window was focused most recently. Hyprland already answers it,
// in `hyprctl -j clients`, and the answer is in a field whose sign is the
// opposite of what every instinct expects. That is why this is Go and tested
// rather than a couple of lines of QML.
//
// Hyprland has no built-in alt-tab to reuse. There is no `tab` bind in any
// shipped default config at any of the last three releases, no compositor-
// internal Tab handling, and no switcher plugin in the official plugins repo.
// The built-in `cycleNext` walks layout order scoped to the active workspace,
// which is neither MRU nor cross-workspace. There is correct MRU cycling in the
// source — `CWindowQuery::cycleHistory` — and it has zero call sites.
//
// That is why this package exists rather than a dispatch.
package hypr

import (
	"context"
	"encoding/json"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// probeTimeout is a deadline on the compositor.
//
// The socket hyprctl writes to is synchronous with a five-second timeout on the
// Hyprland side, so a call that blocks here is a compositor that is busy or gone,
// and the switcher must not hang open waiting to hear about it.
const probeTimeout = 2 * time.Second

// Window is the part of a Hyprland client this package uses.
//
// Not every field `hyprctl -j clients` emits. A struct mirroring the whole
// schema is a struct that has to be edited when the schema changes, and three
// of the fields an earlier version of this had — `fakeFullscreen`,
// `fullscreenMode` — no longer exist at all.
type Window struct {
	Address string `json:"address"`
	Class   string `json:"class"`
	Title   string `json:"title"`
	Mapped  bool   `json:"mapped"`
	Monitor int    `json:"monitor"`
	// FocusHistoryID is Hyprland's own MRU rank. Read the comment on rank().
	FocusHistoryID int `json:"focusHistoryID"`
	Workspace      struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"workspace"`
}

// Clients returns the switchable windows, most recently focused first.
//
// A failure is an empty list, never an error: a switcher with nothing to show
// closes itself, which is the correct behaviour for a keybind that fired.
func Clients() []Window {
	out, err := run("hyprctl", "-j", "clients")
	if err != nil {
		return []Window{}
	}

	var clients []Window
	if err := json.Unmarshal(out, &clients); err != nil {
		return []Window{}
	}
	return selectMRU(clients)
}

// selectMRU is Clients without the subprocess, so the ordering can be tested.
//
// The order is taken once at open and never re-queried. Focusing a window
// mutates focusHistoryID immediately, so re-reading on each keypress reshuffles
// the list under the user between one Tab and the next.
func selectMRU(clients []Window) []Window {
	windows := make([]Window, 0, len(clients))
	for _, client := range clients {
		if client.Mapped && !special(client.Workspace.ID) {
			windows = append(windows, client)
		}
	}
	sort.SliceStable(windows, func(i, j int) bool {
		return rank(windows[i].FocusHistoryID) < rank(windows[j].FocusHistoryID)
	})
	return windows
}

// rank makes focusHistoryID sortable.
//
// This is the single fact the whole feature rests on, and it is the inverse of
// what it looks like. Hyprland's history vector is ordered oldest to newest, and
// the id is computed as `size - index - 1` — so the window you were just in has
// id 0, and *lower is more recent*. Sorting descending, which is the obvious
// thing to do, produces least-recently-used.
//
// -1 means "not in the history vector", which happens for a window that was
// never focused. It is pushed to the end rather than the front, where a raw
// ascending sort would put it: a window nobody has ever focused is not the window
// the user just came from.
func rank(id int) int {
	if id < 0 {
		return 1 << 30
	}
	return id
}

// special reports whether a workspace id is a scratchpad.
//
// The range is -99..-2, from `SPECIAL_WORKSPACE_START` in Hyprland's macros.hpp,
// tested with `id >= SPECIAL_WORKSPACE_START && id <= -2` in its own source.
//
// `id < 0` is the tempting one-liner and it is wrong: name-based *normal*
// workspaces are negative too, starting at -1337 and counting downward. That test
// hides every named workspace from the switcher. -1 is WORKSPACE_INVALID and
// belongs to neither.
func special(id int) bool {
	return id >= -99 && id <= -2
}

// run executes a command with a deadline.
//
// LC_ALL=C because the JSON keys and values are parsed, not scanned, and a
// command that fails here should fail rather than return a locale-flavoured
// string that happens to unmarshal.
func run(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return out, nil
}

// Focus moves focus to a window by address.
//
// One call and one line, which is why it is not a CLI subcommand. Unlike the
// parsers above, this is a documented dispatch with nothing to interpret, and a
// subcommand wrapping it would be a command that exists only to be called by one
// caller.
//
// The argument is `address:0x...` and is passed as a single argv element, so no
// quoting rule applies. This is the hyprlang form; Hyprland 0.55 and later wrap
// dispatch in `hl.dispatch(...)` under a Lua config, which is a different
// argument entirely. Zenith ships 0.53.3, where this is correct — see
// system/packages.yaml.
func Focus(address string) error {
	_, err := run("hyprctl", "dispatch", "focuswindow", "address:"+address)
	return err
}

// Label is the one line a window is listed by.
func Label(window Window) string {
	if window.Class != "" {
		return window.Class
	}
	return strings.TrimSpace(window.Title)
}
