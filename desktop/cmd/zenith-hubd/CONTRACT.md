# zenith-hubd contract

Every verb and shape that differs from Ryoku's `ryoku-hub`, or is dropped. Anything not listed
here is Ryoku's, unchanged. W08/W09 read this; verbs a page calls but this list does not mention
exist as in Ryoku.

## Dropped or unavailable

A read answers `{"available":false,"reason":"..."}` and exits 0; a verb that would change
something prints `zenith-hubd: <verb>: not available: <reason>` and exits 1. Pages must show the
reason, not an empty state.

- `voxtype *`: no dictation engine on Ubuntu. The Dictation page (W09) shows "not available".
- `gpu mux|apply|hook|vm`: the MUX helper and the GPU-passthrough setup are dropped (Looking Glass
  and kvmfr are not in the archive; the setup edits boot configuration).
- `gpu caps|mode|tune`: unavailable while `zenith-gpu` is not on PATH (W18's helper;
  `zenith-gpu mode` must write `env = AQ_DRM_DEVICES,...` to
  `~/.config/zenith/hyprland/gpu.conf`, which is where `gpu mode get` reads it).
- `cpu caps|active|set`: unavailable while `zenith-power` is not on PATH (it is not built). `cpu
  switch` goes through the daemon and does not need it. No tunable with `gpu == "idle"` is ever
  returned (hypridle drives idle from the `org.zenith.desktop` keys) and `cpu set idle ...` is
  refused, so the GPU page shows no IDLE sections.
- `rice publish`: dropped (online sharing). `share status`, `rice list|capture|apply|...` stay.
- QMK/VIA lighting provider: dropped (`qmk_hid` is not packaged). OpenRGB and Aura stay.
- `wm preview <name>`: always an error. `wm list` has exactly one row (`hyprland`, active) and
  no `package`/`available`/`reclaim`: W08 hides the compositor switch.
- `desktop plugins rebuild|add|remove`: exit 1. Plugins are the packaged `hyprland-plugin-*`.
- `lock apply-greeter`: dropped (Zenith leaves the SDDM configuration alone). `lock list|set`
  keep Ryoku's qylock-skin shape but nothing on Zenith reads the preference (hyprlock locks);
  W10 should not offer it.

## Shapes that differ

- `desktop plugins list` is `{available, compositor:{version,commit}, toolchain:{ok:false,missing},
  plugins:[{id,name,bundled,installed,loaded,enabled,status,source,package,version,path,docs,desc,
  detail,rebuildable:false,removable:false}]}`, `status` is `running|ready|missing`. Only
  hyprbars, hyprfocus and hyprscrolling exist; the glass, image-border, dynamic-cursor and
  key-sound blocks stay in the store, are reported by `desktop unhonored`, and their 22 rows are
  gone from `desktop schema` (87 rows left).
- `desktop unhonored` also names: glow (Hyprland 0.53 has none), a scrolling layout or a plugin
  that is switched on but not installed, and any switched-on unpackaged plugin.
- `keybinds` rows come from `session/binds.json` (override with `ZENITH_BINDS_JSON`) joined with
  `hyprctl binds -j`. `unhonored` is "Not bound in the running session." when live and the chord
  is not bound; live binds outside the catalogue are listed under "Hyprland", store binds under
  "Custom". `locked` is any `XF86` chord; `rebindable` excludes locked and mouse chords.
- `desktop caps` (`wm list`) `generatedFiles` lists the live files and their `user_edits` copies.
- `desktop anim-preset [name]` ids: `zenith` (default, writes nothing), dusky, bounce, fade, fast,
  mechanical, minimal, rage, slowmotion, air, hallucination, exaggerated, disable. Setting one
  regenerates `settings.conf`. `ryoku` is now `zenith`.
- `desktop defaults`: borders `#e6e6e6`/`#3a3a3a`, cursor `Adwaita`; the `DYNAMIC` cursor
  resolves to `Adwaita` (no recolouring package). The numeric defaults are Ryoku's: they must
  match the packaged `hyprland.conf`, since only values that differ from them are written.
  The other brand fallbacks are monochrome too: fastfetch accent `230;230;230`, lighting `#E6E6E6`.
- `rice apply` sends the shell look to the daemon (`settings.patch`, one call per top-level key)
  and writes `shell.json` itself only when no daemon answers.
- `outputs apply` writes `~/.config/zenith/outputs-applied.json`, regenerates `settings.conf`
  (which carries the `monitor =` lines) and pushes live with `keyword monitor`; scales are snapped
  to what Hyprland accepts. The report is `{provider, written}`.
- `import apply` puts what the Hub cannot model into `~/.config/zenith/hyprland/user.conf` (and its
  `user_edits` copy) as a `# >>> zenith-import` block of native hyprlang, checked with
  `Hyprland --verify-config`. A bind Hyprland would reject is kept as a comment and listed in
  `unresolved`. Conflicts are checked against the catalogue.
- `desktop matugen set` persists knobs only (as in Ryoku). `desktop scheme|theme-apps|gtk-theme|
  gnome-accent|zenith-theme` render app templates through `runMatugen()`, which still execs
  `matugen -c CFG json CARRIER`; W04 must provide that argv or name the replacement.

## Files

- Writes: `~/.config/zenith/hyprland/{settings.conf,rebinds.conf}` plus
  `~/.config/zenith/user_edits/hyprland/` copies; `hyprland.conf` must source both, then `user.conf`.
  Also `desktop.json`, `hub.toml`, `anim-preset`, `outputs-applied.json`, `output-profiles/`.
- Reads: `shell.json` (changed only through the daemon), `theme.json`, `$XDG_RUNTIME_DIR/zenith-shell.sock`
  (`clipboard stats|prune`), `/usr/share/zenith/hyprland/hyprland.conf` (`ZENITH_HYPRLAND_CONFIG`)
  to learn what a shipped bind does when it is rebound.
- Never runs `hyprctl reload` off a live session: every `hyprctl` call is skipped with no socket.
