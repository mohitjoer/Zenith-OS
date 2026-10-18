package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Load reads a profile file and returns it in the shape Lint expects.
//
// Only the section-aware scalar fields are extracted. Anything else in the file
// is left alone, which means an unrecognised key is ignored rather than
// misread — a linter that guesses at structure will eventually reject a valid
// profile, and that is the worst failure direction for a tool whose output is a
// merge decision.
func Load(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("lint: reading %s: %w", path, err)
	}

	p := Profile{
		Path:  filepath.Base(path),
		Raw:   data,
		lines: strings.Split(string(data), "\n"),
		Match: map[string]string{},
	}

	// section is the top-level key the cursor is inside: "match", "firmware",
	// "network", and so on. key is the innermost key of the current line.
	section := ""
	inMatch := false

	for _, raw := range p.lines {
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)

		// A line at column zero always ends the current section, whether it
		// opens a new one or is a scalar of its own. Keying that off the
		// trailing colon alone silently drops every top-level scalar that
		// follows a nested block — which is where `kernel:` lives in the
		// documented schema, so the field a profile most needs validated went
		// unread.
		if indent == 0 {
			key, value, found := strings.Cut(trimmed, ":")
			if !found {
				continue
			}
			key = strings.TrimSpace(key)
			value = unquote(strings.TrimSpace(value))

			if value == "" {
				section = key
				inMatch = key == "match"
				continue
			}

			section = ""
			inMatch = false
			switch key {
			case "version":
				p.Version = value
			case "id":
				p.ID = value
			case "license":
				p.License = value
			case "tier":
				p.Tier = atoi(value)
			case "kernel":
				p.Kernel = value
			}
			continue
		}

		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = unquote(strings.TrimSpace(value))

		// A list item belongs to the key of the line above it. The firmware
		// packages are the only list this reader needs, and they are written in
		// the flow form the schema example uses: firmware: [a, b].
		if strings.HasPrefix(key, "- ") {
			continue
		}

		switch {
		case inMatch:
			p.Match[key] = value

		case section == "firmware":
			switch key {
			case "packages":
				p.FirmwarePackages = append(p.FirmwarePackages, parseFlowList(value)...)
			case "microcode":
				p.Microcode = append(p.Microcode, parseFlowList(value)...)
			}
		}
	}

	return p, nil
}

// LoadDir reads every profile in a directory.
//
// The example profile is skipped by name rather than by shape: it is annotated
// documentation with placeholder values, and a linter that reports it teaches
// contributors to ignore the tool.
func LoadDir(dir string) ([]Profile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("lint: reading %s: %w", dir, err)
	}

	var profiles []Profile
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".yaml") || name == "example.yaml" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		profile, err := Load(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// Run is the whole check: every profile, then the database-wide pass.
func Run(profiles []Profile) ([]Finding, error) {
	if len(profiles) == 0 {
		return nil, fmt.Errorf("lint: no profiles found; an empty database lints " +
			"clean, which is the one result that should never be printed")
	}

	var findings []Finding
	for _, profile := range profiles {
		findings = append(findings, Lint(profile)...)
	}
	findings = append(findings, LintDatabase(profiles)...)
	return findings, nil
}

// stripComment removes a trailing comment, respecting quotes.
//
// Firmware-version strings and DMI values can contain a "#", so removing
// comments without regard for quoting would truncate a legitimate value and
// either miss a finding or invent one.
func stripComment(line string) string {
	inSingle, inDouble := false, false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				// Only a comment when it starts the line or follows a space,
				// so a value like "GPU#1" is not truncated.
				if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
					return line[:i]
				}
			}
		}
	}
	return line
}

// unquote removes surrounding quotes and folds the escapes a double-quoted YAML
// scalar may carry.
func unquote(value string) string {
	if len(value) >= 2 {
		if value[0] == '"' && value[len(value)-1] == '"' {
			inner := value[1 : len(value)-1]
			inner = strings.ReplaceAll(inner, `\"`, `"`)
			inner = strings.ReplaceAll(inner, `\\`, `\`)
			return inner
		}
		if value[0] == '\'' && value[len(value)-1] == '\'' {
			return strings.ReplaceAll(value[1:len(value)-1], "''", "'")
		}
	}
	return value
}

// parseFlowList reads a flow sequence: [a, b, c].
func parseFlowList(value string) []string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		if value == "" || value == "[]" {
			return nil
		}
		// A single scalar in place of a sequence.
		return []string{unquote(value)}
	}

	inner := strings.TrimSpace(value[1 : len(value)-1])
	if inner == "" {
		return nil
	}

	var values []string
	for _, part := range strings.Split(inner, ",") {
		if trimmed := unquote(strings.TrimSpace(part)); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func atoi(value string) int {
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
		if n > 100 {
			return 0
		}
	}
	return n
}
