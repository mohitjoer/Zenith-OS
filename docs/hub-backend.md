# The Hub backend

`zenith-hubd` is the data plane of Zenith Settings. The Hub's QML never edits a file: it runs
`zenith-hubd <verb>`, which prints JSON or changes one store, and that is the only writer of
what it edits. It is ported from Ryoku's `ryoku-hub` with the same verbs and the same shapes;
`desktop/cmd/zenith-hubd/CONTRACT.md` lists every place that differs and every verb that is
dropped, and is what the Hub pages are written against.

| Piece | Where | Job |
|---|---|---|
| the verbs | `desktop/cmd/zenith-hubd/*.go` | one file per area: `desktop`, `outputs`, `keybinds`, `rice`, `import`, `lighting`, `gpu`, `cpu`, `fastfetch`, `profile`, ... |
| the compositor seam | `desktop/cmd/zenith-hubd/internal/wm` | what the compositor can do, the neutral store, and the hyprlang it is rendered into |
| the stores | `~/.config/zenith/` | `desktop.json` (window and input settings), `hub.toml` (Hub state), `outputs-applied.json`, `anim-preset` |
| the output | `~/.config/zenith/hyprland/` | `settings.conf` and `rebinds.conf`, generated; `user.conf`, the user's, wins |

## From a setting to the compositor

Ryoku ran a provider binary per compositor and rendered Lua. Zenith runs Hyprland 0.53 only, so
the provider is the `wm` package, called in process, and it renders hyprlang. A save writes
`desktop.json`, renders it to `settings.conf` and `rebinds.conf`, mirrors both into
`user_edits/hyprland/` so `zenith materialize` cannot replace them with the packaged seeds, and
reloads. A slider drag renders the same options as `hyprctl keyword` calls and writes nothing.

Only what differs from the defaults is written. The defaults table (`store.go`) mirrors the
packaged `hyprland.conf`, which keeps an untouched setting falling through to the packaged
value; when a packaged value changes, that table changes with it.

`settings.conf` is checked against the compositor's own parser, because Hyprland refuses a
whole file for one unknown option: the test runs `Hyprland --verify-config` over a file that
sets every option the generator knows. What 0.53 has no option for (window glow) is not
written and is listed by `desktop unhonored`.

## Rebinds

Hyprland has no "move this bind to another chord". A rebind is an `unbind` of the shipped
chord and the same bind on the new one, so the backend has to know what the shipped bind does.
It reads that from the packaged `hyprland.conf` (the file that says what the keys do) and falls
back to the words in `session/binds.json` when that file cannot be read. The legend the Hub
and the cheat sheet draw is `session/binds.json` joined with `hyprctl binds -j`.

## Safety

No verb touches the compositor without a live session: with no socket, `hyprctl` is not run, so
a save from a TTY or a test cannot reload anything. The suite sets its own runtime and config
directories (`testmain_test.go`) and cannot reach the real shell daemon or compositor from
whatever environment it is started in.

A verb with no backend on Ubuntu (dictation, GPU passthrough, the MUX switch, QMK lighting)
answers `{"available":false,"reason":"..."}` instead of erroring, so a page can say what is
missing. Calls out to `zenith-power` and `zenith-gpu` are detected the same way.
