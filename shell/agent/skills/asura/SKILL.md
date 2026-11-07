---
name: asura
description: >
  Customize an Asura OS desktop: a Hyprland compositor and
  a Quickshell shell (the QS Bar, the dock, widgets, the launcher, and the Hub).
  Use for end-user requests that touch the desktop or its config. Triggers:
  Hyprland, window rules, keybinds, monitors, gaps, borders, the bar, the dock,
  bar widgets, plugins, themes, wallpaper, colours, night light, idle, lock
  screen, and user-facing asura commands (asura, asura-shell, asura-hubd,
  asura-wallpaper). Read the system configuration first; act through commands, not by
  editing shipped files.
---

# Asura

Asura OS is an independent modern operating system: a Hyprland compositor, a single Quickshell shell
that draws the bar, the dock, the launcher, the popouts and the widgets, and a
set of Go command-line tools that own the config. This skill is for changing a
running Asura OS system on behalf of its user. It is not for developing Asura OS
itself (editing the source checkout, writing migrations, cutting a release).

## When to use this skill

Use it whenever a request would change the desktop or read its state: the bar
layout and widgets, the dock, Hyprland behaviour, themes and wallpaper, keybinds,
idle and lock, plugins, or any `~/.config` file Asura owns. If you are about to
guess a path or edit a config file under `~/.config`, stop and use this skill.

Do not use it to modify the Asura source tree, and never treat a shipped file as
a place to store a user's choice.

## Read the vault first

A maintained knowledge base of THIS machine lives in the Asura Agent vault at
`~/.local/share/asura/agent/`, structured per Google Open Knowledge Format (OKF v0.1).
Read it before searching the filesystem or guessing where anything lives:

- `index.md`: root OKF index and knowledge graph table of contents.
- `AGENTS.md`: the entry contract and the vault's operational rules.
- `desktop.md`: the map. Every subsystem, the config path that owns it, the
  binary that owns it, and how to reload it. Its generated "Bar and dock"
  section lists every bar widget id, its visibility key, and the bar and dock
  commands. Read this before touching the bar.
- `system.md`, `packages.md`, `user.md`, `habits.md`: hardware, packages, where
  this user diverges from the shipped defaults, and this user's directories and
  tool stack.
- `log.md`: audit trail of reindexes and system transitions over time.

Ownership inside the vault: the generated maps (`index.md`, `log.md`, `AGENTS.md`, `desktop.md`,
`system.md`, `packages.md`, `asura-repo.md`, `user.md`, `habits.md`) are rewritten on
every reindex, so read them and never edit them. `memory/` and `journal/` are
yours: write durable notes and dated notes (`journal/YYYY-MM-DD.md`) there and
they survive. `user.md` lists the user's own choices; never revert one to a
shipped default without being asked.

Topic guides sit beside this file. Read the matching one first:

- [`gui.md`](gui.md): the GUI map. Every intent to its Asura Hub page, shell
  picker, or QS Bar Settings route, how to open it, and the command it wraps.
- [`bar.md`](bar.md): the QS Bar and the dock, their layout model, and the
  `asura-shell bar` / `asura-shell dock` commands.
- [`plugins.md`](plugins.md): installing, listing, and removing shell plugins
  with `asura plugin`, and the store.

## Answer policy: GUI first

When a user asks HOW to change something, lead with the GUI path: the exact
keybind or the Asura Hub page. The Hub opens with Super+comma; deep-link one
page with `asura-shell hub open <section>` (the section key names the page, e.g.
`keybinds`, `gpu`, `lockscreen`). Give the command after, as the fallback or the
way to script the same change. When the user asks YOU to change it, act through
the command (the GUI is for the human), then say what you changed and where to
see or undo it in the GUI. Never hand back a bare shell command for a change that
has a page or a picker.

Two surfaces have no Hub page. Wallpaper and theme are the shell picker
(`asura-wallpaper wallpaper ui`) and the bar Wallpaper widget; the bar layout
and the dock are QS Bar Settings (`asura-shell bar settings`), not the Hub. See
[`gui.md`](gui.md) for the full intent-to-surface map.

## Safety rules

Asura separates the files it ships from the files you own, so an update can
refresh the base freely while your changes stand. Respect the split:

- **Never edit a shipped file in place.** `/usr/share/asura/` (the packaged
  base) and the copies `asura materialize` lays into `~/.config/asura/` are re-laid on
  every `asura update` (`asura materialize` clobbers every shipped file), so an
  edit there is lost on the next update. Reading them is safe and useful.
- **A user override goes to the overlay:** `~/.config/asura/user_edits/`, which
  mirrors `~/.config/asura`. A file there wins at its mirrored path and survives every
  update. To change a shipped Hyprland or app config, drop your version at the
  mirrored path under `user_edits` (a fork), or, better, use the dedicated
  override file the tool already reads (`hyprland/user.conf` for Hyprland;
  `settings.conf` and `rebinds.conf` beside it are written by Asura Settings, so
  leave them), which the package never ships and never touches. `asura reset <path>` drops an overlay file back to the base.
- **Prefer a command over a file edit.** The tool that owns a setting is its one
  writer; hand-editing its store drifts. Asura Settings' own state (bar, colours,
  launcher, device lighting) lives under `~/.config/asura/*.json`, written by
  their tools (the shell daemon, `asura-hubd`, `asura-wallpaper`); do not hand-edit those
  JSON stores, drive them through the command or the GUI so one writer stays in
  charge.

## Command discovery

Asura's behaviour lives behind five command-line tools, all self-documenting.
Prefer a command to a file edit; read a command's `--help` before running it.

| Tool | Owns |
|---|---|
| `asura` | Updates, rollback, status, materialize, reset, doctor. `asura --help` lists them. |
| `asura-shell` | The live shell: the bar, the dock, menus, popouts, and the `shell.json` settings store (the sole writer of `shell.json`). |
| `asura-hubd` | The Asura Settings data plane and the Hyprland overlay files it generates. Read `asura-hubd --help`. |
| `asura-wallpaper` | Wallpapers and the colour palette (`asura-wallpaper wallpaper set|next|random`). |

```bash
asura --help                 # the asura CLI surface
asura-shell bar catalog      # every bar widget, its id, and its settings
asura-shell bar list         # the live bar, per section, with shown state
asura-wallpaper wallpaper --help
```

To find WHERE a setting is read (which QML file, which key), use `prowl`
inside the vault's read-only source mirror at
`~/.local/share/asura/agent/source/`, which indexes the live `~/.config/asura`:

```bash
cd ~/.local/share/asura/agent/source && prowl search "barPosition"
cd ~/.local/share/asura/agent/source && prowl find barShellStyle
```

The mirror is read-only and rebuilt on every reindex; never edit files in it,
edit the real path `desktop.md` names.

## Decision framework

When a request would change the system, in order:

1. **Is there a GUI for it?** A Asura Hub page (`asura-shell hub open <section>`,
   Super+comma), the wallpaper picker (`asura-wallpaper wallpaper ui`), or QS Bar Settings
   (`asura-shell bar settings`). If a user asks how to do it, name the surface
   first; whether you or they drive it, the change still lands through the
   command that page wraps, so read on.
2. **Is there a command for it?** Use it. The bar and dock have a full CLI
   (`asura-shell bar ...`, `asura-shell dock ...`, see `bar.md`); wallpaper has
   `asura-wallpaper wallpaper set`; updates have `asura update`.
3. **Is it a config edit with no command?** Edit the override, never the shipped
   file: the tool's own `user.*` file, or a fork at the mirrored path under
   `~/.config/asura/user_edits/`. Then reload (`systemctl --user reload asura-shell` for the shell, `hyprctl
   reload` for Hyprland).
4. **Is it a plugin?** A shell widget installs from git with
   `asura plugin add <url> --bar`, or from the store; see `plugins.md`. Never
   run a plugin's code to install it. A Hyprland compositor plugin (title
   bars, cursor motion, key sounds, a `.so` the compositor loads) is managed
   by `asura-hubd desktop plugins list|rebuild|add|remove` and Hub > Plugins;
   a "version mismatch" after an update means
   `asura-hubd desktop plugins rebuild --stale`.
5. **Is it a theme or wallpaper?** Drive it through `asura-wallpaper` and `asura-hubd`,
   which own the colour master; never write the palette or theme shadow by hand.
6. **Is it a package?** `asura update` for the whole system; `apt` for one
   package (installing needs root: give the user the `sudo apt install ...` line
   to run in a terminal).
7. **Unsure a command exists?** Read the tool's `--help`, or `desktop.md`.

## Example requests

- "Put the bar at the bottom" -> QS Bar Settings > Layout
  (`asura-shell bar settings layout`), or `asura-shell bar position bottom`
- "Move the clock to the right" -> QS Bar Settings > Layout, or
  `asura-shell bar move clock --section right`
- "Hide the GPU widget" -> QS Bar Settings > Widgets, or `asura-shell bar hide gpu`
- "Make the bar islands" -> QS Bar Settings > Bars, or `asura-shell bar form islands`
- "Open the bar settings" -> `asura-shell bar settings` (the launcher mark opens it too)
- "Turn the dock off" -> QS Bar Settings > Dock, or `asura-shell dock hide`
- "Pin Firefox to the dock" -> QS Bar Settings > Dock, or `asura-shell dock pin firefox`
- "Change my wallpaper" -> the wallpaper picker (`asura-wallpaper wallpaper ui`);
  scripted: `asura-wallpaper wallpaper set <path>`
- "Remap a key" -> Asura Hub > Keybinds (Super+comma, `asura-shell hub open keybinds`)
- "Lock after ten minutes" -> Asura Hub > Graphics & Power > Idle
  (`asura-shell hub open gpu`)
- "Add a weather plugin from GitHub" -> `asura plugin add <git-url> --bar`
- "List my installed plugins" -> Asura Hub > Add-ons, or `asura plugin list`
- "Update the system" -> Asura Hub > Updates, or `asura update`
- "Roll back a bad update" -> `asura rollback`
