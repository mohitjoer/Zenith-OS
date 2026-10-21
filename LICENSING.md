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
| The shell's surfaces — the vendored upstream components | GPL-3.0-only and Apache-2.0, by component | `shell/vendor/COPYING`, `shell/vendor/LICENSE.apache-2.0` and the manifest's per-component record; see below |
| Firmware blobs | mixed: most free, some proprietary, each separately licensed | per-package copyright files; see below |
| Hardware profiles in `system/hardware/profiles/` | GPL-3.0 by convention, but data is data | the profile's own `license:` field |

The shell is two things, and the table is where the two are kept apart. The **engine** is
Quickshell: an LGPL-3.0-only binary, built by Zenith and shipped unmodified. The **surfaces**
are the bar, the dock, the launcher, the popouts and the widgets, and a good part of them are
a vendored upstream component under GPL-3.0. The composed shell is therefore **GPL-3.0**. Any
sentence of the form "the shell is LGPL" is true of the engine and false of the shell, and
this document is written so that a reader cannot take it to mean the whole thing.

## Zenith's own code

Everything authored for Zenith is GPL-3.0. This is deliberate: the base is GPL, GPL-3.0 is
what every serious Linux distribution uses, and it guarantees that anyone receiving Zenith can
modify and redistribute the parts that are Zenith's own, forever.

Contributions are made under the Developer Certificate of Origin. Commit sign-off makes the
licence unambiguous and is the only contribution requirement.

### "GPL-3.0" and "GPL-3.0-or-later" are not the same grant

`release/mkdeb/copyright` says **GPL-3.0-or-later**, and the summary table above says
**GPL-3.0**. One of them is wrong, and the difference is not cosmetic: "or later" is a grant
that lets a recipient relicense the work under a later version of the GPL, and a bare
"GPL-3.0" is not.

The position is **GPL-3.0**, and it is the position the root `LICENSE` already takes: that
file is the canonical, unmodified GNU GPL v3 text, and the tree's own sources carry no
per-file header that grants a later version.

The `release/mkdeb/copyright` file is also **not what the build installs**.
`release/build-debs.sh` copies the root `LICENSE` into
`/usr/share/doc/<pkg>/copyright` instead, so the copyright file on an installed system
says GPL-3.0 and matches this document. The discrepancy is therefore between a dead file
and the position, not between two shipped things.

Reconciling it is a change in `release/`, and it is deliberately left to whoever owns that
tree: either delete the unused file, or bring it into line at GPL-3.0. Leaving it as it is
means the repository says two things about its own licence, and the thing it says in the
place nobody reads is the one that is wrong.

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

## The shell's engine is LGPL, and the binary is shipped unmodified

Zenith builds Quickshell, because no component of the base layer carries it. Quickshell is
**LGPL-3.0-only** — it has been, in every release from the first, and this document
previously said MIT, which was wrong.

That sentence is about the **engine**, and only about the engine. The surfaces the engine
draws are a separate matter with a separate answer, and they are set out in
[the vendored components](#the-vendored-components): the composed shell is GPL-3.0, because a
good part of the surfaces are a vendored upstream component under GPL-3.0. Anyone reading
this section as a statement about the whole shell has read it wrongly, which is why the
section is titled with the word "engine" in it.

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

**One thing this section used to get wrong.** It said that the shell was the only thing in
the stack Zenith compiles for itself, and that "there is no vendored blob". The first half
is still true — Quickshell is still the only binary Zenith builds. The second half is not:
there is now a vendored blob in the shell, and it is a large one. It is GPL-3.0, it is
declared, and it is described in the next section.

## The vendored components

Zenith's shell draws some of its surfaces with a vendored open-source Quickshell
configuration. The vendored tree is a **declared tree in this repository**, not a package
pulled in at build time and not a fork: it is checked in, kept byte-identical to a pinned
upstream commit, and every file in it is listed with a hash in
`shell/vendor/manifest.yaml`.

### What is vendored, and at which commit

| Component | Upstream | Licence | Pinned commit | Vendored as |
|---|---|---|---|---|
| The "ii" Quickshell configuration | [`end-4/dots-hyprland`](https://github.com/end-4/dots-hyprland) | GPL-3.0-only | `2f2a376fdb2ad738bf55f693dd0b474b64bb0ab8` | `shell/modules/`, `shell/services/` |
| The rounded-polygon widget kit | [`end-4/rounded-polygon-qmljs`](https://github.com/end-4/rounded-polygon-qmljs) | Apache-2.0 | `e31ec4cb4ebf6a46b267f5c42eabf6874916fa16` | `shell/modules/common/widgets/shapes/` |

The two licence texts sit with the tree in the repository: `shell/vendor/COPYING` is the
GPL-3.0 text, copied verbatim from the upstream repository at the pinned commit, and
`shell/vendor/LICENSE.apache-2.0` is the Apache-2.0 text, likewise copied verbatim. The
manifest records, per component: the upstream name, the URL, the path inside the upstream
repository, the pinned commit, the licence, the licence file, the copyright line, and a hash
for every file. `sh shell/vendor/vendor.sh verify` compares the tree against those hashes,
and a file that has been edited by hand fails it.

### The GPL-3.0 obligations, and how they are met

GPL-3.0 §5 is what governs a distributed work that is based on the component, and §6 is what
governs the object code. The obligations that apply here, and where each is discharged:

| Obligation | How Zenith meets it |
|---|---|
| §4 — convey the licence with the work | `shell/vendor/COPYING` is the component's own licence text, copied from the upstream commit. It has to travel in the package with the tree, not merely live in the repository |
| §5(a) — carry prominent notices that the work was modified, with a date | The manifest names each component, its upstream, its pinned commit and the fact that the vendored files themselves are unmodified; Zenith's changes to the composed shell are the overlay files, which are Zenith's own code and are named as such. The package's copyright file has to carry the same record |
| §5(c) — licence the entire work as a whole under the GPL | The composed shell is GPL-3.0: Zenith's own code is GPL-3.0 and the "ii" component is GPL-3.0-only. The Apache-2.0 component keeps its own licence and is licensed separately, as §4 of that licence allows |
| §5(d) — interactive interfaces display Appropriate Legal Notices | Upstream's own interfaces do not display them, and the licence says that a work need not make them do so where the program it is based on does not. Zenith does not add them, and records that it does not rather than leaving the question open |
| §6(d) — offer the corresponding source at the same place as the object code | The corresponding source of the composed shell is this repository: the vendored tree byte-identical at the pinned commit, the three overlay files, any patch in `shell/vendor/patches/`, and the recipe that builds the engine. Zenith publishes the repository beside the packages, so the source is offered in the same way the binary is |

### The re-theme is a modification, and its source is the repository

This is the part worth being precise about, because it is easy to read the other way.

Zenith's re-theme is **not** a configuration value and it is **not** a cosmetic pass over
upstream's files. Upstream's own `Appearance.qml` and `Config.qml` — the two files its
components read for their colours and their settings — are **replaced by Zenith's own files
at the same paths**, and upstream's components read Zenith's monochrome palette and Zenith's
GSettings-backed configuration because they were told to. No vendored file is modified; the
composition is the modification.

So the distributed shell is a modified work of the "ii" component, and the corresponding
source is the repository plus the overlay files: the vendored tree at the pinned commit, the
three overlays, and any patch. That is what §6(d) asks for, and it is why keeping the tree
byte-identical is a licensing decision as much as a maintenance one — an unmodified tree
means the corresponding source is the repository, with nothing to reconstruct.

### The Apache-2.0 component

The shapes kit is under Apache-2.0, and Zenith has **not** modified it: every file is
byte-identical to the pinned commit. So the obligations are the simpler ones.

| Obligation | How Zenith meets it |
|---|---|
| §4(a) — give recipients a copy of the licence | `shell/vendor/LICENSE.apache-2.0`, copied verbatim from the upstream commit |
| §4(b) — mark modified files | Nothing is modified, so there is nothing to mark. The manifest records the component as unmodified at the pinned commit, which is the stronger statement |
| §4(d) — carry the NOTICE file, if the work has one | Upstream ships no NOTICE file at the pinned commit, so there is none to carry |

### Where the attribution has to travel

Per-component attribution is not finished by writing it in a document. The built package has
to carry it: the two licence texts, and a copyright file that names each component, its
upstream, its licence and its pinned commit.

That is a requirement on the packaging step, and the packaging step does not meet it yet:
`release/build-debs.sh` installs Zenith's own QML and nothing of the vendored tree, and
neither licence text is installed anywhere. The attribution is recorded in the repository and
in this document; it is not yet in a built package. That is a gap rather than a decision, and
it is the gap that closes when the vendored tree is packaged.

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
- **Vendoring third-party code.** The rule here used to be: do not paste it into this tree,
  add it as a proper package under `release/`, keep its own licence file, and record it in
  the third-party notices file. **That rule is changed, deliberately, and the reason is
  worth stating rather than leaving as a silent amendment.**

  A package under `release/` is the right answer for code Zenith *builds* — something with
  its own build system, its own dependencies, its own release cadence, and its own licence
  that Zenith would otherwise have to re-derive. Vendoring into the repository is the right
  answer for a different thing: source that ships as source, that is read and configured
  rather than compiled, and that Zenith wants to hold byte-identical to a pinned upstream
  commit so that the corresponding source of the composed work is the repository itself.

  The shell's vendored tree is the second kind. It is **declared**: a named tree in this
  repository (`shell/modules/`, `shell/services/`), pinned to an exact commit, listed with a
  hash for every file in `shell/vendor/manifest.yaml`, carrying its own licence texts under
  `shell/vendor/`, and shipped with per-component attribution. It is not a fork, not a
  patch set applied in place, and not a directory of files that were pasted in and edited.

  The obligations that come with it are in [the vendored components](#the-vendored-components).

## Licence hygiene in this repository

- `LICENSE` is the canonical, unmodified GNU GPL v3 text. Do not edit it and do not append
  project-specific terms to it.
- Any directory that is independently distributable carries its own `LICENSE` copy.
- Per-directory licence files must match `LICENSE` at the root. If one changes, all change.
  The `licence` workflow enforces it by comparing every tracked file whose path matches
  `(^|/)LICENSE$` against the root file, byte for byte.

### The rule for vendored licence files

A vendored component brings **its own** licence text, and that text is not the root
licence, so it must not be named `LICENSE` — a file called `LICENSE` in this repository is
held to the rule above and would fail the workflow.

The rule for a vendored licence file is therefore:

- **It is named after the licence it carries**, not `LICENSE`. `shell/vendor/COPYING` is the
  GPL-3.0 text and `shell/vendor/LICENSE.apache-2.0` is the Apache-2.0 text. The name says
  which licence it is, so a reader who finds the file knows what they are looking at without
  opening it.
- **It is byte-identical to the upstream copy** at the pinned commit. It is not reformatted,
  not re-wrapped, and not given a Zenith header, for the same reason the vendored tree is
  not: an edited licence file is a different legal instrument.
- **It is exempt from the root-`LICENSE` rule by its name, not by luck.** The workflow
  matches `(^|/)LICENSE$`, so `shell/vendor/LICENSE.apache-2.0` does not match and the check
  passes — but that is an accident of the pattern, not a design. A future file named
  `shell/vendor/LICENSE` would match the pattern and fail, and a future component that
  upstream ships as `LICENSE` would fail it too. The naming rule above is what makes the
  exemption deliberate: a vendored licence file is never called `LICENSE`, so it is never
  caught by a rule that is about Zenith's own licence.

The root `LICENSE` rule and the vendored-file rule are different rules and they do not
conflict. One says every `LICENSE` in this repository is the GPL-3.0 text; the other says a
third party's licence travels under a name that says whose it is.
