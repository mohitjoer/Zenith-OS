#!/bin/sh
# Check the packaging properties that are easy to lose and expensive to discover
# later: the package installs only inside the boundary, two builds are identical,
# and the repository signature validates with the keyring it ships.
#
# Usage: release/release_test.sh
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

fail=0
check() {
    if [ "$2" = "0" ]; then
        echo "ok    — $1"
    else
        echo "FAIL  — $1"
        fail=$((fail + 1))
    fi
}

# ------------------------------------------------------------------ builds
echo "building twice"
"$REPO_ROOT/release/build-debs.sh" "$WORK/a" >/dev/null 2>&1
"$REPO_ROOT/release/build-debs.sh" "$WORK/b" >/dev/null 2>&1

A=$(sha256sum "$WORK"/a/*.deb | cut -d' ' -f1)
B=$(sha256sum "$WORK"/b/*.deb | cut -d' ' -f1)
[ "$A" = "$B" ]
check "two builds are byte-identical" $?

# ---------------------------------------------------------------- boundary
echo "checking the boundary"
STAGE="$WORK/stage"
mkdir -p "$STAGE"
dpkg-deb -x "$WORK"/a/*.deb "$STAGE"
"$REPO_ROOT/release/check-boundary.sh" "$STAGE" zenith >/dev/null
check "the package installs only inside the boundary" $?

# A package that gains another package's file must be refused, or the check is
# decorative.
BAD="$WORK/bad"
mkdir -p "$BAD/usr/share/zenith" "$BAD/etc/NetworkManager"
touch "$BAD/usr/share/zenith/ok.conf" "$BAD/etc/NetworkManager/NetworkManager.conf"
if "$REPO_ROOT/release/check-boundary.sh" "$BAD" zenith >/dev/null 2>&1; then
    check "a boundary violation is refused" 1
else
    check "a boundary violation is refused" 0
fi

# ------------------------------------------------------------- permissions
echo "checking installed modes"
WRONG=$(dpkg-deb -c "$WORK"/a/*.deb | awk '$1 !~ /^drwxr-xr-x/ && $1 !~ /^-rw-r--r--/ && $1 !~ /^-rwxr-xr-x/ {print $1, $NF}')
[ -z "$WRONG" ]
check "installed files carry 0644, 0755 or directories at 0755" $?

# ---------------------------------------------------------------- repository
echo "building the repository"
"$REPO_ROOT/release/mkrepo.sh" "$WORK/a" "$WORK/repo" >/dev/null 2>&1

[ -f "$WORK/repo/dists/stable/InRelease" ]
check "the repository publishes an InRelease" $?

gpgv --keyring "$WORK/repo/zenith-archive-keyring.gpg" \
     "$WORK/repo/dists/stable/Release.gpg" \
     "$WORK/repo/dists/stable/Release" >/dev/null 2>&1
check "the shipped keyring validates the detached signature" $?

gpgv --keyring "$WORK/repo/zenith-archive-keyring.gpg" \
     "$WORK/repo/dists/stable/InRelease" >/dev/null 2>&1
check "the shipped keyring validates InRelease" $?

grep -q '^Signed-By:' "$WORK/repo/zenith.sources"
check "the sources line names a keyring rather than a keyserver" $?

# A Release file whose checksums do not match its indexes is exactly the failure
# the signature cannot catch, so it is checked here rather than assumed.
( cd "$WORK/repo/dists/stable" && \
  grep -A5 '^SHA256:' Release | grep 'Packages.gz' ) >/dev/null
check "the Release file carries checksums for the index" $?

echo
if [ "$fail" -eq 0 ]; then
    echo "all release checks passed"
else
    echo "$fail release check(s) failed"
fi
exit $fail
