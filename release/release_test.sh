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

# An assertion is written as
#
#     rc=0; <command> || rc=$?
#     check "what it was checking" $rc
#
# and not as the more obvious
#
#     <command>
#     check "what it was checking" $?
#
# because the second form cannot report a failure at all. Under `set -e` a bare
# failing command aborts the run on the spot, so `check` is never reached and the
# suite dies mid-output with no verdict. A suite that cannot print FAIL is a
# suite that reads as passing. `|| rc=$?` puts the command in an exempt context
# and carries its status to the check that reports it.
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
rc=0; [ "$A" = "$B" ] || rc=$?
check "two builds are byte-identical" $rc

# ---------------------------------------------------------------- boundary
echo "checking the boundary"
STAGE="$WORK/stage"
mkdir -p "$STAGE"
dpkg-deb -x "$WORK"/a/*.deb "$STAGE"
rc=0; "$REPO_ROOT/release/check-boundary.sh" "$STAGE" zenith >/dev/null 2>&1 || rc=$?
check "the package installs only inside the boundary" $rc

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
rc=0; [ -z "$WRONG" ] || rc=$?
check "installed files carry 0644, 0755 or directories at 0755" $rc

# The mode check above is deliberately loose — 0644 is fine for content — which
# means it cannot catch the one mode that matters most: something the system
# executes and cannot. The session entry point is that something, and a
# non-executable one fails on every login rather than at build time.
MODES=$(dpkg-deb -c "$WORK"/a/*.deb)

rc=0; echo "$MODES" | grep -q '^-rwxr-xr-x .*zenith-session$' || rc=$?
check "the session entry point is executable" $rc

rc=0; echo "$MODES" | grep -q '^-rwxr-xr-x .*/zenith$' || rc=$?
check "the control tool is executable" $rc

# The two that shipped broken. The generic mode check above passes on 0644, and
# a file that is non-executable in the .deb is executable in the checkout, so
# nothing else in this suite notices.
for program in zenith-hub zenith-greeter; do
    rc=0; echo "$MODES" | grep -q "^-rwxr-xr-x .*/$program\$" || rc=$?
    check "$program is executable" $rc
done

# Every path listed as executable in release/build-debs.sh, from the script
# rather than from this list, so a program added to one and forgotten in the
# other is a failure here instead of a 0644 on somebody's machine.
rc=0
while read -r program; do
    case "$program" in
        \#*|"") continue ;;
    esac
    echo "$MODES" | grep -q "^-rwxr-xr-x .*$program\$" || {
        echo "    not executable: $program" >&2
        rc=1
    }
done <<EOF
$(awk "/<<'EXECUTABLES'/{inside=1;next} inside&&/^EXECUTABLES\$/{exit} inside" \
    "$REPO_ROOT/release/build-debs.sh")
EOF
check "every path in the build's executable list ships executable" $rc

# ---------------------------------------------------------------- repository
echo "building the repository"
# One secret keyring, reused for both builds, exactly as a release process
# would. It sits beside the served tree rather than inside it.
SECRET="$WORK/.zenith-archive-secret.gpg"
rc=0
ZENITH_SECRET_KEYRING="$SECRET" \
    "$REPO_ROOT/release/mkrepo.sh" "$WORK/a" "$WORK/repo" >/dev/null 2>&1 || rc=$?
check "the repository builds" $rc

rc=0; [ -f "$WORK/repo/dists/stable/InRelease" ] || rc=$?
check "the repository publishes an InRelease" $rc

rc=0; gpgv --keyring "$WORK/repo/zenith-archive-keyring.gpg" \
     "$WORK/repo/dists/stable/Release.gpg" \
     "$WORK/repo/dists/stable/Release" >/dev/null 2>&1 || rc=$?
check "the shipped keyring validates the detached signature" $rc

rc=0; gpgv --keyring "$WORK/repo/zenith-archive-keyring.gpg" \
     "$WORK/repo/dists/stable/InRelease" >/dev/null 2>&1 || rc=$?
check "the shipped keyring validates InRelease" $rc

rc=0; grep -q '^Signed-By:' "$WORK/repo/zenith.sources" || rc=$?
check "the sources line names a keyring rather than a keyserver" $rc

# A Release file whose checksums do not match its indexes is exactly the failure
# the signature cannot catch, so it is checked here rather than assumed.
#
# The section is read as a section rather than with a fixed number of lines
# after the header: a -A5 that happens to cover today's four index files stops
# covering them the day a fifth arrives, and a check that quietly stops
# looking is a check that quietly passes.
ARCH=$(dpkg --print-architecture 2>/dev/null || echo amd64)
release_section() {
    sed -n '/^SHA256:/,/^[A-Za-z][A-Za-z0-9]*:/p' "$1" | sed '1d;$d'
}

rc=0; release_section "$WORK/repo/dists/stable/Release" | grep -q 'Packages.gz' \
    || rc=$?
check "the Release file carries checksums for the index" $rc

# Two properties of a Release file that a signature cannot vouch for, because a
# signature over a wrong Release verifies exactly like a signature over a right
# one. The index name is the one that matters: apt looks an index up by its
# uncompressed name, and a Release that names only the compressed copy leaves
# it with nothing to resolve against — a repository it reads, verifies, and
# then quietly takes no packages from. The Date is policy rather than
# mechanism: apt warns when it is missing, and Debian's own rules ask for one,
# but a Release without a Date is not what broke this repository. Both are
# checked here so neither can go missing unnoticed.
rc=0; grep -q '^Date: ' "$WORK/repo/dists/stable/Release" || rc=$?
check "the Release file carries a Date apt will accept" $rc

rc=0; release_section "$WORK/repo/dists/stable/Release" \
    | grep -q ' main/binary-' || rc=$?
check "index entries name suite-relative paths, as apt looks them up" $rc

# The uncompressed index has to be listed as well as the compressed one. apt
# looks an index up by its uncompressed name, and a Release that lists only
# the .gz leaves apt with a name it cannot resolve against — a repository it
# reads, verifies, and then quietly takes no packages from. This is the bug
# that one cost the most to find, because nothing about it looks wrong until
# apt is the one reading it.
rc=0; release_section "$WORK/repo/dists/stable/Release" \
    | grep -q " main/binary-$ARCH/Packages$" || rc=$?
check "the Release lists the uncompressed index as well" $rc

# And the file it names has to be there. A Release that lists an index the
# repository does not publish is a repository apt rejects.
rc=0; [ -s "$WORK/repo/dists/stable/main/binary-$ARCH/Packages" ] || rc=$?
check "the uncompressed index the Release names is published" $rc

# The packages have to be readable by the user apt fetches as, which is not
# root. A .deb built through mktemp is 0600, and a pool full of packages apt
# cannot open is a repository that verifies and then cannot install.
rc=0; [ -z "$(find "$WORK/repo/pool" -name '*.deb' ! -readable 2>/dev/null)" ] || rc=$?
check "the published packages are readable by every user" $rc

# The signing key has to survive a rebuild, or every rebuild invalidates every
# machine that already trusts it. This is a regression test for that: an earlier
# mkrepo.sh kept the secret key in a temporary GNUPGHOME, deleted it at the end
# of the run, and then tried to reuse it on the next one, failing at "no secret
# key" every time.
echo
echo "checking that a rebuild keeps the same key"
keyid_of() {
    gpg --batch --with-colons --import-options show-only --import "$1" 2>/dev/null \
        | awk -F: '/^fpr:/ {print $10; exit}'
}
FIRST_KEY=$(keyid_of "$WORK/repo/zenith-archive-keyring.gpg")

# The second build must not merely succeed; it must succeed with the same key.
# A rebuild that quietly makes a new one still produces a valid InRelease that
# verifies against its own new keyring, and every machine that trusted the old
# key now rejects the repository. Only the key comparison catches that.
rc=0; ZENITH_SECRET_KEYRING="$SECRET" \
    "$REPO_ROOT/release/mkrepo.sh" "$WORK/a" "$WORK/repo-again" >/dev/null 2>&1 || rc=$?
check "a second build signs" $rc

rc=0; [ -s "$WORK/repo-again/dists/stable/InRelease" ] || rc=$?
check "a second build publishes an InRelease" $rc

rc=0; [ "$FIRST_KEY" = "$(keyid_of "$WORK/repo-again/zenith-archive-keyring.gpg")" ] || rc=$?
check "a rebuild publishes the same key, not a new one" $rc

rc=0; gpgv --keyring "$WORK/repo-again/zenith-archive-keyring.gpg" \
    "$WORK/repo-again/dists/stable/InRelease" >/dev/null 2>&1 || rc=$?
check "a rebuilt repository verifies against its own keyring" $rc

rc=0; [ ! -e "$WORK/repo/.zenith-archive-secret.gpg" ] || rc=$?
check "the secret key is not inside the served tree" $rc

echo
if [ "$fail" -eq 0 ]; then
    echo "all release checks passed"
else
    echo "$fail release check(s) failed"
fi
exit $fail
