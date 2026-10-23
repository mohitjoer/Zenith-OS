# Roadmap

Milestones in order. Each one is bootable or testable on its own — a milestone that runs
on a developer machine is finished — and none of them depends on work that has not landed.
The order is deliberate: hardware reach comes early because it decides whether Zenith is
usable by anyone but its author.

The ISO (M4), the Limine/UKI boot chain and Btrfs rollback are deferred: the product today is the Zenith session on a stock Ubuntu 26.04 install, laid down by the `zenith` package (`zenith adopt --check` is the preflight), and `zenith update` rolls back through the config git snapshot.

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

*Status: complete. The exit criterion is met — verified against a stock image of the base
layer (`ubuntu:26.04`) and a real signed APT repository, by `release/update_test.sh`, which
the `m2` job of the CI workflow runs.*

The verification earned its place by finding a fault in the release tooling that every
other check had passed: a repository that published only the compressed index, so apt —
which looks an index up by its uncompressed name — fetched the meta index, verified the
signature, committed it, and then took no packages from it. No warning, no error, exit
zero. A signed repository that verifies perfectly and delivers nothing at all, which is
the kind of fault no signature check can catch and no amount of reading the Release file
would have shown.

The first working code. Go, because the tools must be static binaries that start instantly.

- [x] `zenith update` / `rollback` / `materialize` / `reset` / `doctor` — the command surface
  `docs/architecture.md` defines
- [x] `zenith status` — current revision, resolved profile and overlay state at a glance. New in
  this milestone; the command table in `docs/architecture.md` has no row for it yet
- [x] The overlay model: base in `/usr/share/zenith/`, user overlay in `~/.config/zenith/user_edits/`
- [x] A git snapshot of `~/.config` before every update
- [x] Verified against a stock base-layer image and a real APT repository, which is what the
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
tested, and `zenith hardware report|match|lint-profiles|capture` reaches all of them. The
report has run on a real laptop for the first time, and it refused to emit: two DMI
attributes that real machines carry and the captured fixture trees do not were
unclassified, so the sweep could not tell the report's own `board_vendor` from an
unread `chassis_vendor` and refused the report as a leak. That is the class of fault a
fixture tree cannot hold, and it is fixed. The machine's candidate profile now comes out
of the report, with the two things detection cannot decide — the kernel flavour and the
display mux — left for a human.*

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
  deliberately absent and `Secret` served by `xdg-desktop-portal-lxqt`
- [x] The Secret portal, which was the worst gap in the project: every Flatpak
  that stores a password had nowhere to store one. `zenith doctor` reads each
  named backend's `.portal` file, so an interface pointed at a backend that
  cannot serve it is reported rather than sitting there looking handled
- [x] The Camera portal, which looked like a missing backend and was not. No
  backend implements `org.freedesktop.portal.Camera` — but the interface is
  implemented by `xdg-desktop-portal` itself, and it asks an **Access** backend
  for consent before opening a device. Access was un-named, so it fell to
  `[default] use=hyprland`, and Hyprland does not implement it. Naming
  `Access` → `gtk` is the whole fix
- [x] Portal resolution, and the `zenith doctor` assertion that the
  configuration is actually being read
- [x] `zenith-shell.service` and `zenith-portal.service` — the two user units,
  the second ordered after `zenith-session.target` and the first after the
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

*Status: the shell's source exists — the schema, the entry point, the widget
catalogue, the notification server and the settings bridge — and it is packaged.
The surfaces it draws are a vendored upstream Quickshell configuration, pinned to
a commit and kept byte-identical to it, and re-themed to Zenith's design system
rather than forked. It has still never been run, because Quickshell is built from
source and is not in the archive, so there is no quickshell here to run and no
qmllint to check the QML with. What is checked instead is everything checkable
without an engine, and that check found four real faults while it was being
written. The vendored tree is exempt from it by declaration, and that exemption
has a price: the bridges into the tree are a contract that nothing checks.*

- `zenith-greeter`, which runs `weston --shell=kiosk` and hosts Zenith's own login
  surface. It had no owner until the SDDM drop-in made the dependency explicit:
  the drop-in points at it, so the login screen cannot ship without it. M6 owns it
  because it is a Quickshell surface like everything else on the desktop.
- [x] `org.zenith.shell` and `org.zenith.desktop`, the schemas the shell reads.
  The keys adoption imports into — `wallpaper-path`, `wallpaper-path-dark`,
  `favorites`, `dock-position` — keep the names and types
  [adopting-zenith.md](adopting-zenith.md) already uses
- [x] The bar, the dock, the launcher and the popover surfaces, taken from a
  vendored upstream component at a pinned commit and re-themed, rather than
  written from nothing. Since the Ryoku port the running shell draws Ryoku's
  components for these, adapted in place (see [`NOTICE.md`](../NOTICE.md))
- [x] The widget catalogue, Zenith's own, driven by `org.zenith.shell`
- [x] The vendored tree itself, held byte-identical to its upstream commit.
  `sh shell/vendor/vendor.sh verify` hashes every vendored file against the
  manifest and fails on a file that was edited by hand, which is the only check
  that can catch an edit nobody intended
- [x] The monochrome re-theme, carried into the vendored components by three
  overlay files at upstream's own paths — the palette bridge, the settings bridge
  and the shell's own state — with no vendored file modified
- [x] `zenith config set` as the only writer of those schemas; the shell's look in
  `shell.json` has its own, the `zenith-shell` daemon
- [x] `zenith doctor` checks the shell's source: the widget catalogue naming a
  file that is not there, a surface reading a property the settings wrapper does
  not declare, a `schemaId` with no schema file, a path outside the boundary, an
  import outside the closed module list
- [x] Popovers — one at a time, tracked by a single `currentPopover` rather than
  three booleans, so opening the third cannot leave two orphaned surfaces with
  no keybinding that closes them
- [x] Quick settings and the status area
- [x] The icon font, settled from the archive rather than vendored: `fonts-font-awesome` (4.7, no dependencies) for the status set and `fonts-powerline` for separators. Every glyph is declared in `Icons.qml` as a `\uXXXX` escape and checked to be inside its font's real range — which caught that the first draft used U+F8FF and U+F8C0, neither of which is a Font Awesome 4.7 glyph
- [ ] The vendored components' own icon names. Upstream names its icons the
  Material Symbols way and asks for a font the base layer does not carry, and
  Font Awesome 4.7 is a codepoint font with no ligatures — so those names resolve
  to nothing until they are mapped onto the codepoints the archive's font has.
  The map is the missing piece, and it belongs in the bridge
- [x] `zenith-greeter` — the kiosk Weston wrapper, which is the part that can be
  tested without a display, plus twelve checks that run it against stubbed weston
  and modetest
- [ ] The login surface itself, and the SDDM drop-in with it. Both stay
  uninstalled: a greeter that cannot authenticate is worse than no greeter,
  because it is a machine that takes a password and does nothing with it. What is
  left is the sddm-greeter authentication protocol
- [x] The notification server. `org.freedesktop.Notifications` is a
  session-scoped bus name and no daemon in the image holds it, so with the
  shell not running it every application that sends a notification gets an error
  back. Actions are advertised because the popup renders them; body markup is
  not, and the body is `Text.PlainText` to match; persistence is not advertised,
  because there is no notification centre yet and a client that believes it need
  not show its own reminder is a client with no reminder and no history
- [x] Idle policy in `org.zenith.desktop` — dim, blank and lock timeouts, each
  independent, 0 meaning never — run by hypridle through `zenith-idle.service`;
  `Idle.qml` draws the dim scrim
- [x] The lock itself, by `hyprlock` rather than by the shell. That is a decision,
  not a platform limit: Quickshell here links libpam and has `Quickshell.Services.Pam`.
  `hyprlock` is mature, runs as a process of its own, and is in the archive
  (resolute-universe, depends on `libpam0g`), so rule 2 settles it; every
  competing locker is now excluded, so the image has exactly one
- [x] The idle screen, as a scrim. It answers the question rather than deferring
  it: the gap between idle and lock is exactly the window where someone walks
  past, so the shell dims what is on screen. It does not blank the display and
  does not claim to — power saving is the compositor's job, and a shell drawing
  opaque black would make the screen look off while the panel drew at full power
- [x] The wallpaper. It existed because adoption imports into
  `org.zenith.shell/wallpaper-path` and no GNOME shell exists to draw it; a
  setting an import writes and nothing draws is an import that succeeds and does
  nothing
- [x] The launcher, which until now had a `refresh()` that set its model to an
  empty list and showed nothing. Empty `launcher-apps` means every launchable
  application, which is the correct default rather than a fallback

## M7 — Zenith Hub

*Status: the settings catalogue and the page registry exist, and the invariant
between them is checked by `zenith doctor` rather than asserted. The Hub's window
is Ryoku's Hub, ported (a grouped rail, a global search and a page per area of the
desktop), drawn with the shell's own component kit, and the settings catalogue is its
**Zenith** group; `zenith-hubd` is its data plane for the compositor and the machine.*

- `zenith config` — `list`, `get`, `set`, `reset`, `pages`. Values are validated
  against the key's type, range and enum *before* the write, because a value that
  is well-formed GVariant and outside a declared range is accepted by GSettings,
  committed to dconf, and then read by the shell, which does something nobody
  asked for
- A typed catalogue of all 14 settings, read from the shipped `.gschema.xml`
  rather than from `gschemas.compiled` — the compiled file answers "what is this
  set to", and the tool needs "what is this, and what will it accept"
- The page registry, and the check that **every setting has a page and every page
  has a command**, in both directions. A key with no page is a setting only a
  command line can change; a page with no command is a control the user can see
  and use that either does nothing or writes dconf behind the tool's back
- [x] The Hub window, drawn in Quickshell (QML) — one UI toolkit and one design
  system on the desktop, not two. It draws pages it did not write: `zenith config
  export` hands it the navigation, the keys, their types, their permitted values,
  their ranges and their current values
- [x] The Hub's chrome built from the shell's component kit — its navigation rail, its content
  page, its grouped rows and its text — with the window's own code at the shell's
  root rather than in a configuration root of its own, because the kit's files reach
  each other through imports that only resolve from there. A change to the kit's look
  reaches the settings window and the desktop shell together, which is what one design
  system means in practice
- [x] Every control writes through `zenith config set`, never through GSettings,
  so every write to dconf on this desktop passes the same validation the tool
  does. A control writing dconf itself could put 999999 in a bar height that
  declares a maximum of 96
- [x] A third invariant: every setting type has a control, and every control is
  used by some setting. A type with no control is a page that renders the row and
  leaves it unchangeable — which reads as a broken Hub rather than a missing
  widget

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
- [ ] The other packages the stack names — `zenith-terminal` and the session and portal
  packages. Everything built so far ships in the one `zenith` package, and `zenith-shell`
  and `zenith-hub` are a binary and a launcher inside it, not packages of their own
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
- [x] The adoption state file, written atomically and versioned. `--undo`, `zenith doctor`
  and every failure path read it, and a truncated one read as empty is a state file that
  makes undo do nothing and report success
- [x] The adoption plan, derived from the machine rather than a fixed script. A registration
  already byte-identical to its source is not a step, because writing it again is a change
  that changes nothing and makes "what adoption did" a worse record
- [x] The undo script, generated from the same plan and written *before* the first write, so
  it exists even if adoption dies half way through. A plain file rather than a subcommand,
  because the moment it is needed there may be no working desktop to run one from
- [ ] `zenith adopt` — execute the plan. The plan, the state and the undo exist; running
  them as root does not
- [x] A GNOME audit in the manifest verifier, in three parts: nothing GNOME in the
  `packages:` list, the metapackages that would pull a desktop in *transitively* excluded,
  and no GNOME tool invoked anywhere in the tree. `gnome-keyring` is allowed and named
  with its reason; its transitive closure was verified by hand — 199 packages, no desktop
  component among them. `gsettings` is deliberately not treated as a GNOME tool: it is
  GLib's, it reads the same dconf the shell reads, and it is the interface GLib documents
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

## M11 — Theming, sidebar, switching, plugins, rollback, agent, installer

*Recorded 2026-10-19, from a comparison against the project the shell was inspired
by. This is a scope decision, not a feature list discovered by reading someone
else's codebase.*

**What is deliberately not being copied.** The project this shell was inspired by is an
Arch distribution whose desktop runs on Hyprland or niri, and whose base is rolling.
Zenith keeps its Ubuntu 26.04 base layer and its single Hyprland compositor, and the
reasons are already recorded: the base layer is what gives the project its hardware
reach, and a second compositor is a second session to keep tested rather than a
feature. Those two are settled and are not revisited here.

**What is not taken from the vendored upstream.** Vendoring a component is not taking
all of it. The upstream is a complete desktop configuration, and most of that
configuration is not Zenith's desktop: its Material theme loader and the palette
derivation behind it, its AI assistant, its booru image service, its weather
service, its Google Cloud translation and vision services, its JSON settings store,
and its Hyprland Lua configuration are all either in the tree or upstream of it, and
none of them is part of Zenith. The reason they are not removed is structural rather
than diplomatic: dropping a file from a vendored tree is an edit to it, and an edit to
it is the one thing a vendored tree does not allow. So the rule is that a component
that is present and unused is inert, and a component that is absent is a diff against
upstream that has to be re-applied at every re-vendor. Carrying it and not starting
it is the answer.

**What is being taken**, in the order it is worth doing:

- [x] **Vendored upstream components.** The shell's surfaces are a vendored
  open-source Quickshell configuration, held byte-identical to a pinned upstream
  commit and re-themed rather than forked. The decision is a trade, and the trade is
  recorded rather than implied: Zenith gives up writing a bar, a dock and a
  launcher from nothing, and takes on a dependency it does not control, pinned to a
  commit it has to re-vendor and re-check when upstream moves. What makes the trade
  worth taking is the byte-identical rule. The tree is upstream's; every change is a
  configuration value, an overlay or a patch; and the shell's design system reaches
  the components through three overlay files at upstream's own paths, so the desktop
  looks like Zenith's without a vendored file being edited. The cost is stated
  rather than discovered: a re-vendor is a re-theme event, because the bridges are a
  contract that no linter checks.
- [x] **The monochrome re-theme.** Strict monochrome is now the whole desktop's rule
  and not only the shell's own surfaces. The vendored components are monochrome by
  configuration and not by exception: a token bridge answers upstream's Material
  property names with Zenith's greys, and the handful of colours upstream hard-codes
  in its own files are a patch to make rather than an exception to grant. The
  wallpaper palette is namespaced and kept as the record of what the wallpaper would
  have said rather than as something the desktop draws with. The decision recorded
  here is the hard one: a component that cannot be made monochrome without being
  patched is a finding against vendoring it, not a reason to grant it an exception.
  An exception for one component is how a desktop ends up with two design systems,
  one of which is a coincidence.
- [x] **The shell daemon.** `zenith-shell daemon` (unit `zenithd.service`) replaces the
  shell's two-second poll of `zenith state` with a subscription to a streaming Go daemon
  speaking `zenith-shell/1` over a Unix socket. The decision is where the parsing lives: it
  stays in Go, where it is tested against real captured output of tools that do not
  behave, and the shell reads a frame rather than interpreting a tool's output
  itself. `zenith state` survives as a CLI of its own, so a terminal, a script and
  anything that is not Quickshell keep a way in; it probes its tools through
  `desktop/internal/desktopstate`, which the daemon, a port of Ryoku's, does not use.
- [x] **Theming.** A wallpaper that recolours the desktop, with no matugen on the machine:
  `zenith theme generate` makes the scheme and `zenith theme render` fills the app
  templates, both in Go (`desktop/internal/theme`). The daemon runs them on every wallpaper
  change and writes `colors.json` and `tones.json` under `$XDG_CACHE_HOME/zenith/`, and the
  shell reads them, so picking a wallpaper repaints the system and nothing restarts. The
  default scheme is `scheme-monochrome`: the wallpaper drives tone and not hue, unless a
  person picks a coloured scheme. See [`theming.md`](theming.md)
- [x] **The sidebar.** A slide-in panel for audio, network, bluetooth, display
  and media, on the right edge. Quick settings are a popover; this is the surface
  that belongs to the edge rather than to a keybinding. `zenith state` probes all
  five tools in Go and emits JSON, and `Sidebar.qml` is a renderer that never
  interprets a tool's output itself — the same contract the Hub has. That split is
  the whole design: wpctl reporting `Volume: 1.53`, nmcli escaping the colons in a
  MAC address, and bluetoothctl exiting 0 while printing an error are five parsers
  that would have had no tests in QML, so they are in Go with fixtures taken from
  real captured output. Nothing fails on absent hardware; a section with no
  adapter is hidden rather than shown as an error. The state comes from `zenithd`
  over its socket rather than from a two-second poll of the CLI, and a closed
  panel costs nothing because the daemon decides when to push. It opens through
  `togglePopover("sidebar")` in
  `shell.qml`, alongside the launcher and quick settings — and, like those two,
  nothing calls that function yet. No keybinding layer exists in this shell at
  all, so the sidebar is reachable only from QML today
- [ ] **Notifications in the sidebar.** The roadmap originally folded this into the
  sidebar and it is not there. The server keeps a 32-entry history, but reaching it
  means reading an `id` across a file boundary in QML, and Quickshell is not
  installed on any machine this project can test on. Guessing at an API that cannot
  be checked is the one thing here that is worse than shipping late
- [x] **Window switching.** A most-recently-used switcher over Hyprland, on
  Alt+Tab (Super+Tab opens the workspace overview instead). `zenith windows`
  reads
  `hyprctl -j clients`, drops unmapped and scratchpad windows, and sorts by
  `focusHistoryID` — which is *inverted* from what it looks like: Hyprland's
  history vector is oldest-first and the id is `size - index - 1`, so **lower is
  more recent** and zero is the window you are already in. Sorting descending,
  the obvious thing, is least-recently-used. That is the whole feature, so it is
  sorted in Go with tests rather than in QML. Also gotcha-shaped and worth
  writing down: scratchpad workspaces are `-99..-2`, so `id < 0` is *wrong* — it
  also catches name-based normal workspaces, which start at -1337
- [x] **The binds.** The compositor configuration is in the repository now
  (`session/hyprland/`, hyprlang, for Hyprland 0.53.3). Alt+Tab advances the switcher
  and its release, a `bindr`, closes it; Alt+Shift+Tab walks back; Super+Tab opens the
  workspace overview and Super+Alt+Tab steps its desktops. The first version put the
  switcher on Super+Tab and closed it with `bindl`, which is Hyprland's *locked* flag and
  not release, so the card flashed for about 50 ms (`journal/2026-10-23.md`)
- [x] **Plugins.** A mechanism for shell plugins, and the thing that makes desktop
  widgets addable by anyone: `zenith plugin`, the plugin runtime in `shell/plugins/` and
  the `qs.PluginKit` kit. See [`plugins.md`](plugins.md)
- [ ] **Rollback.** Btrfs snapshots around every update. Zenith's overlay
  rollback is configuration only — it restores files this project wrote and
  cannot restore a package database
- [ ] **The agent.** M10: a local knowledge vault, an index over the machine, and
  a dashboard. The daemon, the vault and the dashboard are built as Zenith Agent
  (`zenith-agent`, off until the user switches it on; [`agent.md`](agent.md)), but no
  engine ships, so this stays open until one is wired to its `engine.command` seam
- [ ] **The installer.** Blocked on M4 and on root. Recorded here so the scope is
  visible, not because it is reachable

## Later, unsequenced

- A package index for the shell's plugins (the plugin mechanism itself is built)
- `zenith benchmark` so performance regressions are visible rather than anecdotal
- A second base release, once hardware profiles justify it