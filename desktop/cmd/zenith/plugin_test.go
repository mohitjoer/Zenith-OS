package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pluginTestWrite lays a plugin tree under a fresh temp dir: manifest.json plus
// the named entry files, each with placeholder content. It returns the dir.
func pluginTestWrite(t *testing.T, manifestJSON string, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifestJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("// placeholder\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// a well-formed manifest with both entry points present.
const pluginGoodManifest = `{
  "id": "weathercard",
  "name": "Weather Card",
  "version": "1.2.0",
  "hosts": ["desktopWidget", "topbarGlyph"],
  "entryPoints": { "main": "service/Main.qml", "content": "content/Widget.qml" }
}`

func TestPluginValidateManifestGood(t *testing.T) {
	dir := pluginTestWrite(t, pluginGoodManifest, "service/Main.qml", "content/Widget.qml")
	m, err := pluginValidateManifest(dir, map[string]bool{"clock": true})
	if err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	if m.ID != "weathercard" || m.Version != "1.2.0" {
		t.Fatalf("parsed %+v", m)
	}
}

func TestPluginValidateManifestRejections(t *testing.T) {
	reserved := map[string]bool{"clock": true, "launcher": true}
	cases := []struct {
		name     string
		manifest string
		files    []string
		want     string
	}{
		{
			name: "reserved id",
			manifest: `{"id":"clock","name":"Impostor","version":"1.0.0",
				"hosts":["topbarGlyph"],"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`,
			files: []string{"service/Main.qml", "content/Widget.qml"},
			want:  "reserved",
		},
		{
			name: "missing entry point file",
			manifest: `{"id":"noentry","name":"No Entry","version":"1.0.0",
				"hosts":["topbarGlyph"],"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`,
			files: []string{"service/Main.qml"}, // content missing
			want:  "does not exist",
		},
		{
			name: "escaping entry point",
			manifest: `{"id":"escape","name":"Escape","version":"1.0.0",
				"hosts":["topbarGlyph"],"entryPoints":{"main":"../outside.qml","content":"content/Widget.qml"}}`,
			files: []string{"content/Widget.qml"},
			want:  "relative path",
		},
		{
			name: "bad host",
			manifest: `{"id":"badhost","name":"Bad Host","version":"1.0.0",
				"hosts":["islandGlyph"],"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`,
			files: []string{"service/Main.qml", "content/Widget.qml"},
			want:  "unknown host",
		},
		{
			name: "no hosts",
			manifest: `{"id":"nohost","name":"No Host","version":"1.0.0",
				"hosts":[],"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`,
			files: []string{"service/Main.qml", "content/Widget.qml"},
			want:  "no hosts",
		},
		{
			name: "bad id",
			manifest: `{"id":"Bad Id","name":"Bad","version":"1.0.0",
				"hosts":["topbarGlyph"],"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`,
			files: []string{"service/Main.qml", "content/Widget.qml"},
			want:  "lowercase",
		},
		{
			name: "missing version",
			manifest: `{"id":"nover","name":"No Ver",
				"hosts":["topbarGlyph"],"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`,
			files: []string{"service/Main.qml", "content/Widget.qml"},
			want:  "version",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := pluginTestWrite(t, c.manifest, c.files...)
			_, err := pluginValidateManifest(dir, reserved)
			if err == nil {
				t.Fatalf("want error containing %q, got nil", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q, want substring %q", err, c.want)
			}
		})
	}
}

// TestPluginValidateManifestSymlink rejects a plugin that ships a symlink
// anywhere in its tree.
func TestPluginValidateManifestSymlink(t *testing.T) {
	dir := pluginTestWrite(t, pluginGoodManifest, "service/Main.qml", "content/Widget.qml")
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "content", "sneaky.qml")); err != nil {
		t.Skipf("symlink not supported here: %v", err)
	}
	_, err := pluginValidateManifest(dir, map[string]bool{})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink must be rejected, got %v", err)
	}
}

// TestPluginSafeRel pins the relative-path guard entry-point validation relies on.
func TestPluginSafeRel(t *testing.T) {
	good := []string{"a.qml", "service/Main.qml", "./x/y.qml"}
	bad := []string{"", "/abs/path.qml", "../escape.qml", "a/../../b.qml"}
	for _, p := range good {
		if !pluginSafeRel(p) {
			t.Errorf("pluginSafeRel(%q) = false, want true", p)
		}
	}
	for _, p := range bad {
		if pluginSafeRel(p) {
			t.Errorf("pluginSafeRel(%q) = true, want false", p)
		}
	}
}

// TestPluginReservedIDs checks the built-in widget ids are reserved, whether
// they come from the catalogue file or the fallback set, and that a made-up id
// is not.
func TestPluginReservedIDs(t *testing.T) {
	reserved := pluginReservedIDs()
	for _, id := range []string{"clock", "launcher", "cputemp", "ai"} {
		if !reserved[id] {
			t.Errorf("built-in id %q not reserved", id)
		}
	}
	if reserved["totally-made-up"] {
		t.Error("a non-built-in id must not be reserved")
	}
}

// pluginTestEnv points every directory the tool reads at a fresh temp tree and
// returns the data root and the dev-override root.
func pluginTestEnv(t *testing.T) (data, dev string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	dev = filepath.Join(root, "dev")
	t.Setenv("ZENITH_PLUGINS_DIR", dev)
	return pluginDataRoot(), dev
}

func pluginTestPutManifest(t *testing.T, dir, manifest string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPluginListRows merges an installed plugin and a dev override with the
// plugins.json placement, marking each row's source.
func TestPluginListRows(t *testing.T) {
	data, dev := pluginTestEnv(t)
	installed := filepath.Join(data, "weathercard")
	pluginTestPutManifest(t, installed, pluginGoodManifest)
	devDir := filepath.Join(dev, "devcard")
	pluginTestPutManifest(t, devDir, `{"id":"devcard","name":"Dev Card","version":"9.9.9","hosts":["topbarGlyph"],
		"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`)
	if err := os.MkdirAll(filepath.Dir(pluginConfigFile()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginConfigFile(), []byte(`{"weathercard":{"enabled":true,"host":"topbarGlyph"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	byID := map[string]pluginRow{}
	rows := pluginListRows()
	for _, r := range rows {
		byID[r.ID] = r
	}
	got, ok := byID["weathercard"]
	if !ok {
		t.Fatalf("installed plugin missing from list: %+v", rows)
	}
	if got.Source != "installed" || got.Version != "1.2.0" || !got.Enabled || got.Host != "topbarGlyph" || got.Dir != installed {
		t.Fatalf("installed row = %+v", got)
	}
	dev1, ok := byID["devcard"]
	if !ok || dev1.Source != "dev" || dev1.Version != "9.9.9" || dev1.Dir != devDir {
		t.Fatalf("dev row = %+v (rows %+v)", dev1, rows)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d: %+v", len(rows), rows)
	}
}

// TestPluginListDevShadowsInstalled checks a dev override shadows an installed
// plugin of the same id, listed once and marked dev, as discover.sh loads it.
func TestPluginListDevShadowsInstalled(t *testing.T) {
	data, dev := pluginTestEnv(t)
	pluginTestPutManifest(t, filepath.Join(data, "dup"), `{"id":"dup","name":"Dup","version":"1.0.0"}`)
	pluginTestPutManifest(t, filepath.Join(dev, "dup"), `{"id":"dup","name":"Dup","version":"2.0.0"}`)
	rows := pluginListRows()
	if len(rows) != 1 || rows[0].Source != "dev" || rows[0].Version != "2.0.0" {
		t.Fatalf("dev override must win: %+v", rows)
	}
}
