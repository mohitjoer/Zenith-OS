# Wallpaper

`zenith-wallpaper` owns the wallpaper. It is a Go daemon (`desktop/cmd/zenith-wallpaper`, user
unit `zenith-wallpaper.service`) ported from Ryoku's `ryogami`; the picker is Ryoku's wall-ui
(itself the skwd-wall picker), and the shell draws what the daemon publishes.

| Part | Where |
|---|---|
| daemon, socket `$XDG_RUNTIME_DIR/zenith-wallpaper.sock` | `desktop/cmd/zenith-wallpaper` |
| picker window (entry file at the shell root so `qs.*` resolves) | `shell/wallpaper-ui.qml`, `shell/wallpaper-ui/`, launcher `session/zenith-wallpaper-ui` |
| the surface that paints the frame | `shell/modules/wallpaper/` (`Wallpaper.qml` subscribes, `Backdrop.qml` reveals and plays clips) |
| Depth/Parallax cut-out engine | `shell/scripts/zenith-stage`, drawn by `shell/modules/stage/` |

## Where things live

| What | Path |
|---|---|
| daemon config (`paths`, `restoreOnStartup`, `resource_tier`, `matugen.videoFrame`) | `~/.config/zenith/zenith-wallpaper.json` |
| picker config (layout, transitions, day/night, sources) | `~/.config/zenith-wallpaper-wall/config.json` |
| catalogue, thumbnails, stored choice (`outputs.json`), playlists, transcodes, logs | `~/.cache/zenith/wallpaper/` |
| the wallpaper on screen, one absolute path | `$XDG_STATE_HOME/zenith-wallpaper` |
| library | `~/Pictures/Wallpapers`, or an existing `Pictures/wallpaper` spelling of it |

The look settings the picker shows (fit, video engine, transcode) are `wallpaper.*` in `shell.json`.
The picker changes them with `settings.patch` on `zenith-shell.sock`; it never writes the file.

## The wire

One socket carries three things: `subscribe wallpaper` streams the full frame
`{default: ENTRY, outputs: {connector: ENTRY}}` (path, revision, fit, live, videoPath, mute,
volume, transition, depth), verb lines (`wallpaper set|random|next|ui|restore|repaint|resource|live-reload`,
`depth set|clear`) answer `ok`/`err ...`, and JSON-RPC lines (`wall.*`, `playlist.*`, `effects.*`,
`grade.*`, `optimize.*`, `wm.*`) serve the picker, which also receives `zenith-wallpaper.wall.*` events.

## The palette handshake

The daemon never computes colours. `zenith-shell` subscribes to the `wallpaper` topic
(`desktop/cmd/zenith-shell/wallpaper.go`); when the default slot's path or live flag changes it
schedules its theming pass, which runs `zenith theme generate` on the picture (a clip's extracted
still) and renders the templates. The topic replays the current frame on every connect, so a
shell daemon that starts after the wallpaper still themes from it. Cut-outs come back the other
way: the shell daemon's stage worker sends `depth set <json>` / `depth clear`.

## What differs from Ryoku

- **Clips play in the shell.** The external C player (`ryogami-live`) is not built; with no
  `zenith-wallpaper-live` on `PATH` the daemon forces `video_engine` to `in_shell`, and
  `Backdrop.qml` plays the clip through QtMultimedia (`qml6-module-qtmultimedia`).
- **No upscale.** `waifu2x-ncnn-vulkan` is not in the Ubuntu archive, so the daemon has no
  `upscale.*` and the picker has no Upscale tab.
- **First start** paints the wallpaper `org.zenith.shell` names (`wallpaper-path[-dark]`, so an
  adopted desktop keeps its background), else the first image in the library. It reads the keys and
  never writes them.
- The Browse surface drops Ryoku's store source; Wallhaven, Steam, moewalls, motionbgs and GitHub
  repositories remain, all on demand.
- The picker's accent is the inverse plate, not a hue (`wallpaper-ui/qml/Colors.qml`).
