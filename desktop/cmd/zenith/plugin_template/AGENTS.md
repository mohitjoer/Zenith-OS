# Agent guide for the {{ID}} plugin

This folder is a Zenith shell plugin. If you are an AI agent editing it, follow
every rule below. They are the same rules `zenith plugin audit` enforces, so
work that breaks one will not install. This file is self-contained: you do not
need any other document to work here.

## The rules

R1  Place. A plugin is one folder named after its id. Author it under
`$(xdg-user-dir DOCUMENTS)/zenith-plugins/<id>/` (fallback
`~/zenith-plugins/<id>/`); `zenith plugin new <id>` creates it there. Never write
into `~/.local/share/zenith/plugins/` (the install root; `zenith plugin install`
owns it) or `/usr/share/zenith/` (the shipped shell).

R2  Shape. `manifest.json` at the root; `service/Main.qml` (logic, no UI);
`content/Widget.qml` (the one view); optional `content/Panel.qml` (a bar panel);
`README.md`; `LICENSE`; `assets/preview-widget.png` (a real capture); scripts
only under `bin/` with a shebang and the exec bit. List every extra file in
manifest `files`.

R3  Id. Lowercase `[a-z0-9][a-z0-9-]*`, unique, not a built-in widget id.

R4  Imports. Only `QtQuick*`, `Quickshell*`, `qs.PluginKit`,
`qs.PluginKit.Singletons`, and files inside the plugin folder. Never any other
`qs.*` module, `shell.*`, or a relative import that climbs out of the folder.
A plugin written for Ryoku imports `Ryoku.PluginKit`: change it to
`qs.PluginKit` (and `Ryoku.PluginKit.Singletons` to `qs.PluginKit.Singletons`).

R5  Settings. Declared in `metadata.settings` (types the bar renders: toggle,
choice, multi, int, text); read through `pluginApi.pluginSettings` behind a
default; written only through `pluginApi.saveSetting(key, value)`. Never edit
`shell.json`, `plugins.json` or anything else under `~/.config/zenith`.

R6  Commands. Every external program the plugin runs is either in its own `bin/`
or listed in `dependencies.commands`. No `sudo`, `doas`, `su`. A privileged
action is allowed only through `pkexec`, only on an explicit click, only when
listed in manifest `capabilities.privileged` (exact command strings) and
explained in README.

R7  Network. A plugin has no network by default. Every host it talks to is
listed in `capabilities.network`, and a plugin with none listed may not use
`curl`, `wget`, `XMLHttpRequest`, `WebSocket` or a URL. No `curl ... | sh`, no
downloading and running code.

R8  Files. Write only under `pluginApi.stateDir`
(`$XDG_STATE_HOME/zenith/plugins/<id>`), `$XDG_CACHE_HOME/zenith/plugins/<id>`,
or a temp dir. Never touch `~/.ssh`, `/etc`, shell rc files, or another plugin's
folder.

R9  Shell. No `sh -c` with a string built from settings or program output
(injection). Pass argv arrays.

R10 Secrets and binaries. No tokens, keys or credentials in the tree; no
compiled binaries (ELF/Mach-O/.so); scripts only.

R11 Honesty. `official` is never true for a community plugin. `author` is
`Name <mail>`. README says what runs, what it reads, what it writes, and every
privileged/network capability.

## Look

The shell is monochrome: black, grey and white, with ink weight, shape and
position doing the work a colour would. Draw with the kit's `Theme` tokens
(`import qs.PluginKit.Singletons`) and never hard-code a colour, a font or a
radius. The one colour a plugin may add is an error or warning state.

## The workflow

1. `zenith plugin new <id>` scaffolds this folder (already done).
2. Edit: `service/Main.qml` for logic, `content/Widget.qml` for the view, and
   `content/Panel.qml` for the bar panel. Capture `assets/preview-widget.png`.
3. `zenith plugin audit .` and fix every blocking finding (warnings are
   advisory but read them).
4. `zenith plugin install . --bar --yes` to install it, then verify it on the bar
   and on the bar's Community page.
5. `zenith plugin share <id>` ONLY when the user asks to publish. Never share a
   plugin unless asked.

## What the host sets

Every entry point receives, from the host: `pluginApi` (see below), `s` (a scale
factor), `active`, `density`, and `widthBudget`. Read them; never assign them.

`pluginApi` exposes:

- `mainInstance`: the live `service/Main.qml` instance every view shares.
- `pluginSettings`: the resolved settings object; read behind a default.
- `pluginDir`: this plugin's folder.
- `stateDir`: `$XDG_STATE_HOME/zenith/plugins/<id>`; the only place to write.
- `saveSetting(key, value)`: persist one setting.
- `panelOpen` (bool), `openPanel()`, `closePanel()`, `togglePanel()`: the bar
  panel controls (present but inert off the bar). The sanctioned widget click is
  `onClicked: pluginApi.togglePanel()`.
