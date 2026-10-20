package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"zenith/desktop/internal/gschema"
	"zenith/desktop/internal/settings"
)

// The shell reads its settings through this command.
//
// The shell is QML, and Quickshell has no GSettings type — it is a compositor
// shell, not a settings library, and it will not grow one. So the shell asks
// the one tool that already understands the schemas: the same catalogue the
// doctor validates against and the Hub writes through.
//
// One command rather than one per key, because the shell wants the whole set
// at once and a shell that read fourteen keys by spawning fourteen processes
// would spend its first second starting processes.
//
// The values are typed rather than printed, and the names are camelCase
// rather than kebab-case, because the shell's own code reads them as
// `Config.barHeight`. A setting that is read through two different names in
// two different places is a setting that is renamed in one place.
//
// An enum is a number here and not a nick, because the shell compares an edge
// against `Config.edgeBottom` and the mapping from nick to number belongs to
// the schema, not to every file that wants an edge.

// configValues prints every setting, as JSON, for the shell.
func configValues(catalogue *settings.Catalogue) int {
	values := map[string]map[string]any{}

	for _, schema := range catalogue.Schemas() {
		keys := map[string]any{}
		for _, key := range schema.Keys {
			keys[camelCase(key.Name)] = schemaValue(catalogue, schema, key)
		}
		values[schema.ID] = keys
	}

	encoded, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith config values:", err)
		return exitFail
	}
	fmt.Println(string(encoded))
	return exitOK
}

// schemaValue is one key's current value, typed.
func schemaValue(catalogue *settings.Catalogue, schema gschema.Schema, key gschema.Key) any {
	raw := key.Default
	if settings.Available() {
		if value, err := catalogue.Get(settings.Ref{Schema: schema.ID, Key: key.Name}); err == nil {
			raw = value
		}
	}

	// An enum's value is its nick in GVariant form and its number everywhere
	// else, and the number is the enum's own: the schema says which nick is
	// which, and nothing else gets to say it again.
	if key.Enum != "" {
		nick := strings.Trim(raw, "'")
		for _, value := range schema.Enums[key.Enum] {
			if value.Nick == nick {
				return value.Value
			}
		}
		return 0
	}

	switch key.Type {
	case "b":
		return raw == "true"
	case "i":
		number, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return 0
		}
		return number
	case "as":
		return parseStringList(raw)
	default:
		return strings.Trim(raw, "'")
	}
}

// parseStringList reads a GVariant string list.
//
// A hand-written reader rather than a GVariant parser, because the only lists
// in these schemas are lists of plain strings and a dependency that can parse
// GVariant would be a dependency this project has committed to not carrying.
func parseStringList(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return []string{}
	}

	body := trimmed[1 : len(trimmed)-1]
	var items []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	flush := func() {
		if current.Len() > 0 {
			items = append(items, current.String())
			current.Reset()
		}
	}

	for _, r := range body {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '\'':
			inQuotes = !inQuotes
		case r == ',' && !inQuotes:
			flush()
		case (r == ' ' || r == '\t') && !inQuotes:
			// The space between two items is GVariant's, not the value's.
		default:
			current.WriteRune(r)
		}
	}
	flush()

	if items == nil {
		return []string{}
	}
	return items
}

// camelCase turns a GSettings key name into the name the shell reads it by.
//
// `bar-height` becomes `barHeight`, and a key that is already camelCase is
// left alone rather than being mangled by a rule that assumes kebab-case. The
// first word keeps its case: a setting named `BarHeight` is a different
// setting from one named `barHeight`, and the schema's own name is the one
// that decides.
func camelCase(name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			parts[i] = strings.ToLower(part[:1]) + part[1:]
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}
