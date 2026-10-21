# Vendored components

The shell draws some of its surfaces with code Zenith did not write. That code is **vendored**:
copied into this repository from an upstream project, at a pinned commit, byte for byte.

This directory is the bookkeeping around that. It holds the script that writes the vendored
tree, the manifest that says what the tree must contain, the licence texts, and the notices
that say whose code is whose.

## The rule

**A vendored file is never edited in place.**

That is the project's own second rule, from [`agents/agents.md`](../../agents/agents.md):
files laid into the system by a package are replaced on update, so a change to one belongs in
an override, in the store the owning tool reads, or in the tool itself. A vendored file is a
shipped file in the most literal sense — it is a byte-for-byte copy of a file someone else
wrote, put here on purpose — so the rule applies to it in full.

The rule is also what makes the tree trustworthy. Every vendored file's content hash is
recorded in `shell/vendor/manifest.yaml`, `sh shell/vendor/vendor.sh verify` recomputes those
hashes, and `zenith doctor` checks the same ones. A hand edit does not survive either: the
next verify reports `MODIFIED` and the next sync overwrites it. That is the point. A vendored
tree that has been quietly patched is a tree whose licence provenance nobody can state.

## The two components

| Component | Upstream | Pinned commit | Licence | Lands at |
|---|---|---|---|---|
| `ii` | [end-4/dots-hyprland](https://github.com/end-4/dots-hyprland) — `dots/.config/quickshell/ii` | `2f2a376fdb2ad738bf55f693dd0b474b64bb0ab8` (2026-10-11) | GPL-3.0-only | `shell/modules/`, `shell/services/` |
| `shapes` | [end-4/rounded-polygon-qmljs](https://github.com/end-4/rounded-polygon-qmljs) | `e31ec4cb4ebf6a46b267f5c42eabf6874916fa16` | Apache-2.0 | `shell/modules/common/widgets/shapes/` |

`ii` is the upstream's Quickshell panel family: the bar, the dock, the sidebars, the overview,
the lock screen and the service graph underneath them. It is vendored from a subdirectory of a
larger dotfiles repository, and the paths are stripped so the tree lands at the shell root's
own `modules/` and `services/` — upstream's files import each other with absolute module paths
(`import qs.modules.common.widgets`), so they resolve without a vendored file being edited.

`shapes` is the rounded-polygon geometry the shape widgets draw with. Upstream carries it as
a submodule; a submodule is not a file, so it is vendored here as a component in its own right,
at its own pin and under its own licence.

## Where a change goes

Three homes, and only three. A hand edit of a vendored file is not one of them.

1. **A configuration value.** The setting upstream already reads. Nothing in the tree changes;
   only a value does. This is the cheapest home and the one to reach for first.
2. **A Zenith-owned overlay file, placed at upstream's path.** Upstream's code looks for a
   file at a path; Zenith puts its own file there. Upstream's code finds it and reads it, and
   nothing upstream is touched. The files below are exactly this.
3. **A patch in `shell/vendor/patches/`.** The diff that takes upstream to what Zenith needs,
   kept in the repository as a file rather than applied to the tree by hand, so that a
   re-vendor reproduces the same result.

The choice matters because of the rule at the top. A vendored file is upstream's, carries
upstream's licence, and is replaced on the next sync. Anything Zenith needs it to do that it
does not already do must arrive through one of the three homes.

## The overlay files

Some paths are excluded from the vendored copy. The sync script sets them aside before the
copy and puts them back afterwards, so upstream's version never lands there and Zenith's
survives every re-vendor. They are not in the manifest either: the manifest records vendored
files, and these are not vendored files.

| Path | Why Zenith owns it |
|---|---|
| `shell/GlobalStates.qml` | The shell's own state. Upstream keeps the "which surfaces are open" flags in a singleton at the shell root, and the vendored components read it — the bar asks whether the overview is open before it draws itself over it, the dock asks whether the screen is locked before it hides. The shell root is Zenith's, and upstream's file imports the whole service graph, which closes a cycle through the modules directory. So this is Zenith's file, at upstream's path, carrying the same flags. |
| `shell/modules/common/Appearance.qml` | The colour seam. Every vendored widget reads its colours from this one file. It is where Zenith hands out the monochrome token set from [`shell/Theme.qml`](../Theme.qml) in the shape upstream expects, so the vendored widgets draw in Zenith's palette. Owning the file upstream reads is the only way to re-theme the tree without editing a vendored file. |
| `shell/modules/common/Config.qml` | The settings seam. Same arrangement, one layer down: the vendored components read their settings from this one file, and this is where Zenith's version reads GSettings and dconf — the store the project's own rules say owns them. |
| `shell/modules/common/widgets/AppIcon.qml` | A widget the base layer cannot run. The vendored components instantiate it by that name, so the path has to exist; upstream's version is a KDE Frameworks `Kirigami.Icon`, and Zenith's base layer carries no KDE Frameworks — one widget is not a reason to put an application framework on a desktop whose manifest deliberately excludes it. Zenith's file draws the same icon from the same source, in the same two properties the one consumer uses. |

The whole of the re-theme and the re-bind is `Appearance.qml` and `Config.qml`: upstream's
components read Zenith's palette and Zenith's settings because they were pointed at a file
that holds them. No vendored file is modified.

The path is Zenith's; the content at it is Zenith's to write.

## Checking the tree

```sh
sh shell/vendor/vendor.sh verify
```

Recomputes the hash of every file the manifest lists and reports `MISSING` or `MODIFIED` for
any that do not match. Exit status is zero only when every vendored file is byte-identical to
the pinned commits.

`zenith doctor` runs the same check against the installed tree, without needing git or a
checkout of upstream: it recomputes each file's git blob id and compares it with the manifest's.
It runs on every doctor pass, because a check that only runs when somebody remembers to run it
is not a check.

```sh
sh shell/vendor/vendor.sh sync
```

Fetches the pinned commits and writes the tree, then rewrites the manifest. The upstream
repositories are cloned into a cache outside the tree — `${XDG_CACHE_HOME:-~/.cache}/zenith/vendor`
by default — so nothing upstream owns lands in the repository except the files the script
copies.

```sh
sh shell/vendor/vendor.sh update ii <commit>
sh shell/vendor/vendor.sh update shapes <commit>
```

Moves one pin and syncs. The commit has to exist in the upstream repository; the script does
not choose one, because choosing a version of someone else's code is a decision, not a
mechanical step.

## What the manifest records

`shell/vendor/manifest.yaml` is **generated**. Do not edit it by hand — the hashes in it are
what `zenith doctor` checks the tree against, and an edited manifest is a manifest that
describes a tree nobody has.

For each component it records the component's name, the upstream project and its URL, the path
inside the upstream repository, the pinned commit, the licence and the file that licence's text
is in, the copyright line, the SPDX identifier, and the path to the notices file. Then, for
every file in the component, the path it lands at and the git blob id of its contents.

Those blob ids are the whole mechanism. `verify` and `zenith doctor` recompute them, and a
file whose contents no longer hash to the recorded id is reported, not accepted.

## The licences

| File | What it is |
|---|---|
| `shell/vendor/COPYING` | The GPL-3.0 text of the `ii` component. Byte-identical to that project's own `LICENSE` file, and to this repository's root `LICENSE`. |
| `shell/vendor/LICENSE.apache-2.0` | The Apache-2.0 text of the `shapes` component. Byte-identical to that project's own `LICENSE` file. |
| `shell/vendor/notices.md` | The third-party notices: which files inside the vendored tree came from another project, under which licence, and where the full text is. |

The licence file for a component is not always named `LICENSE`, because the repository's own
check requires every file named `LICENSE` to be byte-identical to the root licence. A component
under a different licence therefore carries its text under a name that says which licence it
is.

Nothing the manifest lists is Zenith's own code, and nothing in it is Zenith's own design.
The design lives in `shell/Theme.qml` and the shell's own widgets, which are Zenith's own code
under Zenith's own licence.
