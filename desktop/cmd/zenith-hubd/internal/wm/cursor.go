package wm

// The cursor store may name a role instead of a concrete theme. "DYNAMIC" is the
// Hub's "Follow the wallpaper" pick in Ryoku, where a package recolours a pointer
// set from the palette. Zenith ships no such set and is monochrome by design, so
// the sentinel is still understood (an imported rice may carry it) and resolves
// to the default theme rather than reaching the compositor as a name that does
// not exist on disk.

// CursorThemeDynamic is the stored sentinel for the wallpaper-following pick.
const CursorThemeDynamic = "DYNAMIC"

// CursorThemeDefault is the theme a fresh install uses and the one the sentinel
// resolves to. Adwaita is on every Ubuntu desktop image.
const CursorThemeDefault = "Adwaita"

// ResolveCursorTheme maps the stored theme onto one installed on disk. Anything
// that is not the sentinel passes through untouched.
func ResolveCursorTheme(theme string) string {
	if theme == CursorThemeDynamic {
		return CursorThemeDefault
	}
	return theme
}
