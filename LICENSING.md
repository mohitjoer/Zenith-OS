# Licensing in Ken OS

Ken is not licensed under a single licence, and it would be misleading to say that it is.
Ken is a distribution: it ships its own code, plus software from its base layer, plus
firmware that is not entirely free. This document explains the split so the obligations are
clear before you build on Ken.

## Summary

| What | Licence | Where it is recorded |
|---|---|---|
| Ken's own code | GPL-3.0 | [`LICENSE`](LICENSE) |
| The base layer and every Debian/Ubuntu package | whatever each package declares | `/usr/share/doc/<pkg>/copyright` on an installed system |
| Firmware blobs | mixed: most free, some proprietary, each separately licensed | per-package copyright files; see below |
| Hardware profiles in `system/hardware/profiles/` | GPL-3.0 by convention, but data is data | the profile's own `license:` field |

## Ken's own code

Everything authored for Ken is GPL-3.0. This is deliberate: the base is GPL, GPL-3.0 is
what every serious Linux distribution uses, and it guarantees that anyone receiving Ken can
modify and redistribute the parts that are Ken's own, forever.

Contributions are made under the Developer Certificate of Origin. Commit sign-off makes the
licence unambiguous and is the only contribution requirement.

### Historical code

This repository was formerly `zeo-widgets`, a collection of GNOME Shell extensions. That
code remains in the Git history before the project became Ken OS and is not part of Ken. It
was GPL-3.0 and it stays GPL-3.0 in history — relicensing history is not possible and is not
attempted.

Ken replaces GNOME Shell with Hyprland and a Quickshell-drawn shell, so GNOME Shell
extensions are not a Ken extension mechanism. The GNOME Shell internals those extensions
depended on are archived in [`agents/gnome-shell-notes.md`](agents/gnome-shell-notes.md) as
reference material only.

## The base layer and its packages

Ubuntu supplies Ken's base layer. Its packages are individually licensed — mostly GPL-family,
some LGPL, some Apache-2.0, some BSD, some public domain — and each ships its own copyright
file. Ken neither relicenses nor overrides them, and by policy never modifies them in place.

Redistributing Ken means redistributing that base layer, so both the per-package licences and
Ubuntu's own terms of service apply. This is not optional legal fine print; it is the most
common way a distribution goes wrong.

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
rather than of Ken.

Consequences you must accept if you redistribute Ken:

- A Ken ISO is **not** fully free software and must not be presented as though it is. See
  the firmware and free software section of
  [`docs/architecture.md`](docs/architecture.md) for exactly what is and is not free.
- Firmware must be traceable per file. The policy lives with the package manifest in
  [`system/packages.yaml`](system/packages.yaml), and the packages a given machine needs are
  declared by its profile under `system/hardware/profiles/`. Neither may contain firmware
  files, only firmware package names.
- Ken **references** the base layer's `linux-firmware-*` packages rather than repackaging
  firmware itself. This is deliberate: it keeps Ken's own repository entirely free software,
  and it means Ken never takes on a redistribution it has no right to. Some firmware forbids
  redistribution outright; those must never enter a Ken package.

## Practical guidance

- **Building on Ken.** GPL-3.0 is copyleft. If you distribute a modified Ken you must release
  your modifications under GPL-3.0 too. Linking Ken's code into a closed-source product is
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