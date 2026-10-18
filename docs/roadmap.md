# Roadmap

Milestones in order. Each one is bootable or testable on its own — a milestone that runs
on a developer machine is finished — and none of them depends on work that has not landed.
The order is deliberate: hardware reach comes early because it decides whether Zenith is
usable by anyone but its author.

---

## M1 — Foundations

*Status: complete in `main`.*

Repository scaffold, licensing, the design system, and the architecture record.
Documentation only, no working code. The point of doing this first is that every later
decision has somewhere to be written down.

- [x] Repo layout, `.gitignore`, and the placeholder trees — `docs/journal/`, `installation/`,
  `release/`, `tests/`
- [x] `LICENSE` (GPL-3.0), `LICENSING.md` covering the three-way split
- [x] `AGENTS.md` + `agents/agents.md` — the agent contract and design system
- [x] `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`, the `.github/` issue forms and
  pull request template, and the markdown gate
- [x] `docs/architecture.md` — decisions, rationale, rejected options
- [x] `docs/adopting-zenith.md`, `docs/hardware-detection.md` and `docs/session.md` — the
  adoption path, the detection subsystem and the session chain, all decided before any of them
  is written as code
- [x] `system/hardware/` — support tiers, the profile schema and an annotated example profile
- [x] `system/kernel/fleet.yaml` and `system/packages.yaml` — the pinned kernel fleet and the
  reviewed package manifest, GNOME exclusions included

## M2 — The `zenith` control tool

*Status: the overlay model, the update sequence and all six commands are written and tested. The
exit criterion is met against a temporary tree, not against a stock base-layer image, and
`zenith update`'s fetch step has never run against a real APT repository.*

The first working code. Go, because the tools must be static binaries that start instantly.

- [x] `zenith update` / `rollback` / `materialize` / `reset` / `doctor` — the command surface
  `docs/architecture.md` defines
- [x] `zenith status` — current revision, resolved profile and overlay state at a glance. New in
  this milestone; the command table in `docs/architecture.md` has no row for it yet
- [x] The overlay model: base in `/usr/share/zenith/`, user overlay in `~/.config/zenith/user_edits/`
- [x] A git snapshot of `~/.config` before every update
- [ ] Verified against a stock base-layer image and a real APT repository, which is what the
  exit criterion actually asks for

The invariant the whole model rests on is that a user's override wins at its mirrored path and
no update can clobber it. That means the user's content is *written* to
`~/.config/zenith/<path>`, not left to be shadowed from `user_edits/` — applications read the
mirrored path and know nothing of the overlay directory, so a model that skipped writing it
would resolve the edit in the one place nothing reads.

Exit criterion: on a stock image of the base layer, `zenith update` lays down a base and an
overlay, a user edit survives the next update, and `zenith rollback` restores the previous
revision. **The last two hold and are tested; the first has not been run against a real image.**


## M3 — Hardware detection

*Status: the detection, matching, redaction, report and profile-lint libraries are written and
tested, and `zenith hardware report|match|lint-profiles|capture` reaches all of them. This has
been exercised only against captured fixture trees, never against a real laptop.*

The subsystem that decides whether Zenith works on someone else's laptop.

- [x] Detection: DMI, PCI, USB, ACPI, EDID, device tree, network chips, GPU topology
- [x] Every reader takes a filesystem root, enforced by an AST test, so a captured machine is a
  fixture and the subsystem is testable without a hardware lab
- [x] Profile matching with a defined tie-break when several match, and a reported conflict
  when the tie-break cannot decide
- [x] Report redaction, two layers: the reader allowlist and a sweep that refuses to emit
- [x] `zenith hardware report` — text, JSON and the candidate profile shape, swept before it is
  emitted; `--raw` refuses under CI
- [x] `zenith hardware report --profile` — render a candidate profile, schema-shaped and
  token-free, with no kernel patch pin and no firmware file
- [x] `zenith hardware match` — report tier and the profile that matched, plus the candidates
  that matched and lost. Also new in this milestone; `docs/architecture.md` names only `report`
- [x] `zenith hardware lint-profiles` — every rule in "what a profile must not contain",
  plus the database-wide overlap check that catches the profile pair which becomes a
  conflict once a third arrives
- [x] `zenith hardware capture` — write a machine's sysfs tree as a fixture, filtered through an
  allowlist of what the readers consume, so a committed fixture cannot carry a serial

The whole milestone is now reachable from one binary, and `probe → report → lint` closes: a
profile generated from a captured machine passes the linter that checks it.

The device tree is in the list because DMI alone caps the whole architecture. An ARM laptop
answers in a device tree and not in SMBIOS, so a probe set without it could never claim one and
every ARM machine would be tier 3 permanently — a ceiling on support rather than a limitation of
it. Its identity is projected onto the same match-key fields rather than a second vocabulary, so
a profile is written once and works whichever architecture answers.

Exit criterion: detection correctly identifies GPU, WiFi, audio codec and storage on a real
x86 laptop *and* a real ARM laptop, and produces a candidate profile that a human can read and
confirm.

## M4 — Bootable ISO

The first thing anyone can install.

- `live-build` for the ISO, `mmdebstrap` for the rootfs seed
- Hardware detection in the installer, profile selection, tier display
- Limine on a separate ESP, Secure Boot via shim + MOK
- The offline firmware path: a broad rescue bundle plus `zenith-firmware load`

Exit criterion: a Zenith ISO installs on a clean machine with no network, and the installed
image boots into the base layer's stock Hyprland session entry. Zenith's own session — SDDM,
`zenith-session`, the portal stack — is M5's, and the shell is M6's.

## M5 — Session and portals

*Status: the session is now wired end to end in source and packaged — entry
point, both systemd units, the session-target drop-in, the SDDM drop-in and all
six registrations, with `zenith doctor` asserting that every registration is
installed and that every unit points at a binary that exists. It has still never
been run under a login manager, a compositor, or a display, so the milestone's
own verification criterion is not met.*

Before any shell work, because it validates the base-layer bet.

- [x] `zenith-session` — a `/bin/sh` entry point that sets the session
  environment, imports it into dbus activation, and `exec`s Hyprland
- [x] `zenith.desktop` — the session registration, with `DesktopNames=Zenith` so
  the portal dispatcher derives the right configuration file name
- [x] `zenith-portals.conf` — the backend preference, with `GlobalShortcuts`
  deliberately absent and `Secret` present but stated as not working
- [x] Portal resolution, and the `zenith doctor` assertion that the
  configuration is actually being read
- [x] `zenith-shell.service` and `zenith-portal.service` — the two user units,
  the second ordered after `hyprland-session.target` and the first after the
  second, which is the order that is easiest in the session to get wrong and
  fails quietly when it is
- [x] The session-target drop-in
- [ ] The SDDM drop-in — held, not shipped. Its entire content is
  `DisplayServer=wayland` with `CompositorCommand=zenith-greeter`, and SDDM has no
  way to start a Wayland greeter that does not exist, so installing it before the
  greeter lands does not degrade the login screen, it removes it. The
  registration is on the sanctioned list and `zenith doctor` reports it, so the
  gap is visible rather than forgotten. It ships with the greeter, which is now
  an M6 item.
- [x] All six registrations packaged, every one on the sanctioned list in
  [architecture.md](architecture.md), each a pointer to content under
  `/usr/share/zenith/`
- [x] `zenith doctor` asserts every registration is installed and every unit's
  `ExecStart` names an existing binary — the packaging faults that are silent
  in both the package build and the login
- [x] The screenshot toolchain in the manifest: `grim`, `slurp` and `hyprpicker`.
  The portal backend shells out to them, so without them it does not create the
  Screenshot portal at all and a screenshot keybinding produces nothing
- [ ] Verified under screen sharing, screenshot, camera access, the GTK file
  chooser and a secret round-trip — none of which can be done without a display

The units are installed and correct. What they run is not there yet:
`/usr/bin/quickshell -p /usr/share/zenith/shell` has no configuration directory
to read, and `/usr/libexec/xdg-desktop-portal` is the base layer's, which is
correct — the dispatcher is a shadow of it, not a replacement. `zenith doctor`
reports the missing shell configuration, which is the honest state of the tree.
M6 owns the shell.

**The portal assertion is the point of this milestone.** `xdg-desktop-portal`
derives its per-desktop configuration file name from `XDG_CURRENT_DESKTOP`,
lower-cased. A session that does not declare itself correctly finds no
`zenith-portals.conf`, falls back to the system `portals.conf`, and quietly uses
whatever that names — on a machine adopted from GNOME, a dispatcher activating a
backend that is not installed. Nothing is printed and nothing fails. The symptom
is a file chooser that is wrong, reported as "Zenith's file chooser is broken"
rather than as a session identity problem, and `zenith doctor` now names it.

Two details in that resolution are pinned by tests because both look like
non-issues and neither is:

- **`XDG_CURRENT_DESKTOP=Zenith` and `XDG_SESSION_DESKTOP=zenith` are different
  strings.** The dispatcher lower-cases the first to derive the file name, so the
  capitalisation is load-bearing. A session that swaps them still resolves a
  filename — just the wrong one.
- **Lower-casing means `zenith` finds the same file `Zenith` does.** The
  configuration is read either way, and the identity is still wrong. The report
  says so rather than passing on the strength of the file being found.

The Secret limitation is reported as a **notice, not a problem**, and `zenith
doctor` exits 0 on a healthy machine despite it. A gap the user has no lever on
is not a misconfiguration, and a doctor that fails for one teaches users to
ignore its exit code.


## M6 — The shell

*Status: the shell's source exists — the schema, the entry point, the bar, the
dock, the launcher and the widget catalogue — and is packaged. It has never been
run, because Quickshell is built from source and is not in the archive, so there
is no quickshell here to run and no qmllint to check the QML with. What is
checked instead is everything checkable without an engine, and that check found
four real faults while it was being written.*

- `zenith-greeter`, which runs `weston --shell=kiosk` and hosts Zenith's own login
  surface. It had no owner until the SDDM drop-in made the dependency explicit:
  the drop-in points at it, so the login screen cannot ship without it. M6 owns it
  because it is a Quickshell surface like everything else on the desktop.
- [x] `org.zenith.shell` and `org.zenith.desktop`, the schemas the shell reads.
  The keys adoption imports into — `wallpaper-path`, `wallpaper-path-dark`,
  `favorites`, `dock-position` — keep the names and types
  [adopting-zenith.md](adopting-zenith.md) already uses
- [x] The bar, the dock, the launcher and the widget catalogue, driven by
  `org.zenith.shell`
- [x] `zenith-shell` as the only writer of those schemas
- [x] `zenith doctor` checks the shell's source: the widget catalogue naming a
  file that is not there, a surface reading a property the settings wrapper does
  not declare, a `schemaId` with no schema file, a path outside the boundary, an
  import outside the closed module list
- [ ] Popovers, and the surfaces a stock desktop used to own: notification
  server, lock, idle screen, quick settings, status icons

## M7 — Zenith Hub

- `zenith-hub`, the settings application: a Go binary like `zenith` and `zenith-shell`, window
  drawn in Quickshell (QML) — one UI toolkit and one design system on the desktop, not two
- Sole writer of `org.zenith.desktop`
- Every setting that has a command also has a page, and vice versa

## M8 — Repository and release

*Status: the control-tool package builds reproducibly and the repository signs and validates
against the keyring it ships. Nothing has been published, and `zenith update`'s apt step has
still never run against this repository.*

- [x] The `zenith` package, built reproducibly: `SOURCE_DATE_EPOCH` pinned, `-trimpath`, mtimes
  and modes normalized. Two builds are byte-identical, which is checked rather than asserted
- [x] Signed APT repository with `Release.gpg`, `InRelease`, and a keyring shipped alongside it,
  referenced by `Signed-By` in `sources.list.d`
- [x] `release/check-boundary.sh`, which refuses a package that installs a path outside
  `/usr/share/zenith/` and `/etc/zenith/` other than the sanctioned ones — checked on the
  staged tree, so it sees what `dpkg-deb` will emit rather than what the script appears to copy
- [ ] The other packages the stack names — `zenith-terminal`, `zenith-shell`, `zenith-hub`,
  and the session and portal packages — none of which has anything to package yet
- [ ] Published, and `zenith update` verified against a machine that installed from it

The boundary check exists because a package that quietly gains a file another package owns is
a fork of that package, and that cost arrives at the next rebase as a conflict nobody remembers
the origin of. A reviewer reading a build script cannot see what `dpkg-deb` will emit, so the
check runs on the staged tree. Its own test asserts that a planted violation is refused —
a guard that cannot fail is not a guard.


## M9 — Adoption

*Status: the preflight is written and tested — all eleven checks, the three exit codes, and the
never-write promises enforced as data rather than prose. `zenith adopt` itself does not exist,
and the preflight has never been run against a machine that was about to be switched.*

Switching an existing GNOME desktop on the base layer over to the Zenith shell, in place,
without reinstalling and without touching the user's data. Designed in
[`adopting-zenith.md`](adopting-zenith.md).

This is also the project's growth path: it is how someone decides whether they want Zenith
before they commit to installing it. That is worth more than a landing page.

- [x] `zenith adopt --check` — preflight. Reports what would change and changes nothing, needs
  no root, and is safe to run as often as the user likes
- [x] Eleven checks, each with its own fail condition: base release, desktop session, display
  manager, GNOME version, GPU and driver, composite support, portal availability, disk space,
  APT health, third-party extensions, boot chain
- [x] Blockers separated from report-only warnings, with three distinct exit codes so the same
  command is usable from a script
- [x] `--json`, which is how `zenith adopt` re-runs the preflight as its own step zero rather
  than reimplementing any of it
- [x] The never-write promises encoded as data, so a test can assert against them instead of a
  comment being trusted
- [ ] `zenith adopt` — install the session so it appears at the login screen
- [ ] `zenith adopt --verify`, `--prune`, `--undo`

Two decisions in the preflight are worth recording because they are counterintuitive.

**The GPU check never blocks.** Refusing adoption for a GPU this project does not recognise
would put a ceiling on reach at exactly the hardware the project exists to support, and adoption
installs no driver. So an unusual GPU is a warning naming it, and a blocker only appears when
the *portal backends cannot be installed* — the one future-facing check that genuinely blocks,
because a desktop whose file choosers and screen sharing silently do not work is not a desktop.

**A dpkg lock file is a warning, not a blocker.** A lock file existing is normal; apt creates
one per operation. What cannot be established without `fuser` is *who* holds it, so the honest
answer is a warning that says so. A half-configured package from `dpkg --audit` is different,
and is a blocker — running apt in that state is unsafe.


## M10 — Agent layer

The resident OS agent: a local knowledge vault, an index over the machine, and a dashboard.
Distro-independent by construction.

---

## Later, unsequenced

- Plugin mechanism and a package index for the shell
- `zenith benchmark` so performance regressions are visible rather than anecdotal
- A second base release, once hardware profiles justify it