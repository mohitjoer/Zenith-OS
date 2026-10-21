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

# ----------------------------------------------------------- the vendored tree
echo "checking the vendored tree"

# The shell's configuration is a vendored upstream tree, and all of it has to
# ship: the vendored components import each other through the shell root's own
# modules/ and services/ directories, so a package that installed only the
# files at the shell's root would install a shell whose every import fails to
# resolve — a shell that comes up and then renders nothing.
#
# The check is against the repository, file by file, rather than against a
# count. A count says the package shipped some of the tree; it does not say
# which, and a tree that lost a directory still has files in it.
rc=0
while read -r file; do
    [ -n "$file" ] || continue
    if [ ! -f "$STAGE/usr/share/zenith/shell/$file" ]; then
        echo "    missing from the package: $file" >&2
        rc=1
    fi
done <<EOF
$(cd "$REPO_ROOT/shell" && find modules services -type f | sort)
EOF
check "every file of the vendored tree is installed" $rc

# And the bytes. A packaging step that rewrites a vendored file — a line
# ending, a reindentation, a licence header — produces a file that is still a
# file and no longer the thing upstream published. The manifest's hashes stop
# matching, and the doctor reports a tree nobody edited.
rc=0
while read -r file; do
    [ -n "$file" ] || continue
    if [ ! -f "$STAGE/usr/share/zenith/shell/$file" ]; then
        continue
    fi
    cmp -s "$REPO_ROOT/shell/$file" "$STAGE/usr/share/zenith/shell/$file" || {
        echo "    differs from the repository: $file" >&2
        rc=1
    }
done <<EOF
$(cd "$REPO_ROOT/shell" && find modules services -type f | sort)
EOF
check "every vendored file is byte-identical to the repository" $rc

# The manifest, which is what the doctor's byte-identity check reads. A
# package that ships the tree without the manifest installs a check that reads
# nothing, and a doctor that reads nothing looks exactly like a doctor that
# found nothing.
rc=0
cmp -s "$REPO_ROOT/shell/vendor/manifest.yaml" \
       "$STAGE/usr/share/zenith/shell/vendor/manifest.yaml" || rc=$?
check "the vendored tree's manifest is installed" $rc

# The attribution. It is installed at the path the manifest names and in the
# documentation directory, and the two are the same file: a copy that can
# differ from its original is a copy of something else. It is the repository's
# own notices file and not one rendered from the manifest, because the
# per-file credits inside the tree — the MIT and Apache-2.0 code that other
# projects contributed to it — are facts the manifest does not carry.
rc=0
cmp -s "$STAGE/usr/share/zenith/shell/vendor/notices.md" \
       "$STAGE/usr/share/doc/zenith/vendor-manifest.md" || rc=$?
check "the attribution manifest is installed in both places" $rc

# ------------------------------------------------------------------ licences
echo "checking the licences"

# The texts. GPL-3.0 section 4 asks for a copy of the licence with the work,
# and the Apache licence asks for its own, and dpkg's own tools and
# licensecheck look for them in the documentation directory.
rc=0
for text in GPL-3 Apache-2.0; do
    if [ ! -s "$STAGE/usr/share/doc/zenith/$text" ]; then
        echo "    missing: usr/share/doc/zenith/$text" >&2
        rc=1
    fi
done
check "the licence texts are installed" $rc

# One GPL-3 text, not two. The vendored component's copy of the licence and
# the repository's own are the same document, and a package that ships two
# different GPL-3 texts is shipping a licence nobody has read.
rc=0
cmp -s "$REPO_ROOT/LICENSE" "$STAGE/usr/share/doc/zenith/GPL-3" || rc=$?
check "the GPL-3 text the package ships is the repository's own" $rc

# The copyright file, and what it claims.
#
# The format resolves a file to the last paragraph whose Files: patterns match
# it, so a paragraph meant to override another one has to come after it. This
# reads the file the way the format says to read it and asks it about every
# file the package installs.
#
# The licence each file is claimed under comes from the manifest: a file the
# manifest lists belongs to the component that lists it, and a file it does
# not list is Zenith's own. The short names are the format's, and the mapping
# from the manifest's SPDX identifiers to them is written out rather than
# derived, because the format's names are the format's business — an
# identifier this table does not know is a failure, not a guess.
manifest_claims() {
    awk '
    function short_name(id) {
        if (id == "GPL-3.0-only") return "GPL-3"
        if (id == "GPL-3.0-or-later") return "GPL-3+"
        if (id == "Apache-2.0") return "Apache-2.0"
        return ""
    }
    /^  - name: / { spdx = ""; next }
    /^    spdx: / { spdx = $0; sub(/^    spdx: /, "", spdx); next }
    /^      - path: / {
        path = $0
        sub(/^      - path: /, "", path)
        name = short_name(spdx)
        if (name == "") {
            printf "unknown licence identifier: %s\n", spdx > "/dev/stderr"
            exit 1
        }
        printf "usr/share/zenith/shell/%s\t%s\n", path, name
    }
    ' "$REPO_ROOT/shell/vendor/manifest.yaml"
}

# What the copyright file says about each path it is asked about. One parse
# of the file, then one answer per path, in the order the paths arrived.
copyright_claims() {
    awk -v copyright="$STAGE/usr/share/doc/zenith/copyright" '
    function glob_to_regex(pattern,   out, i, c) {
        out = "^"
        for (i = 1; i <= length(pattern); i++) {
            c = substr(pattern, i, 1)
            if (c == "*") out = out ".*"
            else if (c == "?") out = out "."
            else if (c ~ /[A-Za-z0-9_\/-]/) out = out c
            else out = out "\\" c
        }
        return out "$"
    }
    function add(stanza, text,   n, parts, i) {
        n = split(text, parts, /[ \t]+/)
        for (i = 1; i <= n; i++) {
            if (parts[i] == "") continue
            count[stanza]++
            pattern[stanza SUBSEP count[stanza]] = parts[i]
        }
    }
    BEGIN {
        stanza = 0
        while ((getline line < copyright) > 0) {
            if (line ~ /^Files:/) {
                stanza++
                in_files = 1
                text = line
                sub(/^Files:[ \t]*/, "", text)
                add(stanza, text)
                continue
            }
            if (in_files == 1 && line ~ /^[ \t]/) {
                add(stanza, line)
                continue
            }
            in_files = 0
            if (line ~ /^License:/) {
                text = line
                sub(/^License:[ \t]*/, "", text)
                if (stanza > 0) licence[stanza] = text
                continue
            }
        }
        close(copyright)
    }
    {
        answer = "none"
        for (s = 1; s <= stanza; s++) {
            for (i = 1; i <= count[s]; i++) {
                if ($0 ~ glob_to_regex(pattern[s SUBSEP i])) {
                    answer = licence[s]
                    break
                }
            }
        }
        printf "%s\t%s\n", $0, answer
    }
    '
}

MANIFEST_PATHS="$WORK/manifest-paths"
manifest_claims | cut -f1 | sort -u > "$MANIFEST_PATHS"

EXPECTED="$WORK/expected-licences"
{
    manifest_claims
    ( cd "$STAGE" && find usr/share/zenith/shell -type f ) |
        grep -Fxv -f "$MANIFEST_PATHS" |
        while IFS= read -r file; do
            printf '%s\t%s\n' "$file" "GPL-3+"
        done
} > "$EXPECTED"

rc=0
cut -f1 "$EXPECTED" | copyright_claims > "$WORK/actual-licences" || rc=1
if ! diff -u "$EXPECTED" "$WORK/actual-licences" > "$WORK/licences.diff" 2>&1; then
    rc=1
    cat "$WORK/licences.diff" >&2
fi
check "the copyright file claims every installed file under the right licence" $rc

# Every licence group, and no others. The package is under three: Zenith's
# own GPL-3.0-or-later, the vendored GPL-3.0-only component, and the vendored
# Apache-2.0 component. A copyright file that names one licence for a package
# under three tells a reader the package is under a licence it is not entirely
# under.
rc=0
named=$(grep '^License: ' "$STAGE/usr/share/doc/zenith/copyright" |
    sed 's/^License: //' | sort -u | tr '\n' ' ')
if [ "$named" != "Apache-2.0 GPL-3 GPL-3+ " ]; then
    echo "    the copyright file names: $named" >&2
    echo "    it should name:        Apache-2.0 GPL-3 GPL-3+ " >&2
    rc=1
fi
check "the copyright file names every licence group, and no others" $rc

# The format itself, so a file that is not in it is not read by the tools
# that read it.
rc=0
grep -q '^Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/$' \
    "$STAGE/usr/share/doc/zenith/copyright" || rc=$?
check "the copyright file declares the Debian copyright format" $rc

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

# The converse, and the one that actually caught a bug.
#
# The list above is written by hand in the build script, and a file that is
# executable in the repository and missing from that list ships as 0644. The
# checkout says one thing, the package says another, and only the machine
# finds out — at login, in a session that shows a compositor and no desktop.
#
# So the set of executables is derived from the repository instead: every
# file the build installs that is executable in the checkout has to be
# executable in the package. A new program is executable in the checkout by
# being executable in the checkout, and this asks the package to agree.
#
# Only the session tree, because only the session tree is installed under the
# names it has in the repository. The build scripts are executable and are not
# installed at all, and a check that asked the package about them would be
# asking about files the package does not have. The test scripts are the same:
# they run from a checkout and are not installed either, and they are named
# for it.
rc=0
while read -r program; do
    case "$program" in
        \#*|"") continue ;;
        *_test.sh) continue ;;
    esac
    echo "$MODES" | grep -q "^-rwxr-xr-x .*/$program\$" || {
        echo "    not executable in the package: $program" >&2
        rc=1
    }
done <<EOF
$(find "$REPO_ROOT/session" -type f -perm -u+x 2>/dev/null \
    | sed "s|^$REPO_ROOT/session/||" | sort)
EOF
check "every executable file in the repository ships executable" $rc

# The shell's tree is installed under the names it has in the repository, so
# the same rule applies to it. A vendored script upstream ships executable and
# the package ships non-executable is a script that works in the checkout and
# fails on the machine, and the checkout is where it gets tested.
rc=0
while read -r program; do
    case "$program" in
        \#*|"") continue ;;
    esac
    echo "$MODES" | grep -q "^-rwxr-xr-x .*/usr/share/zenith/shell/$program\$" || {
        echo "    not executable in the package: $program" >&2
        rc=1
    }
done <<EOF
$(find "$REPO_ROOT/shell/modules" "$REPO_ROOT/shell/services" -type f -perm -u+x 2>/dev/null \
    | sed "s|^$REPO_ROOT/shell/||" | sort)
EOF
check "every executable file in the shell's tree ships executable" $rc

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
#
# It ends at the next stanza header, and at end of file when there is none. A
# SHA256 section that runs to EOF is what the hand-written Release carries —
# nothing follows it — and a helper that unconditionally dropped its last line
# dropped the only Packages.gz entry in it, so on a machine without
# apt-ftparchive this check failed for a reason that had nothing to do with the
# Release file. The end pattern is a header; a checksum entry starts with a
# space and can never match it.
ARCH=$(dpkg --print-architecture 2>/dev/null || echo amd64)
release_section() {
    awk '/^SHA256:/{in_section=1; next}
         in_section && /^[A-Za-z][A-Za-z0-9]*:/{exit}
         in_section{print}' "$1"
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
