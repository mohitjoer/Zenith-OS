package main

import (
	"fmt"
	"os"
	"strings"

	"zenith/desktop/internal/hub"
	"zenith/desktop/internal/settings"
)

// settingsDir is where the schemas are installed.
//
// The .gschema.xml files rather than the compiled gschemas.compiled, because
// the tool needs types, ranges, enums and the schema's own prose — and
// gschemas.compiled carries values, not descriptions. The compiled file answers
// "what is this set to"; this answers "what is this, and what will it accept".
const settingsDir = "/usr/share/glib-2.0/schemas"

// settingsCatalogue finds Zenith's schemas, installed or in a checkout.
//
// The installed directory is the truth on a real machine. ZENITH_SCHEMA_DIR
// exists so the command also works in a checkout, because a command that only
// works after installation is a command nobody runs before installing — and the
// alternative, hard-coding a path relative to the working directory, breaks the
// moment anyone runs the binary from anywhere else.
func settingsCatalogue() (*settings.Catalogue, error) {
	if catalogue, err := settings.Load(settingsDir); err == nil {
		return catalogue, nil
	}

	override := os.Getenv("ZENITH_SCHEMA_DIR")
	if override == "" {
		return nil, fmt.Errorf(
			"Zenith's schemas are not installed in %s; install the zenith "+
				"package, or set ZENITH_SCHEMA_DIR to a directory of "+
				"org.zenith.*.gschema.xml", settingsDir)
	}
	catalogue, err := settings.Load(override)
	if err != nil {
		return nil, fmt.Errorf("ZENITH_SCHEMA_DIR=%s: %w", override, err)
	}
	return catalogue, nil
}

func zenithConfig(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, `zenith config — read and write Zenith's settings

Usage:
  zenith config list [--page SLUG]
  zenith config get SCHEMA KEY
  zenith config set SCHEMA KEY VALUE
  zenith config reset SCHEMA KEY
  zenith config pages

Every setting has a Hub page and every page has a command; "zenith config
pages" prints the mapping.
`)
		return exitUsage
	}

	catalogue, err := settingsCatalogue()
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith config:", err)
		return exitFail
	}

	switch args[0] {
	case "list":
		return configList(catalogue, args[1:])
	case "get":
		return configGet(catalogue, args[1:])
	case "set":
		return configSet(catalogue, args[1:])
	case "reset":
		return configReset(catalogue, args[1:])
	case "pages":
		return configPages()
	default:
		fmt.Fprintf(os.Stderr, "zenith config: unknown subcommand %q\n", args[0])
		return exitUsage
	}
}

func configList(catalogue *settings.Catalogue, args []string) int {
	only := ""
	if len(args) == 2 && args[0] == "--page" {
		only = args[1]
	} else if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "zenith config list: takes no arguments, "+
			"or --page SLUG")
		return exitUsage
	}

	// The current value is read once per setting, and its absence is not an
	// error. A machine with no dconf yet is a correct machine: every setting is
	// at its default, which is what the schema says it is.
	values := map[string]string{}
	if settings.Available() {
		for _, ref := range catalogue.Refs() {
			if value, err := catalogue.Get(ref); err == nil {
				values[ref.String()] = value
			}
		}
	}

	page := ""
	for _, ref := range catalogue.Refs() {
		qualified := ref.Schema + "." + ref.Key
		if only != "" {
			if found, ok := hub.Slug(qualified); !ok || found != only {
				continue
			}
		}

		if page != "" && page != ref.Schema {
			fmt.Println()
		}
		page = ref.Schema

		value, ok := values[ref.String()]
		if !ok {
			value = "(default)"
		}

		key, schema, _ := catalogue.Lookup(ref)
		fmt.Printf("%-34s %-20s %s", ref.Key, key.Type, value)

		if key.Enum != "" {
			fmt.Printf("   %s", strings.Join(schema.Nicks(key.Enum), "|"))
		} else if key.HasRange {
			fmt.Printf("   %d..%d", key.Min, key.Max)
		}
		if slug, ok := hub.Slug(qualified); ok {
			fmt.Printf("   [%s]", slug)
		}
		fmt.Println()
	}

	if only != "" {
		return exitOK
	}
	return exitOK
}

func configGet(catalogue *settings.Catalogue, args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "zenith config get: needs SCHEMA and KEY")
		return exitUsage
	}
	ref := settings.Ref{Schema: args[0], Key: args[1]}

	if !settings.Available() {
		fmt.Fprintln(os.Stderr, "zenith config: gsettings is not installed, "+
			"so the current value cannot be read; the default is in the schema")
		return exitFail
	}

	value, err := catalogue.Get(ref)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith config get:", err)
		return exitFail
	}
	fmt.Println(value)
	return exitOK
}

func configSet(catalogue *settings.Catalogue, args []string) int {
	if len(args) != 3 {
		fmt.Fprintln(os.Stderr, "zenith config set: needs SCHEMA, KEY and VALUE")
		return exitUsage
	}
	ref := settings.Ref{Schema: args[0], Key: args[1]}

	// Validated before gsettings is reached, and validated here rather than
	// there: a value that is well-formed GVariant and outside a declared range
	// is accepted by GSettings, and then the shell reads it and does something
	// nobody asked for.
	if err := catalogue.ValidateValue(ref, args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "zenith config set:", err)
		return exitFail
	}
	if !settings.Available() {
		fmt.Fprintln(os.Stderr, "zenith config: gsettings is not installed")
		return exitFail
	}
	if err := catalogue.Set(ref, args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "zenith config set:", err)
		return exitFail
	}

	// The page is named on success because "I changed it" and "I changed it
	// somewhere I can see it" are different sentences.
	if slug, ok := hub.Slug(ref.Schema + "." + ref.Key); ok {
		fmt.Printf("%s set. Visible in the Hub under %q.\n", ref, slug)
	} else {
		fmt.Printf("%s set.\n", ref)
	}
	return exitOK
}

func configReset(catalogue *settings.Catalogue, args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "zenith config reset: needs SCHEMA and KEY")
		return exitUsage
	}
	ref := settings.Ref{Schema: args[0], Key: args[1]}

	if !settings.Available() {
		fmt.Fprintln(os.Stderr, "zenith config: gsettings is not installed")
		return exitFail
	}
	if err := catalogue.Reset(ref); err != nil {
		fmt.Fprintln(os.Stderr, "zenith config reset:", err)
		return exitFail
	}
	fmt.Printf("%s reset to its default.\n", ref)
	return exitOK
}

func configPages() int {
	fmt.Printf("%-12s %-20s %s\n", "PAGE", "TITLE", "SETTINGS")
	for _, page := range hub.Pages {
		fmt.Printf("%-12s %-20s %s\n", page.Slug, page.Title,
			strings.Join(page.Keys, ", "))
	}
	fmt.Println()
	fmt.Println("Every setting has a page and every page has a command.")
	return exitOK
}
