# Shell plugins

A plugin is a small widget a user drops into the desktop. You write the logic and
the views; Zenith owns how it looks, moves, sizes, and where it lives, so a plugin
always reads as a native part of the shell. When a user asks you to build a
widget, follow the rules, then run the commands in order. Zenith never runs a
plugin's code to install it.

## What a plugin ships

```
<plugin-id>/
  manifest.json        what it is, its hosts, its defaults, and its settings schema
  service/Main.qml     persistent logic and state, no UI
  content/Widget.qml   the one view, rendered at the host's density
  content/Panel.qml    optional: a bar plugin's panel (opens under the glyph)
  bin/                 optional scripts (shebang + exec bit)
  README.md            what it runs, reads, writes, and every capability
  LICENSE
  assets/preview-widget.png
```

Installed plugins live at `~/.local/share/zenith/plugins/<id>/` (receipt-owned;
never author there). The three hosts are `topbarGlyph` (a mark on the QS Bar),
`desktopWidget` (a tile on the wallpaper), and `framePopout` (a surface that
grows from a screen edge, or floats centred).

## The rules

`zenith plugin new` writes these into the plugin's `AGENTS.md`;
`zenith plugin validate` enforces the machine-checkable ones. Follow all eleven.

- **R1 Place.** One folder named after the id, authored under
  `$(xdg-user-dir DOCUMENTS)/zenith-plugins/<id>/` (fallback
  `~/zenith-plugins/<id>/`); `zenith plugin new <id>` creates it there. Never write
  into `~/.local/share/zenith/plugins/` or `~/.config/zenith/shell/`.
- **R2 Shape.** `manifest.json` at the root; `service/Main.qml` (logic, no UI);
  `content/Widget.qml`; optional `content/Panel.qml`; `README.md`; `LICENSE`;
  `assets/preview-widget.png` (a real capture); scripts only under `bin/` with a
  shebang and the exec bit. List every extra file in manifest `files`.
- **R3 Id.** Lowercase `[a-z0-9][a-z0-9-]*`, unique, not a built-in widget id.
- **R4 Imports.** Only `QtQuick*`, `Quickshell*`, `qs.PluginKit`,
  `qs.PluginKit.Singletons`, and files inside the plugin folder. Never
  `shell.*`, `qs.Ui*` internals, or a relative import that climbs out.
- **R5 Settings.** Declared in `metadata.settings`; read through
  `pluginApi.pluginSettings` behind a default; written only through
  `pluginApi.saveSetting(key, value)`. Never edit `shell.json` or `plugins.json`.
- **R6 Commands.** Every external program is in the plugin's own `bin/` or listed
  in `dependencies.commands`. No `sudo`, `doas`, `su`. Escalate only through
  `pkexec`, only on an explicit click, only when listed in
  `capabilities.privileged` (exact command strings) and explained in the README.
- **R7 Network.** Every host is listed in `capabilities.network`. No
  `curl … | sh`, no downloading and running code.
- **R8 Files.** Write only under `pluginApi.stateDir`
  (`$XDG_STATE_HOME/zenith/plugins/<id>`), `$XDG_CACHE_HOME/zenith/plugins/<id>`,
  or a temp dir. Never touch `~/.ssh`, `/etc`, shell rc files, or another
  plugin's folder.
- **R9 Shell.** No `sh -c` with a string built from settings or program output;
  pass argv arrays.
- **R10 Secrets and binaries.** No tokens, keys or credentials in the tree; no
  compiled binaries (ELF/Mach-O/`.so`) and no symlinks; scripts only.
- **R11 Honesty.** `official` is never `true` for a community plugin; `author` is
  `Name <mail>`; the README states what runs, reads, writes, and every
  privileged or network capability.

## The commands, in order

```
zenith plugin new <id> [--bar|--desktop|--popout] [--name N] [--author "N <m>"] [--to <dir>]
zenith plugin validate <dir> [--json] [--allow <rule>,...]
zenith plugin add <dir> --bar --yes
zenith plugin share <id>
```

1. **Scaffold.** `zenith plugin new <id>` writes R1's folder from a working
   template. `--bar` is the default and also adds `entryPoints.panel` and
   `content/Panel.qml`. It writes the manifest, `service/Main.qml`,
   `content/Widget.qml`, `README.md`, `LICENSE`, an `AGENTS.md` with these rules,
   and a first git commit as the author.
2. **Edit.** Fill in `service/Main.qml`, `content/Widget.qml`,
   `content/Panel.qml`, `manifest.json`, and `README.md`; capture
   `assets/preview-widget.png`.
3. **Validate.** `zenith plugin validate <dir>` runs the manifest checks and the
   static audit, printing `rule-id  path:line  message`. Clear every blocking
   finding (see Security). The template validates clean.
4. **Install.** `zenith plugin add <dir> --bar --yes` validates again, installs
   through the store's transaction, and enables it on the bar; the shell picks it
   up on the `plugins.json` change, no restart. `add` refuses blocking findings
   unless you pass `--allow-findings`.
5. **Verify.** Confirm it on the bar and under QS Bar Settings > Community.
6. **Share.** `zenith plugin share <id>` opens the store pull request. Run it
   only when the user asks to publish (see below).

Other commands: `zenith plugin list [--json]` (installed plugins and their host /
enabled state; `--json` rows carry `capabilities`), `zenith plugin remove <id>`
(uninstall and drop placement), `zenith plugin export <id> [--to <dir>]` (a
store-shaped folder under git).

### Security: the validator's rules

Blocking (exit 1; `add` refuses without `--allow-findings`): `symlink`,
`escalation` (sudo/doas/su, or pkexec not in `capabilities.privileged`),
`pipe-shell`, `internal-import` (R4), `config-write` (shell.json / plugins.json /
~/.ssh / /etc / rc files), `secret`, `binary`. Warnings (printed, exit 0):
`undeclared-command`, `undeclared-host`, `dynamic-shell` (R9), `outside-write`
(R8), `large-tree` (> 20 MB). `--allow <rule>` downgrades one blocking rule to a
warning for the run.

## What the host sets

Declare these on your roots and read them; never assign them.

`content/Widget.qml` on the bar (host `topbarGlyph`):

| Property | Value |
| --- | --- |
| `density` | `"glyph"` |
| `widthBudget` | `220` (cap any text you draw to it) |
| `s` | scale multiplier; multiply sizes and font sizes by it |
| `active` | `true` while mounted |
| `screen` | the monitor this copy is on |
| `pluginApi` | your handle (below) |

`content/Panel.qml` (a bar plugin's panel, `entryPoints.panel`):

| Property | Value |
| --- | --- |
| `density` | `"full"` |
| `widthBudget` | manifest `panel.width` (default `320`, clamped `240..520`) |
| `s` | scale multiplier |
| `active` | `true` while the panel is open |
| `pluginApi` | your handle (below) |

The panel reports its `implicitHeight`; the host sizes the card to it, capped to
the screen. Widget.qml's left click opens it: `onClicked: pluginApi.togglePanel()`.
No click on the bar ever mutates anything.

The `pluginApi` every entry point receives:

| Member | What it is |
| --- | --- |
| `mainInstance` | your live `service/Main.qml` instance |
| `pluginSettings` | the resolved settings; read each key behind its default |
| `pluginDir` | the installed plugin folder (build `bin/` paths from it) |
| `stateDir` | `$XDG_STATE_HOME/zenith/plugins/<id>`; the only place you write (R8) |
| `saveSetting(key, value)` | persist one setting; `pluginSettings` re-derives (R5) |
| `panelOpen` / `openPanel()` / `closePanel()` / `togglePanel()` | the bar panel |

## Settings types

Declare user options as a `metadata.settings` schema; the bar renders native
controls and persists changes to `~/.config/zenith/plugins.json`. Each entry has a
`key`, `type`, `label`, `group`, and `default`. The types the bar renders:

- `toggle` - a boolean switch.
- `choice` - one of a set; give `options: [{ "value", "label" }, ...]`.
- `multi` - several of a set; same `options`.
- `int` - a stepper; give `min` and `max`.
- `text` - a line of text; optional `placeholder`.

`slider` and `image` are desktop-widget menu types and do not render on the bar,
so a bar widget picks from the five above. Read a value guarded, e.g.
`pluginApi.pluginSettings.poll ?? 10`, and write it only through
`pluginApi.saveSetting`.

## Never share unless asked

`zenith plugin share <id>` opens a public pull request against the store. Do not
run it unless the user explicitly asks to publish. Scaffolding, editing,
validating, and `zenith plugin add`ing a widget onto the user's own bar is the
whole job; sharing is a separate, opt-in step.

## A worked example

The user asks: "make me a bar widget that shows my VPN status." The exact
commands:

```bash
zenith plugin new vpn --bar --name "VPN" --author "Nero <nero@example.com>"
# edit ~/Documents/zenith-plugins/vpn/service/Main.qml, content/Widget.qml,
#      content/Panel.qml, manifest.json, README.md; capture the preview png
zenith plugin validate ~/Documents/zenith-plugins/vpn
zenith plugin add ~/Documents/zenith-plugins/vpn --bar --yes
```

Then tell the user it is on the bar and under QS Bar Settings > Community, and
stop. Do not run `zenith plugin share vpn` unless they ask to publish. The VPN
plugin already in the store is the reference safe bar plugin: a glyph that only
reads state, a panel that acts on an explicit click, `pkexec` declared in
`capabilities.privileged`, and `login.tailscale.com` declared in
`capabilities.network`.

## The store

The store is the curated distribution front for plugins, bar styles, and rices;
community plugins (not `"official": true`) list under a warning that Zenith does
not review or maintain them. Git and a local folder remain the open door for
anything not in the store. `zenith plugin share <id>` is how a widget made here
gets listed there. See `docs/plugins.md` in the Zenith source for the full
authoring contract.
