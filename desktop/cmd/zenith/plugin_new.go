package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// plugin_new.go is `zenith plugin new`: it scaffolds a plugin folder from the
// embedded template, generates a manifest for the chosen host, git-inits the
// folder and commits it as the author, then prints the next steps. Ported from
// Ryoku's plugin scaffold (GPL-3.0). The template is a complete, working demo (a
// ticking counter, a mark, a panel with a button) that passes `zenith plugin
// audit` with no findings, and a test keeps it that way.

//go:embed plugin_template
var pluginTemplate embed.FS

const pluginTemplateRoot = "plugin_template"

const pluginNewUsage = `Usage: zenith plugin new <id> [flags]

Scaffolds a working demo plugin into
$(xdg-user-dir DOCUMENTS)/zenith-plugins/<id> (or ~/zenith-plugins/<id>),
git-inits it and commits it as the author. The id is lowercase letters, digits
and dashes.

  --bar | --desktop | --popout   the host to build for (default --bar, which also
                                 adds a panel)
  --name NAME                    display name (default: the id, title-cased)
  --author "Name <mail>"         author (default: your git identity)
  --to DIR                       scaffold into DIR instead
  --dry-run                      list what would be written; write nothing
`

func pluginCmdNew(args []string, pio pluginIO) int {
	set := pluginFlags("plugin new", pluginNewUsage, pio)
	bar := set.Bool("bar", false, "build a bar glyph")
	desktop := set.Bool("desktop", false, "build a desktop widget")
	popout := set.Bool("popout", false, "build a frame popout")
	name := set.String("name", "", "display name")
	author := set.String("author", "", "author, as `Name <mail>`")
	to := set.String("to", "", "scaffold into this directory")
	dryRun := set.Bool("dry-run", false, "write nothing")
	positional, code, ok := pluginParse(set, args)
	if !ok {
		return code
	}
	if len(positional) != 1 {
		fmt.Fprint(pio.err, pluginNewUsage)
		return exitUsage
	}
	id := positional[0]

	host, err := pluginHostFlag(*bar, *desktop, *popout)
	if err != nil {
		fmt.Fprintf(pio.err, "zenith plugin new: %v\n", err)
		return exitUsage
	}
	if host == "" {
		host = "topbarGlyph"
	}
	if !pluginIDRe.MatchString(id) {
		fmt.Fprintf(pio.err, "zenith plugin new: id %q must be lowercase letters, digits and dashes, not starting with a dash\n", id)
		return exitUsage
	}
	if pluginReservedIDs()[id] {
		fmt.Fprintf(pio.err, "zenith plugin new: %q is a reserved built-in widget id; choose another\n", id)
		return exitUsage
	}
	if *name == "" {
		*name = pluginTitleFromID(id)
	}
	if *author == "" {
		*author = pluginDefaultAuthor()
	}
	if strings.ContainsAny(*name+*author, "\r\n") {
		fmt.Fprintln(pio.err, "zenith plugin new: the name and author are one line each")
		return exitUsage
	}

	dir := *to
	if dir == "" {
		dir = filepath.Join(pluginAuthorRoot(), id)
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		fmt.Fprintf(pio.err, "zenith plugin new: %s already exists and is not empty\n", dir)
		return exitFail
	}

	isBar := host == "topbarGlyph"
	files, err := pluginScaffoldFiles(id, *name, *author, host, isBar)
	if err != nil {
		return pluginFail(pio, "new", err)
	}

	if *dryRun {
		fmt.Fprintf(pio.out, "would scaffold %s at %s (%s)\n", id, dir, host)
		for _, rel := range pluginSortedFiles(files) {
			fmt.Fprintf(pio.out, "  %-26s %5d bytes\n", rel, len(files[rel]))
		}
		fmt.Fprintf(pio.out, "  %-26s\n", "assets/")
		return exitOK
	}

	if err := pluginWriteScaffold(dir, files); err != nil {
		return pluginFail(pio, "new", err)
	}
	pluginGitInit(dir, *author)
	pluginPrintNextSteps(pio, id, dir, isBar)
	return exitOK
}

func pluginSortedFiles(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for rel := range files {
		names = append(names, rel)
	}
	sort.Strings(names)
	return names
}

// pluginScaffoldFiles renders the template for one plugin into memory: every
// template file with its placeholders filled, plus the generated manifest. The
// panel file is left out unless the host is the bar. Building the tree in memory
// first is what lets --dry-run report exactly what a real run writes.
func pluginScaffoldFiles(id, name, author, host string, bar bool) (map[string][]byte, error) {
	authorName, _ := pluginSplitAuthor(author)
	// A name goes into QML string literals, so a quote or a backslash in it must
	// not end the literal early. The documents take it as written.
	text := strings.NewReplacer(
		"{{ID}}", id,
		"{{NAME}}", name,
		"{{AUTHOR}}", author,
		"{{AUTHORNAME}}", authorName,
		"{{YEAR}}", fmt.Sprint(time.Now().Year()),
		"{{HOST}}", host,
	)
	qml := strings.NewReplacer(
		"{{ID}}", id,
		"{{NAME}}", pluginQMLEscape(name),
		"{{AUTHOR}}", pluginQMLEscape(author),
		"{{AUTHORNAME}}", pluginQMLEscape(authorName),
		"{{YEAR}}", fmt.Sprint(time.Now().Year()),
		"{{HOST}}", host,
	)

	files := map[string][]byte{}
	err := fs.WalkDir(pluginTemplate, pluginTemplateRoot, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, pluginTemplateRoot+"/")
		if !bar && rel == "content/Panel.qml" {
			return nil
		}
		body, err := pluginTemplate.ReadFile(p)
		if err != nil {
			return err
		}
		replacer := text
		if strings.HasSuffix(rel, ".qml") {
			replacer = qml
		}
		if rel == "gitignore" {
			rel = ".gitignore"
		}
		files[rel] = []byte(replacer.Replace(string(body)))
		return nil
	})
	if err != nil {
		return nil, err
	}

	manifest, err := pluginManifestJSON(id, name, author, host, bar)
	if err != nil {
		return nil, err
	}
	files["manifest.json"] = manifest
	return files, nil
}

// pluginQMLEscape makes s safe between double quotes in a QML string literal.
func pluginQMLEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

// pluginWriteScaffold lays the rendered tree into dir, plus the empty assets/
// directory the preview image goes in.
func pluginWriteScaffold(dir string, files map[string][]byte) error {
	for _, rel := range pluginSortedFiles(files) {
		dest := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, files[rel], 0o644); err != nil {
			return err
		}
	}
	return os.MkdirAll(filepath.Join(dir, "assets"), 0o755)
}

// pluginScaffold writes a whole scaffold to dir. It is the one call the tests
// make to get a plugin on disk.
func pluginScaffold(dir, id, name, author, host string, bar bool) error {
	files, err := pluginScaffoldFiles(id, name, author, host, bar)
	if err != nil {
		return err
	}
	return pluginWriteScaffold(dir, files)
}

// pluginManifestJSON generates manifest.json for the chosen host: entryPoints
// (with panel and a panel width only for the bar), the density set, an example
// setting, and honest metadata (official false, the author as Name <mail>).
func pluginManifestJSON(id, name, author, host string, bar bool) ([]byte, error) {
	type entryPoints struct {
		Main    string `json:"main"`
		Content string `json:"content"`
		Panel   string `json:"panel,omitempty"`
	}
	type panelSpec struct {
		Width int `json:"width"`
	}
	type capSpec struct {
		Densities []string `json:"densities"`
	}
	type depSpec struct {
		Commands []string `json:"commands"`
	}
	type settingSpec struct {
		Key     string `json:"key"`
		Type    string `json:"type"`
		Label   string `json:"label"`
		Group   string `json:"group"`
		Default any    `json:"default"`
	}
	type metaSpec struct {
		Settings []settingSpec `json:"settings"`
	}
	type manifestOut struct {
		ID           string         `json:"id"`
		Name         string         `json:"name"`
		Version      string         `json:"version"`
		Author       string         `json:"author"`
		Description  string         `json:"description"`
		License      string         `json:"license"`
		Official     bool           `json:"official"`
		Hosts        []string       `json:"hosts"`
		EntryPoints  entryPoints    `json:"entryPoints"`
		Panel        *panelSpec     `json:"panel,omitempty"`
		Files        []string       `json:"files"`
		Capabilities capSpec        `json:"capabilities"`
		Defaults     map[string]any `json:"defaults"`
		Dependencies depSpec        `json:"dependencies"`
		Metadata     metaSpec       `json:"metadata"`
	}

	manifest := manifestOut{
		ID:           id,
		Name:         name,
		Version:      "0.1.0",
		Author:       author,
		Description:  name + " is a Zenith shell plugin.",
		License:      "MIT",
		Official:     false,
		Hosts:        []string{host},
		EntryPoints:  entryPoints{Main: "service/Main.qml", Content: "content/Widget.qml"},
		Files:        []string{},
		Capabilities: capSpec{Densities: pluginDensitiesFor(host)},
		Defaults:     map[string]any{"host": host, "label": name},
		Dependencies: depSpec{Commands: []string{}},
		Metadata: metaSpec{Settings: []settingSpec{
			{Key: "showCount", Type: "toggle", Label: "Show the tick count", Group: name, Default: true},
		}},
	}
	if bar {
		manifest.EntryPoints.Panel = "content/Panel.qml"
		manifest.Panel = &panelSpec{Width: 320}
	}
	body, err := pluginMarshalIndent(manifest)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func pluginDensitiesFor(host string) []string {
	switch host {
	case "topbarGlyph":
		return []string{"glyph", "compact"}
	case "desktopWidget":
		return []string{"compact", "full"}
	default:
		return []string{"compact"}
	}
}

// pluginMarshalIndent is two-space JSON without HTML escaping, so an author's
// `Name <mail>` survives as written (encoding/json would print \u003c).
func pluginMarshalIndent(v any) ([]byte, error) {
	var buffer strings.Builder
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return []byte(buffer.String()), nil
}

// pluginAuthorRoot is R1's authoring root: the XDG DOCUMENTS dir's
// zenith-plugins folder, falling back to ~/zenith-plugins.
func pluginAuthorRoot() string {
	if pluginHas("xdg-user-dir") {
		if out, err := pluginOutput("xdg-user-dir", "DOCUMENTS"); err == nil {
			if dir := strings.TrimSpace(out); dir != "" && dir != pluginHome() {
				return filepath.Join(dir, "zenith-plugins")
			}
		}
	}
	return filepath.Join(pluginHome(), "zenith-plugins")
}

// pluginTitleFromID turns a plugin id into a display name: demo-x -> "Demo X".
func pluginTitleFromID(id string) string {
	parts := strings.Split(id, "-")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

// pluginDefaultAuthor resolves the author when --author is absent: git's
// configured identity, then $USER, so a fresh plugin still carries an honest
// author (R11).
func pluginDefaultAuthor() string {
	name := ""
	mail := ""
	if pluginHas("git") {
		name, _ = pluginGit(".", "config", "user.name")
		mail, _ = pluginGit(".", "config", "user.email")
	}
	name, mail = strings.TrimSpace(name), strings.TrimSpace(mail)
	switch {
	case name != "" && mail != "":
		return fmt.Sprintf("%s <%s>", name, mail)
	case name != "":
		return name
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "zenith"
}

// pluginGitInit makes the folder a git repo with one commit authored by the
// plugin author. Best-effort: no git, no repo, but the folder is still valid.
func pluginGitInit(dir, author string) {
	if !pluginHas("git") {
		return
	}
	if _, err := pluginGit(dir, "init", "-q"); err != nil {
		return
	}
	_, _ = pluginGit(dir, "add", "-A")
	_ = pluginCommitAs(dir, author, "Initial plugin scaffold")
}

func pluginPrintNextSteps(pio pluginIO, id, dir string, bar bool) {
	fmt.Fprintf(pio.out, "%s %s at %s\n\n", pio.green("scaffolded"), id, dir)
	fmt.Fprintln(pio.out, "Next steps:")
	edit := "  1. edit:    service/Main.qml, content/Widget.qml"
	if bar {
		edit += ", content/Panel.qml"
	}
	fmt.Fprintln(pio.out, edit)
	fmt.Fprintln(pio.out, "  2. capture assets/preview-widget.png (see README.md), then list it in manifest files")
	fmt.Fprintf(pio.out, "  3. check:   zenith plugin audit %s\n", dir)
	if bar {
		fmt.Fprintf(pio.out, "  4. install: zenith plugin install %s --bar --yes\n", dir)
		fmt.Fprintln(pio.out, "     then find it on the bar, and on the bar's Community page")
	} else {
		fmt.Fprintf(pio.out, "  4. install: zenith plugin install %s --yes\n", dir)
		fmt.Fprintf(pio.out, "     then turn it on: zenith plugin enable %s --desktop|--popout\n", id)
	}
}
