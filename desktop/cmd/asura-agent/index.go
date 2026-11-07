package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// OKFMeta carries metadata required by the Google Open Knowledge Format (OKF v0.1).
type OKFMeta struct {
	Type        string
	Title       string
	Description string
	Resource    string
	Tags        []string
}

// formatOKFDoc produces a valid Google Open Knowledge Format (OKF v0.1) document
// featuring YAML frontmatter, navigation links, and the fenced generated body.
func formatOKFDoc(meta OKFMeta, intro string, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "type: %s\n", meta.Type)
	fmt.Fprintf(&b, "title: %s\n", meta.Title)
	fmt.Fprintf(&b, "description: %s\n", meta.Description)
	if meta.Resource != "" {
		fmt.Fprintf(&b, "resource: %s\n", meta.Resource)
	}
	if len(meta.Tags) > 0 {
		b.WriteString("tags:\n")
		for _, tag := range meta.Tags {
			fmt.Fprintf(&b, "  - %s\n", tag)
		}
	}
	fmt.Fprintf(&b, "timestamp: %s\n", time.Now().UTC().Format(time.RFC3339))
	b.WriteString("---\n\n")
	if intro != "" {
		b.WriteString(intro)
		b.WriteString("\n\n")
	}
	b.WriteString(buildFence(body))
	b.WriteByte('\n')
	return b.String()
}

// writeVaultDoc lays down a generated OKF doc as YAML frontmatter + header + fenced body
// on first write, and thereafter rewrites only the fenced body and updates the frontmatter timestamp
// so user out-of-fence edits survive across reindexes.
func writeVaultDoc(name string, meta OKFMeta, intro string, body string) error {
	path := filepath.Join(VaultDir(), name)
	existing := readFileOrEmpty(path)
	var doc string
	if existing == "" {
		doc = formatOKFDoc(meta, intro, body)
	} else {
		doc = ReplaceFenced(existing, body)
		if strings.HasPrefix(doc, "---\n") {
			re := regexp.MustCompile(`(?m)^timestamp: .+$`)
			newTS := "timestamp: " + time.Now().UTC().Format(time.RFC3339)
			if re.MatchString(doc) {
				doc = re.ReplaceAllString(doc, newTS)
			}
		} else {
			doc = formatOKFDoc(meta, intro, body)
		}
	}
	return atomicWrite(path, []byte(doc), 0o644)
}

// Reindex regenerates the machine-owned OKF vault docs. Every probe is best
// effort, so a missing tool degrades one section rather than failing the index.
func Reindex() error {
	if err := EnsureVault(); err != nil {
		return err
	}
	if err := writeIndexVaultDoc(); err != nil {
		return err
	}
	if err := writeLogVaultDoc(); err != nil {
		return err
	}
	if err := writeSystemVaultDoc(); err != nil {
		return err
	}
	if err := writeDesktopVaultDoc(); err != nil {
		return err
	}
	if err := writePackagesVaultDoc(); err != nil {
		return err
	}
	if err := writeRepoVaultDoc(); err != nil {
		return err
	}
	if err := writeUserVaultDoc(); err != nil {
		return err
	}
	if err := WriteHabits(); err != nil {
		return err
	}
	// Best effort, bounded, and never fatal: mirror the live config under the
	// vault and refresh its prowl index so search_code works off a checkout.
	_ = RefreshSourceMirror()
	return nil
}

// ReindexUser refreshes only the user-owned changes layer; cheap enough for
// the watcher to run whenever the live config drifts.
func ReindexUser() error {
	if err := EnsureVault(); err != nil {
		return err
	}
	return writeUserVaultDoc()
}

func writeIndexVaultDoc() error {
	meta := OKFMeta{
		Type:        "Index",
		Title:       "Asura OS Knowledge Base",
		Description: "Root table of contents and system knowledge graph for Asura OS coding agents.",
		Resource:    "/",
		Tags:        []string{"asura", "knowledge-graph", "index", "system"},
	}
	intro := "# Asura OS Knowledge Base\n\n" +
		"System knowledge graph formatted per Google Open Knowledge Format (OKF v0.1).\n" +
		"Use this graph to discover hardware components, desktop subsystems, configurations, and user patterns."
	return writeVaultDoc("index.md", meta, intro, indexBody())
}

func indexBody() string {
	var b strings.Builder
	b.WriteString("## Knowledge Graph Map\n\n")
	b.WriteString("| Subsystem Document | OKF Type | Resource | Description |\n")
	b.WriteString("| :--- | :--- | :--- | :--- |\n")
	b.WriteString("| [`system.md`](./system.md) | Hardware Component | `/sys` | CPU, GPU, memory, disks, battery, displays, kernel |\n")
	b.WriteString("| [`desktop.md`](./desktop.md) | Desktop Subsystem | `/etc/asura` | Compositor, shell surfaces, configs, reload commands, GUI map |\n")
	b.WriteString("| [`packages.md`](./packages.md) | Package Inventory | `/var/db/xbps` | Installed native packages, Flatpaks, updates |\n")
	b.WriteString("| [`asura-repo.md`](./asura-repo.md) | Repository Architecture | Monorepo root | Asura OS source tree structure and component map |\n")
	b.WriteString("| [`user.md`](./user.md) | User Configuration | `~/.config/asura` | User dotfile overrides and live configuration diffs |\n")
	b.WriteString("| [`habits.md`](./habits.md) | Workflow Patterns | `~/.local/share` | Mined shell history frequency, rhythms, active tools |\n")
	b.WriteString("| [`log.md`](./log.md) | Changelog | `/var/log` | System reindex history and hardware/package transitions |\n\n")

	b.WriteString("## Core Directives\n\n")
	b.WriteString("1. **Read `desktop.md` first**: Do not traverse the filesystem to locate desktop configuration files or daemon controls.\n")
	b.WriteString("2. **GUI-first answers**: Answer desktop configuration questions GUI-first (Settings Hub deep-links) before CLI commands.\n")
	b.WriteString("3. **Respect User Configs**: Never overwrite user overrides documented in `user.md` with default configurations without asking.\n")
	b.WriteString("4. **Agent Memory**: Keep durable agent notes in `memory/` and dated notes in `journal/`.\n")
	return b.String()
}

func writeLogVaultDoc() error {
	meta := OKFMeta{
		Type:        "Changelog",
		Title:       "Asura OS Knowledge Base Changelog",
		Description: "Chronological audit log of system indexing events, hardware updates, and package state transitions.",
		Resource:    "/var/log",
		Tags:        []string{"asura", "changelog", "audit", "history"},
	}
	intro := "# Asura OS Knowledge Base Changelog\n\n" +
		"Audit log tracking system reindex operations and state drift over time.\n\n" +
		"[← Back to Index](./index.md)"

	model, _ := cpuModelCores()
	rel := kernelRelease()
	totalMem, _ := memTotalUsed()
	memStr := "unknown memory"
	if totalMem > 0 {
		memStr = humanBytes(totalMem) + " memory"
	}
	newEntry := fmt.Sprintf("- **%s**: System reindex completed. Hardware: %s (%s), Kernel: %s.",
		time.Now().UTC().Format(time.RFC3339),
		orUnknown(model),
		memStr,
		orUnknown(rel),
	)

	path := filepath.Join(VaultDir(), "log.md")
	existing := readFileOrEmpty(path)
	var prevEntries []string
	if bi := strings.Index(existing, vaultFenceBegin); bi >= 0 {
		if ei := strings.Index(existing, vaultFenceEnd); ei > bi {
			fenceContent := existing[bi+len(vaultFenceBegin) : ei]
			for _, line := range strings.Split(fenceContent, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "- **") {
					prevEntries = append(prevEntries, trimmed)
				}
			}
		}
	}

	var all []string
	all = append(all, newEntry)
	for _, pe := range prevEntries {
		if len(all) >= 20 {
			break
		}
		all = append(all, pe)
	}

	body := "## Reindex Events\n\n" + strings.Join(all, "\n") + "\n"
	return writeVaultDoc("log.md", meta, intro, body)
}

func writeSystemVaultDoc() error {
	meta := OKFMeta{
		Type:        "Hardware Component",
		Title:       "System Hardware and Environment",
		Description: "Live hardware telemetry and topology: CPU, GPU, memory, disks, battery, and displays.",
		Resource:    "/sys",
		Tags:        []string{"asura", "hardware", "cpu", "gpu", "memory", "battery", "disks"},
	}
	intro := "# System Hardware and Environment\n\n" +
		"[← Back to Index](./index.md) · [Desktop Subsystem →](./desktop.md)"
	return writeVaultDoc("system.md", meta, intro, systemBody())
}

func writeDesktopVaultDoc() error {
	meta := OKFMeta{
		Type:        "Desktop Subsystem",
		Title:       "Asura Desktop Subsystem and GUI Map",
		Description: "Subsystem map of compositor, shell surfaces, config paths, reload commands, bar layout, and GUI deep-links.",
		Resource:    "/etc/asura",
		Tags:        []string{"asura", "desktop", "hyprland", "quickshell", "gui-map", "plugins"},
	}
	intro := "# Asura Desktop Map\n\n" +
		"[← Back to Index](./index.md) · [User Configuration →](./user.md)"
	return writeVaultDoc("desktop.md", meta, intro, desktopBody())
}

func writePackagesVaultDoc() error {
	meta := OKFMeta{
		Type:        "Package Inventory",
		Title:       "Installed Packages and Software",
		Description: "Native XBPS package inventory, explicit installations, Flatpaks, and pending updates.",
		Resource:    "/var/db/xbps",
		Tags:        []string{"asura", "packages", "xbps", "flatpak", "software"},
	}
	intro := "# Package Inventory\n\n" +
		"[← Back to Index](./index.md)"
	return writeVaultDoc("packages.md", meta, intro, packagesBody())
}

func batteryDescribe() []string {
	matches, err := filepath.Glob("/sys/class/power_supply/BAT*")
	if err != nil || len(matches) == 0 {
		return nil
	}
	var out []string
	for _, dir := range matches {
		name := filepath.Base(dir)
		capB, _ := os.ReadFile(filepath.Join(dir, "capacity"))
		statusB, _ := os.ReadFile(filepath.Join(dir, "status"))
		capStr := strings.TrimSpace(string(capB))
		statusStr := strings.TrimSpace(string(statusB))
		if capStr != "" {
			if statusStr != "" {
				out = append(out, fmt.Sprintf("%s: %s%% (%s)", name, capStr, statusStr))
			} else {
				out = append(out, fmt.Sprintf("%s: %s%%", name, capStr))
			}
		}
	}
	return out
}

func systemBody() string {
	var b strings.Builder
	model, cores := cpuModelCores()
	total, _ := memTotalUsed()

	b.WriteString("## CPU\n\n")
	fmt.Fprintf(&b, "- Model: %s\n", orUnknown(model))
	fmt.Fprintf(&b, "- Logical cores: %s\n\n", orUnknownInt(cores))

	b.WriteString("## Memory\n\n")
	if total > 0 {
		fmt.Fprintf(&b, "- Total: %s\n\n", humanBytes(total))
	} else {
		b.WriteString("- Total: unknown\n\n")
	}

	b.WriteString("## Kernel\n\n")
	fmt.Fprintf(&b, "- Release: %s\n\n", orUnknown(kernelRelease()))

	b.WriteString("## GPU\n\n")
	if gpus := gpuDescribe(); len(gpus) > 0 {
		for _, g := range gpus {
			fmt.Fprintf(&b, "- %s\n", g)
		}
		b.WriteString("\n")
	} else {
		b.WriteString("- (none detected)\n\n")
	}

	b.WriteString("## Battery & Power\n\n")
	if bats := batteryDescribe(); len(bats) > 0 {
		for _, bt := range bats {
			fmt.Fprintf(&b, "- %s\n", bt)
		}
		b.WriteString("\n")
	} else {
		b.WriteString("- (no battery detected - desktop / AC power)\n\n")
	}

	b.WriteString("## Displays\n\n")
	if mons := monitorRows(); len(mons) > 0 {
		for _, m := range mons {
			fmt.Fprintf(&b, "- %s\n", m)
		}
		b.WriteString("\n")
	} else {
		b.WriteString("- (none detected)\n\n")
	}

	b.WriteString("## Disks\n\n")
	if rows := diskRows(); len(rows) > 0 {
		b.WriteString("| Device | Mount | Size | Used |\n|---|---|---|---|\n")
		for _, r := range rows {
			b.WriteString(r)
			b.WriteByte('\n')
		}
	} else {
		b.WriteString("(no block devices detected)\n")
	}
	return b.String()
}

// desktopMapRow is one verified line of the Asura map.
type desktopMapRow struct {
	subsystem, path, owner, reload string
}

// desktopMap is distilled from docs/structure.md, docs/cli.md, the hub and shell
// READMEs, and the shell IPC surface. Every path, owner, and reload command is
// verified against the repo. The shell exposes no per-component restart verb, so
// shell surfaces reload via `asura reload`.
var desktopMap = []desktopMapRow{
	{"Shell surfaces (bar, dock, sidebar, widgets, launcher, Settings)", "/usr/share/asura/shell/ (packaged, read-only)", "asura-shell daemon", "systemctl --user reload asura-shell"},
	{"Shell look config", "~/.config/asura/shell.json", "asura-shell daemon (its only writer)", "applies live"},
	{"Settings store", "GSettings org.asura.shell and org.asura.desktop", "asura config set / Asura Settings", "written by Asura Settings; no reload"},
	{"User overrides", "~/.config/asura/user_edits/ (wins at the mirrored path)", "asura materialize", "asura materialize"},
	{"Terminal", "~/.config/ghostty/", "Ghostty", "relaunch ghostty"},
	{"Shell + prompt", "~/.zshrc, ~/.config/starship.toml", "zsh; starship", "open a new shell"},
	{"Editor", "~/.config/zed/", "Zed", "relaunch zed"},
	{"File manager", "dconf org.gnome.nautilus.*", "Nautilus", "relaunch nautilus"},
	{"Asura CLI", "binary `asura`; state ~/.local/state/asura/", "asura", "asura update | status | materialize | doctor"},
	{"Palette", "~/.cache/asura/colors.json", "asura-shell daemon (asura theme generate)", "asura-wallpaper wallpaper repaint"},
	{"Packages", "XBPS database / Flatpak", "xbps-install, xbps-query, flatpak", "asura update"},
	{"System vault (OKF)", "~/.local/share/asura/" + productName + "/", binName, binName + " index"},
}

// actingBody is the first thing desktop.md says: how an agent changes this
// desktop. It names the shipped `asura` skill with its resolved path, so an
// agent that only followed the vault pointer (and never loaded the skill by
// itself) still reaches the plugin and command contracts instead of reading
// shell source.
func actingBody() string {
	var b strings.Builder
	b.WriteString("## Acting on this desktop\n\n")
	b.WriteString("Answer a \"how do I\" question GUI-first: when the change has a GUI path (a\n")
	b.WriteString("Asura Hub page, the wallpaper/theme picker, or QS Bar Settings; see the GUI\n")
	b.WriteString("map below), name that path first with its keybind, then give the command as\n")
	b.WriteString("the headless fallback. You still act through commands (`asura`, `asura-shell`,\n")
	b.WriteString("`asura-hubd`, `asura-wallpaper`, `asura wm act`), never by editing shipped files. The\n")
	b.WriteString("`asura` skill is the contract for that: `SKILL.md` (rules and the command\n")
	b.WriteString("catalogue), `gui.md` (the GUI map), `bar.md` (the QS Bar model), `plugins.md`\n")
	b.WriteString("(how a new widget is written and installed).\n")
	if d := skillSourceDir(); d != "" {
		fmt.Fprintf(&b, "Read them at `%s/` (also linked into each agent's skills dir as `asura`).\n", d)
	} else {
		b.WriteString("It is linked into each agent's skills dir as `asura` by `" + binName + " wire`.\n")
	}
	b.WriteString("\n")
	return b.String()
}

// guiSections is the Asura Hub page list the GUI map advertises, in rail order.
// Only pages a user routinely reaches for are listed; each is deep-linked with
// `asura-shell hub open <key>`. Compositor-gated pages are hidden at runtime
// when the active window manager cannot back them, so a listed row may not
// appear on a given box.
var guiSections = []struct{ name, key string }{
	{"General", "global"},
	{"Updates", "updates"},
	{"Displays", "displays"},
	{"Connections", "connections"},
	{"Input", "input"},
	{"Graphics & Power", "gpu"},
	{"Animations", "animations"},
	{"Lockscreen", "lockscreen"},
	{"Window Manager", "windowmanager"},
	{"Plugins", "plugins"},
	{"Bar Studio", "bar-studio"},
	{"Desktop", "desktop"},
	{"Widgets", "widgets"},
	{"App Launcher", "launcher"},
	{"Keybinds", "keybinds"},
	{"Performance", "performance"},
	{"Session", "session"},
	{"Recording", "recording"},
	{"Dictation", "dictation"},
	{"Add-ons", "addons"},
	{"Agent", "agent"},
}

// guiMapBody renders the GUI-first map: the wallpaper/theme picker, QS Bar
// Settings, the app launcher, and every routinely-used Asura Hub page with its
// deep link, so an agent names the GUI path before the command behind it.
func guiMapBody() string {
	var b strings.Builder
	b.WriteString("## GUI map\n\n")
	b.WriteString("Most desktop changes have a GUI surface; name it first, then the command as\n")
	b.WriteString("the headless fallback. Compositor-gated pages are hidden when the running\n")
	b.WriteString("window manager cannot back them, so a row here may not appear on every box.\n\n")
	b.WriteString("| Want to change | Open | Deep link | Command behind it |\n|---|---|---|---|\n")
	b.WriteString("| Wallpaper or theme | Wallpaper picker (Super+W), or the bar Wallpaper widget | - | `asura-wallpaper wallpaper ui` |\n")
	b.WriteString("| Bar or dock layout | QS Bar Settings | - | `asura-shell bar settings` |\n")
	b.WriteString("| Launch an app | App launcher (Super+Space) | - | - |\n")
	for _, s := range guiSections {
		fmt.Fprintf(&b, "| Asura Hub: %s | Super+comma | `asura-shell hub open %s` | - |\n", s.name, s.key)
	}
	b.WriteString("\nIdle and power timeouts live under Graphics & Power (`gpu`).\n\n")
	return b.String()
}

// asuraPackages are queried for installed versions in desktop.md.
var asuraPackages = []string{"asura"}

func desktopBody() string {
	var b strings.Builder
	b.WriteString(actingBody())
	b.WriteString(guiMapBody())
	b.WriteString("## Subsystem map\n\n")
	b.WriteString("| Subsystem | Config path | Owner | Reload |\n|---|---|---|---|\n")
	b.WriteString("| Hyprland (compositor) | ~/.config/asura/" + compositorConfigDir + "/ | Hyprland; Asura Settings writes settings.conf and rebinds.conf, user.conf is yours | `hyprctl reload` |\n")
	for _, r := range desktopMap {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.subsystem, mdCell(r.path), r.owner, mdCell(r.reload))
	}
	b.WriteString("\n## Asura package versions\n\n")
	any := false
	for _, pkg := range asuraPackages {
		if v := packageVersion(pkg); v != "" {
			fmt.Fprintf(&b, "- %s %s\n", pkg, v)
			any = true
		} else {
			fmt.Fprintf(&b, "- %s (not installed)\n", pkg)
		}
	}
	if !any {
		b.WriteString("\n(no Asura packages installed; likely a dev checkout)\n")
	}
	b.WriteString(barSectionBody())
	return b.String()
}

// ---- the QS Bar catalogue ---------------------------------------------------

// barWidget mirrors one entry of the qsbar widget catalogue (widgets.json) and
// the plugin rows `asura-shell bar catalog --json` adds. Only id and label are
// guaranteed; a reader tolerates a widget that omits visKey or settings.
type barWidget struct {
	ID       string       `json:"id"`
	Label    string       `json:"label"`
	VisKey   string       `json:"visKey"`
	Kind     string       `json:"kind"`
	Hosts    []string     `json:"hosts"`
	Settings []barSetting `json:"settings"`
}

type barSetting struct {
	Key string `json:"key"`
}

// barCatalogFile resolves the shipped widgets.json: the live config first, then
// the packaged base, then the dev checkout the deploy recorded. Returns "" when
// none exists.
func barCatalogFile() string {
	const rel = "shell/modules/bar/barstyles/controlbar/core/widgets.json"
	const legacyRel = "shell/modules/bar/barstyles/qsbar/core/widgets.json"
	cands := []string{
		filepath.Join(configHome(), "asura", rel),
		filepath.Join(configHome(), "asura", legacyRel),
		filepath.Join(baseConfigDir(), rel),
		filepath.Join(baseConfigDir(), legacyRel),
	}
	if repo := recordedCheckout(); repo != "" {
		cands = append(cands, filepath.Join(repo, rel), filepath.Join(repo, legacyRel))
	}
	for _, p := range cands {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// loadBuiltinWidgets reads the built-in widget catalogue. It stays the source of
// the visibility key the daemon's JSON omits, so built-ins always come from the
// file. Returns (nil, "") when the file is absent or unparseable.
func loadBuiltinWidgets() ([]barWidget, string) {
	path := barCatalogFile()
	if path == "" {
		return nil, ""
	}
	var ws []barWidget
	if json.Unmarshal([]byte(readFileOrEmpty(path)), &ws) != nil {
		return nil, ""
	}
	return ws, path
}

// loadPluginWidgets asks the daemon for installed bar-capable plugins. Without a
// running daemon (or its bar CLI) it returns nil, so the section degrades to
// built-ins only. The query is read-only.
func loadPluginWidgets() []barWidget {
	out, ok := probe(5, "asura-shell", "bar", "catalog", "--json")
	if !ok {
		return nil
	}
	var all []barWidget
	if json.Unmarshal([]byte(out), &all) != nil {
		return nil
	}
	var plugins []barWidget
	for _, w := range all {
		if w.Kind == "plugin" {
			plugins = append(plugins, w)
		}
	}
	return plugins
}

// settingKeys joins a catalogue widget's setting keys for the id table.
func settingKeys(rows []barSetting) string {
	var keys []string
	for _, r := range rows {
		if r.Key != "" {
			keys = append(keys, r.Key)
		}
	}
	if len(keys) == 0 {
		return "(none)"
	}
	return strings.Join(keys, ", ")
}

// barSectionBody renders the "Bar and dock" markdown, fenced by its own
// markers so it can be located and updated without rewriting all of desktop.md.
func barSectionBody() string {
	var b strings.Builder
	b.WriteString("\n## Bar and dock\n\n")
	b.WriteString(vaultBarFenceBegin + "\n")
	b.WriteString("The default bar is QS Bar (`qsbar`). Its layout is data in\n")
	b.WriteString("`~/.config/asura/shell.json` under `qsbar.layout`:\n\n")
	b.WriteString("```json\n")
	b.WriteString("\"qsbar\": { \"layout\": { \"version\": 1,\n")
	b.WriteString("  \"left\": [ids], \"center\": [ids], \"right\": [ids] } }\n")
	b.WriteString("```\n\n")
	b.WriteString("An entry is a widget id (a built-in below, or an installed plugin's id).\n")
	b.WriteString("Placement and visibility are separate: a built-in renders when\n")
	b.WriteString("`qsbar.widgets[<key>]` is true (a blank key is always shown); a plugin when\n")
	b.WriteString("`~/.config/asura/plugins.json` has it `enabled` with `host: \"topbarGlyph\"`.\n")
	b.WriteString("The shell daemon is the only writer of `shell.json`; change the bar with\n")
	b.WriteString("`asura-shell bar ...`, never by editing the file.\n\n")

	builtins, src := loadBuiltinWidgets()
	b.WriteString("### Built-in widgets\n\n")
	if len(builtins) == 0 {
		b.WriteString("(widget catalogue not found; deploy or install the shell to populate it)\n\n")
	} else {
		b.WriteString("| id | label | visibility key | settings keys |\n|---|---|---|---|\n")
		for _, w := range builtins {
			vis := w.VisKey
			if vis == "" {
				vis = "(always)"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", w.ID, mdCell(w.Label), mdCell(vis), mdCell(settingKeys(w.Settings)))
		}
		fmt.Fprintf(&b, "\nSource: `%s`.\n\n", tildeAbbrev(src))
	}

	if plugins := loadPluginWidgets(); len(plugins) > 0 {
		b.WriteString("### Installed bar plugins\n\n")
		b.WriteString("| id | label | hosts | settings keys |\n|---|---|---|---|\n")
		for _, w := range plugins {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", w.ID, mdCell(w.Label), mdCell(strings.Join(w.Hosts, ", ")), mdCell(settingKeys(w.Settings)))
		}
		b.WriteString("\n")
	}

	b.WriteString("### Commands\n\n")
	b.WriteString("```\n")
	b.WriteString("bar list [--json] | catalog [--json]\n")
	b.WriteString("bar move <id> --section left|center|right [--index N | --before <id> | --after <id>]\n")
	b.WriteString("bar show <id> | hide <id> | set <id> <key> <value>\n")
	b.WriteString("bar position top|bottom | form full|fit|dock|notch|islands | defaults | settings [route]\n")
	b.WriteString("dock show|hide | edge auto|top|bottom|left|right | autohide on|off | pin <app>|unpin <app> | list [--json]\n")
	b.WriteString("```\n\n")
	b.WriteString("Run these through `asura-shell` (e.g. `asura-shell bar move clock --section right`).\n")
	b.WriteString("Installed plugins live in `~/.local/share/asura/plugins/<id>/`; their bar\n")
	b.WriteString("placement and settings persist in `~/.config/asura/plugins.json`.\n\n")
	b.WriteString("### Adding a widget\n\n")
	b.WriteString("A widget that is not in the table above is a plugin, never an edit to the\n")
	b.WriteString("shipped QML under `/usr/share/asura/shell/`. Read the\n")
	b.WriteString("`asura` skill's `plugins.md` for the contract (manifest.json, service/Main.qml,\n")
	b.WriteString("content/Widget.qml, host `topbarGlyph`), write it in its own directory, then:\n\n")
	b.WriteString("```\n")
	b.WriteString("asura plugin validate <dir>\n")
	b.WriteString("asura plugin add <dir-or-git-url> --bar --yes\n")
	b.WriteString("```\n\n")
	b.WriteString("It lands on the bar and under QS Bar Settings > Community (`asura-shell bar\n")
	b.WriteString("settings community`); `asura plugin remove <id>` takes it off again, and\n")
	b.WriteString("`asura plugin share <id>` (only when the user asks to publish) opens the\n")
	b.WriteString("store pull request that lists it for everyone.\n")
	b.WriteString(vaultBarFenceEnd + "\n")
	return b.String()
}

func packagesBody() string {
	var b strings.Builder

	// Native Void Linux package manager (xbps)
	if out, ok := probe(5, "xbps-query", "-l"); ok {
		lines := nonEmptyLines(out)
		fmt.Fprintf(&b, "## Total installed (XBPS)\n\n%d packages\n\n", len(lines))
		if mout, mok := probe(5, "xbps-query", "-m"); mok {
			mlines := nonEmptyLines(mout)
			b.WriteString("## Explicitly installed (XBPS)\n\n")
			fmt.Fprintf(&b, "<details><summary>%d manually installed packages</summary>\n\n", len(mlines))
			for _, l := range mlines {
				fmt.Fprintf(&b, "- %s\n", l)
			}
			b.WriteString("\n</details>\n\n")
		}
		if uout, uok := probe(5, "xbps-install", "-un"); uok {
			ulines := nonEmptyLines(uout)
			fmt.Fprintf(&b, "## Pending updates\n\n%d packages\n\n", len(ulines))
		}
	} else if out, ok := probe(5, "dpkg-query", "-f", ".\n", "-W"); ok {
		// Fallback for dpkg
		fmt.Fprintf(&b, "## Total installed\n\n%d packages\n\n", len(nonEmptyLines(out)))
		if mout, mok := probe(5, "apt-mark", "showmanual"); mok {
			lines := nonEmptyLines(mout)
			b.WriteString("## Explicitly installed\n\n")
			fmt.Fprintf(&b, "<details><summary>%d manually installed packages</summary>\n\n", len(lines))
			for _, l := range lines {
				fmt.Fprintf(&b, "- %s\n", l)
			}
			b.WriteString("\n</details>\n\n")
		}
		if uout, uok := probe(5, "apt", "list", "--upgradable"); uok {
			n := 0
			for _, l := range nonEmptyLines(uout) {
				if strings.Contains(l, "/") {
					n++
				}
			}
			fmt.Fprintf(&b, "## Pending updates\n\n%d packages\n\n", n)
		}
	} else {
		b.WriteString("## Total installed\n\n(package manager query unavailable)\n\n")
	}

	// Flatpaks if present
	if fout, fok := probe(5, "flatpak", "list", "--app"); fok {
		flines := nonEmptyLines(fout)
		if len(flines) > 0 {
			b.WriteString("## Flatpaks\n\n")
			fmt.Fprintf(&b, "<details><summary>%d Flatpak applications</summary>\n\n", len(flines))
			for _, fl := range flines {
				fmt.Fprintf(&b, "- %s\n", fl)
			}
			b.WriteString("\n</details>\n\n")
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func gpuDescribe() []string {
	if _, err := exec.LookPath("asura-gpu-detect"); err == nil {
		if out, ok := probe(5, "asura-gpu-detect"); ok {
			if lines := nonEmptyLines(out); len(lines) > 0 {
				return lines
			}
		}
	}
	out, ok := probe(5, "lspci")
	if !ok {
		return nil
	}
	var gpus []string
	for _, l := range nonEmptyLines(out) {
		ll := strings.ToLower(l)
		if strings.Contains(ll, "vga compatible controller") ||
			strings.Contains(ll, "3d controller") ||
			strings.Contains(ll, "display controller") {
			gpus = append(gpus, strings.TrimSpace(l))
		}
	}
	return gpus
}

// lsblkNode mirrors `lsblk -J` with nullable string fields, tolerating any that
// the running lsblk omits.
type lsblkNode struct {
	Name       string      `json:"name"`
	Mountpoint *string     `json:"mountpoint"`
	Size       *string     `json:"size"`
	Fsused     *string     `json:"fsused"`
	Children   []lsblkNode `json:"children"`
}

func diskRows() []string {
	out, ok := probe(5, "lsblk", "-J", "-o", "NAME,MOUNTPOINT,SIZE,FSUSED")
	if !ok {
		return nil
	}
	var doc struct {
		BlockDevices []lsblkNode `json:"blockdevices"`
	}
	if json.Unmarshal([]byte(out), &doc) != nil {
		return nil
	}
	var rows []string
	var walk func(n lsblkNode)
	walk = func(n lsblkNode) {
		if n.Mountpoint != nil && *n.Mountpoint != "" {
			rows = append(rows, fmt.Sprintf("| %s | %s | %s | %s |",
				n.Name, *n.Mountpoint, orDash(n.Size), orDash(n.Fsused)))
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, d := range doc.BlockDevices {
		walk(d)
	}
	return rows
}

func packageVersion(pkg string) string {
	if out, ok := probe(5, "xbps-query", "-p", "pkgver", pkg); ok && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out)
	}
	out, ok := probe(5, "dpkg-query", "-W", "-f", "${Version}", pkg)
	if !ok {
		return ""
	}
	return strings.TrimSpace(out)
}

// probe runs a command with a timeout, returning its stdout and whether it
// succeeded. It is the single best-effort exec path for the indexer.
func probe(seconds int, name string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func orUnknownInt(n int) string {
	if n <= 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d", n)
}

func orDash(s *string) string {
	if s == nil || *s == "" {
		return "-"
	}
	return *s
}

// mdCell escapes the pipe so a path or command never breaks the table.
func mdCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
