# 天 Zenith OS

**untouchable · dominating**

天 — read *ten* — is heaven: the highest point of the sky. A zenith is that same point. The
mark and the name say one thing, and they say it about the architecture as much as the
project: this is the top of the stack, and it sits above everything it is built on.

Nothing reaches up to touch it. That is the standard the project holds itself to, and it is
why the base layer is treated as substrate and never as identity — see
[the base-layer boundary](docs/architecture.md#the-base-layer-boundary).

Zenith is a Linux distribution. Its **base layer** is Ubuntu 26.04 LTS. Everything above that
line is Zenith's own — its compositor configuration, its shell drawn in Quickshell, its control
tools, its hardware matrix, its updater.

Zenith is not an Ubuntu flavour. Ubuntu supplies the substrate and nothing else. Zenith is the
distribution; Ubuntu is what Zenith is built on top of.

> **Status: pre-alpha.** Nothing here is bootable. This repository currently holds the
> design, the licensing, the hardware support framework, and the pinned package and kernel
> manifests. There is no ISO, no installer, and no working control tool. Do not install this
> on anything you care about.

---

## What Zenith is

Most Linux desktops are a distribution's defaults plus whatever you can find. Zenith is a
single coherent opinion: a curated package set, one design language applied everywhere, a
hardware profile for your specific machine, and an updater that can roll back.

| Layer | Choice |
|---|---|
| Base layer | Ubuntu 26.04 LTS — the substrate only |
| Compositor | Hyprland |
| Desktop shell | Quickshell (QML) — bar, dock, launcher, popouts, widgets |
| Control tools | Go — `zenith`, `zenith-shell`, `zenith-hub` |
| Settings store | GSettings schemas + dconf, one writer per setting |
| Terminal | kitty — built from source by Zenith, because the base layer does not package it |
| User shell | zsh + starship |
| Editor | Neovim |
| Boot chain | Limine |
| Login manager | SDDM |
| Desktop services | pipewire + wireplumber, power-profiles-daemon, iwd, NetworkManager, fprintd |
| Portals | `xdg-desktop-portal-hyprland` + `-gtk`, with `gnome-keyring` for secrets |
| Package manager | APT, with a signed first-party repository |
| Agent layer | resident OS agent with a local knowledge vault |

### Why Ubuntu is the base layer

Ubuntu's reason for existing in Zenith is hardware reach, not convenience. Zenith targets laptops,
and the hardest part of that is drivers. Ubuntu's archive has the widest firmware, DKMS and
hardware-enablement coverage of any Linux distribution, and it carries the firmware packages
laptops need, which Zenith references rather than repackages. The base is chosen for hardware
reach.

26.04 LTS specifically: the HWE kernel stream is an archive of its own here, and that stream is
the hardware reach the whole distribution exists for.

The terminal is the deliberate exception, and the cost is stated rather than hidden. The base
layer maintains a terminal for us; it does not package kitty, so Zenith builds kitty from source
and owns that build for the life of the project. That is a real ongoing cost and Zenith pays it on
purpose: kitty is the most capable terminal in the stack, and the terminal is the surface a
developer looks at all day.

### No GNOME

Zenith contains no GNOME — not GNOME Shell, not the GNOME daemons, not the GNOME portal backend.
It draws its own shell on Hyprland and owns the surfaces a stock desktop used to provide:
notifications, global shortcuts, locking, idle handling, and quick settings.

What this costs: Zenith writes those surfaces itself. What it simplifies: there is no
`xdg-desktop-portal-gnome` to convince, and no GNOME daemon stack to fight. See
[`docs/architecture.md`](docs/architecture.md) for the portal arrangement, and
[`agents/agents.md`](agents/agents.md) for the archived GNOME Shell notes.

---

## Hardware support

Zenith publishes support tiers rather than claiming universal support, because no Linux
distribution supports every laptop. Every machine falls into one of these.

| Tier | Meaning |
|---|---|
| **1 — Verified** | Installs, boots, suspend/resume, GPU acceleration, audio and peripherals confirmed by a maintainer. |
| **2 — Community confirmed** | Working on a contributor's machine, with a profile submitted. Not independently verified. |
| **3 — Best effort** | No matching profile. Installs conservative defaults: the `generic` kernel, full firmware bundle. May need manual intervention. |

The `generic` in that row is not prose. It is the same value as `default_for_unknown` in
[`system/kernel/fleet.yaml`](system/kernel/fleet.yaml), which is the authority for which kernel
a machine gets, and where the two disagree that file is right.

The matrix grows through [`system/hardware/`](system/hardware/) — a database of verified
machine profiles in this repository. Nobody can test every laptop, but every user can test
one. A machine that boots successfully emits a candidate profile via `zenith hardware report`;
contributors attach it to a pull request; merged profiles ship in the next ISO and are
selected automatically at install time.

This is the single most important mechanism in Zenith. It turns an unbounded hardware problem
into a union of working setups that grows without anyone owning a hardware lab.

---

## Repository layout

```
desktop/             the desktop stack: shell, hub, agent layer, assets
system/hardware/     machine profiles and support tiers
system/kernel/       pinned kernel fleet
installation/        installer backend and ISO build
release/             .deb packaging, signed APT repository, keyring
docs/                guides and architecture notes
agents/              the agent contract and the design system
tests/               CI checks
```

The desktop source lives under `desktop/`, never in a directory named after one of the
control tools — a `zenith/` directory collides with the built binary of the same name and
silently defeats the repository's own ignore rules.

Detection is not a directory. It is `zenith hardware` code that does not exist yet;
`system/hardware/` holds the data those probes will read and the tiers they resolve against.

## Branches

| Branch | Holds |
|---|---|
| `main` | Documentation. Design, decisions, hardware framework, project rules. Always reviewable. |
| `dev-unstable` | Code. The working implementation, merged back into `main` when a milestone completes. |

Documentation changes land on `main` directly. Implementation lands on `dev-unstable`. A
milestone is done when its code has merged forward, not when it is written.

Only the documentation framework exists today. See [`docs/roadmap.md`](docs/roadmap.md) for
what lands in what order.

---

## Design system

Zenith's visual language is monochrome by construction: black, grey and white, with accent
colour never carrying meaning. The rules live in [`agents/agents.md`](agents/agents.md) and
are binding, not advisory:

- Strict monochrome. Traffic lights and destructive/warning/error states are the only
  exceptions, because they carry meaning a grey cannot.
- A 6/7/8/10/12/14/18/22px radius scale, a 4px spacing grid, and two-stop elevation. One
  shadow reads as a flat sticker.
- Dark and light variants declare the same properties in the same order and differ only in
  token values.
- Contrast floor: 4.5:1 for body ink, 3:1 for UI shapes and indicators.

Aesthetic character comes from material, shape and rhythm — never from hue.

---

## Contributing

Zenith is open source and contributions are the point. **Hardware profiles are the most valuable
contribution and the easiest for a new person to make** — you need one machine and no code.

- [`CONTRIBUTING.md`](CONTRIBUTING.md) — how to contribute, including hardware profiles
- [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) — the behaviour expected of everyone
- [`SECURITY.md`](SECURITY.md) — reporting a vulnerability privately
- [`agents/agents.md`](agents/agents.md) — the project rules and the design system

Contributions are made under the Developer Certificate of Origin.

---

## Licence

Zenith is free software, but not under one licence, and not entirely free of proprietary code on
disk. Both facts matter, so both are documented:

- Zenith's own code and Zenith's own repository are **GPL-3.0** and entirely free.
- Some laptop hardware needs proprietary firmware. Zenith references the base layer's
  `linux-firmware-*` packages rather than redistributing firmware itself, and declares per
  machine what is needed.

[`LICENSING.md`](LICENSING.md) has the full split. [`LICENSE`](LICENSE) is the GNU General
Public License v3.0.