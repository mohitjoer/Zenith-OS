package i18n

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The catalogs are data the shell reads by file name and by key, so nothing
// fails to compile when one is wrong: a catalog that is not JSON is English in
// that language, and a placeholder lost in translation prints a literal "%1".
// This reads them the way I18n.qml does.

var i18nPlaceholder = regexp.MustCompile(`%(?:%|\d|\[\d+\][a-zA-Z]|[-+#0-9.]*[a-zA-Z])`)

var i18nCall = regexp.MustCompile(`I18n\.tr\(\s*(?:"((?:\\.|[^"\\])*)"|'((?:\\.|[^'\\])*)')`)

func placeholders(s string) string {
	found := i18nPlaceholder.FindAllString(s, -1)
	sort.Strings(found)
	return strings.Join(found, " ")
}

func readCatalog(t *testing.T, path string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s is not a JSON object of strings: %v", filepath.Base(path), err)
	}
	return m
}

func TestI18nCatalogs(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "shell", "i18n", "data")

	var table struct {
		Languages []struct{ Code, Dir string } `json:"languages"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "langs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &table); err != nil || len(table.Languages) == 0 {
		t.Fatalf("langs.json: %v", err)
	}

	// English is the key set, and each key is its own translation.
	en := readCatalog(t, filepath.Join(dir, "en.json"))
	for k, v := range en {
		if k != v {
			t.Fatalf("en.json maps %q to %q; English is the source string", k, v)
		}
	}

	shipped := map[string]bool{"langs.json": true, "en.json": true}
	for _, l := range table.Languages {
		if l.Dir != "ltr" && l.Dir != "rtl" {
			t.Errorf("%s: dir %q is neither ltr nor rtl", l.Code, l.Dir)
		}
		if l.Code == "en" {
			continue
		}
		shipped[l.Code+".json"] = true
		cat := readCatalog(t, filepath.Join(dir, l.Code+".json"))
		if len(cat) != len(en) {
			t.Errorf("%s: %d strings, en.json has %d", l.Code, len(cat), len(en))
		}
		for k, v := range cat {
			if _, ok := en[k]; !ok {
				t.Errorf("%s: %q is not in en.json", l.Code, k)
			} else if placeholders(k) != placeholders(v) {
				t.Errorf("%s: %q => %q changes the placeholders", l.Code, k, v)
			}
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !shipped[e.Name()] {
			t.Errorf("%s ships but langs.json has no such language", e.Name())
		}
	}

	// A string the shell wraps and the catalog lacks is shown in English, which
	// is correct and silent. A little of that is normal while the tree moves; a
	// catalog that covers none of it is the wrong one.
	total, covered := 0, 0
	var missing []string
	err = filepath.WalkDir(filepath.Join(root, "shell"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(filepath.Join(root, "shell"), path)
		if d.IsDir() {
			switch filepath.ToSlash(rel) {
			case "i18n", "vendor", "modules/ii":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".qml") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, m := range i18nCall.FindAllStringSubmatch(string(src), -1) {
			body, single := m[1], m[2] != ""
			if single {
				body = strings.ReplaceAll(strings.ReplaceAll(m[2], `\'`, `'`), `"`, `\"`)
			}
			key, err := strconv.Unquote(`"` + body + `"`)
			key = strings.TrimSpace(key)
			if err != nil || len(key) < 2 || !regexp.MustCompile(`[A-Za-z]`).MatchString(key) {
				continue
			}
			total++
			if _, ok := en[key]; ok {
				covered++
			} else if len(missing) < 8 {
				missing = append(missing, key)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d of %d I18n.tr strings in shell/ are in the catalog; the rest show in English, e.g. %q",
		covered, total, missing)
	if total == 0 || covered*100 < total*95 {
		t.Errorf("only %d of %d I18n.tr strings in shell/ are in shell/i18n/data/en.json", covered, total)
	}
}
