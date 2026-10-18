# Zenith OS architecture

Why Zenith is built the way it is. Read this before designing or changing anything.

This document records **decisions, not aspirations**. Where a decision is settled it says so
and gives the reason. Where a question is open it says that too, so nobody re-litigates a
closed question or assumes an open one is closed.

---

## Settled decisions

| Decision | Choice | Reason |
|---|---|---|
| Base | Ubuntu 26.04 LTS | Widest firmware and DKMS coverage of any distro, and the most complete driver story available. Hardware reach is why the base exists. |
| Filesystem & Rollback | Btrfs subvolumes with automated pre-APT snapshot hooks | Enables atomic updates and instant rollback without wasting space on full system copies. |
| Compositor | Hyprland | Zenith draws its own shell and does not need a stock one. |
| Desktop shell | Quickshell (QML) — bar, dock, launcher, lockscreen, greeter, built by Zenith because the base layer carries no such shell | Compositor-agnostic, proven, and the design language already lives in QML/CSS. See [the shell](#the-shell). |
| Control plane & State | Go — `zenith`, `zenithd` (typed state daemon, zero-CGo), `zenith-hub` | Static binaries, cross-compiles cleanly, fast startup, with strong typing and no CGo dependencies. |
| Terminal | Ghostty — built and pinned upstream by Zenith, which is what sets the base version | GPU-accelerated and client-side rendered; Ghostty's own GTK/Adwaita floor policy is adopted verbatim, so base and terminal move together on purpose. |
| User shell | zsh + starship | fish and blesh ship alongside as alternatives. |
| Editor | Neovim | Modal, extensible, and the ubiquitous developer editor. |
| Boot chain | Limine + Unified Kernel Images (UKI) | Limine is small, modern, and proven; UKIs reduce boot complexity and improve security. |
| Login manager | `sddm` in Wayland greeter mode, with the login surface drawn by Zenith | The base layer's login manager, run as shipped, with a greeter Zenith owns. See [what draws the login screen](#what-draws-the-login-screen). |
| Desktop services | pipewire + wireplumber, tuned-ppd, iwd + NetworkManager, fprintd | Integrated audio/video, platform daemons, wireless/ wired networking, and fingerprint support. |
| Portals | `xdg-desktop-portal-hyprland` + `-gtk`, with `gnome-keyring` for secrets | Hyprland-native portal backend with GTK fallback; secrets handled via gnome-keyring. |
| Package manager | APT, backed by Zenith's signed first-party repository | Familiar, robust, and cryptographically signed for trustworthy updates. |
| Agent layer | Resident OS agent with a local knowledge vault | Provides durable knowledge, self-diagnosis, and contextual assistance. |
| Support model | Tiered, not universal | No distro supports every laptop; claiming otherwise is dishonest. |
| Scope | Open source, all laptops | Hardware reach is the product. |
| Identity | Zenith is the distribution; Ubuntu is the base layer | See [the base-layer boundary](#the-base-layer-boundary). |
| Free software | Free at the distribution level, non-free declared per machine | See [firmware and free software](#firmware-and-free-software). |

### Rejected, and why

- **Owning the compositor.** Zenith uses Hyprland. Forking a compositor puts a decade of
  undifferentiated work on the critical path for no user-visible gain.
- **GNOME.** Not GNOME Shell, not the GNOME daemons, not `xdg-desktop-portal-gnome`. Zenith
  owns its own shell and its own surfaces. What that costs, and what it simplifies, is
  written up in [the portal seam](#the-portal-seam). The extensions that motivated this repo
  are archived in [`../agents/gnome-shell-notes.md`](../agents/gnome-shell-notes.md).
- **Multi-release ISO matrix.** 26.04 only, for now. Revisit when hardware profiles exist for
  machines still running 22.04 or 24.04.
- **snap.** The overlay model gives better rollback, and dropping snapd off the boot path is
  free performance.
- **A unified version manager (mise and friends) for all three languages.** Each language's
  own official tool is what its ecosystem expects; Zenith should not put its own opinion in the
  way. See [runtime stack](#runtime-stack).
- **Claiming to be 100% free software.** Zenith's own code and its own repository are free. The
  *image* for a given machine may need firmware that is not. See
  [firmware and free software](#firmware-and-free-software).

---

## The base-layer boundary

Zenith is the distribution. Ubuntu is its base layer — the substrate, not the identity. That
distinction only stays true if it is enforced, and the enforcement is a single rule:

> **Zenith's packages write only to `/usr/share/zenith/` and `/etc/zenith/`. Never to another
> package's files.**

The moment Zenith edits `/etc/NetworkManager/`, or patches a file inside `/usr/lib/`, it is no
longer building over the base layer — it is forking it. The cost arrives later and all at
once: every rebase onto a newer Ubuntu becomes a merge conflict across files nobody
remembered touching, and the release train stops.

Zenith's packages follow from that rule:

- A base-layer package with the wrong configuration gets a **Zenith's own config package**, or a
  drop-in shadowing it under a `zenith-` name. It is never edited in place.
- A base-layer package that is merely missing something gets a `zenith-` package that depends on
  it and supplements it.
- The only two directories Zenith owns outright are `/usr/share/zenith/` and `/etc/zenith/`. Anything
  Zenith needs outside them is a systemd drop-in, a GSettings schema, or an overlay the user
  controls.
- Ship only what Zenith needs from the base layer. Every package in the image is a liability on
  the next rebase; the manifest of what Zenith installs is itself a reviewed artefact.

The payoff is that rebasing Zenith onto a new Ubuntu becomes a package-set change rather than a
merge. That is the difference between a distribution that can track its base and one that
slowly becomes an unmaintainable fork of it.

### The sanctioned list outside the boundary

Two rules cannot both be absolute. A handful of files have to sit outside `/usr/share/zenith/`
and `/etc/zenith/`, because the binaries that read them look nowhere else — a systemd unit
directory, a GLib schema directory, the login manager's fixed config directory. Those files are
**registrations**: pointers a third-party binary must be able to find.

So the boundary rule is enforced as a closed list rather than an absolute. Every path outside
the two directories must appear here, and a path that is not on this list is a boundary
violation until it is added:

| Path | What it is | Why it cannot live under `/usr/share/zenith/` |
|---|---|---|
| `/usr/share/wayland-sessions/zenith.desktop` | Session registration | The login manager scans the fixed `wayland-sessions` directory and nowhere else |
| `/usr/share/glib-2.0/schemas/org.zenith.*.gschema.xml` | GSettings schemas | GLib compiles schemas from the fixed schema directory |
| `/usr/lib/systemd/user/zenith-shell.service`, `zenith-portal.service` | User unit registration | `systemd --user` loads units from the fixed unit directory |
| `/usr/lib/systemd/user/hyprland-session.target.d/10-zenith.conf` | Drop-in | systemd reads drop-ins from a fixed directory beside the unit |
| `/usr/share/xdg-desktop-portal/zenith-portals.conf` | Portal backend preference | The dispatcher reads it from a fixed directory |
| `/etc/sddm.conf.d/10-zenith.conf` | Login manager preferences | SDDM reads its drop-ins from a fixed directory beside `/etc/sddm.conf` |
| `/usr/share/sddm/themes/zenith/` | Login screen theme | The login manager reads themes from a fixed directory |
| `/boot/limine.conf` | Boot configuration | The boot loader reads its configuration from the boot volume, which the base layer's kernel packages already write to |

Content stays inside the two directories: the login surface's own files belong at
`/usr/share/zenith/session/`, and everything SDDM needs to point at belongs there too. A
registration is a pointer, never a place to put content — a file that grows past a few lines of
pointer belongs under `/usr/share/zenith/` with a registration pointing at it.

`zenith doctor` reports drift between the registrations and the content they point at, the same
way it reports drift everywhere else.

---

## Layer map

Zenith's layers, from bottom up.

| Layer | Owner | Notes |
|---|---|---|
| Kernel, firmware, boot | `system/kernel/`, installer | Pinned fleet, not a single kernel |
| Base system | Ubuntu packages | Unmodified; Zenith adds only what it needs |
| Desktop services | Zenith + non-GNOME system services | power-profiles-daemon, pipewire, wireplumber, iwd |
| Display | Hyprland | Behind a `zenith wm` seam so it can be swapped |
| Shell surfaces | Quickshell | Bar, dock, launcher, popouts, widgets, notifications, global shortcuts |
| Settings store | GSettings / dconf | One writer per setting |
| Control tools | `zenith`, `zenith-shell`, `zenith-hub` | The only writers |
| Portals | `xdg-desktop-portal-hyprland` + `-gtk` | See below |
| Updates | `zenith update` · `rollback` · `doctor` | Overlay model |

---

## The portal seam

Every GTK and Qt application reaches the desktop through `xdg-desktop-portal` rather than
the compositor directly. An app that has ever shown a file chooser, shared a screen, or asked
for a secret has gone through it. If this layer is wrong, a thousand apps break in a thousand
different ways, so it is worth getting right early.

Zenith's stack, with each backend's interfaces verified against what it actually exports rather
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
Thunderbird, Evolution, VS Code — has nowhere to store a password on Zenith. This is the single
worst portal gap and it is not cosmetic.

Three ways out, none chosen yet:

1. Ship `xdg-desktop-portal-gnome` purely for Secret. Works, but reintroduces the GNOME
   backend the project rejected.
2. Adopt a Secret backend derived from the KDE portal implementation. Keeps the no-GNOME rule;
   costs a package to track.
3. Ship a small Zenith-owned Secret backend over `gnome-keyring`'s service interface. Most
   control, most work, and it is genuinely Zenith's code.

Until this is resolved, the contract below is **not** true and must not be claimed.

### The contract

An application that works on a stock desktop must work on Zenith — with the Secret interface as
the known exception until one of the three options above lands. A hardware report should ask
contributors to confirm portal behaviour, and a portal regression is a distribution-wide bug,
not a cosmetic one.

`zenith doctor` must assert that the portal configuration file is actually being read: the
per-desktop file name is derived from `XDG_CURRENT_DESKTOP`, and a session that does not
declare itself correctly falls back to `portals.conf` and silently uses the wrong backends.

Interfaces Zenith still owns itself, because no backend provides them:

- **Camera.** No portal backend implements it; capture is Pipewire's job.
- **Notifications.** With no notification daemon in the image, the shell hosts the
  notification server.

GlobalShortcuts and InputCapture *are* compositor-backend work and are handled by the Hyprland
backend, not by the shell.

---

## Overlay and rollback

Zenith's packages write **only** to `/usr/share/zenith/` and `/etc/zenith/`. Never to `~`. That single
rule is what makes rollback possible.

```
/usr/share/zenith/                     the packaged base; replaced on update
/etc/zenith/                           machine-level config; replaced on update
~/.config/zenith/                      materialized base, user's copy
~/.config/zenith/user_edits/           the user's overlay; wins at the mirrored path
~/.local/share/zenith/plugins/         installed plugins
~/.local/state/zenith/                 runtime state, revisions, snapshots
```

| Command | Does |
|---|---|
| `zenith update` | snapshot `~/.config` to git, fetch debs, re-materialize base, repaint |
| `zenith rollback` | re-materialize the previous revision |
| `zenith materialize <path>` | re-lay the base file, clobbering shipped content |
| `zenith reset <path>` | drop one overlay file back to the base |
| `zenith doctor` | report drift between base and overlay |

A user override always wins at its mirrored path, and `zenith update` can never clobber it.
This is the discipline that makes the rest of the system safe to automate.

---

## Settings

Every desktop setting lives in **GSettings schemas**, which give typed values, validation,
per-user scoping, atomic apply through dconf, and `gsettings list-recursively` for
introspection — which is exactly what a widget catalogue needs.

- `org.zenith.desktop` — desktop-level behaviour
- `org.zenith.shell` — bar layout, dock, widgets, launcher

The shell daemon and `zenith-hub` are the only writers. Hand-editing dconf directly causes drift
and `zenith doctor` reports it.

---

## Runtime stack

### The shell

Quickshell, and Zenith builds it. The base layer carries no compositor-agnostic
shell toolkit at all — verified rather than assumed: `quickshell`, `ags`, `eww` and
`ironbar` are in no component of 26.04. What the archive does carry is *panels* —
`waybar`, `yambar`, `tint2`, `polybar`, `xfce4-panel` — and a panel is not a
substitute. A panel draws a bar. Zenith's shell hosts the bar, dock, launcher,
popouts, widget catalogue, notification server, lock, idle screen, quick settings
and status icons, and the Hub window as well, in one compositor-agnostic toolkit,
so that the desktop has one UI toolkit and one design system rather than two.

That makes this the second recorded exception to the base-layer boundary, alongside
the terminal. It is a comfortable one: Quickshell is MIT, and the build needs
nothing the archive does not already carry — `qt6-declarative-dev`, `qt6-wayland`,
`qml6-module-qtquick`, `qml6-module-qtquick-controls`, `qml6-module-qtquick-layouts`,
`qt6-base-dev`, `meson`, `ninja-build`, `cmake` and `pkgconf` are all archive
packages, so there is no vendored blob and no redistribution question. The rule the
exception sits against is the one in the manifest's header: never repackage what the
base layer carries. Quickshell is not in the base layer, so building it is not a fork.

`zenith-shell` owns that build, and it is build-only — none of those names enter the
runtime image, they are what the build needs to produce a package the image installs.
The manifest's `zenith_built:` key exists so the departure from its own rule is
visible in one place rather than implied by a comment, and
`system/verify-packages.py` checks it in the inverse direction: a `zenith_built:`
entry that turns up in the archive is a warning, because rebuilding a package the
base layer now maintains is exactly the fork the rule prevents.

The test matrix question stays open — see [open questions](#open-questions) — and it
now has more weight than it did, because a shell Zenith builds is a shell Zenith has
to rebuild when upstream moves.

### Terminal: Ghostty

Client-side rendered on the GPU, with its own renderer rather than a VTE text grid, so it costs
far less than a GTK terminal. It is the most capable terminal in the stack and it is the one
thing here Zenith is willing to accept a packaging burden for, because the terminal is the
surface every developer looks at all day.

Four places declare it the default, and all four must be set or the desktop is inconsistent:

1. **Zenith's shell** — the terminal-launcher setting the bar button calls
2. **`gsettings set org.gnome.desktop.default-applications.terminal exec ghostty`** — so the
   file manager's "Open in Terminal" and GTK dialogs use it
3. **`update-alternatives`** on `/usr/bin/x-terminal-emulator` with an explicit `--slave` and
   alternatives name — for applications that hardcode the VTE path. Ghostty's own packages do
   not register this alternative, so `zenith-terminal` owns doing it.
4. **Ghostty's zsh shell integration**, sourced from `~/.zshrc` — prompt marking via OSC 133,
   which is what the pager integration and starship's own status reporting depend on

**Packaging consequence, stated plainly:** Ghostty is carried by the base layer's archive, but
at a version behind upstream, and it is in `universe` — the component the base layer does not
treat as part of a release. So Zenith builds and pins Ghostty itself in `zenith-terminal` and
owns that build for the life of the project. That is a deliberate trade: the archive would
maintain one binary for us, and we maintain a current one ourselves. See
[the rejected options](#rejected-and-why).

**That choice is what sets the base version, and the coupling is deliberate.** Ghostty's own
version policy is *"minimum GTK/Adwaita in the most recent LTS at the time of a minor release."*
Zenith adopts that policy verbatim, which means the base version and the terminal version are
coupled on purpose rather than by accident. Ghostty 1.2.x needs GTK 4.14 and Adwaita 1.5;
1.1.x needs GTK 4.8 and Adwaita 1.2. A Zenith release that moves its base moves Ghostty's
floor in the same release.

Build dependencies `zenith-terminal` carries beyond the base layer, which are **required, not
optional**: `gtk4-layer-shell` — absent from a stock Ubuntu image — plus `libglslang`, `libhwy`,
`libsimdutf` and `oniguruma`. Also `gtk4`, `libadwaita`, `freetype`, `fontconfig`, `harfbuzz`,
`pixman`, `libpng`, `zlib`, `wayland` and `libx11` from the archive.

**One archive entry, not three.** Arch splits Ghostty's shell integration and terminfo into
`ghostty-shell-integration` and `ghostty-terminfo`. The base layer does not: Ubuntu's single
`ghostty` package ships `/usr/share/ghostty/shell-integration/zsh/ghostty-integration` and
`/usr/share/terminfo/x/xterm-ghostty` inside itself, verified against the 26.04 Contents index.
Naming the split packages would fail the build on packages that do not exist here. That same
index confirms the `x-terminal-emulator` alternative is *not* registered by the package, which is
why the third declaration above is entirely Zenith's work.

What Ghostty brings that a stock terminal does not, and why it is worth the build:

| | |
|---|---|
| GPU rendering | The whole grid is drawn client-side, so scrollback and redraw stay cheap on a Wayland session |
| Shell integration | OSC 133 prompt marking, hyperlink handling, clickable paths |
| Tab and split configuration | Tabs and splits configured declaratively, per config rather than per menu |
| `ghostty +…` CLI | `ghostty +show-config`, `ghostty +list-fonts`, `ghostty +version` — the config is inspectable and scriptable rather than a GUI-only surface |
| Single config file | `~/.config/ghostty/config`, which sits under Zenith's overlay like everything else, so `zenith update` cannot clobber it |

Default font: **JetBrainsMono Nerd Font** — for devicon and powerline glyphs, not because
ligatures require it.

### User shell: zsh

zsh is the shell, starship is the prompt, and Ghostty is where they meet.

- zsh is the login shell; Ghostty's `command` setting points at it explicitly so the session
  does not depend on what `$SHELL` happens to say
- Ghostty's zsh integration is sourced from `~/.zshrc`, guarded so a non-Ghostty terminal does
  not error on the missing file
- `starship` renders the prompt; its configuration lives under Zenith's overlay like everything
  else, so `zenith update` cannot clobber it
- fish and blesh ship alongside zsh for users who prefer them; neither is the default

Zenith's overlay model gives the shell the same protection as the rest of the desktop: a change
to `~/.zshrc` is a user's change and survives every update.

### Languages

| | Ships | Managed by | Why |
|---|---|---|---|
| Rust | `rustup`, shared toolchain at `/opt/zenith/rust` | rustup | Ubuntu's apt `rustc` lags years behind. Never ship Rust via apt. |
| Node | `nodejs` + `npm` pinned to LTS, `corepack` enabled | fnm | System Node is the floor; per-project versions per repository. |
| Python | `python3`, `python3-venv`, `python3-dev` | mise + `uv` | System Python stays pristine — it is an OS dependency and PEP 668 marks it externally managed. |

Preinstalled Rust tooling, because they are near-universal wins:
`stable` toolchain, `cargo-edit`, `just`, `sccache`, `mold`, `cargo-nextest`, `ripgrep`, `fd`.

The system shell is **zsh** with starship. fish and blesh ship alongside it for users who
prefer them.

---

## Hardware strategy

This is the largest subsystem and the one that decides whether Zenith is usable by anyone but
its author.

### Support tiers

Defined in [`../system/hardware/tiers.yaml`](../system/hardware/tiers.yaml). Tier 1 verified,
tier 2 community confirmed, tier 3 best effort. Every machine falls into exactly one, and the
tier is determined by whether a profile exists, not by marketing.

### The profile database

`zenith-machines` is a database of verified machine profiles in this repository. Nobody can test
every laptop, but every user can test one:

1. A machine boots successfully
2. `zenith hardware report` emits a candidate profile
3. The contributor attaches it to a pull request
4. Merged profiles ship in the next ISO and are selected automatically at install time

This turns an unbounded hardware problem into a union of working setups that grows without
anyone owning a hardware lab. It is the highest-leverage mechanism in Zenith.

### Kernel fleet

A single kernel cannot support both old drivers and new silicon. Zenith pins a fleet:

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
not have. This is why every Linux ISO hits the same wall. Zenith's answer is a broad firmware
rescue bundle on the ISO plus `zenith-firmware load` from a USB stick. Unglamorous, and the
difference between "installs on my laptop" and "installs on most laptops".

---

## Firmware and free software

Zenith is free software. It cannot honestly claim to be free of proprietary code *on disk*, and
the difference matters, so state it precisely.

**The distribution level.** Zenith's own code is GPL-3.0 and Zenith's own APT repository contains
nothing but free software. That claim is clean, verifiable with `apt-cache` , and true.

**The image level.** A specific machine's firmware is a separate question. Most of
`linux-firmware` is free — Intel WiFi, AMD GPU, much of MediaTek, Atheros. But NVIDIA's GSP
firmware, Broadcom's full-FMAC blob, some Realtek NVMe and WiFi firmware, and several audio
codec firmwares are proprietary binaries. Those laptops do not have a free driver to fall
back on. An image without them may not bring up networking or graphics at all.

So the honest formulation is:

> Zenith's code and Zenith's repository are free software. Some hardware requires proprietary
> firmware, which Zenith does not redistribute — it installs the base layer's own
> `linux-firmware-*` packages, where the redistributor already holds the rights.

Note what the base layer actually offers: there is **no `non-free-firmware` component**, and
neither `firmware-nonfree` nor `linux-firmware-nonfree` exists. Firmware arrives as the
`linux-firmware` metapackage, per-vendor `linux-firmware-<vendor>` packages, and
`firmware-sof-signed`. Most of that is free software — Intel WiFi, AMD GPU, much of MediaTek,
Atheros — and some of it is not.

Three consequences, all deliberate:

1. **Zenith references, it does not repackage.** The installer pulls the base layer's
   `linux-firmware-<vendor>` packages rather than carrying firmware in a Zenith package. This
   keeps Zenith's repository entirely free software and stops Zenith taking on a redistribution it
   has no right to.
2. **A hardware profile declares what it needs.** `firmware:` in a profile names *packages*.
   That is the same mechanism a community profile already uses for its driver list, and it is
   why per-machine declaration scales — see [the hard list](#the-hard-list).
3. **Nothing claims FSF eligibility.** Zenith does not describe itself as a fully free
   distribution, and no Zenith ISO should be labelled that way. `firmware-reporter` would say
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

Secure Boot is a two-key problem for Limine. Limine does **not** use UKS: it verifies
integrity by hash, not authenticity by signature. Signing the Limine EFI executable and
enrolling the **BLAKE2B hash of `limine.conf` inside it** is what enforces an unmodified
config, and once that hash is enrolled every file path in the config must carry a matching
`#<blake2b>` suffix. Separately, on Windows-initialised firmware only Microsoft keys are
enrolled, so shim must present a MOK for Zenith's key before chainloading Limine at all.

Package flow is an APT repository signed with Zenith's own GPG key, referenced by `signed-by`
in `sources.list.d`. Releases are published with signed checksums.

---

## Performance plan

Zenith ships a deliberately small daemon set, which makes the performance work subtractive rather
than desperate. In rough order of payoff:

1. Ship only what the base layer provides that Zenith actually uses — every extra package is
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

Measure before claiming a win. `zenith benchmark` should record idle RSS, boot time and CPU so
regressions are visible rather than anecdotal.

---

## What draws the login screen

The greeter runs before any user session exists, so neither Hyprland nor XWayland is available
to it. That was the open question that blocked the image from logging in at all, and it is now
settled: **SDDM in Wayland greeter mode, with a Weston kiosk compositor hosted by Zenith.**

The chain, in the order it happens at login:

| Stage | Who | What |
|---|---|---|
| Login surface appears | SDDM | `DisplayServer=wayland`, launching `CompositorCommand` rather than starting an X server |
| Something holds the DRM card | Zenith's greeter wrapper | `weston --shell=kiosk`, run at each output's top mode so the login screen matches a high-refresh session instead of the EDID-preferred 60Hz |
| Login form is drawn | Zenith | Zenith's own login surface, on the compositor above |
| Authentication | the base layer | PAM, in `sddm-session`. Zenith does not touch `/etc/pam.d/` |
| Session starts | `zenith-session` | `exec Hyprland`, then the shell's unit after the compositor |

**Why SDDM and not `greetd`.** The greeter needs a Wayland compositor of its own, because
there is no session to borrow one from. SDDM provides that seam — a `CompositorCommand` hook —
and the base layer already ships and maintains it. `greetd` draws directly onto the TTY, which
means either X11 or writing a KMS/DRM renderer into the login path; both are new code on the
critical path for no user-visible gain. Choosing `greetd` would have meant owning a display
manager to get a login screen the base layer already provides.

**Why Weston and not the Xorg server.** The obvious alternative was keeping `xorg-server` in the
image purely so the greeter could draw on it. That keeps a display server Zenith otherwise has
no use for, adds an X11 login path whose failure modes nobody would be watching, and means the
login screen renders through XWayland-era code. A kiosk Weston costs one process at login and
nothing afterwards. `modetest` reads each connector's modes without holding DRM master, so it
runs before Weston takes the card; any uncertainty — no `modetest`, nothing connected, an empty
parse — falls back to plain Weston, so **the greeter always comes up.**

This is a working answer, not a theory: Ryoku ships exactly this chain and it is the reason
this question is closed rather than open. Zenith's copy must be its own — a `zenith-greeter`
wrapper and a Zenith-drawn login surface, under a drop-in in `/etc/sddm.conf.d/` — because a
shipped file is never edited in place.

---

## Open questions

| Question | Why it matters |
|---|---|
| ~~**What draws the login screen**~~ | **Settled** — SDDM's Wayland greeter mode, with a Weston kiosk compositor hosted by Zenith. Written up in [what draws the login screen](#what-draws-the-login-screen). |
| The Secret portal | No backend in the stack implements `org.freedesktop.portal.Secret`, and `gnome-keyring` does not substitute for one. Password storage is broken in Flatpaks until one of the three options is chosen. See [the portal seam](#the-portal-seam). |
| Repository hosting | Zenith's APT repository and ISO hosting need a home. GitHub Pages serves static files; a package repo wants object storage. |
| Plugin mechanism | Zenith's shell has surfaces no stock desktop has. What a Zenith plugin looks like is undefined. |
| Hardware report scope | What `zenith hardware report` captures, and how aggressively it strips PII before emitting. |
| ISO firmware policy | Whether one ISO carries firmware or firmware is fetched at install time. See [firmware and free software](#firmware-and-free-software). |
| live-build and Limine | live-build cannot emit a Limine ISO. A custom hook, or `mmdebstrap` plus a hand-built boot chain. |
| Test matrix | Which Hyprland, Quickshell and kernel versions CI must pass before a release. |