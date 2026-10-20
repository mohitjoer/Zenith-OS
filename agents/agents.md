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

Zenith is a Linux distribution. Its **base layer** is Ubuntu 26.04 LTS; everything above that
line is Zenith's own. Hyprland is the compositor and a Quickshell-drawn shell is the desktop;
Go binaries (`zenith`, `zenith-shell`, `zenith-hub`) own the settings; GSettings schemas and dconf are
the store. **There is no GNOME in Zenith** — no GNOME Shell, no GNOME daemons, no
`xdg-desktop-portal-gnome`.

Before designing or changing anything, read [`docs/architecture.md`](../docs/architecture.md).
It maps every layer to its owner and explains why Zenith is built this way and not another way.
Do not re-derive decisions that are already recorded there.

## The one rule

**One writer per setting.** If a setting has a command or a tool that owns it, that tool is
the only thing allowed to write it. Hand-editing a store that a tool owns causes drift, and
drift is the single most expensive bug class in a distribution. Read a command's `--help`
before running it.

The second rule: **never edit a shipped file in place.** Files laid into the system by a
package are replaced on update. A change to one belongs in an override, in the store the
owning tool reads, or in the tool itself.

## Ownership model

| Path | Owner | A change goes |
|---|---|---|
| `/usr/share/zenith/` | the package | an override or a new package version |
| `/etc/zenith/` | the package | an override |
| `~/.config/zenith/` | the user, materialized by `zenith` | `zenith-shell` / `zenith-hub` |
| `~/.config/zenith/user_edits/` | the user overlay | wins at the mirrored path |
| `~/.local/share/zenith/plugins/` | the plugin | the plugin's own manifest |
| `~/.local/state/zenith/` | `zenith` | the tool that wrote it; holds rollback revisions and snapshots |
| GSettings / dconf | `zenith-shell`, `zenith-hub` | `gsettings`, through the tool |

Zenith's packages write **only** to `/usr/share/zenith/` and `/etc/zenith/`, never to `~`. That is
what makes `zenith update` able to roll back: the base can be replaced freely because the
user's state lives somewhere the package cannot reach.

## Design system

Zenith's visual language is monochrome by construction. These rules are the reason Zenith looks
like Zenith; they are not suggestions.

- **Strict monochrome.** Black, grey and white only, no coloured accents. Traffic lights and
  destructive/warning/error states are the sole exceptions, because they carry meaning a
  grey cannot.
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

- [ ] Lints pass (if configured)
- [ ] Tests pass (if present)
- [ ] Change maps back to a goal in [`docs/roadmap.md`](../docs/roadmap.md)
- [ ] Any new rule or style pattern discovered this session was recorded in this file
- [ ] `AGENTS.md`, `CLAUDE.md` and `GEMINI.md` still exist and still point to this file

## Verification commands

```bash
# GSettings schemas — must compile clean before anything reads them
glib-compile-schemas --strict --dry-run desktop/**/schemas/

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

# gofmt / vet — the control tools are Go
gofmt -l desktop/
go vet ./...

# Every licence file must match the root LICENSE
for f in $(find . -name LICENSE -not -path './.git/*'); do cmp -s LICENSE "$f" || echo "MISMATCH: $f"; done
```

`zenith doctor` is the check that runs on a machine rather than in a checkout: it
reports what differs between the base and the user's tree, whether the session
can actually reach the shell, and whether the portal configuration is being
read. It is the first thing to run when a session comes up wrong, and it answers
questions the other checks cannot — they read the repository, and the failure
is always in what was installed.

These paths are aspirational until `desktop/` exists. They document the shape the tree is
meant to take, so a contributor adding the first file knows where it belongs. The source
tree is `desktop/`, never a directory named after a control tool — a `zenith/` directory
collides with the built binary and silently defeats the repository's own ignore rules.

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