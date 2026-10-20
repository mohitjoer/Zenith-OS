package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The applications on this machine, for the shell's launcher.
//
// The shell is QML and Quickshell has no catalogue of what is installed: it is
// a compositor shell, not a software centre, and it will not grow one. So the
// shell asks the tool that already knows how to read a machine's files, and
// the answer is the same list `zenith apps` prints for a person.
//
// The .desktop files are read here rather than by a library, because the
// format is a handful of keys in one section and a dependency that parses it
// would be a dependency this project has committed to not carrying — the same
// reasoning as the gsettings calls elsewhere.
//
// What is listed is what a launcher offers: an application, visible, with a
// name. A file with NoDisplay or Hidden is not offered, and a file that is
// not an application at all is not either.

// application is one launchable application.
type application struct {
	// ID is the desktop file id, without the .desktop suffix. It is what
	// the dock's favourites hold, and what the hub writes.
	ID string `json:"id"`
	// Name is what the user sees.
	Name string `json:"name"`
	// Icon is an icon name or a path, in the form the desktop entry uses.
	Icon string `json:"icon"`
	// Exec is the command to run, without the field codes.
	Exec string `json:"exec"`
	// Args are the command's arguments, without the field codes.
	Args []string `json:"args"`
	// Terminal says the application wants a terminal, which is the case a
	// launcher has to get right: a terminal application run without one is an
	// application that opens a window and then immediately exits.
	Terminal bool `json:"terminal"`
}

// applicationDirs are where desktop entries live, in the order the
// specification searches them. A user directory shadows a system one, so it
// comes last: a file is taken from the first directory that has it.
var applicationDirs = []string{
	"/usr/local/share/applications",
	"/usr/share/applications",
}

// userApplicationDir is the user's own.
func userApplicationDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "applications")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "applications")
}

// listApplications reads the desktop entries on this machine.
func listApplications() []application {
	dirs := append([]string{}, applicationDirs...)
	if user := userApplicationDir(); user != "" {
		dirs = append(dirs, user)
	}

	seen := map[string]bool{}
	var found []application

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".desktop") {
				continue
			}
			id := strings.TrimSuffix(name, ".desktop")
			if seen[id] {
				continue
			}
			seen[id] = true

			body, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			app, ok := parseDesktopEntry(id, string(body))
			if !ok {
				continue
			}
			found = append(found, app)
		}
	}

	sort.Slice(found, func(i, j int) bool {
		return strings.ToLower(found[i].Name) < strings.ToLower(found[j].Name)
	})
	return found
}

// parseDesktopEntry reads one desktop entry.
//
// The Desktop Entry section and the keys a launcher needs, and nothing else:
// the format has a great deal in it that is about actions, MIME types and
// localised strings, and a launcher wants the name, the icon and the command.
func parseDesktopEntry(id, body string) (application, bool) {
	keys := map[string]string{}
	inEntry := false

	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inEntry = strings.TrimSpace(line) == "[Desktop Entry]"
			continue
		}
		if !inEntry {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		// A localised name is a name for a language, not a name for the
		// machine, and the unlocalised one is the one that is always there.
		if strings.Contains(key, "[") {
			continue
		}
		keys[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	if keys["Type"] != "Application" {
		return application{}, false
	}
	if keys["NoDisplay"] == "true" || keys["Hidden"] == "true" {
		return application{}, false
	}
	name := keys["Name"]
	if name == "" {
		return application{}, false
	}
	exec := keys["Exec"]
	if exec == "" {
		return application{}, false
	}

	command, args := splitCommand(exec)

	return application{
		ID:       id,
		Name:     name,
		Icon:     iconReference(keys["Icon"]),
		Exec:     command,
		Args:     args,
		Terminal: keys["Terminal"] == "true",
	}, true
}

// iconReference is the icon as the shell will ask for it.
//
// An icon in a desktop entry is either a name in the icon theme or a path to
// a file, and the shell needs the two in the forms a QML image takes: a name
// is a name, and a path is a URL. A path handed over as a path is a path
// nothing can open, and the icon is simply missing.
//
// A name is looked up on this machine and handed over as a file, for the
// reason the lookup happens here and not in the shell: the icon theme is a
// setting of the session, and a shell that asked the icon engine for a name
// would draw whatever the session's theme happens to have, which is not the
// same thing as drawing the icon the application asked for.
func iconReference(icon string) string {
	if icon == "" {
		return ""
	}
	if strings.HasPrefix(icon, "/") {
		return "file://" + icon
	}
	if found := findIconFile(icon); found != "" {
		return "file://" + found
	}
	return icon
}

// splitCommand splits an Exec line into its command and its arguments.
//
// The field codes — %f, %U, %u and the rest — are removed rather than
// substituted, because a launcher has no file or URL to substitute and a
// command with %U in it is a command that fails.
func splitCommand(exec string) (string, []string) {
	fields := strings.Fields(exec)
	var kept []string

	for _, field := range fields {
		if strings.HasPrefix(field, "%") && len(field) <= 2 {
			continue
		}
		kept = append(kept, field)
	}

	if len(kept) == 0 {
		return "", nil
	}
	return kept[0], kept[1:]
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
