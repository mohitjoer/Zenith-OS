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
mkdir -p "$STAGE/usr/share/zenith/session"
cp "$REPO_ROOT/session/zenith-session" "$STAGE/usr/share/zenith/session/"
cp "$REPO_ROOT/session/zenith.desktop" "$STAGE/usr/share/zenith/session/"
cp "$REPO_ROOT/session/zenith-portals.conf" "$STAGE/usr/share/zenith/session/"
chmod 0755 "$STAGE/usr/share/zenith/session/zenith-session"

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
# The two files something executes. Everything else is content and stays 0644:
# a session entry point that is not executable is a login manager whose Exec=
# line fails, and it fails on every login rather than at build time.
chmod 0755 "$STAGE/usr/bin/zenith"
chmod 0755 "$STAGE/usr/share/zenith/session/zenith-session"
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
