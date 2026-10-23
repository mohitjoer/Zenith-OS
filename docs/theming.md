# Theming

How a wallpaper becomes colours, which file carries them, and who writes it. The generator is
Zenith's own, in `desktop/internal/theme`: there is no matugen on Ubuntu, so `zenith theme` makes
the scheme matugen would and fills the templates itself.

## The pipeline

```
wallpaper
  -> zenith theme generate --json      the roles, in the JSON shape matugen prints
  -> the daemon writes colors.json     what the shell draws with
                       tones.json      what Ink.qml picks tones from
  -> the daemon writes a carrier and runs  zenith theme render --config CONFIG --palette CARRIER
                                       what the apps get
```

The daemon (`zenith-shell`) runs this on every wallpaper change, in matugen's order, and swaps the
`matugen` executable for the two `zenith theme` calls. `zenith theme generate IMAGE` with no
`--json` does the middle step by hand: it writes `colors.json` and `tones.json` and nothing else.

## The on-disk contract

| File | Keys | Written by | Read by |
|---|---|---|---|
| `~/.cache/zenith/colors.json` | one flat map of `#rrggbb` strings: `background`, `foreground`, `cursor`, `color0` to `color15`, and 32 camelCase Material roles (`surface`, `onSurface`, `primaryContainer`, `outlineVariant`, `shadow`, `scrim`, ...) | the daemon; `zenith theme generate IMAGE` by hand | `Services/Scheme.qml`, `Ui/Singletons/Tokens.qml`, `hub/Singletons/Scheme.qml` |
| `~/.cache/zenith/tones.json` | `{ramp: {tone: hex}}` for `primary`, `secondary`, `tertiary`, `neutral`, `neutralVariant`, `error`; tones 0 to 100 in 18 steps | the same | `Ui/Singletons/Ink.qml` |
| `~/.config/zenith/theme.json` | `followWallpaper` (default true), `themeApps` | the Hub | the singletons above; the daemon |
| `~/.config/zenith/shell.json` | `themePalette`, the fixed named scheme that outranks the wallpaper | the daemon | the singletons above |

`colors.json` is a flat string map on purpose: the shell parses it from raw text and the daemon's
tests unmarshal it into `map[string]string`. Nothing else goes in it. The old Zenith shape
(`background`, `surface`, `text`, `accent`, ...) is gone; nothing reads it.

## Where hue is allowed

The wallpaper drives tone: dark or light, contrast, how far the surfaces lift. It drives no hue
unless a person picks a coloured scheme. That follows the design system (black, grey and white; the
only colours are the traffic lights and error) and the roadmap's line that the token set is
canonical.

- The default scheme is `monochrome`: every role is a neutral grey except the `error` roles, which
  are red in every scheme.
- Any other scheme is an explicit choice, and its hue goes everywhere the palette goes: shell,
  terminal, GTK. There is one palette and no split between "our" surfaces and the apps.
- A grey wallpaper has no hue to take, so no scheme invents one.

The daemon and the Hub pass `--scheme` from `matugen.json`, so the default only holds if their
shipped `schemeType` is `scheme-monochrome`; the upstream default is `scheme-tonal-spot`.

## Generating

```
zenith theme generate [--image PATH | IMAGE] [--mode dark|light|smart] [--scheme NAME]
    [--contrast N] [--lightness-dark N] [--lightness-light N] [--source-color-index N] [--json]
```

`--scheme` takes the nine names the Hub lists, with or without `scheme-`. `--mode smart` follows
the image's brightness. `--prefer` and `--dry-run` are accepted and ignored: matugen ignores
`--prefer` when an index is given and the daemon always gives one, and `--json` writes nothing.

With `--json` the output is `{"colors": {ROLE: {"dark", "light", "default": {"color": "#rrggbb"}}},
"palettes": {RAMP: {TONE: {"color": ...}}}, "mode", "is_dark_mode", "scheme"}`: 49 Material roles
plus `source_color`, and the six ramps. `default` is the requested mode. The same image and flags
give the same bytes.

This is not matugen's arithmetic. Roles are built in CIE LCh instead of CAM16, and a tone is L*, so
the tone distance between a role and its surface, which is what fixes the contrast, is Material's
own. Hues and chromas land close to matugen's, not on them. The seed is the existing extractor's
accent (a 5-bit histogram scored by saturation and mid-tone), one candidate per hue, and
`--source-color-index` takes the nth. Vibrant and expressive use one hue rotation where matugen
reads a table, and fidelity and content approximate the complement and the analogous hue.

PNG, JPEG and GIF decode in the standard library. Other formats (webp, avif) go through `ffmpeg`
when it is installed and fail with a message saying so when it is not.

## Rendering

`zenith theme render --config CONFIG --palette CARRIER` (matugen's `-c CONFIG [json] CARRIER` works
too). CONFIG holds `[templates.NAME]` blocks with `input_path` and `output_path`; `~/` is the home directory. A
template fills `{{ colors.NAME.BUCKET.FIELD }}` from the carrier, the JSON the daemon builds
(`{"colors": {NAME: {"default": {"hex": ...}, "dark": ..., "light": ...}}}`), and understands one
filter, `| replace: "#", ""`. A colour that does not resolve fails that template and the others
still render; the exit code is 1 if any failed.

Templates are read from `~/.config/zenith/shell/matugen/` (the materialised copy, where a user edit
wins) and from `/usr/share/zenith/shell/matugen/` before `zenith materialize` has run.

A render writes only into an app's own per-user directory, and never over something that is not
its own:

- Under `~/.config/zenith`, a file with a mirror under `user_edits/` is left alone.
- Under `~/.cache/zenith`, the file is replaced.
- Anywhere else, an existing file is replaced only if its first 512 bytes still carry
  `Rendered by zenith theme`; a symlink is never written through. Every shipped template has the
  line, so keep it in one you edit.

Each block prints `wrote`, `unchanged`, `left alone: <why>` or `error: <why>`.

## The templates

| Block | File | Reads it |
|---|---|---|
| `ghostty` | `~/.config/zenith/apps/ghostty/colors` | `apps/ghostty/config`, with `config-file = colors` |
| `btop` | `~/.config/btop/themes/zenith.theme` | btop, with `color_theme = "zenith"` |
| `qt6ct` | `~/.config/qt6ct/colors/zenith.conf` | qt6ct, with `color_scheme_path` set to this file |
| `gtk3`, `gtk4` | `~/.config/gtk-3.0/gtk.css`, `~/.config/gtk-4.0/gtk.css` | GTK apps, while "Theme apps" is on |

`config.toml` holds the first three and always renders; `apps.toml` holds the GTK pair and the
daemon filters it by the roster. The shipped `apps/ghostty/colors` is a static default; a render
replaces it, and `zenith materialize` puts the static one back until the next render.

Dropped, because Zenith does not ship the app: kitty, fish, yazi, KDE, cava, zathura, tmux, micro,
alacritty, OBS, Zed, Discord (vesktop, equibop), Steam, Telegram, Heroic, Obsidian, Sidra and
Ryotunes. Papirus folders are gone too (the package is excluded, `papirus-folders` is not in the
archive). Two more blocks of the upstream set have no reader here: the Hyprland border file (the daemon recolours the
live border from `colors.json`) and the screen-share picker's stylesheet (no picker in the archive).
The Hub's roster keys for the dropped apps now do nothing.

`adw-gtk3` is not in the archive, so on Adwaita or Yaru the GTK 3 sheet takes the palette only in
part: those themes hardcode some of their colours.
