package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// iconsTestTouch creates an empty file, making its parent directories.
func iconsTestTouch(t *testing.T, path string) {
	t.Helper()
	appsTestWrite(t, path, "")
}

// iconsTestTheme writes a theme's index.theme with an Inherits= line.
func iconsTestTheme(t *testing.T, root, theme, inherits string) {
	t.Helper()
	body := "[Icon Theme]\nName=" + theme + "\n"
	if inherits != "" {
		body += "Inherits=" + inherits + "\n"
	}
	appsTestWrite(t, filepath.Join(root, theme, "index.theme"), body)
}

// TestParseIconSizeDir reads the leading integer and doubles it for @2x.
func TestParseIconSizeDir(t *testing.T) {
	cases := map[string]int{
		"48":       48,
		"48x48":    48,
		"256x256":  256,
		"512x512":  512,
		"48x48@2x": 96,
		"32@2x":    64,
		"scalable": 0,
		"symbolic": 0,
		"apps":     0,
		"":         0,
	}
	for in, want := range cases {
		if got := parseIconSizeDir(in); got != want {
			t.Errorf("parseIconSizeDir(%q) = %d, want %d", in, got, want)
		}
	}
}

// TestParseIconInherits pulls the Inherits list out of the [Icon Theme]
// section only: the per-directory sections that follow it can carry keys of
// their own.
func TestParseIconInherits(t *testing.T) {
	body := []byte("[Icon Theme]\nName=X\nInherits=a,b, c\n\n[48x48/apps]\nSize=48\nInherits=ignored\n")
	if got, want := parseIconInherits(body), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseIconInherits = %v, want %v", got, want)
	}
}

// TestIconThemeChain pins the order: the theme, its parents depth-first and
// deduplicated, and hicolor last, never duplicated when a theme names it.
func TestIconThemeChain(t *testing.T) {
	root := t.TempDir()
	iconsTestTheme(t, root, "themeA", "themeB,themeC")
	iconsTestTheme(t, root, "themeB", "themeC")
	iconsTestTheme(t, root, "themeC", "hicolor")

	got := iconThemeChain("themeA", []string{root})
	want := []string{"themeA", "themeB", "themeC", "hicolor"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("chain = %v, want %v", got, want)
	}
}

// TestIconThemeChainFallsBackToAdwaita pins the fallback the brief names: a
// theme that does not carry an icon falls through to Adwaita before hicolor,
// when Adwaita is installed, and an empty or unknown theme still resolves.
func TestIconThemeChainFallsBackToAdwaita(t *testing.T) {
	root := t.TempDir()
	iconsTestTheme(t, root, "Papirus", "breeze")
	iconsTestTheme(t, root, "Adwaita", "hicolor")

	if got, want := iconThemeChain("Papirus", []string{root}), []string{"Papirus", "breeze", "Adwaita", "hicolor"}; !reflect.DeepEqual(got, want) {
		t.Errorf("chain = %v, want %v", got, want)
	}
	if got, want := iconThemeChain("", []string{root}), []string{"Adwaita", "hicolor"}; !reflect.DeepEqual(got, want) {
		t.Errorf("empty theme chain = %v, want %v", got, want)
	}
	if got, want := iconThemeChain("Adwaita", []string{root}), []string{"Adwaita", "hicolor"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Adwaita chain = %v, want %v", got, want)
	}

	bare := t.TempDir()
	if got, want := iconThemeChain("Nowhere", []string{bare}), []string{"Nowhere", "hicolor"}; !reflect.DeepEqual(got, want) {
		t.Errorf("uninstalled-theme chain = %v, want %v", got, want)
	}
}

// TestSizePreferenceWithinTheme pins that within one theme a scalable SVG beats
// the largest sized raster, which beats a symbolic variant, and that among
// sized directories the largest wins with @2x counting double.
func TestSizePreferenceWithinTheme(t *testing.T) {
	root := t.TempDir()
	iconsTestTheme(t, root, "theme", "")
	base := filepath.Join(root, "theme")

	iconsTestTouch(t, filepath.Join(base, "16x16", "apps", "pref.png"))
	iconsTestTouch(t, filepath.Join(base, "48x48", "apps", "pref.png"))
	iconsTestTouch(t, filepath.Join(base, "scalable", "apps", "pref.svg"))
	iconsTestTouch(t, filepath.Join(base, "symbolic", "apps", "pref.svg"))

	iconsTestTouch(t, filepath.Join(base, "64x64", "apps", "raster.png"))
	iconsTestTouch(t, filepath.Join(base, "48x48@2x", "apps", "raster.png"))

	index := buildIconIndexFrom("theme", []string{root}, nil)
	if got := index.Icons["pref"]; got != filepath.Join(base, "scalable", "apps", "pref.svg") {
		t.Errorf("pref resolved to %q, want the scalable svg", got)
	}
	if got := index.Icons["raster"]; got != filepath.Join(base, "48x48@2x", "apps", "raster.png") {
		t.Errorf("raster resolved to %q, want the 48x48@2x png (96)", got)
	}
}

// TestRasterSizePreferenceStopsAtTheTarget pins that the index does not pick a
// 1024px PNG to draw a small icon: from the target size up the smallest wins,
// below it the largest.
func TestRasterSizePreferenceStopsAtTheTarget(t *testing.T) {
	root := t.TempDir()
	iconsTestTheme(t, root, "theme", "")
	base := filepath.Join(root, "theme")

	for _, size := range []string{"64x64", "256x256", "256x256@2x", "1024x1024"} {
		iconsTestTouch(t, filepath.Join(base, size, "apps", "big.png"))
	}
	for _, size := range []string{"48x48", "128x128"} {
		iconsTestTouch(t, filepath.Join(base, size, "apps", "mid.png"))
	}
	for _, size := range []string{"512x512", "1024x1024"} {
		iconsTestTouch(t, filepath.Join(base, size, "apps", "huge.png"))
	}

	index := buildIconIndexFrom("theme", []string{root}, nil)
	for name, want := range map[string]string{
		"big":  "256x256",
		"mid":  "128x128",
		"huge": "512x512",
	} {
		if got := index.Icons[name]; got != filepath.Join(base, want, "apps", name+".png") {
			t.Errorf("%s resolved to %q, want the %s png", name, got, want)
		}
	}
}

// TestFirstThemeWins pins that the first theme in the chain that has a name
// keeps it, even when a later theme has a nominally better candidate.
func TestFirstThemeWins(t *testing.T) {
	root := t.TempDir()
	iconsTestTheme(t, root, "themeA", "themeB")
	iconsTestTheme(t, root, "themeB", "")
	iconsTestTouch(t, filepath.Join(root, "themeA", "16x16", "apps", "dup.png"))
	iconsTestTouch(t, filepath.Join(root, "themeB", "scalable", "apps", "dup.svg"))

	index := buildIconIndexFrom("themeA", []string{root}, nil)
	if got := index.Icons["dup"]; got != filepath.Join(root, "themeA", "16x16", "apps", "dup.png") {
		t.Errorf("dup resolved to %q, want themeA's raster", got)
	}
}

// TestEarlierRootWins pins that the user's copy beats the system copy at the
// same score.
func TestEarlierRootWins(t *testing.T) {
	userRoot := t.TempDir()
	systemRoot := t.TempDir()
	iconsTestTheme(t, userRoot, "theme", "")
	iconsTestTheme(t, systemRoot, "theme", "")
	iconsTestTouch(t, filepath.Join(userRoot, "theme", "scalable", "apps", "app.svg"))
	iconsTestTouch(t, filepath.Join(systemRoot, "theme", "scalable", "apps", "app.svg"))

	index := buildIconIndexFrom("theme", []string{userRoot, systemRoot}, nil)
	if got := index.Icons["app"]; got != filepath.Join(userRoot, "theme", "scalable", "apps", "app.svg") {
		t.Errorf("app resolved to %q, want the user root copy", got)
	}
}

// TestPixmapsFallback pins that a name found nowhere in the theme chain falls
// back to a pixmaps directory, svg over png over xpm, and that a themed name is
// never overridden by it.
func TestPixmapsFallback(t *testing.T) {
	root := t.TempDir()
	pixmaps := t.TempDir()
	iconsTestTheme(t, root, "theme", "")
	iconsTestTouch(t, filepath.Join(root, "theme", "scalable", "apps", "themed.svg"))
	iconsTestTouch(t, filepath.Join(pixmaps, "themed.png"))
	iconsTestTouch(t, filepath.Join(pixmaps, "only.png"))
	iconsTestTouch(t, filepath.Join(pixmaps, "only.svg"))
	iconsTestTouch(t, filepath.Join(pixmaps, "only.xpm"))

	index := buildIconIndexFrom("theme", []string{root}, []string{pixmaps})
	if got := index.Icons["themed"]; got != filepath.Join(root, "theme", "scalable", "apps", "themed.svg") {
		t.Errorf("themed resolved to %q, want the themed svg", got)
	}
	if got := index.Icons["only"]; got != filepath.Join(pixmaps, "only.svg") {
		t.Errorf("only resolved to %q, want the pixmaps svg", got)
	}
}

// TestSymbolicNaming pins that a symbolic variant is indexed under its own
// -symbolic stem and loses to a plain icon of the same base name.
func TestSymbolicNaming(t *testing.T) {
	root := t.TempDir()
	iconsTestTheme(t, root, "theme", "")
	base := filepath.Join(root, "theme")
	iconsTestTouch(t, filepath.Join(base, "scalable", "apps", "go-home.svg"))
	iconsTestTouch(t, filepath.Join(base, "scalable", "apps", "go-home-symbolic.svg"))

	index := buildIconIndexFrom("theme", []string{root}, nil)
	if got := index.Icons["go-home"]; got != filepath.Join(base, "scalable", "apps", "go-home.svg") {
		t.Errorf("go-home resolved to %q, want the plain svg", got)
	}
	if _, ok := index.Icons["go-home-symbolic"]; !ok {
		t.Error("go-home-symbolic was not indexed under its own stem")
	}
}

// TestLinkedSizeTreesAreFollowed pins the Papirus layout: whole size
// directories that are links into a sibling theme.
func TestLinkedSizeTreesAreFollowed(t *testing.T) {
	root := t.TempDir()
	iconsTestTheme(t, root, "Dark", "")
	iconsTestTouch(t, filepath.Join(root, "Base", "128x128", "apps", "linked.png"))
	if err := os.MkdirAll(filepath.Join(root, "Dark"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "Base", "128x128"), filepath.Join(root, "Dark", "128x128")); err != nil {
		t.Fatal(err)
	}

	index := buildIconIndexFrom("Dark", []string{root}, nil)
	if got := index.Icons["linked"]; got == "" {
		t.Error("an icon behind a linked size directory was not found")
	}
}

// TestResolveIconName passes an existing absolute path through, drops a
// missing one, and otherwise looks the name up in the index.
func TestResolveIconName(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.svg")
	iconsTestTouch(t, real)
	index := iconIndex{Icons: map[string]string{"kitty": "/usr/share/icons/hicolor/scalable/apps/kitty.svg"}}

	if got := resolveIconName(real, index); got != real {
		t.Errorf("absolute existing path = %q, want %q", got, real)
	}
	if got := resolveIconName("/no/such/icon.svg", index); got != "" {
		t.Errorf("absolute missing path = %q, want empty", got)
	}
	if got := resolveIconName("kitty", index); got != "/usr/share/icons/hicolor/scalable/apps/kitty.svg" {
		t.Errorf("kitty = %q, want the indexed path", got)
	}
	if got := resolveIconName("nope", index); got != "" {
		t.Errorf("unknown name = %q, want empty", got)
	}
	if got := resolveIconName("", index); got != "" {
		t.Errorf("empty name = %q, want empty", got)
	}
}

// TestGtkSettingsIconTheme reads the theme a session without GNOME's schemas
// keeps in settings.ini, GTK 4's file separately from GTK 3's.
func TestGtkSettingsIconTheme(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	appsTestWrite(t, filepath.Join(config, "gtk-3.0", "settings.ini"),
		"[Settings]\ngtk-application-prefer-dark-theme=0\ngtk-icon-theme-name = 'Papirus-Dark'\n")
	appsTestWrite(t, filepath.Join(config, "gtk-4.0", "settings.ini"),
		"[Settings]\ngtk-icon-theme-name=Yaru\n")

	if got := gtkSettingsIconTheme("gtk-3.0"); got != "Papirus-Dark" {
		t.Errorf("gtk-3.0 theme = %q, want Papirus-Dark", got)
	}
	if got := gtkSettingsIconTheme("gtk-4.0"); got != "Yaru" {
		t.Errorf("gtk-4.0 theme = %q, want Yaru", got)
	}
	if got := gtkSettingsIconTheme("gtk-9.0"); got != "" {
		t.Errorf("a missing file gave %q, want empty", got)
	}
}

// TestIconReferenceKeepsNamesAndPathsApart pins the three kinds of icon.
//
// A desktop entry names its icon either by name in the theme or by path to a
// file, and the shell needs the two in different forms: a name stays a name
// when nothing has it, and a path becomes a URL. A path handed over as a path
// is a path that nothing can open, and the icon is missing with nothing to say
// why.
//
// The icon directory below is one this test writes and points the lookup at
// through XDG_DATA_HOME, rather than the machine's own.
func TestIconReferenceKeepsNamesAndPathsApart(t *testing.T) {
	home, _ := appsTestSession(t)
	installed := filepath.Join(home, "icons", "hicolor", "scalable", "status", "multimedia-volume-control.svg")
	appsTestWrite(t, installed, "<svg xmlns='http://www.w3.org/2000/svg'/>\n")
	appsTestWrite(t, filepath.Join(home, "icons", "hicolor", "48x48", "apps", "stemmed.png"), "")

	icons := &iconResolver{}
	if got, want := icons.reference("multimedia-volume-control"), "file://"+installed; got != want {
		t.Errorf("a themed name became %q, want %q", got, want)
	}
	if got := icons.reference("stemmed.png"); !strings.HasSuffix(got, "/stemmed.png") || !strings.HasPrefix(got, "file://") {
		t.Errorf("a name written with an extension became %q, want the themed file", got)
	}
	if got := icons.reference("definitely-not-an-icon-anywhere"); got != "definitely-not-an-icon-anywhere" {
		t.Errorf("a name that is not installed became %q", got)
	}
	if got := icons.reference("/opt/thing/icon.png"); got != "file:///opt/thing/icon.png" {
		t.Errorf("a path became %q", got)
	}
	if got := icons.reference(""); got != "" {
		t.Errorf("no icon became %q", got)
	}
}

// TestZenithIconsPrintsTheIndex runs the command against a theme this test
// writes and reads what it prints, as the shell does.
func TestZenithIconsPrintsTheIndex(t *testing.T) {
	home, _ := appsTestSession(t)
	appsTestWrite(t, filepath.Join(home, "icons", "hicolor", "index.theme"), "[Icon Theme]\nName=hicolor\n")
	svg := filepath.Join(home, "icons", "hicolor", "scalable", "apps", "demo.svg")
	appsTestWrite(t, svg, "<svg/>")

	out := iconsTestCapture(t, func() int { return zenithIcons(nil) })
	if !strings.Contains(out, `"demo":"`+svg+`"`) || !strings.Contains(out, `"hicolor"`) {
		t.Errorf("the index does not hold the icon: %.200s", out)
	}

	out = iconsTestCapture(t, func() int { return zenithIcons([]string{"demo", "nothing"}) })
	if want := svg + "\n\n"; out != want {
		t.Errorf("spot check printed %q, want %q", out, want)
	}
}

// iconsTestCapture runs fn with standard output captured.
func iconsTestCapture(t *testing.T, fn func() int) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = writer
	code := fn()
	os.Stdout = saved
	writer.Close()
	out, _ := io.ReadAll(reader)
	if code != exitOK {
		t.Fatalf("command exited %d", code)
	}
	return string(out)
}
