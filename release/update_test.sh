#!/bin/sh
# The M2 exit criterion, exercised for real: `zenith update` against a stock
# base-layer image and a real, signed APT repository.
#
# Every other check in this repository runs the overlay model against a
# temporary tree with the fetch step handed in as a stub. That is the right way
# to test the code, and it is not the exit criterion. The criterion names three
# properties a user is promised — update lays down a base and an overlay, a
# user edit survives the next update, rollback restores the previous revision —
# and it names where they must hold: a stock image of the base layer, with a
# real APT repository answering the fetch. The fetch is exactly the part a stub
# cannot vouch for, because the failures it hides are apt's: a repository apt
# refuses to trust, a signature it will not verify, a dependency that resolves
# differently against a real archive, a postinst that behaves unlike the tree
# the tests materialize.
#
# What the run looks like:
#
#   1. Build two revisions of the package, the second from a source tree with a
#      visible change in it, so "the update arrived" is a fact on disk rather
#      than a claim about timestamps.
#   2. Serve revision 1 from a signed repository built by release/mkrepo.sh,
#      add it to the machine with signed-by, and let apt itself install the
#      package — the first acquisition, before any Zenith tool exists.
#   3. zenith update: revision recorded, base materialized, no drift.
#   4. Write a user edit, record it in user_edits/ the way the model requires.
#   5. Publish revision 2, run zenith update again: apt fetches through
#      zenith's own aptFetch, the base on disk becomes revision 2, and the user
#      edit is still where it was.
#   6. zenith rollback: the previous base comes back, the edit still stands.
#
# This installs a package, adds an apt source, and rewrites the invoking user's
# home directory. It refuses to run as anything but root, on a machine that
# already has zenith installed, or without ZENITH_M2_ALLOW_INSTALL=1 (or CI) to
# say the machine is disposable — a container, a VM, a runner. It cleans up
# after itself, but "cleans up after itself" is a courtesy and not a safety
# property; throw the machine away.
#
# Usage: sh release/update_test.sh   (root, disposable, ZENITH_M2_ALLOW_INSTALL=1)
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SOURCES=/etc/apt/sources.list.d/zenith-m2test.sources
BASE=/usr/share/zenith

# ---------------------------------------------------------------- refusal
# Each refusal names what the test would otherwise do, because a guard that
# says only "refusing" teaches the next reader nothing about what they just
# avoided.
refuse() {
    echo "update_test: $*" >&2
    exit 2
}

[ "$(id -u)" -eq 0 ] || \
    refuse "must run as root: the fetch step is apt and apt does not negotiate"
[ -n "${HOME:-}" ] || \
    refuse "HOME is unset, and the overlay is rooted at the invoking user's home"

if [ -z "${CI:-}" ] && [ -z "${ZENITH_M2_ALLOW_INSTALL:-}" ]; then
    refuse "this test installs a package, adds an apt source and rewrites a" \
           "home directory; run it in a disposable container or VM and set" \
           "ZENITH_M2_ALLOW_INSTALL=1 to say that is what you meant"
fi

# git is deliberately absent: nothing here runs it. The snapshot phase needs
# it, and by then the package is installed and git has arrived as its Depends —
# which is also what makes the snapshot assertion below a fact about the
# installed system rather than about this preflight.
missing=""
for tool in apt-get apt-cache dpkg dpkg-deb dpkg-scanpackages gpg go sed awk; do
    command -v "$tool" >/dev/null 2>&1 || missing="$missing $tool"
done
[ -z "$missing" ] || refuse "missing tools:$missing"

if dpkg-query -W -f='${Status}' zenith 2>/dev/null \
        | grep -q 'install ok installed'; then
    refuse "zenith is already installed here; refusing to take over a machine" \
           "that is in use"
fi
[ -e "$SOURCES" ] && refuse "$SOURCES already exists; refusing to overwrite it"
[ -e "$BASE" ] && \
    refuse "$BASE exists without a package owning it; this is a leftover, and" \
           "a leftover base would let the assertions below pass against it"

# ---------------------------------------------------------------- workspace
WORK=$(mktemp -d)
chmod 755 "$WORK" # apt's _apt user reads the file:// repository living here

cleanup() {
    # Every step is best-effort on purpose: a failure while cleaning up must
    # not mask the exit status that says whether the checks passed.
    rm -f "$SOURCES"
    dpkg --purge zenith >/dev/null 2>&1 || :
    rm -rf "$HOME/.config/zenith" "$HOME/.local/state/zenith" 2>/dev/null || :
    rm -rf "$WORK" 2>/dev/null || :
}
trap cleanup EXIT

# The check form is release/release_test.sh's, and for the same reason: under
# `set -e` a bare failing command aborts the run mid-output with no verdict,
# and a suite that cannot print FAIL reads as passing.
fail=0
check() {
    if [ "$2" = "0" ]; then
        echo "ok    — $1"
    else
        echo "FAIL  — $1"
        fail=$((fail + 1))
    fi
}

# ---------------------------------------------------------------- revisions
echo "building revision 1"
ZENITH_VERSION=0.1+m2test1 \
    sh "$REPO_ROOT/release/build-debs.sh" "$WORK/debs1" >/dev/null

# Revision 2 comes from a copy of the source with a visible marker added to a
# file the package ships. A version bump alone would make apt fetch something
# new while leaving the base byte-identical, and then "the update arrived" and
# "rollback restored the old base" would both be unfalsifiable — the two
# claims this whole script exists to test.
echo "preparing revision 2"
cp -a "$REPO_ROOT" "$WORK/src2"
echo '<!-- zenith-m2 revision 2 -->' >> "$WORK/src2/public/zenith-logo.svg"

echo "building revision 2"
ZENITH_VERSION=0.1+m2test2 \
    sh "$WORK/src2/release/build-debs.sh" "$WORK/debs2" >/dev/null

# ---------------------------------------------------------------- repository
# One secret keyring across both publications, exactly as a release process
# reuses its key: a repository that re-signs with a new key on every publish is
# one that has invalidated every machine that trusted the old one.
echo "publishing the signed repository"
ZENITH_SECRET_KEYRING="$WORK/archive-secret.gpg" \
    sh "$REPO_ROOT/release/mkrepo.sh" "$WORK/debs1" "$WORK/repo" >/dev/null
chmod 600 "$WORK/archive-secret.gpg" 2>/dev/null || :

echo "adding the repository to the machine"
cp "$WORK/repo/zenith.sources" "$SOURCES"

# The update log is printed unconditionally. On the failure this debugs, apt
# exits zero while quietly not delivering the index, and a log shown only on
# failure cannot show the difference between "read and empty" and "never
# fetched at all" — which is the whole question.
rc=0; apt-get update >"$WORK/apt-update.log" 2>&1 || rc=$?
cat "$WORK/apt-update.log"
check "apt accepts the signed repository" $rc

# Stronger than exit zero: the source has to appear in apt's own output. apt
# can exit 0 having fetched nothing from a source it never managed to read,
# and then "zenith: unable to locate" is the first symptom three steps later.
rc=0; grep -F -q "$WORK/repo" "$WORK/apt-update.log" || rc=$?
check "apt fetched from the zenith repository" $rc

rc=0; policy=$(apt-cache policy zenith 2>&1) || rc=$?
if [ "$rc" -ne 0 ] || ! printf '%s\n' "$policy" | grep -q 'Candidate: 0.1+m2test1'; then
    printf '%s\n' "$policy"
    echo "--- the index apt would have read ---"
    ls -la "$WORK/repo/dists/stable/main/binary-amd64/" 2>/dev/null || :
    zcat "$WORK/repo/dists/stable/main/binary-amd64/Packages.gz" 2>/dev/null \
        | head -n 20 || :
    rc=1
fi
check "the repository offers revision 1 as the candidate" $rc

# The first acquisition is plain apt, because that is what happens on a real
# machine: the base layer installs zenith, and only from the second fetch on
# does zenith install itself. Everything after this line goes through the
# tool's own aptFetch.
rc=0; DEBIAN_FRONTEND=noninteractive \
    apt-get install --yes zenith >"$WORK/apt-install.log" 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then
    echo "--- apt-get install zenith ---"
    cat "$WORK/apt-install.log"
fi
check "zenith installs from the signed repository" $rc

rc=0; command -v zenith >/dev/null 2>&1 || rc=$?
check "the control tool is on PATH" $rc

# ------------------------------------------------------------- first update
echo "running the first zenith update"
rc=0; out1=$(zenith update 2>&1) || rc=$?
[ "$rc" -eq 0 ] || printf '%s\n' "$out1"
check "the first zenith update succeeds" $rc

rc=0; printf '%s\n' "$out1" | grep -q '^updated to revision ' || rc=$?
check "the first update records a revision" $rc
rev1=$(printf '%s\n' "$out1" | sed -n 's/^updated to revision //p' | head -n 1)

rc=0; printf '%s\n' "$out1" \
    | grep -F -q '0 user edit(s), 0 stale, 0 missing' || rc=$?
check "the first update materializes with no drift" $rc

rc=0
[ -d "$BASE" ] && [ -f "$HOME/.config/zenith/branding/zenith-logo.svg" ] || rc=$?
check "update lays down a base and an overlay" $rc

rc=0; status1=$(zenith status 2>&1) || rc=$?
check "zenith status runs" $rc
rc=0; printf '%s\n' "$status1" | grep -F -q "revision:  $rev1" || rc=$?
check "status reports the recorded revision" $rc

# --------------------------------------------------------------- user edit
# Both halves, because the model has two: the content lives at the mirrored
# path (that is the file applications read — user_edits/ is invisible to
# them), and user_edits/ is the record that makes the next materialization
# treat it as the user's rather than as drift to be overwritten.
echo "writing a user edit"
EDIT='zenith-m2: this edit belongs to the user and must survive every update'
mkdir -p "$HOME/.config/zenith/user_edits/session"
printf '%s\n' "$EDIT" > "$HOME/.config/zenith/user_edits/session/zenith.desktop"
printf '%s\n' "$EDIT" > "$HOME/.config/zenith/session/zenith.desktop"

rc=0; status2=$(zenith status 2>&1) || rc=$?
check "status still runs after the edit" $rc
rc=0; printf '%s\n' "$status2" | grep -F -q 'edits:     1' || rc=$?
check "status counts the user edit" $rc

# ------------------------------------------------------------- second fetch
echo "publishing revision 2"
ZENITH_SECRET_KEYRING="$WORK/archive-secret.gpg" \
    sh "$REPO_ROOT/release/mkrepo.sh" "$WORK/debs2" "$WORK/repo" >/dev/null

echo "running the second zenith update"
rc=0; out2=$(zenith update 2>&1) || rc=$?
[ "$rc" -eq 0 ] || printf '%s\n' "$out2"
check "the second zenith update succeeds" $rc

rc=0; printf '%s\n' "$out2" | grep -q '^updated to revision ' || rc=$?
check "the second update records a revision" $rc
rev2=$(printf '%s\n' "$out2" | sed -n 's/^updated to revision //p' | head -n 1)
rc=0; [ -n "$rev2" ] && [ "$rev2" != "$rev1" ] || rc=$?
check "the new revision is not the old one" $rc

# The fetch happened or it did not, and dpkg is the witness. Every assertion
# about the base below rests on this one: without it, a base that never
# changed would still "have" revision 2's content trivially.
rc=0
[ "$(dpkg-query -W -f='${Version}' zenith 2>/dev/null)" = "0.1+m2test2" ] || rc=$?
check "apt fetched revision 2 through zenith's own fetch step" $rc

rc=0; grep -q 'zenith-m2 revision 2' "$BASE/branding/zenith-logo.svg" 2>/dev/null \
    || rc=$?
check "the base on disk is revision 2" $rc

rc=0; grep -q 'zenith-m2 revision 2' \
    "$HOME/.config/zenith/branding/zenith-logo.svg" 2>/dev/null || rc=$?
check "the config tree re-materialized from the new base" $rc

# The invariant the whole model exists for.
rc=0; printf '%s\n' "$out2" \
    | grep -F -q '1 user edit(s), 0 stale, 0 missing' || rc=$?
check "the user edit survives the update, with no drift" $rc

rc=0; grep -F -q "$EDIT" "$HOME/.config/zenith/session/zenith.desktop" 2>/dev/null \
    || rc=$?
check "the edit is still at the mirrored path" $rc

rc=0; [ -f "$HOME/.config/zenith/user_edits/session/zenith.desktop" ] || rc=$?
check "the edit's record is still in user_edits/" $rc

rc=0; status3=$(zenith status 2>&1) || rc=$?
check "status runs after the update" $rc
rc=0; printf '%s\n' "$status3" | grep -F -q "revision:  $rev2" || rc=$?
check "status reports the new revision" $rc
rc=0; printf '%s\n' "$status3" | grep -F -q "previous:  $rev1" || rc=$?
check "status names the revision rollback would return to" $rc
rc=0; printf '%s\n' "$status3" | grep -F -q 'edits:     1' || rc=$?
check "status still counts the user edit" $rc

# The git snapshot promised by the milestone, asserted where it lives: the
# config tree is under version control by the second update, because the first
# one ran before there was anything to snapshot.
rc=0; [ -d "$HOME/.config/zenith/.git" ] || rc=$?
check "the config tree is under git" $rc
rc=0
commits=$(git -C "$HOME/.config/zenith" rev-list --count HEAD 2>/dev/null) \
    || commits=0
[ "$commits" -ge 1 ] || rc=$?
check "a snapshot commit exists before the update ran" $rc

# ---------------------------------------------------------------- rollback
echo "running zenith rollback"
rc=0; out3=$(zenith rollback 2>&1) || rc=$?
[ "$rc" -eq 0 ] || printf '%s\n' "$out3"
check "zenith rollback succeeds" $rc

rc=0; printf '%s\n' "$out3" \
    | grep -F -q "rolled back to revision $rev1" || rc=$?
check "rollback names the revision it restored" $rc

# The preserved base is the previous content or it is nothing. Revision 2's
# marker being gone from the base is the only way to tell the difference from
# a rollback that restored nothing and reported success anyway.
rc=0
if grep -q 'zenith-m2 revision 2' "$BASE/branding/zenith-logo.svg" 2>/dev/null; then
    rc=1
fi
check "rollback restored the previous base" $rc

rc=0
if grep -q 'zenith-m2 revision 2' \
        "$HOME/.config/zenith/branding/zenith-logo.svg" 2>/dev/null; then
    rc=1
fi
check "the config tree was rebuilt from the restored base" $rc

rc=0; grep -F -q "$EDIT" "$HOME/.config/zenith/session/zenith.desktop" 2>/dev/null \
    || rc=$?
check "the user edit survives the rollback too" $rc

rc=0; status4=$(zenith status 2>&1) || rc=$?
check "status runs after the rollback" $rc
rc=0; printf '%s\n' "$status4" | grep -F -q "revision:  $rev1" || rc=$?
check "status reports the restored revision as current" $rc
rc=0; printf '%s\n' "$status4" | grep -F -q 'edits:     1' || rc=$?
check "the user edit is still counted after the rollback" $rc

echo
if [ "$fail" -eq 0 ]; then
    echo "the exit criterion holds: a stock base layer, a real signed" \
         "repository, and update, edit and rollback all behaved as promised"
else
    echo "$fail check(s) failed"
fi
exit $fail
