// Package settings is the catalogue of everything a user can change about
// Zenith, and the commands that change it.
//
// It exists so that "every setting that has a command also has a page, and vice
// versa" can be checked rather than asserted. That sentence in
// docs/architecture.md is a claim about two lists — the keys the control tool
// can reach, and the pages the Hub draws — and a claim about two lists is only
// true if somebody compares them.
//
// The comparison is the point. A key with a command and no page is a setting the
// user can only change by remembering a command line, and a page with no
// command is a control that either does nothing or writes dconf behind the
// tool's back. Both are silent.
package settings

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"zenith/desktop/internal/gschema"
)

// Ref names one setting: a schema and a key.
type Ref struct {
	Schema string
	Key    string
}

func (r Ref) String() string { return r.Schema + " " + r.Key }

// Catalogue is every setting Zenith ships, plus the command that reaches it.
type Catalogue struct {
	schemas []gschema.Schema
}

// ZenithSchemaPrefix is the id prefix Zenith owns.
//
// Load keeps only these, and the filter is the point rather than a convenience.
//
// The schema directory on a real machine is /usr/share/glib-2.0/schemas, and it
// holds every schema the desktop has: GTK, dconf, gsettings-desktop-schemas,
// Firefox. Reading all of it produces a catalogue of several hundred settings
// that have nothing to do with Zenith — and the Hub's page registry would then
// be checked against GNOME's preferences, which says nothing about whether
// Zenith's own settings have pages.
//
// It also means `zenith config list` on a machine with GNOME installed lists
// GNOME's settings, which is not a tool anybody asked for.
const ZenithSchemaPrefix = "org.zenith."

// Load reads Zenith's own schemas from a directory of .gschema.xml files.
func Load(dir string) (*Catalogue, error) {
	schemas, err := gschema.LoadDir(dir)
	if err != nil {
		return nil, err
	}

	var own []gschema.Schema
	for _, schema := range schemas {
		if strings.HasPrefix(schema.ID, ZenithSchemaPrefix) {
			own = append(own, schema)
		}
	}
	if len(own) == 0 {
		return nil, fmt.Errorf("no org.zenith.* schemas in %s", dir)
	}
	return &Catalogue{schemas: own}, nil
}

// Schemas returns the schemas, sorted by id.
func (c *Catalogue) Schemas() []gschema.Schema { return c.schemas }

// Lookup finds a setting and returns the schema it belongs to.
func (c *Catalogue) Lookup(ref Ref) (gschema.Key, gschema.Schema, error) {
	for _, schema := range c.schemas {
		if schema.ID != ref.Schema {
			continue
		}
		key, ok := schema.Key(ref.Key)
		if !ok {
			return gschema.Key{}, gschema.Schema{}, fmt.Errorf(
				"%s has no key %q", ref.Schema, ref.Key)
		}
		return key, schema, nil
	}
	return gschema.Key{}, gschema.Schema{}, fmt.Errorf(
		"no schema %q; Zenith ships %s", ref.Schema, c.schemaIDs())
}

func (c *Catalogue) schemaIDs() string {
	ids := make([]string, 0, len(c.schemas))
	for _, s := range c.schemas {
		ids = append(ids, s.ID)
	}
	return strings.Join(ids, ", ")
}

// Refs returns every setting, sorted.
func (c *Catalogue) Refs() []Ref {
	var refs []Ref
	for _, schema := range c.schemas {
		for _, key := range schema.Keys {
			refs = append(refs, Ref{Schema: schema.ID, Key: key.Name})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Schema != refs[j].Schema {
			return refs[i].Schema < refs[j].Schema
		}
		return refs[i].Key < refs[j].Key
	})
	return refs
}

// Has reports whether a setting exists.
func (c *Catalogue) Has(ref Ref) bool {
	_, _, err := c.Lookup(ref)
	return err == nil
}

// ValidateValue checks a value against the key's type, range and enum.
//
// It is checked here rather than left to GSettings at write time because a
// rejected write is a message on stderr after the user has typed it, and a value
// that fails a range has already been committed to dconf by the time anyone
// notices.
func (c *Catalogue) ValidateValue(ref Ref, value string) error {
	key, schema, err := c.Lookup(ref)
	if err != nil {
		return err
	}

	if key.Enum != "" {
		nicks := schema.Nicks(key.Enum)
		for _, nick := range nicks {
			if nick == value {
				return nil
			}
		}
		return fmt.Errorf("%s must be one of %s", ref, strings.Join(nicks, ", "))
	}

	switch key.Type {
	case "i":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s is an integer, not %q", ref, value)
		}
		if key.HasRange && (n < key.Min || n > key.Max) {
			return fmt.Errorf("%s must be between %d and %d, not %d",
				ref, key.Min, key.Max, n)
		}
	case "b":
		if value != "true" && value != "false" {
			return fmt.Errorf("%s is true or false, not %q", ref, value)
		}
	case "as":
		if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
			return fmt.Errorf("%s is a list, written as ['a', 'b'], not %q",
				ref, value)
		}
	case "s":
	default:
		return fmt.Errorf("%s has type %q, which this tool does not know how "+
			"to validate", ref, key.Type)
	}
	return nil
}

// gsettings is the tool used to read and write values.
//
// GSettings rather than a binding because there is no pure-Go GSettings binding
// worth carrying, and `gsettings` is the interface GLib documents: it reads the
// same dconf the shell reads, it honours the same schemas, and it is the thing
// someone can run by hand when the tool and dconf disagree. Shelling out to the
// documented interface beats linking a C library this project has committed to
// not carrying — the same reasoning as apt-get and dpkg elsewhere.
func gsettings(args ...string) (string, error) {
	cmd := exec.Command("gsettings", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return "", fmt.Errorf("gsettings: %s", message)
		}
		return "", fmt.Errorf("gsettings: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Available reports whether gsettings is installed.
//
// It is checked rather than assumed because the control tool is built and tested
// on machines that are not Zenith, and `zenith config` on such a machine should
// say what is missing rather than reporting a missing schema.
func Available() bool {
	_, err := exec.LookPath("gsettings")
	return err == nil
}

// Get reads a setting's current value.
func (c *Catalogue) Get(ref Ref) (string, error) {
	if _, _, err := c.Lookup(ref); err != nil {
		return "", err
	}
	return gsettings("get", ref.Schema, ref.Key)
}

// Set writes a setting, after checking the value against the key.
//
// The check happens before the write rather than after, because a GVariant
// parse failure is reported by gsettings and leaves nothing behind — but a value
// that is well-formed GVariant and outside a declared range is accepted, and then
// the shell reads it and does something nobody asked for.
func (c *Catalogue) Set(ref Ref, value string) error {
	if err := c.ValidateValue(ref, value); err != nil {
		return err
	}
	_, err := gsettings("set", ref.Schema, ref.Key, value)
	return err
}

// Reset returns a setting to its default.
func (c *Catalogue) Reset(ref Ref) error {
	if _, _, err := c.Lookup(ref); err != nil {
		return err
	}
	_, err := gsettings("reset", ref.Schema, ref.Key)
	return err
}
