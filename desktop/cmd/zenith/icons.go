package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Icon names resolved to files, without going through Qt's icon engine.
//
// The shell used to load every app and theme icon as an image://icon/<name> URL.
// That routes through libqsvgicon under QIcon and re-rasterises on a
// fractional-scale change, a path that raced the dock's icon warm-up and
// segfaulted a surface when a monitor's scale arrived after the surface mapped.
// A plain file:// path to the theme's own SVG or PNG keeps the ordinary image
// plugin in the loop, and that is size-stable across scale changes.
//
// So the name is resolved here, against the machine's own icon directories, and
// the shell is handed a file. The theme is searched in the order the icon theme
// specification gives: the session's theme, the themes it inherits, Adwaita,
// and hicolor last as the fallback every application installs into.
//
// "zenith icons" prints the whole index as one JSON object and needs no daemon.
// The shell asks the daemon's "zenith-shell icons" first and falls back to this
// one, so icons resolve whether or not the daemon is up. The index and the
// lookup order are ported from Ryoku's ryoku-shell icons (GPL-3.0).

func init() {
	register("icons", "the icon index the shell draws from, as JSON", zenithIcons)
}

// iconIndex is the whole answer: the active theme, its resolved inheritance
// chain (ending in hicolor), and every icon name mapped to an absolute path.
type iconIndex struct {
	Theme string            `json:"theme"`
	Chain []string          `json:"chain"`
	Icons map[string]string `json:"icons"`
}

// iconCand is one candidate file for a name within a single theme, scored so
// the preferred one wins: a scalable SVG beats a sized raster, which beats a
// symbolic variant; among rasters see iconSizeFit; then SVG over the rest.
type iconCand struct {
	path string
	tier int // 3 scalable svg, 2 sized, 1 symbolic
	size int
	svg  bool
}

// iconTargetSize is the raster size the index aims for. The shell draws icons
// at most a few hundred device pixels across, so anything from here up is sharp
// enough, and a 1024px PNG decoded to paint a 40px icon is memory and a stall
// on the frame that first shows it, for nothing.
const iconTargetSize = 256

// iconSizeFit ranks raster sizes, higher is better: any size at or above the
// target beats any size below it, the smallest of those wins, and below the
// target the largest wins.
func iconSizeFit(size int) int {
	if size >= iconTargetSize {
		return 1<<30 - size
	}
	return size
}

// iconBetter reports whether a is the preferred candidate over b.
func iconBetter(a, b iconCand) bool {
	if a.tier != b.tier {
		return a.tier > b.tier
	}
	if fa, fb := iconSizeFit(a.size), iconSizeFit(b.size); fa != fb {
		return fa > fb
	}
	if a.svg != b.svg {
		return a.svg
	}
	return false
}

const iconsUsage = `zenith icons — the icon index the shell draws from

Usage:
  zenith icons          print every icon name and its file, as JSON
  zenith icons NAME...  print the file each name resolves to, one per line

The index follows the session's icon theme, the themes it inherits, Adwaita and
hicolor. An unknown name prints an empty line.
`

// zenithIcons is the CLI entry. With no names it prints the whole index as
// JSON; with names it prints one resolved path per line.
func zenithIcons(args []string) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Print(iconsUsage)
		return exitOK
	}

	index := buildIconIndex()
	if len(args) == 0 {
		encoded, err := json.Marshal(index)
		if err != nil {
			fmt.Fprintln(os.Stderr, "zenith icons:", err)
			return exitFail
		}
		fmt.Println(string(encoded))
		return exitOK
	}
	for _, name := range args {
		fmt.Println(resolveIconName(name, index))
	}
	return exitOK
}

// resolveIconName answers one spot-check lookup: an absolute path passes
// through when it exists, otherwise the name is looked up in the index.
func resolveIconName(name string, index iconIndex) string {
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "/") {
		if info, err := os.Stat(name); err == nil && !info.IsDir() {
			return name
		}
		return ""
	}
	return index.Icons[name]
}

// buildIconIndex builds the index for the live system: the active theme and
// the real icon roots, with the pixmaps directories as the last fallback.
func buildIconIndex() iconIndex {
	var pixmaps []string
	for _, dir := range xdgDataDirs() {
		pixmaps = append(pixmaps, filepath.Join(dir, "pixmaps"))
	}
	return buildIconIndexFrom(currentIconTheme(), iconRoots(), pixmaps)
}

// buildIconIndexFrom is the pure core, with the theme, the roots and the pixmaps
// directories injected, so the lookup order is testable without touching
// gsettings or /usr.
func buildIconIndexFrom(theme string, roots, pixmaps []string) iconIndex {
	chain := iconThemeChain(theme, roots)
	icons := make(map[string]string, 4096)
	for _, name := range chain {
		// The best candidate per name within this theme, across every root,
		// then merged: the first theme in the chain that has a name keeps it.
		best := map[string]iconCand{}
		for _, root := range roots {
			scanIconThemeDir(filepath.Join(root, name), best)
		}
		for icon, cand := range best {
			if _, taken := icons[icon]; !taken {
				icons[icon] = cand.path
			}
		}
	}
	for _, dir := range pixmaps {
		scanIconPixmaps(dir, icons)
	}
	return iconIndex{Theme: theme, Chain: chain, Icons: icons}
}

// currentIconTheme reads the theme the session is using. GNOME's interface
// setting comes first because GTK itself prefers it when the schema exists,
// then GTK 4's and GTK 3's settings.ini, which is where a session without the
// GNOME schemas keeps it. An empty answer is a real answer: the chain still
// ends in Adwaita and hicolor.
func currentIconTheme() string {
	if out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "icon-theme").Output(); err == nil {
		if name := strings.Trim(strings.TrimSpace(string(out)), "'\""); name != "" {
			return name
		}
	}
	for _, version := range []string{"gtk-4.0", "gtk-3.0"} {
		if name := gtkSettingsIconTheme(version); name != "" {
			return name
		}
	}
	return ""
}

// gtkSettingsIconTheme reads gtk-icon-theme-name from the user's settings.ini for
// one GTK major version.
func gtkSettingsIconTheme(version string) string {
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		config = filepath.Join(home, ".config")
	}
	body, err := os.ReadFile(filepath.Join(config, version, "settings.ini"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(body), "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if found && strings.TrimSpace(key) == "gtk-icon-theme-name" {
			return strings.Trim(strings.TrimSpace(value), "'\"")
		}
	}
	return ""
}

// iconRoots are the icon search roots in preference order: each data
// directory's icons (the user's first, the Flatpak exports among them), then
// the old ~/.icons.
func iconRoots() []string {
	var roots []string
	for i, dir := range xdgDataDirs() {
		roots = append(roots, filepath.Join(dir, "icons"))
		if i == 0 {
			if home, err := os.UserHomeDir(); err == nil {
				roots = append(roots, filepath.Join(home, ".icons"))
			}
		}
	}
	return roots
}

// iconThemeChain resolves the theme's inheritance chain: the theme, then its
// Inherits parents depth-first and deduplicated, then Adwaita when it is
// installed and not already named, with hicolor always last.
//
// hicolor is held back from the walk and appended once, because a theme that
// names it early would otherwise let it outrank a theme listed after it.
func iconThemeChain(theme string, roots []string) []string {
	var chain []string
	seen := map[string]bool{"hicolor": true}
	var visit func(name string)
	visit = func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		chain = append(chain, name)
		for _, parent := range iconThemeInherits(name, roots) {
			visit(parent)
		}
	}
	visit(theme)
	if !seen["Adwaita"] && iconThemeInstalled("Adwaita", roots) {
		visit("Adwaita")
	}
	return append(chain, "hicolor")
}

// iconThemeInstalled reports whether any root carries the theme.
func iconThemeInstalled(theme string, roots []string) bool {
	for _, root := range roots {
		if _, err := os.Stat(filepath.Join(root, theme, "index.theme")); err == nil {
			return true
		}
	}
	return false
}

// iconThemeInherits reads the Inherits= list from a theme's index.theme, taking
// the first root that has one.
func iconThemeInherits(theme string, roots []string) []string {
	for _, root := range roots {
		body, err := os.ReadFile(filepath.Join(root, theme, "index.theme"))
		if err != nil {
			continue
		}
		return parseIconInherits(body)
	}
	return nil
}

// parseIconInherits pulls the comma-separated Inherits= value out of the
// [Icon Theme] section of an index.theme. The per-directory sections can carry
// keys of their own, which is why the section is checked.
func parseIconInherits(body []byte) []string {
	section := ""
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		if section != "Icon Theme" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found || strings.TrimSpace(key) != "Inherits" {
			continue
		}
		var parents []string
		for _, parent := range strings.Split(value, ",") {
			if parent = strings.TrimSpace(parent); parent != "" {
				parents = append(parents, parent)
			}
		}
		return parents
	}
	return nil
}

// scanIconThemeDir walks one theme directory (one root) and merges the best
// candidate per icon name into best. It follows directory symlinks, because
// Papirus and its dark and light variants build whole size trees out of them
// (Papirus-Dark/128x128 is a link to ../Papirus/128x128) and filepath.WalkDir
// does not. Directories are deduplicated by resolved path so an aliased tree is
// walked once and a symlink cycle cannot loop.
func scanIconThemeDir(dir string, best map[string]iconCand) {
	seen := map[string]bool{}
	var walk func(current string)
	walk = func(current string) {
		key := current
		if real, err := filepath.EvalSymlinks(current); err == nil {
			key = real
		}
		if seen[key] {
			return
		}
		seen[key] = true
		entries, err := os.ReadDir(current)
		if err != nil {
			return
		}
		for _, entry := range entries {
			name := entry.Name()
			path := filepath.Join(current, name)
			// An icon-extension entry is a file, real or a link to one, and is
			// classified straight away: the path resolves through the link when
			// it is opened, so no stat is needed here, which is what keeps a
			// link-heavy theme well inside its time budget.
			if cand, icon, ok := classifyIconFile(dir, path, name); ok {
				if prior, exists := best[icon]; !exists || iconBetter(cand, prior) {
					best[icon] = cand
				}
				continue
			}
			// Otherwise it may be a directory: recurse into real ones, and stat
			// only the extension-less links, the size-tree aliases.
			isDir := entry.IsDir()
			if entry.Type()&fs.ModeSymlink != 0 {
				info, err := os.Stat(path)
				if err != nil {
					continue
				}
				isDir = info.IsDir()
			}
			if isDir {
				walk(path)
			}
		}
	}
	walk(dir)
}

// classifyIconFile scores one file under a theme directory. It returns the
// candidate and the icon name (the file stem), or ok=false for a file that is
// not an icon.
func classifyIconFile(themeDir, path, base string) (iconCand, string, bool) {
	ext := strings.ToLower(filepath.Ext(base))
	if ext != ".svg" && ext != ".png" && ext != ".xpm" {
		return iconCand{}, "", false
	}
	name := strings.TrimSuffix(base, filepath.Ext(base))
	rel, err := filepath.Rel(themeDir, path)
	if err != nil {
		return iconCand{}, "", false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	svg := ext == ".svg"
	scalable := false
	symbolic := strings.HasSuffix(name, "-symbolic")
	size := 0
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case "scalable":
			scalable = true
		case "symbolic":
			symbolic = true
		}
		if n := parseIconSizeDir(part); n > size {
			size = n
		}
	}
	tier := 2
	switch {
	case symbolic:
		tier = 1
	case scalable && svg:
		tier = 3
	}
	// A scalable directory with no numeric size still beats any fixed raster.
	if scalable && size == 0 {
		size = 1 << 20
	}
	return iconCand{path: path, tier: tier, size: size, svg: svg}, name, true
}

// parseIconSizeDir reads the pixel size out of a size directory name: the leading
// integer of forms like "48", "48x48" and "256x256", doubled for an "@2x"
// suffix. It returns 0 for a directory that is not a size (scalable, symbolic,
// apps).
func parseIconSizeDir(name string) int {
	multiple := 1
	if at := strings.Index(name, "@"); at >= 0 {
		suffix := name[at+1:]
		name = name[:at]
		if factor := strings.TrimSuffix(suffix, "x"); factor != suffix {
			if n, err := strconv.Atoi(factor); err == nil && n > 0 {
				multiple = n
			}
		}
	}
	end := 0
	for end < len(name) && name[end] >= '0' && name[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	n, err := strconv.Atoi(name[:end])
	if err != nil {
		return 0
	}
	return n * multiple
}

// scanIconPixmaps fills any name still missing from a pixmaps directory's top
// level, preferring svg over png over xpm. A themed name is never overridden.
func scanIconPixmaps(dir string, icons map[string]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	rank := func(ext string) int {
		switch ext {
		case ".svg":
			return 3
		case ".png":
			return 2
		case ".xpm":
			return 1
		}
		return 0
	}
	best := map[string]iconCand{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		r := rank(strings.ToLower(filepath.Ext(entry.Name())))
		if r == 0 {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if cur, ok := best[name]; !ok || r > cur.size {
			best[name] = iconCand{path: filepath.Join(dir, entry.Name()), size: r}
		}
	}
	for name, cand := range best {
		if _, taken := icons[name]; !taken {
			icons[name] = cand.path
		}
	}
}

// iconResolver answers icon lookups for a desktop entry's Icon= value. The
// index behind it is built on first use and kept for the life of the resolver,
// so listing a hundred applications walks the icon directories once and not
// once per application.
type iconResolver struct {
	index *iconIndex
}

// lookup returns the file for an icon name, or "" when no theme has it.
func (r *iconResolver) lookup(name string) string {
	if r.index == nil {
		built := buildIconIndex()
		r.index = &built
	}
	return r.index.Icons[name]
}
