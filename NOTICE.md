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
