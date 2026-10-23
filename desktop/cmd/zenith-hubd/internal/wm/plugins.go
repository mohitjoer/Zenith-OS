package wm

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// The compositor-plugin subsystem, cut down from Ryoku's. Ryoku built plugins
// from git on the user's machine against the exact compositor build and tracked
// when a copy went stale. Ubuntu packages the plugins it supports
// (hyprland-plugin-*), each pinned to the compositor ABI by a package dependency,
// so a copy cannot go stale and building here is not needed: the roster reports
// what is installed and loaded, the config is written for what is installed, and
// the verbs that built or removed a copy say so instead of pretending.
//
// Plugins Ryoku built itself (hyprglass, imgborders, dynamic cursors, key
// sounds) have no Ubuntu package. They stay in the store so a rice that names
// them still imports, and unhonored() reports each one that is switched on.

type bundledPlugin struct {
	ID      string // key in the store (wm.hyprland.plugins.<id>)
	Name    string // what the Hub tab is called
	Loaded  string // the name Hyprland reports in `plugin list`
	Package string // the Ubuntu package; empty when none exists
	Docs    string
	Desc    string
}

var bundledPlugins = []bundledPlugin{
	{ID: "hyprbars", Name: "Title bars", Loaded: "hyprbars", Package: "hyprland-plugin-hyprbars",
		Docs: "https://github.com/hyprwm/hyprland-plugins/tree/main/hyprbars",
		Desc: "Title bars with close and maximise buttons on every window."},
	{ID: "hyprfocus", Name: "Focus flash", Loaded: "hyprfocus", Package: "hyprland-plugin-hyprfocus",
		Docs: "https://github.com/hyprwm/hyprland-plugins/tree/main/hyprfocus",
		Desc: "A brief flash or shrink on the window that takes focus."},
	{ID: "hyprscrolling", Name: "Scrolling layout", Loaded: "hyprscrolling", Package: "hyprland-plugin-hyprscrolling",
		Docs: "https://github.com/hyprwm/hyprland-plugins/tree/main/hyprscrolling",
		Desc: "A horizontally scrolling tiling layout (core in Hyprland 0.54)."},
}

// unpackagedPlugins are the store blocks Zenith keeps but cannot run, with the
// reason shown beside each one that is switched on.
var unpackagedPlugins = map[string]string{
	"dynamicCursors": "The dynamic cursors plugin has no Ubuntu package.",
	"imgborders":     "The image borders plugin has no Ubuntu package.",
	"hyprglass":      "The glass plugin has no Ubuntu package.",
	"keysounds":      "The key sounds plugin is Ryoku's own and Zenith does not build it.",
}

func bundledByID(id string) (bundledPlugin, bool) {
	for _, d := range bundledPlugins {
		if d.ID == id {
			return d, true
		}
	}
	return bundledPlugin{}, false
}

// pluginDirs are the directories a packaged plugin lands in: Debian's multiarch
// plugin directory, with the plain lib directory and the local prefix after it.
func pluginDirs() []string {
	dirs, _ := filepath.Glob("/usr/lib/*/hyprland/plugins")
	dirs = append(dirs, "/usr/lib/hyprland/plugins", "/usr/local/lib/hyprland/plugins")
	return dirs
}

var pluginRoot = ""

// pluginSoPath finds the installed copy of a bundled plugin, "" when none is.
func pluginSoPath(id string) string {
	dirs := pluginDirs()
	if pluginRoot != "" {
		dirs = []string{pluginRoot}
	}
	for _, dir := range dirs {
		for _, name := range []string{"lib" + id + ".so", id + ".so"} {
			p := filepath.Join(dir, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
	}
	return ""
}

func scrollingAvailable() bool { return pluginSoPath("hyprscrolling") != "" }

// pluginEnabled reads a bundled plugin's switch out of the override set.
func pluginEnabled(o Overrides, id string) bool {
	p := o.Plugins
	switch id {
	case "hyprbars":
		return p.Hyprbars.Enabled
	case "hyprfocus":
		return p.Hyprfocus.Enabled
	case "hyprscrolling":
		return o.Appearance.Layout == "scrolling"
	}
	return false
}

// loadedPlugins is the set of plugin names Hyprland reports as loaded. The
// listing is text: one "Plugin <name> by <author>:" header per plugin.
func loadedPlugins() map[string]bool {
	out, err := ctl("plugin", "list")
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "Plugin ")
		if !ok {
			continue
		}
		if name, _, found := strings.Cut(rest, " by "); found {
			set[strings.TrimSpace(name)] = true
		}
	}
	return set
}

// pkgVersion asks dpkg for an installed package's version, "" when it is not
// installed or dpkg-query is absent. Read-only.
func pkgVersion(pkg string) string {
	if _, err := exec.LookPath("dpkg-query"); err != nil {
		return ""
	}
	out, err := exec.Command("dpkg-query", "-W", "-f=${Version}", pkg).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// pluginInfo is one row of the roster, in the shape the Plugins page reads.
type pluginInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Bundled     bool   `json:"bundled"`
	Installed   bool   `json:"installed"`
	Loaded      bool   `json:"loaded"`
	Enabled     bool   `json:"enabled"`
	Status      string `json:"status"`
	Source      string `json:"source,omitempty"`
	Package     string `json:"package,omitempty"`
	Version     string `json:"version,omitempty"`
	Path        string `json:"path,omitempty"`
	Docs        string `json:"docs,omitempty"`
	Desc        string `json:"desc,omitempty"`
	Detail      string `json:"detail,omitempty"`
	Rebuildable bool   `json:"rebuildable"`
	Removable   bool   `json:"removable"`
}

type pluginRoster struct {
	Available  bool   `json:"available"`
	Reason     string `json:"reason,omitempty"`
	Compositor struct {
		Version string `json:"version,omitempty"`
		Commit  string `json:"commit,omitempty"`
	} `json:"compositor"`
	Toolchain struct {
		OK      bool     `json:"ok"`
		Missing []string `json:"missing"`
	} `json:"toolchain"`
	Plugins []pluginInfo `json:"plugins"`
}

func compositorVersion() (version, commit string) {
	out, err := ctl("version", "-j")
	if err != nil {
		return "", ""
	}
	var v struct {
		Tag    string `json:"tag"`
		Commit string `json:"commit"`
	}
	if json.Unmarshal(out, &v) != nil {
		return "", ""
	}
	return strings.TrimPrefix(strings.TrimSpace(v.Tag), "v"), strings.TrimSpace(v.Commit)
}

func buildPluginRoster(o Overrides) pluginRoster {
	var r pluginRoster
	r.Available = true
	r.Compositor.Version, r.Compositor.Commit = compositorVersion()
	// Building a plugin from source is not offered, so the toolchain is reported
	// as unavailable with the reason; the page then hides the build controls
	// instead of offering a button that cannot work.
	r.Toolchain.Missing = []string{"building plugins from source is not supported on Zenith; install the packaged ones with apt"}
	loaded := map[string]bool{}
	if live() {
		loaded = loadedPlugins()
	}
	for _, d := range bundledPlugins {
		so := pluginSoPath(d.ID)
		p := pluginInfo{
			ID: d.ID, Name: d.Name, Bundled: true, Installed: so != "", Path: so,
			Enabled: pluginEnabled(o, d.ID), Docs: d.Docs, Desc: d.Desc, Package: d.Package,
			Loaded: loaded[d.Loaded],
		}
		switch {
		case so == "":
			p.Status = "missing"
			p.Detail = "Install " + d.Package + " to use this."
		case p.Loaded:
			p.Status = "running"
		default:
			p.Status = "ready"
		}
		if so != "" {
			p.Source = "package"
			p.Version = pkgVersion(d.Package)
		}
		r.Plugins = append(r.Plugins, p)
	}
	return r
}

func runPlugins(args []string) ([]byte, error) {
	switch args[0] {
	case "list":
		return compactJSON(buildPluginRoster(loadStore(DesktopStorePath())))
	case "sync":
		syncPlugins(loadStore(DesktopStorePath()))
		return compactJSON(map[string]bool{"synced": true})
	case "rebuild", "add", "remove":
		return nil, fmt.Errorf("plugins %s: not available on Zenith: plugins are the packaged hyprland-plugin-* ones, pinned to the installed Hyprland; install or remove them with apt", args[0])
	default:
		return nil, fmt.Errorf("unknown plugins subcommand: %s", args[0])
	}
}

// syncPlugins reconciles the live plugin set to the store after an apply and
// reload: it loads an enabled, installed plugin the reload did not, unloads one
// the store now switches off, and pushes every plugin's config. It reconciles
// toward the store, so it needs no before/after diff.
func syncPlugins(o Overrides) {
	if !live() {
		return
	}
	loaded := loadedPlugins()
	for _, d := range bundledPlugins {
		so := pluginSoPath(d.ID)
		if so == "" {
			continue
		}
		on := loaded[d.Loaded]
		switch want := pluginEnabled(o, d.ID); {
		case want && !on:
			_, _ = ctl("plugin", "load", so)
		case !want && on:
			_, _ = ctl("plugin", "unload", so)
		}
	}
	pushKeywords(keywordLines(genPluginConfig(o)))
}

// pluginBlock is one enabled plugin resolved for emission.
type pluginBlock struct {
	id   string
	so   string // "" when no packaged copy is installed
	opts hlOpts // the plugin's own options, under plugin:<name>:
	// adds are repeatable plugin keywords (the hyprbars buttons). They are part of
	// the file but not of a live push, which would add them again every time.
	adds hlOpts
}

func pluginBlocks(o Overrides) []pluginBlock {
	var out []pluginBlock
	p := o.Plugins
	if hb := p.Hyprbars; hb.Enabled {
		b := pluginBlock{id: "hyprbars", so: pluginSoPath("hyprbars")}
		b.opts.set("plugin:hyprbars:bar_height", hb.Height)
		b.opts.set("plugin:hyprbars:bar_text_size", hb.TextSize)
		b.opts.set("plugin:hyprbars:bar_blur", hb.Blur)
		if hb.Buttons {
			b.adds = append(b.adds,
				hlOpt{"plugin:hyprbars:hyprbars-button", "rgb(ff5f57), 12, \u00d7, hyprctl dispatch killactive"},
				hlOpt{"plugin:hyprbars:hyprbars-button", "rgb(28c840), 12, +, hyprctl dispatch fullscreen 1"})
		}
		out = append(out, b)
	}
	if hf := p.Hyprfocus; hf.Enabled {
		b := pluginBlock{id: "hyprfocus", so: pluginSoPath("hyprfocus")}
		anim := "flash"
		if hf.Mode == "bounce" {
			anim = "shrink"
		}
		b.opts.set("plugin:hyprfocus:enabled", true)
		b.opts.set("plugin:hyprfocus:focus_animation", anim)
		b.opts.set("plugin:hyprfocus:flash:flash_opacity", hf.Opacity)
		b.opts.set("plugin:hyprfocus:shrink:shrink_percentage", hf.Bounce)
		out = append(out, b)
	}
	return out
}

// genPlugins renders the enabled plugins for settings.conf: a load line for each
// installed one with its options, and a comment for one that is switched on but
// not installed so the file explains why nothing happened. The load line is only
// written for a file that exists, because Hyprland shows an error banner for a
// plugin it cannot load.
func genPlugins(o Overrides) string {
	var b strings.Builder
	var opts hlOpts
	for _, pb := range pluginBlocks(o) {
		d, _ := bundledByID(pb.id)
		if pb.so == "" {
			fmt.Fprintf(&b, "# %s: not installed (apt install %s)\n", pb.id, d.Package)
			continue
		}
		fmt.Fprintf(&b, "plugin = %s\n", pb.so)
		opts = append(opts, pb.opts...)
		opts = append(opts, pb.adds...)
	}
	if o.Appearance.Layout == "scrolling" {
		if so := pluginSoPath("hyprscrolling"); so != "" {
			fmt.Fprintf(&b, "plugin = %s\n", so)
		} else {
			b.WriteString("# hyprscrolling: not installed (apt install hyprland-plugin-hyprscrolling)\n")
		}
	}
	if len(opts) > 0 {
		b.WriteString(opts.blocks())
	}
	return b.String()
}

// genPluginConfig is the options of the enabled, installed plugins as hyprlang
// lines, for the live push: the preview, and the pass after a load, when a
// plugin that just came up still holds its defaults.
func genPluginConfig(o Overrides) string {
	var b strings.Builder
	for _, pb := range pluginBlocks(o) {
		if pb.so == "" {
			continue
		}
		for _, opt := range pb.opts {
			fmt.Fprintf(&b, "%s = %s\n", opt.path, opt.val)
		}
	}
	return b.String()
}

// pluginUnhonored names each switched-on plugin feature Zenith cannot run.
func pluginUnhonored(o Overrides) []Unhonored {
	var out []Unhonored
	for _, d := range bundledPlugins {
		if pluginEnabled(o, d.ID) && pluginSoPath(d.ID) == "" {
			out = append(out, Unhonored{
				Key:    "wm.hyprland.plugins." + d.ID,
				Reason: "Not installed: " + d.Package + ".",
			})
		}
	}
	p := o.Plugins
	for id, on := range map[string]bool{
		"dynamicCursors": p.DynamicCursors.Enabled, "imgborders": p.Imgborders.Enabled,
		"hyprglass": p.Hyprglass.Enabled, "keysounds": p.Keysounds.Enabled,
	} {
		if on {
			out = append(out, Unhonored{Key: "wm.hyprland.plugins." + id, Reason: unpackagedPlugins[id]})
		}
	}
	for _, id := range sortedKeys(p.Extra) {
		if p.Extra[id].Enabled {
			out = append(out, Unhonored{
				Key:    "wm.hyprland.plugins.extra." + id,
				Reason: "Plugins added from git are not supported on Zenith.",
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
