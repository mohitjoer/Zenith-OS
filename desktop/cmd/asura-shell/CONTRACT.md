# asura-shell: where it differs from Ryoku's `ryoku-shell`

Everything not listed is Ryoku's, unchanged: the framing (`subscribe <topic>`, `call <method> <json>`, one
JSON line per frame/reply), every topic frame, every control call, every verb. `go test` pins the QML side:
`qmlcontract_test.go` fails if the shell's QML subscribes to, calls or spawns anything the daemon lacks.

- Socket `$XDG_RUNTIME_DIR/asura-shell.sock` (mode 0700) plus `.lock`; unit `asurad.service`
  (`ExecStart=/usr/bin/asura-shell daemon`). State `~/.local/state/asura`, cache `~/.cache/asura`, config
  `~/.config/asura`.
- The daemon does not start or supervise Quickshell; systemd does (`asura-shell.service`). It reaches the
  shell with `quickshell ipc -p /usr/share/asura/shell call <target> <fn> ...` (`ASURA_SHELL_DIR` points
  at a checkout's `shell/`). Targets used: `shell` (openSurface, closeSurface, closeAllMenus, keyringPrompt,
  keyringHide, voiceShow/Off/Hide, pluginPopout, sessionConfirm), `theme.reload`, `qsbar.settings`,
  `lifecycle.ready`.
- `reload` = reload cover, `systemctl --user reload asura-shell`, then wait for a new main pid that answers
  `lifecycle ready`. `status` prints `shell: running|starting|stopped`. No `components`, no `state` verb, no
  `performance.json` `disabledComponents` / unload-when-idle workers (there is no second process to park).
- `bar <edge> <action>` still emits `setBar`, which the shell has no handler for (inherited no-op).
  `bar list|catalog|...` and `dock ...` are unchanged; the dock default is `enabled: true` like `Config.qml`.
- `hub open [page]` runs `asura-hub` under `flock -n -o $XDG_RUNTIME_DIR/asura-hub.lock`, navigates with
  `quickshell ipc -p <shell>/hub.qml call nav open|section`, raises the window by title `Asura Settings`;
  `hub close` = `call hub close`.
- Lock: `lock` starts `hyprlock -c ~/.config/asura/hyprland/hyprlock.conf` (else the packaged
  `/usr/share/asura/hyprland/hyprlock.conf`), once; `lock status` is `locked` while a hyprlock of this user
  lives. There is no proof marker, no token, no qylock.
- Sleep guard: two logind *delay* inhibitors (Ryoku held a hard *block*, which would stop `systemctl suspend`,
  the lid and hypridle from suspending at all). On `PrepareForSleep(true)` it locks, then releases both.
  `suspend`, `suspend transaction <token>`, `suspend-cancel`, `sleep-ready` are kept.
- Not available, answered `err <verb>: not available on Asura: <why>`: `unlock-prepare`, `lock-quiesce`,
  `state`, `lock session <id>`; `browser-host` (no extension) exits 1 with the same message. Dropped with no
  verb: the boot guard marker, stray-shell reaping, the QML import-path export.
- `voice` needs `voxtype` (not in the archive): without it the pill shows its "off" note, as in Ryoku.
- Palette: `asura theme generate --image I --mode dark|light --scheme S --contrast C --lightness-dark D
  --lightness-light L --source-color-index N --prefer P --json` (matugen-shaped JSON on stdout); templates:
  `asura theme render --config CFG --palette CARRIER`. The default scheme is `scheme-monochrome`. Template
  configs are read from `~/.config/asura/shell/matugen/` (else `/usr/share/asura/shell/matugen/`). Writers:
  `colors.json`, `tones.json` (daemon).
- `updates` topic = verbatim stdout of `asura status --json` (the CLI owns the apt part), checked at start,
  then per `update_interval` in `~/.config/asura/hub.toml`.
- `wm` topic: same frame. Capabilities not claimed: `globalShortcuts`, `focusGrab`, `plugins`,
  `nativeOverview`, `overviewBackdrop`, `outputMirror`, `outputHdr`; `nightLight` only while `gammastep` is
  installed (`nightLightProcess` = `gammastep`), `touchpadToggle` only while `asura-touchpad` is.
  `configFiles` = `["asura/hyprland/user.conf"]`. Workspace `layout` is `general:layout` unless this daemon
  applied `workspace.layout`. The settings frame's `deadKeys` is always `[]`.
- `wm.act`: same action ids, hyprlang underneath. `input.touchpad` runs `asura-touchpad`, `output.cycle` runs
  `asura-monitor toggle`, `window.fullscreen` focuses the window first, `app.focus` is a class regex.
- Night light files, written only by `asura-cmd-nightlight`: `$XDG_STATE_HOME/asura-nightlight` (temp),
  `asura-nightlight-enabled` (marker), `asura-nightlight-schedule.json` (daemon).
- Sounds come from the installed sound theme (Yaru, then freedesktop), not from embedded clips.
- The polkit agent registers only with `ASURA_POLKIT_AGENT=1` (the unit sets it); the unit conflicts with
  `hyprpolkitagent.service`. Key presses need read access to `/dev/input`; without it the `keypress` frame
  says `error: Cannot read keyboard input: ...`.
- Weather: no location set = no request of any kind and the frame `status` is `"noLocation"` (not an error,
  no `error` text; QML shows "set a location"). The IP lookup (ipwho.is) happens only for the explicit
  location `auto`; a place name geocodes and coordinates are used as given. Nothing is decided before the first
  `weather.configure`. Holidays fetch only on `calendar.configure`.
- The `keypress` backend is switched off when its last subscriber disconnects.
- Needs from other lanes: `asura-cmd-nightlight` (W24) on `/usr/bin`, `asura-dns` (privileged DNS helper,
  no owner yet; `network.dnsSet` errors without it), `asura-plugins-place` (W15), `asura-hubd lighting
  apply|accent` (W06, optional).
