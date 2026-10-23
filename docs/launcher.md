# The launcher

The launcher is the command palette on Super+Space: one search box for apps, commands,
files, packages, radio, the calculator, open windows and system actions. It is Ryoku's
launcher, ported as it is (GPL-3.0, see `NOTICE.md`), and lives in `shell/modules/launcher/`.
`shell.qml` loads it, builds it on the first open and tears it down a while after the last
close. Its look is `~/.config/zenith/launcher.json` (`variant`: hero, main, okshell or
kairos).

## Providers

A provider answers a query with result rows. The unprefixed search fans out across the
default ones; a prefix sends the query to one. Each provider that needs a tool outside the
shell checks for it once when the launcher is built and **registers only if the tool is
there**. A provider that did not register has no prefix, no row in the F1 help, and no mode
key, and nothing is run or logged for it.

| Provider | Prefix | Needs | Without it |
|---|---|---|---|
| apps | none | Quickshell desktop entries | n/a |
| windows | none | the daemon's `wm` topic (`zenith-shell`) | no rows until the daemon is up |
| actions | `/` | the command each action runs | that action is left out |
| calc | `=`, a leading digit | `zenith-cmd-calc` (uses `qalc` when installed) | not registered |
| packages | `>` | `apt-cache`, `dpkg-query`; installs use `pkexec` and a terminal | not registered |
| find | `/file /folder /image /video`, the IMG and FILE keys | `plocate` with a readable database | not registered, keys hidden |
| radio | `@` | `zenith-cmd-radio` (mpv, yt-dlp) | not registered, status never polled |
| web | `?` | a browser; the instant answer uses `curl` | no answer panel |
| ask | `\` | `zenith-agent` | the panel says "not available: install zenith-agent" |
| recent, snippets, script, mpris | various | files under `~/.config/zenith/`, D-Bus | n/a |

## Packages

`>name` searches. `apt-cache search` gives the names, ranked name matches first and capped at
30, and `dpkg-query` says what is installed. `>install name` and `>remove name` filter the
same way; Enter opens a terminal (`zenith-app terminal`) running `pkexec apt-get`, so the
password prompt and apt's output are visible. A non-root `apt-cache` takes about a second when
dpkg's status is newer than apt's cache file, so the query is debounced and cached per term.

## Files

plocate has no types and no roots, so a short script keeps what is under `$HOME`, drops
dot-directories, `node_modules` and `Trash`, and tests file or directory. plocate's database
is rebuilt daily by its own timer: a file made today may not show, and until the first build
the finder is hidden.

## Icons

`shell/Services/Icons.qml` loads one name-to-file index. It asks `zenith-shell icons` and falls
back to `zenith icons`, which prints the same JSON without the daemon. The index follows the
active icon theme (gsettings, then GTK 4 and GTK 3 `settings.ini`), the themes it inherits,
Adwaita, and hicolor last. A scalable SVG wins; otherwise the smallest raster of 256 px or
more, so a 1024 px PNG is not decoded to draw a small icon.

## Applications

`zenith apps` reads the desktop entries in the specification's order (the user's first, then
Flatpak's export directories, then `XDG_DATA_DIRS`) and applies `Hidden`, `NoDisplay`,
`OnlyShowIn` and `NotShowIn` against `XDG_CURRENT_DESKTOP` (`Zenith` in a Zenith session) and
`TryExec`. The launcher itself lists Quickshell's `DesktopEntries` and honours `NoDisplay`
only, so a GNOME-only entry that does not set it still appears. Launch counts, which rank
results, are kept in `~/.local/state/zenith/launcher-usage.json`.
