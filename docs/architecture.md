# Ken OS architecture

Why Ken is built the way it is. Read this before designing or changing anything.

This document records **decisions, not aspirations**. Where a decision is settled it says so
and gives the reason. Where a question is open it says that too, so nobody re-litigates a
closed question or assumes an open one is closed.

---

## Settled decisions

| Decision | Choice | Reason |
|---|---|---|
| Base | Ubuntu 26.04 LTS | Widest firmware and DKMS coverage of any distro, and the most complete driver story available. Hardware reach is why the base exists. |
| Compositor | Hyprland | Ken draws its own shell and does not need a stock one. |
| Shell | Quickshell (QML) | Compositor-agnostic, proven, and the design language already lives in QML/CSS. |
| Desktop stack | No GNOME | No GNOME Shell, no GNOME daemons, no GNOME portal backend. See [the portal seam](#the-portal-seam). |
| Control tools | Go (`ken`, `ken-shell`, `ken-hub`) | Static binaries, cross-compiles cleanly, fast startup. |
| Terminal | kitty | The most capable terminal in the stack — graphics protocol, `kitty @`, shell integration. Chosen over the terminal the base layer packages, on the understanding that **Ken owns its source build forever**. A deliberate trade, not an oversight. |
| User shell | zsh + starship | fish and blesh ship alongside as alternatives. |
| Settings store | GSettings + dconf | Typed, validated, per-user, atomic. Strictly better than a hand-rolled JSON store. |
| Boot chain | Limine | Small, modern, and already proven on the reference system. |
| Login manager | SDDM | Works on Ubuntu, themable, no GNOME session assumptions. |
| ISO tooling | live-build + live-installer | Subiquity is Python and near-unforkable. |
| Support model | Tiered, not universal | No distro supports every laptop; claiming otherwise is dishonest. |
| Scope | Open source, all laptops | Hardware reach is the product. |
| Identity | Ken is the distribution; Ubuntu is the base layer | See [the base-layer boundary](#the-base-layer-boundary). |
| Free software | Free at the distribution level, non-free declared per machine | See [firmware and free software](#firmware-and-free-software). |

### Rejected, and why

- **Owning the compositor.** Ken uses Hyprland. Forking a compositor puts a decade of
  undifferentiated work on the critical path for no user-visible gain.
- **GNOME.** Not GNOME Shell, not the GNOME daemons, not `xdg-desktop-portal-gnome`. Ken
  owns its own shell and its own surfaces. What that costs, and what it simplifies, is
  written up in [the portal seam](#the-portal-seam). The extensions that motivated this repo
  are archived in [`../agents/gnome-shell-notes.md`](../agents/gnome-shell-notes.md).
- **Multi-release ISO matrix.** 26.04 only, for now. Revisit when hardware profiles exist for
  machines still running 22.04 or 24.04.
- **snap.** The overlay model gives better rollback, and dropping snapd off the boot path is
  free performance.
- **A unified version manager (mise and friends) for all three languages.** Each language's
  own official tool is what its ecosystem expects; Ken should not put its own opinion in the
  way. See [runtime stack](#runtime-stack).
- **Claiming to be 100% free software.** Ken's own code and its own repository are free. The
  *image* for a given machine may need firmware that is not. See
  [firmware and free software](#firmware-and-free-software).

---

## The base-layer boundary

Ken is the distribution. Ubuntu is its base layer — the substrate, not the identity. That
distinction only stays true if it is enforced, and the enforcement is a single rule:

> **Ken's packages write only to `/usr/share/ken/` and `/etc/ken/`. Never to another
> package's files.**

The moment Ken edits `/etc/NetworkManager/`, or patches a file inside `/usr/lib/`, it is no
longer building over the base layer — it is forking it. The cost arrives later and all at
once: every rebase onto a newer Ubuntu becomes a merge conflict across files nobody
remembered touching, and the release train stops.

Ken's packages follow from that rule:

- A base-layer package with the wrong configuration gets a **Ken's own config package**, or a
  drop-in shadowing it under a `ken-` name. It is never edited in place.
- A base-layer package that is merely missing something gets a `ken-` package that depends on
  it and supplements it.
- The only two directories Ken owns outright are `/usr/share/ken/` and `/etc/ken/`. Anything
  Ken needs outside them is a systemd drop-in, a GSettings schema, or an overlay the user
  controls.
- Ship only what Ken needs from the base layer. Every package in the image is a liability on
  the next rebase; the manifest of what Ken installs is itself a reviewed artefact.

The payoff is that rebasing Ken onto a new Ubuntu becomes a package-set change rather than a
merge. That is the difference between a distribution that can track its base and one that
slowly becomes an unmaintainable fork of it.

---

## Layer map

Ken's layers, from bottom up.

| Layer | Owner | Notes |
|---|---|---|
| Kernel, firmware, boot | `system/kernel/`, installer | Pinned fleet, not a single kernel |
| Base system | Ubuntu packages | Unmodified; Ken adds only what it needs |
| Desktop services | Ken + non-GNOME system services | power-profiles-daemon, pipewire, wireplumber, iwd |
| Display | Hyprland | Behind a `ken wm` seam so it can be swapped |
| Shell surfaces | Quickshell | Bar, dock, launcher, popouts, widgets, notifications, global shortcuts |
| Settings store | GSettings / dconf | One writer per setting |
| Control tools | `ken`, `ken-shell`, `ken-hub` | The only writers |
| Portals | `xdg-desktop-portal-hyprland` + `-gtk` | See below |
| Updates | `ken update` · `rollback` · `doctor` | Overlay model |

---

## The portal seam

Every GTK and Qt application reaches the desktop through `xdg-desktop-portal` rather than
the compositor directly. An app that has ever shown a file chooser, shared a screen, or asked
for a secret has gone through it. If this layer is wrong, a thousand apps break in a thousand
different ways, so it is worth getting right early.

Ken's stack, with each backend's interfaces verified against what it actually exports rather
than what its name suggests:

| Component | Interfaces it implements |
|---|---|
| `xdg-desktop-portal` | the dispatcher every application talks to |
| `xdg-desktop-portal-hyprland` | ScreenCast, Screenshot, **GlobalShortcuts**, **InputCapture** |
| `xdg-desktop-portal-gtk` | FileChooser, AppChooser, Print, Notification, Inhibit, Access, Account, Email, DynamicLauncher, Lockdown, Settings |

There is no GNOME in this stack, and that is deliberate. Earlier drafts ran
`xdg-desktop-portal-gnome`, which had to be convinced it was not on a GNOME session: it reads
`GDK_BACKEND` and `XDG_SESSION_TYPE`, and when it decides the session is incompatible it sets
a `settings_only` flag and exports almost nothing. Choosing not to ship GNOME at all removes
the problem instead of working around it.

`OpenURI` is not implemented by the GTK backend directly, but it only requires an
`AppChooser` backend, which the GTK backend does implement — so it resolves in practice.

### The Secret interface is a real gap

**No backend in the stack implements `org.freedesktop.portal.Secret`, and `gnome-keyring` does
not fix this.** `gnome-keyring` exposes `org.freedesktop.Secret.Service` on the session bus,
but `xdg-desktop-portal` never asks it — the dispatcher looks for a *portal backend* that
declares the interface, finds none, and returns success without exporting the portal at all.
No error, no warning, no fallback.

The consequence is concrete: every Flatpak that uses the Secret interface — Firefox,
Thunderbird, Evolution, VS Code — has nowhere to store a password on Ken. This is the single
worst portal gap and it is not cosmetic.

Three ways out, none chosen yet:

1. Ship `xdg-desktop-portal-gnome` purely for Secret. Works, but reintroduces the GNOME
   backend the project rejected.
2. Adopt a Secret backend derived from the KDE portal implementation. Keeps the no-GNOME rule;
   costs a package to track.
3. Ship a small Ken-owned Secret backend over `gnome-keyring`'s service interface. Most
   control, most work, and it is genuinely Ken's code.

Until this is resolved, the contract below is **not** true and must not be claimed.

### The contract

An application that works on a stock desktop must work on Ken — with the Secret interface as
the known exception until one of the three options above lands. A hardware report should ask
contributors to confirm portal behaviour, and a portal regression is a distribution-wide bug,
not a cosmetic one.

`ken doctor` must assert that the portal configuration file is actually being read: the
per-desktop file name is derived from `XDG_CURRENT_DESKTOP`, and a session that does not
declare itself correctly falls back to `portals.conf` and silently uses the wrong backends.

Interfaces Ken still owns itself, because no backend provides them:

- **Camera.** No portal backend implements it; capture is Pipewire's job.
- **Notifications.** With no notification daemon in the image, the shell hosts the
  notification server.

GlobalShortcuts and InputCapture *are* compositor-backend work and are handled by the Hyprland
backend, not by the shell.

---

## Overlay and rollback

Ken's packages write **only** to `/usr/share/ken/` and `/etc/ken/`. Never to `~`. That single
rule is what makes rollback possible.

```
/usr/share/ken/                     the packaged base; replaced on update
/etc/ken/                           machine-level config; replaced on update
~/.config/ken/                      materialized base, user's copy
~/.config/ken/user_edits/           the user's overlay; wins at the mirrored path
~/.local/share/ken/plugins/         installed plugins
~/.local/state/ken/                 runtime state, revisions, snapshots
```

| Command | Does |
|---|---|
| `ken update` | snapshot `~/.config` to git, fetch debs, re-materialize base, repaint |
| `ken rollback` | re-materialize the previous revision |
| `ken materialize <path>` | re-lay the base file, clobbering shipped content |
| `ken reset <path>` | drop one overlay file back to the base |
| `ken doctor` | report drift between base and overlay |

A user override always wins at its mirrored path, and `ken update` can never clobber it.
This is the discipline that makes the rest of the system safe to automate.

---

## Settings

Every desktop setting lives in **GSettings schemas**, which give typed values, validation,
per-user scoping, atomic apply through dconf, and `gsettings list-recursively` for
introspection — which is exactly what a widget catalogue needs.

- `org.ken.desktop` — desktop-level behaviour
- `org.ken.shell` — bar layout, dock, widgets, launcher

The shell daemon and `ken-hub` are the only writers. Hand-editing dconf directly causes drift
and `ken doctor` reports it.

---

## Runtime stack

### Terminal — kitty

GPU-accelerated and scriptable, with its own OpenGL renderer rather than a VTE text grid, so
it costs far less than a GTK terminal. It is the most capable terminal in the stack and it
is the one thing here Ken is willing to accept a packaging burden for, because the terminal
is the surface every developer looks at all day.

Four places declare it the default, and all four must be set or the desktop is inconsistent:

1. **Ken's shell** — the terminal-launcher setting the bar button calls
2. **`gsettings set org.gnome.desktop.default-applications.terminal exec kitty`** — so the
   file manager's "Open in Terminal" and GTK dialogs use it
3. **`update-alternatives`** on `/usr/bin/x-terminal-emulator` with an explicit `--slave` and
   alternatives name — for applications that hardcode the VTE path
4. **kitty's zsh shell integration**, sourced from `~/.zshrc` — prompt marking, and what makes
   `kitty @` and the pager integration work

**Packaging consequence, stated plainly:** kitty is not carried by the base layer's archive,
so `ken-terminal` builds it from source and Ken owns that build for the life of the project.
That is a deliberate trade: the base layer maintains one binary for us, and we maintain a
better one ourselves. See [the rejected options](#rejected-and-why).

What kitty brings that a stock terminal does not, and why it is worth the build:

| | |
|---|---|
| Terminal graphics protocol | Inline images and animations. kitty originated it; `icat` displays remote images in a pane |
| `kitty @` remote control | The shell and the bar can open tabs, set colours, and query panes over a socket |
| Kittens | Python extensions for `diff`, `ssh`, `choose_files`, `hints`, `notify` |
| Shell integration | Prompt marking, hyperlink handling, clickable paths |
| Layouts | Tabs, splits, grid, stack — configurable per session |

Default font: **JetBrainsMono Nerd Font** — for devicon and powerline glyphs, not because
ligatures require it.

### User shell — zsh

zsh is the shell, starship is the prompt, and kitty is where they meet.

- zsh is the login shell; kitty's `shell` setting points at it explicitly so the session does
  not depend on what `$SHELL` happens to say
- kitty's zsh integration is sourced from `~/.zshrc`, guarded so a non-kitty terminal does
  not error on the missing file
- `starship` renders the prompt; its configuration lives under Ken's overlay like everything
  else, so `ken update` cannot clobber it
- fish and blesh ship alongside zsh for users who prefer them; neither is the default

Ken's overlay model gives the shell the same protection as the rest of the desktop: a change
to `~/.zshrc` is a user's change and survives every update.

### Languages

| | Ships | Managed by | Why |
|---|---|---|---|
| Rust | `rustup`, shared toolchain at `/opt/ken/rust` | rustup | Ubuntu's apt `rustc` lags years behind. Never ship Rust via apt. |
| Node | `nodejs` + `npm` pinned to LTS, `corepack` enabled | fnm | System Node is the floor; per-project versions per repository. |
| Python | `python3`, `python3-venv`, `python3-dev` | mise + `uv` | System Python stays pristine — it is an OS dependency and PEP 668 marks it externally managed. |

Preinstalled Rust tooling, because they are near-universal wins:
`stable` toolchain, `cargo-edit`, `just`, `sccache`, `mold`, `cargo-nextest`, `ripgrep`, `fd`.

The system shell is **zsh** with starship. fish and blesh ship alongside it for users who
prefer them.

---

## Hardware strategy

This is the largest subsystem and the one that decides whether Ken is usable by anyone but
its author.

### Support tiers

Defined in [`../system/hardware/tiers.yaml`](../system/hardware/tiers.yaml). Tier 1 verified,
tier 2 community confirmed, tier 3 best effort. Every machine falls into exactly one, and the
tier is determined by whether a profile exists, not by marketing.

### The profile database

`ken-machines` is a database of verified machine profiles in this repository. Nobody can test
every laptop, but every user can test one:

1. A machine boots successfully
2. `ken hardware report` emits a candidate profile
3. The contributor attaches it to a pull request
4. Merged profiles ship in the next ISO and are selected automatically at install time

This turns an unbounded hardware problem into a union of working setups that grows without
anyone owning a hardware lab. It is the highest-leverage mechanism in Ken.

### Kernel fleet

A single kernel cannot support both old drivers and new silicon. Ken pins a fleet:

- **upstream generic** — the baseline, always boots
- **HWE** — recent hardware enablement, backported continuously
- **OEM/vendor** — only where a profile demands it

`linux-headers` ships for every one, because DKMS is unavoidable for parts of the laptop
market.

### The hard list

Every item here is a real support ticket someone will file:

| Area | Requirement |
|---|---|
| GPU | `nvidia-open` vs proprietary vs nouveau, pinned per kernel; Intel `xe`/`i915`; AMD `amdgpu`; PRIME hybrid mux; vendor mux tools (`supergfxctl`, `envy`, `asusctl`) |
| WiFi | `iwlwifi`, broadcom `brcmfmac` with non-free fallback, Realtek `rtl8xx8` → DKMS `rtl8821ce`, `mt76`, `ath10k`/`ath11k` |
| Firmware | `linux-firmware` plus per-vendor `linux-firmware-<vendor>` packages, and `firmware-sof-signed`. There is no `non-free-firmware` component on this base layer, and neither `firmware-nonfree` nor `linux-firmware-nonfree` exists. |
| Microcode | `intel-microcode`, `amd64-microcode`. Both already hook into the initramfs, so no extra work is needed — only the correct names. |
| Audio | pipewire + wireplumber, SOF firmware, Realtek codec quirks |
| Suspend/resume | the most-reported laptop bug class; hybrid graphics makes it worse |
| Fingerprint | `fprintd` + libfprint, with goodix quirks |
| Displays | `ddcutil` for external-monitor brightness, docks, multi-GPU output routing |
| Handhelds | Steam Deck, ROG Ally, Legion Go — gamescope session, gyro, controller mapping |
| Secure Boot | shim + MOK, signed. Non-optional or nothing installs on a locked laptop |

### The offline problem

A laptop whose WiFi needs an unloaded driver cannot fetch that driver over the network it does
not have. This is why every Linux ISO hits the same wall. Ken's answer is a broad firmware
rescue bundle on the ISO plus `ken-firmware load` from a USB stick. Unglamorous, and the
difference between "installs on my laptop" and "installs on most laptops".

---

## Firmware and free software

Ken is free software. It cannot honestly claim to be free of proprietary code *on disk*, and
the difference matters, so state it precisely.

**The distribution level.** Ken's own code is GPL-3.0 and Ken's own APT repository contains
nothing but free software. That claim is clean, verifiable with `apt-cache` , and true.

**The image level.** A specific machine's firmware is a separate question. Most of
`linux-firmware` is free — Intel WiFi, AMD GPU, much of MediaTek, Atheros. But NVIDIA's GSP
firmware, Broadcom's full-FMAC blob, some Realtek NVMe and WiFi firmware, and several audio
codec firmwares are proprietary binaries. Those laptops do not have a free driver to fall
back on. An image without them may not bring up networking or graphics at all.

So the honest formulation is:

> Ken's code and Ken's repository are free software. Some hardware requires proprietary
> firmware, which Ken does not redistribute — it installs the base layer's own
> `linux-firmware-*` packages, where the redistributor already holds the rights.

Note what the base layer actually offers: there is **no `non-free-firmware` component**, and
neither `firmware-nonfree` nor `linux-firmware-nonfree` exists. Firmware arrives as the
`linux-firmware` metapackage, per-vendor `linux-firmware-<vendor>` packages, and
`firmware-sof-signed`. Most of that is free software — Intel WiFi, AMD GPU, much of MediaTek,
Atheros — and some of it is not.

Three consequences, all deliberate:

1. **Ken references, it does not repackage.** The installer pulls the base layer's
   `linux-firmware-<vendor>` packages rather than carrying firmware in a Ken package. This
   keeps Ken's repository entirely free software and stops Ken taking on a redistribution it
   has no right to.
2. **A hardware profile declares what it needs.** `firmware:` in a profile names *packages*.
   That is the same mechanism a community profile already uses for its driver list, and it is
   why per-machine declaration scales — see [the hard list](#the-hard-list).
3. **Nothing claims FSF eligibility.** Ken does not describe itself as a fully free
   distribution, and no Ken ISO should be labelled that way. `firmware-reporter` would say
   otherwise within minutes of someone checking.

### The ISO question

An ISO is where this becomes concrete, and it is genuinely unresolved. One image that bakes
in firmware is maximally convenient and maximally dishonest. One image that refuses firmware
is honest and fails to bring up WiFi on a lot of laptops. Two images — one with firmware, one
without — is the honest compromise and costs one extra build target.

This is tracked as an open question below rather than decided here, because it is a
distributional judgement rather than a technical one, and it should be made deliberately
rather than inherited from a draft.

---

## ISO and installer

- **live-build** as the ISO builder. Declarative, reproducible, and it ships in the base
  layer's `main` with real Ubuntu support (`lb_config --mode ubuntu` selects the right
  archive and security hosts). Caveats to design around: the packaged version is a 2012-era
  `3.0~a57` snapshot carried forward by Ubuntu deltas, so it is not as maintained as it looks.
- **live-installer** as the installer UI. It is a real, active, forkable GTK project. It
  targets the Debian edition, so porting it to this base layer is plausible work rather than a
  drop-in, and its licensing needs confirming before a fork.
- **mmdebstrap** as the rootfs seed when tighter control is needed than live-build gives.
- **Limine** as the boot chain, on a separate ESP with its own `/boot`.

**live-build cannot emit a Limine ISO.** Its bootloader templates are `extlinux`, `isolinux`,
`pxelinux` and `syslinux` — there is no Limine template, and `LB_BOOTLOADER` defaults to
`syslinux` on amd64. Choosing both live-build and Limine means hand-writing a boot-chain hook,
which undercuts the reason live-build was chosen in the first place. This is unresolved: the
alternatives are a custom hook, or building the rootfs with `mmdebstrap` and the boot chain by
hand.
- Secure Boot is a two-key problem for Limine. Limine does **not** use UKS: it verifies
  integrity by hash, not authenticity by signature. Signing the Limine EFI executable and
  enrolling the **BLAKE2B hash of `limine.conf` inside it** is what enforces an unmodified
  config, and once that hash is enrolled every file path in the config must carry a matching
  `#<blake2b>` suffix. Separately, on Windows-initialised firmware only Microsoft keys are
  enrolled, so shim must present a MOK for Ken's key before chainloading Limine at all.

Package flow is an APT repository signed with Ken's own GPG key, referenced by `signed-by`
in `sources.list.d`. Releases are published with signed checksums.

---

## Performance plan

Ken ships a deliberately small daemon set, which makes the performance work subtractive rather
than desperate. In rough order of payoff:

1. Ship only what the base layer provides that Ken actually uses — every extra package is
   boot time and memory
2. `tracker3`, `gnome-software`, `packagekit` autoupdate, `apport`, `whoopsie`, cups-bsd — the
   image should not contain a software centre
3. `systemd-zram-generator` — the package is not called `zram-generator`, and it is a
   systemd generator, not an initramfs hook
4. Wayland-forcing environment in `/etc/environment`; `GSK_RENDERER=gl` only if GTK apps are
   slow, since forcing GL is a trade, not a free win
5. Trim the initramfs (initramfs-tools, not mkinitcpio — keep the base layer's, tune it)
6. `systemd` services profile, `journald` limits, early-microcode
7. ~~Prelinking~~ — **removed, not deferred.** The `prelink` package does not exist in this
   base layer; it was dropped during the 26.04 cycle. Upstream has been frozen for over a
   decade, it ships no `postinst`, and it is fundamentally incompatible with ASLR. There is
   nothing to measure because there is nothing to install.

Measure before claiming a win. `ken benchmark` should record idle RSS, boot time and CPU so
regressions are visible rather than anecdotal.

---

## Open questions

| Question | Why it matters |
|---|---|
| The Secret portal | No backend in the stack implements `org.freedesktop.portal.Secret`, and `gnome-keyring` does not substitute for one. Password storage is broken in Flatpaks until one of the three options is chosen. See [the portal seam](#the-portal-seam). |
| Repository hosting | Ken's APT repository and ISO hosting need a home. GitHub Pages serves static files; a package repo wants object storage. |
| Plugin mechanism | Ken's shell has surfaces no stock desktop has. What a Ken plugin looks like is undefined. |
| Hardware report scope | What `ken hardware report` captures, and how aggressively it strips PII before emitting. |
| ISO firmware policy | Whether one ISO carries firmware or firmware is fetched at install time. See [firmware and free software](#firmware-and-free-software). |
| Test matrix | Which Hyprland, Quickshell and kernel versions CI must pass before a release. |