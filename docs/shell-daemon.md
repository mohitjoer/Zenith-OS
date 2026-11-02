# The shell daemon

`asura-shell daemon` (unit `asurad.service`) is the process the shell's services talk to. It owns the
things QML should not do itself: the system tray host, NetworkManager, power profiles, the clipboard
history, weather, the night light, the key-press stream, the PolicyKit agent and the keyring prompter.
Every other `asura-shell <verb>` is a client that sends one line to it.

It is ported from Ryoku's `ryoku-shell` daemon. What differs on the wire is in
`desktop/cmd/asura-shell/CONTRACT.md`; this page is how it runs here.

## Processes

```
asura-session.target
 ├─ asurad.service         asura-shell daemon     socket $XDG_RUNTIME_DIR/asura-shell.sock
 └─ asura-shell.service    quickshell -p /usr/share/asura/shell
```

systemd starts and restarts both; the daemon does not start or supervise Quickshell. It reaches the
running shell with `quickshell ipc -p /usr/share/asura/shell call ...`, and `asura-shell reload` is
`systemctl --user reload asura-shell`. The unit has no `WAYLAND_DISPLAY` condition: nothing the daemon
probes is a Wayland client. It conflicts with `hyprpolkitagent.service` because only one agent can hold
the session's polkit slot.

The compositor is reached through `desktop/internal/wm`, in-process: `hyprctl` and Hyprland's event
socket. `asura wm` uses the same package.

## What each part talks to

| Part | Talks to | Without it |
|---|---|---|
| tray | StatusNotifier on the session bus | empty tray |
| network | NetworkManager (system bus); DNS edits through `asura-dns` | no Wi-Fi controls |
| power profiles | `net.hadess.PowerProfiles` (power-profiles-daemon) | control hidden |
| clipboard | `wl-paste --watch`, `wl-copy` | empty history |
| night light | `asura-cmd-nightlight`, backend `gammastep` | control hidden |
| lock, sleep | `hyprlock`; logind delay inhibitors and `PrepareForSleep` | `lock` says hyprlock is missing |
| palette | `asura theme generate` / `render` | the desktop keeps its last palette |
| updates | `asura status --json` | no update glyph |
| polkit, keyring | polkitd, `polkit-agent-helper-1`; gnome-keyring's prompter | no auth dialog |
| key presses | `/dev/input/event*` | the topic says it cannot read them |
| weather, holidays | open-meteo; ipwho.is only for the location `auto`; nager.at; nothing while no location is set | quiet "set a location" state |

A missing tool is detected and reported in the topic or the reply; the daemon never fails to start
over one.

## Running it by hand

Never against the live socket. A private runtime directory and session bus keep it away from the real
desktop:

```
T=$(mktemp -d); mkdir -m 700 $T/run $T/home
XDG_RUNTIME_DIR=$T/run HOME=$T/home dbus-run-session -- \
    go run ./cmd/asura-shell daemon
```

`go test ./cmd/asura-shell/ ./internal/wm/` is hermetic. `ASURA_BIN=<built asura>` also runs the
palette round trip against the real generator and renderer.
