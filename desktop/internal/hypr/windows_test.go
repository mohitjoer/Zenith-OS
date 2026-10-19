package hypr

import (
	"encoding/json"
	"testing"
)

// The fixture is the shape `hyprctl -j clients` emits, transcribed from
// Hyprland's own source rather than invented, with the fields this package does
// not read left out. Kept small on purpose: the bug this file exists to prevent
// is a wrong sort, and a sort cannot be wrong in a way a bigger fixture hides.

const clients = `[
  {"address":"0x55aaa","class":"firefox","title":"Wiki","mapped":true,
   "monitor":0,"focusHistoryID":3,"workspace":{"id":2,"name":"2"}},

  {"address":"0x55bbb","class":"foot","title":"IRC","mapped":true,
   "monitor":0,"focusHistoryID":0,"workspace":{"id":1,"name":"1"}},

  {"address":"0x55ccc","class":"ghost","title":"never focused","mapped":true,
   "monitor":0,"focusHistoryID":-1,"workspace":{"id":1,"name":"1"}},

  {"address":"0x55ddd","class":"mpv","title":"Video","mapped":true,
   "monitor":1,"focusHistoryID":1,"workspace":{"id":-99,"name":"special:magic"}},

  {"address":"0x55eee","class":"named-ws","title":"Named","mapped":true,
   "monitor":0,"focusHistoryID":2,"workspace":{"id":-1340,"name":"web"}},

  {"address":"0x55fff","class":"unmapped","title":"Hidden","mapped":false,
   "monitor":0,"focusHistoryID":4,"workspace":{"id":1,"name":"1"}}
]`

func TestClientsOrder(t *testing.T) {
	windows := mustSelect(t, clients)

	// focusHistoryID ascending, because Hyprland's history vector is oldest-first
	// and the id is computed as size-index-1. Zero is what you were just in.
	// Sorting descending here — the obvious thing — yields least-recently-used,
	// which looks like it works until you use it twice.
	want := []string{"0x55bbb", "0x55eee", "0x55aaa", "0x55ccc"}

	if len(windows) != len(want) {
		t.Fatalf("got %d windows %v, want %d", len(windows), addrs(windows), len(want))
	}
	for i, address := range want {
		if windows[i].Address != address {
			t.Errorf("position %d = %s, want %s (full order %v)",
				i, windows[i].Address, address, addrs(windows))
		}
	}
}

func TestClientsDrops(t *testing.T) {
	windows := mustSelect(t, clients)
	for _, w := range windows {
		switch w.Class {
		case "mpv":
			t.Error("a special workspace window was listed; id -99 is scratchpad")
		case "unmapped":
			t.Error("an unmapped window was listed")
		}
	}
}

func TestSpecial(t *testing.T) {
	specials := []int{-99, -50, -2}
	normals := []int{1, 2, 99, -1, -100, -1340}

	for _, id := range specials {
		if !special(id) {
			t.Errorf("special(%d) = false; scratchpads are -99..-2", id)
		}
	}
	for _, id := range normals {
		// -1340 is the case that matters: name-based *normal* workspaces are
		// negative and start at -1337 counting downward, so `id < 0` would hide
		// every named workspace from the switcher.
		if special(id) {
			t.Errorf("special(%d) = true; that is a normal workspace", id)
		}
	}
}

func TestRankPutsUnfocusedLast(t *testing.T) {
	if rank(-1) <= rank(0) {
		t.Error("an unfocused window ranked ahead of the focused one")
	}
}

func TestLabel(t *testing.T) {
	if got := Label(Window{Class: "foot"}); got != "foot" {
		t.Errorf("got %q", got)
	}
	if got := Label(Window{Title: "  IRC  "}); got != "IRC" {
		t.Errorf("got %q; a classless window falls back to its trimmed title", got)
	}
}

func mustSelect(t *testing.T, raw string) []Window {
	t.Helper()
	var clients []Window
	if err := json.Unmarshal([]byte(raw), &clients); err != nil {
		t.Fatal(err)
	}
	return selectMRU(clients)
}

func addrs(windows []Window) []string {
	out := make([]string, len(windows))
	for i, w := range windows {
		out[i] = w.Address
	}
	return out
}
