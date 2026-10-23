package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// The applications on this machine, for the shell's launcher.
//
// The shell is QML and Quickshell has no catalogue of what is installed
// beyond a raw list of desktop entries: it is a compositor shell, not a
// software centre, and it will not grow one. So the shell asks the tool that
// already knows how to read a machine's files, and the answer is the same list
// `zenith apps` prints for a person.
//
// The .desktop files are read here rather than by a library, because the
// format is a handful of keys in two kinds of section and a dependency that
// parses it would be a dependency this project has committed to not carrying
// -- the same reasoning as the gsettings calls elsewhere.
//
// What is listed is what a launcher offers, by the rules of the desktop entry
// specification: an application, with a name and a command, not Hidden and not
// NoDisplay, that this desktop is allowed to show (OnlyShowIn and NotShowIn are
// read against XDG_CURRENT_DESKTOP, which the Zenith session sets to Zenith)
// and whose TryExec, if it names one, is really installed. The directories are
// searched in the specification's order, so a user's file shadows a system one,
// and Flatpak's export directories are searched even when the session's
// environment does not name them.

// application is one launchable application.
//
// The first six fields are the shape the legacy shell reads and stay exactly as
// they were; the rest is additive, for a launcher that wants more than a name,
// an icon and a command.
type application struct {
	// ID is the desktop file id, without the .desktop suffix. It is what
	// the dock's favourites hold, and what the hub writes.
	ID string `json:"id"`
	// Name is what the user sees.
	Name string `json:"name"`
	// Icon is an icon file as a URL when it could be resolved on this machine,
	// otherwise the icon name the desktop entry used.
	Icon string `json:"icon"`
	// Exec is the command to run, without the field codes.
	Exec string `json:"exec"`
	// Args are the command's arguments, without the field codes.
	Args []string `json:"args"`
	// Terminal says the application wants a terminal, which is the case a
	// launcher has to get right: a terminal application run without one is an
	// application that opens a window and then immediately exits.
	Terminal bool `json:"terminal"`

	GenericName string      `json:"genericName,omitempty"`
	Comment     string      `json:"comment,omitempty"`
	Keywords    []string    `json:"keywords,omitempty"`
	Categories  []string    `json:"categories,omitempty"`
	WorkingDir  string      `json:"path,omitempty"`
	WMClass     string      `json:"wmClass,omitempty"`
	Actions     []appAction `json:"actions,omitempty"`
	// Source says where the file came from: "user", "system" or "flatpak".
	Source string `json:"source"`
	// File is the .desktop file the entry was read from.
	File string `json:"file"`
}

// appAction is one [Desktop Action] of an application, such as "New Window".
type appAction struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Icon string   `json:"icon,omitempty"`
	Exec string   `json:"exec"`
	Args []string `json:"args"`
}

// appEnv is what the visibility and naming rules read from the session. It is
// a value so a test can say exactly which desktop and which locale it means.
type appEnv struct {
	// desktops are the names in XDG_CURRENT_DESKTOP, which is a colon list.
	desktops []string
	// locales are the suffixes a localised key may carry, most specific first.
	locales []string
	// installed says whether a TryExec resolves to an executable.
	installed func(string) bool
}

// currentAppEnv reads the session's desktop and locale.
//
// An unset XDG_CURRENT_DESKTOP is read as Zenith rather than as nothing: a
// shell started from a terminal, or a test, has no session variable, and a
// launcher that then hid every OnlyShowIn=Zenith application would be wrong in
// the one place nobody would think to look.
func currentAppEnv() appEnv {
	var desktops []string
	for _, name := range strings.Split(os.Getenv("XDG_CURRENT_DESKTOP"), ":") {
		if name = strings.TrimSpace(name); name != "" {
			desktops = append(desktops, name)
		}
	}
	if len(desktops) == 0 {
		desktops = []string{"Zenith"}
	}
	return appEnv{desktops: desktops, locales: desktopLocaleSuffixes(), installed: desktopTryExecInstalled}
}

// desktopLocaleSuffixes lists the key suffixes to try for a localised value, in the
// order the specification gives: lang_COUNTRY@MODIFIER, lang_COUNTRY,
// lang@MODIFIER, lang. LANGUAGE is a colon list and wins when the locale is
// not C, the way gettext reads it; otherwise LC_ALL, LC_MESSAGES and LANG are
// taken in that order.
func desktopLocaleSuffixes() []string {
	var names []string
	primary := ""
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(key); value != "" {
			primary = value
			break
		}
	}
	if primary != "" && primary != "C" && primary != "POSIX" {
		names = append(names, strings.Split(os.Getenv("LANGUAGE"), ":")...)
		names = append(names, primary)
	}

	var out []string
	seen := map[string]bool{}
	for _, name := range names {
		// Drop the codeset: de_DE.UTF-8 is matched as de_DE.
		if dot := strings.IndexByte(name, '.'); dot >= 0 {
			rest := ""
			if at := strings.IndexByte(name, '@'); at > dot {
				rest = name[at:]
			}
			name = name[:dot] + rest
		}
		modifier := ""
		if at := strings.IndexByte(name, '@'); at >= 0 {
			name, modifier = name[:at], name[at:]
		}
		lang, country, hasCountry := strings.Cut(name, "_")
		if lang == "" || lang == "C" || lang == "POSIX" {
			continue
		}
		var tries []string
		if hasCountry && modifier != "" {
			tries = append(tries, lang+"_"+country+modifier)
		}
		if hasCountry {
			tries = append(tries, lang+"_"+country)
		}
		if modifier != "" {
			tries = append(tries, lang+modifier)
		}
		tries = append(tries, lang)
		for _, try := range tries {
			if !seen[try] {
				seen[try] = true
				out = append(out, try)
			}
		}
	}
	return out
}

// desktopTryExecInstalled reports whether a TryExec names an executable that exists.
//
// A shell started by systemd has the manager's PATH, which does not carry
// ~/.local/bin or the Flatpak and Snap export directories, so an application
// installed there would otherwise be reported missing and quietly vanish from
// the launcher. Those few directories are searched as well.
func desktopTryExecInstalled(name string) bool {
	if _, err := exec.LookPath(name); err == nil {
		return true
	}
	if strings.Contains(name, "/") {
		return false
	}
	var extra []string
	if home, err := os.UserHomeDir(); err == nil {
		extra = append(extra, filepath.Join(home, ".local", "bin"))
	}
	extra = append(extra, "/snap/bin", "/var/lib/flatpak/exports/bin")
	if home := xdgDataHome(); home != "" {
		extra = append(extra, filepath.Join(home, "flatpak", "exports", "bin"))
	}
	for _, dir := range extra {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return true
		}
	}
	return false
}

// xdgDataHome is $XDG_DATA_HOME, or ~/.local/share when it is unset.
func xdgDataHome() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share")
}

// xdgDataDirs are the XDG data directories in precedence order: the user's own,
// then Flatpak's export directories, then XDG_DATA_DIRS.
//
// Flatpak is added by hand because its exports reach XDG_DATA_DIRS only through
// a login-shell profile script or a systemd environment generator, and a
// session started some other way never sees them: every Flatpak application
// would be missing from the launcher and its icon from the index, with nothing
// to say why. A directory that does not exist costs one failed read. The order
// is the one Flatpak's own generator produces, and a directory the environment
// does name is kept where this puts it, not twice.
func xdgDataDirs() []string {
	var dirs []string
	seen := map[string]bool{}
	add := func(dir string) {
		if dir == "" {
			return
		}
		dir = filepath.Clean(dir)
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}

	home := xdgDataHome()
	add(home)
	if home != "" {
		add(filepath.Join(home, "flatpak", "exports", "share"))
	}
	add("/var/lib/flatpak/exports/share")

	system := os.Getenv("XDG_DATA_DIRS")
	if system == "" {
		system = "/usr/local/share:/usr/share"
	}
	for _, dir := range strings.Split(system, ":") {
		add(dir)
	}
	return dirs
}

// desktopFile is one .desktop file that won the search for its id.
type desktopFile struct {
	id     string
	path   string
	source string
	body   string
}

// scanDesktopFiles reads every desktop entry on the machine, once per id.
//
// The first file found for an id is the one that counts, whether or not it is
// offered: a user's Hidden=true copy of a system entry is how a person removes
// an application from the menus, and it only works if the system file stays
// shadowed.
func scanDesktopFiles(dirs []string) []desktopFile {
	var files []desktopFile
	seen := map[string]bool{}
	home := xdgDataHome()

	for _, dir := range dirs {
		root := filepath.Join(dir, "applications")
		source := "system"
		switch {
		case home != "" && filepath.Clean(dir) == filepath.Clean(home):
			source = "user"
		case strings.Contains(filepath.ToSlash(dir), "/flatpak/exports/"):
			source = "flatpak"
		}

		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			// Exported Flatpak entries are links; a link to a directory is not
			// followed, which keeps a loop from ever being a problem.
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".desktop") {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			// The id is the path below applications/ with the separators
			// turned into dashes, so kde4/foo.desktop is kde4-foo.
			id := strings.TrimSuffix(strings.ReplaceAll(filepath.ToSlash(rel), "/", "-"), ".desktop")
			if seen[id] {
				return nil
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			seen[id] = true
			files = append(files, desktopFile{id: id, path: path, source: source, body: string(body)})
			return nil
		})
	}
	return files
}

// listApplications reads the desktop entries on this machine that a launcher
// offers, sorted by name.
func listApplications() []application {
	return classifyApplications(scanDesktopFiles(xdgDataDirs()), currentAppEnv(), &iconResolver{})
}

// classifyApplications keeps the files a launcher offers. A nil resolver leaves
// icons as the names the entries gave.
func classifyApplications(files []desktopFile, env appEnv, icons *iconResolver) []application {
	offered := []application{}
	for _, file := range files {
		app, ok := parseDesktopFile(file.id, file.body, env, icons)
		if !ok {
			continue
		}
		app.Source = file.source
		app.File = file.path
		offered = append(offered, app)
	}

	sort.Slice(offered, func(i, j int) bool {
		a, b := strings.ToLower(offered[i].Name), strings.ToLower(offered[j].Name)
		if a != b {
			return a < b
		}
		return offered[i].ID < offered[j].ID
	})
	return offered
}

// desktopGroup is one section of a desktop file: its plain keys, and for each key
// the values that carry a locale suffix.
type desktopGroup struct {
	keys      map[string]string
	localized map[string]map[string]string
}

func newDesktopGroup() *desktopGroup {
	return &desktopGroup{keys: map[string]string{}, localized: map[string]map[string]string{}}
}

// get returns the value for a key in the first of the locales that has one,
// otherwise the plain key.
func (g *desktopGroup) get(key string, locales []string) string {
	if byLocale, ok := g.localized[key]; ok {
		for _, locale := range locales {
			if value, ok := byLocale[locale]; ok {
				return value
			}
		}
	}
	return g.keys[key]
}

// desktopParseGroups reads a desktop file into its [Desktop Entry] section and its
// [Desktop Action ...] sections. Comments, blank lines and any other section
// are skipped: the format has a great deal in it that a launcher does not use.
func desktopParseGroups(body string) (entry *desktopGroup, actions map[string]*desktopGroup) {
	actions = map[string]*desktopGroup{}
	var current *desktopGroup

	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := line[1 : len(line)-1]
			switch {
			case name == "Desktop Entry":
				if entry == nil {
					entry = newDesktopGroup()
				}
				current = entry
			case strings.HasPrefix(name, "Desktop Action "):
				id := strings.TrimPrefix(name, "Desktop Action ")
				current = newDesktopGroup()
				actions[id] = current
			default:
				current = nil
			}
			continue
		}
		if current == nil {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if open := strings.IndexByte(key, '['); open > 0 && strings.HasSuffix(key, "]") {
			base, locale := key[:open], key[open+1:len(key)-1]
			if current.localized[base] == nil {
				current.localized[base] = map[string]string{}
			}
			current.localized[base][locale] = value
			continue
		}
		if strings.Contains(key, "[") {
			continue
		}
		current.keys[key] = value
	}
	return entry, actions
}

// desktopUnescape undoes the value escapes of the specification: \s is a space,
// \n \t \r are what they say and \\ is a backslash.
func desktopUnescape(value string) string {
	if !strings.Contains(value, `\`) {
		return value
	}
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 == len(value) {
			out.WriteByte(value[i])
			continue
		}
		i++
		switch value[i] {
		case 's':
			out.WriteByte(' ')
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		case 'r':
			out.WriteByte('\r')
		case '\\':
			out.WriteByte('\\')
		default:
			out.WriteByte('\\')
			out.WriteByte(value[i])
		}
	}
	return out.String()
}

// desktopList splits a semicolon list, where \; keeps a literal semicolon, and
// drops the empty item a trailing semicolon leaves.
func desktopList(value string) []string {
	var items []string
	var current strings.Builder
	for i := 0; i < len(value); i++ {
		switch {
		case value[i] == '\\' && i+1 < len(value) && value[i+1] == ';':
			current.WriteByte(';')
			i++
		case value[i] == ';':
			if item := strings.TrimSpace(current.String()); item != "" {
				items = append(items, desktopUnescape(item))
			}
			current.Reset()
		default:
			current.WriteByte(value[i])
		}
	}
	if item := strings.TrimSpace(current.String()); item != "" {
		items = append(items, desktopUnescape(item))
	}
	return items
}

// parseDesktopEntry reads one desktop entry against the current session. It is
// the narrow form of parseDesktopFile for a caller that has no use for the
// icon index.
func parseDesktopEntry(id, body string) (application, bool) {
	return parseDesktopFile(id, body, currentAppEnv(), &iconResolver{})
}

// parseDesktopFile reads one desktop entry and says whether a launcher offers
// it.
func parseDesktopFile(id, body string, env appEnv, icons *iconResolver) (application, bool) {
	entry, actionGroups := desktopParseGroups(body)
	if entry == nil || entry.keys["Type"] != "Application" {
		return application{}, false
	}
	if entry.keys["NoDisplay"] == "true" || entry.keys["Hidden"] == "true" {
		return application{}, false
	}
	if !desktopShownOn(entry.keys["OnlyShowIn"], entry.keys["NotShowIn"], env.desktops) {
		return application{}, false
	}
	if tryExec := desktopUnescape(entry.keys["TryExec"]); tryExec != "" && env.installed != nil && !env.installed(tryExec) {
		return application{}, false
	}

	name := desktopUnescape(entry.get("Name", env.locales))
	execLine := entry.keys["Exec"]
	if name == "" || execLine == "" {
		return application{}, false
	}

	icon := desktopUnescape(entry.get("Icon", env.locales))
	command := desktopExecArgv(desktopUnescape(execLine), name, icon, "")
	if len(command) == 0 {
		return application{}, false
	}

	app := application{
		ID:          id,
		Name:        name,
		Icon:        icons.reference(icon),
		Exec:        command[0],
		Args:        command[1:],
		Terminal:    entry.keys["Terminal"] == "true",
		GenericName: desktopUnescape(entry.get("GenericName", env.locales)),
		Comment:     desktopUnescape(entry.get("Comment", env.locales)),
		Keywords:    desktopList(entry.get("Keywords", env.locales)),
		Categories:  desktopList(entry.keys["Categories"]),
		WorkingDir:  desktopUnescape(entry.keys["Path"]),
		WMClass:     desktopUnescape(entry.keys["StartupWMClass"]),
	}
	if app.Args == nil {
		app.Args = []string{}
	}

	for _, actionID := range desktopList(entry.keys["Actions"]) {
		group := actionGroups[actionID]
		if group == nil {
			continue
		}
		actionName := desktopUnescape(group.get("Name", env.locales))
		actionExec := desktopExecArgv(desktopUnescape(group.keys["Exec"]), actionName, icon, "")
		if actionName == "" || len(actionExec) == 0 {
			continue
		}
		app.Actions = append(app.Actions, appAction{
			ID:   actionID,
			Name: actionName,
			Icon: icons.reference(desktopUnescape(group.keys["Icon"])),
			Exec: actionExec[0],
			Args: append([]string{}, actionExec[1:]...),
		})
	}
	return app, true
}

// desktopShownOn applies OnlyShowIn and NotShowIn. An entry with an OnlyShowIn
// list is shown only on a desktop it names; NotShowIn hides it on one it names.
// The names are case-sensitive, as the specification has them.
func desktopShownOn(onlyShowIn, notShowIn string, desktops []string) bool {
	if only := desktopList(onlyShowIn); len(only) > 0 && !desktopAnyIn(only, desktops) {
		return false
	}
	return !desktopAnyIn(desktopList(notShowIn), desktops)
}

func desktopAnyIn(list, desktops []string) bool {
	for _, item := range list {
		for _, desktop := range desktops {
			if item == desktop {
				return true
			}
		}
	}
	return false
}

// desktopExecArgv splits an Exec line into the argument vector to run, by the quoting
// rules of the specification: arguments are separated by spaces, an argument
// may be double-quoted, and inside quotes a backslash escapes ", `, $ and \.
//
// The field codes are removed or filled in. The ones that stand for a file or a
// URL (%f %F %u %U %d %D %n %N %v %m) go, because a launcher has none and a
// command with %U in it is a command that fails; %i becomes --icon NAME, %c the
// name, %k the file's path, and %% a percent sign.
func desktopExecArgv(line, name, icon, path string) []string {
	var args []string
	var current strings.Builder
	started := false
	inQuote := false

	flush := func() {
		if started {
			args = append(args, current.String())
		}
		current.Reset()
		started = false
	}

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case inQuote && c == '\\' && i+1 < len(line) && strings.IndexByte("\"`$\\", line[i+1]) >= 0:
			current.WriteByte(line[i+1])
			i++
		case c == '"':
			inQuote = !inQuote
			started = true
		case !inQuote && (c == ' ' || c == '\t'):
			flush()
		default:
			current.WriteByte(c)
			started = true
		}
	}
	flush()

	var out []string
	for _, arg := range args {
		switch arg {
		case "%i":
			if icon != "" {
				out = append(out, "--icon", icon)
			}
			continue
		case "%f", "%F", "%u", "%U", "%d", "%D", "%n", "%N", "%v", "%m":
			continue
		}
		out = append(out, desktopFieldCodes(arg, name, path))
	}
	return out
}

// desktopFieldCodes fills the codes that may sit inside a longer argument.
func desktopFieldCodes(arg, name, path string) string {
	if !strings.Contains(arg, "%") {
		return arg
	}
	var out strings.Builder
	for i := 0; i < len(arg); i++ {
		if arg[i] != '%' || i+1 == len(arg) {
			out.WriteByte(arg[i])
			continue
		}
		i++
		switch arg[i] {
		case '%':
			out.WriteByte('%')
		case 'c':
			out.WriteString(name)
		case 'k':
			out.WriteString(path)
		case 'f', 'F', 'u', 'U', 'd', 'D', 'n', 'N', 'v', 'm', 'i':
			// Stands for something a launcher does not have.
		default:
			out.WriteByte('%')
			out.WriteByte(arg[i])
		}
	}
	return out.String()
}

// splitCommand splits an Exec line into its command and its arguments.
func splitCommand(line string) (string, []string) {
	argv := desktopExecArgv(line, "", "", "")
	if len(argv) == 0 {
		return "", nil
	}
	return argv[0], argv[1:]
}

// reference turns a desktop entry's Icon= into something the shell can open.
//
// An icon in a desktop entry is either a name in the icon theme or a path to a
// file, and the shell needs the two in the forms a QML image takes: a path is
// a URL, and a name is looked up on this machine and handed over as a file, for
// the reason the lookup happens here and not in the shell. The icon theme is a
// setting of the session, and a shell that asked the icon engine for a name
// would draw whatever the session's theme happens to have, which is not the
// same thing as drawing the icon the application asked for.
//
// A name nothing has stays a name: there is nothing to point at, and a name is a
// better answer than a path to nothing.
func (r *iconResolver) reference(icon string) string {
	if icon == "" || r == nil {
		return icon
	}
	if strings.HasPrefix(icon, "/") {
		return "file://" + icon
	}
	if found := r.lookup(icon); found != "" {
		return "file://" + found
	}
	// Some entries write Icon=foo.png, which the specification does not allow
	// but the themes answer to by the stem.
	switch strings.ToLower(filepath.Ext(icon)) {
	case ".png", ".svg", ".xpm":
		if found := r.lookup(strings.TrimSuffix(icon, filepath.Ext(icon))); found != "" {
			return "file://" + found
		}
	}
	return icon
}

// configApps prints the applications on this machine, as JSON.
func configApps(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "zenith apps: takes no arguments")
		return exitUsage
	}

	encoded, err := json.MarshalIndent(listApplications(), "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith apps:", err)
		return exitFail
	}
	fmt.Println(string(encoded))
	return exitOK
}
