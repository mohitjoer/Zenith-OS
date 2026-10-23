// Package hub is the settings application: which settings exist, which page
// shows them, and the check that the two agree.
//
// The check is the reason this package exists. docs/architecture.md states that
// "every setting that has a command also has a page, and vice versa", and a
// sentence like that is a claim about two lists that nothing compares. Both
// directions fail silently and differently:
//
//   - A key with no page is a setting the user can change only by remembering a
//     command line. It exists, it works, and it is undiscoverable.
//   - A page with no command is a control the user can see and use. Either it
//     writes dconf behind the control tool's back, which the architecture record
//     forbids, or it does nothing at all.
//
// The second is the worse one, because the first at least leaves a trace in the
// tool's help.
//
// The one exception is a setting nothing reads (Unread): it has no page on
// purpose, because a page for it would be the second fault.
package hub

import (
	"fmt"
	"sort"
	"strings"

	"zenith/desktop/internal/settings"
)

// Page is one screen in the Hub.
type Page struct {
	// Slug is the stable name, used in the command line and as the file name.
	Slug string
	// Title is what the navigation shows.
	Title string
	// Summary is one line of description.
	Summary string
	// Keys are the settings this page owns, as "schema.key".
	Keys []string
}

// Pages is the Hub's navigation.
//
// Declared here rather than discovered from the QML because this list is the
// thing the invariant is checked against. A registry built by scanning a
// directory would contain whatever is on disk, which means a stray file becomes
// a page and a forgotten page is simply absent — the same shape of fault as the
// shell's widget catalogue, and it is listed explicitly for the same reason.
var Pages = []Page{
	{
		Slug:    "idle",
		Title:   "Idle and Locking",
		Summary: "When the screen dims, blanks and locks",
		Keys: []string{
			"org.zenith.desktop.idle-dim-seconds",
			"org.zenith.desktop.idle-blank-seconds",
			"org.zenith.desktop.idle-lock-seconds",
		},
	},
}

// Unread lists the settings that have a command and no page, because nothing in
// the running desktop reads them. A control for one is a control wired to
// nothing, which is the second fault above in another form: the user changes it
// and the desktop stays as it was. Each reason is what was found when the key was
// put here. The day something reads the key it comes off this list and onto a
// page, and Check reports a key that is on both.
//
// The schemas still declare them: the legacy shell (legacy-shell.qml and the
// files at the shell's root) reads them, and the importer in internal/adopt
// writes some of them.
var Unread = map[string]string{
	"org.zenith.desktop.color-scheme":      "read once by zenith-wallpaper to pick a fresh account's starting wallpaper, never again",
	"org.zenith.shell.wallpaper-path":      "read once by zenith-wallpaper for a fresh account's starting wallpaper, never again",
	"org.zenith.shell.wallpaper-path-dark": "read once by zenith-wallpaper for a fresh account's starting wallpaper, never again",
	"org.zenith.shell.bar-position":        "no reader",
	"org.zenith.shell.bar-height":          "no reader",
	"org.zenith.shell.bar-widgets":         "read only by the legacy shell's Bar.qml",
	"org.zenith.shell.dock-position":       "no reader",
	"org.zenith.shell.dock-autohide":       "read only by the legacy shell's Dock.qml",
	"org.zenith.shell.favorites":           "read only by the legacy shell's Dock.qml",
	"org.zenith.shell.launcher-apps":       "read only by the legacy shell's Launcher.qml",
	"org.zenith.shell.launcher-columns":    "no reader",
}

// Control is the widget a setting is edited with.
//
// The mapping from a GVariant type to a widget lives here rather than in the
// QML, because it is the second half of a third invariant: every setting type
// the schemas use has a control, and every control is used by some setting.
//
// Both directions are silent otherwise. A type with no control is a page that
// renders the row and leaves the value unchangeable — the setting appears, the
// control does not, and nothing says why. A control no setting uses is a widget
// written, styled and shipped for nothing.
//
// It is keyed by the GVariant type and by "enum" for enum-typed keys, because an
// enum is not a type of its own: it is an integer with named values, and the
// control it wants is a list of those names rather than a number.
var controls = map[string]string{
	"s":    "text",
	"i":    "spin",
	"b":    "switch",
	"as":   "list",
	"enum": "choice",
}

// ControlFor returns the control a key is edited with.
func ControlFor(keyType string, enum string) (string, bool) {
	if enum != "" {
		control, ok := controls["enum"]
		return control, ok
	}
	control, ok := controls[keyType]
	return control, ok
}

// UsedTypes returns every (type, enum) pair the catalogue actually contains, in
// sorted order.
func UsedTypes(catalogue *settings.Catalogue) []string {
	seen := map[string]bool{}
	for _, ref := range catalogue.Refs() {
		key, _, err := catalogue.Lookup(ref)
		if err != nil {
			continue
		}
		kind := key.Type
		if key.Enum != "" {
			kind = "enum"
		}
		seen[kind] = true
	}

	var kinds []string
	for kind := range seen {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// CheckControls asserts that every setting type in the schemas has a control, and
// that every control is used by some setting.
//
// The first direction is the one a user meets: a type with no control is a row
// that renders and cannot be changed, which looks like a broken Hub rather than
// a missing widget.
func CheckControls(catalogue *settings.Catalogue) []Finding {
	var findings []Finding

	used := map[string]bool{}
	for _, kind := range UsedTypes(catalogue) {
		control, ok := ControlFor(kind, "")
		if !ok {
			findings = append(findings, Finding{
				Ref: kind,
				Reason: "no control is mapped to this setting type, so a page " +
					"showing one renders the row and leaves it unchangeable",
			})
			continue
		}
		used[control] = true
	}

	for _, kind := range sortedKeys(controls) {
		if used[controls[kind]] {
			continue
		}
		findings = append(findings, Finding{
			Ref: kind,
			Reason: fmt.Sprintf("control %q is mapped but no setting uses "+
				"type %s; it is a widget written for nothing",
				controls[kind], kind),
		})
	}

	return findings
}

func sortedKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Finding is one disagreement between the settings and the pages.
type Finding struct {
	// Page is the page involved, or "" for a setting with no page.
	Page string
	// Ref is the setting involved, or "" for a page with no command.
	Ref string
	// Reason is what is wrong.
	Reason string
}

func (f Finding) String() string {
	if f.Page == "" {
		return f.Ref + ": " + f.Reason
	}
	return "page " + f.Page + ": " + f.Reason
}

// Check compares every setting against every page, in both directions.
//
// It returns findings rather than a boolean because the two directions have
// different fixes, and a user reading one line needs to know which it was: add
// a page, or remove a key from one.
func Check(catalogue *settings.Catalogue) []Finding {
	var findings []Finding

	// "schema.key" is the form the page list uses and the form a user types.
	owned := map[string]string{}

	for _, page := range Pages {
		for _, key := range page.Keys {
			if other, taken := owned[key]; taken {
				findings = append(findings, Finding{
					Page: page.Slug,
					Ref:  key,
					Reason: fmt.Sprintf("also owned by page %q; two pages "+
						"editing one setting is two places for them to disagree",
						other),
				})
				continue
			}
			owned[key] = page.Slug
		}
	}

	// Direction one: a setting with a command and no page.
	for _, ref := range catalogue.Refs() {
		qualified := ref.Schema + "." + ref.Key
		_, unread := Unread[qualified]
		if _, ok := owned[qualified]; !ok && !unread {
			findings = append(findings, Finding{
				Ref: qualified,
				Reason: "can be changed with `zenith config set` but no Hub " +
					"page shows it, so the only way to change it is to " +
					"remember a command line",
			})
		}
	}

	for qualified := range Unread {
		if page, ok := owned[qualified]; ok {
			findings = append(findings, Finding{
				Page: page,
				Ref:  qualified,
				Reason: "is on this page and on the Unread list; a setting " +
					"with a reader belongs on a page, one without belongs on " +
					"the list",
			})
		}
		if !catalogue.Has(qualifiedRef(qualified)) {
			findings = append(findings, Finding{
				Ref: qualified,
				Reason: "is on the Unread list and no schema declares it; " +
					"delete the entry",
			})
		}
	}

	// Direction two: a page showing a setting with no command. This is the one
	// that matters, because the control would be visible and either inert or
	// writing dconf outside the tool.
	for _, page := range Pages {
		for _, key := range page.Keys {
			if !catalogue.Has(qualifiedRef(key)) {
				findings = append(findings, Finding{
					Page: page.Slug,
					Ref:  key,
					Reason: "shows a setting that no schema declares, so " +
						"`zenith config` cannot reach it; the control would " +
						"be inert",
				})
			}
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Page != findings[j].Page {
			return findings[i].Page < findings[j].Page
		}
		return findings[i].Ref < findings[j].Ref
	})
	return findings
}

// PageFor returns the page that owns a setting, if any.
func PageFor(ref settings.Ref) (Page, bool) {
	qualified := ref.Schema + "." + ref.Key
	for _, page := range Pages {
		for _, key := range page.Keys {
			if key == qualified {
				return page, true
			}
		}
	}
	return Page{}, false
}

// Slug returns the page slug for a qualified "schema.key" string.
func Slug(qualified string) (string, bool) {
	for _, page := range Pages {
		for _, key := range page.Keys {
			if key == qualified {
				return page.Slug, true
			}
		}
	}
	return "", false
}

// Document is the page registry in the form the Hub window reads.
//
// The window reads this instead of repeating the registry in QML, and that is
// the whole reason it exists. A registry written twice is two lists that agree
// until someone edits one of them, and the failure is a page showing settings it
// does not own, or a setting with no page — which is the exact thing the
// invariant above exists to prevent.
//
// JSON rather than QML rather than a bespoke format: the QML side parses it with
// one call, and the Go side writes it with the standard library.
type Document struct {
	Pages []DocumentPage `json:"pages"`
}

// DocumentPage is one page and the settings it shows.
type DocumentPage struct {
	Slug    string        `json:"slug"`
	Title   string        `json:"title"`
	Summary string        `json:"summary"`
	Keys    []DocumentKey `json:"keys"`
}

// DocumentKey is one setting, fully described.
//
// Everything the window needs to draw a control and to describe it, including
// the current value — so the window never has to read dconf itself, and the only
// writer stays `zenith config`.
type DocumentKey struct {
	Schema   string   `json:"schema"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Control  string   `json:"control"`
	Value    string   `json:"value"`
	Default  string   `json:"default"`
	Summary  string   `json:"summary"`
	Minimum  int      `json:"minimum,omitempty"`
	Maximum  int      `json:"maximum,omitempty"`
	HasRange bool     `json:"hasRange,omitempty"`
	Choices  []string `json:"choices,omitempty"`
}

// Export builds the document.
//
// A setting whose value cannot be read is exported with an empty value rather
// than being omitted. A window that silently drops a row when dconf is
// unavailable is a window whose settings list depends on machine state, and
// "the row is missing" is much harder to report than "the row has no value".
func Export(catalogue *settings.Catalogue) Document {
	document := Document{}

	for _, page := range Pages {
		out := DocumentPage{
			Slug:    page.Slug,
			Title:   page.Title,
			Summary: page.Summary,
			Keys:    []DocumentKey{},
		}

		for _, qualified := range page.Keys {
			ref := qualifiedRef(qualified)
			key, schema, err := catalogue.Lookup(ref)
			if err != nil {
				// The invariant check reports this properly; here it means the
				// window shows the page without the row.
				continue
			}

			control, _ := ControlFor(key.Type, key.Enum)

			entry := DocumentKey{
				Schema:   ref.Schema,
				Name:     ref.Key,
				Type:     key.Type,
				Control:  control,
				Default:  key.Default,
				Summary:  key.Summary,
				Minimum:  key.Min,
				Maximum:  key.Max,
				HasRange: key.HasRange,
				Choices:  schema.Nicks(key.Enum),
			}
			if value, err := catalogue.Get(ref); err == nil {
				entry.Value = value
			}
			out.Keys = append(out.Keys, entry)
		}
		document.Pages = append(document.Pages, out)
	}
	return document
}

// qualifiedRef turns "org.zenith.shell.bar-height" back into a Ref.
//
// Split on the *last* dot rather than the first: schema ids contain dots and key
// names cannot, so the last one is the separator. Splitting on the first would
// produce schema "org" and key "zenith.shell.bar-height", and the error would be
// about a schema that does not exist rather than about a typo.
func qualifiedRef(qualified string) settings.Ref {
	i := strings.LastIndex(qualified, ".")
	if i < 0 {
		return settings.Ref{Key: qualified}
	}
	return settings.Ref{Schema: qualified[:i], Key: qualified[i+1:]}
}
