# The Zenith GUI map

Zenith has a GUI for almost every setting: the Zenith Hub for system and shell
config, the wallpaper picker for looks, and QS Bar Settings for the bar and the
dock. When a user asks how to change something, name the surface first, then give
the command the page wraps (SKILL.md, "Answer policy: GUI first"). This file is
the lookup from an intent to its surface, its keybind, and the command behind it.

## Opening the Hub

Zenith Hub is the settings app; it replaces the old reference settings window.
Super+comma opens it. To jump straight to a page, deep-link it by its section
key:

```bash
zenith-shell hub open              # the Hub, last page
zenith-shell hub open keybinds     # straight to Keybinds
zenith-shell hub open gpu          # Graphics & Power
```

The keys are the ones in the table below; they are the same strings the Hub's
rail uses, so `zenith-shell hub open <key>` and clicking the rail reach the same
page. Escape closes the Hub.

## The Hub pages

Every row is one page in the Hub rail. `hub open <key>` is shorthand for
`zenith-shell hub open <key>`. Pages marked advanced show only when the Hub's
advanced view is on; gated pages appear only when the active compositor backs
them (see below), never as dead settings.

| Rail name | `hub open` key | Page | What it owns |
|---|---|---|---|
| Profile | `profile` | ProfilePage.qml | the user profile, avatar, and hero |
| General | `global` | GlobalPage.qml | Hub-wide preferences |
| Updates | `updates` | UpdatesPage.qml | system updates and the release channel |
| Displays | `displays` | DisplaysPage.qml | monitors, resolution, arrangement (gated: `monitorConfig`) |
| Connections | `connections` | ConnectionsPage.qml | wifi and bluetooth |
| Input | `input` | InputPage.qml | keyboard, mouse, and touchpad |
| Graphics & Power | `gpu` | GpuPage.qml | GPU mode, power profile, and the idle/lock timers |
| Animations | `animations` | AnimationsPage.qml | compositor animation curves (Hyprland only) |
| Lockscreen | `lockscreen` | LockscreenPage.qml | the qylock theme and lock behaviour |
| Window Manager | `windowmanager` | WindowManagerPage.qml | gaps, borders, tiling (gated: needs a backing provider) |
| Plugins | `plugins` | PluginsPage.qml | compositor plugins (gated: `plugins`) |
| Layer Rules | `layerrules` | LayerRulesPage.qml | layer-shell rules (advanced; shows only with rules present) |
| Bar Studio | `bar-studio` | BarStudioPage.qml | the bar style and its studio |
| Desktop | `desktop` | DesktopPage.qml | the desktop stage and its surfaces |
| Widgets | `widgets` | WidgetsPage.qml | desktop widgets |
| App Launcher | `launcher` | LauncherPage.qml | how the Super+Space launcher behaves |
| Keybinds | `keybinds` | KeybindsPage.qml | every shortcut; the one compositor-specific page kept out of the compositor group |
| App Overrides | `appoverrides` | AppOverridesPage.qml | per-app tweaks (advanced) |
| Window Rules | `windowrules` | WindowRulesPage.qml | window placement rules (advanced) |
| Performance | `performance` | PerformancePage.qml | performance and gaming scripts (Hyprland only) |
| Session | `session` | SessionPage.qml | session and login behaviour |
| Recording | `recording` | RecordingPage.qml | screen recording |
| Dictation | `dictation` | DictationPage.qml | voice typing |
| Fastfetch | `fastfetch` | FastfetchPage.qml | the fastfetch splash (advanced) |
| Import config | `import` | ImportPage.qml | import settings from another config (advanced) |
| Add-ons | `addons` | AddonsPage.qml | store add-ons and extras |
| Agent | `agent` | AgentPage.qml | Zenith Agent: switch on, wire, the vault, the dashboard |
| Credits | `credits` | CreditsPage.qml | credits |

## Surfaces outside the Hub

Wallpaper, theme, the bar, the launcher, and the Stash are not Hub pages. Reach
them here:

| Intent | GUI surface | How to open | The command behind it |
|---|---|---|---|
| Change wallpaper or theme | Wallpaper picker | `zenith-wallpaper wallpaper ui` | `zenith-wallpaper wallpaper ui` (picker), `zenith-wallpaper wallpaper set\|next\|random` |
| Bar layout, widgets, dock | QS Bar Settings | `zenith-shell bar settings [route]` | `zenith-shell bar ...`, `zenith-shell dock ...` (see bar.md) |
| Launch an app | App launcher | Super+Space | the shell launcher (the App Launcher page tunes it) |
| Screen time and downloads | Stash | Super+S | a shell surface; no config command |

QS Bar Settings routes are `bars`, `layout`, `widgets`, `dock`, and `community`,
so `zenith-shell bar settings layout` opens straight to the layout lanes. The bar
also carries a Wallpaper widget that opens the same picker. bar.md covers the bar
and dock model in full.

## Compositor-gated pages

The Hub shows no page for a capability the running compositor lacks. It gates
Displays (`monitorConfig`), Plugins (`plugins`), Window Manager (a backing
provider), and Layer Rules (shown only with rules present); each appears only
when the active compositor supports it. Animations and the performance and gaming
scripts are Hyprland-specific and do nothing under another compositor. Keybinds
is the deliberate exception: it is compositor-specific yet stays a top-level page
rather than sitting in the compositor group. A setting you change for a
compositor applies only after switching to that compositor.

## A worked answer

The user asks: "how do I make the screen lock after ten minutes?" The GUI-first
answer names the page first:

> Open Zenith Hub with Super+comma and go to Graphics & Power > Idle, or jump
> straight there with `zenith-shell hub open gpu`, then set the lock timer to ten
> minutes.

When the user asks YOU to set it, change it on that page's behalf through the
command it wraps, then tell them it now locks after ten minutes and that Graphics &
Power > Idle is where they see or change it. Same shape for wallpaper: point them
at the picker (`zenith-wallpaper wallpaper ui`) first; when you set it yourself,
use `zenith-wallpaper wallpaper set <path>` and tell them where the picker is.
