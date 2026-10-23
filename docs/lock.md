# The lock screen

hyprlock locks the session. That is a decision, not a fallback: it is mature, it
is in the Ubuntu archive (0.9.2), and it runs as a process of its own, apart from
the shell. The shell, hypridle and the `zenith-shell` daemon only decide when it
runs.

| Piece | Where | Job |
|---|---|---|
| hyprlock | `session/hyprland/hyprlock.conf` | the look: blurred wallpaper, clock, date, password field, battery, now playing |
| hypridle | `session/hyprland/hypridle.conf` | the idle stages (dim, lock, screen off) and locking before sleep |
| `zenith-idle` | `session/zenith-idle`, `session/units/zenith-idle.service` | runs hypridle with the timeouts |
| the shell | `shell/Idle.qml`, `shell/IdleOverlay.qml` | draws the dim scrim; keeps the lock's wallpaper link current |
| Lockscreen page | `shell/hub/pages/LockscreenPage.qml` | says what is missing; locks now |

## Timeouts

`org.zenith.desktop` holds `idle-dim-seconds`, `idle-blank-seconds` and
`idle-lock-seconds`; 0 means never. hypridle cannot read GSettings, so
`zenith-idle start` reads them with `gsettings` and exports `ZENITH_IDLE_DIM`,
`ZENITH_IDLE_BLANK` and `ZENITH_IDLE_LOCK`, leaving a variable unset for a 0.
`hypridle.conf` wraps each listener in `# hyprlang if VAR`, so an unset variable
drops its stage. There is no generated file.

A write through `zenith config set` calls the shell's `reloadSettings`, which runs
`zenith-idle apply`: it restarts the service only when a timeout differs from the
one it started with. A change made by hand takes effect at the next
`zenith-idle apply` or login.

## One clock

hypridle is the only idle clock. It respects D-Bus and systemd inhibitors (a
browser playing a video, Keep Awake) as well as Wayland ones. The dim stage does
not touch the backlight: it writes `dim` to `$XDG_RUNTIME_DIR/zenith-idle`, the
shell watches that file and draws the scrim, and `awake` on activity clears it.
The screen-off stage is `hyprctl dispatch dpms off`, so there is no blank scrim.

## The lock command

`zenith-shell lock` is what every trigger runs (the bind, hypridle's lock stage,
`loginctl lock-session`, before sleep). It must:

1. do nothing, successfully, when this user's hyprlock is already running;
2. run `hyprlock -c CONF`, with CONF the first readable of
   `~/.config/zenith/hyprland/hyprlock.conf` and
   `/usr/share/zenith/hyprland/hyprlock.conf` (hyprlock exits without locking
   when its config is missing, as a bare `hyprlock` does with no
   `~/.config/hypr/hyprlock.conf`);
3. work when the daemon is not running, since hypridle calls it directly;
4. fail loudly when hyprlock is not installed.

## Choices and limits

- `hypridle` 0.1.7 aborts when given `-c` unless a config also exists on its
  default search path (fixed in 0.1.8). `zenith-idle` puts a link to the real one
  on `XDG_CONFIG_DIRS` for it.
- `inhibit_sleep = 3` makes the machine wait for the lock before it suspends. If
  hyprlock never starts, logind suspends after its delay anyway.
- hyprlock's own fingerprint support is left off. Ubuntu's `libpam-fprintd` puts
  the reader in the PAM stack, and enabling both would have each ask for it.
- The lock background is a link at `~/.config/zenith/current/background`, kept at
  the current wallpaper by `shell/Idle.qml`. A video wallpaper keeps the last
  image.
- There is no suspend stage: `org.zenith.desktop` has no key for it. Adding
  `idle-suspend-seconds` (schema and registry page) and one more `# hyprlang if`
  block in `hypridle.conf` that runs `zenith-shell suspend` is all it takes.
- Session confirmations (logout, reboot, shutdown) stay as they are in
  `shell/modules/confirm/`; nothing offers suspend or hibernate to confirm.
