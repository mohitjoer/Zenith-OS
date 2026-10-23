# NOTICE

The desktop shell in `shell/` is a port of Ryoku (https://github.com/ryoku-dev/ryoku),
which is GPL-3.0. Zenith is GPL-3.0 as well, so the licence is compatible, and this
notice is the honest ledge for where the code came from.

- Source: Ryoku, under GPL-3.0.
- What came across: the whole quickshell shell tree (the bar's frame, the dock,
  the launcher, the desktop, the wallpaper engine, the services, the ui kit).
- What did not come across: Ryoku's name, its redistributable installers, its
  usage dashboards, and its welcome/pin/screenshot client configs.
- Branding in the running desktop says "Zenith". The library names inside the
  tree are the ones the imports resolve through.
- First-party audio and image assets were not copied. The fonts and wallpapers
  in `shell/assets/` and the splash images are Ryoku's generic component assets
  rescinded for the transitional state; please replace them with Zenith's own
  fonts and wallpapers at the earliest packaging opportunity.

## The second port: Ryoku 0.75.5-beta.20

The paragraphs above describe the first port, the shell tree. A second port brought
across the processes, windows and configuration around that tree. Where the two
differ, this section is current: the welcome tour, the pin, the reload cover, the
screenshot tool (without its beautify editor) and the AI-usage collectors did come
across, so the "did not come across" line above no longer holds for them.

- Taken from: Ryoku commit `27b4e706b3eba9196734fa362392cdcf3f551ef3`, `VERSION`
  0.75.5-beta.20, codename "Onogoro". Ryoku is GPL-3.0, and so is everything below
  that came from it, except the portions named under "Third-party code".
- How: `tools/port.py SRC DST` copies text files from a Ryoku checkout, applies the
  rename rules (Ryoku's names and QML module names to Zenith's), never overwrites a
  file, skips binary assets, leaves any line that carries a copyright, SPDX or credit
  marker untouched, and prints every mention of the old name it left. Go imports and
  everything Arch-specific were then fixed by hand. A re-port is that one command
  against a newer checkout, followed by the same hand repair.
- Attribution: files keep the copyright, SPDX and "ported from" lines they came
  with or were given. Where a port differs from Ryoku on purpose, the file says so,
  and `desktop/cmd/zenith-shell/CONTRACT.md` and `desktop/cmd/zenith-hubd/CONTRACT.md`
  list every difference in the wire and the verbs.

### What came across

| In this tree | From Ryoku | Notes |
|---|---|---|
| `shell/shell.qml`, `Services/`, `Ui/`, `components/`, `utils/`, `Barkit/`, `FrameBars/`, `modules/` (bar, capture, clipboard, confirm, desktop, dock, launcher, notifications, osd, overview, stage, visualizer, wallpaper) | `ryoku/shell/quickshell/shell/`, `ryoku/ui/`, `ryoku/shell/framebars/` | The first port, adapted since |
| `shell/Blobs-src/` | `ryoku/shell/plugin/` | The frame-blob renderer: C++ sources and shaders |
| `shell/PluginKit/`, `shell/plugins/`, `shell/scripts/zenith-plugins-place` | `ryoku/shell/quickshell/plugins/` | The plugin kit, runtime and placement script |
| `shell/hub.qml`, `shell/hub/` | `ryoku/hub/quickshell/` | Zenith Settings. `Zenith.qml`, `ZenithPage.qml`, `PageView.qml`, `Editor.qml`, `GVariant.qml` and `Controls/` are Zenith's own |
| `shell/shot.qml`, `shell/shot/` | `ryoku/shell/quickshell/ryoshot/` | The screenshot tool, without the beautify editor |
| `shell/pin.qml`, `shell/welcome.qml`, `shell/reload-cover.qml` and the directories of the same names | `ryoku/shell/quickshell/ryopin/`, `welcome/`, `reload-cover/` | Pinned shots, the first-run tour, the restart curtain |
| `shell/store.qml`, `shell/store/` | `ryoku/apps/ryostore/quickshell/` | The store window |
| `shell/vm.qml`, `shell/vm/` | `ryoku/apps/ryovm/quickshell/` | The VM manager window |
| `shell/wallpaper-ui.qml`, `shell/wallpaper-ui/` | `ryoku/shell/ryogami/wall-ui/` | The wallpaper picker (skwd-wall's, below) |
| `shell/agent/skills/` | `ryoku/rashin/skills/ryoku/` | Skills shipped to the agent's engine |
| `shell/matugen/` | `ryoku/shell/matugen/` | Templates only; those for apps Zenith does not ship are gone |
| `shell/scripts/`, `shell/bin/` | `ryoku/shell/scripts/`, `ryoku/shell/bin/` | Helper scripts and the three usage collectors |
| `shell/i18n/data/`, `shell/i18n/overrides/` | `ryoku/i18n/catalog/`, `catalog/overrides/`, `langs.json` | Ryoku's machine translations (34 languages besides English), rebranded. Its extractor, translator and runtimes did not come across |
| `desktop/cmd/zenith-shell/`, `desktop/internal/wm/` | `ryoku/shell/ipc/`, `ryoku/wm/` | The shell daemon and its CLI (`ryoku-shell`), and the compositor seam, Hyprland only |
| `desktop/cmd/zenith-hubd/` | `ryoku/hub/backend/`, and the Hyprland provider and `settings.lua` generator of `ryoku/wm/` | The Hub's data plane (`ryoku-hub`); compositor settings are written as hyprlang |
| `desktop/cmd/zenith-wallpaper/` | `ryoku/shell/ryogami/daemon/` | The wallpaper daemon (`ryogami`) |
| `desktop/cmd/zenith-agent/` | `ryoku/rashin/backend/` | Zenith Agent (Rashin). Its resident engine is not part of it; the engine is the user's own (`docs/agent.md`) |
| `desktop/cmd/zenith-store/` | `ryoku/apps/ryostore/backend/` | apt only; no remote catalogue |
| `desktop/cmd/zenith-vm/` | `ryoku/apps/ryovm/` (`bin/ryovm`, `mon/`, `remote/`) | `zenith-vmd`, `zenith-vm-mon`, `zenith-vm-ssh` |
| `desktop/cmd/zenith/plugin*.go`, `plugin_template/`, `wm.go` | `ryoku/cli/` | `zenith plugin` and `zenith wm`. `icons.go`, `status_live.go` and `reconcile.go` follow Ryoku's lookup order, field names and doctor pattern |
| `desktop/internal/theme/scheme.go` | `ryoku/shell/ipc/matugen.go` | `shellRoleKeys` and `baseSixteen`. The generator itself is Zenith's: it replaces the matugen binary and is not matugen's arithmetic |
| `desktop/internal/backlight/` tests | `system/hardware/display/ryoku-hw-backlight` | The panel-selection cases |
| `session/hyprland/*.conf` | `ryoku/hyprland/` (`hyprland.lua`, `modules/`, `keyboard.lua`, `monitors.lua`, `shaders/`) | Translated from Lua to hyprlang for Hyprland 0.53; the shaders are copied |
| `session/scripts/zenith-workspace`, `zenith-monitor` | `ryoku/hyprland/scripts/ryoku-workspace`, `ryoku-monitor` | |
| `session/units/zenithd.service`, `zenith-wallpaper.service`, `zenith-idle.service`, `zenith-agent.service`, `zenith-ai-usage.*` | `ryoku/shell/systemd/user/`, `ryoku/rashin/systemd/` | From `ryoku-shell`, `ryogami`, `ryoku-idle`, `ryoku-rashin` and `ryoku-ai-usage` |
| `apps/` | `ryoku/apps/` (`ghostty`, `zsh`, `starship`, `wireplumber`, `mimeapps.list`) | Adapted to ghostty, zsh and starship as the defaults |
| `system/helpers/` | `system/hardware/audio/`, `system/hardware/display/`, `system/extras/ryoku-pkg-add` | `zenith-volume`, `zenith-restart-audio`, `zenith-hw-backlight`, and `zenith-pkg-add`/`-remove` rewritten for apt and polkit. `zenith-restart-audio` says it is itself ported from `omarchy-restart-audio` |
| `system/helpers/tools/tests/`, `shell/reload-cover/reload_cover_test.sh`, `session/scripts/monitor_test.sh` | `tests/` (`calc-eval.sh`, `radio-engine.sh`, `game-mode.sh`, `stash-install.sh`, `reload-cover.sh`, `monitor-persist.sh`) | Hermetic script tests, rewritten where they assumed Arch. The other tests in those directories are Zenith's own |
| `docs/plugins.md` | `docs/plugins.md` | The plugin author's guide |

The vendored `shell/modules/common/`, `shell/modules/ii/` and `shell/services/` and the
shapes kit are end-4's, not Ryoku's. They are recorded in `shell/vendor/manifest.yaml`,
`shell/vendor/notices.md` and `LICENSING.md`.

### What did not come across

- Ryoku's name, brand art, decors and wallpapers (`ryoku/assets/`), and its Hub and
  dashboard fonts (`ryoku/hub/quickshell/fonts/`, `ryoku/rashin/backend/web/fonts/`).
- Ryoku's installer (`installation/` and `ryoku-shell-installer/` in its repository)
  and the Arch, AUR and pacman paths.
- niri: the provider, `ryoku/niri/`, and the Lua configuration itself, which was
  translated rather than copied.
- The Hyprland C++ plugins (the key-sounds plugin among them), the lock skins
  (`ryoku/lockscreen/`, qylock), the browser extension and the palette bridge.
- `system/hardware/` tuning: the udev rules, NetworkManager, Bluetooth, GPU, power and
  input drivers and rules. This is deliberate. The hardware profile system owns
  machine-specific tuning, and the base-layer boundary forbids those paths.
- Features with no backend in the Ubuntu archive, or that Zenith decided against:
  wallpaper upscaling (`waifu2x-ncnn-vulkan`), the C live-wallpaper player, rice
  publishing, voxtype dictation, GPU mux and passthrough, QMK lighting, the store's
  remote catalogue, the VM parallel downloader, the screenshot beautify editor and
  Rashin's resident engine.
- Image and font files from the first port that came from Ryoku's tree and are not
  Zenith's, and whose licences this file does not establish: the fonts in
  `shell/assets/fonts/`, the SVGs in `shell/Ui/art/`,
  `shell/modules/launcher/shared/art/hands-adam.png` and the symbolic icons in
  `shell/components/icons/`. Check each before redistributing.

### Third-party code

Ryoku's own NOTICE credits these, and the files named still carry their credit.

- Caelestia, https://github.com/caelestia-dots/shell, GPL-3.0: the frame-blob
  renderer (`shell/Blobs-src/`) and parts of the animation and motion curves
  (`shell/Ui/Anim.qml`, `shell/Ui/Singletons/Tokens.qml`).
- OkShell, https://github.com/JohnOberhauser/OkShell/tree/87a9d09d923163cd03f396a395be3bb02b335b6e,
  by John Oberhauser, GPL-3.0: the configurable frame-bar layout, the rail and widget
  catalogue organisation, the Bar Studio workflow (`shell/hub/barstudio/`) and the
  OkShell launcher variant (`shell/modules/launcher/variants/okshell/`).
- DankMaterialShell, https://github.com/AvengeMedia/DankMaterialShell, MIT, Copyright
  (c) 2025 Avenge Media LLC: portions of the display configuration UI (the Hub's
  Displays page, `shell/hub/pages/DisplaysPage.qml`).
- skwd-wall, https://github.com/liixini/skwd-wall, MIT, Copyright (c) 2026 liixini:
  the wallpaper picker (`shell/wallpaper-ui/`, which carries its own `LICENSE`), the 39
  shader transitions in `shell/modules/wallpaper/skwd/`, and the Go port of its
  daemon's modules in `desktop/cmd/zenith-wallpaper/`.
- qsbar, by HANCORE-linux, MIT, Copyright (c) 2026 HANCORE-linux:
  `shell/modules/bar/barstyles/qsbar/`, which carries its own `LICENSE`. Parts of it
  are adapted from other projects and say so in comments (Shibumi-Shell's
  `SearchEngine.js`, `omarchy-agent-usage`).
- Omarchy, https://github.com/basecamp/omarchy, by David Heinemeier Hansson and
  contributors: Ryoku's alpha series was a fork of it, and Ryoku records that no
  Omarchy code remains in its beta tree. A few ported files still name it as the
  source of an idea or a script, and keep that credit.

The Hub's Credits page lists every project Ryoku credits, including those whose code
did not come across.

The MIT-licensed portions above are under this licence, each with its own copyright line:

```text
Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
