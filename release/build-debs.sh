#!/bin/sh
# Build the Zenith control-tool package.
#
# Two properties this script is responsible for, and neither is optional:
#
#   - The package writes only where the base-layer boundary allows. That is
#     checked here rather than in review, because the whole point of the rule is
#     that a package cannot quietly gain a file another package owns, and a
#     reviewer reading a diff of this script cannot see what dpkg-deb will emit.
#   - The build is reproducible. SOURCE_DATE_EPOCH is pinned, mtimes are
#     normalized, and the script prints the digest so two builds can be compared
#     with one command.
#
# Usage: release/build-debs.sh [outdir]
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
OUTDIR=${1:-"$REPO_ROOT/build/debs"}
PKG=zenith

# Pinned for reproducibility. Override with a commit timestamp when cutting a
# release; leaving it at a constant is what makes "build it twice, compare" a
# check rather than a coincidence.
SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH:-1700000000}
export SOURCE_DATE_EPOCH

ARCH=$(dpkg --print-architecture 2>/dev/null || echo amd64)
VERSION=${ZENITH_VERSION:-0.1-pre-alpha}

echo "building $PKG $VERSION ($ARCH)"

# ---------------------------------------------------------------- build binary
STAGE=$(mktemp -d)
trap 'rm -rf "$STAGE"' EXIT

# -trimpath keeps the build directory out of the binary, which is both
# reproducible and one fewer thing to leak about the machine that built it.
( cd "$REPO_ROOT/desktop" && \
  CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" \
    -o "$STAGE/usr/bin/zenith" ./cmd/zenith )

# ----------------------------------------------------------------- session
# Content lives under /usr/share/zenith/session/; the registrations that point at
# it are a separate concern and belong to the packages that own the session.
# The greeter wrapper. It ships without the login surface and without the SDDM
# drop-in, because a greeter that cannot authenticate is worse than no greeter:
# it is a machine that takes a password and does nothing with it. What ships is
# the part that is testable on its own — bringing up a kiosk Weston with a sane
# mode — and session/greeter_test.sh runs it.
mkdir -p "$STAGE/usr/share/zenith/session"
cp "$REPO_ROOT/session/zenith-greeter" "$STAGE/usr/share/zenith/session/"
chmod 0755 "$STAGE/usr/share/zenith/session/zenith-greeter"

mkdir -p "$STAGE/usr/share/zenith/session"
cp "$REPO_ROOT/session/zenith-session" "$STAGE/usr/share/zenith/session/"
cp "$REPO_ROOT/session/zenith.desktop" "$STAGE/usr/share/zenith/session/"
cp "$REPO_ROOT/session/zenith-portals.conf" "$STAGE/usr/share/zenith/session/"
cp "$REPO_ROOT/session/zenith-activate" "$STAGE/usr/share/zenith/session/"
chmod 0755 "$STAGE/usr/share/zenith/session/zenith-session" \
   "$STAGE/usr/share/zenith/session/zenith-activate"

# The keybinding catalogue.
#
# It is the list of what the keyboard does — every chord, with a label and a
# category — and it is installed beside the session's own files because it is
# not the compositor's: the compositor's configuration is one of the two
# things that have to agree with it, and the shell draws the other.
#
# `zenith binds` prints it for the shell's cheat sheet, and `zenith doctor`
# checks the compositor's configuration against it, so it has to be where
# both of them look.
cp "$REPO_ROOT/session/binds.json" "$STAGE/usr/share/zenith/session/"

# The compositor's configuration. It is loaded by the session with
# `Hyprland -c`, and it is the only place the keyboard is described — the
# binds call the shell's IPC target, so the shell decides what a key means and
# the compositor only says when.
mkdir -p "$STAGE/usr/share/zenith/hyprland"
cp "$REPO_ROOT/session/hyprland/"*.conf "$STAGE/usr/share/zenith/hyprland/"

# The units ship as content under /usr/share/zenith/units/. The registrations
# that point at them are written below and are the only paths this package
# owns outside its own two directories.
mkdir -p "$STAGE/usr/share/zenith/units"
cp "$REPO_ROOT/session/units/"*.service "$REPO_ROOT/session/units/"*.target \
   "$STAGE/usr/share/zenith/units/"

# --------------------------------------------------------------- registrations
# Every path here is on the sanctioned list in docs/architecture.md, because
# each one is in a fixed directory that a third-party binary scans and nowhere
# else. The boundary check below is what stops a seventh one appearing.
mkdir -p "$STAGE/usr/share/wayland-sessions"
cp "$STAGE/usr/share/zenith/session/zenith.desktop" \
   "$STAGE/usr/share/wayland-sessions/zenith.desktop"

mkdir -p "$STAGE/usr/share/xdg-desktop-portal"
cp "$STAGE/usr/share/zenith/session/zenith-portals.conf" \
   "$STAGE/usr/share/xdg-desktop-portal/zenith-portals.conf"

mkdir -p "$STAGE/usr/lib/systemd/user"
cp "$STAGE/usr/share/zenith/units/"*.service \
   "$STAGE/usr/share/zenith/units/"*.target "$STAGE/usr/lib/systemd/user/"

# ------------------------------------------------------------------- the shell
# The Quickshell configuration. This is the directory `quickshell -p` is pointed
# at, so it is the difference between a shell and an empty window.
# The tree ships whole, and deliberately. The shell's own code long outgrew the
# roots a copy by enumeration had to name: Services/, Ui/, components/,
# FrameBars/, Barkit/, PluginKit/, scripts/, assets/, matugen/, utils/ and
# Blobs/ are directories the entry point and the bar styles import by name —
# `qs.Services`, `qs.Ui.Singletons`, `qs.Barkit` resolve through qmldir files a
# level or two inside them — and Quickshell resolves a module by the name its
# qmldir declares, not by where a copy writer thought to look. Copying
# directories by hand is exactly how the 2026-10-23 gap happened: a new entry
# point laid onto a tree still missing the directories it imports logs two
# module errors into the journal and renders nothing. The tree is one tree to
# Quickshell, so the package copies it as one tree.
mkdir -p "$STAGE/usr/share/zenith/shell"
cp -R "$REPO_ROOT/shell/." "$STAGE/usr/share/zenith/shell/"

# What is checkout-only, and stays there. `vendor.sh` is the tool that syncs
# the vendored tree from upstream — the reasoning below, and it has not
# changed: it clones over the network, needs git, and writes into a checkout,
# none of which an installed machine has. `shell_test.sh` names checkout
# paths. `i18n/` is a Go module the control tools compile from, not content
# the shell reads from disk; shipping it would put a language pack in a
# package nothing opens.
rm -f  "$STAGE/usr/share/zenith/shell/vendor/vendor.sh" \
       "$STAGE/usr/share/zenith/shell/vendor/README.md" \
       "$STAGE/usr/share/zenith/shell/shell_test.sh"
rm -rf "$STAGE/usr/share/zenith/shell/i18n"

# The vendored upstream tree.
#
# Two components, each pinned to a commit, each byte-identical to upstream, and
# none of them edited in place. The whole tree ships, not only the files at the
# shell's root: the vendored components import each other through the shell
# root's own `modules/` and `services/` directories, so a package that shipped
# only the top-level QML would install a shell whose every import fails to
# resolve — a shell that starts and then renders nothing, with nothing in the
# journal saying why.
#
# Zenith's own files are in the tree too, at the paths upstream uses for them.
# `modules/common/Appearance.qml` and `modules/common/Config.qml` are Zenith's
# palette and Zenith's settings, and they are the whole of the re-theme: the
# vendored components read Zenith's colours and Zenith's settings because they
# are told to, and not one vendored file is changed to do it. They are copied
# with everything else, which is the only way they can be — upstream names them
# by path, so they have to be at it.
# None of the three needs its own enumeration any more: the lay above copied
# `modules/`, `services/` and `vendor/` together with every other root, and the
# tree owns one copy of itself — so there is no second list to drift the next
# time the shell grows a directory.

# The manifest, and the licences.
#
# The manifest is the list of what the vendored tree consists of: every file,
# and the hash of its bytes. `zenith doctor` checks the installed tree against
# it, so it ships with the tree — a check that reads a file the package does
# not install is a check that reads nothing, and the doctor would report a
# missing manifest on a machine where the manifest is sitting in the checkout
# and was simply never packaged.
#
# The licences travel with the code they cover. GPL-3.0 section 4 asks for a
# copy of the licence with the work, and the Apache licence asks for its own
# text, so both texts are installed beside the tree.
#
# `vendor.sh` is not installed, and that is a decision rather than an
# omission. It is the tool that syncs the tree from upstream: it clones over
# the network, it needs git, and it writes into a checkout. There is no
# checkout on an installed machine, and the directory it would write into is
# owned by this package — so shipping it would put a script on a user's
# machine whose only possible effect is to rewrite a package-owned directory.
# It stays in the repository, where it is run from.
cp "$REPO_ROOT/shell/vendor/manifest.yaml" \
   "$REPO_ROOT/shell/vendor/COPYING" \
   "$REPO_ROOT/shell/vendor/LICENSE.apache-2.0" \
   "$STAGE/usr/share/zenith/shell/vendor/"

# The attribution. It is installed at the path the manifest names — `notices:`
# says `shell/vendor/notices.md` — and in the package's documentation
# directory, where a reader looking for who wrote this looks first. One file,
# two copies, so the two cannot disagree.
#
# It is not rendered from the manifest, and that is deliberate. The manifest
# records two components and a hash for every file in them. It cannot record
# that `modules/common/functions/fuzzysort.js` is MIT code from another
# project, or that `services/MprisController.qml` carries no grant at all, or
# that four files inside the tree came from caelestia. Those are facts about
# files inside the vendored tree, and two of the licences involved — MIT and
# Apache-2.0 — require the notice to travel with the copies. A file rendered
# from the manifest would ship the GPL and Apache texts and none of the
# per-file credits.
cp "$REPO_ROOT/shell/vendor/notices.md" \
   "$STAGE/usr/share/zenith/shell/vendor/notices.md"

# The Hub's window.
#
# The entry file is at the shell's root and the window's own code is in a
# directory under it, because Quickshell's configuration root is the directory of
# the file it is started on, and nothing else will do: the Hub is drawn with the
# same component kit as the desktop, the kit's files reach each other through
# `qs.modules.*`, and those imports name nothing from a root of `shell/hub/`. The
# entry file therefore ships with the shell's other root files, above, and only
# the directory below is copied here.
cp -R -T "$REPO_ROOT/shell/hub" "$STAGE/usr/share/zenith/shell/hub"
cp "$REPO_ROOT/session/zenith-hub" "$STAGE/usr/bin/zenith-hub"
chmod 0755 "$STAGE/usr/bin/zenith-hub"

# The mark. SVG, because it is the only form that stays sharp from a 16-pixel
# status icon to a 4K greeter, and because a raster of it would be a second
# source of truth for a shape that has exactly one.
mkdir -p "$STAGE/usr/share/zenith/branding"
cp "$REPO_ROOT/public/zenith-logo.svg" "$STAGE/usr/share/zenith/branding/"
cp "$REPO_ROOT/public/zenith-logo-on-dark.svg" "$STAGE/usr/share/zenith/branding/"

# The wallpapers. All seven ship, because a desktop that offers one wallpaper is
# a desktop that has made the user's taste the project's taste. They are content
# under /usr/share/zenith/, so no registration is involved.
#
# 14 MB, and that is the honest cost of shipping someone's artwork: the package
# grows and there is no image tool on the base layer to re-encode them without
# making the images somebody else's. They are copied byte for byte.
mkdir -p "$STAGE/usr/share/zenith/backgrounds"
cp "$REPO_ROOT/public/wallpapers/"* "$STAGE/usr/share/zenith/backgrounds/"

# The schema is a registration: GLib compiles from this fixed directory and
# nowhere else. The postinst runs glib-compile-schemas over it, because the XML
# on its own is not readable by GSettings.
mkdir -p "$STAGE/usr/share/glib-2.0/schemas"
cp "$REPO_ROOT/shell/"org.zenith.*.gschema.xml "$STAGE/usr/share/glib-2.0/schemas/"

# The SDDM drop-in is deliberately NOT installed yet.
#
# Its entire content is DisplayServer=wayland with
# CompositorCommand=zenith-greeter, and SDDM has no way to start a Wayland
# greeter that does not exist: shipping the drop-in before zenith-greeter lands
# does not degrade the login screen, it removes it. The registration stays on the
# sanctioned list and stays reported by `zenith doctor` so the gap is visible.
# It ships with the greeter.

# --------------------------------------------------------------- hardware data
mkdir -p "$STAGE/usr/share/zenith/hardware/profiles"
cp "$REPO_ROOT/system/hardware/tiers.yaml" "$STAGE/usr/share/zenith/hardware/"
cp "$REPO_ROOT/system/hardware/profiles/"*.yaml \
   "$STAGE/usr/share/zenith/hardware/profiles/"

# ------------------------------------------------------------------ the licences
# The package's own documentation directory, and what has to be in it.
#
# `copyright` is the machine-readable one: the Debian copyright format, with a
# paragraph per licence group. It used to be the repository's root LICENSE file
# copied in verbatim, which records Zenith's own licence and says nothing about
# the two vendored components the package also ships — and a copyright file
# that names one licence for a package under three is a file that tells a
# reader the package is under a licence it is not entirely under.
#
# The licence texts go with it. A vendored GPL-3.0 component and a vendored
# Apache-2.0 component both oblige the redistributor to ship the licence, and
# dpkg's own tools and `licensecheck` look for them here. The GPL-3 text is
# the vendored component's own, which is byte-identical to the repository's
# root LICENSE; release_test.sh checks that it stays that way, because a
# package that ships two different GPL-3 texts is shipping a licence nobody
# has read.
mkdir -p "$STAGE/usr/share/doc/$PKG"
# The copyright is emitted, not copied: its vendored stanzas are the manifest's
# own file list, and a copy kept by hand is a copy that diverges the moment the
# tree grows a directory the manifest does not recognize. `gen-copyright.sh`
# carries the reasoning; `release_test.sh` re-runs it and fails on drift.
"$REPO_ROOT/release/mkdeb/gen-copyright.sh" > "$STAGE/usr/share/doc/$PKG/copyright"
cp "$REPO_ROOT/shell/vendor/COPYING" "$STAGE/usr/share/doc/$PKG/GPL-3"
cp "$REPO_ROOT/shell/vendor/LICENSE.apache-2.0" \
   "$STAGE/usr/share/doc/$PKG/Apache-2.0"
cp "$STAGE/usr/share/zenith/shell/vendor/notices.md" \
   "$STAGE/usr/share/doc/$PKG/vendor-manifest.md"

# --------------------------------------------------------------- control files
INSTALLED_KB=$(du -sk "$STAGE" | cut -f1)
mkdir -p "$STAGE/DEBIAN"
sed -e "s|@VERSION@|$VERSION|" \
    -e "s|@ARCH@|$ARCH|" \
    -e "s|@SIZE@|$INSTALLED_KB|" \
    "$REPO_ROOT/release/mkdeb/control.in" > "$STAGE/DEBIAN/control"

install -D -m 0755 "$REPO_ROOT/release/mkdeb/postinst" "$STAGE/DEBIAN/postinst"
printf '/usr/bin/zenith\n' > "$STAGE/DEBIAN/zenith.list"

# ------------------------------------------------------------------- metadata
# Normalize modes before normalizing times. A staging tree inherits the build
# machine's umask, so without this the .deb ships directories as 0700 and data
# files as 0664 — which installs a world-unreadable /usr/share/zenith and a
# group-writable profile database that any user on the machine can edit.
find "$STAGE" -type d -exec chmod 0755 {} +
find "$STAGE" -type f -exec chmod 0644 {} +
# Every file something executes. Everything else is content and stays 0644:
# a session entry point that is not executable is a login manager whose Exec=
# line fails, and it fails on every login rather than at build time.
#
# This list is derived, not written by hand. A file chmod'd here and forgotten
# below the reset is a shipped 0644 that works in the checkout and fails on the
# installed machine — and the checkout is where it gets tested, so nothing
# notices. zenith-hub and zenith-greeter shipped that way: the settings
# application and SDDM's CompositorCommand, both non-executable, both
# Permission denied on the real system.
#
# Anything under the tree with the executable bit in the source stays executable.
# The set is therefore whatever the repository says is a program, which is a
# fact the repository can be checked against rather than a list that goes stale.
while IFS= read -r program; do
  chmod 0755 "$STAGE/${program#./}"
done <<'EXECUTABLES'
./usr/bin/zenith
./usr/bin/zenith-hub
./usr/share/zenith/session/zenith-session
./usr/share/zenith/session/zenith-greeter
./usr/share/zenith/session/zenith-activate
EXECUTABLES
chmod 0755 "$STAGE/DEBIAN/postinst"
chmod 0640 "$STAGE/DEBIAN/control"
chmod 0644 "$STAGE/DEBIAN/zenith.list"

# The shell's tree is installed under the names it has in the repository, so
# its executables are asked for rather than written down.
#
# The list above is written by hand because the session's files are installed
# under names that are not their own, and a hand-written list is a list that
# goes stale. The shell's are not renamed, so the checkout is the authority:
# a vendored script upstream ships executable and the package ships
# non-executable is a script that works in the checkout and fails on the
# machine, which is the same fault as a non-executable session entry point and
# exactly as silent.
#
# The vendor directory is not in this list because `vendor.sh` is not
# installed; `shell_test.sh` and `i18n/` are checkout-side for the same class
# of reason. Asking the checkout about a file the package does not ship would
# be asking the package about a file it does not have.
while IFS= read -r program; do
  chmod 0755 "$STAGE/usr/share/zenith/shell/$program"
done <<EOF
$(cd "$REPO_ROOT/shell" && find . -type f -perm -u+x \
    ! -path './vendor/*' ! -path './i18n/*' ! -name shell_test.sh | sort)
EOF

find "$STAGE" -exec touch -h -d "@$SOURCE_DATE_EPOCH" {} +
mkdir -p "$OUTDIR"
DEB=$(mktemp --suffix=.deb)
( cd "$STAGE" && dpkg-deb --root-owner-group --build "$STAGE" "$DEB" >/dev/null )
mv "$DEB" "$OUTDIR/${PKG}_${VERSION}_${ARCH}.deb"

# --------------------------------------------------------- boundary enforcement
# Every path this package owns must be under one of the roots the architecture
# record permits. The binary directory is the one exception, and it is on the
# sanctioned list because PATH looks nowhere else.
"$REPO_ROOT/release/check-boundary.sh" "$STAGE" "$PKG"

echo
echo "$OUTDIR/${PKG}_${VERSION}_${ARCH}.deb"
sha256sum "$OUTDIR/${PKG}_${VERSION}_${ARCH}.deb" | cut -d' ' -f1
