# Writing a Zenith shell plugin

A plugin is a small widget you write that the **user** drops into the Zenith
desktop. You write the *content and the logic*. **Zenith owns everything about how
it looks, moves, sizes, and where it lives.** That split is the whole point: a
plugin always looks and behaves like a native part of the shell, because the
shell - not you - draws the surface around it.

If you only read one thing, read **"Who does what"** below.

This is about **shell** plugins, widgets that run in Quickshell. A **Hyprland
compositor** plugin (a `.so` the compositor loads: title bars, cursor motion,
key sounds) is a different thing, managed on Settings > Plugins; see
`docs/hyprland-plugins.md`.

---

## The rules

Eleven rules make a plugin safe to install and native to the shell.
`zenith plugin new` writes them into the plugin's `AGENTS.md`, and
`zenith plugin audit` enforces the ones a machine can check.

- **R1 Place.** A plugin is one folder named after its id, authored under
  `$(xdg-user-dir DOCUMENTS)/zenith-plugins/<id>/` (fallback
  `~/zenith-plugins/<id>/`); `zenith plugin new <id>` creates it there. Never write
  into `~/.local/share/zenith/plugins/` (the install root, which `zenith plugin
  install` fills) or `/usr/share/zenith/` (the shipped shell). *Why:* those trees
  belong to the installer and the package, so a stray file there is either
  loaded unaudited or replaced on the next update.
- **R2 Shape.** `manifest.json` at the root; `service/Main.qml` (logic, no UI);
  `content/Widget.qml` (the one view); optional `content/Panel.qml` (a bar
  panel); `README.md`; `LICENSE`; `assets/preview-widget.png` (a real capture);
  scripts only under `bin/` with a shebang and the exec bit. Every extra file is
  listed in manifest `files`. *Why:* the list is what a reader, and any catalogue
  that packages the plugin, takes the plugin to consist of.
- **R3 Id.** Lowercase `[a-z0-9][a-z0-9-]*`, unique, and not a built-in widget
  id. *Why:* the id is the folder name, the layout key and the settings
  namespace, so a collision shadows a built-in or another plugin.
- **R4 Imports.** Only `QtQuick*`, `Quickshell*`, `qs.PluginKit`,
  `qs.PluginKit.Singletons`, and files inside the plugin folder. Never
  `shell.*`, any other `qs.*` module, or a relative import that climbs out of the
  folder. *Why:* the kit is the only stable surface, so reaching into shell
  internals breaks on every update and escapes the folder.
- **R5 Settings.** Declared in `metadata.settings` (the types the bar renders:
  `toggle`, `choice`, `multi`, `int`, `text`); read through
  `pluginApi.pluginSettings` behind a default; written only through
  `pluginApi.saveSetting(key, value)`. Never edit `shell.json` or `plugins.json`
  directly. *Why:* the shell renders and persists settings for you, and
  hand-editing those files corrupts state it owns.
- **R6 Commands.** Every external program is in the plugin's own `bin/` or listed
  in `dependencies.commands`. No `sudo`, `doas`, `su`. A privileged action is
  allowed only through `pkexec`, only on an explicit click, only when listed in
  manifest `capabilities.privileged` (exact command strings) and explained in the
  README. *Why:* a reader and the validator can then see every binary the plugin
  runs and every escalation it can ask for.
- **R7 Network.** Every host the plugin talks to is listed in
  `capabilities.network`. No `curl … | sh`, no downloading and running code.
  *Why:* the manifest declares the whole network reach up front, and
  fetch-and-run hides code from review.
- **R8 Files.** Write only under `pluginApi.stateDir`
  (`$XDG_STATE_HOME/zenith/plugins/<id>`), `$XDG_CACHE_HOME/zenith/plugins/<id>`,
  or a temp dir. Never touch `~/.ssh`, `/etc`, shell rc files, or another
  plugin's folder. *Why:* a widget has no business outside its own state, and
  those paths are where damage and data theft happen.
- **R9 Shell.** No `sh -c` with a string built from settings or program output;
  pass argv arrays. *Why:* a value spliced into a shell string is a command
  injection hole.
- **R10 Secrets and binaries.** No tokens, keys or credentials in the tree; no
  compiled binaries (ELF/Mach-O/`.so`) and no symlinks; scripts only. *Why:* the
  tree is public and readable, so a secret leaks and a binary cannot be reviewed.
- **R11 Honesty.** `official` is never `true` for a community plugin; `author` is
  `Name <mail>`; the README says what runs, what it reads, what it writes, and
  every privileged or network capability. *Why:* users decide to trust an
  unsandboxed widget from the manifest and README, so both must be truthful.

---

## The workflow

Start to finish, a plugin is one command to scaffold, an edit loop, one command
to check, and one to install:

1. **Scaffold.** `zenith plugin new <id> [--bar|--desktop|--popout]` writes R1's
   folder from a working template (default `--bar`, which also adds
   `entryPoints.panel` and `content/Panel.qml`): the manifest, `service/Main.qml`,
   `content/Widget.qml`, `README.md`, `LICENSE`, an `AGENTS.md` carrying these
   rules, and a first git commit as the author.
2. **Edit.** Fill in the service, the view, the panel, the manifest and the
   README, and capture `assets/preview-widget.png`.
3. **Audit.** `zenith plugin audit <dir>` runs the manifest checks and the
   static audit. Clear every blocking finding (see [Security](#security)); the
   template audits clean, so keep it that way.
4. **Install.** `zenith plugin install <dir> --bar --yes` audits again, copies
   the folder into the install root, and enables it on the bar; the shell picks
   it up on the `plugins.json` change, no restart.
5. **Verify.** Confirm it on the bar and on the bar's Community page.

---

## Who does what

| You (the contributor) write... | Zenith (the shell) handles for you... |
|---|---|
| The **logic**: fetch data, hold state, run commands (`service/Main.qml`). | **Where the widget lives** (frame popout, desktop widget) and letting the user choose and move it. |
| **One view** of your widget (`content/Widget.qml`) - the labels, buttons, grid, etc. | The **card / popout surface** behind your view: background, rounded corners, shadow, hairline. |
| A small **manifest** describing your plugin and its defaults. | **Dragging, resizing, the right-click menu, hover-to-open** - all the interaction. |
| A **settings schema** in your manifest (`metadata.settings`). | **Sizing**: the popout grows to fit your content; the desktop tile scales as the user resizes it. |
| Optionally, **shipped scripts/binaries** (`bin/`). | **Theming, motion, the brand look** (the "deck dialect"), so you match the shell automatically. |

**The golden rule:** never set your own position, never draw your own window
chrome, never assume your size. You declare *how big your content naturally
wants to be*; Zenith does the rest. Break this rule and your plugin will look
bolted-on instead of native.

---

## Files you ship

```
<your-plugin-id>/
  manifest.json        what your plugin is + its suggested defaults   (required)
  service/Main.qml     persistent logic and state, no UI               (required)
  content/Widget.qml   ONE view of your widget                          (required)
  bin/                 scripts or binaries your plugin runs            (optional)
  README.md            what it is + a preview.gif                      (recommended)
```

- **Installed** to `~/.local/share/zenith/plugins/<id>/`.
- **For local dev**, point `ZENITH_PLUGINS_DIR` at a folder of plugins (colon-
  separated for several): they shadow installed ones of the same id, and the
  shell discovers them live.

---

## The three hosts a user can choose

When a user enables your plugin they pick **one** of these in Zenith Settings →
Plugins. Your *same* `content/Widget.qml` is used for all three - Zenith just
renders it differently.

### 1. Desktop widget - a tile on the wallpaper

It sits on the wallpaper next to the clock and weather, and behaves **exactly
like them**, because Zenith gives it the identical machinery:

- **Left-drag** to move it anywhere (snaps to a grid).
- **Right-click** it for its menu (Lock to freeze it, Hide to turn it off).
- **Drag the bottom-right corner** to resize (scales 50%–250%, live).
- It's drawn on a **card** (rounded, translucent, soft shadow) matching the
  clock/weather.

**You do nothing to get any of that.** You just write the content; Zenith wraps it
in the draggable, resizable, right-clickable card. The only thing you owe it: a
content root that reports its natural size (see "Sizing" below), so the card can
size itself around you.

### 2. Frame popout - grows out of a screen edge, or floats at the centre

It melts out of the frame border on hover (like the volume mixer and power
menu), fused into the same blob. The user picks the **edge** (top / right /
bottom / left) and where along it the popout sits (`start`, `center`, `end`), by
dragging the popout chip around a mock of their screen in Zenith Settings.

Dropped in the **middle** of that screen instead, the popout becomes a centred
surface: it floats in the middle of the display, all four corners rounded, with
no hover edge. A centred popout opens only when it is asked for, by
`zenith-shell plugin <id>` or a click, which makes it the placement
for a modal view. It shares the middle of the screen with quick settings
(`Super+Escape`) and the stash (`Super+S`), but the shell only ever shows one
surface at a time, so they take turns rather than overlap.

- Zenith handles the **hover trigger, the open/close animation, and the fuse into
  the frame**.
- **Dimensions & Sizing**:
  - By default, edge popouts use a compact `360px` width budget and grow vertically
    to fit your content's `implicitHeight`.
  - For rich or modal views (calculators, email, system monitors), you can specify
    a custom width and height:
    - **In manifest defaults**: declare `"width"` and/or `"height"` under `defaults.framePopout`.
    - **In plugin settings**: declare `"width"` and/or `"height"` controls in `metadata.settings`.
      Zenith Settings will expose them directly to users, and the popout will resize live.
    - **Implicit fallback**: if no fixed setting is set, Zenith respects your root
      `implicitWidth` (falling back to standard 680px for centered popouts).
- **Dismissal & Lifecycle**:
  - **Edge hover popouts** auto-retract after the pointer leaves (`autoDismiss: true`).
  - **Centered modal popouts** stay open until explicitly closed via `Escape`, clicking
    outside, or toggling the shortcut (`autoDismiss: false`).
  - To override this behavior on any popout, set `"autoDismiss": true` or `false`
    in `defaults.framePopout`.

`zenith-shell plugin <id>` toggles your frame popout open (bind it to a key).

### 3. Bar glyph - rides the bar

It rides the top bar as its own module, in a section you choose (left, centre or
right) and at the position in that lane you drag it to. Your content is rendered
at `glyph` density on the bar's axis, so it must stay small: report a mark-sized
`implicitWidth`/`implicitHeight` (roughly the bar's inner height) and Zenith
centres it and reserves exactly that much room; in islands form it gets its own
island.

- Zenith handles the **island pill, spacing, and the bar's width arithmetic**.
- A bar glyph **is a layout entry**, the same kind of entry as a built-in widget:
  it lives in `shell.json` `qsbar.layout`, so a user drags it between lanes in QS
  Bar Settings' Layout route or moves it from a terminal with `zenith-shell bar
  move <id> --section left|center|right`. It shows once it is enabled with
  `host: topbarGlyph` in `plugins.json`.

Declare `topbarGlyph` in `hosts`, and include `"glyph"` in
`capabilities.densities` - a plugin that only draws a card is a poor bar glyph.

What the bar host sets on your `content` (read them, never assign them):

| Property | Value on the bar |
| --- | --- |
| `density` | `"glyph"` |
| `widthBudget` | `220` (logical px; cap any text you draw to it) |
| `active` | `true` while mounted |
| `pluginApi` | your handle: `mainInstance`, `pluginSettings`, `pluginDir` |

The host reads your `implicitWidth`/`implicitHeight` and centres the view in a
slot about 32 px tall. Draw the mark with the shell's own icon font: `Text {
font.family: "Material Symbols Rounded"; text: "vpn_lock" }` (ligature names are
the symbol names; the kit's `GlyphIcon` draws vector glyphs and needs no font). Hover text is yours to
add; `QtQuick.Controls`' `ToolTip` resolves inside a plugin and matches the
shell when styled from the kit `Theme`. On the bar your settings are rendered
by the bar's settings, which draw these `metadata.settings` types: `toggle`,
`choice` (a segment bar up to four plain options, chips beyond), `multi`, `int`
(a stepper with `min`/`max`) and `text`; `slider` and `image` are desktop-widget
menu types and do not render there, so a bar widget picks from the first set.

Install a bar plugin straight from git, or from a folder on this desktop, with
`zenith plugin install <git-url|dir> --bar`: it validates the manifest, audits
the tree, copies it to `~/.local/share/zenith/plugins/<id>`, enables it on the
bar, and the shell picks it up on the `plugins.json` change, no restart. It
lists on the bar's Community page (any plugin whose manifest is not
`"official": true`) with the community warning, its author, its switch and
settings. `zenith plugin list|enable|disable|remove|audit` manage it from a
terminal.

> Island and window hosts are planned but not built yet. Declare only
> `framePopout`, `desktopWidget`, or `topbarGlyph` in your manifest today.

---

## A bar panel

A bar plugin (host `topbarGlyph`) can ship a second view, `content/Panel.qml`,
and declare it as `entryPoints.panel`. When it does, the bar host renders that
file in the shared plugin panel window under the plugin's glyph, in the same
surface the built-in panels use: a `PillShadow`, a connected surface, `Escape`
or an outside click closes it, and only one panel is open at a time. The mark on
the bar stays your `content/Widget.qml`; the panel is what opens beneath it.

`Panel.qml` receives the same host-set properties as `content/Widget.qml`, at
`full` density:

| Property Zenith sets | Value on `Panel.qml` |
| --- | --- |
| `pluginApi` | your handle (the reference below) |
| `density` | `"full"` |
| `s` | scale multiplier; multiply your sizes and font sizes by it |
| `widthBudget` | the panel width from manifest `panel.width` (default `320`, clamped `240..520`) |
| `active` | `true` while the panel is open |

The host fills `Panel.qml` to the card's inside width, `panel.width` minus 24 px
of padding, and reads back its `implicitHeight` to size the card, capping that
height to the screen. `content/Widget.qml`'s click handler is your own; the
sanctioned pattern is a left click that toggles the panel, and no click ever
mutates anything:

```qml
onClicked: pluginApi.togglePanel()
```

### The `pluginApi`

Every entry point (`main`, `content`, `panel`) receives one `pluginApi` object.
Read it; never reassign it.

| Member | Kind | What it is |
| --- | --- | --- |
| `mainInstance` | `var` | your live `service/Main.qml` instance |
| `pluginSettings` | `var` | the resolved settings; read each key behind its default |
| `pluginDir` | `string` | the installed plugin folder (build `bin/` paths from it) |
| `stateDir` | `string` | `$XDG_STATE_HOME/zenith/plugins/<id>`, created by the host on load; the only place you write (R8) |
| `saveSetting(key, value)` | function | persists one setting through `zenith-plugins-place`; `pluginSettings` re-derives on the `plugins.json` change (R5) |
| `panelOpen` | `bool` (readonly) | whether the bar panel is open (bar host; present but inert elsewhere) |
| `openPanel()` / `closePanel()` / `togglePanel()` | function | open, close, or toggle the bar panel |

Your service and its `pluginApi` survive a settings write: `saveSetting`
persists the value and re-derives `pluginSettings` without tearing anything
down, so the service stays alive and the panel stays open. The host destroys and
recreates the service and the api only when the bar row itself is rebuilt, when a
widget in that lane is moved, added, or removed.

`zenith plugin new <id> --bar` writes a worked bar example: a glyph that shows
a live value, a 320-wide panel with a button, and no click on the bar that
changes anything.

---

## Writing `content/Widget.qml`

This is your view. Zenith sets a few properties on its root **for you to read** -
**never assign them**:

| Property Zenith sets | What it means | What you do with it |
|---|---|---|
| `pluginApi` | handle to your service + settings + folder | read `pluginApi.mainInstance` for your live `service/Main.qml` |
| `density` | `"glyph"`, `"compact"`, or `"full"` | lay out smaller/larger for the space you're given |
| `widthBudget` | the width you have to lay out within | size your content to this width, not to the screen |
| `active` | true while your widget is open/visible | start/stop work (e.g. don't poll when hidden) |
| `screen` | the monitor this copy is on | usually ignore it |

### Sizing - the one thing you MUST get right

**Declare your content's natural size; never hardcode geometry.** Zenith reads
your root's `implicitWidth` / `implicitHeight` to size the card or grow the
popout. If you don't report a size, your widget collapses to nothing.

```qml
import QtQuick
import qs.PluginKit.Singletons   // the kit's tokens: Theme, Motion

Item {
    id: root

    // Zenith sets these; you only read them.
    property var pluginApi
    property string density: "full"
    property real widthBudget: 0
    property bool active: false

    readonly property var service: pluginApi ? pluginApi.mainInstance : null

    // Pick ONE content width from the budget, and lay everything out from it.
    readonly property real contentW: widthBudget > 0 ? widthBudget : 360

    // Report your natural size so the host can size its surface around you.
    implicitWidth: contentW
    implicitHeight: column.implicitHeight

    Column {
        id: column
        width: root.contentW          // bind children to contentW, NOT parent.width
        spacing: 12
        // ... your eyebrow, search, list, grid ...
    }
}
```

Rules that keep it native:
- **Bind children to your own `contentW`**, never to `parent.width` through
  nested layouts (that leaves width-derived heights at zero and your widget
  collapses).
- **Reflow with `Column` / `Row` / `Grid` / `Flow`** so you adapt to the width
  Zenith gives you. Don't position with absolute `x`/`y`.
- **Three densities**: `glyph` = an icon (+ optional badge); `compact` = a small
  summary; `full` = the rich panel. Lay out for whichever `density` you're handed.

---

## Styling - use the kit, match the shell

Your runtime content imports the **plugin kit** so it looks like the rest of the
shell automatically:

```qml
import qs.PluginKit            // GlyphIcon, MicroLabel, SearchField, Card,
                               // CornerTicks, WaveMeter
import qs.PluginKit.Singletons // Theme, Motion
```

Use `Theme` colors and `Motion` curves - mono eyebrows, hairline dividers, the
project's morph timing. **Don't hardcode colors**; read them from `Theme`. The
shell is monochrome: the accent is the ink itself, so show emphasis with weight
and position, and reserve colour for an error or warning state.

Your **settings are not QML** - you declare them as a `metadata.settings` schema
in the manifest (below) and Zenith renders native controls for them, both in
Zenith Settings and in the desktop widget's right-click menu.

---

## `service/Main.qml` - your logic

A non-visual `QtObject`/`Item` that holds state and does the work (HTTP, running
your `bin/` scripts, parsing results). Zenith keeps one instance alive and hands
it to your content as `pluginApi.mainInstance`.
Declare user options as a `metadata.settings` schema in your manifest (below).
Zenith renders the controls, seeds your defaults when the plugin is enabled, and
persists changes to `plugins.json`; read the live values from
`pluginApi.pluginSettings`. Read every key behind its default anyway: a plugin
enabled before a setting existed, or a settings block a user trimmed, has no
value for it, and `undefined` must not become your poll interval.

---

## `manifest.json`

```json
{
  "id": "your-plugin",
  "name": "Your Plugin",
  "version": "1.0.0",
  "author": "You <you@example.com>",
  "description": "One sentence describing it.",
  "license": "MIT",
  "official": false,
  "entryPoints": {
    "main": "service/Main.qml",
    "content": "content/Widget.qml",
    "panel": "content/Panel.qml"
  },
  "panel": { "width": 320 },
  "files": ["content/Helper.qml", "assets/example.jpg"],
  "capabilities": {
    "densities": ["glyph", "compact", "full"],
    "network": ["login.tailscale.com"],
    "privileged": ["pkexec tailscale set --operator"]
  },
  "hosts": ["topbarGlyph", "desktopWidget"],
  "defaults": {
    "host": "topbarGlyph",
    "bar": { "section": "right" },
    "icon": "vpn_lock",
    "label": "Your Plugin"
  },
  "commands": ["bin/your-tool"],
  "dependencies": { "commands": ["curl", "jq"] },
  "metadata": {
    "settings": [
      { "key": "poll", "type": "int", "label": "Refresh seconds", "group": "General", "default": 10, "min": 3, "max": 60 },
      { "key": "barLabel", "type": "choice", "label": "Beside the mark", "group": "General", "default": "name",
        "options": [ { "value": "none", "label": "Nothing" }, { "value": "name", "label": "Name" }, { "value": "ip", "label": "IP" } ] },
      { "key": "confirmOff", "type": "toggle", "label": "Ask before turning off", "group": "General", "default": true }
    ]
  }
}
```

- `hosts` - declare **only the hosts you actually support and have tested**
  (today: `framePopout`, `desktopWidget`, `topbarGlyph`). Don't list hosts that
  don't work.
- `defaults` - *suggestions*. The user's choices in Settings always win. For
  `framePopout`, `align` is `start`, `center` or `end`, and `edge: "center"`
  asks for the centred (modal) surface, which opens only on request. A bar
  widget's defaults are just `{ "host": "topbarGlyph", "icon": "...", "label":
  "..." }`, plus an optional `"bar": { "section": "left|center|right" }` for
  the lane it first lands in (the end of the right lane when absent; the
  Layout route moves it from there, and that choice is kept). `icon` is a
  Material Symbols Rounded ligature name (`vpn_lock`, `extension`, ...), the
  mark menus and pickers show for the plugin.
- `official` - leave `false`. Only first-party Zenith plugins set `true`; every
  other plugin lists on the bar's Community page under the community warning.
- **A plugin never gets a keybind of its own.** Zenith has no plugins-menu leader
  and reads no `key` field from your manifest; do not ship one, and do not tell
  users a chord opens your plugin. A frame popout opens on hover at its edge, or
  through `zenith-shell plugin <id>`, which the user can bind to whatever chord
  they like in Settings → Keybinds. The shipped bind table stays Zenith's.
- `files` - any extra files the plugin ships beyond its entry points and
  `commands` (helper QML a view imports, images, data). Install fetches the entry
  points, `commands`, `README.md`, and everything listed here; a file you forget
  to list is simply missing on install, so the plugin can fail to render.
- `entryPoints.panel` - a bar plugin's panel view, `content/Panel.qml` (see
  [A bar panel](#a-bar-panel)). Declare it only alongside host `topbarGlyph`.
- `panel.width` - the panel's width budget in logical px, default `320`, clamped
  `240..520`.
- `capabilities.network` / `capabilities.privileged` - every host the plugin
  talks to and every exact `pkexec` command it can run (R6, R7). Both are
  optional; omit them when the plugin does neither. The two lines shown above are
  an example; declare only your own.

---

## Security

`zenith plugin audit <dir>` (alias `validate`) runs the manifest checks and a
static audit, and prints each finding as `rule-id  path:line  message`.
**Blocking** rules make `audit` exit non-zero and make `zenith plugin install`
refuse to install (unless
you pass `--allow-findings`); `--allow <rule>` downgrades one named blocking rule
to a warning for the run. **Warning** rules are printed but do not block.

| Rule id | Blocks? | What it catches |
| --- | --- | --- |
| `symlink` | **yes** | any symlink in the tree (R2, R10) |
| `escalation` | **yes** | `sudo`, `doas` or `su` anywhere, or `pkexec` not declared in `capabilities.privileged` (R6) |
| `pipe-shell` | **yes** | `curl … \| sh` and other download-and-run (R7) |
| `internal-import` | **yes** | importing `shell.*`, `Ryoku.*` or any `qs.*` module but the kit, or climbing out of the folder (R4) |
| `config-write` | **yes** | writing to `shell.json`, `plugins.json`, `~/.ssh`, `/etc`, or shell rc files (R5, R8) |
| `secret` | **yes** | a token, key or credential in the tree (R10) |
| `binary` | **yes** | a compiled binary, ELF/Mach-O/`.so` (R10) |
| `undeclared-command` | no | a `Process` / `execDetached` `argv[0]` not in `bin/`, `dependencies.commands`, or the allowlist (R6) |
| `undeclared-host` | no | an `https?://host` literal not in `capabilities.network` (R7) |
| `dynamic-shell` | no | `sh` / `bash -c` whose argument is not a single string literal (R9) |
| `outside-write` | no | a write outside `stateDir`, cache, or a temp dir (R8) |
| `removed-name` | no | a Ryoku name Zenith changed: `ryoku-*` commands, `~/.config/ryoku`, `RYOKU_*` (see below) |
| `large-tree` | no | a tree larger than 20 MB |

The audit is a safety net, not a sandbox: a plugin runs unsandboxed inside the
shell with your permissions, and nothing reviews a community plugin for you.
R1..R11 and the capabilities the manifest declares are what keep it honest, and
`install` shows the warning before it copies anything. A plugin that declares no
`capabilities.network` has no business on the network, and the audit flags every
URL it finds.

---

## Install, enable, place

- **Install**: `zenith plugin install <git-url|dir> [--bar]` (audited, then copied
  to `~/.local/share/zenith/plugins/<id>`; `add` is an alias). A folder you drop
  there by hand loads too, but nothing audited it.
- **Enable & place**: `zenith plugin enable <id> [--bar|--desktop|--popout]` and
  `disable <id>`, or the bar's Community page. Placement saves to
  `~/.config/zenith/plugins.json`, written only by `zenith-plugins-place`; the
  shell watches that file and retunes live - no restart.
- **Desktop widgets** are then moved/resized/hidden directly on the wallpaper
  (drag, corner-resize, right-click) - not from Settings.

---

## Porting a Ryoku plugin

A plugin written for Ryoku needs these renames, and `zenith plugin audit` reports
each one with the fix:

| Ryoku | Zenith |
|---|---|
| `import Ryoku.PluginKit` | `import qs.PluginKit` |
| `import Ryoku.PluginKit.Singletons` | `import qs.PluginKit.Singletons` |
| `ryoku-shell`, `ryoku-plugins-place`, `ryoku` | `zenith-shell`, `zenith-plugins-place`, `zenith` |
| `~/.config/ryoku`, `~/.local/share/ryoku`, `$XDG_STATE_HOME/ryoku/plugins/<id>` | the same paths with `zenith` |
| `RYOKU_*`, `RYOSTORE_PLUGINS_DIR` | `ZENITH_*`, `ZENITH_PLUGINS_DIR` |

The kit keeps every token name it had (`Theme.verm`, `Theme.flameInk` ...) so the
plugin still loads, but the values are greys: the design system has no accent
hue, and a plugin that leaned on one will look plainer, not break.

Publishing a plugin to a catalogue (`export`, `share`) is not part of Zenith's
`zenith plugin`.

---

## Gotcha: images in grid/list delegates

An `Image` inside an inline QML `component` used as a delegate **loads but never
paints** in Quickshell (a scene-graph quirk). Put image/thumbnail delegates
inline - a plain `Rectangle { Image {} }` - or in their own `.qml` file, **not**
inside an inline `component { ... }`. See `wallhaven`'s grid for the working
pattern.

