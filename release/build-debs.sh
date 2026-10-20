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
chmod 0755 "$STAGE/usr/share/zenith/session/zenith-session"

# The compositor's configuration. It is loaded by the session with
# `Hyprland -c`, and it is the only place the keyboard is described — the
# binds call the shell's IPC target, so the shell decides what a key means and
# the compositor only says when.
mkdir -p "$STAGE/usr/share/zenith/hyprland"
cp "$REPO_ROOT/session/hyprland/"*.conf "$STAGE/usr/share/zenith/hyprland/"

# The units and drop-ins ship as content under /usr/share/zenith/units/. The
# registrations that point at them are written below and are the only paths this
# package owns outside its own two directories.
mkdir -p "$STAGE/usr/share/zenith/units"
cp "$REPO_ROOT/session/units/"*.service "$REPO_ROOT/session/units/"*.conf \
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

mkdir -p "$STAGE/usr/lib/systemd/user/hyprland-session.target.d"
cp "$STAGE/usr/share/zenith/units/"*.service "$STAGE/usr/lib/systemd/user/"
cp "$STAGE/usr/share/zenith/units/hyprland-session.target.d-10-zenith.conf" \
   "$STAGE/usr/lib/systemd/user/hyprland-session.target.d/10-zenith.conf"

# ------------------------------------------------------------------- the shell
# The Quickshell configuration. This is the directory `quickshell -p` is pointed
# at, so it is the difference between a shell and an empty window.
mkdir -p "$STAGE/usr/share/zenith/shell/Widgets"
cp "$REPO_ROOT/shell/"*.qml "$STAGE/usr/share/zenith/shell/"
cp "$REPO_ROOT/shell/Widgets/"*.qml "$STAGE/usr/share/zenith/shell/Widgets/"

# The Hub's window.
#
# Config.qml is copied in beside it as well. Quickshell's -p sets the import path
# for one window and nothing else, so the Hub cannot see the singleton the shell
# uses from its own directory — and it uses `config.dark` like every other
# surface. One source file, two installed copies, rather than a second file that
# could drift from the first.
mkdir -p "$STAGE/usr/share/zenith/hub/Controls"
cp "$REPO_ROOT/shell/hub/"*.qml "$STAGE/usr/share/zenith/hub/"
cp "$REPO_ROOT/shell/hub/Controls/"*.qml "$STAGE/usr/share/zenith/hub/Controls/"
cp "$REPO_ROOT/shell/Config.qml" "$STAGE/usr/share/zenith/hub/"
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
mkdir -p "$STAGE/usr/share/doc/$PKG"
cp "$REPO_ROOT/LICENSE" "$STAGE/usr/share/doc/$PKG/copyright"

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
EXECUTABLES
chmod 0755 "$STAGE/DEBIAN/postinst"
chmod 0640 "$STAGE/DEBIAN/control"
chmod 0644 "$STAGE/DEBIAN/zenith.list"

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
