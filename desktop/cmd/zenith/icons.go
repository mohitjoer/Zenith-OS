package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The icon an application points at, as something the shell can open.
//
// A desktop entry names its icon in one of two ways: a path to a file, or a
// name in the icon theme. A path is turned into a URL and handed over as it
// is. A name has to be looked up, because the shell does not read the icon
// theme: it asks the icon engine for the name, the icon engine asks the
// desktop's theme, and the desktop's theme is a setting of the session rather
// than a fact about the machine. An icon that is missing from the theme the
// session happens to be using is an icon that is missing, and the shell draws
// a broken-image placeholder where the application's icon should be.
//
// So the name is resolved here, against the machine's own icon directories,
// and the shell is handed a file. The theme is searched in the order the icon
// specification searches it: the session's theme first, then the fallback,
// then everything else that is installed.

// iconDirs are the directories the icon specification says to look in.
func iconDirs() []string {
	var dirs []string

	if home := os.Getenv("XDG_DATA_HOME"); home != "" {
		dirs = append(dirs, filepath.Join(home, "icons"))
	} else if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "icons"))
	}

	paths := os.Getenv("XDG_DATA_DIRS")
	if paths == "" {
		paths = "/usr/local/share:/usr/share"
	}
	for _, path := range strings.Split(paths, ":") {
		if path != "" {
			dirs = append(dirs, filepath.Join(path, "icons"))
		}
	}
	return dirs
}

// findIconFile looks an icon name up in the machine's icon directories.
//
// The name is matched on the file's own name, with or without an extension,
// because an icon is a file with a size and a context in its path and neither
// is part of the name the desktop entry used.
func findIconFile(name string) string {
	if name == "" || strings.Contains(name, "/") {
		return ""
	}

	// The name is used as it is given. A desktop entry's icon name is not a
	// file name, and stripping what looks like an extension from it is how
	// "org.gnome.Nautilus" turns into "org.gnome" — the last dot in a name is
	// usually part of the name, and the file behind it is found by the name
	// with the extension added, not the extension taken away.
	base := name

	for _, dir := range iconDirs() {
		if found := walkIcons(dir, base); found != "" {
			return found
		}
	}
	return ""
}

// walkIcons finds an icon in one icon directory.
//
// The themes are searched in the order the specification gives them: the
// session's own theme, then the fallback, then the rest. A theme that is
// installed but not current is still a theme a user can switch to, so an
// icon that only exists there is a better answer than no icon at all.
func walkIcons(root, base string) string {
	themes := []string{currentIconTheme(), "hicolor"}
	seen := map[string]bool{}

	for _, theme := range themes {
		if theme == "" || seen[theme] {
			continue
		}
		seen[theme] = true
		if found := findInTree(filepath.Join(root, theme), base); found != "" {
			return found
		}
	}

	// Everything else that is installed, in a stable order, so that the same
	// machine gives the same answer twice.
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && !seen[entry.Name()] {
			names = append(names, entry.Name())
		}
	}
	for _, name := range names {
		if found := findInTree(filepath.Join(root, name), base); found != "" {
			return found
		}
	}
	return ""
}

// findInTree looks for an icon file by name under a directory.
func findInTree(root, base string) string {
	found := ""
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if name != base && strings.TrimSuffix(name, filepath.Ext(name)) != base {
			return nil
		}
		// A cursor is not an image. It is in the icon directory because
		// the pointer themes live there too, and handing one to an image
		// is a picture that never appears.
		if filepath.Ext(name) == ".cursor" {
			return nil
		}
		found = path
		return nil
	})
	return found
}

// currentIconTheme is the theme the session is using.
//
// It is read from the desktop's own setting, because that is what the icon
// engine will use, and an icon found under a different theme is an icon the
// user did not choose.
func currentIconTheme() string {
	return strings.Trim(strings.TrimSpace(gsettingsOutput("get", "org.gnome.desktop.interface", "icon-theme")), "'")
}

// gsettingsOutput runs gsettings and returns what it printed.
//
// A setting that cannot be read is a setting that is not there, and a theme
// that is not there is a theme the search falls back from: the caller asks
// for the session's theme and is told nothing, and the search continues with
// the fallback.
func gsettingsOutput(args ...string) string {
	out, err := exec.Command("gsettings", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
