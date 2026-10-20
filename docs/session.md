# The session path

Zenith from power-on to a desktop, and back down again. This document fixes the order of
everything on that path and names the owner of each step. A desktop with two owners for one
job is a desktop that breaks in ways nobody can reproduce, so every stage below has exactly
one owner and the reason for every ordering is written next to it.

Three rules run through the whole document:

- **One owner per stage.** Where a base-layer component owns a step, Zenith does not
  duplicate it. Where no base-layer component owns it, Zenith does.
- **Order is a dependency, not a preference.** Where two things must happen in a particular
  sequence, the failure that the sequence avoids is named.
- **Everything is a file.** Every unit, config file and desktop entry named here is a file
  on the image, with a path and an owner.

The companion documents are [`architecture.md`](architecture.md) for the decisions and
[`adopting-zenith.md`](adopting-zenith.md) for putting an existing machine on this path.
Where this document repeats a decision, it repeats the reason with it.

---

## Where Zenith's files live

Zenith's packages write to `/usr/share/zenith/` and `/etc/zenith/`, and to nothing else that
belongs to another package. A handful of files have to sit outside those two directories
anyway, because the components that read them look nowhere else. Listing them is the point:
a boundary that is not written down gets crossed by accident.

| Path | What it is | Why it cannot live under `/usr/share/zenith/` |
|---|---|---|
| `/usr/share/wayland-sessions/zenith.desktop` | Session registration | The login manager scans the fixed `wayland-sessions` directory and nowhere else |
| `/usr/lib/systemd/user/zenith-shell.service`, `zenith-portal.service` | User unit registration | `systemd --user` loads units from the fixed unit directory |
| `/usr/lib/systemd/user/hyprland-session.target.d/10-zenith.conf` | Drop-in | systemd reads drop-ins from a fixed directory beside the unit |
| `/usr/share/glib-2.0/schemas/org.zenith.*.gschema.xml` | GSettings schema | GLib compiles schemas from the fixed schema directory |
| `/usr/share/xdg-desktop-portal/zenith-portals.conf` | Portal backend preference | The dispatcher reads it from a fixed directory. On the sanctioned list in [`architecture.md`](architecture.md#the-sanctioned-list-outside-the-boundary) |
| `/usr/share/sddm/themes/zenith/` | Login screen theme | The login manager reads themes from a fixed directory. Same list |
| `/boot/limine.conf` | Boot configuration | The boot loader reads its configuration from the boot volume, which the base layer's kernel packages already write to |

These are registrations — pointers a third-party binary must find — not Zenith-owned content.
The content stays at `/usr/share/zenith/session/`, `/usr/share/zenith/shell/`,
`/usr/share/zenith/portal/` and `/usr/share/zenith/schemas/`, and `zenith doctor` reports
drift between the two the same way it reports drift everywhere else.

And the list of things Zenith deliberately does **not** touch:

- `/etc/sddm.conf` and `/etc/sddm.conf.d/` — the login manager's configuration, which Zenith
  leaves at its defaults.
- `/etc/pam.d/` — the PAM stack. The keyring is unlocked by the base layer's own PAM module.
- `/etc/initramfs-tools/` — the initramfs configuration. Zenith's copy lives in
  `/etc/zenith/initramfs/` and is passed to `mkinitramfs` with `-d`.
- `/usr/share/xdg-desktop-portal/portals/` — the dispatcher's backend directory.
- `/usr/lib/udev/rules.d/`, `/etc/NetworkManager/`, and every other package's configuration.

---

## Boot chain

### Stages and owners

| Stage | Owner | What it owns | Zenith's part |
|---|---|---|---|
| Firmware | The machine | Secure Boot verification, the EFI variable store, the boot order | Nothing |
| Secure Boot shim | Base layer | First-stage verification, the MOK manager, the chainloader slot | Nothing |
| Limine | `zenith-boot` | The boot menu, loading the kernel and the initramfs, the boot parameters | The config, the volume layout, the hashes |
| Kernel | Base layer, pinned by `system/kernel/` | Drivers, the initramfs unpacker, early microcode | The fleet, the initramfs contents |
| initramfs | Base layer's initramfs-tools, tuned by Zenith | Root discovery, the microcode blob, zram | The config directory, the module list |
| systemd | Base layer | The system manager, logind, the per-user manager | The unit files, the session units |

### The four Limine stages

| Stage | What it does | What it owns |
|---|---|---|
| 1 | The boot sector, or the EFI entry point | Finding and loading stage 2, passing on the firmware's boot context |
| 2 | The loader proper | The filesystem drivers (FAT12/16/32, ISO9660), the config parser, the menu, loading stage 3 |
| 3 | The protocol stage | Building the boot structures that describe the machine — memory map, framebuffer, module list, command line |
| 4 | `limine-exec`, the final stage | Loading the kernel and the initramfs into memory, setting the video mode and the kernel boot parameters, handing over |

Limine reads **FAT12/16/32 and ISO9660 and nothing else**. It cannot open an ext4
partition. Two designs follow from that, and Zenith takes the first:

1. The kernel and the initramfs live on the FAT volume and Limine loads them directly. No
   second bootloader is involved, so the Secure Boot chain is one link shorter and there is
   no second boot configuration to keep in sync with the kernel fleet.
2. Limine chainloads a second bootloader that can read ext4, and that bootloader loads the
   kernel. The chain grows by one link, that bootloader becomes part of the trust chain, and
   it becomes part of the update path.

### The boot volume

The `ZENITHBOOT` volume is FAT32 and is mounted at `/boot`. It is the ESP, and it carries
the whole boot chain.

| Path on `ZENITHBOOT` | Contents |
|---|---|
| `limine.sys` | The Limine configuration search target on the boot volume |
| `limine-bios.sys` | The BIOS boot record payload |
| `EFI/BOOT/BOOTX64.EFI` | Limine's EFI binary, and the fallback boot path |
| `EFI/zenith/shimx64.efi` | The Secure Boot first stage |
| `EFI/zenith/mmX64.efi` | The MOK manager |
| `EFI/zenith/grubx64.efi` | Shim's chainloader slot, holding Limine's EFI binary |
| `vmlinuz-<version>` | The kernel |
| `initrd.img-<version>` | The initramfs |
| `limine.conf` | The boot configuration |

FAT constrains what can live here, and the design works with the constraints rather than
against them:

- **No symbolic links.** `/boot/initrd.img` and `/boot/vmlinuz` do not exist. Limine is
  pointed at versioned file names.
- **No execute bit.** Every helper script lives under `/usr/share/zenith/`, never under
  `/boot`.
- **Case-insensitive names.** Every kernel and initramfs file name is lower case.
- **No ownership.** Nothing on `/boot` is trusted to be root-owned; Limine verifies the
  files by hash instead, which is the only integrity claim the volume can support.

### The Limine configuration

`/boot/limine.conf`, regenerated by `zenith-boot` on every kernel change:

```ini
timeout: 5
default_entry: 1
quiet: yes
interface_branding: Zenith OS
interface_branding_colour: 000000

/Zenith OS
protocol: linux
path: boot():/vmlinuz-6.14.0-33-generic#9f86d081884c7d659a2feaa0c55ad015
module_path: boot():/initrd.img-6.14.0-33-generic#ca6914d2a3b8c1f0e5d6473ab19e5c0
cmdline: quiet loglevel=4 systemd.show_status=true root=UUID=6a1f9c40-2b7e-4d55-9a11-3c8e0f7b2d64 rw
```

Four things in that file are load-bearing:

- **The config lives at the volume root.** Limine searches, in order, the directory of its
  own EFI application, then `/boot/limine/limine.conf`, `/boot/limine.conf`,
  `/limine/limine.conf`, `/limine.conf`; the first match wins and the rest are ignored. One
  canonical path means one file, and the installer must make sure no other volume on the
  disk carries a `limine.conf` that would shadow it.
- **The hash suffix is not optional once the config hash is enrolled.** Limine refuses to
  load any file whose path does not carry its BLAKE2B digest, and panics on a mismatch.
  `zenith-boot` computes the digests when it writes the file.
- **Limine verifies by hash, not by signature.** It does not use a UKS. What enforces an
  unmodified configuration is the **BLAKE2B hash of `limine.conf` enrolled inside the EFI
  binary** with `limine enroll-config`, which is why every path in the file needs a digest
  once that hash is enrolled. Separately, on a Windows-initialised firmware only Microsoft
  keys are enrolled, so shim must present a MOK for Zenith's own key before it can chainload
  Limine at all. Two separate problems, two separate answers.
- **The kernel command line carries the root device.** The initramfs resolves the root
  filesystem from it; the installer writes the UUID from the machine's partition table and
  `zenith-boot` rewrites the line on every kernel change.

### Microcode has to load early

The CPU runs the wrong microcode until something loads the right one, and a machine that
needs an update for stability is unstable during exactly the window that matters. The base
layer's `intel-ucode` and `amd64-microcode` packages ship initramfs-tools hooks that call
`prepend_earlyinitramfs`, and `mkinitramfs` writes that blob as a **separate cpio archive in
front of** the main initramfs. The kernel's built-in early-microcode loader applies the
first archive before it unpacks the real one, so the CPU is correct from the first
instruction rather than from the moment the root filesystem is mounted.

Two consequences for Zenith:

- The microcode has to be **inside the initramfs**, not only in `/lib/firmware`. An image
  built without those packages boots on fallback microcode for the whole initramfs phase.
- `zenith-boot` rebuilds the initramfs whenever a kernel **or a microcode package** changes,
  so the blob is never stale relative to the kernel.

### The initramfs

The base layer uses initramfs-tools, and Zenith keeps it. What Zenith changes is the
configuration directory and the module set.

```
mkinitramfs -d /etc/zenith/initramfs -o /boot/initrd.img-6.14.0-33-generic 6.14.0-33-generic
```

| Setting | Value | Why |
|---|---|---|
| `MODULES` | `most` | A laptop may boot from NVMe, USB or a RAID array. The module set has to cover all of them, because a missing storage driver is an unbootable machine. |
| `COMPRESS` | `zstd` | Decompresses faster than gzip on the same size, which is the part of the boot that is on the critical path. |
| `BOOTSIZE` | `20%` | Enough for the storage drivers, small enough to stay in memory. |
| `KEYMAP` | from the installer | The keyboard has to work at the login prompt, which is a VT, not a compositor. |

Tier-1 machine profiles may drop `MODULES` to `dep` and ship a smaller initramfs, but only
after the profile has proved the machine boots from that set. A profile that guesses wrong
produces an unbootable machine that no amount of debugging can reach.

zram is configured in the initramfs, not by the systemd generator: `systemd-zram-generator`
is a systemd generator that runs at boot and writes the units that create and swap on a
zram device, and it does not by itself place zram in the initramfs. The initramfs
configuration is what does that.

### systemd takes over

The initramfs mounts the root filesystem and executes `/sbin/init`, which is systemd. From
that point the boot is a dependency graph, and the rest of this document is that graph.

---

## The systemd target chain

The system side first, then the user side.

### The system instance

```ini
default.target
└── graphical.target                                 Wants=display-manager.service
    │                                                Requires=multi-user.target
    │                                                After=multi-user.target rescue.service
    │                                                rescue.target display-manager.service
    ├── display-manager.service                      [Install] Alias= of sddm.service
    │   └── sddm.service                             After=systemd-user-sessions.service
    │                                                getty@tty1.service plymouth-quit.service
    │                                                systemd-logind.service
    │                                                Conflicts=getty@tty1.service
    │                                                Restart=always
    └── multi-user.target                            Requires=basic.target
        │                                            After=basic.target
        └── basic.target                             Wants=sockets.target timers.target paths.target
                                                     slices.target tmp.mount
                                                     After=sysinit.target sockets.target paths.target
                                                     slices.target tmp.mount
            └── sysinit.target                       Wants=local-fs.target swap.target
                                                     After=local-fs.target swap.target
```

`default.target` is a symlink to `graphical.target`, and `graphical.target` wants
`display-manager.service` — an alias created by the `[Install]` section of `sddm.service`.
Zenith never edits `sddm.service`; the only command that puts a login manager on the image
is `systemctl enable display-manager.service`.

### The user instance

The session is not a system service. It is a set of user units, and the root of it is
`graphical-session.target`, which systemd defines as "active whenever any graphical session
is running".

```ini
user@1000.service                                  started by systemd-logind for the seat session
│                                                   PAMName=systemd-user, Slice=user-1000.slice
└── systemd --user
    └── default.target
        ├── graphical-session-pre.target             environment setup, agents, autostart
        └── hyprland-session.target                 activated by the compositor itself
            │                                        Wants=wireplumber.service
            │                                        BindsTo=graphical-session.target
            ├── wireplumber.service                 audio policy
            ├── zenith-portal.service                the portal dispatcher
            │   │                                    After=hyprland-session.target
            │   │                                    ConditionEnvironment=WAYLAND_DISPLAY
            │   └── xdg-desktop-portal-hyprland      activated on demand by the dispatcher
            │       xdg-desktop-portal-gtk          activated on demand by the dispatcher
            └── zenith-shell.service                the shell
                │                                    After=hyprland-session.target
                │                                    After=zenith-portal.service
                │                                    After=wireplumber.service
                │                                    ConditionEnvironment=WAYLAND_DISPLAY
                └── the shell's own children         lock screen, quick settings,
                                                     notification server — inside the shell
```

Two systemd mechanisms carry the weight here.

- **`BindsTo=graphical-session.target` on the session target** starts the target when the
  session starts and stops it when the session ends.
- **`PartOf=graphical-session.target` on every session service** stops that service when
  the session ends. A unit that is only `Wants=`-ed by the session target would survive
  logout and be adopted by the next session, which is how a second copy of the shell ends up
  running after a fast login cycle.

The upstream compositor ships its own session target, and Zenith's units attach to it with
a drop-in. That is the sanctioned way to affect systemd without owning systemd:

```ini
# /usr/lib/systemd/user/hyprland-session.target.d/10-zenith.conf
[Unit]
Wants=zenith-portal.service zenith-shell.service
After=zenith-portal.service
```

### The units Zenith ships

```ini
# /usr/lib/systemd/user/zenith-portal.service
[Unit]
Description=Zenith portal dispatcher
PartOf=graphical-session.target
After=hyprland-session.target
ConditionEnvironment=WAYLAND_DISPLAY

[Service]
Type=dbus
BusName=org.freedesktop.portal.Desktop
ExecStart=/usr/libexec/xdg-desktop-portal
Restart=on-failure
RestartSec=1

[Install]
WantedBy=hyprland-session.target

# /usr/lib/systemd/user/zenith-shell.service
[Unit]
Description=Zenith shell (Quickshell)
Documentation=man:zenith-shell(1)
PartOf=graphical-session.target
After=hyprland-session.target zenith-portal.service wireplumber.service
Requires=hyprland-session.target
ConditionEnvironment=WAYLAND_DISPLAY

[Service]
Type=exec
ExecStart=/usr/bin/quickshell -p /usr/share/zenith/shell
ExecReload=/bin/kill -SIGUSR1 $MAINPID
Restart=on-failure
RestartSec=1
Slice=app.slice

[Install]
WantedBy=hyprland-session.target
```

`zenith-portal.service` is a shadow of the base layer's `xdg-desktop-portal.service` under
a `zenith-` name, which is the pattern the base-layer boundary prescribes for a component
that needs different configuration: never an edit in place, always a drop-in of your own.

The base layer's unit must not also be enabled. Two dispatchers on one bus is one
dispatcher that wins the name and one that loses it, and the loser exits — the name is
taken from the bus, not from the unit, so the second instance terminates with "dbus name
was lost" and takes the session's portal with it.

---

## The login manager and the session file

The login manager is the base layer's, and Zenith uses it as shipped.

### What it does

1. Starts the greeter on the first virtual terminal.
2. Authenticates the user through PAM, in the `sddm-session` service.
3. Starts the session file the user selected, as that user, with the session's environment.

The greeter is a **Wayland** surface. The base layer's login manager has no Wayland greeter of
its own, so Zenith supplies one: `DisplayServer=wayland` in a drop-in under `/etc/sddm.conf.d/`,
with `CompositorCommand` pointing at `zenith-greeter`, which runs `weston --shell=kiosk` to hold
the DRM card and hosts Zenith's own login surface on it. The package manifest therefore excludes
the Xorg metapackage and ships XWayland, and nothing is needed for the login screen at all —
which is the point. XWayland remains the compositor serving X11 applications inside the session;
it is not a second session type, and it is not what draws the login screen.

This is settled, and the reasoning is written up in
[`architecture.md`](architecture.md#what-draws-the-login-screen).

### The session file

The login manager reads Wayland sessions from `/usr/share/wayland-sessions/` and runs the
`Exec=` line of the selected file through its own session script, which sources the user's
login profile and then executes it.

```ini
# /usr/share/zenith/session/zenith.desktop
# materialized to /usr/share/wayland-sessions/zenith.desktop
[Desktop Entry]
Type=Application
Name=Zenith
Comment=Zenith desktop
TryExec=/usr/share/zenith/session/zenith-session
Exec=/usr/share/zenith/session/zenith-session
DesktopNames=Zenith
Keywords=Hyprland;Quickshell;
```

Three lines carry the design:

- **`Exec=/usr/share/zenith/session/zenith-session`.** The session entry point is Zenith's
  own script, and it runs from the packaged base rather than from a copy in the user's home.
- **`DesktopNames=Zenith`**, not `GNOME`. The login manager uses this to group and label
  the session, and the portal dispatcher derives its per-desktop configuration file name
  from it. Setting it to anything else would make a display manager treat a Zenith session
  as one of its own.
- **The file lives in `/usr/share/wayland-sessions/`, not `/usr/share/xsessions/`.** The
  directory is what decides the session type: the login manager sets
  `XDG_SESSION_TYPE=wayland` for a file found there, and `x11` for one found in the other.

`/etc/zenith/session/zenith.desktop` overrides the packaged base, and
`~/.config/zenith/session/` — through the usual overlay — overrides both. That is where a
machine-specific `Exec` line goes.

### What `zenith-session` does

`/usr/share/zenith/session/zenith-session` is a small shell script, because it runs before
anything else exists and it must be readable when the disk is having a bad day.

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
exec /usr/bin/Hyprland -c /usr/share/zenith/hyprland/hyprland.conf
```

The compositor is started with Zenith's own configuration, because the keybinds live there
and nowhere else. It is the only place the keyboard is described: a bind names the shell's
IPC target and the shell decides what the key means, so the compositor never has to know
what a launcher is. A compositor started without that file runs on Hyprland's own defaults,
which is a desktop with no keybindings at all.

The configuration sources the user's own file, which the session creates — empty, in the
user's configuration directory — before the compositor reads it. Hyprland treats a source
that resolves to nothing as a configuration error and refuses to start, so the file has to
exist; the user's half is theirs, and it is sourced after the packaged one, so it can
override anything and add anything.

The `exec` at the end is what makes a failure safe: anything that goes wrong before it — a
bad `/etc/zenith/session/zenith-session.env`, an unreadable schema, a disk-full — exits with
the compositor never started, and the display manager puts the user back on the login
screen. A broken session that exits before the compositor is a safe failure mode by
construction. Every step above is also written so that it cannot exit for a trivial reason,
which is why the overlay files are guarded with `if` rather than with `&&`.

Two details in the script are load-bearing beyond the obvious:

- **`XDG_CURRENT_DESKTOP=Zenith` and `XDG_SESSION_DESKTOP=zenith` are different strings,
  and both are deliberate.** The dispatcher lower-cases the current desktop to derive its
  per-desktop configuration file name, so `Zenith` finds `zenith-portals.conf`. The session
  desktop is the lower-case name the login manager records in `loginctl show-session`.
- **The environment is published twice, to two different consumers.** `systemctl --user
  import-environment` reaches the per-user manager, so every unit started from the manager
  inherits it. `dbus-update-activation-environment --systemd` reaches the bus's activation
  environment, so a service activated on demand inherits it. A backend that is D-Bus
  activated rather than started as a unit sees nothing without the second one.

### How this differs from a stock desktop session

| | A stock desktop session | The Zenith session |
|---|---|---|
| Session file | `/usr/share/wayland-sessions/<desktop>.desktop` | The same mechanism, `zenith.desktop` |
| `Exec=` | A session manager that owns a private list of what a session is | A shell script that sets the environment and hands over to the compositor |
| Root of the session | The session manager's own idea of a session | `hyprland-session.target` and `graphical-session.target`; there is no second session manager |
| Services | Started by the session manager from a private list | systemd user units with `PartOf=graphical-session.target` |
| Environment | The session manager's exports plus a desktop-specific `environment.d` | One environment, published to the user manager and to bus activation |
| The display | The session manager talks to the compositor | The compositor is the session's anchor, and Zenith's units attach to its target |
| The display name | The stock desktop's own name | `Zenith`, which is what every library branches on |

---

## What starts, in order

| Order | Unit | Started by | What it is |
|---|---|---|---|
| 0 | `zenith-session` | The display manager | Environment, bus activation, `exec Hyprland` |
| 1 | `hyprland-session.target` | The compositor | The anchor for everything that needs a live compositor |
| 2 | `wireplumber.service` | The session target | Audio policy, so the shell's volume widget has something to talk to |
| 3 | `zenith-portal.service` | The drop-in | The portal dispatcher, which activates the backends on demand |
| 4 | `zenith-shell.service` | The drop-in, ordered after the portals | The shell |
| 5 | The shell's children | The shell | Lock screen, quick settings, notification server — inside the shell, not separate units |

### Why the compositor comes before the portal dispatcher

The Hyprland portal backend connects to a Wayland compositor at startup and exits if it
cannot:

```cpp
m_sWaylandConnection.display = wl_display_connect(nullptr);
if (!m_sWaylandConnection.display) {
    Debug::log(CRIT, "Couldn't connect to a wayland compositor");
    exit(1);
}
```

Its unit also carries `ConditionEnvironment=WAYLAND_DISPLAY`, so the backend is not even
started unless the environment carries that variable. Both facts point the same way: the
portal stack has to come after the compositor, and the compositor's environment has to be
in place before the backend is activated.

Three more dependencies hide in the same backend:

- **Screenshots need `grim` on `PATH`.** Without it the backend does not create its
  Screenshot portal at all, and a screenshot key binding produces nothing.
- **Region screenshots need `slurp`,** and colour picking needs `hyprpicker`.
- **Screen sharing needs pipewire.** The backend opens a pipewire context; without it the
  screencopy portal is never created.

So the image carries `grim`, `slurp`, `hyprpicker` and pipewire, and the order is compositor
first.

### Why the portal dispatcher comes before the shell

This is the ordering that is easiest to get wrong, because both sides of it work
individually and the failure is quiet.

**The dispatcher resolves its backends once, at startup, and never re-reads them.** It
reads `XDG_CURRENT_DESKTOP` and the `portals.conf` chain, decides which backend implements
which interface, and only then takes its bus name. That decision holds for the life of the
process: a backend that appears later is not noticed, and a configuration file that changes
later is not re-read. Anything that changes the dispatcher's environment after it has
started has no effect on it.

**The dispatcher's default resolution would take notifications away from the shell.** The
base layer's GTK backend implements `org.freedesktop.impl.portal.Notification`, and the
Hyprland backend does not. With no explicit configuration, the dispatcher resolves the
Notification interface to the GTK backend, and a portal notification is rendered by a GTK
window instead of by the shell's own surface. The notification server Zenith does not ship
is replaced by one Zenith did not ask for, and nothing in the interface says so.

**The shell's first interaction with the portal races the dispatcher's activation.** The
shell draws a launcher with an "open file" action, a screen-share entry point and a print
action. Each is a call to `org.freedesktop.portal.Desktop` on the session bus. If the
dispatcher is not up when the user first clicks, the call goes through a D-Bus activation,
and the activation is a separate event with its own latency. On a cold session the first
such call pays for it, and if the activation cannot run at all the call fails with
`org.freedesktop.DBus.Error.ServiceUnknown`. The user sees an empty dialog and a message
that the file chooser could not be opened. The shell is the first thing the user touches, so
it must not be the thing that triggers an activation.

Ordering the dispatcher first removes both failures: the backend set is resolved from an
environment that already names Zenith, and the dispatcher is already on the bus when the
user first clicks something.

### The portal configuration

The dispatcher reads its backend preference from a per-desktop file whose name is derived
from `XDG_CURRENT_DESKTOP`, lower-cased. For a Zenith session that is `zenith-portals.conf`.
The packaged base lives at `/usr/share/zenith/portal/zenith-portals.conf` and is
materialized to the registration path, so the overlay rules apply to it like everything
else.

```ini
# /usr/share/zenith/portal/zenith-portals.conf
# materialized to /usr/share/xdg-desktop-portal/zenith-portals.conf
[preferred]
default=hyprland;gtk
org.freedesktop.impl.portal.ScreenCast=hyprland
org.freedesktop.impl.portal.Screenshot=hyprland
org.freedesktop.impl.portal.GlobalShortcuts=hyprland
org.freedesktop.impl.portal.InputCapture=hyprland
```

Reading it:

- **`[preferred]` is the group the dispatcher reads.** The older `[default] use=…` form is
  not what the current dispatcher parses, and a file in that form is ignored rather than
  half-honoured. A file with no `[preferred]` group is silently skipped, and the session
  falls back to whatever the base layer ships.
- **`default=hyprland;gtk` is the fallback list**, searched in order for every interface.
  `hyprland` is the source name of the Hyprland backend's `.portal` file and `gtk` is the
  source name of the GTK backend's; a backend's source name is its file name without the
  `.portal` suffix.
- **The explicit lines override the default** for those interfaces, so the compositor
  interfaces cannot be taken by another backend that happens to sort first.
- **The GTK backend is the fallback for everything it implements**: FileChooser, AppChooser,
  Print, Notification, Inhibit, Access, Account, Email, DynamicLauncher, Lockdown and
  Settings. `OpenURI` is not implemented by it directly, but the dispatcher's OpenURI only
  needs an AppChooser backend, which the GTK backend is, so it resolves in practice.
- **`Secret` is not in the file because no backend implements it.** See
  [the Secret gap](#the-secret-gap).

`zenith doctor` must assert that this file is the one being read. A session that does not
declare itself correctly falls back to the base layer's `portals.conf` and silently uses the
wrong backends, which is the failure mode that is hardest to notice and the easiest to
cause.

---

## What the shell owns

Everything a stock desktop shell used to provide is now the shell's job or the compositor's
job. The notification server, the global shortcuts and the lock screen are the shell's
outright, because no other component in the image provides them. Idle is the exception: it
is split three ways, and confusing the split is how a laptop ends up suspending with the
screen unlocked.

### The notification server

There is no notification daemon in the image, so nothing else can own the notification
surface. The shell draws it and hosts the server.

Two bus names reach the shell, and they are not the same name:

- **`org.freedesktop.Notifications`** is the freedesktop notification specification name.
  It is what `libnotify` clients call, which is to say what every ordinary GTK and Qt
  application calls. The shell owns this name, and if the shell is not up, nothing is
  notified and there is no fallback. The probe is one call:

  ```bash
  gdbus call --session --dest org.freedesktop.Notifications \
    --object-path /org/freedesktop/Notifications \
    --method org.freedesktop.Notifications.Notify \
    "portal probe" 0 "" "" [] {} 3000
  #   (uint32 1,)      ← a banner must appear. A returned id with no banner means
  #                     the shell is up but is not the server.
  ```

- **`org.freedesktop.portal.Notification`** is the portal path, and the dispatcher routes
  it to a backend that implements the corresponding implementation interface. The GTK
  backend implements it, so by default a portal client gets a GTK window rather than the
  shell's surface. Routing that path to the shell needs a Zenith-owned portal backend, which
  is a decision the portal seam has not made yet.

The shell also owns the *ordering* of notifications, their grouping, their urgency styling
and their history, because those are surface decisions and the surface is the shell's.

### Global shortcuts

A global shortcut has to be caught by the compositor, in the input path, before anything
else sees the key. The split is:

- **Compositor-level keys** — the workspace switcher, the display layout — are the
  compositor's own bindings, and they belong to it.

  The **window switcher is the exception**, and it is not a preference. Hyprland has no
  window switcher to bind to: there is no `tab` bind in any shipped default config across
  the last three releases, no compositor-internal Tab handling, and no switcher in the
  official plugins repository. Its built-in cycling walks *layout* order scoped to the
  active workspace, which is neither most-recently-used nor cross-workspace. Correct MRU
  cycling exists in Hyprland's source as `CWindowQuery::cycleHistory` and has zero call
  sites. So this key belongs to the shell, with Hyprland only catching it.
- **Everything else** — the launcher, the terminal, quick settings, the lock key — is the
  shell's, bound through the compositor's own configuration and verified with
  `hyprctl binds`.

The *portal* interface is a different thing and is not the shell's. The Hyprland backend
implements `org.freedesktop.impl.portal.GlobalShortcuts`, and the portal configuration above
routes it there. The shell's bindings are faster and more reliable than the portal route,
which is a request over D-Bus answered by a client that may not be running, and whose
binding is not installed until somebody answers.

### The lock screen

The shell hosts the lock surface and speaks PAM itself — `pam_start`, `pam_authenticate`,
`pam_acquire_authtok` — so that unlocking is a real authentication rather than a prompt
with a password field bolted on. The lock surface is drawn with the session-lock protocol
the compositor implements, so the lock is enforced by the compositor rather than by a
window that merely looks in front of the others. The verified path is:

```bash
systemctl --user lock-session
```

That command reaches logind, and logind does not call a lock interface on the shell's
behalf: it emits the `org.freedesktop.login1.Session.Lock` signal on the system bus, and the
shell's lock surface answers it. The shell therefore also listens for that signal directly.

The shell implements `org.freedesktop.ScreenSaver` on the session bus as well, with `Lock`,
`Unlock`, `GetActive`, `ActiveChanged`, `Inhibit` and `UnInhibit`. The `Inhibit` and
`UnInhibit` methods are what the base layer's portal Inhibit backend calls, so an
application that asks the portal to hold the screensaver while a video plays holds Zenith's
lock as well.

The shell reports its state back with `org.freedesktop.login1.Session.SetLockedHint`, so
`loginctl show-session` tells the truth.

### Idle

Idle is the one place where the compositor, the shell and logind each own a different part,
and confusing them is how a laptop ends up suspending with the screen unlocked.

- **Detection is the compositor's.** The compositor knows when the user last did something;
  no Wayland client can find that out for itself.
- **The shell draws the idle screen** and locks at `org.zenith.desktop lock-delay`.
- **The actions go through logind**, so the standard inhibitors apply. Locking is
  `loginctl lock-session`; suspending is `systemctl suspend`.

Idle is also the thing that makes the suspend path safe: the shell holds a delay inhibitor
across the sleep request, so a machine that suspends while unlocked is a machine that was
asked to suspend with the screen already locked, not one that raced.

---

## Logout, suspend and shutdown

### Logout

1. The user asks to log out. The shell asks the user manager to stop the session:
   `systemctl --user stop hyprland-session.target`.
2. Stopping the session target stops `graphical-session.target`, because the session target
   is bound to it.
3. Every session service stops, because each one carries `PartOf=graphical-session.target`.
4. The compositor exits, the session leader exits with it, and the display manager returns
   to the greeter.

A second session on the same machine is independent: it has its own user manager instance,
its own session target, and its own compositor. The virtual terminal switch is the display
manager's.

### Suspend and resume

1. Something asks for the suspend: the user closes the lid, presses the power key, or the
   shell's own idle policy reaches its threshold.
2. The request goes to logind, which takes a delay inhibitor and emits
   `PrepareForSleep(true)`.
3. The shell locks itself, answering the `true` half of the signal while it holds a delay
   inhibitor, so the machine does not sleep with an unlocked screen.
4. logind releases the inhibitor, `systemd-suspend.service` runs, and the machine sleeps.
5. On resume, `PrepareForSleep(false)` arrives and the shell unlocks.

### Shutdown and reboot

The greeter's power buttons call the display manager's own halt and reboot commands, which
are `systemctl poweroff` and `systemctl reboot`. They go through logind, which runs the
shutdown inhibitors, and then the system stops. The session is torn down by the same path as
a logout, because the session's units stop when the manager stops.

Nothing in the display manager's configuration has to change for this, which is the reason
it is worth checking: a change there would mean writing outside the two directories Zenith
owns.

### Who owns what

| Concern | Owner | Mechanism |
|---|---|---|
| Screen lock | The shell | `org.freedesktop.ScreenSaver`, the session lock protocol, PAM |
| Lock on suspend | The shell | `PrepareForSleep` on the system bus |
| Idle detection | The compositor | Its own idle notifications, surfaced to the shell |
| Idle policy | The shell, reading `org.zenith.desktop` | The timeouts live in GSettings, and `zenith-shell` and `zenith-hub` are their only writers |
| Inhibitors | logind | Delay inhibitors are held by the shell and by logind itself |
| Suspend, poweroff, reboot | logind | `systemctl`, the display manager's halt and reboot commands |
| Session teardown | systemd | `graphical-session.target` and `PartOf=` |

---

## Failure modes and recovery

### The compositor starts but the shell does not

**What the user sees.** A black desktop with a working cursor and a working keyboard. No
bar, no dock, no launcher, no popouts. The window switcher does nothing. Notifications are
not answered, so they are lost rather than shown. The screen never locks, because the lock
surface is the shell's.

**Why.** `zenith-shell.service` failed, or was never reached. In order of likelihood: the
shell's QML failed to load and the process exited; a GSettings schema did not compile, so
the shell aborted while reading it; or the compositor's environment never reached the user
manager, so the unit's `ConditionEnvironment=WAYLAND_DISPLAY` was never satisfied.

**What to run.**

```bash
systemctl --user status zenith-shell.service
journalctl --user -b -u zenith-shell.service
systemctl --user show-environment | grep WAYLAND_DISPLAY
hyprctl clients
```

The second command is the one that matters: the shell's own error is in its journal, not in
the display manager's. To see it in the foreground:

```bash
systemctl --user stop zenith-shell.service
quickshell -p /usr/share/zenith/shell
```

**Recovery.** `systemctl --user restart zenith-shell.service` restarts it in place. If the
user's overlay is what broke, move it aside and restart:

```bash
mv ~/.config/zenith/shell/shell.qml ~/.config/zenith/shell/shell.qml.bak
systemctl --user restart zenith-shell.service
```

`zenith doctor` reports drift between the base and the overlay.

### Portals are missing, so file choosers fail

**What the user sees.** An application that opens a file chooser shows nothing, or an empty
window, or "Could not open the file chooser". A "share screen" button does nothing at all.
A print action produces no dialog.

**Why.** Either the dispatcher is not running, or it is running and has resolved an
interface to a backend that does not implement it. The second case is the more common one
and the more confusing, because the dispatcher is healthy and every request returns a
well-formed "no implementation" error.

**What to run.**

```bash
systemctl --user status zenith-portal.service
journalctl --user -b -u zenith-portal.service
busctl --user list | grep -i portal
```

To see which backend the dispatcher resolved for each interface, turn on its debug output
and restart it:

```bash
systemctl --user set-property zenith-portal.service Environment=G_MESSAGES_DEBUG=all
systemctl --user restart zenith-portal.service
journalctl --user -b -u zenith-portal.service | grep 'Preferred portals'
```

To see who implements what on the image, and to check that Zenith's own preference file is
the one in play:

```bash
grep -H '^Interfaces=' /usr/share/xdg-desktop-portal/portals/*.portal
ls -l /usr/share/xdg-desktop-portal/zenith-portals.conf
```

To check the file chooser itself:

```bash
zenity --file-selection --title "portal probe"
```

**Recovery.** `systemctl --user restart zenith-portal.service` is enough for a dispatcher
that is running with the wrong configuration, because the configuration is re-read at
startup. A backend that is missing entirely is a packaging problem:
`xdg-desktop-portal-gtk` for FileChooser and Print, `xdg-desktop-portal-hyprland` for
ScreenCast and Screenshot.

### The secret gap

**What the user sees.** An application that asks for a password to store gets no error and
no store. A Flatpak that uses the Secret interface — Firefox, Thunderbird, Evolution, VS
Code — has nowhere to keep a password. The request appears to succeed and the password is
gone.

**Why.** No backend in the stack implements `org.freedesktop.portal.Secret`. The dispatcher
looks for a portal backend that declares the interface, finds none, and returns success
without exporting the portal at all. `gnome-keyring` does not substitute: it exposes
`org.freedesktop.Secret.Service` on the session bus, and the dispatcher never asks it.

**What to run.**

```bash
busctl --user introspect org.freedesktop.portal.Desktop /org/freedesktop/portal/desktop
gdbus call --session --dest org.freedesktop.portal.Desktop \
  --object-path /org/freedesktop/portal/desktop \
  --method org.freedesktop.DBus.Properties.Get \
  org.freedesktop.portal.Desktop version
```

The absence of `org.freedesktop.portal.Secret` in the dispatcher's object tree is the
finding.

**Recovery.** There is none yet, and the document should not pretend otherwise. The
architecture records three ways out and none is chosen: ship the GNOME backend for Secret
alone, adopt a Secret backend derived from another implementation, or write a small
Zenith-owned backend over `gnome-keyring`'s service interface. The last one is the most
control and the most work. Until one lands, the contract that an application which works on
a stock desktop works on Zenith has the Secret interface as its known exception, and the
image's own `libpam-gnome-keyring` module — which unlocks the keyring at login so a prompt
does not ask for a second password — is the half of the problem that is already solved.

### The session hangs at login

**What the user sees.** The password is accepted and then nothing happens. The screen sits
on the greeter's background, or goes black. Sometimes the greeter comes back after a
minute, which is the session leader exiting and the display manager falling back to the
greeter.

**Why, in order of likelihood.**

1. The compositor never started, so `zenith-session` never reached its `exec`, and the
   display manager returned the user to the login screen. This is the safe failure, and it
   is safe by construction.
2. The user bus is not up, so `zenith-session` cannot publish the environment and the
   session never starts.
3. The session file is missing or its `Exec=` line points at a file that is not there.
4. The shell failed to start after a successful compositor start. The desktop is empty, but
   the session is alive.

**What to run.** From a virtual terminal — Ctrl+Alt+F3 reaches a TTY that depends on none
of this — or from another session:

```bash
journalctl -b -u sddm.service
journalctl -b _UID=1000
loginctl session-status
cat ~/.local/state/zenith/session.log
```

The last one is Zenith's own session log, written by `zenith-session` before it execs
anything. It is the first thing to read, because it records the session's environment
hand-off and the moment the compositor was started.

To see the session units from outside the session, as the user, with the right runtime
directory:

```bash
runuser -u <user> -- env XDG_RUNTIME_DIR=/run/user/1000 systemctl --user status zenith-shell.service
runuser -u <user> -- env XDG_RUNTIME_DIR=/run/user/1000 journalctl --user -b -u zenith-portal.service
loginctl user-status <user>
```

To check the session registration itself:

```bash
ls -l /usr/share/wayland-sessions/zenith.desktop
grep -E '^(Exec|TryExec|DesktopNames)=' /usr/share/wayland-sessions/zenith.desktop
```

**Recovery.** Most hangs at login are a stale session rather than a broken image. Killing
the session's leader and starting a new one from the greeter is enough:

```bash
loginctl terminate-session <session-id>
```

If the session cannot be reached at all, the same commands work from the virtual terminal.

**The black screen.** A hang that reaches the compositor but not the shell leaves the user
at a black screen with no way to find out why. Two things narrow that window: the shell's
unit carries `Restart=on-failure` with a one second delay, so a crash in the shell's QML is
a one second gap rather than a session's end; and the shell's own journal is the only place
the error is, which is why the failure mode has to name the command.

---

## Open questions

| Question | Why it matters |
|---|---|
| ~~The login screen's display server~~ | **Settled** — SDDM in Wayland greeter mode with a Weston kiosk compositor hosted by Zenith. The package manifest keeps the Xorg metapackage excluded. See [architecture.md](architecture.md#what-draws-the-login-screen). |
| The Secret portal | No backend in the stack implements `org.freedesktop.portal.Secret`, and `gnome-keyring` does not substitute for one. Password storage is broken in Flatpaks until one of the three options is chosen. See [the secret gap](#the-secret-gap). |
| The shell as a portal backend | The shell owns `org.freedesktop.Notifications`, so every ordinary application is served. The portal's own Notification interface still resolves to the GTK backend, because the shell is not a registered portal backend. Whether it becomes one decides whether a portal client sees the shell's surface or a GTK window. |
| The `zenith-session` script's scope | It is the session entry point today. Whether it also owns the pre-flight checks — the portal preference file, the PAM stack, the keyring — is not decided. The failure modes above are all "something was wrong and nobody was told", and that is the argument for it. |
| Idle reporting to logind | Logind's own idle actions only fire if a session reports its idle state, and on Wayland nothing does by default. Whether the shell reports the hint or drives the timers itself decides whether `IdleAction` in logind's configuration is a setting or a dead letter. |
