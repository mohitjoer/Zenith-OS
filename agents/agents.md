# Zenith OS — Agent Guidelines

Zenith's master agent contract. `AGENTS.md`, `CLAUDE.md` and `GEMINI.md` are pointers to this
file. Everything below is binding unless it says otherwise.

Those three are symlinks, created once per clone and deliberately untracked — which agent you run
is your machine's business, and the contract they point to is what the repository actually cares
about. A fresh clone does not have them, so create them before your first pull request or the
Definition of Done cannot be satisfied:

```sh
ln -s agents/agents.md AGENTS.md
ln -s agents/agents.md CLAUDE.md
ln -s agents/agents.md GEMINI.md
```

## What Zenith is

Zenith's wordmark is **天** — *zenith* — healthy, robust, strong. Use it as the logo and as the
project's name in prose: "Zenith", never "Zenith's" as a possessive of an acronym, never anything
else.

Zenith is a Linux distribution. Its **base layer** is Ubuntu Server 26.04 LTS; everything above that
line is Zenith's own. Hyprland is the compositor and a Quickshell-drawn shell is the desktop;
Go binaries (`zenith`, `zenithd`, `zenith-hub`) own the settings; GSettings schemas and dconf are
the store. **There is no GNOME in Zenith** — no GNOME Shell, no GNOME daemons, no
`xdg-desktop-portal-gnome`.

Before designing or changing anything, read [`docs/architecture.md`](../docs/architecture.md).
It maps every layer to its owner and explains why Zenith is built this way and not another way.
Do not re-derive decisions that are already recorded there.

The shell's **surfaces** are not all Zenith's. A vendored tree of open-source Quickshell
components lives under `shell/modules/` and `shell/services/`, kept byte-identical to a
pinned upstream commit. Zenith owns the engine, the design system, the settings binding and
the three overlay files that carry both into the vendored components. The rules for that
tree are in [the one rule](#the-one-rule), and the record is in
[the architecture](../docs/architecture.md#vendored-upstream-components).

## The one rule

**One writer per setting.** If a setting has a command or a tool that owns it, that tool is
the only thing allowed to write it. Hand-editing a store that a tool owns causes drift, and
drift is the single most expensive bug class in a distribution. Read a command's `--help`
before running it.

The second rule: **never edit a shipped file in place.** Files laid into the system by a
package are replaced on update. A change to one belongs in an override, in the store the
owning tool reads, or in the tool itself.

The third rule: **a vendored third-party file is a shipped file.** The vendored tree under
`shell/modules/` and `shell/services/` is kept byte-identical to the upstream commit it is
pinned to, and the second rule applies to it with exactly the same force. A vendored file
is not Zenith's to edit: a hand edit is destroyed by the next sync, cannot be re-vendored
from upstream, and makes the manifest — which records a hash for every vendored file — lie
about what changed.

Every change to a vendored component goes in one of three places, and there is no fourth:

- a **configuration value** — a setting the component reads;
- an **overlay file** — a Zenith-owned file at the path the component expects, which is how
  the monochrome palette and the GSettings-backed configuration reach it;
- a **patch** in `shell/vendor/patches/` — the path the vendoring script names for them.
  There is no patch there today, because no change has needed one yet.

`sh shell/vendor/vendor.sh verify` is the check, and it is not advisory: it compares every
vendored file against the hash the manifest records, and a file edited by hand fails it.

## Ownership model

| Path | Owner | A change goes |
|---|---|---|
| `/usr/share/zenith/` | the package | an override or a new package version |
| `/etc/zenith/` | the package | an override |
| `/usr/share/zenith/shell/modules/`, `/usr/share/zenith/shell/services/` | upstream, at the pinned commit | a configuration value, an overlay, or a patch — never a hand edit |
| `~/.config/zenith/` | the user, materialized by `zenith` | `zenithd` / `zenith-hub` |
| `~/.config/zenith/user_edits/` | the user overlay | wins at the mirrored path |
| `~/.local/share/zenith/plugins/` | the plugin | the plugin's own manifest |
| `~/.local/state/zenith/` | `zenith` | the tool that wrote it; holds rollback revisions and snapshots |
| GSettings / dconf | `zenithd`, `zenith-hub` | `gsettings`, through the tool |

Zenith's packages write **only** to `/usr/share/zenith/` and `/etc/zenith/`, never to `~`. That is
what makes `zenith update` able to roll back: the base can be replaced freely because the
user's state lives somewhere the package cannot reach.

## Naming

Zenith's shell was inspired by another project, and the README says so in one line. That
line is the whole of the attribution. The name appears nowhere else in this repository —
not in an identifier, not in a comment, not in a path, not in a licence notice, not in a
design document.

The reason is not secrecy. A name in an identifier becomes an identifier in every fork,
every downstream package and every issue that quotes it, and a name that stands for a
decision rather than for a thing stops being a name at all. Where a document would
naturally credit a project by name, it credits the project that is actually being used:
the vendored upstream, with its URL, its licence and its pinned commit. That is the
attribution the licence requires, and it is the correct attribution anyway.

Zenith's own names are Zenith's own: `zenith`, `zenithd`, `zenith-hub`, `zenith-shell`,
`org.zenith.*`, `/usr/share/zenith/`.

## Design system

Zenith's visual language is monochrome by construction. These rules are the reason Zenith looks
like Zenith; they are not suggestions.

- **Strict monochrome.** Black, grey and white only, no coloured accents. Traffic lights and
  destructive/warning/error states are the sole exceptions, because they carry meaning a
  grey cannot.
- **Strict monochrome covers the whole desktop, vendored components included.** A vendored
  surface is monochrome by configuration or by patch — never by exception, and never by
  asking the component to keep its own palette. A component that cannot be made monochrome
  without being patched is a finding against vendoring it, not a reason to let a second
  palette onto the desktop.
- **Two-stop elevation.** Every raised surface gets a 1px contact border plus a soft ambient
  shadow. One shadow reads as a flat sticker. Use the radius scale (6/7/8/10/12/14/18/22px)
  and the 4px spacing grid everywhere; do not invent values between them.
- **Dark and light variants declare the same properties in the same order**, differing only
  in token values. A variant that adds, drops or reorders a selector is a bug, because it
  silently diverges and nobody diffs it.
- **Contrast floor: 4.5:1 for body ink, 3:1 for UI shapes and indicators** — workspace dots,
  inactive states, dividers. Any value written with alpha must be contrast-checked *after
  compositing over its real backdrop*, not against raw black or white. A scrim that looks
  right in isolation can fail once the compositor's own blur and dim are applied.
- **Express character through material, shape and rhythm, never through accent colour.**

## Style patterns (not linter-enforceable)

These are the traps. The linter will not catch them.

- GTK window controls achieve a circular aspect ratio with vertical margin constraints,
  where the margin equals `(headerbar height - button diameter) / 2`.
- GTK 4 CSS has no `spacing`, no `row-spacing` / `column-spacing`, and no `::placeholder`.
  Use margin/padding on the container instead.
- QML has no `:has()`. Scope with property bindings or a child object, not a relational
  selector.
- Compositing: never assume a value's final appearance. Compute it over the actual
  backdrop chain.

## Definition of Done

- [ ] Lints pass (if configured) — over **Zenith's own sources**. The vendored tree is
      exempt by declaration in `shell/vendor/manifest.yaml`: a vendored file is upstream's,
      and upstream's style is not Zenith's to fail a lint over. Every file Zenith has
      changed in the vendored tree — an overlay, or a patched file — is Zenith's own code
      and is lint-clean, and the manifest is where that exemption is declared.
- [ ] Tests pass (if present)
- [ ] Change maps back to a goal in [`docs/roadmap.md`](../docs/roadmap.md)
- [ ] Any new rule or style pattern discovered this session was recorded in this file
- [ ] `AGENTS.md`, `CLAUDE.md` and `GEMINI.md` still exist and still point to this file

## Verification commands

```bash
# GSettings schemas — must compile clean before anything reads them.
# The schemas live in the shell's own directory: the shell reads them, and the
# shell is the shell. There is no schemas/ directory to glob.
glib-compile-schemas --strict --dry-run shell/

# The shell's configuration, loaded by the engine that will run it.
# This is the check that catches what reading the QML cannot: a property that
# does not exist, a type from a module that is not imported, a name another
# library already has. It skips itself, and says so, on a machine with no
# quickshell or no Wayland display.
sh shell/shell_test.sh

# The session, the greeter, the package and the repository
sh session/session_test.sh
sh session/greeter_test.sh
sh release/release_test.sh

# gofmt / vet — the control tools are Go. There is no go.mod at the
# repository root: `desktop/` is one module and `system/hardware/` is
# another, so `go vet ./...` is run from inside a module and not from the
# root, where it would find nothing to vet and say so.
gofmt -l desktop/
(cd desktop/ && go vet ./...)

# The vendored tree — every vendored file byte-identical to its pinned
# upstream commit. A file edited by hand fails this and nothing else would.
sh shell/vendor/vendor.sh verify

# Every licence file must match the root LICENSE
for f in $(find . -name LICENSE -not -path './.git/*'); do cmp -s LICENSE "$f" || echo "MISMATCH: $f"; done
```

`zenith doctor` is the check that runs on a machine rather than in a checkout: it
reports what differs between the base and the user's tree, whether the session
can actually reach the shell, and whether the portal configuration is being
read. It is the first thing to run when a session comes up wrong, and it answers
questions the other checks cannot — they read the repository, and the failure
is always in what was installed.

These paths are the shape the tree has taken: `desktop/` is the control tools' module and
`system/hardware/` is a module of its own, so a Go command has to be run from inside one of
them rather than from the repository root, which has no `go.mod` and would find nothing.
The source tree is `desktop/`, never a directory named after a control tool — a `zenith/`
directory collides with the built binary and silently defeats the repository's own ignore
rules.

## Rules

- [2026-10-27] Never use or ship kitty in this OS; Ghostty is the sole supported terminal emulator.

## Branches

| Branch | Holds |
|---|---|
| `main` | Documentation. Design, decisions, hardware framework, project rules. |
| `dev-unstable` | Code. The working implementation, merged forward when a milestone completes. |

Documentation lands on `main`. Implementation lands on `dev-unstable`. A milestone is done
when its code has merged forward, not when it was written.

## Where things are

Read [`docs/architecture.md`](../docs/architecture.md) for the layer map. Read
[`system/hardware/README.md`](../system/hardware/README.md) before touching hardware
detection or adding a machine profile.

The tree under `shell/modules/` and `shell/services/` is upstream's, not Zenith's. Read
[`docs/architecture.md`](../docs/architecture.md#vendored-upstream-components) before
touching it, and run `sh shell/vendor/vendor.sh verify` afterwards: a hand edit does not
fail a review, it fails that.

## When something breaks

Gather before changing anything. A failure right after an update is usually the update.
Prefer a reversible fix, ask before anything destructive, and record what you found in a
dated note under `docs/journal/`.

## Historical reference

[`gnome-shell-notes.md`](gnome-shell-notes.md) records GNOME Shell internals learned while
this repository was the Zeo GNOME Extensions project. Zenith replaces GNOME Shell, so none of
it applies to Zenith's own shell. It is kept because it explains *why* the project moved, and
because it is the kind of knowledge that is expensive to rediscover if the decision is ever
revisited.