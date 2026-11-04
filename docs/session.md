# The session path

ASURA from power-on to a desktop, and back down again. This document fixes the order of
everything on that path and names the owner of each step. A desktop with two owners for one
job is a desktop that breaks in ways nobody can reproduce, so every stage below has exactly
one owner and the reason for every ordering is written next to it.

Three rules run through the whole document:

- **One owner per stage.** Where a base-layer component owns a step, ASURA does not
  duplicate it. Where no base-layer component owns it, ASURA does.
- **Order is a dependency, not a preference.** Where two things must happen in a particular
  sequence, the failure that the sequence avoids is named.
- **Everything is a file.** Every unit, config file and desktop entry named here is a file
  on the image, with a path and an owner.

The companion documents are [`architecture.md`](architecture.md) for the decisions and
[`adopting-asura.md`](adopting-asura.md) for putting an existing machine on this path.
Where this document repeats a decision, it repeats the reason with it.

---

## Where ASURA's files live

ASURA's packages write to `/usr/share/asura/` and `/etc/asura/`, and to nothing else that
belongs to another package. A handful of files have to sit outside those two directories
anyway, because the components that read them look nowhere else. Listing them is the point:
a boundary that is not written down gets crossed by accident.

| Path | What it is | Why it cannot live under `/usr/share/asura/` |
|---|---|---|
| `/usr/share/wayland-sessions/asura.desktop` | Session registration | The login manager scans the fixed `wayland-sessions` directory and nowhere else |
| `/etc/sv/asura-sysd`, `/etc/sv/scx-lavd`, `/etc/sv/ananicy-cpp` and `~/.config/service/` user services | runit service registration | runit supervises services from `/etc/sv/` (system) and `~/.config/service/` (user) |
| `/usr/share/glib-2.0/schemas/org.asura.*.gschema.xml` | GSettings schema | GLib compiles schemas from the fixed schema directory |
| `/usr/share/xdg-desktop-portal/asura-portals.conf` | Portal backend preference | The dispatcher reads it from a fixed directory. On the sanctioned list in [`architecture.md`](architecture.md#the-sanctioned-list-outside-the-boundary) |
| `/usr/bin/asura`, `/usr/bin/asura-*` and the bare-name helpers | The executables, and the scripts the shell and the compositor call by name | `PATH` resolution looks nowhere else, and the user manager's `PATH` has no `~/.local/bin`. Same list |
| `/usr/share/applications/asura-*.desktop`, `asura-mimeapps.list` | Desktop entries, and the default applications for ASURA sessions | The desktop-entry and mime-apps lookups scan `XDG_DATA_DIRS/applications` and nowhere else. Same list |
| `/boot/grub/grub.cfg` | Boot configuration | The boot loader reads its configuration from the boot volume, which the installer writes via `grub-mkconfig` |

These are registrations — pointers a third-party binary must find — not ASURA-owned content.
The content stays at `/usr/share/asura/session/`, `/usr/share/asura/shell/`,
`/usr/share/asura/portal/` and `/usr/share/asura/schemas/`, and `asura doctor` reports
drift between the two the same way it reports drift everywhere else.

**Runtime state is not a registration.** The state daemon's socket is at
`$XDG_RUNTIME_DIR/asura-shell.sock`, and the wallpaper daemon's and the agent's sit beside
it, and none of them is on the list because it is not a file a package installs. Each daemon
creates its socket at run time, in the per-user runtime directory, which no ASURA package
writes to, and it is removed when the session ends. A path that exists only while the session
does cannot be a registration, and a registration is by definition a path that exists on
every boot.

And the list of things ASURA deliberately does **not** touch:

- `/etc/sddm.conf` and `/etc/sddm.conf.d/` — the login manager's configuration. No package
  writes it; the opt-in `10-asura.conf` drop-in is installed by the user, with the command
  `asura greeter enable` prints.
- `/etc/pam.d/` — the PAM stack. The keyring is unlocked by the base layer's own PAM module.
- `/etc/dracut.conf` — the initramfs configuration. ASURA's copy lives in
  `/etc/asura/initramfs/`.
- `/usr/share/xdg-desktop-portal/portals/` — the dispatcher's backend directory.
- `/usr/lib/udev/rules.d/`, `/etc/NetworkManager/`, and every other package's configuration.

---

## Boot chain

The shipped boot chain is **GRUB EFI**. The installer (`installation/asura-installer.sh`)
writes `/boot/grub/grub.cfg` via `grub-mkconfig`, and the ISO boots through GRUB on both
BIOS and UEFI firmware. Secure Boot rides the Void-shipped GRUB shim.

### Stages and owners

| Stage | Owner | What it owns | ASURA's part |
|---|---|---|---|
| Firmware | The machine | Secure Boot verification, the EFI variable store, the boot order | Nothing |
| Secure Boot shim | Base layer (Void-shipped GRUB shim) | First-stage verification, the MOK manager, the chainloader slot | Nothing |
| GRUB | Base layer | The boot menu, loading the kernel and the initramfs, the boot parameters | The installer writes `/boot/grub/grub.cfg` |
| Kernel | Base layer, pinned by `system/kernel/` | Drivers, the initramfs unpacker, early microcode | The fleet, the initramfs contents |
| initramfs | Base layer's dracut integration, tuned by ASURA | Root discovery, the microcode blob, zram | The config directory, the module list |
| runit | Base layer | The system manager and the user-level service supervisor | The service files (`/etc/sv/`, `~/.config/service/`) |

### The boot volume

The ESP is a FAT32 volume mounted at `/boot`, carrying the kernel, the initramfs and
`grub.cfg`.

### Microcode has to load early

The CPU runs the wrong microcode until something loads the right one, and a machine that
needs an update for stability is unstable during exactly the window that matters. The base
layer's microcode packages include the early microcode blob in the initramfs, so the kernel
applies it before unpacking the real initramfs.

### The initramfs

The base layer's dracut integration builds the initramfs. ASURA customises it through
`/etc/asura/initramfs/`.

zram is configured by `zramen` in `/etc/sv/zramen/conf`.

---

## The runit service graph

The system side first, then the user side.

### The system instance

runit (system services in `/etc/sv/`) starts the base layer. ASURA adds：

- `asura-sysd` (`system/runit/services/asura-sysd/run`) — root daemon at `/run/asura/sysd.sock`
- `scx-lavd` (`system/runit/services/scx-lavd/run`) — sched_ext LAVD scheduler
- `ananicy-cpp` (`system/runit/services/ananicy-cpp/run`) — gaming/desktop priority rules
- `asura-boot-guard` (`system/runit/services/asura-boot-guard/run`) — boot health marker

runit services are enabled by creating a symlink in `/etc/runit/runsvdir/default/`.

### The user instance

The session is not a system service. It is a set of user-level runit services supervised by
`asura-user-services` (`session/runit/asura-user-services`, installed at
`~/.config/service/asura-user-services` via the user runit supervisor):

- `asura-userd` (`system/runit/user-services/asura-userd/run`)
- `asura-agent` (`system/runit/user-services/asura-agent/run`) — opt-in, off until the user
  switches it on in Settings
- `asurad` (`system/runit/user-services/asurad/run`) — shell state daemon
- `asura-wallpaper` (`system/runit/user-services/asura-wallpaper/run`) — wallpaper daemon
- `quickshell` (`system/runit/user-services/quickshell/run`) — the shell
- `portal` (`system/runit/user-services/portal/run`) — xdg-desktop-portal dispatcher
- `portal-hyprland` (`system/runit/user-services/portal-hyprland/run`) — Hyprland backend
- `asura-idle` (`system/runit/user-services/asura-idle/run`) — hypridle with the idle timeouts
- `pipewire` (`system/runit/user-services/pipewire/run`) — audio
- `wireplumber` (`system/runit/user-services/wireplumber/run`) — audio policy

So the session owns its anchor, and starts it once the compositor is up:

### The services ASURA ships

The runit service scripts (`system/runit/user-services/*/run`) are the units. Whichever
daemon each one runs is fixed by its `run` script. The shell's QML is read by
`quickshell -p /usr/share/asura/shell` from the `quickshell` runit service.

The daemon listens on a Unix socket at `$XDG_RUNTIME_DIR/asura-shell.sock` and speaks
`asura-shell/1` over it — line-framed, one JSON document per line. The socket lives in the
per-user runtime directory rather than under `/usr/share/asura/`, because it is created at
run time by a process, and a path a process creates is not a file a package installs.

The base layer's portal dispatcher and ASURA's cannot both run. Two dispatchers on one bus
is one dispatcher that wins the name and one that loses it, and the loser exits — the name
is taken from the bus, not from the service, so the second instance terminates with "dbus
name was lost" and takes the session's portal with it. Only one portal service directory
should be supervised.

---

## The login manager and the session file

The login manager is the base layer's, and ASURA uses it as shipped.

### What it does

1. Starts the greeter on the first virtual terminal.
2. Authenticates the user through PAM, in the `sddm-session` service.
3. Starts the session file the user selected, as that user, with the session's environment.

The greeter is a **Wayland** surface. The base layer's login manager has no Wayland greeter of
its own, so ASURA supplies one: `DisplayServer=wayland` in a drop-in under `/etc/sddm.conf.d/`,
with `CompositorCommand` pointing at `asura-greeter`, which runs `weston --shell=kiosk` to hold
the DRM card and hosts ASURA's own login surface on it. The package manifest therefore excludes
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
# /usr/share/asura/session/asura.desktop
# materialized to /usr/share/wayland-sessions/asura.desktop
[Desktop Entry]
Type=Application
Name=ASURA
Comment=ASURA desktop
TryExec=/usr/share/asura/session/asura-session
Exec=/usr/share/asura/session/asura-session
DesktopNames=ASURA
Keywords=Hyprland;Quickshell;
```

Three lines carry the design:

- **`Exec=/usr/share/asura/session/asura-session`.** The session entry point is ASURA's
  own script, and it runs from the packaged base rather than from a copy in the user's home.
- **`DesktopNames=ASURA`**, not `GNOME`. The login manager uses this to group and label
  the session, and the portal dispatcher derives its per-desktop configuration file name
  from it. Setting it to anything else would make a display manager treat an ASURA session
  as one of its own.
- **The file lives in `/usr/share/wayland-sessions/`, not `/usr/share/xsessions/`.** The
  directory is what decides the session type: the login manager sets
  `XDG_SESSION_TYPE=wayland` for a file found there, and `x11` for one found in the other.

`/etc/asura/session/asura.desktop` overrides the packaged base, and
`~/.config/asura/session/` — through the usual overlay — overrides both. That is where a
machine-specific `Exec` line goes.

### What `asura-session` does

`/usr/share/asura/session/asura-session` is a small shell script, because it runs before
anything else exists and it must be readable when the disk is having a bad day.

```sh
#!/bin/sh
# /usr/share/asura/session/asura-session
set -eu

LOG="${XDG_STATE_HOME:-$HOME/.local/state}/asura/session.log"
mkdir -p "$(dirname "$LOG")"
exec >>"$LOG" 2>&1

echo "--- asura-session $(date -Is) ${XDG_SESSION_ID:-no-session-id}"

export XDG_SESSION_DESKTOP=asura
export XDG_CURRENT_DESKTOP=ASURA
export DESKTOP_SESSION=asura
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
if [ -r /etc/asura/session/asura-session.env ]; then
    . /etc/asura/session/asura-session.env
fi
if [ -r "${XDG_CONFIG_HOME:-$HOME/.config}/asura/session/asura-session.env" ]; then
    . "${XDG_CONFIG_HOME:-$HOME/.config}/asura/session/asura-session.env"
fi

# Make dbus-activated user services and anything the compositor spawns see the
# desktop identity, or apps probe for GNOME and get confused.
export XDG_SESSION_DESKTOP XDG_CURRENT_DESKTOP DESKTOP_SESSION XDG_SESSION_TYPE WAYLAND_DISPLAY
dbus-update-activation-environment XDG_SESSION_DESKTOP XDG_CURRENT_DESKTOP \
    DESKTOP_SESSION XDG_SESSION_TYPE WAYLAND_DISPLAY

echo "asura-session: env ok, starting Hyprland"
exec /usr/bin/Hyprland -c /usr/share/asura/hyprland/hyprland.conf
```

The compositor is started with ASURA's own configuration, because the keybinds live there
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
bad `/etc/asura/session/asura-session.env`, an unreadable schema, a disk-full — exits with
the compositor never started, and the display manager puts the user back on the login
screen. A broken session that exits before the compositor is a safe failure mode by
construction. Every step above is also written so that it cannot exit for a trivial reason,
which is why the overlay files are guarded with `if` rather than with `&&`.

Two details in the script are load-bearing beyond the obvious:

- **`XDG_CURRENT_DESKTOP=Asura` and `XDG_SESSION_DESKTOP=asura` are different strings,
  and both are deliberate.** The dispatcher lower-cases the current desktop to derive its
  per-desktop configuration file name, so `Asura` finds `asura-portals.conf`. The session
  desktop is the lower-case name the login manager records in `loginctl show-session`.
### How this differs from a stock desktop session

| | A stock desktop session | The ASURA session |
|---|---|---|
| Session file | `/usr/share/wayland-sessions/<desktop>.desktop` | The same mechanism, `asura.desktop` |
| `Exec=` | A session manager that owns a private list of what a session is | A shell script that sets the environment and hands over to the compositor |
| Services | Started by the session manager from a private list | runit user services with the user-level supervisor |
| The display | The session manager talks to the compositor | The compositor is the session's anchor |
| The display name | The stock desktop's own name | `ASURA`, which is what every library branches on |

---

## What starts, in order

| Order | Service | Started by | What it is |
|---|---|---|---|
| 0 | `asura-session` | The display manager | Environment, bus activation, `exec Hyprland` |
| 1 | `wireplumber` | runit user supervisor | Audio policy, so the shell's volume widget has something to talk to |
| 2 | `portal` | runit user supervisor | The portal dispatcher, which activates the backends on demand |
| 3 | `asurad` | runit user supervisor | The shell daemon, which the shell subscribes to over its socket |
| 4 | `asura-wallpaper`, `asura-idle` | runit user supervisor | The wallpaper daemon, and hypridle with the idle timeouts |
| 5 | `quickshell` | runit user supervisor | The shell |
| 6 | The shell's children | The shell | Quick settings, notification server — inside the shell, not separate services |

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

The backend runs only once the compositor's `WAYLAND_DISPLAY` is in place. Both facts point
the same way: the portal stack has to come after the compositor, and the compositor's
environment has to be in place before the backend is activated.

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
window instead of by the shell's own surface. The notification server ASURA does not ship
is replaced by one ASURA did not ask for, and nothing in the interface says so.

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
environment that already names ASURA, and the dispatcher is already on the bus when the
user first clicks something.

### The portal configuration

The dispatcher reads its backend preference from a per-desktop file whose name is derived
from `XDG_CURRENT_DESKTOP`, lower-cased. For an ASURA session that is `asura-portals.conf`.
The packaged base lives at `/usr/share/asura/portal/asura-portals.conf` and is
materialized to the registration path, so the overlay rules apply to it like everything
else.

```ini
# /usr/share/asura/portal/asura-portals.conf
# materialized to /usr/share/xdg-desktop-portal/asura-portals.conf
[preferred]
default=hyprland;gtk
org.freedesktop.impl.portal.ScreenCast=hyprland
org.freedesktop.impl.portal.Screenshot=hyprland
org.freedesktop.impl.portal.GlobalShortcuts=hyprland
org.freedesktop.impl.portal.InputCapture=hyprland
org.freedesktop.impl.portal.Secret=lxqt
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
- **`Secret` names `lxqt`**, which is not in the default list: no other backend in the stack
  implements the interface, and `gnome-keyring` does not substitute, because the dispatcher
  never asks the session bus's Secret service. See
  [the portal seam](architecture.md#the-portal-seam) and [the Secret gap](#the-secret-gap).

`asura doctor` must assert that this file is the one being read. A session that does not
declare itself correctly falls back to the base layer's `portals.conf` and silently uses the
wrong backends, which is the failure mode that is hardest to notice and the easiest to
cause.

---

## What the shell owns

Everything a stock desktop shell used to provide is now the shell's job or the compositor's
job. The notification server and the global shortcuts are the shell's outright, because no
other component in the image provides them. The lock screen is not one of them: `hyprlock`
is in the base layer's archive and does the locking, and `hypridle` owns the decision to lock;
the shell only draws the dim scrim. Idle is the exception: it is split several ways, and
confusing the split is how a laptop ends up suspending with the screen unlocked.

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
  shell's surface. Routing that path to the shell needs an ASURA-owned portal backend, which
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
  sites. So this key belongs to the shell, with Hyprland only catching it: it is Alt+Tab,
  while Super+Tab opens the workspace overview.
- **Everything else** — the launcher, the terminal, quick settings, the lock key — is the
  shell's, bound through the compositor's own configuration and verified with
  `hyprctl binds`.

The *portal* interface is a different thing and is not the shell's. The Hyprland backend
implements `org.freedesktop.impl.portal.GlobalShortcuts`, and the portal configuration above
routes it there. The shell's bindings are faster and more reliable than the portal route,
which is a request over D-Bus answered by a client that may not be running, and whose
binding is not installed until somebody answers.

### The keybinding catalogue

The keybinds are written down twice: once in the compositor's configuration, where Hyprland
reads them, and once in `session/binds.json`, where a person can read them. The second list is
the catalogue, and it is the one the shell draws.

Each entry says what the key is, what it is for, which chord presses it, and what kind of
thing it does — a call into the shell's own IPC surface, a program, or one of the compositor's
own actions:

```json
{
  "id": "launcher",
  "category": "desktop",
  "label": "Open the launcher",
  "chord": "SUPER + Space",
  "kind": "shell",
  "action": "togglePopover",
  "argument": "launcher"
}
```

The chord is spelled one way throughout: the modifiers in the order `SUPER`, `CTRL`, `ALT`,
`SHIFT`, separated from the key by a plus, and the key in the spelling the keyboard itself
uses — `Space`, `Return`, `Escape`, `comma`, `XF86AudioRaiseVolume`. A family of ten keys is
written with a placeholder, `SUPER + {n}`, and the placeholder takes the number of the
position rather than the key, so the 0 key is the tenth workspace and not a workspace called
zero.

The compositor's configuration is not generated from the catalogue. Hyprland reads a plain
configuration file and understands no JSON, so the file has to stay a file, with its own
comma-separated syntax. What the catalogue calls `SUPER + SHIFT + 1` the configuration writes
as `SUPER SHIFT, 1`, and the difference is a separator and nothing else: the compositor's
constraint rather than a style choice. The two are kept in step by a check rather than by a
convention anybody has to remember.

`asura doctor` compares them in both directions. Every chord in the catalogue has to be bound,
every bind has to be in the catalogue, and every `call asura <function>` in the configuration
has to name a function the shell's `IpcHandler` actually defines. A keybind that names a
function the shell has not got runs, succeeds, and does nothing, and that is the quietest
failure in the desktop.

`asura binds` prints the catalogue as JSON. It is what the shell's cheat sheet reads — Super+K
draws every chord, grouped by category, with each key as a cap — and it is the same list the
doctor checks the configuration against.

### The lock screen

**The shell does not draw the lock screen, and it does not speak PAM.** `hyprlock` does the
locking, and that is a decision rather than a limit of the platform: Quickshell as built here
links libpam and has `Quickshell.Services.Pam`, so a shell-drawn lock could authenticate.
`hyprlock` is mature, it is in the base layer's archive, and it runs as a process of its own,
apart from the shell. An earlier version of this section said the shell hosts the lock surface
and authenticates itself, and a later one said there is no QML PAM API and that a shell-drawn
lock would accept any password; neither was right.

So the division is the one recorded in
[`architecture.md`](architecture.md#the-shell): `hypridle` decides **when** —
`asura-idle` runs it with the timeouts of `org.asura.desktop`'s `idle-*` keys — and
`asura-shell lock` is the one command every trigger runs (the bind, hypridle's lock stage,
`loginctl lock-session`, before sleep). That command runs `hyprlock`, and the shell only draws
the dim scrim (`Idle.qml`). The base layer carries hyprlock, so under the base-layer rule
ASURA does not repackage it. Every competing locker is excluded, so the image has exactly one
and which one is in front never depends on start order. [`lock.md`](lock.md) has the details.

The `org.freedesktop.ScreenSaver` interface on the session bus is no longer the shell's. Its
`Inhibit` and `UnInhibit` methods are what the base layer's portal Inhibit backend calls,
so an application that asks the portal to hold the screensaver while a video plays holds
ASURA's lock as well — and the interface belongs to the locker, because the locker is the
thing that can authenticate. What the design fixes is only that the shell is not the thing
answering it.

Before a suspend, hypridle runs `asura-shell lock` again and sets `inhibit_sleep = 3`, which
delays the sleep until the session reports it is locked, so the machine does not suspend with
the desktop showing. If hyprlock never starts, logind suspends after its own delay anyway.

### Idle

Idle is the one place where the compositor, hypridle, the shell and logind each own a
different part, and confusing them is how a laptop ends up suspending with the screen
unlocked.

- **Detection is the compositor's, and hypridle is the one idle clock.** The compositor knows
  when the user last did something; no Wayland client can find that out for itself, so
  hypridle asks it through the compositor's idle-notify protocol. The `asura-idle` runit
  service runs hypridle with the three timeouts of `org.asura.desktop` (`idle-dim-seconds`,
  `idle-blank-seconds`, `idle-lock-seconds`; 0 means never). hypridle also honours D-Bus and
  elogind inhibitors, so a video or Keep Awake holds every stage.
- **The shell draws the dim scrim** (`Idle.qml`) when hypridle says so, and locks nothing
  itself. Screen-off is `hyprctl dispatch dpms off`, and there is no suspend stage.
- **The actions go through logind**, so the standard inhibitors apply. Locking is
  `asura-shell lock`, which `loginctl lock-session` reaches too.

Idle is also the thing that makes the suspend path safe: `asura-shell`, the daemon, holds
delay inhibitors across the sleep request, so a machine that suspends while unlocked is a
machine that was asked to suspend with the screen already locked, not one that raced.

---

## Logout, suspend and shutdown

### Logout

1. The user asks to log out. The compositor exits, the session leader exits with it, and
   the display manager returns to the greeter.

### Suspend and resume

1. Something asks for the suspend: the user closes the lid or presses the power key. There
   is no idle suspend stage: `org.asura.desktop` has no key for one.
2. The request goes to logind, which takes a delay inhibitor and emits
   `PrepareForSleep(true)`.
3. `asura-shell` answers the `true` half of the signal by running the lock while it holds
   its own delay inhibitors, so the machine does not sleep with an unlocked screen. The
   locker does the locking; the daemon decides that it should happen.
4. logind releases the inhibitor and the machine sleeps.
5. On resume, `PrepareForSleep(false)` arrives and the locker releases the session.

### Shutdown and reboot

The greeter's power buttons call the display manager's own halt and reboot commands, which
go through logind, which runs the shutdown inhibitors, and then the system stops.

Nothing in the display manager's configuration has to change for this, which is the reason
it is worth checking: a change there would mean writing outside the two directories ASURA
owns.

### Who owns what

| Concern | Owner | Mechanism |
|---|---|---|
| Screen lock | `hyprlock`, from the base layer's archive | The locker authenticates against PAM. `hypridle` decides *when*, and the shell draws only the dim scrim |
| Lock on suspend | The shell daemon (`asura-shell`) | `PrepareForSleep` on the system bus, and the delay inhibitors it holds across the request |
| Idle detection | The compositor, through hypridle | Its own idle notifications, read by hypridle |
| Idle policy | `org.asura.desktop`, handed to hypridle by `asura-idle` | The timeouts live in GSettings, and `asura config set` is their only writer |
| Inhibitors | logind | Delay inhibitors are held by the shell daemon and by logind itself |
| Live desktop state | `asurad` | The `asura-shell/1` socket at `$XDG_RUNTIME_DIR/asura-shell.sock` |
| Suspend, poweroff, reboot | logind | the display manager's halt and reboot commands |

---

## Failure modes and recovery

### The compositor starts but the shell does not

**What the user sees.** A black desktop with a working cursor and a working keyboard. No
bar, no dock, no launcher, no popouts. The window switcher does nothing. Notifications are
not answered, so they are lost rather than shown. The dim scrim never appears, because the
shell draws it; hypridle still locks and blanks the screen on schedule.

**Why.** The `quickshell` runit service failed, or was never reached. In order of
likelihood: the shell's QML failed to load and the process exited; or a GSettings schema
did not compile, so the shell aborted while reading it.

**What to run.**

```bash
sv status ~/.config/service/quickshell
cat ~/.local/state/asura/quickshell.log
hyprctl clients
```

To see it in the foreground:

```bash
quickshell -p /usr/share/asura/shell
```

**Recovery.** `sv restart ~/.config/service/quickshell` restarts it in place. If the
user's overlay is what broke, move it aside and restart:

```bash
mv ~/.config/asura/shell/shell.qml ~/.config/asura/shell/shell.qml.bak
sv restart ~/.config/service/quickshell
```

`asura doctor` reports drift between the base and the overlay.

### The shell is up but the daemon is not

**What the user sees.** The desktop is there — bar, dock, launcher, popouts — and the
things that read live state are wrong. The status area shows the last volume it heard about.
The sidebar's sections do not update, or they show the last thing they saw and never change.
The volume slider moves the sink and the number does not follow.

**Why.** The `asurad` runit service failed, or was never reached. The shell starts
regardless: it can draw a surface with nothing to subscribe to, and it does, because a
shell that refuses to start because a state daemon is down is a session with no desktop in
it at all. The daemon is the thing that fails quietly.

**What to run.**

```bash
sv status ~/.config/service/asurad
cat ~/.local/state/asura/asurad.log
ls -l "$XDG_RUNTIME_DIR/asura-shell.sock"
```

The socket is the daemon's address, and it is in the per-user runtime directory rather than
under `/usr/share/asura/` — it is created at run time and it is not a file a package
installs. A missing socket with a healthy unit is a daemon that has not bound it yet; a
missing socket with a failed unit is the failure itself.

**Recovery.** `sv restart ~/.config/service/asurad`. If the daemon was down when the
shell started, restart the shell as well, `sv restart ~/.config/service/quickshell`: in
this Quickshell build a first `Socket` connect that fails fires no signal, so the services
that connected once never retry. If the shell is showing nothing at all rather than something
stale, the daemon is not the fault and the
[shell failure mode](#the-compositor-starts-but-the-shell-does-not) is.

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
sv status ~/.config/service/portal
busctl --user list | grep -i portal
```

To see who implements what on the image, and to check that ASURA's own preference file is
the one in play:

```bash
grep -H '^Interfaces=' /usr/share/xdg-desktop-portal/portals/*.portal
ls -l /usr/share/xdg-desktop-portal/asura-portals.conf
```

To check the file chooser itself:

```bash
zenity --file-selection --title "portal probe"
```

**Recovery.** `sv restart ~/.config/service/portal` is enough for a dispatcher
that is running with the wrong configuration, because the configuration is re-read at
startup. A backend that is missing entirely is a packaging problem:
`xdg-desktop-portal-gtk` for FileChooser and Print, `xdg-desktop-portal-hyprland` for
ScreenCast and Screenshot.

### The secret gap

The gap is closed: `xdg-desktop-portal-lxqt` implements `org.freedesktop.portal.Secret` and
`asura-portals.conf` names it. This section is the failure mode for when it reopens.

**What the user sees.** An application that asks for a password to store gets no error and
no store. A Flatpak that uses the Secret interface — Firefox, Thunderbird, Evolution, VS
Code — has nowhere to keep a password. The request appears to succeed and the password is
gone.

**Why.** The dispatcher looks for a portal backend that declares the interface. With
`xdg-desktop-portal-lxqt` not installed, or `asura-portals.conf` not the file being read, it
finds none and returns success without exporting the portal at all. `gnome-keyring` does not
substitute: it exposes `org.freedesktop.Secret.Service` on the session bus, and the
dispatcher never asks it.

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

**Recovery.** Install `xdg-desktop-portal-lxqt` (`sudo xbps-install -S xdg-desktop-portal-lxqt`)
and make sure `asura-portals.conf` is the file in use (`asura doctor` reads each named
backend's `.portal` file and reports an interface pointed at a backend that cannot serve
it), then `sv restart ~/.config/service/portal`. The image's own `gnome-keyring` PAM
module, which unlocks the keyring at login so a prompt does not ask for a second password,
is the other half of the problem and is already solved.

### The session hangs at login

**What the user sees.** The password is accepted and then nothing happens. The screen sits
on the greeter's background, or goes black. Sometimes the greeter comes back after a
minute, which is the session leader exiting and the display manager falling back to the
greeter.

**Why, in order of likelihood.**

1. The compositor never started, so `asura-session` never reached its `exec`, and the
   display manager returned the user to the login screen. This is the safe failure, and it
   is safe by construction.
2. The user bus is not up, so `asura-session` cannot publish the environment and the
   session never starts.
3. The session file is missing or its `Exec=` line points at a file that is not there.
4. The shell failed to start after a successful compositor start. The desktop is empty, but
   the session is alive.

**What to run.** From a virtual terminal — Ctrl+Alt+F3 reaches a TTY that depends on none
of this — or from another session:

```bash
journalctl -b -u sddm.service
cat ~/.local/state/asura/session.log
```

The last one is ASURA's own session log, written by `asura-session` before it execs
anything. It is the first thing to read, because it records the session's environment
hand-off and the moment the compositor was started.

To see the session services from outside the session, as the user, with the right runtime
directory:

```bash
sudo -u <user> env XDG_RUNTIME_DIR=/run/user/1000 sv status ~/.config/service/quickshell
sudo -u <user> env XDG_RUNTIME_DIR=/run/user/1000 sv status ~/.config/service/portal
```

To check the session registration itself:

```bash
ls -l /usr/share/wayland-sessions/asura.desktop
grep -E '^(Exec|TryExec|DesktopNames)=' /usr/share/wayland-sessions/asura.desktop
```

**Recovery.** Most hangs at login are a stale session rather than a broken image. Killing
the session's leader and starting a new one from the greeter is enough:

**The black screen.** A hang that reaches the compositor but not the shell leaves the user
at a black screen with no way to find out why. The runit supervisor restarts the shell,
so a crash in the shell's QML is a short gap rather than a session's end; and the shell's
own log is the only place the error is, which is why the failure mode has to name the
command.

---

## Open questions

| Question | Why it matters |
|---|---|
| ~~The login screen's display server~~ | **Settled** — SDDM in Wayland greeter mode with a Weston kiosk compositor hosted by ASURA. The package manifest keeps the Xorg metapackage excluded. See [architecture.md](architecture.md#what-draws-the-login-screen). |
| ~~The Secret portal~~ | **Settled** — `xdg-desktop-portal-lxqt`, named for Secret in `asura-portals.conf`; see [the portal seam](architecture.md#the-portal-seam). [The secret gap](#the-secret-gap) is the failure mode if it reopens. |
| The shell as a portal backend | The shell owns `org.freedesktop.Notifications`, so every ordinary application is served. The portal's own Notification interface still resolves to the GTK backend, because the shell is not a registered portal backend. Whether it becomes one decides whether a portal client sees the shell's surface or a GTK window. |
| The `asura-session` script's scope | It is the session entry point today. Whether it also owns the pre-flight checks — the portal preference file, the PAM stack, the keyring — is not decided. The failure modes above are all "something was wrong and nobody was told", and that is the argument for it. |
| ~~Idle reporting to logind~~ | **Settled** — hypridle drives the timers from the `org.asura.desktop` idle keys (see [lock.md](lock.md)), so nothing relies on logind's own `IdleAction`. |
