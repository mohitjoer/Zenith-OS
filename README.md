<p align="center">
  <img src="public/zenith-logo.svg" alt="Zenith OS" width="180">
</p>

# 天 Zenith OS

<p align="center">
  <img src="public/zenith-logo-on-dark.svg" alt="" width="96">
</p>

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
| Desktop shell | Quickshell (QML) — bar, dock, launcher, popouts, widgets, built by Zenith |
| Control tools | Go — `zenith`, `zenith-shell`, `zenith-hub` |
| Settings store | GSettings schemas + dconf, one writer per setting |
| Terminal | Ghostty, from the base layer's archive — configured by Zenith, never rebuilt |
| User shell | zsh + starship |
| Editor | Neovim |
| Boot chain | Limine |
| Login manager | SDDM, in Wayland greeter mode |
| Desktop services | pipewire + wireplumber, power-profiles-daemon, iwd, NetworkManager, fprintd |
| Portals | `xdg-desktop-portal-hyprland` + `-gtk`. **The Secret interface is unsolved** — no backend in the stack implements it |
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

One thing the base layer's reach does not settle is the terminal. It carries Ghostty, but in
`universe` and a version behind upstream, which leaves two honest options: build it, or take
what the archive has. Zenith takes what the archive has. It installs that package as-is rather
than rebuilding it, and accepts the gap in exchange for a terminal that costs nothing to
maintain — no source pin to resolve, no rebuild against each rebase, and no `zenith-terminal`
release to ship when Ghostty patches. A version behind is a gap; a build is a liability that
grows every release cycle, and the gap is the cheaper one. Ghostty is still the most capable
terminal in the stack, and what Zenith ships around it is configuration, not packaging.

Taking the archive's terminal also means the base version is a free parameter again. Ghostty's
own policy is "minimum GTK/Adwaita in the most recent LTS at the time of a minor release", and a
distribution building it would have inherited that as a constraint. Zenith does not build it, so
the floor is the archive's problem and `26.04` is chosen above for one reason only: hardware
reach.

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
desktop/             the control tools: cmd/zenith, and the shell, hub, agent layer
system/hardware/     detection, matching, redaction, reports, profiles and tiers
system/kernel/       pinned kernel fleet
installation/        installer backend and ISO build
release/             .deb packaging, signed APT repository, boundary check
session/             the session entry point, its registration and portal config
docs/                guides and architecture notes
agents/              the agent contract and the design system
tests/               CI checks
```

The desktop source lives under `desktop/`, never in a directory named after one of the
control tools — a `zenith/` directory collides with the built binary of the same name and
silently defeats the repository's own ignore rules.

The four design documents, and what each one decides:

| Document | Decides |
|---|---|
| [`docs/architecture.md`](docs/architecture.md) | The stack and the reasoning behind it — settled decisions, rejected options, the base-layer boundary, the portal seam |
| [`docs/hardware-detection.md`](docs/hardware-detection.md) | The subsystem that decides whether Zenith works on someone else's laptop — probes, matching, PII stripping, tier assignment |
| [`docs/session.md`](docs/session.md) | The boot and login chain — Limine through systemd to Hyprland, the session entry point, what starts in what order, and who owns logout, suspend and shutdown |
| [`docs/adopting-zenith.md`](docs/adopting-zenith.md) | `zenith adopt` — switching an existing GNOME desktop over in place, built around a reversibility contract |

[`docs/roadmap.md`](docs/roadmap.md) orders the work.

Detection is code, and it lives beside its data. `system/hardware/` holds both: the tiers and
profiles the probes resolve against, and the Go that reads them —

```
system/hardware/
  probe/               five sysfs readers, each taking a filesystem root
  match/               pure resolution: candidates, tie-breaks, conflicts
  redact/              the report redaction policy, two layers
  tiers.yaml           the three support tiers, the authority
  profiles/            the machine database, one YAML per machine
```

It lives there rather than under `desktop/` because every byte of hardware-dependent code is
behind a filesystem root, which is what makes the whole subsystem testable without a hardware
lab: `probe` readers take a root, so a captured directory tree is a machine. `TestNoAbsoluteSysfsPath`
walks the AST of package `probe` and fails on any hard-coded `/sys` or `/proc` path, so the
property cannot decay into a review comment. `system/hardware/` is its own Go module and depends
on nothing outside the standard library; `desktop/` is a second module that depends on it.

The commands that reach it:

```console
$ zenith adopt --check                # can this machine switch? changes nothing
$ zenith hardware report              # this machine, redacted, safe to paste
$ zenith hardware report --profile    # a candidate profile to review and submit
$ zenith hardware match               # which profile claims this machine, and at what tier
$ zenith hardware lint-profiles       # check the database
$ zenith hardware capture --out DIR   # write this machine as a test fixture
```

And the overlay model, which is what makes an update safe to automate:

```console
$ zenith status          # revision, paths, how much drift
$ zenith doctor          # drift, plus whether the portal config is really being read
$ zenith materialize     # re-lay the base, clobbering shipped content
$ zenith reset PATH      # drop one of your edits back to the base
$ zenith update          # snapshot, fetch, re-materialize; your edits survive
$ zenith rollback        # re-materialize the previous revision
```

Every command takes `--root DIR`, so the whole surface runs against a temporary tree or a
captured machine as happily as against the live one. Exit code 3 from `match` means no profile
matched — an answer, not a failure.

The one rule the overlay model exists to keep: **a user's override wins at its mirrored path,
and no update can clobber it.**

`zenith adopt --check` is a preflight for moving an existing GNOME desktop over in place. It
needs no root, changes nothing, and exits `0` ready, `1` blocked, `2` ready with warnings — so a
script can tell the three cases apart. `--json` emits the same report machine-readably.

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