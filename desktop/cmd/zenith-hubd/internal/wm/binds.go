package wm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// The bind legend's vocabulary. The shipped catalogue is session/binds.json, the
// one list the cheat sheet, `zenith doctor` and this backend all read; Ryoku
// compiled the same list into its seam, which here would be a second copy of a
// file that already exists. What stays in code is the chord algebra: how a
// catalogue chord is spelled, normalised, expanded into a family and shown.

// BindKind groups a bind by who acts on it, so a consumer can colour or section
// the legend without reading the id.
type BindKind string

const (
	BindWM     BindKind = "wm"     // the window manager performs it
	BindShell  BindKind = "shell"  // the Zenith shell performs it
	BindApp    BindKind = "app"    // it launches or focuses an app
	BindCustom BindKind = "custom" // a bind outside the shipped catalogue
)

// BindRow is one line of the effective bind legend: a shipped bind resolved
// against the user's rebinds, or a bind the catalogue does not own.
type BindRow struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Label    string `json:"label"`
	Hint     string `json:"hint,omitempty"`
	// Keys are the display tokens of the effective chord, ready for the sheet.
	Keys []string `json:"keys"`
	// Default is the shipped chord in Hub form ("SUPER + Q"), the key a rebind is
	// recorded over. A family keeps its {n} so the Hub knows it stands for ten.
	Default string `json:"default"`
	// Chord is the effective chord after the user's rebind, what the session emits.
	Chord      string   `json:"chord"`
	Kind       BindKind `json:"kind"`
	Rebindable bool     `json:"rebindable"`
	Locked     bool     `json:"locked"`
	// Unhonored, when set, is why the running compositor does not perform this
	// bind. The row is still listed so the sheet reads honestly rather than
	// silently dropping a bind the user expects to see.
	Unhonored string `json:"unhonored,omitempty"`
}

// CatalogBind is one shipped bind, as session/binds.json describes it.
type CatalogBind struct {
	ID       string
	Category string
	Label    string
	Hint     string
	Chord    string
	Kind     BindKind
	Locked   bool
	// Action, Argument and ReleaseAction are the catalogue's own words for what
	// the chord does, kept so a rebind can be written when the packaged
	// configuration cannot be read.
	Action        string
	Argument      string
	ReleaseAction string
	// Family means Chord carries {n} and stands for ten binds, n = 1..10, on the
	// digit key n%10 so the tenth is the 0 key. Numpad families read the number
	// pad instead, KP_1..KP_9 then KP_0.
	Family bool
	Numpad bool
}

type catalogueFile struct {
	Categories []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"categories"`
	Binds []struct {
		ID            string `json:"id"`
		Category      string `json:"category"`
		Label         string `json:"label"`
		Hint          string `json:"hint"`
		Chord         string `json:"chord"`
		Kind          string `json:"kind"`
		Action        string `json:"action"`
		Argument      string `json:"argument"`
		ReleaseAction string `json:"releaseAction"`
		Locked        bool   `json:"locked"`
	} `json:"binds"`
}

// DecodeCatalogue parses binds.json. Unknown fields are tolerated on purpose:
// the file belongs to the compositor lane, and a field added there must not
// blank this page.
func DecodeCatalogue(raw []byte) ([]CatalogBind, error) {
	var f catalogueFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("bind catalogue: %w", err)
	}
	labels := map[string]string{}
	for _, c := range f.Categories {
		labels[c.ID] = c.Label
	}
	out := make([]CatalogBind, 0, len(f.Binds))
	for _, b := range f.Binds {
		cat := labels[b.Category]
		if cat == "" {
			cat = b.Category
		}
		out = append(out, CatalogBind{
			ID:            b.ID,
			Category:      cat,
			Label:         b.Label,
			Hint:          b.Hint,
			Chord:         b.Chord,
			Kind:          BindKind(b.Kind),
			Locked:        b.Locked || chordHasToken(b.Chord, func(t string) bool { return strings.HasPrefix(t, "XF86") }),
			Action:        b.Action,
			Argument:      b.Argument,
			ReleaseAction: b.ReleaseAction,
			Family:        strings.Contains(b.Chord, "{n}"),
			Numpad:        strings.Contains(b.Chord, "KP_{n}"),
		})
	}
	return out, nil
}

// CataloguePath is where the packaged catalogue is read from: an explicit
// override for tests and development trees, the user's materialised copy, then
// the package.
func CataloguePath() string {
	if p := os.Getenv("ZENITH_BINDS_JSON"); p != "" {
		return p
	}
	for _, p := range []string{
		filepath.Join(configHome(), "zenith", "session", "binds.json"),
		"/usr/share/zenith/session/binds.json",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "/usr/share/zenith/session/binds.json"
}

// LoadCatalogue reads the shipped catalogue from CataloguePath.
func LoadCatalogue() ([]CatalogBind, error) {
	raw, err := os.ReadFile(CataloguePath())
	if err != nil {
		return nil, err
	}
	return DecodeCatalogue(raw)
}

func chordHasToken(chord string, want func(string) bool) bool {
	for _, tok := range strings.Split(chord, "+") {
		if want(strings.TrimSpace(tok)) {
			return true
		}
	}
	return false
}

// ExpandChord substitutes a family placeholder for the nth member's key: {n}
// becomes the digit n%10, so the tenth member lands on the 0 key, and a KP_{n}
// family carries that same digit onto the number pad (KP_1..KP_0). A chord with
// no placeholder is returned unchanged, so a plain chord passes straight through.
func ExpandChord(chord string, n int) string {
	return strings.Replace(chord, "{n}", strconv.Itoa(n%10), 1)
}

// Expand is the ten chords of a family, or the single chord of a plain bind.
func (c CatalogBind) Expand() []string {
	if !c.Family {
		return []string{c.Chord}
	}
	out := make([]string, 0, 10)
	for n := 1; n <= 10; n++ {
		out = append(out, ExpandChord(c.Chord, n))
	}
	return out
}

// FamilyRebind resolves a family's stored rebind. A family binds ten chords that
// share one key position (the {n} placeholder) and differ only by modifier, so a
// user rebinds the whole family with a single entry that keeps the placeholder
// and changes only the modifiers: ["SUPER + {n}"] = "SUPER + CTRL + {n}". It
// returns the stored chord and true when the value is that shape, the {n} key
// kept in place and every other token a modifier, otherwise def and false.
// Guarding the shape keeps a stray value from collapsing ten workspace keys onto
// one chord.
func FamilyRebind(def string, rebinds map[string]string) (string, bool) {
	to := strings.TrimSpace(rebinds[def])
	if to == "" || to == def {
		return def, false
	}
	parts := strings.Split(to, " + ")
	defParts := strings.Split(def, " + ")
	if parts[len(parts)-1] != defParts[len(defParts)-1] {
		return def, false
	}
	for _, tok := range parts[:len(parts)-1] {
		if _, isMod := modifierOrder[strings.ToUpper(tok)]; !isMod {
			return def, false
		}
	}
	return to, true
}

// numpadNumLockOff maps each number-pad digit keysym to the keysym the same key
// sends with NumLock off. A family on the number pad binds both so a workspace
// shortcut works whichever way NumLock happens to sit.
var numpadNumLockOff = map[string]string{
	"KP_1": "KP_End",
	"KP_2": "KP_Down",
	"KP_3": "KP_Next",
	"KP_4": "KP_Left",
	"KP_5": "KP_Begin",
	"KP_6": "KP_Right",
	"KP_7": "KP_Home",
	"KP_8": "KP_Up",
	"KP_9": "KP_Prior",
	"KP_0": "KP_Insert",
}

// NumpadAliases returns the chord again with its number-pad digit swapped for the
// NumLock-off keysym, or nothing when the chord holds no number-pad digit. The
// token is matched whole, so KP_1 never trips on a longer neighbour.
func NumpadAliases(chord string) []string {
	parts := strings.Split(chord, " + ")
	for i, p := range parts {
		if off, ok := numpadNumLockOff[p]; ok {
			aliased := append([]string(nil), parts...)
			aliased[i] = off
			return []string{strings.Join(aliased, " + ")}
		}
	}
	return []string{}
}

// displayNames turns a raw keysym into the token the sheet shows a user.
var displayNames = map[string]string{
	"SUPER":                 "Super",
	"SHIFT":                 "Shift",
	"CTRL":                  "Ctrl",
	"ALT":                   "Alt",
	"Return":                "Enter",
	"Escape":                "Esc",
	"space":                 "Space",
	"Prior":                 "Page Up",
	"Next":                  "Page Down",
	"grave":                 "`",
	"comma":                 ",",
	"bracketleft":           "[",
	"bracketright":          "]",
	"mouse:272":             "LMB",
	"mouse:273":             "RMB",
	"mouse_up":              "Scroll Up",
	"mouse_down":            "Scroll Down",
	"XF86AudioRaiseVolume":  "Vol +",
	"XF86AudioLowerVolume":  "Vol -",
	"XF86AudioMute":         "Mute",
	"XF86AudioPlay":         "Play",
	"XF86AudioNext":         "Next",
	"XF86AudioPrev":         "Prev",
	"XF86MonBrightnessUp":   "Brightness +",
	"XF86MonBrightnessDown": "Brightness -",
	"XF86TouchpadToggle":    "Touchpad",
	"XF86TouchpadOn":        "Touchpad On",
	"XF86TouchpadOff":       "Touchpad Off",
}

// DisplayKeys splits a chord into the tokens the sheet renders. A family
// placeholder becomes its range ("1 … 0", "Num 1 … 0"), a number-pad digit reads
// "Num 3", and everything else takes its readable face or passes through.
func DisplayKeys(chord string) []string {
	parts := strings.Split(chord, " + ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, displayToken(p))
	}
	return out
}

func displayToken(tok string) string {
	if name, ok := displayNames[tok]; ok {
		return name
	}
	switch tok {
	case "{n}":
		return "1 \u2026 0"
	case "KP_{n}":
		return "Num 1 \u2026 0"
	}
	if d, ok := strings.CutPrefix(tok, "KP_"); ok && len(d) == 1 && d[0] >= '0' && d[0] <= '9' {
		return "Num " + d
	}
	return tok
}

// modifierOrder is the canonical modifier sequence a normalised chord carries, so
// two spellings of the same combo collapse to one string for clash detection.
var modifierOrder = map[string]int{
	"SUPER": 0,
	"CTRL":  1,
	"ALT":   2,
	"SHIFT": 3,
}

// NormChord is a chord's clash-detection form: modifiers upper-cased and put in
// canonical order, single spaces, the key last. Two chords that bind the same
// keys in a different spelling normalise to the same string.
func NormChord(chord string) string {
	var mods, keys []string
	for _, raw := range strings.Split(chord, "+") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		up := strings.ToUpper(tok)
		if _, isMod := modifierOrder[up]; isMod {
			mods = append(mods, up)
			continue
		}
		keys = append(keys, tok)
	}
	sort.SliceStable(mods, func(i, j int) bool {
		return modifierOrder[mods[i]] < modifierOrder[mods[j]]
	})
	return strings.Join(append(mods, keys...), " + ")
}

// SplitChord returns the hyprlang pair for a catalogue chord: the modifier list
// ("SUPER SHIFT") and the key. Hyprland reads the modifiers space separated and
// the key after a comma, so "SUPER + SHIFT + 1" becomes ("SUPER SHIFT", "1").
func SplitChord(chord string) (mods, key string) {
	var ms []string
	for _, raw := range strings.Split(chord, "+") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		if _, isMod := modifierOrder[strings.ToUpper(tok)]; isMod {
			ms = append(ms, strings.ToUpper(tok))
			continue
		}
		key = tok
	}
	sort.SliceStable(ms, func(i, j int) bool { return modifierOrder[ms[i]] < modifierOrder[ms[j]] })
	return strings.Join(ms, " "), key
}
