# Adopting Zenith on an existing machine

`zenith adopt` switches a stock Ubuntu machine's desktop over to the Zenith shell, in place, without
reinstalling and without touching the kernel, the bootloader or the user's data. This document
is the design for that feature: what it is, what it refuses to do, how it stays reversible at
every step, and how a user gets back to a working GNOME desktop if anything goes wrong.

Read [`architecture.md`](architecture.md) first. Every decision referenced here is settled
there; this document does not reopen any of it.

---

## What adoption is

A user runs one command and, at the next login, can choose **Zenith** from the session list at
their existing display manager. Nothing has been uninstalled. Nothing has been repartitioned.
If they do not like it, they pick the other session in the login screen and their machine is
exactly as it was.

What they get is the Zenith shell:

- Hyprland as the compositor
- Zenith's Quickshell-drawn shell — bar, dock, launcher, popouts, widgets
- `zenith`, `zenith-shell` and `zenith-hub` as the control tools and the only writers of `org.zenith.*`
- Zenith's portal stack — `xdg-desktop-portal-hyprland` and `xdg-desktop-portal-gtk`
- Ghostty, zsh, Neovim, and the rest of Zenith's runtime stack

The distribution is still Ubuntu's. Zenith's packages are installed on top of it, exactly as they
are on a Zenith OS install, and they obey the same rule they obey everywhere else:

> **Zenith's packages write only to `/usr/share/zenith/` and `/etc/zenith/`. Never to another package's
> files.**

Adoption does not break that rule. It adds **two** things that are not already sanctioned by
it — a session registration and a systemd unit registration — and both are listed in
[the registrations](#registrations-outside-zeniths-two-directories). They have to be discoverable
by a binary that is not Zenith's, and there is nowhere else to put them.

## What adoption is not

This is the distinction the whole feature rests on, so it is stated plainly and not hedged:
**adoption gives you the Zenith shell, not a Zenith OS installation.**

Adoption is a funnel. It exists so a person who is curious about Zenith can run it on the laptop
already on their desk, at the cost of one command and one reboot, and can undo it in the time
it takes to pick a different session in a login screen. Every design decision below follows
from that: nothing is removed unless asked, the first step is the smallest reversible thing
that can possibly work, and the machine is never in a state that cannot be recovered.

| Adoption does not give you | Why | Where you get it |
|---|---|---|
| A Zenith OS installation | Adoption is a session, not an image. | Install Zenith OS. |
| Zenith's pinned kernel fleet | The kernel is the user's and stays the user's. Adoption never writes `/boot`, an initramfs or a kernel cmdline. | Install Zenith OS. |
| Zenith's Limine boot chain | Boot-chain surgery is how a distribution gets bricked. Adoption has no such operation at all. | Install Zenith OS. |
| The curated package set — no GNOME, no snapd, no software centre | That is a property of the image, decided at build time. | Install Zenith OS. |
| Install-time hardware profile selection and tier defaults | Profiles are matched by the installer at install time. | Install Zenith OS, or run `zenith hardware match` afterwards to see where the machine lands. |
| A support tier | A tier is a claim about a machine Zenith has been run on. An adopted machine is *tier unknown* until someone submits a profile. | Run `zenith hardware report` and open a pull request. |
| snapd removal | snapd may be holding the user's applications. Adoption leaves it running. | Install Zenith OS. |
| A display manager change | GDM works perfectly well as a chooser. Touching the display manager is opt-in and comes last. | `zenith adopt --display-manager sddm`. |

An adopted machine that needs a kernel cmdline change, a firmware blob or a bootloader edit is
outside the funnel. `--check` says so and points at a Zenith OS install, because the whole value
proposition of adoption is that the user's system was not modified.

---

## The reversibility contract

Reversibility is the central requirement, so it is stated as a contract with a machine-checkable
version, not as an intention.

### The first step is the smallest thing that can work

The first step of adoption is adding **one file**: a session registration at
`/usr/share/wayland-sessions/zenith.desktop`. That is it. It appears in the greeter's session
list at the next login screen. The user picks it, or picks the other session, and both
outcomes are supported.

This means **the first step is undone by choosing the other session**. No command, no root, no
package removal, no reboot. If adoption failed at step one and did nothing else, the machine is
already back where it started.

Everything after step one is opt-in and separately reversible, and nothing after step one has
to happen for the user to be able to get back.

### Registrations outside Zenith's two directories

The base-layer rule is the reason rollback works, so adoption writes its exceptions down
instead of quietly accumulating them. Of the three registrations below, GSettings schemas are
already on the sanctioned list in [the base-layer boundary](architecture.md#the-base-layer-boundary).
The other two are new.

| Path | Already sanctioned? | What it is | Why it cannot live under `/usr/share/zenith/` |
|---|---|---|---|
| `/usr/share/wayland-sessions/zenith.desktop` | **No — new** | Session registration | GDM and SDDM scan the fixed `wayland-sessions` directory and nowhere else. |
| `/usr/lib/systemd/user/zenith-shell.service`, `zenith-portal.service` | **No — new** | User unit registration | `systemd --user` loads units from the fixed unit directory. |
| `/usr/share/glib-2.0/schemas/org.zenith.*.gschema.xml` | Yes — "a GSettings schema" | GSettings schema | GLib compiles schemas from the fixed schema directory. |

These are registrations — pointers a third-party binary must find — not Zenith-owned content. The
content stays at `/usr/share/zenith/session/`, `/usr/share/zenith/shell/` and
`/usr/share/zenith/schemas/`; the registration is materialized from it, and `zenith doctor` reports
drift between the two the same way it reports drift everywhere else.

Add "a session or unit registration file" to that list in
[`architecture.md`](architecture.md#the-base-layer-boundary) when this feature lands, so the
rule stays accurate. Do not let it become a habit.

### Paths adoption promises never to write

Every one of these is either load-bearing for booting, or load-bearing for the way back. If a
future change to `zenith adopt` needs one of them, that change is wrong.

```
/boot/**                         /usr/share/xsessions/*
/etc/fstab                       /usr/share/wayland-sessions/*.desktop   (except zenith.desktop)
/etc/default/grub                /usr/share/gnome-session/**
/etc/kernel/**                   /usr/share/gnome-shell/**
/etc/initramfs-tools/**          /etc/gdm3/**
/etc/linux-image/**              /etc/xdg/**
/var/lib/dpkg/info/*.postinst     ~/.local/share/gnome-shell/extensions/**   (read and copy only)
```

APT's own writes to `/var/lib/dpkg/` are not adoption's writes, but they are the reason every
package operation is simulated before it runs. See [prune](#prune-and-what-is-removed).

### The machine cannot become unbootable

A failed adoption must never leave the machine unbootable. This is guaranteed structurally
rather than by care:

1. Adoption has **no operation that touches boot**. No kernel package, no initramfs, no
   bootloader, no partition table, no `fstab`. The kernel on the machine before adoption is the
   kernel after it, byte for byte.
2. The **display manager is not touched** by default. Whatever greeter the user had is still
   the greeter.
3. Every file adoption writes is **copied into a backup directory first**, and a complete
   `undo.sh` is generated before the first write happens.
4. Every package operation is **simulated first** with `apt-get -s` and the result is checked
   against the approved set.
5. If the Zenith session fails to start, `zenith-session` **never reaches `exec Hyprland`**, and the
   display manager returns the user to the login screen. A broken session that exits before the
   compositor is a safe failure mode by construction.
6. `Ctrl+Alt+F3` reaches a TTY that depends on none of the above.

A CI test asserts that the set of paths `zenith adopt` writes is a subset of the table above plus
the three registrations. That test is the enforcement, and it fails the build if the promise is
broken.

---

## The display manager

Two display managers are in scope, and they are not interchangeable.

| | GDM | SDDM |
|---|---|---|
| Greeter | Runs its own `gnome-shell` in `gdm` session mode | Independent of any desktop |
| Lists foreign sessions | Yes — every `.desktop` in `/usr/share/wayland-sessions/` | Yes |
| Depends on `gnome-shell` | **Yes**, the package `gdm3` depends on `gnome-shell` and `gnome-session-bin` for its greeter | No |
| In a Zenith OS image | No | Yes — it is Zenith's login manager |
| Blocks pruning GNOME | Yes | No |

**Adoption does not change the display manager.** GDM is coupled to GNOME — not philosophically,
but as an APT dependency — and its greeter works perfectly well as a chooser for a session it
knows nothing about. Two consequences follow, and both matter:

- Adding `zenith.desktop` puts a second file in `/usr/share/wayland-sessions/`, and a second file
  is what makes GDM show its session wheel at all. With only `gnome.desktop` present there is
  no session selector to click at all.
- GDM picks up the new file without a restart, so the session is visible at the very next login
  screen.

Switching to SDDM is therefore a separate, later, opt-in step with its own command, its own
state and its own undo:

```bash
sudo zenith adopt --display-manager sddm
```

It installs `sddm` and `sddm-themes`, runs `update-alternatives --install
/usr/bin/display-manager sddm`, enables `sddm.service`, and says that a reboot is required.
Getting a display manager wrong is one of the few ways to lose a graphical login entirely, so
the command **refuses to run until `zenith adopt --verify` has passed** and until the user has
confirmed they can log in on a TTY. `--undo` restores the previous display manager from
`/etc/zenith/adopt/state.json`; because the swap is a `systemctl enable`, getting back also needs
a reboot. It is the only step in the whole feature that does.

---

## Preflight — `zenith adopt --check`

Nothing is changed by the preflight. It needs no root, and it is safe to run as often as the
user likes.

| Check | Reads | Fail condition |
|---|---|---|
| Base release | `/etc/os-release` | Not `ID=ubuntu` with `VERSION_ID="26.04"` |
| Desktop session | `$XDG_CURRENT_DESKTOP` | Not GNOME — a non-GNOME session is already a Zenith session or something else entirely |
| Display manager | `/etc/X11/default-display-manager` | Neither `gdm3` nor `sddm` |
| GNOME version | `dpkg-query -W gnome-shell` | Not installed; there is nothing to switch away from |
| GPU and driver | `lspci -nnk`, `/sys/class/drm/card*/device/driver` | Report only. Warns loudly on NVIDIA discrete, hybrid mux and PRIME. |
| Composite support | `ls /dev/dri/render*`, `glxinfo -B` | No render node |
| Portal backends available | `apt-cache policy xdg-desktop-portal-hyprland xdg-desktop-portal-gtk` | Either is not installable — this is a hard blocker, Zenith's shell has no portal backend |
| Disk space | `statvfs /usr` | Less than 2 GiB free |
| APT health | `apt-get check`, `/var/lib/dpkg/lock-*`, `dpkg --audit` | A held lock or a half-configured package — a hard blocker |
| Third-party extensions | `~/.local/share/gnome-shell/extensions/`, `gsettings get org.gnome.shell enabled-extensions` | Report only |
| Boot chain | `efi/`, `/boot/efi`, Secure Boot state | Report only. Warns if the machine would need a cmdline change, which puts it outside the funnel. |

Exit codes, so the same command is usable from a script:

| Code | Meaning |
|---|---|
| `0` | Ready. No blockers. |
| `1` | Blocked. At least one hard blocker; `zenith adopt` will refuse. |
| `2` | Ready with warnings. Proceed, but read them. |

`--json` emits the same report machine-readably, which is how `zenith adopt` re-runs it as its own
step zero without reimplementing any of it.

Sample output on a machine that is ready:

```
$ zenith adopt --check

Base            Ubuntu 26.04 LTS                          ok
Session         GNOME on Wayland, gnome-shell 48.2       ok
Display manager gdm3                                       ok  (left untouched)
GPU             AMD Radeon 890M, amdgpu, render node ok   ok
Portal apt      xdg-desktop-portal-hyprland 1.2.1         ok
                xdg-desktop-portal-gtk 4.0.1             ok
Disk            41 GiB free on /usr                      ok
APT             no locks, dpkg --audit clean             ok
Extensions      4 third-party, all will be archived      ok
Boot chain      UEFI, Secure Boot on, MOK enrolled       ok  (unchanged)

  2 warnings, 0 blockers.  `zenith adopt` will proceed.
```

---

## What `zenith adopt` does

`zenith adopt` with no flags runs the preflight, prints the plan, asks once for elevation, and
performs steps 0 through 11. Nothing is removed. Nothing outside the contract table above is
touched.

| Step | Action | Writes | Root |
|---|---|---|---|
| 0 | Re-run the preflight. Abort on any blocker. | — | no |
| 1 | Install Zenith's APT keyring and sources. | `/etc/apt/keyrings/zenith.gpg`, `/etc/apt/sources.list.d/zenith.sources` | yes |
| 2 | Install `zenith-shell` and its dependencies. | packages | yes |
| 3 | Install and compile the GSettings schemas. | `/usr/share/glib-2.0/schemas/org.zenith.*.gschema.xml` | yes |
| 4 | Install the portal preference for this user. | `~/.config/xdg-desktop-portal/portals.conf` | no |
| 5 | **Materialize the session registration.** | `/usr/share/wayland-sessions/zenith.desktop` | yes |
| 6 | Archive GNOME Shell extension state. | `~/.local/state/zenith/adopt/` | no |
| 7 | Import selected GNOME settings into `org.zenith.*`. | dconf | no |
| 8 | Enable `power-profiles-daemon` and the non-GNOME desktop services. | systemd state | yes |
| 9 | Seed `~/.config/zenith/`, `user_edits/`, and `~/.local/share/zenith/plugins/`. | user | no |
| 10 | Apply the four Ghostty defaults. | dconf, `/usr/bin/x-terminal-emulator`, `~/.zshrc` | mixed |
| 11 | Write the state file and the generated undo script. | `/etc/zenith/adopt/` | yes |
| — | Print the next action: log out, pick Zenith. | — | — |

Elevation happens once. `zenith adopt` runs unprivileged, prints the plan, and then re-executes
the system-writing steps under `pkexec` so the user types one password instead of four.

### The repository stanza

```bash
sudo install -d -m 0755 /etc/apt/keyrings
curl -fsSL https://packages.zenith.sh/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/zenith.gpg
sudo tee /etc/apt/sources.list.d/zenith.sources >/dev/null <<'EOF'
Types: deb
URIs: https://packages.zenith.sh
Suites: stable
Components: main
Architectures: amd64 arm64
Signed-By: /etc/apt/keyrings/zenith.gpg
EOF
sudo apt-get update
```

The host `packages.zenith.sh` is illustrative. Repository hosting is an open question in
[`architecture.md`](architecture.md#open-questions); the *shape* of the stanza is settled
(`signed-by` in a deb822 `sources` file), the hostname is not. `zenith adopt` reads the real value
from the shipped `zenith.repo` config, and `--check` reports the source it will use.

### State file

`/etc/zenith/adopt/state.json` is the record of everything adoption did. `--undo`, `zenith doctor` and
every failure path in this document read it.

```json
{
  "version": 1,
  "adopted_at": "2026-10-17T21:51:03Z",
  "user": { "name": "mohit", "home": "/home/mohit", "uid": 1000 },
  "base": { "id": "ubuntu", "version_id": "26.04" },
  "machine": {
    "display_manager_before": "gdm3",
    "gnome_shell_version": "48.2",
    "gpu": { "vendor": "amd", "driver": "amdgpu", "discrete": false }
  },
  "registrations": [
    { "path": "/usr/share/wayland-sessions/zenith.desktop",
      "materialized_from": "/usr/share/zenith/session/zenith.desktop",
      "byte_identical_to_base": true },
    { "path": "/usr/lib/systemd/user/zenith-shell.service",
      "materialized_from": "/usr/share/zenith/shell/units/zenith-shell.service",
      "byte_identical_to_base": true },
    { "path": "/usr/lib/systemd/user/zenith-portal.service",
      "materialized_from": "/usr/share/zenith/shell/units/zenith-portal.service",
      "byte_identical_to_base": true }
  ],
  "user_writes": [
    { "path": "~/.config/xdg-desktop-portal/portals.conf",
      "archived_to": "~/.local/state/zenith/adopt/portals.conf.gnome" },
    { "path": "~/.zshrc", "action": "appended_ghostty_integration_block" },
    { "dconf": "org.gnome.desktop.default-applications.terminal exec",
      "value_before": "gnome-terminal" },
    { "alternatives": "x-terminal-emulator",
      "value_before": "/usr/bin/gnome-terminal" }
  ],
  "imports": [
    { "from": "org.gnome.desktop.interface color-scheme",
      "to": "org.zenith.desktop color-scheme", "value": "'prefer-dark'" },
    { "from": "org.gnome.desktop.background picture-uri",
      "to": "org.zenith.shell wallpaper-path",
      "value": "'file:///usr/share/backgrounds/warty-final-ubuntu.png'" }
  ],
  "packages": { "installed": ["zenith-shell", "zenith-hub", "quickshell", "hyprland"], "removed": [] },
  "prune": { "performed": false, "removed": [], "purged": [], "skipped_because_gdm_active": [] },
  "backup_dir": "/etc/zenith/adopt/backup/20261017T215103Z"
}
```

### The generated undo script

Written before the first write, so it exists even if adoption dies half way through. It is a
plain file, not a binary, because the moment it is needed the user may not have a working
desktop to run a Go binary from.

The one subtlety is that it runs under `sudo`, which changes `$HOME` to root's and puts
`gsettings` in root's dconf database rather than the user's. So the script names the adopting
user explicitly and splits itself: **it does the root work itself, and hands the dconf work
back to the user**, because there is no honest way for a root process to write another user's
dconf.

```sh
#!/bin/sh
# Generated by zenith adopt at 2026-10-17T21:51:03Z. Safe to run more than once.
set -eu

BACKUP=/etc/zenith/adopt/backup/20261017T215103Z

# sudo changes $HOME to /root and would put every user-scoped path in the
# wrong place, so the adopting user is named explicitly.
ADOPT_USER=mohit
ADOPT_HOME=/home/mohit
ARCHIVE="$ADOPT_HOME/.local/state/zenith/adopt"

# ── root work ──────────────────────────────────────────────────────────────

# 1. Reinstall anything prune removed, before touching any config it owned.
if [ -s "$BACKUP/packages.removed" ]; then
    xargs -a "$BACKUP/packages.removed" apt-get install -y
fi

# 2. Finish an interrupted package transaction. Adoption never writes dpkg's
#    database directly, so there is nothing to restore — only to complete.
dpkg --configure -a
apt-get -f install -y

# 3. Remove every registration adoption materialized. The packaged content
#    under /usr/share/zenith/ stays; nothing there needs removing.
rm -f /usr/share/wayland-sessions/zenith.desktop
rm -f /usr/lib/systemd/user/zenith-shell.service
rm -f /usr/lib/systemd/user/zenith-portal.service
rm -f /usr/share/glib-2.0/schemas/org.zenith.desktop.gschema.xml
rm -f /usr/share/glib-2.0/schemas/org.zenith.shell.gschema.xml
glib-compile-schemas /usr/share/glib-2.0/schemas
systemctl --user daemon-reload 2>/dev/null || true

# 4. Put the default terminal back.
if [ -f "$BACKUP/x-terminal-emulator.path" ]; then
    update-alternatives --set x-terminal-emulator \
        "$(cat "$BACKUP/x-terminal-emulator.path")"
fi

# ── user work ──────────────────────────────────────────────────────────────

# 5. The pre-adoption portal preference, so the GNOME session's own portal
#    backend is selected again. A plain file copy; root-owned on purpose is
#    wrong, so the ownership is fixed afterwards.
if [ -f "$ARCHIVE/portals.conf.gnome" ]; then
    mkdir -p "$ADOPT_HOME/.config/xdg-desktop-portal"
    cp -a "$ARCHIVE/portals.conf.gnome" \
          "$ADOPT_HOME/.config/xdg-desktop-portal/portals.conf"
    chown "$ADOPT_USER" "$ADOPT_HOME/.config/xdg-desktop-portal/portals.conf"
fi

# 6. GNOME Shell extensions. The originals were never moved, so this only
#    re-copies if the user deleted them by hand.
if [ -d "$ARCHIVE/extensions" ]; then
    install -d -m 0755 -o "$ADOPT_USER" "$ADOPT_HOME/.local/share/gnome-shell/extensions"
    cp -a "$ARCHIVE/extensions/." "$ADOPT_HOME/.local/share/gnome-shell/extensions/"
fi

echo
echo "Root work done. Now run these AS $ADOPT_USER, from a login session:"
echo
echo "  gsettings set org.gnome.shell enabled-extensions \"\$(cat $ARCHIVE/enabled-extensions)\""
echo "  gsettings set org.gnome.desktop.default-applications.terminal exec \\\"$(cat "$BACKUP/default-terminal.gvariant")\""
echo
echo "The first writes dconf directly rather than calling 'gnome-extensions enable',"
echo "because GNOME Shell scans the extension directory once at session start and"
echo "never re-scans — enable against a freshly linked directory writes nothing."
echo
echo "Then log out and log back in to GNOME."
echo "The archived Zenith session and shell remain under /usr/share/zenith/."
```

Two properties are worth calling out, because they are the difference between a rollback that
works and one that does not. **Reinstall before restore**: if prune removed `gnome-terminal`
and adoption's undo script restored a `default-applications.terminal exec` pointing at it, the
value would be correct and the command would still fail, so the package reinstall has to be
first. **No root dconf**: `gsettings` run as root writes root's database and silently succeeds,
which is worse than failing — the value would look restored and would not be. So the script
prints the exact user-scoped commands instead of pretending.

### The Ghostty defaults

[`architecture.md`](architecture.md#terminal-ghostty) is explicit that Ghostty is only the
default when all four places are set, or the desktop is inconsistent. Adoption applies all four
rather than leaving a machine that is half-converted.

```bash
gsettings set org.gnome.desktop.default-applications.terminal exec ghostty
update-alternatives --install /usr/bin/x-terminal-emulator x-terminal-emulator /usr/bin/ghostty 30
# ~/.zshrc — appended as a delimited block, never edited in place
#   if [ -f /usr/share/ghostty/shell-integration/zsh/ghostty-integration ]; then
#       . /usr/share/ghostty/shell-integration/zsh/ghostty-integration
#   fi
```

Ghostty is not rebuilt by Zenith — it is the base layer's `ghostty` package, installed as-is. See
[`architecture.md`](architecture.md#terminal-ghostty) for why. That matters here in one concrete
way: the shell integration this block sources is **already on the machine** as part of that
package, so adoption configures the terminal and never installs, builds or replaces it.

The integration lives inside Ubuntu's single `ghostty` package at
`/usr/share/ghostty/shell-integration/zsh/ghostty-integration`, not in the split
`ghostty-shell-integration` package Arch carries — verified against the 26.04 Contents index. So
there is nothing to install for declaration four, and the `[ -f ... ]` guard above is about a
file being absent from an older package, not about a package being unpacked first.

The `update-alternatives` call needs no `--slave`: Ghostty's package registers no alternatives
of its own, so Zenith registers the only one and nothing shadows it later.

The `org.gnome.desktop.default-applications.terminal` write takes effect at the next GNOME
login too, so `zenith` records the previous value and `--undo` restores it. The
`x-terminal-emulator` alternative is restored the same way, from the path recorded before
`update-alternatives --install` ran.

### Importing, not sharing

Step 7 reads GNOME's GSettings keys and writes Zenith's. It **never writes back to
`org.gnome.*`**, it never overwrites an `org.zenith.*` value the user has already set, and it logs
every write. GNOME's values are untouched and remain valid the moment the user logs back into
GNOME.

| From | To | Note |
|---|---|---|
| `org.gnome.desktop.interface color-scheme` | `org.zenith.desktop color-scheme` | Desktop-level, so `org.zenith.desktop`. |
| `org.gnome.desktop.interface gtk-theme` | `org.zenith.desktop gtk-theme` | |
| `org.gnome.desktop.interface font-name` | `org.zenith.desktop font-name` | |
| `org.gnome.desktop.interface text-scaling-factor` | `org.zenith.desktop text-scaling-factor` | Fractional scaling has no GNOME equivalent any more, so it is the one size knob that carries over cleanly. |
| `org.gnome.desktop.background picture-uri` | `org.zenith.shell wallpaper-path` | |
| `org.gnome.desktop.background picture-uri-dark` | `org.zenith.shell wallpaper-path-dark` | |
| `org.gnome.shell favorite-apps` | `org.zenith.shell favorites` | |
| `org.gnome.desktop.wm.preferences button-layout` | `org.zenith.shell dock-position` | |
| `org.gnome.desktop.screensaver lock-enabled` | `org.zenith.desktop lock-enabled` | |
| `org.gnome.desktop.session idle-delay` | `org.zenith.desktop idle-delay` | |
| `org.gnome.desktop.privacy lock-delay` | `org.zenith.desktop lock-delay` | |

`org.gnome.desktop.peripherals.*` is deliberately **not** imported. GTK and Qt applications read
those keys directly — key repeat, cursor blink, tap-to-click — so changing them would change
apps in both sessions. `zenith-shell` reads them instead, when it generates
`/etc/zenith/hyprland/bind.conf`.

---

## Prune, and what is removed

Adoption removes nothing. `--prune` is a separate command, its dry run is the default, and it
is the only part of the feature that can take a package away.

```bash
zenith adopt --prune              # lists exactly what would be removed, changes nothing
zenith adopt --prune --apply      # removes it
zenith adopt --prune --purge      # also removes conffiles. opt-in, and rarely wanted
```

### Prune is gated behind the display manager

`gdm3` depends on `gnome-shell` and `gnome-session-bin` for its greeter. **You cannot remove
GNOME Shell while GDM is installed.** This is not a policy decision, it is an APT dependency,
and it is the reason the display manager step has to exist at all:

```
$ zenith adopt --prune
  BLOCKED: gdm3 is the active display manager. apt-cache depends gdm3 reports:

      gdm3
        Depends: gnome-shell | gnome-shell-x11
        Depends: gnome-session-bin

  Both are required for GDM's own greeter session. Nothing else in the
  removal set is blocked.

  Either stop here — the Zenith session works fine under GDM — or run:
      sudo zenith adopt --display-manager sddm
      sudo zenith adopt --prune --apply

  Nothing has been removed.
```

The gate is not hard-coded. It re-derives the blocking set from
`apt-cache depends $(basename "$(readlink -f /etc/X11/default-display-manager)")"` at run time,
so a future `gdm3` that drops a dependency unblocks the matching prune entry automatically.

An adopted machine with GDM still installed is a perfectly good Zenith machine. Pruning is for the
user who wants the disk space back, and it is available only once they have already proved the
Zenith session works.

### The removal set

| Package | Why removal is safe |
|---|---|
| `xdg-desktop-portal-gnome` | Zenith's portal stack does not use it. [`architecture.md`](architecture.md#the-portal-seam) records that this backend reads `GDK_BACKEND` and silently drops interfaces when it decides it is not on a real GNOME session — the whole reason Zenith ships no GNOME. |
| `gnome-shell` | Replaced by Hyprland and the Zenith shell. `--undo` reinstalls it from the archive's `packages.removed` list. |
| `gnome-session`, `gnome-session-bin` | Only starts the GNOME session, which is no longer the session. |
| `gnome-settings-daemon`, `gnome-settings-daemon-common` | The colour, power, input and notification roles Zenith needs are carried by `power-profiles-daemon`, `colord`, `switcheroo-control`, and the Zenith shell. |
| `gnome-control-center` | Settings live in `zenith-hub`, which is the sole writer of `org.zenith.desktop`. |
| `gnome-shell-extension-prefs` | The extension manager for a shell that no longer exists. |
| `gnome-shell-extension-ubuntu-dock` | The dock is `org.zenith.shell`. |
| `gnome-shell-extension-desktop-icons-ng` | Icons on the desktop are the shell's job. |
| `gnome-shell-extension-appindicator` | The Zenith shell hosts `StatusNotifierItem` itself. Removing this is the single most likely thing to surprise a user, so it is listed last and warned about. |
| `gnome-software`, `gnome-software-common` | Nothing on a GNOME session. **Removed only with `--prune --apply` and only if the user asks; a software centre is not required to run Zenith and some people use it.** |
| `yaru`, `yaru-icon-theme`, `yaru-cursor-theme`, `yaru-theme-gtk` | Purely cosmetic, and Zenith is monochrome. Optional; kept by default. |
| `ubuntu-desktop` (metapackage) | Pulls GNOME back in on the next install. Replaced with `zenith` + `ubuntu-desktop-minimal`. |

A note on metapackages, because this is where prune usually goes wrong. `apt-get remove
gnome-shell` alone leaves `ubuntu-desktop` declaring a dependency that is gone, so the next
`apt-get install` or `apt-get autoremove` quietly restores all of GNOME. The fix is to
`dpkg --remove ubuntu-desktop` — a metapackage holds no files, so removing it is cheap and
reversible by reinstalling — and to leave every unrelated package alone. `zenith adopt --prune`
emits exactly that sequence and the simulation gate checks it.

### What prune never touches

| Kept | Why |
|---|---|
| `gnome-keyring` | The name says GNOME and it is not GNOME. It is the backing store for the Secret portal on every Zenith session. See [`architecture.md`](architecture.md#the-portal-seam). |
| `gdm3` | Only ever removed by an explicit `sudo apt-get purge gdm3`, never by prune. It is the way back. |
| `snapd`, flatpak, any `deb` the user installed | These hold the user's applications. Adoption is not the ISO. |
| `xwayland` | Required by GTK4, Qt5 and most of the user's applications. |
| Any `*-dev`, `lib*` or `-common` library package | Applications link them. `libgnome-desktop-4-1` in particular ships GTK modules apps dlopen at runtime. |
| The kernel, initramfs, bootloader, `mesa` | Off the table entirely. |

### The simulation gate

`--apply` does not trust its own list. It runs `apt-get -s remove` on the full argument list,
parses what APT would actually remove, and refuses if the set contains a single package the tool
did not classify. If the user's own manually-installed package depends on something in the
removal set, APT's simulation says so and adoption stops.

```
$ sudo zenith adopt --prune --apply
  simulate: apt-get -s remove — 31 removals, 4 kept as dependencies
  expected: 31 removals, 0 unclassified
  removing 31 packages, keeping 78 libraries and the 4 blocked dependencies

  not purging. conffiles are kept so --undo restores a working GNOME exactly.
  re-run with --purge if you want the files gone too. That is not reversible by --undo.
```

`--purge` deletes conffiles. On GNOME packages those include the `/etc/skel` templates that
every **future** new account on the machine inherits, and `/etc/gdm3/daemon.conf`. That is why
purge is a separate flag, never the default, and explicitly excluded from the reversibility
promise.

---

## State preservation

### What survives untouched

| Thing | Status |
|---|---|
| `~/.config/**` | Untouched. Zenith writes only `~/.config/zenith/` and the one portal preference file, and archives the latter. |
| `~/Documents`, `~/Pictures`, `~/Music`, `~/Videos`, everything in `~` | Untouched. Adoption reads `~` and never writes a user's data. |
| Applications installed by `apt` | Installed and still installed. Adoption adds packages; it does not remove them. |
| Applications installed by `flatpak` | Untouched. |
| Applications installed by `snap` | Untouched, and snapd is left running. |
| `~/.local/share/gnome-shell/extensions/` | **Copied, never moved.** The originals stay where they are. |
| `org.gnome.desktop.*` GSettings | Read at most. Never written, except the terminal default, which is recorded and restored. |
| `/usr/share/backgrounds/` | Untouched, and one of them is imported as the Zenith wallpaper. |

### What becomes inert

None of these is deleted; they simply have no reader in a Zenith session.

| Schema and key | Why it is inert |
|---|---|
| `org.gnome.shell enabled-extensions` | Nothing reads it. It is archived as the record of what was on. |
| `org.gnome.shell disabled-extensions` | Same. |
| `org.gnome.shell enabled-extension-search-providers` | Same. |
| `org.gnome.shell development-tools` | Same. On an Ubuntu build, `org.gnome.Shell.Eval` is blocked regardless of this key. |
| `org.gnome.shell wm.preferences` | Owned by the gnome-shell process, which does not run. |
| `org.gnome.desktop.screensaver lock-enabled`, `lock-delay` | Shadowed by `org.zenith.desktop`, which is imported from them. |
| `org.gnome.desktop.session idle-delay` | Same. |
| `org.gnome.desktop.a11y.*` | Report only. If a magnifier or screen reader was enabled, `--check` says so and `--undo` leaves it working. |

`org.gnome.desktop.interface`, `org.gnome.desktop.wm.preferences` and
`org.gnome.desktop.peripherals.*` are **not** inert — applications read them directly. They are
left exactly as they are.

### What must be preserved for later: GNOME Shell extensions

A user who has installed four extensions has four pieces of work that cannot trivially be
redone, and losing them is the thing that would make adoption feel expensive. So they are
copied into an archive, not discarded — and specifically *not moved*. The originals stay where
they are, which means even a user who reverts by hand without running `zenith adopt --undo` gets
them back.

The archive is per-user, at `~/.local/state/zenith/adopt/`, which is user scope for user data. The
root-side `/etc/zenith/adopt/backup/` holds only machine-level facts — the package list, the
alternatives path — and never a second copy of anything in `~`.

`zenith adopt` step 6 reads the live session when there is one (`gnome-extensions list`) and falls
back to reading the directories and dconf directly when run from a TTY:

```
~/.local/state/zenith/adopt/
├── extensions/
│   ├── dash-to-dock@microrocket/
│   ├── just-perfection-desktop@self-extender/
│   ├── blur-my-shell@bluesxd/
│   └── caffeine@yaroslavd/
├── enabled-extensions          the gvariant text, ready for gsettings set
├── portals.conf.gnome          the portal preference as it was before adoption
├── gnome-shell-extensions.json
└── adopt-report.json
```

`enabled-extensions` is written out as plain gvariant text at capture time precisely so that
`--undo` never has to parse JSON under `sudo` to perform a dconf write.

`gnome-shell-extensions.json` is the part that matters, and it is built by reading dconf
directly rather than by shelling out:

```json
{
  "captured_at": "2026-10-17T21:51:03Z",
  "gnome_shell_version": "48.2",
  "enabled": [
    "dash-to-dock@microrocket",
    "just-perfection-desktop@self-extender",
    "blur-my-shell@bluesxd",
    "caffeine@yaroslavd"
  ],
  "disabled": [],
  "extensions": [
    { "uuid": "dash-to-dock@microrocket",
      "version": 15, "path": "dash-to-dock@microrocket", "mode": "user",
      "metadata_sha256": "9c1f2b4e7a0d35f81c6b2ae9047d31f5c8e0a76b219d4f83c5e07a1b6d92c480" },
    { "uuid": "just-perfection-desktop@self-extender",
      "version": 44, "path": "just-perfection-desktop@self-extender", "mode": "user",
      "metadata_sha256": "4a70e81c5f26b903da417e6b8c04f1d9275ea63b0c8d412f79b5e3a0c68d2417" },
    { "uuid": "blur-my-shell@bluesxd", "version": 58, "path": "blur-my-shell@bluesxd",
      "mode": "user", "metadata_sha256": "1d3ab5e94c07f2618ae5d0b73c94f18a602d7e5b3c809a146f27d8b5e0c391a64" },
    { "uuid": "caffeine@yaroslavd", "version": 37, "path": "caffeine@yaroslavd",
      "mode": "user", "metadata_sha256": "b60c128fd75a3e04c19b8d6f2a705e3c1d94f806b2ea73c5d190f84b6e023a7c" }
  ]
}
```

Restoring is a dconf write, not a `gnome-extensions enable` call, for the reason recorded in
[`agents/gnome-shell-notes.md`](../agents/gnome-shell-notes.md): GNOME Shell scans
`~/.local/share/gnome-shell/extensions` **once at session start and never re-scans**, so
`gnome-extensions enable` against a freshly linked directory reports that the extension does
not exist and does not write the uuid. `--undo` therefore sets the key directly, using the
plain-text copy captured at adopt time:

```sh
# Run as the adopting user, from a login session. Not under sudo — root would
# write root's dconf and silently appear to succeed.
mkdir -p "$HOME/.local/share/gnome-shell/extensions"
cp -a "$HOME/.local/state/zenith/adopt/extensions/." \
      "$HOME/.local/share/gnome-shell/extensions/"

gsettings set org.gnome.shell enabled-extensions \
  "$(cat "$HOME/.local/state/zenith/adopt/enabled-extensions")"
```

Two consequences worth stating plainly. First, the effect is **at the next GNOME login**, not in
the current session; there is no supported way to load an extension into a running GNOME Shell
on Wayland, and the notes are unambiguous that a logout is the only reload. Second, extensions
restored into a GNOME whose version has moved on may refuse to run — that is a GNOME
compatibility problem, not an adoption one, and the archive keeps the source so the user can
reinstall or update it.

Distro-packaged extensions (`gnome-shell-extension-*`) are not archived, because they are one
`apt-get install` away and `prune` removes them along with GNOME Shell.

---

## The session path

### The session file

The packaged base lives at `/usr/share/zenith/session/zenith.desktop` and is materialized to
`/usr/share/wayland-sessions/zenith.desktop`.

```ini
[Desktop Entry]
Type=Application
Name=Zenith
Comment=Zenith desktop
TryExec=/usr/share/zenith/session/zenith-session
Exec=/usr/share/zenith/session/zenith-session
DesktopNames=Zenith
Keywords=Hyprland;Quickshell;
```

`Type=Application` and `DesktopNames=Zenith`, not `GNOME`. `DesktopNames` is what GDM and SDDM use
to group and label the session; setting it to `GNOME` is what would make a display manager treat
a Zenith session as one of its own. If a menu-driven picker fails to notice the new file,
`sudo update-desktop-database` is the fallback.

`/etc/zenith/session/zenith.desktop` overrides the packaged base, and `~/.config/zenith/session/` — via
the usual overlay — overrides both. That is the overlay model from
[`architecture.md`](architecture.md#overlay-and-rollback), and it is where a machine-specific
`Exec` line goes. `zenith doctor` reports drift here like anywhere else.

### What `zenith-session` does

`/usr/share/zenith/session/zenith-session` is a small shell script, because it runs before anything
else exists and it must be readable when the disk is having a bad day.

```sh
#!/bin/sh
# /usr/share/zenith/session/zenith-session
set -eu

LOG="${XDG_STATE_HOME:-$HOME/.local/state}/zenith/session.log"
mkdir -p "$(dirname "$LOG")"
exec >>"$LOG" 2>&1

echo "--- zenith-session $(date -Is) ${XDG_SESSION_ID:-no-session-id}"

export XDG_SESSION_DESKTOP=zenith
export XDG_CURRENT_DESKTOP=Zenith
export DESKTOP_SESSION=zenith
export XDG_SESSION_TYPE=wayland

# Native Wayland dialogs everywhere, so scaling and theming are the shell's
# problem and not the app's. Set unconditionally rather than inherited: a
# GDK_BACKEND=x11 left in /etc/environment by a GNOME-era tweak would
# otherwise give every GTK4 app an XWayland dialog at the wrong scale, and a
# GDK_BACKEND left set at all is how the GNOME portal backend decides it is
# not on a real GNOME session and silently drops interfaces.
export GDK_BACKEND=wayland
export MOZ_ENABLE_WAYLAND=1
export QT_QPA_PLATFORM=wayland
export QT_QPA_PLATFORMTHEME=gtk3
export QT_WAYLAND_DISABLE_WINDOWDECORATION=1

# Cursor sizing and theming for the XWayland apps that do not read GTK.
export XCURSOR_SIZE=24
export XCURSOR_THEME=Adwaita

# Machine-level additions, then the user's overlay. Written as `if`, not as
# `[ -r f ] && . f` — under `set -e` a false test in an AND-list aborts the
# session before the compositor ever starts, which is a confusing way to fail.
if [ -r /etc/zenith/session/zenith-session.env ]; then
    . /etc/zenith/session/zenith-session.env
fi
if [ -r "${XDG_CONFIG_HOME:-$HOME/.config}/zenith/session/zenith-session.env" ]; then
    . "${XDG_CONFIG_HOME:-$HOME/.config}/zenith/session/zenith-session.env"
fi

# Make dbus-activated user services and anything the compositor spawns see the
# desktop identity, or apps probe for GNOME and get confused.
systemctl --user import-environment XDG_SESSION_DESKTOP XDG_CURRENT_DESKTOP \
    DESKTOP_SESSION XDG_SESSION_TYPE WAYLAND_DISPLAY XDG_RUNTIME_DIR
dbus-update-activation-environment --systemd XDG_SESSION_DESKTOP XDG_CURRENT_DESKTOP \
    DESKTOP_SESSION XDG_SESSION_TYPE WAYLAND_DISPLAY

echo "zenith-session: env ok, starting Hyprland"
exec /usr/bin/Hyprland
```

The `exec` at the end is what makes a failure safe: anything that goes wrong before it — a bad
`/etc/zenith/session/zenith-session.env`, an unreadable schema, a disk-full — exits with the
compositor never started, and the display manager puts the user back on the login screen. Every
step above is also written so that it cannot exit for a trivial reason, which is why the
overlay files are guarded with `if` rather than with `&&`.

Hyprland is exec'd, and the session's own target is started once the compositor is up. That
target is the ordering anchor for everything that needs a live compositor:

```ini
# /usr/lib/systemd/user/zenith-session.target
[Unit]
Description=Zenith session
BindsTo=graphical-session.target
After=graphical-session.target
Wants=zenith-portal.service zenith-shell.service
```

The target is Zenith's own rather than the compositor's. Upstream Hyprland ships one and
Zenith's units used to attach to it with a drop-in; Ubuntu's build of Hyprland ships no
systemd units at all, so the drop-in had a target to attach to on some machines and not on
others, and on the others the shell never started. A unit that requires a target which does
not exist is a unit that cannot start.

The compositor's configuration starts the target, because the compositor is what knows the
display:

```ini
# /usr/share/zenith/hyprland/hyprland.conf
exec-once = /usr/share/zenith/session/zenith-activate
```

`zenith-activate` imports the compositor's environment into the user manager and then starts
the target. The units are started by the user manager, and a unit inherits the manager's
environment rather than the shell's: without the import, the shell's condition on the display
fails and the shell refuses to start.

### Start order

| Order | Unit | Started by | What it is |
|---|---|---|---|
| 0 | `zenith-session` | the display manager | Environment, dbus activation, `exec Hyprland` |
| 1 | `zenith-session.target` | the compositor's configuration, through `zenith-activate` | Compositor-dependent user units |
| 2 | `wireplumber.service` | the session | Audio policy, so the shell's volume widget has something to talk to |
| 3 | `zenith-portal.service` | the session target above | Starts `xdg-desktop-portal` and lets it D-Bus-activate the backends on demand |
| 4 | `zenith-shell.service` | the session target above, ordered after portals | The Quickshell shell |
| 5 | `zenith-shell.service` children | Zenith | Lock screen, quick settings, notification server — inside the shell, not separate units |

```ini
# /usr/lib/systemd/user/zenith-shell.service
[Unit]
Description=Zenith shell (Quickshell)
Documentation=man:zenith-shell(1)
PartOf=graphical-session.target
After=zenith-session.target zenith-portal.service wireplumber.service
Requires=zenith-session.target
ConditionEnvironment=WAYLAND_DISPLAY

[Service]
Type=exec
ExecStart=/usr/bin/quickshell -p /usr/share/zenith/shell
ExecReload=/bin/kill -SIGUSR1 $MAINPID
Restart=on-failure
RestartSec=1
Slice=app.slice

[Install]
WantedBy=zenith-session.target
```

```ini
# /usr/lib/systemd/user/zenith-portal.service
[Unit]
Description=Zenith portal dispatcher
PartOf=graphical-session.target
After=zenith-session.target
ConditionEnvironment=WAYLAND_DISPLAY

[Service]
Type=dbus
BusName=org.freedesktop.portal.Desktop
ExecStart=/usr/libexec/xdg-desktop-portal
Restart=on-failure
RestartSec=1

[Install]
WantedBy=zenith-session.target
```

### What the shell has to provide

Everything GNOME Shell used to provide is now the shell's job or the compositor's job, and
every row here is a row in the verification checklist.

| Capability | GNOME Shell provided it via | Zenith provides it as | Verified by |
|---|---|---|---|
| Notification server | `org.freedesktop.Notifications` on the session bus, implemented inside gnome-shell | The shell hosts the server itself. No backend implements the Notifications portal, so if the shell is not up, **nothing** is notified and there is no fallback. | The [Notify probe](#the-probes) |
| Global shortcuts | `org.gnome.Shell` accelerators, plus GNOME's own keybindings | Compositor bindings for compositor-level keys; the shell owns everything else directly, which is both simpler and faster than a portal round-trip | `hyprctl binds` |
| Screen lock | The GNOME screen shield and lock dialog | `hyprlock` locks and talks to PAM itself, as a process of its own; `zenith-shell lock` is the one command every trigger runs, so no GNOME process is involved | `systemctl --user lock-session` |
| Idle | `gnome-settings-daemon` idle monitor | `hypridle`, run by `zenith-idle.service` with the timeouts of `org.zenith.desktop`'s `idle-*` keys, for the dim, the screen-off and the lock; there is no suspend stage | `systemctl --user status zenith-idle.service` after an idle timeout |
| Power policy | `gnome-settings-daemon` power profiles | `power-profiles-daemon`, which adoption enables at step 8 because GNOME was providing it and would stop on the next prune | `powerprofilesctl get` |
| Wallpaper | The shell's own background | The shell's, sourced from `org.zenith.shell wallpaper-path`, imported from `org.gnome.desktop.background` | `hyprctl` wallpaper, visually |
| Quick settings | The GNOME quick-settings menu | The shell's — Bluetooth, Wi-Fi, power mode, volume, brightness, night light | visually |
| Status icons (tray) | `gnome-shell-extension-appindicator` | The shell hosts `StatusNotifierItem` itself | an app that draws a tray icon |
| Screen capture and share | The GNOME portal backend | `xdg-desktop-portal-hyprland`, plus `grim` and `slurp` bound to keys | the browser call test |
| File chooser | The GNOME portal backend | `xdg-desktop-portal-gtk` | The [FileChooser probe](#the-probes) |
| Colour management | `colord` | Unchanged — `colord` is a base-layer service, not GNOME | `colord` probe |
| Window switching | gnome-shell's own | The shell's most-recently-used switcher on `Alt+Tab`; `Super+Tab` opens the workspace overview | visually |
| Night light | `gnome-settings-daemon` | Hyprland's night light, driven over its IPC socket with the `v0`, `v1`, `gamma` and `enabled` properties | visually |
| Clipboard | gnome-shell's | The shell's | visually |
| Login and logout | GDM | GDM, unchanged | the login screen |

The one row where adoption can take something away is **status icons**: a user who relies on
the tray loses it at prune, not at adoption, because `gnome-shell-extension-appindicator` stays
installed until then. Under GDM with GNOME still present, the extension keeps working. It is
still the row most likely to surprise somebody, which is why it is in the checklist.

`XDG_CURRENT_DESKTOP=Zenith` is a value some libraries branch on. Applications that special-case
`GNOME` may take a different path — usually a benign one — which is why "my existing
applications still run" is a checklist row and not a formality.

---

## Portal migration

### What changes

On a GNOME session, `xdg-desktop-portal` reads its backend preference from
`~/.config/xdg-desktop-portal/portals.conf`, which says `use=gnome;`, and D-Bus-activates
`xdg-desktop-portal-gnome.service`. On a Zenith session it must say `use=hyprland;` and activate
the Hyprland and GTK backends instead.

`zenith adopt` step 4 archives the existing file and writes this one. It is the **user** file
rather than the system file at `/usr/share/xdg-desktop-portal/portals.conf` for three reasons:
it needs no root, it is a single `rm` away, and it does not modify a file a base-layer package
owns.

```ini
# ~/.config/xdg-desktop-portal/portals.conf
# Written by zenith adopt. Previous contents archived to
# ~/.local/state/zenith/adopt/portals.conf.gnome

[default]
use=hyprland;

[org.gnome.impl.portal.ScreenCast]
use=hyprland;

[org.gnome.impl.portal.Screenshot]
use=hyprland;

[org.gnome.impl.portal.FileChooser]
use=gtk;

[org.gnome.impl.portal.OpenURI]
use=gtk;

[org.gnome.impl.portal.Print]
use=gtk;

[org.gnome.impl.portal.Secret]
use=gtk;
```

The interface names carry a `org.gnome.impl.portal.` prefix and mean nothing about whether
GNOME is installed — that namespace predates anyone having an opinion about this.

`GlobalShortcuts` is deliberately absent from that file. The Hyprland backend does not
implement it, no other backend does either, and listing an unimplemented backend in `use=`
produces a dispatcher error rather than a useful fallback. The shell owns global shortcuts
directly; see [`architecture.md`](architecture.md#the-portal-seam).

### Why it must be the user file and not the system file

The single most common way this goes wrong is a stale preference. If anything at
`~/.config/xdg-desktop-portal/portals.conf` still says `use=gnome;` — left over from the
session the user came from, or written by an app that has opinions — the dispatcher never loads
the Hyprland backends, and every failure looks like a Zenith bug: file choosers hang, screen
sharing says "no available source", and the print dialog never appears.

```bash
# Exactly what the dispatcher will load, in a live Zenith session.
$ busctl --user introspect org.freedesktop.portal.Desktop \
      /org/freedesktop/portal/desktop org.freedesktop.portal.DBus \
  | grep -o 'org\.freedesktop\.portal\.[A-Za-z]*' | sort -u
org.freedesktop.portal.Background
org.freedesktop.portal.Camera
org.freedesktop.portal.DynamicLauncher
org.freedesktop.portal.Email
org.freedesktop.portal.FileChooser
org.freedesktop.portal.Inhibit
org.freedesktop.portal.Notification
org.freedesktop.portal.OpenURI
org.freedesktop.portal.Print
org.freedesktop.portal.RemoteDesktop
org.freedesktop.portal.ScreenCast
org.freedesktop.portal.Screenshot
org.freedesktop.portal.Secret
org.freedesktop.portal.Trash
org.freedesktop.portal.Wallpaper

$ pgrep -a xdg-desktop-portal-hyprland
2151 xdg-desktop-portal-hyprland
$ pgrep -a xdg-desktop-portal-gtk
2168 xdg-desktop-portal-gtk
$ pgrep -a xdg-desktop-portal-gnome
$                                     # empty. This is the pass condition.
```

`gnome-keyring` is still running and is supposed to be. It is the backing store behind the
Secret portal and has nothing to do with GNOME Shell — see
[`architecture.md`](architecture.md#the-portal-seam).

### What a user should check by hand

| Portal | How to exercise it | Pass |
|---|---|---|
| FileChooser | `zenity --file-selection --title "portal probe"` | A GTK file chooser opens, and it looks like the Zenith shell's, not a GTK default |
| OpenURI | Open a `file://` link in the browser | The user's file manager opens it |
| Print | `lpstat -r`, then print from any GTK app | `lpstat -r` says the printer is ready; the print dialog opens |
| Secret | Ask an app for a stored credential | The keyring prompt appears and the credential is remembered |
| ScreenCast | Share a screen from the browser, or from the shell's quick settings | A picker appears and the chosen source works |
| Screenshot | The screenshot key, and a screenshot request from an app | A capture is taken by the right backend |
| Notification | Any app that notifies | A banner appears and actions work |

**Known gaps, stated up front.** The `Settings` portal — `org.gnome.impl.portal.Settings`,
which reports the system colour scheme and accent colour — is implemented only by the GNOME
backend. It is not implemented on Zenith, and an application that asks for dark mode through the
portal rather than through GSettings will see the default. libadwaita and GTK4 do not do this,
so the blast radius is small, but it is real and it is in the checklist. Similarly,
`xdg-desktop-portal-hyprland`'s screen capture is a `wlr-screencopy` implementation, which is
the rough edge on machines with a proprietary NVIDIA driver; on those, an app's native
capture path is usually better than the portal one.

---

## Verification

`zenith adopt --verify` runs everything scriptable and prints the rest. It reads; it changes
nothing; it needs no root. It runs inside the Zenith session, and it is the gate in front of
`--display-manager` and in front of `--prune`.

```
$ zenith adopt --verify

  session identity        PASS   XDG_CURRENT_DESKTOP=Zenith, XDG_SESSION_TYPE=wayland
  session type            PASS   loginctl Type=wayland Desktop=zenith
  compositor              PASS   hyprland 0.51.1, 3 monitors
  shell                   PASS   zenith-shell.service active (running), 41s uptime
  audio                   PASS   wpctl: Analog Stereo Output [vol 0.42]
  displays and scaling    PASS   eDP-1 2560x1600@165.00 scale 2.00
                                DP-1  3840x2160@60.00 scale 1.00
  portal dispatcher       PASS   15 interfaces, hyprland + gtk backends up
  notifications           PASS   Notify returned id 1
  shortcuts               PASS   34 bindings, Super+Space opened the launcher
  file chooser            PASS   zenity --file-selection returned a chooser
  printing                PASS   lpstat -r: printer is ready
  compositor IPC          PASS   hyprctl version 0.51.1

  2 checks need a human:  suspend/resume,  screen sharing
  11 passed, 0 failed, 2 manual.  Adoption looks good. Keep ~/.local/state/zenith/.

$ echo $?
0
```

Exit `0` if nothing failed, `1` if anything failed. A failed check names the
[recovery](#failure-and-recovery) row that matches.

### The probes

Each scriptable check is one command. These are the actual probes, so a user who disagrees
with the tool can run them by hand.

```bash
# Session identity and type
printf '%s %s\n' "$XDG_CURRENT_DESKTOP" "$XDG_SESSION_TYPE"      # Zenith wayland
loginctl show-session "$XDG_SESSION_ID" -p Type -p Desktop
#   Type=wayland
#   Desktop=zenith

# Compositor and shell
hyprctl version | head -1
systemctl --user is-active zenith-shell.service                  # active

# Audio
wpctl status | sed -n '/Audio/,/Video/p'
#   │  * Analog Stereo Output                [vol 0.42]

# Displays and scaling
hyprctl monitors all
#   Monitor eDP-1 (DPMS 2560x1600): at 2560x1600, 165.00Hz, scale: 2.00
#   Monitor DP-1  (DPMS 3840x2160): at 3840x2160,  60.00Hz, scale: 1.00

# Shortcuts
hyprctl binds | wc -l                                          # 34

# Notifications — the Notify probe
gdbus call --session --dest org.freedesktop.Notifications \
  --object-path /org/freedesktop/Notifications \
  --method org.freedesktop.Notifications.Notify \
  "portal probe" 0 "" "" [] {} 3000
#   (uint32 1,)      ← a banner must appear. A returned id with no banner means
#                     the shell is up but is not the server.
```

```bash
# File choosers — the FileChooser probe
zenity --file-selection --title "portal probe"
#   A chooser opens. It must be the GTK backend's; a plain GTK default with no
#   portal styling means xdg-desktop-portal-gtk never activated.

# Printing
lpstat -r
#   printer <name> is ready.  accepting jobs since <date>
```

### What only a human can confirm

`--verify` cannot decide these. It tells the user to, in this order, because each one that
fails is worth stopping at.

| Check | How | Pass |
|---|---|---|
| Login at the display manager | Log out, pick Zenith at the greeter | The session starts and the shell appears |
| Existing applications still run | Launch one GTK app, one Qt app, one Electron app, one flatpak, one snap | All five start and render at the right scale |
| Audio | Play something | Sound comes out the right sink, and the shell's volume widget tracks it |
| Suspend and resume | Close the lid. Or `systemctl suspend`. | The machine sleeps and comes back to the locked shell, with no black screen and no lost session |
| Screen sharing | Share a screen from the browser | A source is pickable and the share is smooth |
| Printing | Print a test page | It comes out |
| Display scaling | Compare text crispness against the GNOME session | Crisp on the internal panel at 2.0, native on an external 1.0 monitor |
| Notifications | `notify-send 'probe' 'Zenith adoption verification'` | A banner appears with the app icon |
| Keyboard shortcuts | Trigger the launcher, the terminal, the screenshot key | All three work |
| Tray icons | Look for an app with a tray icon | Its icon is in the shell |
| Night light | Toggle it in quick settings | The screen warms and the compositor reports it |
| External monitor after suspend | Plug in a monitor, suspend, resume | The monitor comes back at the right mode |

### The machine-readable form

`zenith adopt --verify --json` emits the same results as data, which is how a CI job or a
contributor's hardware report consumes it. The report that goes into a profile pull request is
`zenith hardware report`, not this.

---

## Failure and recovery

The promise, restated: **a failed adoption must never leave the machine unbootable.** It
cannot, because nothing in the feature touches the kernel, the initramfs, the bootloader or the
partition table, and the display manager is left alone by default. Everything below is a
recoverable inconvenience.

### Getting back, fast

Three commands, in order of how much they assume is working:

```bash
# 1. Nothing. Pick the other session at the login screen. This is enough
#    if adoption stopped after step one, which is the common case.

# 2. From a TTY (Ctrl+Alt+F3, or Fn+Ctrl+Alt+F3 on Fn-gated layouts):
sudo rm -f /usr/share/wayland-sessions/zenith.desktop    # Zenith session gone, GNOME intact

# 3. The full thing, from a TTY:
sudo bash /etc/zenith/adopt/undo.sh                      # or: sudo zenith adopt --undo
```

`zenith adopt --undo` refuses to run while `$XDG_CURRENT_DESKTOP` is `Zenith`, because removing the
session file out from under a running session is a bad idea. Log out first, or pass `--force`
if you are already at a TTY.

### Failure modes

| What the user sees | What actually happened | What they run | How they get back |
|---|---|---|---|
| No **Zenith** entry in the greeter's session wheel | The registration was not materialized, or `Exec` is not executable | `ls -l /usr/share/wayland-sessions/zenith.desktop /usr/share/zenith/session/zenith-session` | Pick GNOME. Re-run `sudo zenith adopt`. |
| Zenith appears but the wheel is still missing | The picker is menu-driven rather than session-driven | `sudo update-desktop-database` | Pick GNOME |
| Login, then straight back to the login screen | `zenith-session` failed before `exec Hyprland` — bad `/etc/zenith/session/zenith-session.env`, unreadable schema, full disk | `cat ~/.local/state/zenith/session.log` | Pick GNOME. Fix the line, log back in. |
| Black screen with a moving cursor | Hyprland started, `zenith-shell.service` did not | `Ctrl+Alt+F3`, then `journalctl --user -u zenith-shell.service -b` | `systemctl --user daemon-reload` then `systemctl --user restart zenith-shell` from the TTY, or pick GNOME |
| Shell flickers and restarts in a loop | A Quickshell load error, almost always | `journalctl --user -u zenith-shell.service -b --no-pager -n 40` | `systemctl --user stop zenith-shell.service`, log out to GNOME, reinstall `zenith-shell` |
| Apps open but file choosers never appear | `xdg-desktop-portal` never started, or the preference still says `use=gnome` | `pgrep -a xdg-desktop-portal` and `cat ~/.config/xdg-desktop-portal/portals.conf` | `systemctl --user restart xdg-desktop-portal.service`; restore the archived preference from `~/.local/state/zenith/adopt/portals.conf.gnome` |
| "No available screen capture source" | The Hyprland portal backend did not load, or the NVIDIA capture path is the known rough edge | `pgrep -a xdg-desktop-portal-hyprland` | `systemctl --user restart xdg-desktop-portal-hyprland`; use the app's native capture instead |
| Everything is blurry, or tiny text | Per-monitor scale is wrong. GNOME's fractional scaling has no Hyprland equivalent, so this is never imported | `hyprctl monitors all`, then set it in `/etc/zenith/hyprland/monitors.conf` and `hyprctl reload` | `hyprctl reload`; pick GNOME |
| No sound, or the wrong sink | `wireplumber` did not start, or the GNOME session had been providing the sink policy | `wpctl status`, `systemctl --user status pipewire wireplumber` | `systemctl --user restart wireplumber` |
| No notifications anywhere | The shell owns the notification server, and it is not running | `notify-send 'probe' 'does this appear'` | `systemctl --user restart zenith-shell` |
| No tray icons | `StatusNotifierItem` hosting is not implemented or is not running yet | — | Log in to GNOME, where the AppIndicator extension still works |
| Screen black after resume | The most-reported laptop bug class; [`architecture.md`](architecture.md#the-hard-list) calls it out for exactly this reason | `journalctl -b -u systemd-suspend.service`, `hyprctl monitors` | `hyprctl reload`; if it will not come back, suspend again, or pick GNOME |
| No login screen at all after `--display-manager sddm` | The display manager swap went wrong. **The one genuinely bad outcome**, and it is why the swap is gated on a passing `--verify` and a confirmed TTY login. | `Ctrl+Alt+F3`, then `sudo systemctl set-default display-manager gdm3 && sudo systemctl enable gdm3` | The TTY. This is the recovery of last resort, and it works because nothing about it depends on anything adoption changed. |
| `zenith` itself is gone or broken | Something removed or corrupted the control tool | `sudo sh /etc/zenith/adopt/undo.sh` — a plain shell script, no binary required | The TTY, and the script |
| Nothing in this table matches | — | `Ctrl+Alt+F3`, log in, `sudo sh /etc/zenith/adopt/undo.sh` | Pick the GNOME session at the login screen |

### Why the TTY always works

A virtual terminal is served by `systemd`, `getty` and the kernel console driver. Adoption
installs packages, writes one session registration, writes one portal preference file and
starts services inside a graphical session. It does not touch `systemd`, the kernel, the
console or `getty`, so `Ctrl+Alt+F3` works on a machine that adoption has damaged in every way
adoption is capable of damaging.

### The recovery of last resort

If a machine is so broken that even that is not enough — which nothing in this design produces,
but which a user will arrive at anyway:

```bash
# Boot any live image. Mount the root filesystem, chroot, and:
sudo chroot /mnt /bin/sh -c '/etc/zenith/adopt/undo.sh && systemctl set-default display-manager gdm3'
```

Everything `undo.sh` needs is on the machine, with one exception: if prune removed packages, the
reinstall step in step 1 needs the network. On a machine that cannot get one, edit the list or
skip the script — deleting `/usr/share/wayland-sessions/zenith.desktop` and running
`systemctl set-default display-manager gdm3` inside the chroot is enough to get a working GNOME
login back, and the missing packages can be reinstalled afterwards from any network.

---

## Command surface

Consistent with `zenith update | rollback | materialize | reset | doctor` from
[`architecture.md`](architecture.md#overlay-and-rollback): one verb per job, flags for the
variation, `--help` is the reference, and nothing writes a setting another command owns.

| Command | What it does | Needs root | Reversible |
|---|---|---|---|
| `zenith adopt` | Preflight, plan, then steps 0–11. Adds the Zenith session and the shell. Removes nothing. | Yes, once, via `pkexec` | Yes — `zenith adopt --undo` |
| `zenith adopt --check` | Report what would change. Changes nothing. | No | Nothing to reverse |
| `zenith adopt --check --json` | The same report, machine-readable. | No | Nothing to reverse |
| `zenith adopt --apply` | The default. Present for symmetry with `--prune`. | Yes | Yes |
| `zenith adopt --verify` | Post-login verification inside a Zenith session. Read-only. | No | Nothing to reverse |
| `zenith adopt --verify --json` | The same results, machine-readable. | No | Nothing to reverse |
| `zenith adopt --undo` | Reinstall what prune removed, remove every registration, restore every file and dconf value adoption wrote, restore the archived extensions and portal preference. | Yes | Yes — it is the thing everything else is reversible by |
| `zenith adopt --undo --force` | The same, while a Zenith session is running. | Yes | Yes |
| `zenith adopt --prune` | **Dry run by default.** Lists every package it would remove and why each is safe. | No | Nothing to reverse |
| `zenith adopt --prune --apply` | Remove them. Simulates first; refuses if APT wants to remove anything unclassified. | Yes | Yes — `zenith adopt --undo` reinstalls the recorded set |
| `zenith adopt --prune --purge` | Also remove conffiles. Rarely wanted. | Yes | **No.** Outside the reversibility promise, by design |
| `zenith adopt --display-manager sddm` | Install SDDM, switch the default, enable it. Requires a reboot. Refuses until `--verify` passes. | Yes | Yes — `--display-manager gdm3`, plus a reboot |
| `zenith adopt --display-manager gdm3` | Put GDM back. | Yes | Yes, plus a reboot |
| `zenith adopt --apply --refresh` | Repaint the three registrations from the packaged base. Idempotent. | Yes | Yes |
| `zenith adopt --status` | What has adoption done to this machine? Reads `/etc/zenith/adopt/state.json`. | No | Read-only |

Consistency rules with the existing surface:

- `zenith` never grows an `adopt-*` command. It grows `adopt`, with flags.
- `zenith doctor` reports adoption drift — a materialized registration that no longer matches
  `/usr/share/zenith/` — alongside every other drift, because it is the same class of drift.
- `zenith reset` and `zenith materialize` do not touch registrations. A materialized registration is
  repainted by `zenith adopt --apply --refresh`, not by `zenith materialize`, because the registration
  is not a base file with an overlay — it is a derived copy.
- **No flag writes an `org.zenith.*` key.** One writer per setting means Zenith's own settings go
  through `zenith-shell` and `zenith-hub`, and `zenith adopt` is no exception: step 7's import is a one-time
  seeding of unset keys, which `zenith-shell` then owns. `zenith adopt` writes `org.gnome.*` dconf
  values in exactly two places — the imported values and the terminal default — and records every
  one of them in `state.json`.

### Exit codes, shared across the subcommands

| Code | Meaning |
|---|---|
| `0` | Success, or nothing needed doing. |
| `1` | Failed, or blocked by a preflight check. |
| `2` | Completed with warnings. |
| `3` | Refused: a safety precondition failed. The message says which one. |

---

## Ubuntu runtime dependencies

Adoption installs Zenith's own packages from its signed APT repository (`zenith-shell`, `zenith-hub`,
`quickshell`, `hyprland`, and the session/portal stack). The rest of the desktop runs on Ubuntu
26.04 base-layer packages. The table below lists every base-layer package the Zenith session
visibly depends on, what breaks without it, and the `apt` line to restore it.

All of these are in `main` or `universe` on Ubuntu 26.04. Install with `--no-install-recommends`
to avoid pulling GNOME metapackages.

| Package | Purpose | Visible degradation without it | Restore command |
|---|---|---|---|
| `hyprland` | Wayland compositor | No desktop at all — black screen, no windows | `sudo apt install --no-install-recommends hyprland` |
| `xdg-desktop-portal` | Portal dispatcher | File choosers, screen share, screenshots, printing all fail silently | `sudo apt install --no-install-recommends xdg-desktop-portal` |
| `xdg-desktop-portal-hyprland` | Hyprland portal backend (ScreenCast, Screenshot, GlobalShortcuts) | Screenshots and screen sharing produce nothing; global shortcuts via portal don't work | `sudo apt install --no-install-recommends xdg-desktop-portal-hyprland` |
| `xdg-desktop-portal-gtk` | GTK portal backend (FileChooser, Print, OpenURI, Secret via lxqt) | GTK file choosers hang; print dialog never appears | `sudo apt install --no-install-recommends xdg-desktop-portal-gtk` |
| `xdg-desktop-portal-lxqt` | Secret portal backend | Flatpaks (Firefox, VS Code, Thunderbird) cannot store passwords | `sudo apt install --no-install-recommends xdg-desktop-portal-lxqt` |
| `gnome-keyring` | Secret service daemon (backing store for lxqt backend) | Secret portal starts but has nowhere to write credentials | `sudo apt install --no-install-recommends gnome-keyring` |
| `libpam-gnome-keyring` | PAM module unlocking the keyring at login | Every secret prompt asks for a second password | `sudo apt install --no-install-recommends libpam-gnome-keyring` |
| `pipewire` + `pipewire-pulse` + `wireplumber` | Audio stack | No sound; volume widget shows nothing | `sudo apt install --no-install-recommends pipewire pipewire-pulse wireplumber` |
| `power-profiles-daemon` | Power policy (governors, platform profiles) | Power mode stuck; sidebar power control does nothing; laptop runs hot/cold | `sudo apt install --no-install-recommends power-profiles-daemon` |
| `upower` | Battery reporting | Sidebar shows no battery info; no low-battery actions | `sudo apt install --no-install-recommends upower` |
| `iwd` + `network-manager` | Wi-Fi and wired networking | No network; sidebar network section empty | `sudo apt install --no-install-recommends iwd network-manager` |
| `netplan.io` | NetworkManager persistent config storage | Network settings not persisted across reboots | `sudo apt install --no-install-recommends netplan.io` |
| `bluez` | Bluetooth daemon (provides `bluetoothctl`) | Sidebar Bluetooth section empty; no device pairing | `sudo apt install --no-install-recommends bluez` |
| `fprintd` | Fingerprint authentication | Fingerprint login/enroll unavailable | `sudo apt install --no-install-recommends fprintd` |
| `brightnessctl` | Internal panel backlight control | Brightness slider in sidebar does nothing (reads work, writes need udev rule) | `sudo apt install --no-install-recommends brightnessctl` |
| `ddcutil` | External monitor brightness/contrast | External monitor controls in sidebar do nothing | `sudo apt install --no-install-recommends ddcutil` |
| `playerctl` | MPRIS media player control (sidebar media widget) | Media widget shows nothing; media keys (XF86Audio*) no-op | `sudo apt install --no-install-recommends playerctl` |
| `grim` + `slurp` + `hyprpicker` | Screenshot toolchain (shells out from Hyprland portal) | Screenshot keybindings produce nothing; colour picker absent | `sudo apt install --no-install-recommends grim slurp hyprpicker` |
| `hyprlock` | Lock screen (PAM authentication) | `Super+L` and lid-close lock do nothing; no lock screen at all | `sudo apt install --no-install-recommends hyprlock` |
| `wl-clipboard` | Wayland clipboard (`wl-copy`, `wl-paste`) | Clipboard sync between Wayland/XWayland apps broken | `sudo apt install --no-install-recommends wl-clipboard` |
| `xdg-utils` | `xdg-open`, `xdg-email`, `xdg-settings` | "Open in browser/file manager" actions fail everywhere | `sudo apt install --no-install-recommends xdg-utils` |
| `qt6-wayland` | Qt6 Wayland platform plugin | Qt apps flicker on resize, go grey at fractional scaling | `sudo apt install --no-install-recommends qt6-wayland` |
| `libgtk-4-1` + `libadwaita-1-0` | GTK4/libadwaita runtime for third-party apps | GTK4 applications fail to start | `sudo apt install --no-install-recommends libgtk-4-1 libadwaita-1-0` |
| `libglib2.0-bin` | `gsettings` CLI (settings read/write) | `zenith config` and shell settings controls stop working | `sudo apt install --no-install-recommends libglib2.0-bin` |
| `fwupd` | Firmware updates via LVFS | No firmware update capability in Hub | `sudo apt install --no-install-recommends fwupd` |
| `switcheroo-control` | Hybrid GPU mux switching | Hybrid laptops stuck on wrong GPU | `sudo apt install --no-install-recommends switcheroo-control` |
| `rtkit` | Real-time scheduling for PipeWire | Audio crackles under load | `sudo apt install --no-install-recommends rtkit` |
| `accountsservice` | User enumeration for SDDM greeter | Login screen shows empty/uneditable user field | `sudo apt install --no-install-recommends accountsservice` |
| `sddm` | Login manager (Zenith OS default) | No graphical login; adoption leaves GDM in place unless `--display-manager sddm` | `sudo apt install --no-install-recommends sddm` |

### Zenith-provided packages (not in Ubuntu archive)

These come from Zenith's own signed repository and are installed by `zenith adopt` automatically.
If the repository is not yet configured, `zenith adopt --check` will report it as a blocker.

| Package | Purpose | Visible degradation without it |
|---|---|---|
| `zenith-shell` | Quickshell engine (built by Zenith) | No shell at all — bar, dock, launcher, notifications, idle, quick settings, status icons all absent; ~700+/min `ServerNotFoundError` noise in logs |
| `zenith-hub` | Settings application (Go) | No settings UI; `zenith-hub` command missing |
| `zenith-session` | Session entry point + `zenith-activate` | Zenith session doesn't appear in greeter or fails to start |
| `quickshell` | QML runtime engine (built by Zenith) | Shell cannot start; `zenith-shell.service` fails immediately |
| `zenithd` | State daemon (socket at `$XDG_RUNTIME_DIR/zenith-shell.sock`) | Sidebar sections (audio, network, bluetooth, display, media) show stale data or nothing; no live updates |

### One-line restore for the common missing set

```bash
sudo apt update && sudo apt install --no-install-recommends \
  hyprland xdg-desktop-portal xdg-desktop-portal-hyprland xdg-desktop-portal-gtk \
  xdg-desktop-portal-lxqt gnome-keyring libpam-gnome-keyring \
  pipewire pipewire-pulse wireplumber \
  power-profiles-daemon upower iwd network-manager netplan.io \
  bluez fprintd brightnessctl ddcutil playerctl \
  grim slurp hyprpicker hyprlock wl-clipboard xdg-utils \
  qt6-wayland libgtk-4-1 libadwaita-1-0 libglib2.0-bin \
  fwupd switcheroo-control rtkit accountsservice sddm
```

> **Note:** `zenith-shell`, `zenith-hub`, `zenith-session`, `quickshell`, and `zenithd` are not in the
> Ubuntu archive — they come from `packages.zenith.sh`. The command above restores only base-layer
> dependencies. Run `zenith adopt` to install the Zenith packages.

## What is still open

| Question | Why it matters here |
|---|---|
| Whether `zenith adopt` is offered at all, or only as a standalone download | A machine on stock Ubuntu has no Zenith packages and no reason to add a repository until the user chooses to. The first-run experience decides this more than the code does. |
| Whether adoption contributes hardware profiles | A community-confirmed profile from an adopted machine is worth as much as one from a Zenith OS install. The difference is that a Zenith OS install already ran the installer against `system/hardware/profiles/`, and an adopted machine did not — so the report has less context to attach. |
| NVIDIA screen capture under `xdg-desktop-portal-hyprland` | Already listed as open in [`architecture.md`](architecture.md#open-questions) for portal coverage. Adoption sharpens it: an adopted machine keeps whatever driver its user already had, so it will hit this sooner than a curated Zenith image. |
| Whether prune should also handle `snapd` | Zenith OS does not. Adoption deliberately does not either, because snapd may hold the user's applications. Whether an *opt-in* `zenith adopt --prune --snaps` is ever worth having is undecided. |
| The `Settings` portal | No backend implements it on a non-GNOME stack. The mitigation is GSettings, and whether that is sufficient for real applications has not been measured. |

Related documents: [`architecture.md`](architecture.md) for the settled decisions this
implements, [`agents/gnome-shell-notes.md`](../agents/gnome-shell-notes.md) for the GNOME
Shell internals this design has to work around, and [`agents/agents.md`](../agents/agents.md)
for the ownership rules that make `--undo` possible.