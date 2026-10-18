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
		Slug:    "appearance",
		Title:   "Appearance",
		Summary: "Colour scheme and wallpaper",
		Keys: []string{
			"org.zenith.desktop.color-scheme",
			"org.zenith.shell.wallpaper-path",
			"org.zenith.shell.wallpaper-path-dark",
		},
	},
	{
		Slug:    "bar",
		Title:   "Bar",
		Summary: "Position, height and which widgets are on it",
		Keys: []string{
			"org.zenith.shell.bar-position",
			"org.zenith.shell.bar-height",
			"org.zenith.shell.bar-widgets",
		},
	},
	{
		Slug:    "dock",
		Title:   "Dock",
		Summary: "Position, auto-hide and pinned applications",
		Keys: []string{
			"org.zenith.shell.dock-position",
			"org.zenith.shell.dock-autohide",
			"org.zenith.shell.favorites",
		},
	},
	{
		Slug:    "launcher",
		Title:   "Launcher",
		Summary: "Which applications it offers and how it lays out",
		Keys: []string{
			"org.zenith.shell.launcher-apps",
			"org.zenith.shell.launcher-columns",
		},
	},
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
		if _, ok := owned[qualified]; !ok {
			findings = append(findings, Finding{
				Ref: qualified,
				Reason: "can be changed with `zenith config set` but no Hub " +
					"page shows it, so the only way to change it is to " +
					"remember a command line",
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
