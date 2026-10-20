# Licensing in Zenith OS

Zenith is not licensed under a single licence, and it would be misleading to say that it is.
Zenith is a distribution: it ships its own code, plus software from its base layer, plus
firmware that is not entirely free. This document explains the split so the obligations are
clear before you build on Zenith.

## Summary

| What | Licence | Where it is recorded |
|---|---|---|
| Zenith's own code | GPL-3.0 | [`LICENSE`](LICENSE) |
| The base layer and every Debian/Ubuntu package | whatever each package declares | `/usr/share/doc/<pkg>/copyright` on an installed system |
| The shell's engine, built by Zenith | LGPL-3.0-only | the built package's own copyright file; see below |
| Firmware blobs | mixed: most free, some proprietary, each separately licensed | per-package copyright files; see below |
| Hardware profiles in `system/hardware/profiles/` | GPL-3.0 by convention, but data is data | the profile's own `license:` field |

## Zenith's own code

Everything authored for Zenith is GPL-3.0. This is deliberate: the base is GPL, GPL-3.0 is
what every serious Linux distribution uses, and it guarantees that anyone receiving Zenith can
modify and redistribute the parts that are Zenith's own, forever.

Contributions are made under the Developer Certificate of Origin. Commit sign-off makes the
licence unambiguous and is the only contribution requirement.

### Historical code

This repository was formerly `zeo-widgets`, a collection of GNOME Shell extensions. That
code remains in the Git history before the project became Zenith OS and is not part of Zenith. It
was GPL-3.0 and it stays GPL-3.0 in history — relicensing history is not possible and is not
attempted.

Zenith replaces GNOME Shell with Hyprland and a Quickshell-drawn shell, so GNOME Shell
extensions are not a Zenith extension mechanism. The GNOME Shell internals those extensions
depended on are archived in [`agents/gnome-shell-notes.md`](agents/gnome-shell-notes.md) as
reference material only.

## The base layer and its packages

Ubuntu supplies Zenith's base layer. Its packages are individually licensed — mostly GPL-family,
some LGPL, some Apache-2.0, some BSD, some public domain — and each ships its own copyright
file. Zenith neither relicenses nor overrides them, and by policy never modifies them in place.

Redistributing Zenith means redistributing that base layer, so both the per-package licences and
Ubuntu's own terms of service apply. This is not optional legal fine print; it is the most
common way a distribution goes wrong.

## The shell is LGPL, and the binary is shipped unmodified

Zenith builds Quickshell, because no component of the base layer carries it. Quickshell is
**LGPL-3.0-only** — it has been, in every release from the first, and this document
previously said MIT, which was wrong.

The obligation under LGPL-3 is short, and Zenith's build satisfies it:

- The binary is shipped **unmodified**. Zenith does not patch Quickshell, does not link its
  own code into it, and does not distribute it as source it has modified. The shell talks to
  it over a socket and a command line, which is what a separate program does.
- The package carries Quickshell's own copyright file and a copy of the LGPL-3 text, so
  anyone receiving it can read the licence and the source-offer terms that come with it.
  The LGPL text is in the base layer at `/usr/share/common-licenses/LGPL-3`.
- Anyone who wants to modify Quickshell can, under the LGPL's own terms, and can have the
  modifications removed by replacing the package. Nothing Zenith ships prevents that.

The practical consequence for a rebase: a base layer that gains a `quickshell` package ends
this exception, the way the base layer's own `quickshell` package already does in the
releases after 26.04. The manifest's `zenith_built:` key is the thing that notices, and
that is the point of it.

## Firmware is the exception

WiFi adapters, GPUs and many laptops need firmware blobs that are redistributable only as
opaque binaries, under their own licences, frequently with no source available. The base
layer ships them in the `linux-firmware` family: the `linux-firmware` metapackage, the
per-vendor `linux-firmware-<vendor>` packages it pulls in, and `firmware-sof-signed` for the
audio codecs that need signed firmware.

Most of that is free software. Intel WiFi, AMD GPU, much of MediaTek and Atheros firmware is
free. Some of it is not: NVIDIA's GSP firmware, Broadcom's full-FMAC blob, some Realtek NVMe
and WiFi firmware, and several audio codec firmwares are proprietary binaries, and those
laptops have no free driver to fall back on. There is no `non-free-firmware` component to
enable: firmware lives in the packages above, and that split is a property of the archive
rather than of Zenith.

Consequences you must accept if you redistribute Zenith:

- A Zenith ISO is **not** fully free software and must not be presented as though it is. See
  the firmware and free software section of
  [`docs/architecture.md`](docs/architecture.md) for exactly what is and is not free.
- Firmware must be traceable per file. The policy lives with the package manifest in
  [`system/packages.yaml`](system/packages.yaml), and the packages a given machine needs are
  declared by its profile under `system/hardware/profiles/`. Neither may contain firmware
  files, only firmware package names.
- Zenith **references** the base layer's `linux-firmware-*` packages rather than repackaging
  firmware itself. This is deliberate: it keeps Zenith's own repository entirely free software,
  and it means Zenith never takes on a redistribution it has no right to. Some firmware forbids
  redistribution outright; those must never enter a Zenith package.

## Practical guidance

- **Building on Zenith.** GPL-3.0 is copyleft. If you distribute a modified Zenith you must release
  your modifications under GPL-3.0 too. Linking Zenith's code into a closed-source product is
  not permitted.
- **Contributing a hardware profile.** Profiles are configuration data — kernel version,
  package list, firmware set. They are conventionally GPL-3.0 for consistency, but they are
  usually not creative works in the first place.
- **Vendoring third-party code.** Do not paste it into this tree. Add it as a proper package
  under `release/`, keep its own licence file, and record it in the third-party notices file.

## Licence hygiene in this repository

- `LICENSE` is the canonical, unmodified GNU GPL v3 text. Do not edit it and do not append
  project-specific terms to it.
- Any directory that is independently distributable carries its own `LICENSE` copy.
- Per-directory licence files must match `LICENSE` at the root. If one changes, all change.