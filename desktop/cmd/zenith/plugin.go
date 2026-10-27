package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// plugin.go is `zenith plugin`: scaffold a shell plugin, audit one, install it
// from a git repo or a folder, list what is installed, turn one on or off, and
// remove it. Ported from Ryoku's plugin CLI (GPL-3.0). Install stages a copy,
// validates the manifest (relative entry points, a known host set, no symlinks,
// an id that is not a reserved built-in widget), audits it, and copies it into
// ~/.local/share/zenith/plugins/<id>. It never executes anything from the
// plugin.
//
// Ryoku installs through its store's receipt transaction. Zenith's plugin
// folder is the install (the ownership table in agents/agents.md gives the
// directory to "the plugin's own manifest"), so there is no receipt to write and
// none for discover.sh to demand. Placement (on or off, which host, settings)
// lives in ~/.config/zenith/plugins.json, written only by zenith-plugins-place.

func init() {
	register("plugin", "shell plugins: new, audit, install, enable, remove", zenithPlugin)
}

const pluginUsageText = `Usage: zenith plugin <command> [flags]

  new <id> [--bar|--desktop|--popout] [--name N] [--author "N <m>"] [--to DIR]
           [--dry-run]               scaffold a plugin folder and git-init it
                                     (--bar is the default host; adds a panel)
  audit <dir> [--json] [--allow RULE,...]
                                     check a plugin's manifest and run the static
                                     audit; --allow turns a blocking rule into a
                                     warning for this run (alias: validate)
  install <git-url|dir> [--bar] [--yes] [--allow-findings]
                                     fetch, audit and install a plugin; --bar also
                                     puts it on the bar (alias: add)
  enable <id> [--bar|--desktop|--popout]
                                     turn an installed plugin on
  disable <id>                       turn it off, keeping its placement
  remove <id>                        uninstall it and drop its placement
  list [--json]                      installed plugins and their placement

A plugin runs unsandboxed inside the shell with your permissions: install only
what you trust. See docs/plugins.md.
`

type pluginHandler func(args []string, pio pluginIO) int

var pluginSubcommands = map[string]pluginHandler{
	"new":      pluginCmdNew,
	"audit":    pluginCmdAudit,
	"validate": pluginCmdAudit,
	"install":  pluginCmdInstall,
	"add":      pluginCmdInstall,
	"enable":   pluginCmdEnable,
	"disable":  pluginCmdDisable,
	"remove":   pluginCmdRemove,
	"list":     pluginCmdList,
}

func zenithPlugin(args []string) int {
	return pluginDispatch(args, pluginStdIO())
}

func pluginDispatch(args []string, pio pluginIO) int {
	if len(args) == 0 {
		fmt.Fprint(pio.err, pluginUsageText)
		return exitUsage
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(pio.out, pluginUsageText)
		return exitOK
	}
	handler, known := pluginSubcommands[args[0]]
	if !known {
		fmt.Fprintf(pio.err, "zenith plugin: unknown command %q\n\n%s", args[0], pluginUsageText)
		return exitUsage
	}
	return handler(args[1:], pio)
}

// pluginFlags builds a flag set whose own complaints go where the command's go.
func pluginFlags(name, usageText string, pio pluginIO) *flag.FlagSet {
	set := newFlags(name, usageText)
	set.SetOutput(pio.err)
	return set
}

// pluginParse parses a subcommand's arguments, flags before or after the
// positionals. A help request is a success, a bad flag is a usage error.
func pluginParse(set *flag.FlagSet, args []string) ([]string, int, bool) {
	positional, err := parseFlags(set, args)
	if err == nil {
		return positional, exitOK, true
	}
	if errors.Is(err, flag.ErrHelp) {
		return nil, exitOK, false
	}
	return nil, exitUsage, false
}

func pluginFail(pio pluginIO, verb string, err error) int {
	fmt.Fprintf(pio.err, "zenith plugin %s: %v\n", verb, err)
	return exitFail
}

// ---------------------------------------------------------------------------
// Manifest

// pluginKnownHosts is the set a manifest may declare (docs/plugins.md). A
// manifest naming any other host is rejected.
var pluginKnownHosts = map[string]bool{"framePopout": true, "desktopWidget": true, "topbarGlyph": true}

// pluginIDRe is the id grammar discover.sh enforces: lowercase alphanumerics
// and dashes, not starting with a dash.
var pluginIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// pluginBuiltinIDs is the reserved built-in widget id set. It is the fallback
// when the catalogue file cannot be found, so a manifest can never claim a
// built-in id.
var pluginBuiltinIDs = []string{
	"launcher", "workspaces", "status", "memory", "cpu", "volume", "ai", "clock",
	"media", "quick", "network", "battery", "brightness", "power", "bluetooth",
	"cputemp", "gpu", "storage", "layout",
}

const pluginCatalogueRel = "modules/bar/barstyles/controlbar/core/widgets.json"

// pluginCataloguePath is the bar's widget catalogue: the installed shell tree,
// else a checkout found by walking up from the working directory.
func pluginCataloguePath() string {
	installed := filepath.Join("/usr/share/zenith/shell", pluginCatalogueRel)
	if pluginExists(installed) {
		return installed
	}
	legacy := filepath.Join("/usr/share/zenith/shell", "modules/bar/barstyles/qsbar/core/widgets.json")
	if pluginExists(legacy) {
		return legacy
	}
	if dir, err := os.Getwd(); err == nil {
		for depth := 0; depth < 6; depth++ {
			if path := filepath.Join(dir, "shell", pluginCatalogueRel); pluginExists(path) {
				return path
			}
			dir = filepath.Dir(dir)
		}
	}
	return installed
}

// pluginReservedIDs is the set of ids a plugin may not claim: the built-in
// widgets from the catalogue, or the fixed built-in set when it is absent.
func pluginReservedIDs() map[string]bool {
	reserved := map[string]bool{}
	if body, err := os.ReadFile(pluginCataloguePath()); err == nil {
		var catalogue []struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(body, &catalogue) == nil {
			for _, widget := range catalogue {
				if widget.ID != "" {
					reserved[widget.ID] = true
				}
			}
		}
	}
	if len(reserved) == 0 {
		for _, id := range pluginBuiltinIDs {
			reserved[id] = true
		}
	}
	return reserved
}

// pluginManifest is the validated subset of a plugin manifest the CLI uses.
type pluginManifest struct {
	ID      string
	Name    string
	Version string
	Hosts   []string
}

// pluginRawManifest mirrors the on-disk manifest.json shape the CLI reads.
type pluginRawManifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Hosts       []string `json:"hosts"`
	EntryPoints struct {
		Main    string `json:"main"`
		Content string `json:"content"`
	} `json:"entryPoints"`
}

// pluginSafeRel reports whether p is a relative path that stays inside its
// root: no absolute paths, no "" and no ".." escape.
func pluginSafeRel(p string) bool {
	if p == "" || filepath.IsAbs(p) {
		return false
	}
	clean := filepath.Clean(p)
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(os.PathSeparator))
}

// pluginValidateManifest reads and checks <dir>/manifest.json against the
// install rules, returning the validated fields. It never runs plugin code.
func pluginValidateManifest(dir string, reserved map[string]bool) (pluginManifest, error) {
	body, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return pluginManifest{}, fmt.Errorf("no manifest.json in %s", dir)
	}
	var raw pluginRawManifest
	if err := json.Unmarshal(body, &raw); err != nil {
		return pluginManifest{}, fmt.Errorf("manifest.json is not valid JSON: %v", err)
	}
	if !pluginIDRe.MatchString(raw.ID) {
		return pluginManifest{}, fmt.Errorf("manifest id %q must be lowercase letters, digits and dashes", raw.ID)
	}
	if reserved[raw.ID] {
		return pluginManifest{}, fmt.Errorf("%q is a reserved built-in widget id", raw.ID)
	}
	if strings.TrimSpace(raw.Name) == "" {
		return pluginManifest{}, errors.New("manifest is missing a name")
	}
	if strings.TrimSpace(raw.Version) == "" {
		return pluginManifest{}, errors.New("manifest is missing a version")
	}
	if len(raw.Hosts) == 0 {
		return pluginManifest{}, errors.New("manifest declares no hosts")
	}
	for _, host := range raw.Hosts {
		if !pluginKnownHosts[host] {
			return pluginManifest{}, fmt.Errorf("unknown host %q (allowed: framePopout, desktopWidget, topbarGlyph)", host)
		}
	}
	for label, p := range map[string]string{"entryPoints.main": raw.EntryPoints.Main, "entryPoints.content": raw.EntryPoints.Content} {
		if !pluginSafeRel(p) {
			return pluginManifest{}, fmt.Errorf("%s %q must be a relative path with no ..", label, p)
		}
		if !pluginExists(filepath.Join(dir, p)) {
			return pluginManifest{}, fmt.Errorf("%s %q does not exist in the plugin", label, p)
		}
	}
	if link, err := pluginFirstSymlink(dir); err != nil {
		return pluginManifest{}, err
	} else if link != "" {
		return pluginManifest{}, fmt.Errorf("plugin contains a symlink (%s); symlinks are not allowed", link)
	}
	return pluginManifest{ID: raw.ID, Name: raw.Name, Version: raw.Version, Hosts: raw.Hosts}, nil
}

func pluginContains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func pluginReadManifestMap(path string) map[string]any {
	body, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	var manifest map[string]any
	if json.Unmarshal(body, &manifest) != nil || manifest == nil {
		return map[string]any{}
	}
	return manifest
}

func pluginString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func pluginStrings(m map[string]any, key string) []string {
	list, _ := m[key].([]any)
	out := []string{}
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// list

type pluginState struct {
	enabled bool
	host    string
}

// pluginReadState reads the placement file tolerantly: a missing, empty or
// half-written file is no plugins on, not an error.
func pluginReadState() map[string]pluginState {
	out := map[string]pluginState{}
	body, err := os.ReadFile(pluginConfigFile())
	if err != nil {
		return out
	}
	var raw map[string]any
	if json.Unmarshal(body, &raw) != nil {
		return out
	}
	for id, value := range raw {
		entry, ok := value.(map[string]any)
		if !ok {
			continue
		}
		state := pluginState{}
		state.enabled, _ = entry["enabled"].(bool)
		state.host, _ = entry["host"].(string)
		out[id] = state
	}
	return out
}

// pluginRow is one `plugin list` element. Source is "installed" for a folder in
// the data directory and "dev" for one under ZENITH_PLUGINS_DIR.
type pluginRow struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Hosts        []string       `json:"hosts"`
	Dir          string         `json:"dir"`
	Enabled      bool           `json:"enabled"`
	Host         string         `json:"host"`
	Source       string         `json:"source"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
}

// pluginListRows merges the plugin folders with the placement in plugins.json.
// A dev override shadows an installed plugin of the same id, as discover.sh
// loads it, so a duplicate lists once, marked "dev".
func pluginListRows() []pluginRow {
	state := pluginReadState()
	rows := []pluginRow{}
	seen := map[string]bool{}
	scan := func(root, source string) {
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			dir := filepath.Join(root, entry.Name())
			manifest := pluginReadManifestMap(filepath.Join(dir, "manifest.json"))
			id := pluginString(manifest, "id")
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			name := pluginString(manifest, "name")
			if name == "" {
				name = id
			}
			capabilities, _ := manifest["capabilities"].(map[string]any)
			rows = append(rows, pluginRow{
				ID: id, Name: name, Version: pluginString(manifest, "version"),
				Hosts: pluginStrings(manifest, "hosts"), Dir: dir,
				Enabled: state[id].enabled, Host: state[id].host, Source: source,
				Capabilities: capabilities,
			})
		}
	}
	for _, dir := range pluginDevDirs() {
		scan(dir, "dev")
	}
	scan(pluginDataRoot(), "installed")
	return rows
}

func pluginFindRow(id string) (pluginRow, bool) {
	for _, row := range pluginListRows() {
		if row.ID == id {
			return row, true
		}
	}
	return pluginRow{}, false
}

const pluginListUsage = `Usage: zenith plugin list [flags]

Lists the plugins on this machine: the ones installed under
~/.local/share/zenith/plugins and any in ZENITH_PLUGINS_DIR, with whether each
is on and where.

  --json   machine-readable, with each plugin's declared capabilities
`

func pluginCmdList(args []string, pio pluginIO) int {
	set := pluginFlags("plugin list", pluginListUsage, pio)
	asJSON := set.Bool("json", false, "machine-readable output")
	if _, code, ok := pluginParse(set, args); !ok {
		return code
	}
	rows := pluginListRows()
	if *asJSON {
		body, _ := json.Marshal(rows)
		fmt.Fprintln(pio.out, string(body))
		return exitOK
	}
	if len(rows) == 0 {
		fmt.Fprintln(pio.out, "no plugins installed")
		return exitOK
	}
	for _, row := range rows {
		state := "off"
		if row.Enabled {
			state = "on:" + row.Host
		}
		fmt.Fprintf(pio.out, "%-20s %-10s %-9s %-16s %s\n", row.ID, row.Version, row.Source, state, strings.Join(row.Hosts, ","))
	}
	return exitOK
}

// ---------------------------------------------------------------------------
// audit

const pluginAuditUsage = `Usage: zenith plugin audit <dir> [flags]

Checks a plugin's manifest and runs the static security audit over its tree.
Each finding prints as "rule  path:line  message". A blocking finding makes the
command exit 1 and makes "install" refuse; a warning never does.

  --json            the findings as JSON
  --allow RULE,...  treat the named blocking rules as warnings for this run
`

func pluginCmdAudit(args []string, pio pluginIO) int {
	set := pluginFlags("plugin audit", pluginAuditUsage, pio)
	asJSON := set.Bool("json", false, "findings as JSON")
	allowList := set.String("allow", "", "blocking rules to downgrade, comma-separated")
	positional, code, ok := pluginParse(set, args)
	if !ok {
		return code
	}
	if len(positional) != 1 {
		fmt.Fprint(pio.err, pluginAuditUsage)
		return exitUsage
	}
	allow := map[string]bool{}
	for _, rule := range strings.Split(*allowList, ",") {
		if rule = strings.TrimSpace(rule); rule != "" {
			allow[rule] = true
		}
	}
	_, manifestErr := pluginValidateManifest(positional[0], pluginReservedIDs())
	result := pluginAudit(positional[0], allow)
	pluginPrintAudit(pio.out, result, *asJSON)
	if manifestErr != nil {
		return pluginFail(pio, "audit", manifestErr)
	}
	if len(result.Blocking) > 0 {
		return pluginFail(pio, "audit", fmt.Errorf("%d blocking finding(s); fix them or re-run with --allow", len(result.Blocking)))
	}
	return exitOK
}

// ---------------------------------------------------------------------------
// install

const pluginInstallUsage = `Usage: zenith plugin install <git-url|dir> [flags]

Fetches a plugin (clones a git URL, or copies a folder), checks its manifest,
audits it, and installs it into ~/.local/share/zenith/plugins/<id>. Nothing from
the plugin is executed.

A plugin runs unsandboxed inside your shell with your permissions: install only
plugins you trust. Install does not turn a plugin on unless you pass --bar.

  --bar              turn it on, as a bar glyph
  --yes, -y          do not ask for confirmation
  --allow-findings   install despite blocking audit findings
`

func pluginCmdInstall(args []string, pio pluginIO) int {
	set := pluginFlags("plugin install", pluginInstallUsage, pio)
	bar := set.Bool("bar", false, "turn it on, as a bar glyph")
	yes := set.Bool("yes", false, "do not ask for confirmation")
	set.BoolVar(yes, "y", false, "do not ask for confirmation")
	allowFindings := set.Bool("allow-findings", false, "install despite blocking findings")
	positional, code, ok := pluginParse(set, args)
	if !ok {
		return code
	}
	if len(positional) != 1 {
		fmt.Fprint(pio.err, pluginInstallUsage)
		return exitUsage
	}
	if err := pluginInstall(pio, positional[0], *bar, *yes, *allowFindings); err != nil {
		return pluginFail(pio, "install", err)
	}
	return exitOK
}

func pluginInstall(pio pluginIO, source string, bar, yes, allowFindings bool) error {
	// A local folder (a widget written on this desktop, by hand or by an agent)
	// is copied; anything else is a git URL and is cloned.
	local := pluginExists(filepath.Join(source, "manifest.json"))
	if !local && !pluginHas("git") {
		return errors.New("git is required to install a plugin from a URL")
	}

	fmt.Fprintln(pio.out, pio.amber("Warning: a plugin runs unsandboxed inside your shell, with your"))
	fmt.Fprintln(pio.out, pio.amber("permissions. Only install plugins you trust."))
	verb := "Clone"
	if local {
		verb = "Copy"
	}
	if !yes && !pio.confirm(fmt.Sprintf("%s and install %s?", verb, source)) {
		return errors.New("aborted")
	}

	staging, err := os.MkdirTemp("", "zenith-plugin-staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	src := filepath.Join(staging, "src")
	if local {
		if err := pluginCopyTree(source, src); err != nil {
			return fmt.Errorf("copy failed: %w", err)
		}
	} else {
		// "--" keeps a URL from being read as an option; no prompt, because a
		// clone started from the shell's UI has no terminal to answer one.
		clone := exec.Command("git", "clone", "--depth", "1", "--", source, src)
		clone.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if output, err := clone.CombinedOutput(); err != nil {
			return fmt.Errorf("clone failed: %s", pluginTail(string(output), err))
		}
		// The git metadata is not part of the plugin and can carry symlinks.
		_ = os.RemoveAll(filepath.Join(src, ".git"))
	}

	manifest, err := pluginValidateManifest(src, pluginReservedIDs())
	if err != nil {
		return err
	}

	// The same audit `audit` runs. Blocking findings refuse the install unless
	// --allow-findings; warnings are printed and never block.
	result := pluginAudit(src, nil)
	if len(result.Blocking) > 0 || len(result.Warnings) > 0 {
		pluginPrintAudit(pio.out, result, false)
	}
	if len(result.Blocking) > 0 && !allowFindings {
		return fmt.Errorf("%d blocking audit finding(s); fix them or re-run with --allow-findings", len(result.Blocking))
	}

	dest := filepath.Join(pluginDataRoot(), manifest.ID)
	if pluginExists(dest) {
		return fmt.Errorf("plugin %q is already installed (remove it first)", manifest.ID)
	}
	if err := pluginCopyTree(src, dest); err != nil {
		os.RemoveAll(dest)
		return err
	}
	fmt.Fprintf(pio.out, "%s %s (%s) -> %s\n", pio.green("installed"), manifest.Name, manifest.Version, dest)

	if bar {
		if !pluginContains(manifest.Hosts, "topbarGlyph") {
			return fmt.Errorf("%s does not declare the topbarGlyph host, so it cannot go on the bar", manifest.ID)
		}
		if err := pluginTurnOn(manifest.ID, "topbarGlyph"); err != nil {
			return err
		}
		fmt.Fprintln(pio.out, pio.green("enabled")+" on the bar")
	}
	return nil
}

// pluginTurnOn writes the placement. `enabled true` also seeds the manifest's
// default settings; the host is set explicitly afterwards.
func pluginTurnOn(id, host string) error {
	if err := pluginPlace(id, "enabled", "true"); err != nil {
		return err
	}
	return pluginPlace(id, "host", host)
}

// ---------------------------------------------------------------------------
// enable, disable

const pluginEnableUsage = `Usage: zenith plugin enable <id> [flags]

Turns an installed plugin on. Without a host flag it goes where its manifest
defaults say.

  --bar | --desktop | --popout   where to put it (a host the plugin declares)
`

func pluginCmdEnable(args []string, pio pluginIO) int {
	set := pluginFlags("plugin enable", pluginEnableUsage, pio)
	bar := set.Bool("bar", false, "turn it on as a bar glyph")
	desktop := set.Bool("desktop", false, "turn it on as a desktop widget")
	popout := set.Bool("popout", false, "turn it on as a frame popout")
	positional, code, ok := pluginParse(set, args)
	if !ok {
		return code
	}
	if len(positional) != 1 {
		fmt.Fprint(pio.err, pluginEnableUsage)
		return exitUsage
	}
	host, err := pluginHostFlag(*bar, *desktop, *popout)
	if err != nil {
		fmt.Fprintf(pio.err, "zenith plugin enable: %v\n", err)
		return exitUsage
	}
	row, found := pluginFindRow(positional[0])
	if !found {
		return pluginFail(pio, "enable", fmt.Errorf("plugin %q is not installed (zenith plugin list)", positional[0]))
	}
	if host != "" && !pluginContains(row.Hosts, host) {
		return pluginFail(pio, "enable", fmt.Errorf("%s does not declare the %s host (it declares: %s)", row.ID, host, strings.Join(row.Hosts, ", ")))
	}
	if err := pluginPlace(row.ID, "enabled", "true"); err != nil {
		return pluginFail(pio, "enable", err)
	}
	if host != "" {
		if err := pluginPlace(row.ID, "host", host); err != nil {
			return pluginFail(pio, "enable", err)
		}
	}
	fmt.Fprintf(pio.out, "%s %s\n", pio.green("enabled"), row.ID)
	return exitOK
}

// pluginHostFlag turns the --bar/--desktop/--popout trio into a host name,
// refusing more than one.
func pluginHostFlag(bar, desktop, popout bool) (string, error) {
	host, count := "", 0
	for _, choice := range []struct {
		on   bool
		host string
	}{{bar, "topbarGlyph"}, {desktop, "desktopWidget"}, {popout, "framePopout"}} {
		if choice.on {
			host, count = choice.host, count+1
		}
	}
	if count > 1 {
		return "", errors.New("choose one of --bar, --desktop, --popout")
	}
	return host, nil
}

const pluginDisableUsage = `Usage: zenith plugin disable <id>

Turns a plugin off. Its placement and settings are kept, so enabling it again
puts it back where it was.
`

func pluginCmdDisable(args []string, pio pluginIO) int {
	set := pluginFlags("plugin disable", pluginDisableUsage, pio)
	positional, code, ok := pluginParse(set, args)
	if !ok {
		return code
	}
	if len(positional) != 1 {
		fmt.Fprint(pio.err, pluginDisableUsage)
		return exitUsage
	}
	id := positional[0]
	_, installed := pluginFindRow(id)
	if _, placed := pluginReadState()[id]; !installed && !placed {
		return pluginFail(pio, "disable", fmt.Errorf("plugin %q is not installed or placed (zenith plugin list)", id))
	}
	if err := pluginPlace(id, "enabled", "false"); err != nil {
		return pluginFail(pio, "disable", err)
	}
	fmt.Fprintf(pio.out, "%s %s\n", pio.green("disabled"), id)
	return exitOK
}

// ---------------------------------------------------------------------------
// remove

const pluginRemoveUsage = `Usage: zenith plugin remove <id>

Uninstalls a plugin from ~/.local/share/zenith/plugins and drops its placement,
so the shell stops loading it. Its state and cache directories are left alone.
`

func pluginCmdRemove(args []string, pio pluginIO) int {
	set := pluginFlags("plugin remove", pluginRemoveUsage, pio)
	positional, code, ok := pluginParse(set, args)
	if !ok {
		return code
	}
	if len(positional) != 1 {
		fmt.Fprint(pio.err, pluginRemoveUsage)
		return exitUsage
	}
	id := positional[0]
	if !pluginIDRe.MatchString(id) {
		return pluginFail(pio, "remove", fmt.Errorf("%q is not a plugin id", id))
	}
	dest := filepath.Join(pluginDataRoot(), id)
	info, err := os.Lstat(dest)
	if err != nil {
		return pluginFail(pio, "remove", fmt.Errorf("plugin %q is not installed", id))
	}
	// A dev plugin is often a link into a checkout. Removing the link is the
	// whole job; following it would delete the author's repository.
	if info.Mode()&fs.ModeSymlink != 0 {
		err = os.Remove(dest)
	} else {
		err = os.RemoveAll(dest)
	}
	if err != nil {
		return pluginFail(pio, "remove", err)
	}
	// Drop its placement so the shell stops loading it (best-effort).
	if err := pluginPlace(id, "forget"); err != nil {
		fmt.Fprintf(pio.err, "zenith plugin remove: its placement was not cleared: %v\n", err)
	}
	fmt.Fprintf(pio.out, "%s %s\n", pio.green("removed"), id)
	return exitOK
}
