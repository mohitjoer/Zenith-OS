package hub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zenith/desktop/internal/settings"
)

// realCatalogue is the settings this project actually ships.
//
// The invariant is checked against the real schemas rather than a fixture,
// because a registry and a schema that agree in a fixture can still disagree in
// the repository, and the whole point is that they are two lists.
func realCatalogue(t *testing.T) *settings.Catalogue {
	t.Helper()
	catalogue, err := settings.Load(filepath.Join("..", "..", "..", "shell"))
	if err != nil {
		t.Fatal(err)
	}
	return catalogue
}

// TestEverySettingHasAPageAndEveryPageHasCommands is the invariant, stated as
// the architecture record states it.
func TestEverySettingHasAPageAndEveryPageHasCommands(t *testing.T) {
	findings := Check(realCatalogue(t))
	if len(findings) != 0 {
		for _, f := range findings {
			t.Errorf("%s", f)
		}
	}
}

// TestPagesAreWellFormed covers the registry itself rather than its agreement
// with the schemas.
//
// A page with no slug cannot be addressed from a command line, and a page with no
// keys is an entry in the navigation that goes nowhere — which is a control the
// user can see and use that does nothing.
func TestPagesAreWellFormed(t *testing.T) {
	seen := map[string]bool{}

	for _, page := range Pages {
		if page.Slug == "" {
			t.Errorf("page %q has no slug, so it cannot be addressed", page.Title)
		}
		if seen[page.Slug] {
			t.Errorf("slug %q is used twice", page.Slug)
		}
		seen[page.Slug] = true

		if page.Title == "" {
			t.Errorf("page %q has no title", page.Slug)
		}
		if len(page.Keys) == 0 {
			t.Errorf("page %q has no settings; it is a navigation entry that "+
				"goes nowhere", page.Slug)
		}
	}
}

// TestSettingWithNoPageIsFound covers the first direction.
//
// A key with no page is a setting the user can change only by remembering a
// command line: it exists, it works, and it is undiscoverable.
func TestSettingWithNoPageIsFound(t *testing.T) {
	root := t.TempDir()
	write(t, root, "org.zenith.test.gschema.xml", `<?xml version="1.0"?>
<schemalist>
  <schema id="org.zenith.test" path="/org/zenith/test/">
    <key name="orphan" type="b"><default>false</default></key>
  </schema>
</schemalist>`)

	catalogue, err := settings.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	findings := Check(catalogue)
	found := false
	for _, f := range findings {
		if f.Ref == "org.zenith.test.orphan" {
			found = true
		}
	}
	if !found {
		t.Errorf("a setting with no page was not found: %v", findings)
	}
}

// TestPageShowingASettingThatDoesNotExist covers the second direction, and it is
// the one that matters.
//
// A page showing a setting no schema declares is a control the user can see and
// use, and it either writes dconf behind the control tool's back or does nothing
// at all.
func TestPageShowingASettingThatDoesNotExist(t *testing.T) {
	root := t.TempDir()
	// Every setting the pages name, except one.
	write(t, root, "org.zenith.test.gschema.xml", `<?xml version="1.0"?>
<schemalist>
  <schema id="org.zenith.desktop" path="/org/zenith/desktop/">
    <key name="color-scheme" type="s"><default>''</default></key>
  </schema>
</schemalist>`)

	catalogue, err := settings.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	findings := Check(catalogue)
	found := false
	for _, f := range findings {
		if strings.Contains(f.Reason, "inert") {
			found = true
		}
	}
	if !found {
		t.Errorf("a page showing a setting that does not exist was not "+
			"reported: %v", findings)
	}
}

// TestTwoPagesOwningOneSettingIsFound covers the third shape.
//
// Two pages editing one setting is not a duplicate — it is two places for them to
// disagree about what the value is, and neither one is wrong.
func TestTwoPagesOwningOneSettingIsFound(t *testing.T) {
	saved := Pages
	defer func() { Pages = saved }()

	Pages = []Page{
		{Slug: "one", Title: "One", Keys: []string{"org.zenith.desktop.color-scheme"}},
		{Slug: "two", Title: "Two", Keys: []string{"org.zenith.desktop.color-scheme"}},
	}

	findings := Check(realCatalogue(t))
	found := false
	for _, f := range findings {
		if strings.Contains(f.Reason, "also owned by") {
			found = true
		}
	}
	if !found {
		t.Errorf("two pages owning one setting was not reported: %v", findings)
	}
}

// TestQualifiedRefSplitsOnTheLastDot covers the parse.
//
// Schema ids contain dots and key names cannot, so the last dot is the separator.
// Splitting on the first produces schema "org" and key "zenith.shell.bar-height",
// and the resulting error is about a schema that does not exist rather than about
// the typo that caused it.
func TestQualifiedRefSplitsOnTheLastDot(t *testing.T) {
	ref := qualifiedRef("org.zenith.shell.bar-height")
	if ref.Schema != "org.zenith.shell" || ref.Key != "bar-height" {
		t.Errorf("qualifiedRef = %+v", ref)
	}

	ref = qualifiedRef("no-dots-here")
	if ref.Schema != "" || ref.Key != "no-dots-here" {
		t.Errorf("qualifiedRef on a dotless string = %+v", ref)
	}
}

// TestPageForAgreesWithTheSlugLookup covers two ways of asking the same
// question, which is two ways of getting different answers.
func TestPageForAgreesWithTheSlugLookup(t *testing.T) {
	for _, page := range Pages {
		for _, key := range page.Keys {
			ref := qualifiedRef(key)

			byRef, ok := PageFor(ref)
			if !ok || byRef.Slug != page.Slug {
				t.Errorf("PageFor(%s) = %q, %v; want %q", key, byRef.Slug, ok,
					page.Slug)
			}

			slug, ok := Slug(key)
			if !ok || slug != page.Slug {
				t.Errorf("Slug(%s) = %q, %v; want %q", key, slug, ok, page.Slug)
			}
		}
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
