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

# The closed list, as path prefixes relative to the staging root. /usr/bin/$PKG is
# on the sanctioned list because PATH resolution looks nowhere else.
# Everything outside /usr/share/zenith/ and /etc/zenith/ must be on the closed
# sanctioned list in docs/architecture.md. Each entry is a registration: a file
# in a fixed directory that a third-party binary scans and nowhere else.
ALLOWED="usr/share/zenith/
etc/zenith/
# /etc/sddm.conf.d/10-zenith.conf is sanctioned but not installed yet; it lands
# with zenith-greeter, which does not exist yet.
usr/share/doc/$PKG/
usr/share/wayland-sessions/zenith.desktop
usr/share/xdg-desktop-portal/zenith-portals.conf
usr/bin/$PKG
usr/bin/$PKG-hub
usr/share/glib-2.0/schemas/org.zenith.shell.gschema.xml
usr/share/glib-2.0/schemas/org.zenith.desktop.gschema.xml
usr/lib/systemd/user/$PKG-shell.service
usr/lib/systemd/user/$PKG-portal.service
usr/lib/systemd/user/hyprland-session.target.d/10-zenith.conf"

LIST=$(mktemp)
VIOLATIONS=$(mktemp)
trap 'rm -f "$LIST" "$VIOLATIONS"' EXIT

# DEBIAN/ is the package's own control metadata, not something it installs.
find "$STAGE" -type f -printf '%P\n' | grep -v '^DEBIAN/' > "$LIST" || true

while read -r path; do
    [ -n "$path" ] || continue
    permitted=0
    while read -r prefix; do
        case "$path" in
            "$prefix"*) permitted=1; break ;;
        esac
    done <<ALLOWED
$ALLOWED
ALLOWED
    if [ "$permitted" -eq 0 ]; then
        {
            echo "BOUNDARY VIOLATION: $PKG installs '$path', which is outside"
            echo "  /usr/share/zenith/, /etc/zenith/, /usr/share/doc/$PKG/ and"
            echo "  /usr/bin/$PKG. Either move it, or add the path to the sanctioned"
            echo "  list in docs/architecture.md with a reason."
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
