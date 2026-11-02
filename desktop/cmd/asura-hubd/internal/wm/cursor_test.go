package wm

import "testing"

func TestResolveCursorTheme(t *testing.T) {
	cases := map[string]string{
		CursorThemeDynamic: CursorThemeDefault,
		"DMZ-White":        "DMZ-White",
		"Adwaita":          "Adwaita",
		"":                 "",
	}
	for in, want := range cases {
		if got := ResolveCursorTheme(in); got != want {
			t.Errorf("ResolveCursorTheme(%q) = %q, want %q", in, got, want)
		}
	}
}
