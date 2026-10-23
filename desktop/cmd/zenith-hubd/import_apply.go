package main

// Apply a config import: re-scan the source, resolve every keybind conflict per
// the decisions, back up every file to be touched (plus a best-effort snapper
// snapshot), then write. Ingestable Hyprland binds and window rules land in
// desktop.json (the provider regenerates the compositor config from it); a "mine"
// shadow also records an unbind so the imported bind wins over the shipped one.
// Everything else layers into each app's user-include inside a marked,
// idempotent, undoable block. Writes are
// validated (Lua parse) before committing and rolled back from the backup on any
// error.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"zenith/desktop/cmd/zenith-hubd/internal/wm"
)

// pendingWrite is one file apply will replace; confCheck marks hyprlang output that
// must parse before anything is committed.
type pendingWrite struct {
	path      string
	content   []byte
	confCheck bool
}

// conflictChoice is a decoded conflict decision. mode is "zenith" (default),
// "mine", or "remap"; remap carries the new chord for the remap case.
type conflictChoice struct {
	mode  string
	remap string
}

func backupsDir() string { return filepath.Join(configHome(), "zenith", "import-backups") }

func applyImport(dec decisions) (applyResult, error) {
	if strings.TrimSpace(dec.Source) == "" {
		return applyResult{}, fmt.Errorf("decisions.source is empty")
	}
	ts := time.Now().Format("20060102-150405")
	res := applyResult{Ts: ts, BackupDir: filepath.Join(backupsDir(), ts)}

	resolve := map[string]conflictChoice{}
	for norm, raw := range dec.Conflicts {
		resolve[norm] = parseChoice(raw)
	}

	var writes []pendingWrite
	var ingBinds []Keybind
	var ingRules []WindowRule
	var ingUnbinds []string
	binds, rules, unbinds := 0, 0, 0
	var unresolved []string

	// --- Hyprland: ingest into desktop.json + a user.conf raw block ------------
	if included(dec, "hyprland") {
		if hpath, ok := findConfig(dec.Source, "hypr", "hyprland.conf"); ok {
			b, _ := os.ReadFile(hpath)
			_, hs := parseHyprland(string(b))
			plan := planBinds(hs.Binds, zenithShipped(), resolve)

			var userConf []string
			for _, kb := range plan.keep {
				if kb.Ingestable {
					ingBinds = append(ingBinds, Keybind{Keys: kb.Combo, Action: kb.Action, Value: kb.Value})
					binds++
					continue
				}
				if nativeBindOK(kb.Native) {
					userConf = append(userConf, kb.Native)
				} else {
					userConf = append(userConf, portByHand(kb))
					unresolved = append(unresolved, kb.Raw)
				}
			}
			ingUnbinds = append(ingUnbinds, plan.unbinds...)
			unbinds += len(plan.unbinds)

			for _, r := range hs.Rules {
				ingRules = append(ingRules, r.Rule)
				rules++
			}
			for _, rl := range hs.Raws {
				if rl.Conf != "" {
					userConf = append(userConf, rl.Conf)
				} else {
					userConf = append(userConf, commentPreserve(rl.Raw))
				}
			}
			if len(userConf) > 0 {
				// user.conf is the user's own file and wins over the generated ones;
				// the overlay copy keeps it across a materialize.
				target := filepath.Join(wm.HyprConfigDir(), "user.conf")
				body := headerComment("#", ts, dec.Source) + strings.Join(userConf, "\n") + "\n"
				content := upsertBlock(readFileOr(target), ts, body, "#")
				writes = append(writes,
					pendingWrite{path: target, content: []byte(content), confCheck: true},
					pendingWrite{path: filepath.Join(wm.UserEditsHyprDir(), "user.conf"), content: []byte(content)})
			}
		}
	}

	// --- layer tiers: append each app's config into its user-include ----------
	if included(dec, "kitty") {
		if p, ok := findConfig(dec.Source, "kitty", "kitty.conf"); ok {
			body := headerComment("#", ts, dec.Source) + ensureTrailingNL(readFileOr(p))
			writes = append(writes, layerWrite("kitty", "user.conf", "#", ts, body))
		}
	}
	if included(dec, "fish") {
		if p, ok := findConfig(dec.Source, "fish", "config.fish"); ok {
			body := headerComment("#", ts, dec.Source) + fishBody(filepath.Dir(p), p)
			writes = append(writes, layerWrite("fish", "user.fish", "#", ts, body))
		}
	}
	if included(dec, "fastfetch") {
		if p, ok := findConfig(dec.Source, "fastfetch", "config.jsonc"); ok {
			// jsonc uses // comments; the marked block stays a valid comment.
			body := headerComment("//", ts, dec.Source) + ensureTrailingNL(readFileOr(p))
			writes = append(writes, layerWrite("fastfetch", "user.jsonc", "//", ts, body))
		}
	}
	// --- drop tier: any other config the user brought -------------------------
	for _, ga := range scanGeneric(dec.Source) {
		if !included(dec, ga.ID) {
			continue
		}
		var parts []string
		for _, it := range ga.Items {
			parts = append(parts, fmt.Sprintf("# --- %s ---\n", it.Raw)+ensureTrailingNL(readFileOr(filepath.Join(ga.Path, it.Raw))))
		}
		body := headerComment("#", ts, dec.Source) + strings.Join(parts, "")
		writes = append(writes, layerWrite(ga.ID, "user.conf", "#", ts, body))
	}

	// --- persist the ingested binds/rules into desktop.json -------------------
	ingested := binds > 0 || rules > 0 || unbinds > 0
	if ingested {
		ns := readJSONMap(desktopStorePath())
		appendDesktopSection(ns, "keybinds", ingBinds)
		appendDesktopSection(ns, "windowRules", ingRules)
		appendDesktopSection(ns, "unbinds", ingUnbinds)
		writes = append(writes, pendingWrite{path: desktopStorePath(), content: mustJSON(ns)})
		// The provider re-authors these from the store below. Back them up like
		// any other file so an undo restores the exact pre-import config instead
		// of leaving a freshly generated one behind.
		writes = append(writes, generatedConfigWrites()...)
	}

	man, err := backup(res.BackupDir, ts, writes)
	if err != nil {
		return res, err
	}
	for _, w := range writes {
		if w.confCheck && !confContentOK(w.content) {
			return res, fmt.Errorf("generated config for %s failed validation", w.path)
		}
	}
	for _, w := range writes {
		if err := atomicWrite(w.path, w.content, 0o644); err != nil {
			rollback(man)
			return res, fmt.Errorf("write %s: %w (rolled back)", w.path, err)
		}
	}
	if ingested {
		// the provider re-authors settings.conf from the store and reloads.
		_ = applyDesktop()
	}

	res.FilesWritten = relPaths(writePaths(writes))
	res.BindsIngested, res.RulesIngested, res.Unbinds = binds, rules, unbinds
	res.Unresolved = unresolved
	return res, nil
}

// appendDesktopSection appends a typed slice to desktop.<key> in ns, creating the
// array when absent. An empty slice is a no-op.
func appendDesktopSection(ns map[string]any, key string, items any) {
	ib, _ := json.Marshal(items)
	var add []json.RawMessage
	_ = json.Unmarshal(ib, &add)
	if len(add) == 0 {
		return
	}
	d := childMap(ns, "desktop")
	var cur []json.RawMessage
	if raw, ok := d[key]; ok {
		b, _ := json.Marshal(raw)
		_ = json.Unmarshal(b, &cur)
	}
	d[key] = append(cur, add...)
}

// bindPlan is the resolved set of imported binds to apply (remaps already
// folded into Combo/Norm) plus the shipped chords to unbind so a "mine" wins.
type bindPlan struct {
	keep    []importBind
	unbinds []string
}

// planBinds turns the imported binds + conflict decisions into the apply plan.
// It re-derives the same conflict classification scan reports (shipped shadows
// win over self-duplicates, one owner per chord) so the decision keyed by norm
// lands on the right bind.
func planBinds(binds []importBind, shipped map[string]shippedInfo, resolve map[string]conflictChoice) bindPlan {
	kind := map[string]string{}
	firstOf := map[string]int{}
	shippedWinner := map[string]int{}
	dupFirst := map[string]int{}
	dupSecond := map[string]int{}
	for i, b := range binds {
		n := b.Norm
		if n == "" || kind[n] != "" {
			continue
		}
		if _, ok := shipped[n]; ok {
			kind[n] = "shipped"
			shippedWinner[n] = i
			continue
		}
		if fi, ok := firstOf[n]; ok {
			kind[n] = "duplicate"
			dupFirst[n] = fi
			dupSecond[n] = i
			continue
		}
		firstOf[n] = i
	}

	plan := bindPlan{}
	unbound := map[string]bool{}
	for i, b := range binds {
		n := b.Norm
		ch := resolve[n] // absent => zero value => default ("zenith")
		switch kind[n] {
		case "shipped":
			if i != shippedWinner[n] {
				continue // extra bind on an already-shadowed chord: drop
			}
			switch ch.mode {
			case "mine":
				plan.keep = append(plan.keep, b)
				// Only release the shipped chord when the user's replacement will
				// actually take effect (ingestable, or a translatable dispatcher);
				// unbinding for a comment-only bind would leave a dead key.
				if bindApplies(b) {
					if si, ok := shipped[n]; ok && !unbound[n] {
						plan.unbinds = append(plan.unbinds, si.Combo)
						unbound[n] = true
					}
				}
			case "remap":
				plan.keep = append(plan.keep, remapBind(b, ch.remap))
			}
		case "duplicate":
			switch ch.mode {
			case "mine":
				if i == dupSecond[n] {
					plan.keep = append(plan.keep, b)
				}
			case "remap":
				if i == dupFirst[n] {
					plan.keep = append(plan.keep, b)
				} else if i == dupSecond[n] {
					plan.keep = append(plan.keep, remapBind(b, ch.remap))
				}
			default: // zenith: keep the first, drop the rest
				if i == dupFirst[n] {
					plan.keep = append(plan.keep, b)
				}
			}
		default:
			plan.keep = append(plan.keep, b)
		}
	}
	return plan
}

func remapBind(b importBind, combo string) importBind {
	b.Combo = combo
	b.Norm = normCombo(combo)
	return b
}

// parseChoice decodes a decisions.conflicts value: a bare string ("zenith" /
// "mine") or a { "remap": "<combo>" } object.
func parseChoice(raw json.RawMessage) conflictChoice {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return conflictChoice{mode: s}
	}
	var obj struct {
		Remap string `json:"remap"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Remap != "" {
		return conflictChoice{mode: "remap", remap: obj.Remap}
	}
	return conflictChoice{}
}

// --- backup + rollback -------------------------------------------------------

func backup(dir, ts string, writes []pendingWrite) (importManifest, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return importManifest{}, err
	}
	snapshotBestEffort(ts)
	man := importManifest{Ts: ts}
	for i, w := range writes {
		mf := manifestFile{Path: w.path}
		if data, err := os.ReadFile(w.path); err == nil {
			bp := filepath.Join(dir, fmt.Sprintf("%03d-%s", i, filepath.Base(w.path)))
			if err := os.WriteFile(bp, data, 0o644); err != nil {
				return man, err
			}
			mf.Backup = bp
		}
		man.Files = append(man.Files, mf)
	}
	mb, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return man, err
	}
	return man, os.WriteFile(filepath.Join(dir, "manifest.json"), mb, 0o644)
}

func rollback(man importManifest) {
	for _, f := range man.Files {
		if f.Backup != "" {
			if data, err := os.ReadFile(f.Backup); err == nil {
				_ = atomicWrite(f.Path, data, 0o644)
			}
			continue
		}
		_ = os.Remove(f.Path) // file did not exist before the import
	}
}

// snapshotBestEffort takes a snapper snapshot when snapper is installed; it is a
// belt-and-suspenders layer over the per-file backup, so failures are ignored.
func snapshotBestEffort(ts string) {
	if _, err := exec.LookPath("snapper"); err != nil {
		return
	}
	// -c root + -c number so it uses the root config and prunes under NUMBER_LIMIT
	// like update snapshots, instead of piling up untagged forever.
	_ = exec.Command("snapper", "-c", "root", "create", "-c", "number", "-d", "zenith config import "+ts).Run()
	_ = exec.Command("snapper", "-c", "root", "cleanup", "number").Run()
}

// --- write helpers -----------------------------------------------------------

func layerWrite(app, name, prefix, ts, body string) pendingWrite {
	target := filepath.Join(configHome(), app, name)
	return pendingWrite{path: target, content: []byte(upsertBlock(readFileOr(target), ts, body, prefix))}
}

// upsertBlock replaces any prior zenith-import block (idempotent re-import) then
// appends the new one, so a re-scan overwrites rather than stacks.
func upsertBlock(existing, ts, body, prefix string) string {
	cleaned := removeImportBlock(existing, prefix)
	block := prefix + " >>> zenith-import " + ts + " >>>\n" + body + prefix + " <<< zenith-import <<<\n"
	if strings.TrimSpace(cleaned) == "" {
		return block
	}
	if !strings.HasSuffix(cleaned, "\n") {
		cleaned += "\n"
	}
	return cleaned + block
}

func removeImportBlock(s, prefix string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	inBlock := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, prefix+" >>> zenith-import ") {
			inBlock = true
			continue
		}
		if inBlock && strings.HasPrefix(t, prefix+" <<< zenith-import") {
			inBlock = false
			continue
		}
		if !inBlock {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func headerComment(prefix, ts, source string) string {
	return prefix + " imported " + ts + " from " + source + "\n"
}

// commentPreserve keeps a raw Hyprland line in user.conf as a comment, so an
// unclassified line is never lost and never breaks the file.
func commentPreserve(raw string) string { return "# zenith-import: " + raw }

// knownHyprlandDispatchers names dispatchers accepted by Hyprland's parser.
// Used as a fallback syntax gate when Hyprland is not installed.
var knownHyprlandDispatchers = map[string]bool{
	"exec": true, "execr": true, "pass": true, "killactive": true, "closewindow": true,
	"workspace": true, "movetoworkspace": true, "movetoworkspacesilent": true,
	"togglefloating": true, "fullscreen": true, "fullscreenstate": true, "fakefullscreen": true,
	"dpms": true, "pin": true, "movefocus": true, "movewindow": true, "swapwindow": true,
	"centerwindow": true, "resizeactive": true, "moveactive": true, "resizewindowpixel": true,
	"movewindowpixel": true, "cyclenext": true, "swapnext": true, "tagwindow": true,
	"focuswindow": true, "focusmonitor": true, "movecursortocorner": true, "movecursor": true,
	"splitratio": true, "toggleopaque": true, "movecurrentworkspacetomonitor": true,
	"focusworkspaceoncurrentmonitor": true, "swapactiveworkspaces": true, "bringactivetotop": true,
	"alterzorder": true, "togglegroup": true, "changegroupactive": true, "moveintogroup": true,
	"moveoutofgroup": true, "lockgroups": true, "lockactivegroup": true, "movewindoworgroup": true,
	"setignoregrouplock": true, "global": true, "submap": true, "sendshortcut": true,
	"event": true, "togglespecialworkspace": true, "renameworkspace": true, "exit": true,
	"forcerendererreload": true,
}

func parseDispatcherFromLine(line string) string {
	parts := strings.Split(line, ",")
	if len(parts) >= 3 {
		return strings.TrimSpace(parts[2])
	}
	return ""
}

// bindApplies reports whether an imported bind will actually take effect:
// ingestable (a GUI Keybind) or a native line Hyprland accepts in user.conf.
func bindApplies(b importBind) bool { return b.Ingestable || nativeBindOK(b.Native) }

// nativeBindOK asks the compositor's parser whether one bind line stands alone.
// Hyprland rejects a dispatcher it does not know at parse time, and one such
// line would put an error banner on every login, so it is kept as a comment.
func nativeBindOK(line string) bool {
	if strings.TrimSpace(line) == "" {
		return false
	}
	if _, err := exec.LookPath("Hyprland"); err == nil {
		return confContentOK([]byte(line + "\n"))
	}
	disp := strings.ToLower(parseDispatcherFromLine(line))
	return knownHyprlandDispatchers[disp]
}

// portByHand keeps a bind Hyprland would reject as a comment, naming the
// dispatcher the user must port manually; it does not take effect.
func portByHand(b importBind) string {
	return fmt.Sprintf("# zenith-import (port by hand: %s): %s", b.Dispatcher, b.Raw)
}

// fishBody concatenates config.fish and every functions/ + conf.d/ file into one
// user.fish body; fish sources it late, so the appended functions and snippets
// take effect on top of the shipped config.
func fishBody(dir, configPath string) string {
	parts := []string{"# --- config.fish ---\n" + ensureTrailingNL(readFileOr(configPath))}
	for _, sub := range []string{"functions", "conf.d"} {
		entries, _ := os.ReadDir(filepath.Join(dir, sub))
		names := []string{}
		for _, e := range entries {
			if !e.IsDir() {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			parts = append(parts, fmt.Sprintf("# --- %s/%s ---\n", sub, name)+ensureTrailingNL(readFileOr(filepath.Join(dir, sub, name))))
		}
	}
	return strings.Join(parts, "")
}

func included(dec decisions, id string) bool {
	a, ok := dec.Apps[id]
	return ok && a.Include
}

func readFileOr(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func ensureTrailingNL(s string) string {
	if s == "" || strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func writePaths(writes []pendingWrite) []string {
	out := make([]string, 0, len(writes))
	for _, w := range writes {
		out = append(out, w.path)
	}
	return out
}

// relPaths reports absolute paths relative to the config home, deduped and
// sorted, for the JSON summaries.
func relPaths(paths []string) []string {
	base := configHome()
	set := map[string]bool{}
	for _, p := range paths {
		r, err := filepath.Rel(base, p)
		if err != nil {
			r = p
		}
		set[r] = true
	}
	out := make([]string, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// confContentOK is the write-time syntax gate: the compositor's own parser when
// it is installed, else just non-empty. A torn write is the failure it catches.
func confContentOK(b []byte) bool {
	if strings.TrimSpace(string(b)) == "" {
		return false
	}
	bin, err := exec.LookPath("Hyprland")
	if err != nil {
		return true
	}
	f, err := os.CreateTemp("", "zenith-import-*.conf")
	if err != nil {
		return true
	}
	name := f.Name()
	_, _ = f.Write(b)
	_ = f.Close()
	defer os.Remove(name)
	out, _ := exec.Command(bin, "--verify-config", "-c", name).CombinedOutput()
	return strings.Contains(string(out), "config ok")
}
