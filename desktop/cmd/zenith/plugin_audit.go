package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// plugin_audit.go is the static audit `zenith plugin audit` and `zenith plugin
// install` run over a plugin tree, on top of the manifest checks in plugin.go.
// Ported from Ryoku's plugin audit (GPL-3.0). It reads bytes only; it never
// executes anything from the plugin. Each finding carries a stable rule id, the
// file, the line and one sentence. Blocking findings fail the audit unless
// downgraded with --allow; warnings are printed and never fail. The rules mirror
// R1..R11 in docs/plugins.md and in the scaffold's AGENTS.md.
//
// What differs from Ryoku's audit is the names: the shell's internal namespace
// is qs.* here (so any import of it but the kit is internal, and a plugin that
// still imports Ryoku.PluginKit is told the fix), and names Zenith renamed are
// reported as removed-name warnings.

// pluginFinding is one audit result. Line is 1-based, or 0 for a whole-file
// finding (symlink, binary, large-tree).
type pluginFinding struct {
	Rule    string `json:"rule"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// pluginAuditResult groups findings by severity. --json marshals this shape.
type pluginAuditResult struct {
	Blocking []pluginFinding `json:"blocking"`
	Warnings []pluginFinding `json:"warnings"`
}

// pluginBlockingRules is the set that fails the audit. Everything else warns.
var pluginBlockingRules = map[string]bool{
	"symlink":         true,
	"escalation":      true,
	"pipe-shell":      true,
	"internal-import": true,
	"config-write":    true,
	"secret":          true,
	"binary":          true,
}

// pluginCommandAllowlist is the set of external programs a plugin may run
// without declaring them in dependencies.commands. Everything else it runs must
// be in its own bin/ or in dependencies.commands.
var pluginCommandAllowlist = map[string]bool{
	"zenith": true, "zenith-shell": true, "zenith-plugins-place": true,
	"zenith-store": true, "notify-send": true, "xdg-open": true,
	"wl-copy": true, "wl-paste": true, "sh": true, "bash": true, "jq": true,
	"cat": true, "grep": true, "sed": true, "awk": true, "head": true,
	"tail": true, "sleep": true, "date": true, "nmcli": true, "pactl": true,
	"playerctl": true, "brightnessctl": true, "pkexec": true,
}

var (
	pluginEscalationRe   = regexp.MustCompile(`\b(sudo|doas|su)\b`)
	pluginPipeShellRe    = regexp.MustCompile(`(?:curl|wget)[^|\n]*\|\s*(?:sh|bash|zsh)|eval\s+"\$\(curl`)
	pluginImportShellRe  = regexp.MustCompile(`^\s*import\s+shell\.`)
	pluginImportKitRe    = regexp.MustCompile(`^\s*import\s+((?:Ryoku|qs)(?:\.[A-Za-z]+)*)`)
	pluginImportRelRe    = regexp.MustCompile(`^\s*import\s+"(\.\./[^"]*)"`)
	pluginSecretRe       = regexp.MustCompile(`(?:sk|ghp|gho|xox[abp])-[A-Za-z0-9_-]{16,}|AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY`)
	pluginCmdArrayRe     = regexp.MustCompile(`command\s*:\s*\[\s*([^,\]\n]+)`)
	pluginExecDetachedRe = regexp.MustCompile(`execDetached\s*\(\s*\[\s*([^,\]\n]+)`)
	pluginHostRe         = regexp.MustCompile(`https?://([^/"'\s]+)`)
	pluginShcArrayRe     = regexp.MustCompile(`["'](?:sh|bash)["']\s*,\s*["']-c["']\s*,\s*([^\]\)]+)`)
	pluginShcInlineRe    = regexp.MustCompile(`\b(?:sh|bash)\s+-c\b(.*)`)
	pluginOutsideWriteRe = regexp.MustCompile(`~/\.config/|\$HOME/\.`)
	pluginCmdPunctRe     = regexp.MustCompile(`["'\[\](),+]`)
	// The Ryoku names Zenith changed: its commands, its directories, its
	// environment. Code that still uses one loads and then fails quietly, so it
	// warns rather than blocks.
	pluginRemovedRe = regexp.MustCompile(`\bryoku(?:-[a-z-]+)?\b|\bryostore\b|\b(?:RYOKU|RYOSTORE)_[A-Z_]+\b`)
)

// pluginAuditManifest is the manifest data the audit reasons about: what the
// plugin declared it needs. Parsed tolerantly, so the audit still runs when the
// manifest is malformed (the manifest checks report that separately).
type pluginAuditManifest struct {
	Privileged []string
	Network    []string
	Commands   []string
}

func pluginReadAuditManifest(dir string) pluginAuditManifest {
	var am pluginAuditManifest
	body, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return am
	}
	var raw struct {
		Dependencies struct {
			Commands []string `json:"commands"`
		} `json:"dependencies"`
		Capabilities struct {
			Network    []string `json:"network"`
			Privileged []string `json:"privileged"`
		} `json:"capabilities"`
	}
	if json.Unmarshal(body, &raw) != nil {
		return am
	}
	am.Commands = raw.Dependencies.Commands
	am.Network = raw.Capabilities.Network
	am.Privileged = raw.Capabilities.Privileged
	return am
}

// pluginAudit runs the static audit over the plugin tree at dir, returning the
// findings split into blocking and warnings. allow downgrades the named blocking
// rules to warnings for this run. It never executes plugin code.
func pluginAudit(dir string, allow map[string]bool) pluginAuditResult {
	am := pluginReadAuditManifest(dir)
	var findings []pluginFinding
	add := func(rule, p string, line int, msg string) {
		findings = append(findings, pluginFinding{Rule: rule, Path: p, Line: line, Message: msg})
	}

	var total int64
	_ = filepath.WalkDir(dir, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		relSlash := filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			add("symlink", relSlash, 0, "symlinks are not allowed in a plugin (R2)")
			return nil
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			total += info.Size()
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".so" || ext == ".dylib" || ext == ".exe" {
			add("binary", relSlash, 0, "a compiled binary; a plugin ships scripts only (R10)")
			return nil
		}
		content, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		if pluginHasBinaryMagic(content) {
			add("binary", relSlash, 0, "a compiled binary; a plugin ships scripts only (R10)")
			return nil
		}
		pluginAuditFile(relSlash, content, am, add)
		return nil
	})
	if total > 20*1024*1024 {
		add("large-tree", ".", 0, fmt.Sprintf("the plugin tree is %d MB; keep it under 20 MB", total/(1024*1024)))
	}

	result := pluginAuditResult{Blocking: []pluginFinding{}, Warnings: []pluginFinding{}}
	for _, finding := range findings {
		if pluginBlockingRules[finding.Rule] && !allow[finding.Rule] {
			result.Blocking = append(result.Blocking, finding)
		} else {
			result.Warnings = append(result.Warnings, finding)
		}
	}
	pluginSortFindings(result.Blocking)
	pluginSortFindings(result.Warnings)
	return result
}

func pluginSortFindings(list []pluginFinding) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Path != list[j].Path {
			return list[i].Path < list[j].Path
		}
		if list[i].Line != list[j].Line {
			return list[i].Line < list[j].Line
		}
		return list[i].Rule < list[j].Rule
	})
}

// pluginAuditFile applies the per-file rules to one file's content. The class is
// the file kind (qml/js/sh/json/md/other): code-pattern rules run only on
// qml/js/sh, over the comment-stripped text, while secret scanning runs over
// every text file's raw lines, because a secret is a secret even in a comment.
func pluginAuditFile(p string, content []byte, am pluginAuditManifest, add func(rule, p string, line int, msg string)) {
	class := pluginClassify(p, content)
	rawLines := strings.Split(string(content), "\n")

	for i, line := range rawLines {
		if pluginSecretRe.MatchString(line) {
			add("secret", p, i+1, "looks like a hardcoded secret or key (R10)")
		}
	}

	if class != "qml" && class != "js" && class != "sh" {
		return
	}
	// A file with NUL bytes is not source we can reason about line by line.
	if bytes.IndexByte(content, 0) >= 0 {
		return
	}

	codeLines := pluginCodeText(rawLines, class)
	qmlLike := class == "qml" || class == "js"
	for i, code := range codeLines {
		line := i + 1

		if match := pluginEscalationRe.FindString(code); match != "" {
			add("escalation", p, line, fmt.Sprintf("uses %q; a plugin must never escalate privilege (R6)", match))
		}
		if index := strings.Index(code, "pkexec"); index >= 0 && !pluginPkexecAllowed(code[index:], am.Privileged) {
			add("escalation", p, line, "runs pkexec without an exact match in capabilities.privileged (R6)")
		}
		if pluginPipeShellRe.MatchString(code) {
			add("pipe-shell", p, line, "downloads and pipes code into a shell (R7)")
		}

		if qmlLike {
			pluginAuditImport(p, line, code, add)
		}

		pluginAuditConfigWrite(p, line, code, add)

		if qmlLike {
			for _, name := range pluginExtractCommands(code) {
				if !pluginCommandAllowed(name, am.Commands) {
					add("undeclared-command", p, line, fmt.Sprintf("runs %q, not in bin/, dependencies.commands, or the allowlist (R6)", name))
				}
			}
		}
		for _, match := range pluginHostRe.FindAllStringSubmatch(code, -1) {
			host := pluginStripPort(match[1])
			if host != "" && !pluginHostAllowed(host, am.Network) {
				add("undeclared-host", p, line, fmt.Sprintf("contacts %q, not listed in capabilities.network (R7)", host))
			}
		}
		if pluginIsDynamicShell(code) {
			add("dynamic-shell", p, line, "sh/bash -c with a built-up argument invites injection (R9)")
		}
		if pluginOutsideWriteRe.MatchString(code) {
			add("outside-write", p, line, "writes outside the plugin state dir (R8)")
		}
		if match := pluginRemovedRe.FindString(code); match != "" {
			add("removed-name", p, line, fmt.Sprintf("%q is a Ryoku name; here it is zenith (zenith-*, ZENITH_*, ~/.config/zenith): see docs/plugins.md", match))
		}
	}
}

// pluginAuditImport checks one import line against R4: the kit is the only
// stable surface, so any other qs.* module, a Ryoku.* module or shell.* is
// internal, and so is a relative import that climbs out of the folder.
func pluginAuditImport(p string, line int, code string, add func(rule, p string, line int, msg string)) {
	if pluginImportShellRe.MatchString(code) {
		add("internal-import", p, line, "imports shell internals; only qs.PluginKit is allowed (R4)")
		return
	}
	if match := pluginImportKitRe.FindStringSubmatch(code); match != nil {
		switch module := match[1]; {
		case module == "qs.PluginKit" || module == "qs.PluginKit.Singletons":
		case module == "Ryoku.PluginKit" || module == "Ryoku.PluginKit.Singletons":
			add("internal-import", p, line, fmt.Sprintf("%s was renamed: use `import %s` (R4)", module, strings.Replace(module, "Ryoku.", "qs.", 1)))
		default:
			add("internal-import", p, line, fmt.Sprintf("imports shell internals (%s); only qs.PluginKit is allowed (R4)", module))
		}
		return
	}
	if match := pluginImportRelRe.FindStringSubmatch(code); match != nil && pluginImportEscapes(p, match[1]) {
		add("internal-import", p, line, "a relative import climbs out of the plugin folder (R4)")
	}
}

// pluginClassify maps a file to its class by extension, falling back to a
// shebang sniff so a bin/ script with no extension still reads as shell.
func pluginClassify(p string, content []byte) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".qml":
		return "qml"
	case ".js", ".mjs":
		return "js"
	case ".sh", ".bash", ".zsh":
		return "sh"
	case ".json":
		return "json"
	case ".md", ".markdown":
		return "md"
	}
	if bytes.HasPrefix(content, []byte("#!")) {
		first := content
		if i := bytes.IndexByte(content, '\n'); i >= 0 {
			first = content[:i]
		}
		if bytes.Contains(first, []byte("sh")) {
			return "sh"
		}
	}
	return "other"
}

// pluginHasBinaryMagic reports whether b starts with an ELF or Mach-O magic
// number.
func pluginHasBinaryMagic(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	if bytes.HasPrefix(b, []byte{0x7f, 'E', 'L', 'F'}) {
		return true
	}
	for _, magic := range [][]byte{
		{0xFE, 0xED, 0xFA, 0xCE}, {0xFE, 0xED, 0xFA, 0xCF},
		{0xCE, 0xFA, 0xED, 0xFE}, {0xCF, 0xFA, 0xED, 0xFE},
		{0xCA, 0xFE, 0xBA, 0xBE}, {0xBE, 0xBA, 0xFE, 0xCA},
	} {
		if bytes.HasPrefix(b, magic) {
			return true
		}
	}
	return false
}

// pluginCodeText returns each raw line with its comment content blanked
// (spaces), preserving line numbers and column positions, so the pattern rules
// match code and not comments. Handles sh (#), and C-style // and /* */ for
// qml/js.
func pluginCodeText(rawLines []string, class string) []string {
	out := make([]string, len(rawLines))
	if class == "sh" {
		for i, line := range rawLines {
			out[i] = pluginStripShellComment(line)
		}
		return out
	}
	inBlock := false
	for i, line := range rawLines {
		out[i], inBlock = pluginStripCLine(line, inBlock)
	}
	return out
}

func pluginStripShellComment(line string) string {
	var b strings.Builder
	var quote byte
	prevSpace := true
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			b.WriteByte(c)
			if c == quote {
				quote = 0
			}
			prevSpace = false
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			b.WriteByte(c)
			prevSpace = false
			continue
		}
		if c == '#' && prevSpace {
			for j := i; j < len(line); j++ {
				b.WriteByte(' ')
			}
			return b.String()
		}
		b.WriteByte(c)
		prevSpace = c == ' ' || c == '\t'
	}
	return b.String()
}

func pluginStripCLine(line string, inBlock bool) (string, bool) {
	var b strings.Builder
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inBlock {
			if c == '*' && i+1 < len(line) && line[i+1] == '/' {
				b.WriteString("  ")
				i++
				inBlock = false
			} else {
				b.WriteByte(' ')
			}
			continue
		}
		if quote != 0 {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(line) {
				i++
				b.WriteByte(line[i])
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch {
		case c == '"' || c == '\'' || c == '`':
			quote = c
			b.WriteByte(c)
		case c == '/' && i+1 < len(line) && line[i+1] == '/':
			for j := i; j < len(line); j++ {
				b.WriteByte(' ')
			}
			return b.String(), false
		case c == '/' && i+1 < len(line) && line[i+1] == '*':
			b.WriteString("  ")
			i++
			inBlock = true
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), inBlock
}

// pluginPkexecAllowed reports whether the command text starting at a pkexec
// token is covered by a declared privileged string: a declared string,
// whitespace normalised, must be a prefix of the normalised command text.
func pluginPkexecAllowed(fromPkexec string, privileged []string) bool {
	normal := pluginNormalizeCmd(fromPkexec)
	for _, declared := range privileged {
		declared = pluginNormalizeCmd(declared)
		if declared != "" && strings.HasPrefix(normal, declared) {
			return true
		}
	}
	return false
}

func pluginNormalizeCmd(s string) string {
	s = pluginCmdPunctRe.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// pluginAuditConfigWrite flags references to files a plugin must never write:
// shell config, ssh keys, /etc, and shell rc files. plugins.json is allowed when
// the line hands it to zenith-plugins-place, the sanctioned placement tool.
func pluginAuditConfigWrite(p string, line int, code string, add func(rule, p string, line int, msg string)) {
	if strings.Contains(code, "shell.json") {
		add("config-write", p, line, "references shell.json; settings go through pluginApi.saveSetting (R5)")
	}
	if strings.Contains(code, "plugins.json") && !strings.Contains(code, "zenith-plugins-place") {
		add("config-write", p, line, "references plugins.json; settings go through pluginApi.saveSetting (R5)")
	}
	if strings.Contains(code, "~/.ssh") {
		add("config-write", p, line, "touches ~/.ssh; write only under stateDir (R8)")
	}
	if strings.Contains(code, "/etc/") {
		add("config-write", p, line, "writes under /etc; write only under stateDir (R8)")
	}
	for _, rc := range []string{".bashrc", ".zshrc", "config.fish"} {
		if strings.Contains(code, rc) {
			add("config-write", p, line, "touches a shell rc file ("+rc+"); write only under stateDir (R8)")
			break
		}
	}
}

// pluginExtractCommands returns argv[0] of each Process command array and
// execDetached call on a line, skipping the plugin's own bin/ scripts and
// pluginDir paths.
func pluginExtractCommands(code string) []string {
	var names []string
	for _, re := range []*regexp.Regexp{pluginCmdArrayRe, pluginExecDetachedRe} {
		for _, match := range re.FindAllStringSubmatch(code, -1) {
			first := strings.TrimSpace(match[1])
			if strings.Contains(first, "pluginDir") {
				continue
			}
			value, ok := pluginUnquote(first)
			if !ok || value == "" {
				continue
			}
			if strings.HasPrefix(value, "bin/") || strings.HasPrefix(value, "./bin/") {
				continue
			}
			name := value
			if strings.Contains(name, "/") {
				name = path.Base(name)
			}
			names = append(names, name)
		}
	}
	return names
}

func pluginUnquote(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') {
		quote := s[0]
		if end := strings.IndexByte(s[1:], quote); end >= 0 {
			return s[1 : 1+end], true
		}
	}
	return "", false
}

func pluginCommandAllowed(name string, deps []string) bool {
	return pluginCommandAllowlist[name] || pluginContains(deps, name)
}

func pluginStripPort(host string) string {
	if i := strings.IndexByte(host, ':'); i >= 0 {
		return host[:i]
	}
	return host
}

func pluginHostAllowed(host string, network []string) bool {
	host = strings.ToLower(host)
	for _, declared := range network {
		if strings.ToLower(strings.TrimSpace(declared)) == host {
			return true
		}
	}
	return false
}

// pluginIsDynamicShell reports whether a line invokes sh/bash -c with an
// argument that is not a single string literal (R9): the array form
// ["sh","-c", <expr>], or an inline `sh -c` whose tail is built with string
// concatenation.
func pluginIsDynamicShell(code string) bool {
	for _, match := range pluginShcArrayRe.FindAllStringSubmatch(code, -1) {
		arg := strings.TrimRight(strings.TrimSpace(match[1]), " )]")
		if !pluginIsSingleStringLiteral(arg) {
			return true
		}
	}
	if match := pluginShcInlineRe.FindStringSubmatch(code); match != nil && strings.Contains(match[1], "+") {
		return true
	}
	return false
}

func pluginIsSingleStringLiteral(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return false
	}
	quote := s[0]
	if quote != '"' && quote != '\'' {
		return false
	}
	if s[len(s)-1] != quote {
		return false
	}
	return !strings.ContainsRune(s[1:len(s)-1], rune(quote))
}

// pluginImportEscapes resolves a relative import against the importing file's
// dir and reports whether it lands outside the plugin root.
func pluginImportEscapes(filePath, imp string) bool {
	target := path.Clean(path.Join(path.Dir(filePath), imp))
	return target == ".." || strings.HasPrefix(target, "../")
}

// pluginPrintAudit renders findings: one line per finding (blocking first, then
// warnings), then the summary. --json marshals the result directly.
func pluginPrintAudit(w io.Writer, result pluginAuditResult, asJSON bool) {
	if asJSON {
		body, _ := json.Marshal(result)
		fmt.Fprintln(w, string(body))
		return
	}
	for _, finding := range result.Blocking {
		fmt.Fprintf(w, "%s  %s:%d  %s\n", finding.Rule, finding.Path, finding.Line, finding.Message)
	}
	for _, finding := range result.Warnings {
		fmt.Fprintf(w, "%s  %s:%d  %s\n", finding.Rule, finding.Path, finding.Line, finding.Message)
	}
	fmt.Fprintf(w, "%d blocking, %d warnings\n", len(result.Blocking), len(result.Warnings))
}
