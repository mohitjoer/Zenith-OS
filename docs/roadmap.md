# Roadmap

Milestones in order. Each one is bootable or testable on its own — a milestone that runs
on a developer machine is finished — and none of them depends on work that has not landed.
The order is deliberate: hardware reach comes early because it decides whether Zenith is
usable by anyone but its author.

---

## M1 — Foundations

*Status: complete in `main`.*

Repository scaffold, licensing, the design system, and the architecture record.
Documentation only, no working code. The point of doing this first is that every later
decision has somewhere to be written down.

- [x] Repo layout, `.gitignore`, and the placeholder trees — `docs/journal/`, `installation/`,
  `release/`, `tests/`
- [x] `LICENSE` (GPL-3.0), `LICENSING.md` covering the three-way split
- [x] `AGENTS.md` + `agents/agents.md` — the agent contract and design system
- [x] `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`, the `.github/` issue forms and
  pull request template, and the markdown gate
- [x] `docs/architecture.md` — decisions, rationale, rejected options
- [x] `docs/adopting-zenith.md` and `docs/hardware-detection.md` — the session path and the
  detection subsystem, decided before either is written as code
- [x] `system/hardware/` — support tiers, the profile schema and an annotated example profile
- [x] `system/kernel/fleet.yaml` and `system/packages.yaml` — the pinned kernel fleet and the
  reviewed package manifest, GNOME exclusions included

## M2 — The `zenith` control tool

The first working code. Go, because the tools must be static binaries that start instantly.

- `zenith update` / `rollback` / `materialize` / `reset` / `doctor` — the command surface
  `docs/architecture.md` defines
- `zenith status` — current revision, resolved profile and overlay state at a glance. New in
  this milestone; the command table in `docs/architecture.md` has no row for it yet
- The overlay model: base in `/usr/share/zenith/`, user overlay in `~/.config/zenith/user_edits/`
- A git snapshot of `~/.config` before every update

Exit criterion: on a stock image of the base layer, `zenith update` lays down a base and an
overlay, a user edit survives the next update, and `zenith rollback` restores the previous
revision.

## M3 — Hardware detection

The subsystem that decides whether Zenith works on someone else's laptop.

- Detection: DMI, PCI, USB, ACPI, EDID, network chips, GPU topology
- Profile matching with a defined tie-break when several match, and a reported conflict
  when the tie-break cannot decide
- `zenith hardware report` — emit a candidate profile, with PII stripped
- `zenith hardware match` — report tier and the profile that matched. Also new in this
  milestone; `docs/architecture.md` names only `report`

Exit criterion: detection correctly identifies GPU, WiFi, audio codec and storage on a real
laptop and produces a candidate profile that a human can read and confirm.

## M4 — Bootable ISO

The first thing anyone can install.

- `live-build` for the ISO, `mmdebstrap` for the rootfs seed
- Hardware detection in the installer, profile selection, tier display
- Limine on a separate ESP, Secure Boot via shim + MOK
- The offline firmware path: a broad rescue bundle plus `zenith-firmware load`

Exit criterion: a Zenith ISO installs on a clean machine with no network, and the installed
image boots into the base layer's stock Hyprland session entry. Zenith's own session — SDDM,
`zenith-session`, the portal stack — is M5's, and the shell is M6's.

## M5 — Session and portals

Before any shell work, because it validates the base-layer bet.

- The portal stack, four components and no more: `xdg-desktop-portal` as the dispatcher,
  `xdg-desktop-portal-hyprland` for ScreenCast and Screenshot, `xdg-desktop-portal-gtk`
  for FileChooser, OpenURI, Print and Secret, and `gnome-keyring` behind Secret
- The two interfaces no backend provides, which makes them Zenith's own work: global
  shortcuts, as compositor bindings here and as the shell's own in M6, and the
  notification server, which the shell hosts once it exists
- `zenith-session` — a small `/bin/sh` entry point that sets the session environment,
  imports it into dbus activation and `exec`s Hyprland, so whatever fails before the
  compositor puts the user back at the login screen rather than on a black one
- Session wiring: SDDM → `zenith-session` → Hyprland, with the shell's unit ordered after
  the portals
- Verified under screen sharing, screenshot, camera access, the GTK file chooser and a
  secret round-trip

Exit criterion: an app that has never heard of Zenith behaves identically on Zenith and on a
stock GNOME session of the base layer.

## M6 — The shell

- Quickshell bar and dock, layout driven by `org.zenith.shell` schemas
- `zenith-shell` as the only writer of those schemas
- Launcher, popouts, widget catalogue
- The surfaces a stock desktop used to own: notification server, lock, idle screen,
  quick settings, status icons

## M7 — Zenith Hub

- `zenith-hub`, the settings application: a Go binary like `zenith` and `zenith-shell`, window
  drawn in Quickshell (QML) — one UI toolkit and one design system on the desktop, not two
- Sole writer of `org.zenith.desktop`
- Every setting that has a command also has a page, and vice versa

## M8 — Repository and release

- Signed APT repository, `signed-by` in `sources.list.d`
- Reproducible builds: pinned `SOURCE_DATE_EPOCH`, buildinfo diffed in CI
- Signed ISO checksums

## M9 — Adoption

Switching an existing GNOME desktop on the base layer over to the Zenith shell, in place,
without reinstalling and without touching the user's data. Designed in
[`adopting-zenith.md`](adopting-zenith.md).

This is also the project's growth path: it is how someone decides whether they want Zenith
before they commit to installing it. That is worth more than a landing page.

- `zenith adopt --check` — preflight. Reports what would change and changes nothing
- `zenith adopt` — the least-invasive step: install a Zenith session so it appears at the existing
  login screen. Reversible by picking the other session
- `zenith adopt --verify` — confirm login, apps, audio, displays, suspend, screen sharing, file
  choosers, printing, notifications and shortcuts
- `zenith adopt --prune` — opt-in, dry-run by default, lists every package it would remove
- `zenith adopt --undo` — return to a working GNOME desktop

Design constraints that are not negotiable: the first step is reversible, nothing is removed
unless asked, and no failure mode may leave a machine unbootable.

## M10 — Agent layer

The resident OS agent: a local knowledge vault, an index over the machine, and a dashboard.
Distro-independent by construction.

---

## Later, unsequenced

- Plugin mechanism and a package index for the shell
- `zenith benchmark` so performance regressions are visible rather than anecdotal
- A second base release, once hardware profiles justify it