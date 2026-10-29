# Zenith OS architecture

Why Zenith is built the way it is. Read this before designing or changing anything.

This document records **decisions, not aspirations**. Where a decision is settled it says so
and gives the reason. Where a question is open it says that too, so nobody re-litigates a
closed question or assumes an open one is closed.

The ISO, the Limine/UKI boot chain and Btrfs rollback below are decided but deferred, not shipped: the product today is the Zenith session on a stock Ubuntu Server 26.04 install, laid down by the `zenith` package (`zenith adopt --check` is the preflight), and `zenith update` rolls back through the config git snapshot.

---

## Settled decisions

| Decision | Choice | Reason |
|---|---|---|
| Base | Ubuntu 26.04 LTS | Widest firmware and DKMS coverage of any distro, and the most complete driver story available. Hardware reach is why the base exists. |
| Filesystem & Rollback | Btrfs subvolumes with automated pre-APT snapshot hooks | Enables atomic updates and instant rollback without wasting space on full system copies. |
| Compositor | Hyprland | Zenith draws its own shell and does not need a stock one. |
| Desktop shell | Quickshell (QML) — bar, dock, launcher, popouts, widgets; the lock screen is `hyprlock` and the login screen an SDDM theme. The **engine** is built by Zenith because the base layer carries no such shell; the **surfaces** are a ported Quickshell shell tree (Ryoku's, see [`NOTICE.md`](../NOTICE.md)) beside a vendored open-source Quickshell configuration, re-themed to Zenith's design language | Compositor-agnostic, proven, and the design language already lives in QML/CSS. The surfaces are upstream's and the re-theme is Zenith's, and that split is what makes a re-theme possible without a fork. See [the shell](#the-shell) and [vendored upstream components](#vendored-upstream-components). |
| Control plane & State | Go — `zenith`; `zenith-shell` (the shell's state daemon and its CLI, run as `zenithd.service`; zero-CGo); `zenith-hubd` (the Hub's data plane); and one tool each for the wallpaper, the agent, the store and the VM manager | Static binaries, cross-compiles cleanly, fast startup, with strong typing and no CGo dependencies. |
| Terminal | Ghostty, installed from the base layer's archive and configured by Zenith, never rebuilt | GPU-accelerated and client-side rendered. The archive's version is behind upstream and Zenith accepts that rather than owning a source pin, a rebuild and a release. See [terminal: Ghostty](#terminal-ghostty). |
| User shell | zsh + starship | fish ships alongside as an alternative. blesh does not — see [user shell: zsh](#user-shell-zsh). |
| Editor | Neovim | Modal, extensible, and the ubiquitous developer editor. |
| Boot chain | Limine + Unified Kernel Images (UKI) | Limine is small, modern, and proven; UKIs reduce boot complexity and improve security. |
| Login manager | `sddm` in Wayland greeter mode, with the login surface drawn by Zenith | The base layer's login manager, run as shipped, with a greeter Zenith owns. See [what draws the login screen](#what-draws-the-login-screen). |
| Desktop services | pipewire + wireplumber, tuned-ppd, iwd + NetworkManager, fprintd | Integrated audio/video, platform daemons, wireless/ wired networking, and fingerprint support. |
| Portals | `xdg-desktop-portal-hyprland` + `-gtk`, `-lxqt` for the Secret interface, plus interfaces Zenith owns itself | Hyprland-native portal backend with GTK fallback. `gnome-keyring` is installed as a daemon, but it does **not** satisfy the Secret interface; `xdg-desktop-portal-lxqt` does, and `zenith-portals.conf` names it. See [the portal seam](#the-portal-seam). |
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
- **Building the terminal.** Zenith used to build and pin Ghostty from source, and no longer
  does. The archive carries Ghostty a version behind, and a version behind is a gap the user can
  see and the archive can close; a source pin is a liability that compounds every release cycle —
  build dependencies to carry and verify on each rebase, a rebuild when upstream's shader or SIMD
  requirements move, a tag to resolve, and a `zenith-terminal` release to ship. It also handed
  Ghostty's "most recent LTS" floor a veto over `base.release`, which bought nothing. See
  [terminal: Ghostty](#terminal-ghostty).
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
- **Third-party content is content.** A vendored upstream tree, the patches on top of it and
  its licence texts are content, and they live under `/usr/share/zenith/` like every other
  file Zenith ships. They are not registrations and they are not exceptions: nothing in the
  vendored tree is written outside the two directories, and nothing outside them points at it
  except the shell's own entry point.
- Ship only what Zenith needs from the base layer. Every package in the image is a liability on
  the next rebase; the manifest of what Zenith installs is itself a reviewed artefact.

The vendored upstream tree is **content** under `/usr/share/zenith/`, and that is the whole of
its boundary story. It is not a registration: a registration is a pointer a third-party binary
must find in a fixed directory, and the vendored tree is in none of those directories — it is
read by Zenith's own shell, at a path Zenith's own package lays down. The rule that applies to
it is the ordinary one. It is replaced on update, and a change to it is a configuration value,
an overlay, or a patch — never a hand edit.

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
| `/usr/share/doc/zenith/` | The copyright file and the licence texts | Debian policy: a package's copyright file is read from `/usr/share/doc/<package>/` and nowhere else |
| `/usr/share/glib-2.0/schemas/org.zenith.*.gschema.xml` | GSettings schemas | GLib compiles schemas from the fixed schema directory |
| `/usr/lib/systemd/user/zenith-shell.service`, `zenith-portal.service`, `zenithd.service`, `zenith-wallpaper.service`, `zenith-idle.service`, `zenith-agent.service`, `zenith-ai-usage.service`, `zenith-ai-usage.timer` | User unit registration | `systemd --user` loads units from the fixed unit directory |
| `/usr/lib/systemd/user/zenith-session.target` | User unit registration | `systemd --user` loads units from the fixed unit directory |
| `/usr/share/xdg-desktop-portal/zenith-portals.conf` | Portal backend preference | The dispatcher reads it from a fixed directory |
| `/etc/sddm.conf.d/10-zenith.conf` | Login manager preferences, opt-in: no package installs it, and `zenith greeter enable` prints the command that does | SDDM reads its drop-ins from a fixed directory beside `/etc/sddm.conf` |
| `/boot/limine.conf` | Boot configuration, deferred with the boot chain | The boot loader reads its configuration from the boot volume, which the base layer's kernel packages already write to |
| `/usr/bin/zenith`, `/usr/bin/zenith-*`, `/usr/bin/zpm`, `/usr/bin/prime-run`, and the helpers `/usr/bin/cliphist-thumbs.sh`, `localsend.sh`, `stash-cobalt-server.sh`, `stash-cobalt.sh`, `stash-compress.sh`, `stash-download.sh`, `stash-install.sh` | The executables: the control tools (`zenith`, `zenith-shell`, `zenith-hubd`, `zenith-wallpaper`, `zenith-agent`, `zenith-store`, `zenith-vm`, `zpm`, `prime-run`), the window launchers, and the helper scripts the shell and the compositor call by bare name | `PATH` resolution looks nowhere else, and the shell runs under `systemd --user`, whose `PATH` has no `~/.local/bin`. This is the same constraint the rest of this list describes — a fixed location some other thing looks in — reached from the other direction: the others are files a binary *reads*, these are files the loader *runs*. Nothing about it makes the binary a registration, and nothing about it makes it acceptable for the binaries to be owned by anything but a `zenith` package. |
| `/usr/share/applications/zenith-hub.desktop`, `zenith-store.desktop`, `zenith-vm.desktop` | Desktop entries for the windows Zenith adds | The launcher and every other desktop-entry reader scan `XDG_DATA_DIRS/applications` and nowhere else |
| `/usr/share/applications/zenith-mimeapps.list` | Default applications for Zenith sessions | The XDG mime-apps lookup reads `<desktop>-mimeapps.list` from the data directories. Named for the desktop, it applies to Zenith sessions only, and the user's own `mimeapps.list` still wins |

Content stays inside the two directories: the login surface's own files belong at
`/usr/share/zenith/session/`, and everything SDDM needs to point at belongs there too. The
login theme is exactly that: it lives at `/usr/share/zenith/session/sddm/themes/zenith` and the
opt-in drop-in reaches it through `ThemeDir`, so nothing is placed in SDDM's own theme
directory. A registration is a pointer, never a place to put content — a file that grows past a
few lines of pointer belongs under `/usr/share/zenith/` with a registration pointing at it.

The list used to carry `/usr/bin/zenith-shell` and dropped it, because no such executable
existed: the shell is a QML configuration read by Quickshell, started by
`zenith-shell.service` as `quickshell -p /usr/share/zenith/shell`, and the binary that runs
it is Quickshell's. The name has since been given to something real. `/usr/bin/zenith-shell` is
the shell's Go daemon and its command line, covered by the `zenith-*` entry above; it is not
the executable that runs the QML. See [the shell](#the-shell).

**Runtime state is not a package file.** The daemon's socket lives at
`$XDG_RUNTIME_DIR/zenith-shell.sock`, and the wallpaper daemon's and the agent's sit beside it
(`zenith-wallpaper.sock`, `zenith-agent.sock`), in the per-user runtime directory. They are not
on the list because they are not files a package installs: each daemon creates its socket at
run time, in a directory no package writes to, and it goes away when the session ends. A path
that exists only while the session does cannot be a registration, and a registration is by
definition a path that exists on every boot.

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
| Shell surfaces | Quickshell, over a ported shell tree and a vendored upstream component | Bar, dock, launcher, popouts, widgets, notifications, global shortcuts. The ported components are Ryoku's ([`NOTICE.md`](../NOTICE.md)); the vendored ones are upstream's, at a pinned commit; the theme and the settings binding are Zenith's. See [vendored upstream components](#vendored-upstream-components) |
| Shell state | `zenith-shell daemon` (`zenithd.service`) | The state daemon. One socket, one protocol, one writer of `shell.json` |
| Settings store | GSettings / dconf, and `shell.json` | One writer per setting |
| Control tools | `zenith`, `zenith-shell`, `zenith-hubd` | The only writers |
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

**This is closed.** `xdg-desktop-portal-lxqt` is installed and named for Secret in
`portals.conf`, and it implements the interface.

The obvious choice was the KDE backend. It is in the archive, it implements
Secret, and keeping the no-GNOME rule was one of the three options. It is the
wrong one: it depends on `libkf6notifications6`, so carrying it would put a
second notification daemon on a desktop whose shell owns
`org.freedesktop.Notifications` — a race for a bus name, decided by which
process started second. It also pulls in `kio-fuse`, `kio6` and a dozen KF6
libraries for one interface.

`xdg-desktop-portal-lxqt` implements Secret and depends on `libfm-qt6`,
`libkf6windowsystem6` and Qt6's core, dbus, gui and widgets. Nothing it pulls in
competes with the shell.

**The configuration had been quietly wrong the whole time.** `portals.conf` named
`gtk` for Secret. gtk is installed, gtk's `.portal` file is present and readable,
and gtk does not implement Secret — so every other check passed, the file read as
though the interface were handled, and the gap stayed open. `zenith doctor` now
reads each named backend's own `.portal` file and reports an interface the
backend cannot serve.

### The contract

An application that works on a stock desktop must work on Zenith. That now holds for every portal
interface this desktop exposes, including Secret. A hardware report should ask contributors to
confirm portal behaviour, and a portal regression is a distribution-wide bug, not a cosmetic one.

`zenith doctor` must assert that the portal configuration file is actually being read: the
per-desktop file name is derived from `XDG_CURRENT_DESKTOP`, and a session that does not
declare itself correctly falls back to `portals.conf` and silently uses the wrong backends.

It must also assert that each interface named in `portals.conf` is one the named backend
**actually implements**, read from that backend's own `.portal` file. Every gap above was a
configuration that looked correct: `Secret` named `gtk`, which does not implement it, and
`Access` was never named at all. Both files read as though the interfaces were handled.

Interfaces Zenith still owns itself, because no backend provides them:

- **Camera.** No portal backend implements it; capture is Pipewire's job. **This one is currently
  unassigned** — Notifications has an owner in the shell (M6), Camera has none. `pipewire` and
  `wireplumber` are in the manifest, so the capture side of the stack is installed, but nothing
  exports `org.freedesktop.portal.Camera` to an application that asks for it. A Flatpak with a
  camera permission is refused rather than prompted. Until a backend exports it, either Zenith
  ships a small Camera backend over Pipewire, or Zenith ships none and this interface is
  documented as unsupported rather than silently failing. That choice is not made yet — see
  [open questions](#open-questions).
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

Settings live in two stores, and each has one writer.

**GSettings schemas** give typed values, validation, per-user scoping, atomic apply through
dconf, and `gsettings list-recursively` for introspection — which is exactly what a widget
catalogue needs.

- `org.zenith.desktop` — desktop-level behaviour; the idle timeouts are the keys the running
  desktop reads
- `org.zenith.shell` — the keys adoption imports into, among them the wallpaper paths and the
  favourites. The ported shell takes its look from `shell.json` instead, so every key of this
  schema is on the list of unread settings in `desktop/internal/hub`; `zenith-wallpaper` reads
  the wallpaper paths once, to choose a fresh account's first wallpaper

`zenith config set` is their only writer, and it validates a value against the key's type,
range and enum before it reaches dconf. Hand-editing dconf directly causes drift and
`zenith doctor` reports it.

**`~/.config/zenith/shell.json`** is the ported shell's look: the bar style, the dock, the
interface scale, the language and the wallpaper options. The `zenith-shell` daemon owns it, and
the Hub's pages and the welcome tour change it by asking the daemon (`settings.patch`) rather
than by writing the file.

---

## The keybinding catalogue

The keybinds are a list of lines in the compositor's configuration, and they are also a
catalogue: `session/binds.json`, installed at `/usr/share/zenith/session/binds.json`, with a
label and a category for every chord. The claim is the one the check exists to keep true:

> Every chord in the catalogue is bound in the compositor's configuration, every bind in the
> configuration is in the catalogue, and every call the configuration makes into the shell names
> a function the shell's `IpcHandler` defines.

A claim about two lists is only true if somebody compares them, and the comparison is
`zenith doctor`'s. It reports a catalogue chord with no bind, a bind with no catalogue entry,
and a call to a function the shell does not have. All three are silent at run time: a keybind
that names a function the shell has not got runs, the command succeeds, and no launcher opens,
which is what a user reports as "the shortcut stopped working" and what nothing in the journal
explains.

The catalogue is read by the shell through `zenith binds` rather than off the file, for the same
reason the settings catalogue is read through `zenith config export`: a file read twice is read
by two pieces of code, and the two agree until the day one of them is changed. The Go side
owns the format, and the shell asks it.

The compositor's configuration is not generated from the catalogue. Hyprland reads a plain
configuration file and understands no JSON, so that file has to stay a file, with its own
comma-separated syntax. The catalogue's spelling is the XKB one — `SUPER + SHIFT + {n}` — and
the two are folded together by the check rather than by anybody remembering to write them the
same way. The separator is the compositor's constraint and not a style choice.

---

## Runtime stack

### The shell

> **The lock screen is not drawn by the shell, and that is a decision, not a limit of the
> platform.** Everything else the shell owns is drawn in Quickshell. The locker is
> `hyprlock`: it is mature, it is in the archive, and it runs as a process of its own,
> apart from the shell. Quickshell as built here could draw a lock itself — it links libpam
> and has `Quickshell.Services.Pam` — so an earlier version of this box, which said there is
> no QML PAM API and that a shell-drawn lock would accept any password, was wrong.
>
> So the division is: **when** locking happens is `hypridle`'s, run by `zenith-idle.service`
> from `org.zenith.desktop`'s `idle-*` keys, and `zenith-shell lock` is the one command every
> trigger runs; `hyprlock` does the locking; the shell only draws the dim scrim (`Idle.qml`).
> hyprlock is in the archive, which under the base-layer rule settles it — the base layer
> carries it, so Zenith does not repackage it. Every competing locker is excluded, so the
> image has exactly one and which one is in front never depends on start order.

Quickshell, and Zenith builds it. The base layer carries no compositor-agnostic
shell toolkit at all — verified rather than assumed: `quickshell`, `ags`, `eww` and
`ironbar` are in no component of 26.04. What the archive does carry is *panels* —
`waybar`, `yambar`, `tint2`, `polybar`, `xfce4-panel` — and a panel is not a
substitute. A panel draws a bar. Zenith's shell hosts the bar, dock, launcher,
popouts, widget catalogue, notification server, idle screen, quick settings
and status icons, and the Hub window as well, in one compositor-agnostic toolkit,
so that the desktop has one UI toolkit and one design system rather than two.

**The engine is what Zenith builds; the surfaces are what Zenith has adopted.** Those are
two different statements, and the licence depends on the difference, so they are kept apart
here as well. The engine is Quickshell: the binary, built by Zenith, shipped unmodified,
**LGPL-3.0-only**. The surfaces are the QML — some of it Zenith's own, a ported tree from
Ryoku (**GPL-3.0**, with the MIT portions [`NOTICE.md`](../NOTICE.md) records) and a vendored
upstream component under **GPL-3.0**. The composed shell is GPL-3.0. See
[vendored upstream components](#vendored-upstream-components) for the component, the pin and
the three overlay files, and [`LICENSING.md`](../LICENSING.md) for the obligations.

That makes this the recorded exception to the base-layer boundary, and it used to have a
companion: the terminal, which Zenith also used to build. The terminal is not built any more —
see [terminal: Ghostty](#terminal-ghostty) — so this exception is now the only one, and the
shell is the only thing in the stack Zenith compiles for itself. It is a comfortable one:
Quickshell is **LGPL-3.0-only**, and the build needs nothing the archive does not already
carry — `qt6-declarative-dev`, `qt6-declarative-private-dev`, `qt6-base-private-dev`,
`qt6-base-dev`, `qt6-wayland`, `ninja-build`, `cmake` and `pkgconf` are all archive
packages. Redistribution under LGPL-3 is a matter of shipping the licence and the copyright
with the binary, which the package does; there is no question of deriving from it, because
Zenith ships the binary unmodified and links nothing of its own into it. The rule the
exception sits against is the one in the manifest's header: never repackage what the base
layer carries. Quickshell is not in the base layer, so building it is not a fork.

**"There is no vendored blob" was true of the engine and false of the shell.** The sentence
used to end with those words, and it was right about Quickshell and wrong about everything
the shell draws. There is now a vendored tree of several hundred files under
`shell/modules/` and `shell/services/`. It is GPL-3.0, it is declared, it is pinned, and it
is byte-identical to its upstream commit — which is what makes it a dependency Zenith has
taken rather than a fork Zenith has made. The two departures from the base-layer rule are
now of different kinds and are recorded separately: one is a binary Zenith compiles, the
other is source Zenith adopts.

**`zenith-shell` is the daemon, and `zenithd` is the unit that runs it.** Two things in this
project were called `zenith-shell`, and they were not the same thing, which is the kind of
collision that produces a bug report rather than a fix. The name was once the package that
builds Quickshell; it now belongs to the shell's Go daemon. `/usr/bin/zenith-shell` is that
binary: `zenith-shell daemon` is what `zenithd.service` runs, and every other
`zenith-shell <verb>` (`lock`, `hub open`, `reload`, `icons`) is a client that sends one line
to it. The unit that runs the QML keeps its name, `zenith-shell.service`, and the shell's own
configuration is `/usr/share/zenith/shell/`, read by `quickshell -p /usr/share/zenith/shell`
from that unit. So `zenith-shell` serves the shell's state and Quickshell is the engine that
draws it; where this document says `zenithd`, it means the daemon. The manifest's
`zenith_built:` key exists so the departure from its own rule is
visible in one place rather than implied by a comment, and
`system/verify-packages.py` checks it in the inverse direction: a `zenith_built:`
entry that turns up in the archive is a warning, because rebuilding a package the
base layer now maintains is exactly the fork the rule prevents.

The wire protocol the daemon speaks keeps the name `zenith-shell/1`, after the shell it
serves rather than after the process that implements it. A protocol name that changed with
its implementation would break every client that had already learned it, and the shell is
the thing that will still be there when the daemon is replaced. Its framing is the topic
stream and the `call` of `desktop/cmd/zenith-shell/CONTRACT.md`, over
`$XDG_RUNTIME_DIR/zenith-shell.sock`; [`shell-daemon.md`](shell-daemon.md) says how it runs.

The test matrix question stays open — see [open questions](#open-questions) — and the
re-vendoring cost has joined it. A shell Zenith builds is a shell Zenith has to rebuild when
upstream moves: a new Quickshell release is a new build, a new package version and a new
binary to put through every kernel and every Hyprland in the matrix. A vendored component is
a different kind of debt. It is not rebuilt — it is *re-vendored*, which is one command,
`sh shell/vendor/vendor.sh update ii <commit>`, followed by a sync and a verify. The cost is
not the move; it is what the move can break. A new upstream commit can rename the properties
the vendored components read, and the bridges — the palette bridge and the settings bridge
— are exactly the parts that would notice, because a QML binding to a property that no
longer exists is a runtime warning and not a load error. So a re-vendor is a *re-theme*
event as much as a code event, and it has to be checked against the same matrix the engine is
checked against. That is why the matrix question now covers two things rather than one.

### Terminal: Ghostty

Client-side rendered on the GPU, with its own renderer rather than a VTE text grid, so it costs
far less than a GTK terminal. It is the most capable terminal in the stack, and it is the one
thing here Zenith is willing to give up currency for, because the terminal is the surface every
developer looks at all day and a version behind is a far smaller cost than a permanent build.

Four places declare it the default, and all four must be set or the desktop is inconsistent:

1. **Zenith's shell** — the terminal-launcher setting the bar button calls
2. **`gsettings set org.gnome.desktop.default-applications.terminal exec ghostty`** — so the
   file manager's "Open in Terminal" and GTK dialogs use it
3. **`update-alternatives`** on `/usr/bin/x-terminal-emulator`, registered under an explicit
   alternatives name — for applications that hardcode the VTE path. No `--slave` is needed or
   wanted: Ghostty is not itself a registered alternative here, and registering it as a slave of
   something it does not implement would give the user a second way to be wrong. Ghostty's own
   package registers no alternative at all, so `zenith-terminal` owns registering the only one.
4. **Ghostty's zsh shell integration**, sourced from `~/.zshrc` — prompt marking via OSC 133,
   which is what the pager integration and starship's own status reporting depend on

**Packaging consequence, stated plainly:** Ghostty is carried by the base layer's archive, at a
version behind upstream, in `universe` — the component the base layer does not treat as part of
a release. Zenith installs that package as-is. It does not build Ghostty, does not pin a source
tag, and does not own a rebuild. `zenith-terminal` is a configuration package, not a build.

This is the decision that most distinguishes Zenith from a typical "I compile my dotfiles" setup,
and it is worth stating as a trade rather than a virtue. Zenith gives up a current terminal. It
buys back every cost a source pin accrues: no build dependencies to carry and verify on each
rebase, no breakage when Ghostty's shader or SIMD dependencies move, no source tag to resolve
when upstream tags, and no `zenith-terminal` release to ship when Ghostty patches a release. A
version behind is a gap the user can see and the archive can close. A build is a liability that
compounds every release cycle and needs a maintainer forever. The gap is the cheaper one, and
the terminal is not the surface that decides whether this desktop is usable.

**The base version is not set by the terminal.** Ghostty's own version policy is *"minimum
GTK/Adwaita in the most recent LTS at the time of a minor release."* A distribution that built
Ghostty would adopt that policy and inherit a hard constraint: Ghostty 1.2.x needs GTK 4.14 and
Adwaita 1.5, 1.1.x needs GTK 4.8 and Adwaita 1.2, so moving the base would move Ghostty's floor
in the same release and `base.release` would stop being a free parameter. Zenith does not build
it, so it does not adopt that policy, and the floor belongs to the archive. `base.release` is
chosen for hardware reach and nothing else constrains it.

**No build dependencies, because there is no build.** Zenith used to carry Ghostty's build
requirements explicitly — `gtk4-layer-shell`, `glslang-dev`, `libhwy-dev`, `libsimdutf-dev`,
`libonig-dev` and ten more, every one a name that had to be verified against the 26.04 indexes
because every one of them is spelled differently on this archive than in Ghostty's upstream
docs. That list is gone, and its absence is the record of the decision rather than an omission.
This is also why the terminal is no longer an entry in the manifest's `zenith_built:` key: a
package the base layer carries is installed, not rebuilt. See
[the rejected options](#rejected-and-why).

**One archive entry, not three.** Arch splits Ghostty's shell integration and terminfo into
`ghostty-shell-integration` and `ghostty-terminfo`. The base layer does not: Ubuntu's single
`ghostty` package ships `/usr/share/ghostty/shell-integration/zsh/ghostty-integration` and
`/usr/share/terminfo/x/xterm-ghostty` inside itself, verified against the 26.04 Contents index.
Naming the split packages would fail the build on packages that do not exist here. That same
index confirms the `x-terminal-emulator` alternative is *not* registered by the package, which is
why the third declaration above is entirely Zenith's work — and why that declaration is
configuration rather than packaging.

What Ghostty brings that a stock terminal does not, and why it is the terminal:

| | |
|---|---|
| GPU rendering | The whole grid is drawn client-side, so scrollback and redraw stay cheap on a Wayland session |
| Shell integration | OSC 133 prompt marking, hyperlink handling, clickable paths |
| Tab and split configuration | Tabs and splits configured declaratively, per config rather than per menu |
| `ghostty +…` CLI | `ghostty +show-config`, `ghostty +list-fonts`, `ghostty +version` — the config is inspectable and scriptable rather than a GUI-only surface |
| Single config file | `~/.config/ghostty/config`, which sits under Zenith's overlay like everything else, so `zenith update` cannot clobber it |

Default font: **JetBrains Mono** — for the terminal's own text. It is deliberately *not* called a
Nerd Font, because it is not one: `fonts-jetbrains-mono` on this base layer is the upstream build
and ships no patched glyphs, and no Nerd Font package exists in any component of 26.04 under any
name. Verified by extracting the package rather than by reading its description. The devicon and
powerline glyphs the shell's dock and status area want therefore have no archive source, and
which way to get them is an [open question](#open-questions) — an archive icon font, or a
documented boundary exception.

### User shell: zsh

zsh is the shell, starship is the prompt, and Ghostty is where they meet.

- zsh is the login shell; Ghostty's `command` setting points at it explicitly so the session
  does not depend on what `$SHELL` happens to say
- Ghostty's zsh integration is sourced from `~/.zshrc`, guarded so a non-Ghostty terminal does
  not error on the missing file
- `starship` renders the prompt; its configuration lives under Zenith's overlay like everything
  else, so `zenith update` cannot clobber it
- fish ships alongside zsh for users who prefer it; neither is the default
- blesh does not ship, and this is a decision rather than an omission. It is absent from every
  component of the base layer, and it belongs with mise, fnm and uv in the setup step's offer: an
  alternative shell is something a user asks for at setup, not something every machine carries. See
  the manifest's `terminal-and-shell` group, which is the authority.

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

---

## Vendored upstream components

Zenith's shell draws some of its surfaces with a vendored open-source Quickshell
configuration. The tree is checked in, held byte-identical to a pinned upstream commit, and
re-themed by three overlay files that Zenith owns. This section is the record: what is
vendored, what is not, what may be changed, and who owns what.

### What is vendored, and at which commit

| Component | Upstream | Licence | Pinned commit | Vendored as |
|---|---|---|---|---|
| The "ii" Quickshell configuration | [`end-4/dots-hyprland`](https://github.com/end-4/dots-hyprland), the `dots/.config/quickshell/ii` directory | GPL-3.0-only | `2f2a376fdb2ad738bf55f693dd0b474b64bb0ab8` | `shell/modules/`, `shell/services/` |
| The rounded-polygon widget kit | [`end-4/rounded-polygon-qmljs`](https://github.com/end-4/rounded-polygon-qmljs) | Apache-2.0 | `e31ec4cb4ebf6a46b267f5c42eabf6874916fa16` | `shell/modules/common/widgets/shapes/` |

The second is upstream's own submodule, vendored as a component in its own right because
`MaterialShape.qml` — one of the most used widgets in the kit — imports it.

The manifest at `shell/vendor/manifest.yaml` is the record: per component, the upstream name,
the URL, the path inside the upstream repository, the pinned commit, the licence, the licence
file, the copyright line, and a hash for every file. `sh shell/vendor/vendor.sh verify`
compares the tree against those hashes; a file edited by hand fails it, and nothing else
would.

### What is not vendored, and why

The vendored tree is a **subset** of upstream's configuration, and the omissions are
decisions rather than accidents.

| Upstream | Why it is not here |
|---|---|
| The shell root — upstream's `shell.qml` | The shell root is Zenith's. Zenith's shell is a different shell that happens to draw some of the same surfaces, and upstream's root wires upstream's panels, upstream's services and upstream's state |
| The settings window — upstream's `settings.qml` | The settings surface is the Hub, and the Hub is Zenith's |
| Upstream's `Config.qml` and `Appearance.qml` | Replaced by Zenith's own at the same paths. See [the theme boundary](#the-theme-boundary) |
| Upstream's `GlobalStates.qml` | Replaced by Zenith's own at the same path, for the same reason and one more |
| Upstream's assets, defaults, panel families, scripts, translations, welcome screen, kill dialog and reload popup | None of it is Zenith's desktop. Upstream's icons and images are upstream's; Zenith's are its own |
| Upstream's compositor configuration, including its Hyprland Lua | Zenith owns its own compositor configuration. The vendored tree is a Quickshell configuration and nothing else |
| Upstream's licence files in place, and its examples | The licence texts are kept under `shell/vendor/` and the examples are not shipped. A file named `LICENSE` inside the tree would be caught by this repository's own licence rule, which is about Zenith's licence. See [licence hygiene](../LICENSING.md#the-rule-for-vendored-licence-files) |

### What is vendored but not taken

The tree carries more than Zenith's shell uses, and the extra is upstream's rather than
Zenith's. The AI assistant, the booru image service, the weather service, the Google Cloud
translation and vision services, and upstream's own lock surface with its PAM plumbing are all
in the tree, byte-identical and listed in the manifest — and none of them is part of Zenith's
shell.

They are there because removing them would be an edit to the vendored tree, and an edit to the
vendored tree is exactly what the byte-identical rule forbids. What Zenith does instead is not
wire them up. The lock is the clearest case: Zenith's lock is `hyprlock`, the shell decides
*when* to lock and never draws a lock surface, and upstream's lock components are vendored
because they live upstream's directory and not because Zenith wanted them.

### The byte-identical rule

Every file in the vendored tree is byte-identical to the file at the pinned upstream commit,
and that is checked rather than asserted.

The reason is the same rule that governs every other shipped file in this project, applied to
files that are not Zenith's: a shipped file is replaced on update, so a hand edit is destroyed
by the next sync, cannot be re-vendored from upstream, and makes the manifest — a record of
what changed — lie.

### The three homes for a change

| Home | What it is | When it is the answer |
|---|---|---|
| A **configuration value** | A setting the component reads | The component reads the setting and the setting is what you want to change. Upstream's components read `Config.options.*` for exactly this, and Zenith's bridge answers that shape |
| An **overlay file** | A Zenith-owned file at the path the component expects | The component's *interface* is the thing that has to change. The palette bridge, the settings bridge and the shell-state bridge are all overlays, because the file is the interface |
| A **patch** in `shell/vendor/patches/` | A diff applied on top of the pinned tree | The component's behaviour is wrong and no setting reaches it. There is no patch there today, because no change has needed one yet |

### The theme boundary

This is the part worth reading twice, because "strict monochrome" and "the vendored tree is
byte-identical" look like a contradiction and are not.

**Enforced by configuration.** Everything upstream's components read through `Appearance` is
Zenith's: the palette, the layer ramp, the rounding, the spacing, the motion curves. A
component that asks for `Appearance.colors.colPrimary` gets Zenith's monochrome ink; one that
asks for `Appearance.rounding.normal` gets Zenith's radius scale. The components name
Material 3 property names, and those names are Zenith's answers.

The bridge is **complete**, and that is a decision rather than tidiness. Upstream's file
names every colour it uses, so Zenith's does too: a component that reads a name the bridge
does not have gets `undefined`, and `undefined` assigned to a colour is a binding error in
the journal — a failure, but a quiet one, on a surface nobody may be looking at. A complete
set costs one line per name and cannot fail that way.

The vendored files do carry hex literals — thirty-four of them — and twenty-seven are
fallbacks written as `Appearance?.colors.colPrimary ?? "#65558F"`: the value to use when the
palette is not there. The palette is always there, because Zenith supplies it, so those
literals are never the colour that is drawn. Zenith's own overlay carries two more, and both
are `#000000` — the shadow and the scrim, which are black on a monochrome desktop anyway.

A handful name a colour outright, and that is the finding a strict monochrome rule has to
record rather than paper over. `DashedBorder.qml` draws `#ffffff`, `RoundCorner.qml` draws
`#000000`, `SineCookie.qml` draws `#605790`, `ShapeCanvas.qml` draws `#685496`, and
`LightDarkPreferenceButton.qml` mixes three literals into a hue. Black and white are inside
the rule. The two Material purples are not, and they are upstream's, in a tree Zenith does
not edit. That is a patch, or a bridge that overrides them, and until one of those happens
there are two colours on a desktop that has no colours in it. It is recorded here rather
than left to be found in a screenshot.

**Enforced by overlay.** Two things could not be reached by configuration, and both are cases
where the file *is* the interface.

1. *The palette itself.* Upstream's `Appearance.qml` derives its colours from a Material 3
   scheme and from the wallpaper, through `ColorQuantizer`, `MaterialThemeLoader.qml` and
   `AdaptedMaterialScheme.qml`, and it reads the wallpaper path out of the configuration.
   Zenith's design system says the wallpaper does not drive the drawing — the token set is
   canonical, and the wallpaper palette is a record of what the wallpaper would have said.
   That cannot be a setting, because the palette is the thing being set. It has to be a file.
   Hence `shell/modules/common/Appearance.qml`: a monochrome token bridge that answers
   upstream's property names with Zenith's tokens.
2. *The settings store.* Upstream's `Config.qml` is a JSON store — a `JsonAdapter` over a
   file in the shell's own configuration directory, written by the shell itself. Zenith's
   settings live in GSettings and are written by one tool. That cannot be a setting either:
   the store is the thing being replaced. Hence `shell/modules/common/Config.qml`: a
   GSettings-backed bridge that exposes upstream's `options.*` shape.

And a third overlay, which is not a theme question at all: `shell/GlobalStates.qml`.
Upstream keeps its open-surface flags in a singleton at the shell root, and the vendored
components read it — the bar asks whether the overview is open before it draws itself over
it, the dock asks whether the screen is locked before it hides. Upstream's own file is not
vendored for two reasons: it is upstream's shell root, and the shell root is Zenith's; and it
imports the whole service graph, which closes a cycle through the `modules/` directory — the
services reach back into the modules, and the modules reach back into the root. So Zenith
supplies its own at the same path, carrying the same flags.

The three overlays are therefore the entire re-theme and the entire re-bind.

### The settings binding

The chain is four links, and it has one writer.

```
org.zenith.shell  →  the settings bridge  →  upstream's Config.options.*  →  the surface
```

- The **store** is `org.zenith.shell`, with `org.zenith.desktop` beside it.
- The **bridge** is `shell/modules/common/Config.qml`: it answers upstream's `options.*`
  shape from the store, so a vendored component asks for a setting the way upstream taught it
  to and gets Zenith's value.
- The **surface** binds to the property and redraws. It never writes.
- The **writer** is `zenith config set`, which the Hub calls, and nothing else. A vendored
  component binds; it does not write. `zenith config set` validates the
  value against the schema's type, range and enum before it is stored — because a value that
  is well-formed GVariant and outside a declared range is accepted by GSettings, committed to
  dconf, and then read by a surface that does something nobody asked for.

One writer per setting is what stops a component written by somebody else from becoming a
second writer in the middle of a desktop.

The settings bridge is **deliberately partial**, and the partiality is the point. Upstream's
file carries three hundred and seventy-nine options, most of them for surfaces Zenith does
not adopt — an assistant's model, an image viewer's tags, a cheatsheet's keybindings — and
those are not missing by oversight: an option for a surface that does not exist is a setting
nobody can act on. What the bridge carries is what a component would read to draw. When one
of those surfaces is adopted, its options are added to the bridge and bound to the settings
the desktop keeps.

### The licence position

The engine is LGPL-3.0-only and shipped unmodified. The surfaces are GPL-3.0, because the
"ii" component is GPL-3.0-only and the composed shell is a modified work of it. The shapes kit
is Apache-2.0 and is not modified at all. The obligations, the section-by-section map and the
corresponding-source argument are in
[`LICENSING.md`](../LICENSING.md#the-vendored-components); what matters architecturally is the
consequence: **the corresponding source of the composed shell is this repository**, because the
vendored tree is byte-identical and the overlays are in it. A fork would have made the
corresponding source a diff. Keeping the tree pristine makes it a checkout.

### Re-vendoring, and what it costs

The procedure is one command and a check:

```sh
sh shell/vendor/vendor.sh update ii 2f2a376fdb2ad738bf55f693dd0b474b64bb0ab8
sh shell/vendor/vendor.sh verify
```

`update` moves the pin and re-syncs; `verify` is the check that the tree is byte-identical to
the new pin.

The cost is not the move. A vendored tree is not a fork, so there is no merge to reconcile:
the new commit's files land, and Zenith's changes — the three overlays — land on top of them.
The cost is that **the bridges are the contract, and nothing checks the contract.** The
shell's own QML is checked against the schema and against the engine; the vendored QML is
exempt from linting, because it is upstream's. So a commit that renames
`Appearance.colors.colPrimary`, or renames a key in `Config.options`, breaks every component
that reads it — silently, at run time, with a warning in the shell's log that nobody reads.

Which is why a re-vendor is a re-theme event and not only a code event, and why it has to be
checked against the same matrix as the engine: read the manifest's diff, read the two bridges
against the new property names, and load the shell. The three overlays are the only three
files a re-vendor can break, and they are the three files a re-vendor has to be reviewed
against.

### Who owns what, capability by capability

| Capability | Owner | Why |
|---|---|---|
| The notification server | **Zenith** | `org.freedesktop.Notifications` is a session-scoped bus name and no daemon in the image holds it, so the shell draws it and hosts the server |
| The lock screen | **Zenith** owns when; `hyprlock` does the locking | `hypridle`, run by `zenith-idle.service` from the `org.zenith.desktop` `idle-*` keys, decides *when*, and `zenith-shell lock` is the one command every trigger runs; the locker owns the surface and the PAM conversation. `hyprlock` is a decision (mature, in the archive, a process of its own), not a platform limit: Quickshell here has `Quickshell.Services.Pam` |
| The idle policy | **Zenith** | The timeouts live in `org.zenith.desktop`, `zenith config set` is their only writer, and `zenith-idle.service` hands them to hypridle |
| The keybind catalogue | **Zenith** | `session/binds.json`, installed at `/usr/share/zenith/session/binds.json`, checked in both directions against the compositor's configuration by `zenith doctor` |
| The Hub | **Zenith** | The settings application: Ryoku's Hub, ported (`shell/hub/`) — a grouped navigation rail, a global search and a page per area of the desktop. The look pages change `shell.json` by asking the `zenith-shell` daemon, the compositor and machine pages go through `zenith-hubd`, and the **Zenith** group draws what `zenith config export` hands it (the pages, the keys, their types, their permitted values, their ranges and their current values) and writes only through `zenith config set`, never through GSettings. Its chrome is the shell's own component kit, so both windows are one design system |
| The wallpaper | **Zenith** | `zenith-wallpaper` chooses it (the catalogue, the choice per output, playlists, the day/night schedule) and the shell's own surface draws what the daemon publishes. `org.zenith.shell`'s `wallpaper-path` and `wallpaper-path-dark` only seed a fresh account's first wallpaper |
| The palette, the settings binding, the shell's own state | **Zenith** | The three overlays, at upstream's paths |
| The bar, the dock, the launcher and the popover surfaces | **upstream** | The components, byte-identical to the pinned commit |
| The overview, the media controls, the on-screen display, the cheatsheet | **upstream** | The components, byte-identical to the pinned commit |
| The vertical bar, the background, the rounded-polygon widget kit | **upstream** | The components, byte-identical to the pinned commit |
| The service graph — audio, network, bluetooth, battery, media, tray | **upstream** | The components, byte-identical to the pinned commit |

A row says who owns the *capability*, not who draws the pixels. The rows that say *upstream*
describe the vendored end-4 tree, which stays byte-identical to its pinned commit. The
surfaces the running shell draws — the bar, the dock, the launcher, the overview, the
on-screen display, the notifications and the wallpaper — and its service graph
(`shell/Services/`) are Ryoku's ported components, adapted in place and recorded in
[`NOTICE.md`](../NOTICE.md). The shell root (`shell/shell.qml`) hosts them and wires them
together, and Zenith owns the desktop they appear on.

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

**Ryoku's machine tuning is deliberately not ported.** The desktop was ported from a distribution
that tunes each machine under its own `system/hardware/` — udev rules, NetworkManager and
Bluetooth settings, GPU and power setup. None of it came across. Machine-specific tuning is the
hardware profile system's job, and the base-layer boundary forbids a package writing those
paths. Whether some of it should return as profile content is an open question for the user; see
[open questions](#open-questions).

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

### The icon font

The base layer carries no Nerd Font in any component of 26.04, and
`fonts-jetbrains-mono` is the unpatched upstream build — 2.304+ds-6, shipping
`JetBrainsMono*.ttf` and `JetBrainsMonoNL*.ttf` only, with no `*Nerd*` file in the
package. So the devicon and powerline glyphs a shell's dock and status area want
have no archive source *under those names*.

They do have archive sources under other names. The whole font set was
enumerated; what 26.04 carries is `fonts-font-awesome`, `fonts-fork-awesome`,
`fonts-material-design-icons-iconfont`, `fonts-materialdesignicons-webfont`,
`fonts-elusive-icons`, `fonts-circos-symbols`, `fonts-symbola`,
`fonts-opensymbol`, `fonts-powerline`, `fonts-glyphicons-halflings` and
`fonts-weather-icons`.

`fonts-font-awesome` is the choice: Font Awesome 4.7 covers the entire status
set — bluetooth U+F293, battery U+F240–F244, wifi U+F1EB, the volume pair at
U+F026–U+F028, lock U+F023 — in 1.4 MB with **no dependencies at all**, which is
why it beats `fonts-fork-awesome`, whose newer glyph set is not worth pulling a
package in for.

This is also the honest answer rather than the comfortable one. A Nerd Font is a
patched font plus a redistribution obligation, and this project already has a
firmware group precisely so that it is not the distributor of blobs. An icon
font from the archive has neither problem, and nothing about a shell's status
area needs the several thousand devicon glyphs a Nerd Font exists to carry.

Every glyph the shell draws is declared in one file, `Icons.qml`, as a `\uXXXX`
escape with the family it comes from, and the shell check enforces three things
about it: that no glyph is used outside that map, that each is written as an
escape rather than as a raw character, and that each codepoint is inside its
family's real range (Font Awesome 4.7 is F000–F2E0; Powerline is E0A0–E0D4).

The last of those exists because it was violated. An earlier draft used U+F8FF
for bluetooth and U+F8C0 for battery — real private use codepoints, in the same
neighbourhood as the correct ones, and neither of them a Font Awesome 4.7 glyph.
A codepoint a font does not contain renders as a hollow box: no error, nothing in
the log, and a status area that reads as decorative rather than broken.

**The vendored components expect a font the base layer does not carry.** This is the one
place where the vendored tree's expectations and the base layer's inventory do not meet, and
it is worth stating plainly rather than leaving to be discovered.

Upstream names its icons the Material Symbols way — `battery_android_full`,
`signal_wifi_4_bar`, `headphones` — and draws them through a `MaterialSymbol` widget that
asks for the "Material Symbols Rounded" family. No component of 26.04 provides that family.
The archive does carry two material icon packages —
`fonts-material-design-icons-iconfont` and `fonts-materialdesignicons-webfont` — and
neither is the Material Symbols family the components ask for. Zenith does not add one: the
archive's Font Awesome 4.7 and Powerline are what the desktop draws with, and the argument
for taking a font from the archive rather than shipping one applies to a vendored component
exactly as it applies to Zenith's own surfaces.

The consequence is not that the icons look different. It is that they do not resolve. A
Material Symbols *name* is a ligature, and Font Awesome 4.7 is a codepoint font with no
ligatures at all — so `battery_android_full` has nothing behind it in the font Zenith
carries, and the shell's own `Icons.qml` is the only place a codepoint may be declared.
A vendored icon name that has no entry in that map is a hollow box, in exactly the way the
U+F8FF draft was, and for the same reason: the font does not contain the glyph.

Which makes it a finding about the component rather than a licence to add a font. Upstream's
icon names are upstream's, the desktop's font is Zenith's, and they do not match. What
resolves it is a mapping from upstream's names onto the codepoints the desktop already has,
and that mapping belongs in the bridge — the same place the palette and the settings are
bridged — rather than in the component.

**Why Weston and not the Xorg server.** The obvious alternative was keeping `xorg-server` in the
image purely so the greeter could draw on it. That keeps a display server Zenith otherwise has
no use for, adds an X11 login path whose failure modes nobody would be watching, and means the
login screen renders through XWayland-era code. A kiosk Weston costs one process at login and
nothing afterwards. `modetest` reads each connector's modes without holding DRM master, so it
runs before Weston takes the card; any uncertainty — no `modetest`, nothing connected, an empty
parse — falls back to plain Weston, so **the greeter always comes up.**

This is a working answer, not a theory, and the reason it is closed rather than open is
technical rather than rhetorical. SDDM's Wayland greeter mode needs a compositor to hold the
login surface, the base layer carries one, and the only question left was who hosts it. A
kiosk Weston is a process that starts at login and exits at logout, it needs no session, and
it lets the login surface be drawn by the same toolkit the desktop is drawn by. Zenith's copy
must be its own — a `zenith-greeter` wrapper and a Zenith-drawn login surface, under a
drop-in in `/etc/sddm.conf.d/` — because a shipped file is never edited in place.

---

## The mark

<p align="center">
  <img src="../public/zenith-logo.svg" alt="Zenith OS" width="150">
</p>

天 — read *ten* — is heaven: the highest point of the sky. A zenith is that same point. The
mark and the name say one thing, and they say it about the architecture as much as about the
project: this is the top of the stack, and it sits above everything it is built on.

| File | Use |
|---|---|
| `public/zenith-logo.svg` | Black, for light backgrounds |
| `public/zenith-logo-on-dark.svg` | White, for dark backgrounds |

Both are the same drawing. That is enforced, not assumed: `ci.yml` fails if the two files'
path geometry differs by a single character or if they differ by anything other than fill
colour. A logo that has a light variant and a dark variant of two different shapes is two
logos, and only one of them was reviewed.

The paths were traced from the original 500x500 raster at the 50% alpha line, with the
crossings located to sub-pixel precision by bilinear interpolation, simplified with
Douglas-Peucker and fitted with adaptive cubic Beziers. The result agrees with the source
within 27 pixels of 250,000 — 0.011% of the canvas, 0.06% of the inked area — and that is
checked by re-rasterising the SVG and comparing it to the original rather than by asserting
it. A codepoint or a contour invented from memory is a logo nobody looked at, and every other
number in this document was earned by measuring something; this one is no different.

The mark is drawn nowhere it is installed as a raster. Anything that needs it at runtime reads
the SVG.

## Writing a backlight

The sidebar needs to dim the screen, and a user who is not in the `video` group cannot write
the panel's backlight.

`/sys/class/backlight/*/brightness` is `root:video`, mode `0664` (measured on the development
laptop: `-rw-rw-r-- 1 root video`), and Ubuntu's `brightnessctl` is built without logind
support, so `brightnessctl set` fails with `Can't modify brightness: Permission denied` for
every user outside `video`. Reading works unprivileged, writing does not.

**This is settled, and the answer is neither of the two this section used to weigh.**
systemd-logind writes it: `org.freedesktop.login1.Session.SetBrightness(subsystem, name, value)`
sets the value, as root, for the owner of the active session, for both `backlight` and `leds`.
`zenith brightness` makes that call, then tries `brightnessctl`, then the sysfs file if it is
writable (`desktop/internal/backlight`), and the keys, the daemon and the sliders all go
through it as `zenith-cmd-brightness`. It needs no group, no udev rule and no authentication
prompt. A udev rule would have been a registration outside `/usr/share/zenith/` and
`/etc/zenith/`, which is a change to the boundary rather than a change inside it, and polkit
elevation would have put a prompt on every brightness change. Neither is needed, and the
boundary did not move. See [`hardware-keys.md`](hardware-keys.md).

## Window switching is the shell's job here, not Hyprland's

`session.md` puts the window switcher with the compositor's own bindings. It is
recorded there as an exception now, and this is why.

Hyprland has nothing to bind to. Across `v0.53.3` (what 26.04 ships),
`v0.54.3`, `v0.56.2` and `main`, there is no `tab` bind in any shipped default
config and no compositor-internal Tab handling, and the official plugins
repository ships four plugins, none of them a switcher. The built-in `cycleNext`
walks `m_state.windows()` — layout order, active workspace only. That is neither
most-recently-used nor cross-workspace.

Hyprland *has* the capability: `CWindowQuery::cycleHistory` walks
`windowTracker()->fullHistory()` with the floating and fullscreen filters, in
both directions, and is correct. It has zero call sites in every version checked.
So this is a capability upstream built and did not wire up, and the shell supplies
the wiring rather than the algorithm.

Two version facts, both load-bearing and both verified against the archive rather
than assumed:

| Fact | Consequence |
|---|---|
| 26.04 ships `hyprland 0.53.3` | Pre-Lua. hyprlang `bind =` syntax, and `hyprctl dispatch focuswindow address:0x…` is used bare. From 0.55 a Lua config wraps dispatch in `hl.dispatch({window=…})`, which is a different argument and a quoting rule |
| `focusHistoryID` is a rank, not a timestamp | Snapshot the list once on open. Focusing a window mutates the id immediately, so re-querying per keypress makes this press change what the next press means |

`focusHistoryID` is also inverted relative to intuition, and this is the single
easiest thing to get wrong in the whole feature. Hyprland's history vector is
ordered oldest to newest (`WindowHistoryTracker.hpp`: *old -> new, .front() is
oldest*) and the id is computed as `size - index - 1`, so the window you were
just in is `0` and **lower is more recent**. Sorting descending — the obvious
thing — is least-recently-used, and looks correct until you press it twice. `-1`
means never focused and is pushed to the end rather than the front.

**The keys.** Alt+Tab is the switcher: a press moves to the next window and focuses it,
Alt+Shift+Tab walks the list the other way, and the release closes it through `bindr`, with
the card kept up for 450 ms after the last press. Super+Tab is not the switcher; it opens the
workspace overview, and Super+Alt+Tab steps the overview's desktops
(`session/hyprland/binds.conf`). The switcher used to sit on Super+Tab and close on `bindl`,
and `l` is Hyprland's *locked* flag and not "release", so the card opened and closed on the
same press ([the journal](journal/2026-10-23.md)).

## Why the shell subscribes to a daemon

`zenith state` probes five command-line tools and the sidebar redraws from its JSON.
Every other widget in the shell reads a Quickshell service singleton instead —
`PulseAudio.sinks` in `Widgets/Volume.qml`, `NetworkManager.status` in
`Widgets/Network.qml`, `Hyprland.workspaces` in `Widgets/Workspaces.qml`. Those are
event-driven and they push changes; this pulled and re-ran five subprocesses every
two seconds.

It does not any more. The daemon is `zenith-shell daemon`, run as `zenithd.service`: a streaming
Go daemon that speaks `zenith-shell/1` — line-framed JSON over a Unix socket at
`$XDG_RUNTIME_DIR/zenith-shell.sock` — and the shell subscribes to the topics it publishes.
The daemon runs its collectors on a schedule it owns and pushes a frame when something
changes. The live shell no longer polls: the two-second timer is in the old sidebar
(`shell/Sidebar.qml`), which `shell/shell.qml` does not load.

The argument underneath the change is the one that was already true and is still true:

1. **The parsing is the part that is hard, and it is tested.** `wpctl` reports a
   volume of `1.53` and does not clamp it. `nmcli -g` escapes a literal colon as
   `\:` and a bluetooth device's MAC comes back as `CC\:42\:10\:65\:BE\:FB`.
   `bluetoothctl` exits 0 while printing an error, and blocks forever when
   `bluetoothd` is down. Those are tests over real captured output, and they
   would have to be written again against an unverified API.
2. **The parsing stays in Go.** A daemon is not a licence to move the parsers into QML. The
   collectors and their fixtures are the daemon's, and the shell reads a frame rather than
   interpreting a tool's output.
3. **`zenith state` survives as a CLI.** It is a command, it is useful at a terminal, in a
   script, and to anything that is not Quickshell. It still probes its five tools through
   `desktop/internal/desktopstate`, which the daemon does not use: the daemon is a port of
   Ryoku's and has collectors of its own, so the two are separate implementations.

What should not happen is the other migration — hand-written QML parsing to avoid running a
tested binary, in either direction. The daemon is where the tested code lives; the shell is
where the document is drawn.

## Open questions

| Question | Why it matters |
|---|---|
| ~~**What draws the login screen**~~ | **Settled** — SDDM's Wayland greeter mode, with a Weston kiosk compositor hosted by Zenith. Written up in [what draws the login screen](#what-draws-the-login-screen). |
| ~~**The Secret portal**~~ | **Settled** — `xdg-desktop-portal-lxqt`, named for Secret only. Password storage works in Flatpaks. The KDE backend also implements it but drags in a second notification daemon. See [the portal seam](#the-portal-seam). |
| ~~**The Camera portal**~~ | **Settled** — and not the way it looked. No backend implements `org.freedesktop.portal.Camera`, which reads as "nobody wrote one". But Camera is not a backend interface: `desktop-portal/camera.c` is part of the `xdg-desktop-portal` binary, which implements it over PipeWire and asks an **Access** backend for consent first. Access was falling to `[default] use=hyprland`, and Hyprland does not implement Access, so the dispatcher's own Camera portal could never obtain consent. Naming `Access` → `gtk` is the whole fix. See [the portal seam](#the-portal-seam). |
| ~~**The icon font**~~ | **Settled** — `fonts-font-awesome` (4.7, 1.4 MB, no dependencies) for the status area and settings surfaces, and `fonts-powerline` for bar separators. Both from the archive, so the boundary holds and Zenith does not become the distributor of a font blob. No Nerd Font is involved. See the `fonts-and-icons` group in [`system/packages.yaml`](../system/packages.yaml). |
| Repository hosting | Zenith's APT repository and ISO hosting need a home. GitHub Pages serves static files; a package repo wants object storage. |
| ~~**Plugin mechanism**~~ | **Settled** — a plugin is a folder with a `manifest.json`, a `service/Main.qml` for the logic and a `content/Widget.qml` for the one view, drawn through the `qs.PluginKit` kit. `zenith plugin new`, `audit`, `install`, `enable`, `disable`, `remove` and `list` scaffold, check and place it, the plugin runtime in `shell/plugins/` loads it, and it lives in `~/.local/share/zenith/plugins/`. See [`plugins.md`](plugins.md). A package index for plugins is not built. |
| Hardware report scope | What `zenith hardware report` captures, and how aggressively it strips PII before emitting. |
| Ryoku's hardware tuning | Zenith ports none of the per-machine tuning (udev, NetworkManager, Bluetooth, GPU, power) that the desktop it was ported from ships under `system/hardware/`; the hardware profiles own that job. Whether some of it should return as profile content is the user's decision. See [the hard list](#the-hard-list). |
| ISO firmware policy | Whether one ISO carries firmware or firmware is fetched at install time. See [firmware and free software](#firmware-and-free-software). |
| live-build and Limine | live-build cannot emit a Limine ISO. A custom hook, or `mmdebstrap` plus a hand-built boot chain. |
| Test matrix | Which Hyprland, Quickshell and kernel versions CI must pass before a release. |