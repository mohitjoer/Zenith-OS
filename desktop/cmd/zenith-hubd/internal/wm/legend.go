package wm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// The bind legend. The shipped binds are the catalogue (session/binds.json): a
// label, a category and a chord for each, one list the cheat sheet, `zenith
// doctor` and this page share. This file resolves it against the user's rebinds,
// joins it with what the running compositor reports through `hyprctl binds -j`,
// and returns the full effective legend as []BindRow: every catalogue row (marked
// unhonored when the session does not have it bound), then any live bind the
// catalogue does not know, then the user's own custom binds from the store.
//
// Ported from Ryoku's legend (GPL-3.0). Ryoku parsed its Lua bind modules to
// learn what was bound; Zenith asks the compositor, which cannot drift from the
// config it actually loaded.

// liveBind is one bind as the running compositor reports it.
type liveBind struct {
	chord       string // chordKey form
	display     string // catalogue spelling, for a row of our own
	dispatcher  string
	arg         string
	description string
	mouse       bool
}

// modmask bits as Hyprland reports them.
const (
	maskShift = 1 << 0
	maskCtrl  = 1 << 2
	maskAlt   = 1 << 3
	maskSuper = 1 << 6
)

// liveBinds reads the compositor's global bind table, nil when there is no
// session to ask. Binds inside a submap (the recorder, the resize mode) are not
// part of the legend and are left out.
func liveBinds() []liveBind {
	if !live() {
		return nil
	}
	out, err := ctl("binds", "-j")
	if err != nil {
		return nil
	}
	var raw []struct {
		Mouse       bool   `json:"mouse"`
		Modmask     int    `json:"modmask"`
		Submap      string `json:"submap"`
		Key         string `json:"key"`
		Keycode     int    `json:"keycode"`
		CatchAll    bool   `json:"catch_all"`
		Description string `json:"description"`
		Dispatcher  string `json:"dispatcher"`
		Arg         string `json:"arg"`
	}
	if json.Unmarshal(out, &raw) != nil {
		return nil
	}
	binds := make([]liveBind, 0, len(raw))
	for _, r := range raw {
		if r.Submap != "" || r.CatchAll || r.Key == "" {
			continue
		}
		var parts []string
		if r.Modmask&maskSuper != 0 {
			parts = append(parts, "SUPER")
		}
		if r.Modmask&maskCtrl != 0 {
			parts = append(parts, "CTRL")
		}
		if r.Modmask&maskAlt != 0 {
			parts = append(parts, "ALT")
		}
		if r.Modmask&maskShift != 0 {
			parts = append(parts, "SHIFT")
		}
		parts = append(parts, r.Key)
		display := strings.Join(parts, " + ")
		binds = append(binds, liveBind{
			chord: chordKey(display), display: display, dispatcher: r.Dispatcher,
			arg: r.Arg, description: r.Description, mouse: r.Mouse,
		})
	}
	return binds
}

// buildBindRows walks the catalogue in order, resolving each shipped bind against
// the user's rebinds and the live table. live is nil off a session, in which case
// no row is struck: the legend then reports the catalogue as written.
func buildBindRows(cat []CatalogBind, o Overrides, live []liveBind) []BindRow {
	haveLive := live != nil
	liveSet := make(map[string]bool, len(live))
	for _, lb := range live {
		liveSet[lb.chord] = true
	}
	claimed := make(map[string]bool, len(live))

	rows := make([]BindRow, 0, len(cat)+len(live)+len(o.Keybinds))
	for _, c := range cat {
		row, chords := catalogRow(c, o.KeybindRebinds)
		bound := true
		for _, ch := range chords {
			k := chordKey(ch)
			claimed[k] = true
			for _, al := range NumpadAliases(ch) {
				claimed[chordKey(al)] = true
			}
			if !liveSet[k] {
				bound = false
			}
		}
		if haveLive && !bound {
			row.Unhonored = "Not bound in the running session."
		}
		rows = append(rows, row)
	}

	for _, k := range o.Keybinds {
		if _, ok := customRow(0, k); !ok {
			continue
		}
		claimed[chordKey(k.Keys)] = true
		for _, tw := range NumpadAliases(k.Keys) {
			claimed[chordKey(tw)] = true
		}
	}

	// A live bind the catalogue does not claim is the compositor's own: a line in
	// the user's file, or a bind a lane added before cataloguing it. It is listed
	// under the compositor's name rather than dropped.
	exclusive := make([]BindRow, 0)
	for _, lb := range live {
		if claimed[lb.chord] {
			continue
		}
		claimed[lb.chord] = true
		exclusive = append(exclusive, exclusiveRow(lb))
	}
	return append(append(rows, exclusive...), customRows(o)...)
}

// customRows renders the store's own shortcuts.
func customRows(o Overrides) []BindRow {
	var out []BindRow
	for i, k := range o.Keybinds {
		if row, ok := customRow(i, k); ok {
			out = append(out, row)
		}
	}
	return out
}

// catalogRow renders one catalogue entry as a BindRow: the catalogue's own label
// and hint, its default chord, the effective chord after the user's rebind, and
// the display tokens of that chord. It also returns the concrete chords the row
// stands for (ten for a family), for the check against the live table.
func catalogRow(c CatalogBind, rebinds map[string]string) (BindRow, []string) {
	chord := c.Chord
	if c.Family {
		chord, _ = FamilyRebind(c.Chord, rebinds)
	} else if to, ok := rebinds[c.Chord]; ok {
		if t := strings.TrimSpace(to); t != "" {
			chord = t
		}
	}
	row := BindRow{
		ID:         c.ID,
		Category:   c.Category,
		Label:      c.Label,
		Hint:       c.Hint,
		Keys:       DisplayKeys(chord),
		Default:    c.Chord,
		Chord:      chord,
		Kind:       c.Kind,
		Rebindable: rebindableCombo(c.Chord) && !c.Locked,
		Locked:     c.Locked,
	}
	// A rebind that moved a plain bind onto the number pad works in one NumLock
	// state only: the shipped families bind both keypad faces, a rebind
	// registers one keysym. Name that limit so the row reads honestly.
	if !c.Family {
		if note := numpadRebindHint(chord); note != "" {
			if row.Hint != "" {
				row.Hint += ". " + note
			} else {
				row.Hint = note
			}
		}
	}
	chords := []string{chord}
	if c.Family {
		chords = chords[:0]
		for n := 1; n <= 10; n++ {
			chords = append(chords, ExpandChord(chord, n))
		}
	}
	return row, chords
}

// numpadOffNames is the set of NumLock-off number-pad keysyms.
var numpadOffNames = func() map[string]bool {
	off := map[string]bool{}
	for _, v := range numpadNumLockOff {
		off[v] = true
	}
	return off
}()

// numpadRebindHint names the NumLock limitation of a chord a rebind moved onto
// the number pad, "" when the chord holds no number-pad key.
func numpadRebindHint(chord string) string {
	if len(NumpadAliases(chord)) > 0 {
		return "Works with NumLock on"
	}
	for _, tok := range strings.Split(chord, " + ") {
		if numpadOffNames[tok] {
			return "Works with NumLock off"
		}
	}
	return ""
}

// rebindableCombo reports whether the Hub may record a new chord over a bind: a
// plain keyboard chord can be, a pointer button or a dedicated media/hardware key
// cannot.
func rebindableCombo(combo string) bool {
	if strings.TrimSpace(combo) == "" {
		return false
	}
	for _, tok := range strings.Split(combo, " + ") {
		if strings.HasPrefix(tok, "mouse") || strings.HasPrefix(tok, "XF86") {
			return false
		}
	}
	return true
}

// exclusiveRow renders a live bind the catalogue does not document, its label the
// bind's description or a reading of its dispatcher.
func exclusiveRow(lb liveBind) BindRow {
	label := strings.TrimSpace(lb.description)
	if label == "" {
		label = describeDispatcher(lb.dispatcher, lb.arg)
	}
	return BindRow{
		ID:         "hyprland." + chordSlug(lb.display),
		Category:   "Hyprland",
		Label:      capitalize(label),
		Keys:       DisplayKeys(lb.display),
		Default:    lb.display,
		Chord:      lb.display,
		Kind:       BindWM,
		Rebindable: false,
	}
}

// customRow renders one of the store's own shortcuts as a Custom row. A
// degenerate empty-exec bind names nothing, so it is left out.
func customRow(i int, k Keybind) (BindRow, bool) {
	if (k.Action == "exec" || k.Action == "") && strings.TrimSpace(k.Value) == "" {
		return BindRow{}, false
	}
	row := BindRow{
		ID:         fmt.Sprintf("custom.%d", i),
		Category:   "Custom",
		Label:      customLabel(k),
		Keys:       DisplayKeys(k.Keys),
		Default:    k.Keys,
		Chord:      k.Keys,
		Kind:       BindCustom,
		Rebindable: false,
	}
	if reason, ok := customUnhonored(k); ok {
		row.Unhonored = reason
	}
	return row, true
}

func customLabel(k Keybind) string {
	switch k.Action {
	case "exec", "":
		return "Run: " + strings.TrimSpace(k.Value)
	case "close":
		return "Close window"
	case "fullscreen":
		return "Fullscreen"
	case "togglefloating":
		return "Float or tile"
	case "workspace":
		return "Focus workspace " + strings.TrimSpace(k.Value)
	case "movetoworkspace":
		return "Send to workspace " + strings.TrimSpace(k.Value)
	case "movetoworkspacesilent":
		return "Send to workspace " + strings.TrimSpace(k.Value) + " quietly"
	}
	return capitalize(k.Action)
}

// customUnhonored mirrors genKeybind: the exec and the three window actions are
// emitted; any other action produces no bind, so the row is reported with a
// reason instead of silently doing nothing.
func customUnhonored(k Keybind) (string, bool) {
	switch k.Action {
	case "exec", "", "close", "fullscreen", "togglefloating":
		return "", false
	}
	return fmt.Sprintf("Hyprland has no bind action for %q.", k.Action), true
}

// describeDispatcher reads a dispatcher and its argument as a short phrase.
func describeDispatcher(dispatcher, arg string) string {
	arg = strings.TrimSpace(arg)
	switch dispatcher {
	case "exec":
		return "Run: " + arg
	case "killactive":
		return "Close window"
	case "togglefloating":
		return "Float or tile"
	case "fullscreen":
		return "Fullscreen"
	case "workspace":
		return "Focus workspace " + arg
	case "movetoworkspace":
		return "Send to workspace " + arg
	case "movetoworkspacesilent":
		return "Send to workspace " + arg + " quietly"
	}
	if arg == "" {
		return dispatcher
	}
	return dispatcher + " " + arg
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// chordSlug makes a chord safe as part of a row id.
func chordSlug(chord string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(chord) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+', r == ' ':
			b.WriteByte('-')
		default:
			fmt.Fprintf(&b, "%x", r)
		}
	}
	return strings.Trim(b.String(), "-")
}
