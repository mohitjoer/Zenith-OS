#!/bin/sh
# Verify a staged package tree writes only where the base-layer boundary allows.
#
# The rule is that Zenith's packages write only to /usr/share/zenith/ and
# /etc/zenith/, and that every path outside them is on the closed sanctioned list
# in docs/architecture.md. A package that quietly gains a file another package
# owns is a fork of that package, and the cost of that arrives at the next rebase
# as a conflict nobody remembers the origin of.
#
# This runs on the staged tree, so it sees what dpkg-deb will actually emit
# rather than what the build script appears to copy.
#
# Usage: release/check-boundary.sh <staged-root> [package-name]
set -eu

STAGE=${1:?usage: check-boundary.sh <staged-root> [package-name]}
PKG=${2:-zenith}

# The closed list, as shell patterns over paths relative to the staging root.
# Everything outside /usr/share/zenith/ and /etc/zenith/ must be on the closed
# sanctioned list in docs/architecture.md. Each entry is a registration or a
# name PATH resolves: a file in a fixed directory that a third-party binary
# scans and nowhere else.
#
# /etc/sddm.conf.d/10-zenith.conf is sanctioned but never installed by the
# package: `zenith greeter enable` prints the command that copies it, because a
# login screen that fails to come up leaves nobody able to log in.
#
# The last three are the shell's helpers that its QML starts by bare name
# (Stash.qml runs `bash stash-install.sh`) and that therefore have to be in
# /usr/bin, under the names the code asks for.
ALLOWED="usr/share/zenith/*
etc/zenith/*
usr/share/doc/$PKG/*
usr/share/wayland-sessions/zenith.desktop
usr/share/xdg-desktop-portal/zenith-portals.conf
usr/share/glib-2.0/schemas/org.zenith.shell.gschema.xml
usr/share/glib-2.0/schemas/org.zenith.desktop.gschema.xml
usr/lib/systemd/user/$PKG-*.service
usr/lib/systemd/user/$PKG-*.timer
usr/lib/systemd/user/$PKG-session.target
usr/lib/systemd/user/${PKG}d.service
usr/share/applications/$PKG-*.desktop
usr/share/applications/$PKG-mimeapps.list
usr/bin/$PKG
usr/bin/$PKG-*
usr/bin/zpm
usr/bin/cliphist-thumbs.sh
usr/bin/localsend.sh
usr/bin/stash-*.sh"

LIST=$(mktemp)
VIOLATIONS=$(mktemp)
trap 'rm -f "$LIST" "$VIOLATIONS"' EXIT

# DEBIAN/ is the package's own control metadata, not something it installs.
# Everything that is not a directory counts, symlinks included: a link into
# /etc is as much a write there as a file.
find "$STAGE" ! -type d -printf '%P\n' | grep -v '^DEBIAN/' > "$LIST" || true

while read -r path; do
    [ -n "$path" ] || continue
    permitted=0
    while read -r pattern; do
        case "$path" in
            $pattern) permitted=1; break ;;
        esac
    done <<ALLOWED
$ALLOWED
ALLOWED
    if [ "$permitted" -eq 0 ]; then
        {
            echo "BOUNDARY VIOLATION: $PKG installs '$path', which is outside"
            echo "  /usr/share/zenith/ and /etc/zenith/ and not on the closed list"
            echo "  in this script. Either move it, or add the path to the sanctioned"
            echo "  list in docs/architecture.md with a reason, and to ALLOWED here."
        } >&2
        echo "$path" >> "$VIOLATIONS"
    fi
done < "$LIST"

total=$(wc -l < "$LIST" | tr -d ' ')
count=$(wc -l < "$VIOLATIONS" | tr -d ' ')

if [ "$count" -ne 0 ]; then
    echo "$PKG: $count of $total path(s) outside the boundary" >&2
    exit 1
fi

echo "$PKG: all $total installed path(s) are inside the boundary"
