// Package gschema reads the GSettings schemas Zenith ships.
//
// One parser, because the shell's source check and the control tool's settings
// catalogue need to read the same files and must not disagree about what is in
// them. Two parsers is one schema change handled twice, and the second one is
// the one nothing tests.
//
// It parses the subset of the format this project uses: schemas, keys, their
// types, enum-typed keys, ranges and defaults. It is not a general gschema
// parser and does not try to be — glib-compile-schemas is, and it runs at install
// time. This exists so that Go code can ask what settings exist without linking
// GLib.
package gschema

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Key is one setting.
type Key struct {
	// Name is the key name, kebab-case, as GSettings knows it.
	Name string
	// Type is the GVariant type: s, i, b, as, or an enum id.
	Type string
	// Enum is set when the key is enum-typed, and is the enum's id.
	Enum string
	// Min and Max bound an integer key. HasRange says which.
	Min, Max int
	HasRange bool
	// Default is the default value, as written in the schema.
	Default string
	// Summary and Description are the schema's own prose.
	Summary     string
	Description string
}

// Schema is one schema file's contents.
type Schema struct {
	// ID is the schema id, e.g. org.zenith.shell.
	ID string
	// Path is the dconf path the schema's keys live at.
	Path string
	// Keys are the keys, in file order.
	Keys []Key
	// Enums maps an enum id to its permitted values, in declaration order.
	Enums map[string][]EnumValue
}

// EnumValue is one permitted value of an enum.
type EnumValue struct {
	// Nick is the string form, e.g. "prefer-dark".
	Nick string
	// Value is the integer the enum assigns it.
	Value int
}

// Key returns a schema's key by name.
func (s Schema) Key(name string) (Key, bool) {
	for _, k := range s.Keys {
		if k.Name == name {
			return k, true
		}
	}
	return Key{}, false
}

// Nicks returns an enum's permitted values as strings, for a message that
// lists what a key will accept.
func (s Schema) Nicks(enumID string) []string {
	var nicks []string
	for _, v := range s.Enums[enumID] {
		nicks = append(nicks, v.Nick)
	}
	return nicks
}

type document struct {
	Schemas []struct {
		ID   string `xml:"id,attr"`
		Path string `xml:"path,attr"`
		Keys []struct {
			Name        string `xml:"name,attr"`
			Type        string `xml:"type,attr"`
			Enum        string `xml:"enum,attr"`
			Default     string `xml:"default"`
			Summary     string `xml:"summary"`
			Description string `xml:"description"`
			Range       *struct {
				Min string `xml:"min,attr"`
				Max string `xml:"max,attr"`
			} `xml:"range"`
		} `xml:"key"`
	} `xml:"schema"`
	Enums []struct {
		ID     string `xml:"id,attr"`
		Values []struct {
			Nick  string `xml:"nick,attr"`
			Value string `xml:"value,attr"`
		} `xml:"value"`
	} `xml:"enum"`
}

// Parse reads one gschema file.
func Parse(data []byte) (Schema, error) {
	var doc document
	if err := xml.Unmarshal(data, &doc); err != nil {
		return Schema{}, fmt.Errorf("not well-formed XML: %w", err)
	}

	schema := Schema{Enums: map[string][]EnumValue{}}
	for _, e := range doc.Enums {
		values := make([]EnumValue, 0, len(e.Values))
		for _, v := range e.Values {
			n := 0
			if _, err := fmt.Sscanf(v.Value, "%d", &n); err != nil {
				return Schema{}, fmt.Errorf("enum %s value %s: %w", e.ID, v.Nick, err)
			}
			values = append(values, EnumValue{Nick: v.Nick, Value: n})
		}
		schema.Enums[e.ID] = values
	}

	for _, s := range doc.Schemas {
		schema.ID = s.ID
		schema.Path = s.Path
		for _, k := range s.Keys {
			key := Key{
				Name: k.Name, Type: k.Type, Enum: k.Enum,
				Default: strings.TrimSpace(k.Default),
				Summary: strings.TrimSpace(k.Summary),
				// The description is a whole block of indented text. It is kept
				// as prose, and trimmed, because it is shown to a user in the
				// Hub and a description that reads like a YAML document is not
				// an explanation of anything.
				Description: strings.Join(strings.Fields(k.Description), " "),
			}
			if key.Enum != "" && key.Type == "" {
				key.Type = key.Enum
			}
			if k.Range != nil {
				key.HasRange = true
				fmt.Sscanf(k.Range.Min, "%d", &key.Min)
				fmt.Sscanf(k.Range.Max, "%d", &key.Max)
			}
			schema.Keys = append(schema.Keys, key)
		}
	}
	return schema, nil
}

// ParseFile reads one gschema file from disk.
func ParseFile(path string) (Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Schema{}, err
	}
	schema, err := Parse(data)
	if err != nil {
		return Schema{}, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return schema, nil
}

// LoadDir reads every .gschema.xml in a directory.
//
// The schemas ship as XML and the Hub needs types, ranges and prose that
// gschemas.compiled does not carry, so this reads the source rather than
// querying dconf for values.
func LoadDir(dir string) ([]Schema, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var schemas []Schema
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".gschema.xml") {
			continue
		}
		schema, err := ParseFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		schemas = append(schemas, schema)
	}

	// Sorted, because two runs over the same directory must produce the same
	// catalogue and a catalogue that reorders is a diff that lies.
	sort.Slice(schemas, func(i, j int) bool { return schemas[i].ID < schemas[j].ID })
	return schemas, nil
}
