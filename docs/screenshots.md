# Screenshots

`zenith-shot` captures, annotates, and copies or saves a shot. It is a port of the
reference desktop's screenshot tool: a Quickshell layer surface on the Overlay layer
(namespace `zenith-shot`) that freezes each output with the compositor's screencopy and
draws the editor over the frozen frame. `session/zenith-shot` is a wrapper that runs
`quickshell -p /usr/share/zenith/shell/shot.qml` under a lock, so a second key press
while the overlay is open does nothing.

| Key | What it does |
|---|---|
| Print, Super+Shift+S | Start on a region, or the window under the pointer |
| Shift+Print | Start on a whole monitor (`ZENITH_SHOT_MODE=monitor`) |
| Space, Ctrl+A | While choosing: switch the target between region and monitor; take the monitor under the pointer |
| Enter | Copy and save |
| Ctrl+C, Ctrl+S | Copy only, save only |
| Ctrl+P, Ctrl+U | Pin to the desktop; upload (below) |
| `?` | List every key; the tool keys are single letters |

Shots are written to `$XDG_PICTURES_DIR/Screenshots` (else `~/Pictures/Screenshots`) as
`YYYY_MM_DD_HH_MM_SS_screenshot.png`, in UTC. The tool's own settings (tool styles, blur
radius, copy on save, save folder) live in `~/.config/zenith/zenith-shot.json`, which only
the tool writes. Upload sends the shot to litterbox.catbox.moe, a public host, and copies
the 72-hour link; nothing is sent until the button or Ctrl+U is used.

The quick-settings Capture tab is a separate, simpler path (`shell/Services/Capture.qml`):
pick a mode and a delay, and grim takes the shot with no editor. Both paths name files the
same way.

## What it needs

| Tool | For | If it is missing |
|---|---|---|
| `grim` | the frozen frame when screencopy gives none; every shot from the Capture tab | a toast and an on-screen note say to install it |
| `wl-clipboard` | copying when the `zenith-shell` daemon is not running | a toast says the shot was not copied |
| `imagemagick` | the eyedropper, the redaction palette, shots that span monitors | those three do nothing |
| `zenith-cmd-ocr` (tesseract) | the Copy text tool | the script says what is missing |
| `zenith-pin` | Ctrl+P | a toast says pinning is not available |
| `zenity` | choosing the save folder | the button does nothing |

Window snapping reads window boxes from the shell daemon's `wm` feed; without the daemon
the tool still works on regions and monitors.

## Not ported

The beautify editor is not here. It needed changes to run on Ubuntu (its HD upscaler,
`waifu2x-ncnn-vulkan`, is not in the archive, and its watermark and presets carry the
reference desktop's branding), so it was left out rather than shipped half working. The
Capture tab's "Beautify after" switch is stored but opens nothing.
