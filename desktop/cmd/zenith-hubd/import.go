package main

// Config import: bring a migrating user's existing dotfiles onto Zenith without
// losing them and without breaking the desktop. `import scan` reports what was
// found (apps, items, keybind conflicts) as JSON; `import apply` layers the
// chosen config on top of Zenith's defaults (Hyprland binds/rules into the hub
// Overrides model, everything else into each app's user-include) after a full
// backup; `import undo` restores a prior import from that backup. The verbs and
// JSON are the cross-slice contract shared with the CLI and Hub UI; see
// docs/config-import.md.
//
// This file owns the verb dispatch, the scan orchestration, and the pieces both
// scan and apply share: combo normalization and Zenith's shipped-bind legend.
// import_parse.go has the per-app parsers, import_apply.go the apply + backup,
// import_undo.go the restore.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"zenith/desktop/cmd/zenith-hubd/internal/wm"
)

// --- JSON contract (lowerCamel field names are the cross-slice interface) ----

type scanResult struct {
	Source string    `json:"source"`
	Apps   []scanApp `json:"apps"`
}

type scanApp struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Present   bool           `json:"present"`
	Path      string         `json:"path"`
	Tier      string         `json:"tier"` // deep | layer | drop
	Summary   string         `json:"summary"`
	Items     []scanItem     `json:"items"`
	Conflicts []scanConflict `json:"conflicts"`
}

type scanItem struct {
	Kind       string `json:"kind"` // bind|windowrule|monitor|env|exec|setting|raw|file
	Raw        string `json:"raw"`
	Combo      string `json:"combo,omitempty"`      // bind only, display form
	Dispatcher string `json:"dispatcher,omitempty"` // bind only, native token
	Ingestable bool   `json:"ingestable"`
}

type scanConflict struct {
	Combo  string         `json:"combo"` // display form
	Norm   string         `json:"norm"`  // stable key, opaque to UI/CLI
	Zenith conflictZenith `json:"zenith"`
	Mine   conflictMine   `json:"mine"`
	Kind   string         `json:"kind"` // shipped | duplicate
}

type conflictZenith struct {
	Action string `json:"action"`
	Desc   string `json:"desc"`
}

type conflictMine struct {
	Raw  string `json:"raw"`
	Desc string `json:"desc"`
}

// decisions is the apply input. An app absent or include:false is skipped; a
// conflict absent from Conflicts defaults to "zenith" (Zenith's bind kept).
type decisions struct {
	Source    string                     `json:"source"`
	Apps      map[string]appDecision     `json:"apps"`
	Conflicts map[string]json.RawMessage `json:"conflicts"`
}

type appDecision struct {
	Include bool `json:"include"`
}

type applyResult struct {
	Ts            string   `json:"ts"`
	BackupDir     string   `json:"backupDir"`
	FilesWritten  []string `json:"filesWritten"`
	BindsIngested int      `json:"bindsIngested"`
	RulesIngested int      `json:"rulesIngested"`
	Unbinds       int      `json:"unbinds"`

	// Unresolved lists imported bind lines whose dispatcher has no hl.dsp
	// translation, so they were preserved as a "port by hand" comment and did
	// not take effect (and never triggered an unbind of a shipped chord).
	Unresolved []string `json:"unresolved,omitempty"`
}

type undoResult struct {
	Ts       string   `json:"ts"`
	Restored []string `json:"restored"`
}

type importManifest struct {
	Ts       string         `json:"ts"`
	Snapshot string         `json:"snapshot,omitempty"`
	Files    []manifestFile `json:"files"`
}

// manifestFile records one backed-up file. Backup is empty when the file did
// not exist before the import, so undo removes it instead of restoring bytes.
type manifestFile struct {
	Path   string `json:"path"`
	Backup string `json:"backup"`
}

// --- verb dispatch -----------------------------------------------------------

func runImport(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("import needs scan|apply|undo")
	}
	switch args[0] {
	case "scan":
		if len(args) < 2 {
			return fmt.Errorf("import scan needs a path or url")
		}
		src, err := resolveSource(args[1])
		if err != nil {
			return err
		}
		return printJSON(scanSource(src))
	case "apply":
		if len(args) < 2 {
			return fmt.Errorf("import apply needs a decisions file or -")
		}
		dec, err := readDecisions(args[1])
		if err != nil {
			return err
		}
		res, err := applyImport(dec)
		if err != nil {
			return err
		}
		return printJSON(res)
	case "undo":
		ts := ""
		if len(args) > 1 {
			ts = args[1]
		}
		res, err := undoImport(ts)
		if err != nil {
			return err
		}
		return printJSON(res)
	default:
		return fmt.Errorf("unknown import subcommand: %s", args[0])
	}
}

// resolveSource turns the scan argument into a local directory. A git URL is
// cloned to a temp dir whose path is returned as the ScanResult source, so apply
// can reuse it; a plain path is made absolute and must exist.
func resolveSource(arg string) (string, error) {
	if isGitURL(arg) {
		dir, err := os.MkdirTemp("", "zenith-import-*")
		if err != nil {
			return "", err
		}
		cmd := exec.Command("git", "clone", "--depth", "1", arg, dir)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("clone %s: %w", arg, err)
		}
		return dir, nil
	}
	abs, err := filepath.Abs(arg)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("source not found: %s", abs)
	}
	return abs, nil
}

func isGitURL(s string) bool {
	return strings.Contains(s, "://") ||
		strings.HasPrefix(s, "git@") ||
		strings.HasSuffix(s, ".git")
}

func readDecisions(arg string) (decisions, error) {
	var b []byte
	var err error
	if arg == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(arg)
	}
	if err != nil {
		return decisions{}, err
	}
	var d decisions
	if err := json.Unmarshal(b, &d); err != nil {
		return decisions{}, fmt.Errorf("parse decisions: %w", err)
	}
	return d, nil
}

// --- scan orchestration ------------------------------------------------------

// knownApps are the apps with a first-class parser or layer target, in the order
// they appear in the review UI. Anything else the user brought is a generic drop.
var knownApps = map[string]bool{"hypr": true, "kitty": true, "fish": true, "fastfetch": true}

func scanSource(source string) scanResult {
	res := scanResult{Source: source, Apps: []scanApp{}}
	if a, ok := scanHyprland(source); ok {
		res.Apps = append(res.Apps, a)
	}
	if a, ok := scanKitty(source); ok {
		res.Apps = append(res.Apps, a)
	}
	if a, ok := scanFish(source); ok {
		res.Apps = append(res.Apps, a)
	}
	if a, ok := scanFastfetch(source); ok {
		res.Apps = append(res.Apps, a)
	}
	res.Apps = append(res.Apps, scanGeneric(source)...)
	return res
}

// findConfig returns the first existing candidate under the source, trying the
// bare dir, a <name>/ subdir, and a nested .config/<name>/ so a raw dotfiles
// tree, a config-home export, and a whole home directory all work.
func findConfig(source, dir, file string) (string, bool) {
	for _, cand := range []string{
		filepath.Join(source, dir, file),
		filepath.Join(source, ".config", dir, file),
		filepath.Join(source, file),
	} {
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, true
		}
	}
	return "", false
}

// --- combo normalization (shared by scan conflict detection and apply) -------

// normCombo maps a display combo ("SUPER + Q") to a stable, order-independent
// key ("q+super"): modifiers canonicalized, everything lowercased, tokens
// sorted. Two combos collide iff they bind the same chord, whatever the source
// wrote for the modifier or the token order.
func normCombo(combo string) string {
	var toks []string
	for _, p := range strings.Split(combo, "+") {
		if p = strings.TrimSpace(p); p != "" {
			toks = append(toks, normToken(p))
		}
	}
	sort.Strings(toks)
	return strings.Join(toks, "+")
}

func normToken(t string) string {
	switch strings.ToUpper(t) {
	case "SUPER", "MOD", "MAINMOD", "WIN", "META", "LOGO", "SUPER_L", "SUPER_R":
		return "super"
	case "CTRL", "CONTROL", "CONTROL_L", "CONTROL_R":
		return "ctrl"
	case "ALT", "MOD1", "ALT_L", "ALT_R":
		return "alt"
	case "SHIFT", "SHIFT_L", "SHIFT_R":
		return "shift"
	}
	return strings.ToLower(t)
}

// --- Zenith's shipped bind legend (for conflict detection + unbinds) ----------

// shippedInfo is one shipped chord: Combo is the form an unbind keys on,
// Action/Desc describe it for the conflict UI.
type shippedInfo struct {
	Combo  string
	Action string
	Desc   string
}

// zenithShipped reads the bind catalogue into norm -> shippedInfo. A family is
// expanded to its ten concrete chords so a Super+N import is caught as a shadow.
func zenithShipped() map[string]shippedInfo {
	out := map[string]shippedInfo{}
	cat, err := wm.LoadCatalogue()
	if err != nil {
		return out
	}
	for _, c := range cat {
		action := "exec"
		if c.Kind == wm.BindWM {
			action = "dispatch"
			switch c.Action {
			case "killactive":
				action = "close"
			case "fullscreen", "togglefloating", "workspace", "movefocus", "movewindow":
				action = c.Action
			}
		}
		for _, combo := range c.Expand() {
			if n := normCombo(combo); n != "" {
				if _, ok := out[n]; !ok {
					out[n] = shippedInfo{Combo: combo, Action: action, Desc: c.Label}
				}
			}
		}
	}
	return out
}

func describeExec(cmd string) string {
	if strings.HasPrefix(cmd, "zenith-app ") {
		return strings.TrimPrefix(cmd, "zenith-app ")
	}
	switch {
	case cmd == "kitty":
		return "terminal"
	case cmd == "nautilus":
		return "files"
	case cmd == "chromium":
		return "browser"
	case strings.Contains(cmd, "hyprpicker"):
		return "pick a color"
	case strings.Contains(cmd, "set-volume") && strings.Contains(cmd, "%+"):
		return "volume up"
	case strings.Contains(cmd, "set-volume"):
		return "volume down"
	case strings.Contains(cmd, "set-mute"):
		return "mute toggle"
	case strings.Contains(cmd, "play-pause"):
		return "play / pause"
	case strings.Contains(cmd, "playerctl next"):
		return "next track"
	case strings.Contains(cmd, "playerctl previous"):
		return "previous track"
	}
	return cmd
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}
