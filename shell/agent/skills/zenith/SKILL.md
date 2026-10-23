---
name: zenith
description: >
  Customize a Zenith desktop: an Ubuntu-based system with a Hyprland compositor and
  a Quickshell shell (the QS Bar, the dock, widgets, the launcher, and the Hub).
  Use for end-user requests that touch the desktop or its config. Triggers:
  Hyprland, window rules, keybinds, monitors, gaps, borders, the bar, the dock,
  bar widgets, plugins, themes, wallpaper, colours, night light, idle, lock
  screen, and user-facing zenith commands (zenith, zenith-shell, zenith-hubd,
  zenith-wallpaper, zenith-agent). Read the vault first; act through commands, not by
  editing shipped files.
---

# Zenith

Zenith is an Ubuntu-based desktop: a Hyprland compositor, a single Quickshell shell
that draws the bar, the dock, the launcher, the popouts and the widgets, and a
set of Go command-line tools that own the config. This skill is for changing a
running Zenith system on behalf of its user. It is not for developing Zenith
itself (editing the source checkout, writing migrations, cutting a release).

## When to use this skill

Use it whenever a request would change the desktop or read its state: the bar
layout and widgets, the dock, Hyprland behaviour, themes and wallpaper, keybinds,
idle and lock, plugins, or any `~/.config` file Zenith owns. If you are about to
guess a path or edit a config file under `~/.config`, stop and use this skill.

Do not use it to modify the Zenith source tree, and never treat a shipped file as
a place to store a user's choice.

## Read the vault first

A maintained map of THIS machine lives in the Zenith Agent vault at
`~/.local/share/zenith/agent/`. Read it before searching the filesystem or
guessing where anything lives:

- `AGENTS.md`: the entry contract and the vault's own rules.
- `desktop.md`: the map. Every subsystem, the config path that owns it, the
  binary that owns it, and how to reload it. Its generated "Bar and dock"
  section lists every bar widget id, its visibility key, and the bar and dock
  commands. Read this before touching the bar.
- `system.md`, `packages.md`, `user.md`, `habits.md`: hardware, packages, where
  this user diverges from the shipped defaults, and this user's directories and
  tool stack.

Ownership inside the vault: the generated maps (`AGENTS.md`, `desktop.md`,
`system.md`, `packages.md`, `zenith-repo.md`, `user.md`, `habits.md`) are rewritten on
every reindex, so read them and never edit them. `memory/` and `journal/` are
yours: write durable notes and dated notes (`journal/YYYY-MM-DD.md`) there and
they survive. `user.md` lists the user's own choices; never revert one to a
shipped default without being asked.

Topic guides sit beside this file. Read the matching one first:

- [`gui.md`](gui.md): the GUI map. Every intent to its Zenith Hub page, shell
  picker, or QS Bar Settings route, how to open it, and the command it wraps.
- [`bar.md`](bar.md): the QS Bar and the dock, their layout model, and the
  `zenith-shell bar` / `zenith-shell dock` commands.
- [`plugins.md`](plugins.md): installing, listing, and removing shell plugins
  with `zenith plugin`, and the store.

## Answer policy: GUI first

When a user asks HOW to change something, lead with the GUI path: the exact
keybind or the Zenith Hub page. The Hub opens with Super+comma; deep-link one
page with `zenith-shell hub open <section>` (the section key names the page, e.g.
`keybinds`, `gpu`, `lockscreen`). Give the command after, as the fallback or the
way to script the same change. When the user asks YOU to change it, act through
the command (the GUI is for the human), then say what you changed and where to
see or undo it in the GUI. Never hand back a bare shell command for a change that
has a page or a picker.

Two surfaces have no Hub page. Wallpaper and theme are the shell picker
(`zenith-wallpaper wallpaper ui`) and the bar Wallpaper widget; the bar layout
and the dock are QS Bar Settings (`zenith-shell bar settings`), not the Hub. See
[`gui.md`](gui.md) for the full intent-to-surface map.

## Safety rules

Zenith separates the files it ships from the files you own, so an update can
refresh the base freely while your changes stand. Respect the split:

- **Never edit a shipped file in place.** `/usr/share/zenith/` (the packaged
  base) and the copies `zenith materialize` lays into `~/.config/zenith/` are re-laid on
  every `zenith update` (`zenith materialize` clobbers every shipped file), so an
  edit there is lost on the next update. Reading them is safe and useful.
- **A user override goes to the overlay:** `~/.config/zenith/user_edits/`, which
  mirrors `~/.config/zenith`. A file there wins at its mirrored path and survives every
  update. To change a shipped Hyprland or app config, drop your version at the
  mirrored path under `user_edits` (a fork), or, better, use the dedicated
  override file the tool already reads (`hyprland/user.conf` for Hyprland;
  `settings.conf` and `rebinds.conf` beside it are written by Zenith Settings, so
  leave them), which the package never ships and never touches. `zenith reset <path>` drops an overlay file back to the base.
- **Prefer a command over a file edit.** The tool that owns a setting is its one
  writer; hand-editing its store drifts. Zenith Settings' own state (bar, colours,
  launcher, device lighting) lives under `~/.config/zenith/*.json`, written by
  their tools (the shell daemon, `zenith-hubd`, `zenith-wallpaper`); do not hand-edit those
  JSON stores, drive them through the command or the GUI so one writer stays in
  charge.

## Command discovery

Zenith's behaviour lives behind five command-line tools, all self-documenting.
Prefer a command to a file edit; read a command's `--help` before running it.

| Tool | Owns |
|---|---|
| `zenith` | Updates, rollback, status, materialize, reset, doctor. `zenith --help` lists them. |
| `zenith-shell` | The live shell: the bar, the dock, menus, popouts, and the `shell.json` settings store (the sole writer of `shell.json`). |
| `zenith-hubd` | The Zenith Settings data plane and the Hyprland overlay files it generates. Read `zenith-hubd --help`. |
| `zenith-wallpaper` | Wallpapers and the colour palette (`zenith-wallpaper wallpaper set|next|random`). |
| `zenith-agent` | The assistant: the vault, wiring, the dashboard, `index`, `wire`. |

```bash
zenith --help                 # the zenith CLI surface
zenith-shell bar catalog      # every bar widget, its id, and its settings
zenith-shell bar list         # the live bar, per section, with shown state
zenith-wallpaper wallpaper --help
```

To find WHERE a setting is read (which QML file, which key), use `prowl`
inside the vault's read-only source mirror at
`~/.local/share/zenith/agent/source/`, which indexes the live `~/.config/zenith`:

```bash
cd ~/.local/share/zenith/agent/source && prowl search "barPosition"
cd ~/.local/share/zenith/agent/source && prowl find barShellStyle
```

The mirror is read-only and rebuilt on every reindex; never edit files in it,
edit the real path `desktop.md` names.

## Decision framework

When a request would change the system, in order:

1. **Is there a GUI for it?** A Zenith Hub page (`zenith-shell hub open <section>`,
   Super+comma), the wallpaper picker (`zenith-wallpaper wallpaper ui`), or QS Bar Settings
   (`zenith-shell bar settings`). If a user asks how to do it, name the surface
   first; whether you or they drive it, the change still lands through the
   command that page wraps, so read on.
2. **Is there a command for it?** Use it. The bar and dock have a full CLI
   (`zenith-shell bar ...`, `zenith-shell dock ...`, see `bar.md`); wallpaper has
   `zenith-wallpaper wallpaper set`; updates have `zenith update`.
3. **Is it a config edit with no command?** Edit the override, never the shipped
   file: the tool's own `user.*` file, or a fork at the mirrored path under
   `~/.config/zenith/user_edits/`. Then reload (`systemctl --user reload zenith-shell` for the shell, `hyprctl
   reload` for Hyprland).
4. **Is it a plugin?** A shell widget installs from git with
   `zenith plugin add <url> --bar`, or from the store; see `plugins.md`. Never
   run a plugin's code to install it. A Hyprland compositor plugin (title
   bars, cursor motion, key sounds, a `.so` the compositor loads) is managed
   by `zenith-hubd desktop plugins list|rebuild|add|remove` and Hub > Plugins;
   a "version mismatch" after an update means
   `zenith-hubd desktop plugins rebuild --stale`.
5. **Is it a theme or wallpaper?** Drive it through `zenith-wallpaper` and `zenith-hubd`,
   which own the colour master; never write the palette or theme shadow by hand.
6. **Is it a package?** `zenith update` for the whole system; `apt` for one
   package (installing needs root: give the user the `sudo apt install ...` line
   to run in a terminal).
7. **Unsure a command exists?** Read the tool's `--help`, or `desktop.md`.

## Example requests

- "Put the bar at the bottom" -> QS Bar Settings > Layout
  (`zenith-shell bar settings layout`), or `zenith-shell bar position bottom`
- "Move the clock to the right" -> QS Bar Settings > Layout, or
  `zenith-shell bar move clock --section right`
- "Hide the GPU widget" -> QS Bar Settings > Widgets, or `zenith-shell bar hide gpu`
- "Make the bar islands" -> QS Bar Settings > Bars, or `zenith-shell bar form islands`
- "Open the bar settings" -> `zenith-shell bar settings` (the launcher mark opens it too)
- "Turn the dock off" -> QS Bar Settings > Dock, or `zenith-shell dock hide`
- "Pin Firefox to the dock" -> QS Bar Settings > Dock, or `zenith-shell dock pin firefox`
- "Change my wallpaper" -> the wallpaper picker (`zenith-wallpaper wallpaper ui`);
  scripted: `zenith-wallpaper wallpaper set <path>`
- "Remap a key" -> Zenith Hub > Keybinds (Super+comma, `zenith-shell hub open keybinds`)
- "Lock after ten minutes" -> Zenith Hub > Graphics & Power > Idle
  (`zenith-shell hub open gpu`)
- "Add a weather plugin from GitHub" -> `zenith plugin add <git-url> --bar`
- "List my installed plugins" -> Zenith Hub > Add-ons, or `zenith plugin list`
- "Update the system" -> Zenith Hub > Updates, or `zenith update`
- "Roll back a bad update" -> `zenith rollback`
