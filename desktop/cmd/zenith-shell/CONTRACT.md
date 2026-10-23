# zenith-shell: where it differs from Ryoku's `ryoku-shell`

Everything not listed is Ryoku's, unchanged: the framing (`subscribe <topic>`, `call <method> <json>`, one
JSON line per frame/reply), every topic frame, every control call, every verb. `go test` pins the QML side:
`qmlcontract_test.go` fails if the shell's QML subscribes to, calls or spawns anything the daemon lacks.

- Socket `$XDG_RUNTIME_DIR/zenith-shell.sock` (mode 0700) plus `.lock`; unit `zenithd.service`
  (`ExecStart=/usr/bin/zenith-shell daemon`). State `~/.local/state/zenith`, cache `~/.cache/zenith`, config
  `~/.config/zenith`.
- The daemon does not start or supervise Quickshell; systemd does (`zenith-shell.service`). It reaches the
  shell with `quickshell ipc -p /usr/share/zenith/shell call <target> <fn> ...` (`ZENITH_SHELL_DIR` points
  at a checkout's `shell/`). Targets used: `shell` (openSurface, closeSurface, closeAllMenus, keyringPrompt,
  keyringHide, voiceShow/Off/Hide, pluginPopout, sessionConfirm), `theme.reload`, `qsbar.settings`,
  `lifecycle.ready`.
- `reload` = reload cover, `systemctl --user reload zenith-shell`, then wait for a new main pid that answers
  `lifecycle ready`. `status` prints `shell: running|starting|stopped`. No `components`, no `state` verb, no
  `performance.json` `disabledComponents` / unload-when-idle workers (there is no second process to park).
- `bar <edge> <action>` still emits `setBar`, which the shell has no handler for (inherited no-op).
  `bar list|catalog|...` and `dock ...` are unchanged; the dock default is `enabled: true` like `Config.qml`.
- `hub open [page]` runs `zenith-hub` under `flock -n -o $XDG_RUNTIME_DIR/zenith-hub.lock`, navigates with
  `quickshell ipc -p <shell>/hub.qml call nav open|section`, raises the window by title `Zenith Settings`;
  `hub close` = `call hub close`.
- Lock: `lock` starts `hyprlock -c ~/.config/zenith/hyprland/hyprlock.conf` (else the packaged
  `/usr/share/zenith/hyprland/hyprlock.conf`), once; `lock status` is `locked` while a hyprlock of this user
  lives. There is no proof marker, no token, no qylock.
- Sleep guard: two logind *delay* inhibitors (Ryoku held a hard *block*, which would stop `systemctl suspend`,
  the lid and hypridle from suspending at all). On `PrepareForSleep(true)` it locks, then releases both.
  `suspend`, `suspend transaction <token>`, `suspend-cancel`, `sleep-ready` are kept.
- Not available, answered `err <verb>: not available on Zenith: <why>`: `unlock-prepare`, `lock-quiesce`,
  `state`, `lock session <id>`; `browser-host` (no extension) exits 1 with the same message. Dropped with no
  verb: the boot guard marker, stray-shell reaping, the QML import-path export.
- `voice` needs `voxtype` (not in the archive): without it the pill shows its "off" note, as in Ryoku.
- Palette: `zenith theme generate --image I --mode dark|light --scheme S --contrast C --lightness-dark D
  --lightness-light L --source-color-index N --prefer P --json` (matugen-shaped JSON on stdout); templates:
  `zenith theme render --config CFG --palette CARRIER`. The default scheme is `scheme-monochrome`. Template
  configs are read from `~/.config/zenith/shell/matugen/` (else `/usr/share/zenith/shell/matugen/`). Writers:
  `colors.json`, `tones.json` (daemon).
- `updates` topic = verbatim stdout of `zenith status --json` (the CLI owns the apt part), checked at start,
  then per `update_interval` in `~/.config/zenith/hub.toml`.
- `wm` topic: same frame. Capabilities not claimed: `globalShortcuts`, `focusGrab`, `plugins`,
  `nativeOverview`, `overviewBackdrop`, `outputMirror`, `outputHdr`; `nightLight` only while `gammastep` is
  installed (`nightLightProcess` = `gammastep`), `touchpadToggle` only while `zenith-touchpad` is.
  `configFiles` = `["zenith/hyprland/user.conf"]`. Workspace `layout` is `general:layout` unless this daemon
  applied `workspace.layout`. The settings frame's `deadKeys` is always `[]`.
- `wm.act`: same action ids, hyprlang underneath. `input.touchpad` runs `zenith-touchpad`, `output.cycle` runs
  `zenith-monitor toggle`, `window.fullscreen` focuses the window first, `app.focus` is a class regex.
- Night light files, written only by `zenith-cmd-nightlight`: `$XDG_STATE_HOME/zenith-nightlight` (temp),
  `zenith-nightlight-enabled` (marker), `zenith-nightlight-schedule.json` (daemon).
- Sounds come from the installed sound theme (Yaru, then freedesktop), not from embedded clips.
- The polkit agent registers only with `ZENITH_POLKIT_AGENT=1` (the unit sets it); the unit conflicts with
  `hyprpolkitagent.service`. Key presses need read access to `/dev/input`; without it the `keypress` frame
  says `error: Cannot read keyboard input: ...`.
- Weather: no location set = no request of any kind and the frame `status` is `"noLocation"` (not an error,
  no `error` text; QML shows "set a location"). The IP lookup (ipwho.is) happens only for the explicit
  location `auto`; a place name geocodes and coordinates are used as given. Nothing is decided before the first
  `weather.configure`. Holidays fetch only on `calendar.configure`.
- The `keypress` backend is switched off when its last subscriber disconnects.
- Needs from other lanes: `zenith-cmd-nightlight` (W24) on `/usr/bin`, `zenith-dns` (privileged DNS helper,
  no owner yet; `network.dnsSet` errors without it), `zenith-plugins-place` (W15), `zenith-hubd lighting
  apply|accent` (W06, optional).
