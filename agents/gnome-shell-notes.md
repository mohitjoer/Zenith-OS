# GNOME Shell notes — archived reference

**None of this applies to Ken.** Ken replaces GNOME Shell with Hyprland and its own
Quickshell-drawn shell, so there is no `Main.panel`, no `St`, no `Clutter`, no unlock
dialog and no GDM greeter to theme.

This is kept for two reasons: it explains why the project moved away from GNOME Shell, and
if that decision is ever revisited, this is knowledge that took real time to acquire and
would have to be re-earned. It was written while this repository was the Zeo GNOME Extensions
project, where the extensions shipped for GNOME Shell 45 through 50.

---

## Shell extension loading

- **GNOME Shell scans `~/.local/share/gnome-shell/extensions` once at session start and
  never re-scans.** A newly linked extension is invisible to the running shell, so
  `gnome-extensions enable` fails with "does not exist" — and, critically, it does *not*
  write the uuid to `org.gnome.shell enabled-extensions`. Append the uuid to that key
  directly, or the extension loads disabled after the next login and needs a manual toggle.
- On **Wayland a logout is the only reload**. `Alt+F2` + `r` is X11-only.
  `ReloadExtension` on `org.gnome.Shell.Extensions` is a deprecated no-op, and
  `org.gnome.Shell.Eval` is **blocked on Ubuntu builds** regardless of the
  `development-tools` setting.

## Theming the unlock screen and the greeter

- Yaru ships `!important` overrides on the unlock screen (clock `font-weight`,
  `.unlock-dialog StEntry`, `.login-dialog-button` colours). Any rule touching those needs
  `!important` too or Yaru wins on cascade. Do not "clean these up" later.
- Lock screen ink must be verified against
  `wallpaper → Shell.BlurEffect(brightness=0.65) → .screen-shield-background scrim`, not
  against raw white or black. A scrim thin enough to look right fails 4.5:1 once the
  shell's own dim is accounted for.
- GDM runs its own gnome-shell in `gdm` session mode and **ignores user themes**. On GNOME 50
  `org.gnome.shell` has no `theme` key at all, and there is nothing to point at a theme
  directory.
- The way in is `Main._getStylesheet()`: it checks the registered
  `gnome-shell/gdm-theme.gresource` first, then falls back to
  `$XDG_DATA_DIRS/gnome-shell/theme/`. Since the resource ships only `gdm.css`, dropping
  `gdm-dark.css` / `gdm-light.css` into the data dir is picked up with no gresource surgery.
  That file must contain the stock `gdm.css` with overrides appended, because
  `_getDefaultStylesheet()` returns the first hit and never consults the resource's
  `gdm.css` once a variant name resolves.
- The greeter applies **no blur or dim** to its wallpaper, so a scrim that is fine on the
  lock screen is too thin here.

## Why a theme must ship as an extension

A theme that needs to *add to* the shell theme cannot be a second entry in
`~/.local/share/themes/`. St loads one stylesheet per shell theme, and it never resolves
`@import` — libcroco is linked into libst, but St has no `@import` string.

Ship it as an extension and let `ExtensionManager` auto-load it. It picks the first of
`unlock-screen-<variant>.css`, `stylesheet-<variant>.css`, `unlock-screen.css`,
`stylesheet.css`, and reloads on `St.Settings.color-scheme` — so `stylesheet-dark.css` +
`stylesheet-light.css` switch automatically with zero JS. **Only files ending in
`-dark.css` / `-light.css` are eligible for that auto-reload.**

## St and GTK CSS traps

- GNOME Shell CSS supports **neither `@media` nor `::before`/`::after`**.
- St honours the child combinator `>` — GNOME Shell's own `gdm.css` uses it in 60 rules.
- There is **no `:has()`**, so a child combinator is the only disambiguator available when
  two elements share a class and only one should be styled.
- `AuthPrompt` and its inner input well share the class `login-dialog-prompt-layout`. To
  style only the inner one, scope with `> `.

## Dual-runtime daemons

The Zeo project shipped background daemons in both TypeScript and Python so that users with
either environment could run them (Python + PyGObject first, then Bun/tsx/Node, then
`gsettings monitor` as a last resort). Ken's equivalent is not decided yet — Ken's control
tools are Go, which may remove the need for this entirely.

## Verification commands used

```bash
# Schemas
for d in zeo-*@*; do glib-compile-schemas --strict --dry-run "$d/schemas/"; done

# St: load each stylesheet and surface parse errors
#   new St.Theme().load_stylesheet(Gio.File...)

# GTK 4: Gtk.CssProvider with a parsing-error handler — GTK reports unknown
# properties, which catches typos St silently ignores.
```