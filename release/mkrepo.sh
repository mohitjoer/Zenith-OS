#!/bin/sh
# Build a signed APT repository from the packages in an output directory.
#
# The signature is the whole point. An unsigned repository is one apt will
# refuse by default since apt 1.1, and a repository signed by a key that is not
# shipped in the image is one a user has to be told to trust by hand — which is
# the moment most people paste a key from a web page into their keyring.
#
# So the key is generated into the repository's own tree, the keyring is
# installed with the image, and `sources.list.d` refers to it by `signed-by`
# path. Nothing else on the machine has to be trusted.
#
# Usage: release/mkrepo.sh <debs-dir> <repo-dir> [suite] [component]
set -eu

DEBS=${1:?usage: mkrepo.sh <debs-dir> <repo-dir> [suite] [component]}
REPO=${2:?missing repository directory}
SUITE=${3:-stable}
COMPONENT=${4:-main}

ARCH=$(dpkg --print-architecture 2>/dev/null || echo amd64)

# ------------------------------------------------------------------ the pool
# The packages live under pool/<component>/ and the index points at them by
# that path. A repository whose index names a file at its own root works, but
# it is not a repository a mirror can carry, and the name an index records is
# what apt resolves against the repository's own root.
POOL="$REPO/pool/$COMPONENT"
mkdir -p "$POOL"
rm -f "$POOL"/*.deb
cp "$DEBS"/*.deb "$POOL/"
# 0644, and deliberately so. A .deb comes out of build-debs.sh's mktemp at
# 0600, and cp preserves that. A package apt's _apt user cannot read is a
# package apt cannot install — a signed, verified, committed repository that
# yields nothing when the user asks for it.
chmod 0644 "$POOL"/*.deb

# ----------------------------------------------------------------- the index
# Two files per architecture: the index itself, and the compressed copy apt
# downloads. The uncompressed one is not decoration, and dropping it is the
# quietest way to publish a repository that does not work.
#
# apt looks an index up by its uncompressed name — main/binary-amd64/Packages
# — and the hash table it builds from the Release is keyed by the names the
# Release lists, with no aliasing from the compressed one to the uncompressed
# one. A Release that lists only Packages.gz therefore gives apt a name it
# cannot resolve, and apt responds by dropping the target with no warning and
# no error: it fetches the meta index, verifies the signature, commits it,
# and then takes no packages from it. That is the whole failure, and it cost
# more to find than anything else in this repository.
#
# The reason apt wants the uncompressed name is that it verifies the bytes it
# decompresses against the hash of the uncompressed file. A repository that
# publishes only the compressed index has no hash to check those bytes
# against, so apt has nothing to fetch.
#
# -n keeps mtimes out of the archive, so re-indexing an unchanged set of
# packages produces an identical file. Without it the repository's own contents
# change every time it is rebuilt and a mirror cannot tell a real change from a
# clock tick.
INDEX_ROOT="$REPO/dists/$SUITE/$COMPONENT"

echo "indexing $(ls -1 "$DEBS"/*.deb | wc -l) package(s)"

# One architecture, and only the one the machine publishes for. A binary-all
# index is not published: apt only asks for one when the source entry asks
# for the "all" architecture, and an index nothing asks for is a file in the
# tree that means nothing to anyone.
dir="$INDEX_ROOT/binary-$ARCH"
mkdir -p "$dir"

# Scanned from the repository root, so the index records the pool path the
# package actually occupies — the root apt resolves it against.
#
# dpkg-scanpackages' exit status is not allowed to end the run. The old
# version piped it into gzip, which took the pipeline's status and threw
# dpkg's away — an index that came out empty was published with no complaint.
# The index is checked instead, which is the thing that matters, and dpkg's
# own words stay on the terminal.
( cd "$REPO" && dpkg-scanpackages --multiversion "pool/$COMPONENT" /dev/null ) \
    > "$dir/Packages" || true
if [ ! -s "$dir/Packages" ]; then
    echo "mkrepo: the index came out empty; dpkg-scanpackages produced" \
         "nothing for $dir" >&2
    exit 1
fi
gzip -9n -c "$dir/Packages" > "$dir/Packages.gz"

# No Contents file. It maps installed file paths to packages, which apt does not
# need for a repository, and a recompression of Packages labelled as Contents is
# worse than none: it is wrong, and nothing reading it would notice.

# The key lives in two files, and the split between them is the whole design.
#
#   $KEYRING         public, ships with the image, is what sources.list.d names
#   $SECRET_KEYRING  private, signs, and is deliberately kept OUT of $REPO
#
# The secret keyring is not inside $REPO on purpose. $REPO is the tree that gets
# published, and a private key in a published tree is a published private key.
# It sits beside it, and ZENITH_SECRET_KEYRING points it somewhere else in CI.
#
# A key that already exists is reused: regenerating it would invalidate every
# machine that already trusts it, which is the exact failure the shipped keyring
# exists to prevent. This is only true because the secret half survives the
# first run — an earlier version kept the secret key in a temporary GNUPGHOME,
# deleted it at the end, and then claimed to reuse a key it had just destroyed.
# The second build failed at "no secret key" every time.
KEYRING="$REPO/zenith-archive-keyring.gpg"
SECRET_KEYRING=${ZENITH_SECRET_KEYRING:-"$REPO/../.zenith-archive-secret.gpg"}

if [ -z "${GNUPGHOME:-}" ]; then
    GNUPGHOME=$(mktemp -d)
    chmod 700 "$GNUPGHOME"
    export GNUPGHOME
    CREATED_HOME=1
fi

if [ ! -s "$SECRET_KEYRING" ]; then
    echo "generating a repository signing key"
    gpg --batch --quiet --passphrase '' \
        --quick-generate-key "Zenith OS archive signing key <zenith@localhost>" \
        rsa3072 sign never >/dev/null 2>&1
    KEYID=$(gpg --batch --with-colons --list-keys | awk -F: '/^fpr:/ {print $10; exit}')
    gpg --batch --quiet --export-secret-keys "$KEYID" > "$SECRET_KEYRING"
else
    echo "reusing the existing signing key"
    gpg --batch --quiet --import "$SECRET_KEYRING" >/dev/null 2>&1
    KEYID=$(gpg --batch --with-colons --list-secret-keys \
        | awk -F: '/^fpr:/ {print $10; exit}')
fi

if [ -z "$KEYID" ]; then
    echo "no signing key available; refusing to build an unsigned repository" >&2
    exit 1
fi

# The public keyring is rewritten every run from whatever key was found, so it
# always describes the key the Release file was actually signed with.
gpg --batch --quiet --export "$KEYID" > "$KEYRING"

# The Release file lists the hashes apt verifies before it trusts an index. It
# is generated with -o APT::FTPArchive::Release::Origin so apt knows what it is
# looking at.
apt-ftparchive 2>/dev/null >/dev/null || true

# The Release file is written outside the tree it describes and moved into
# place afterwards. A Release file that lists itself is a Release file whose
# checksum of itself is the checksum of a half-written file, and the entries
# for Release, Release.gpg and InRelease from a previous build are worse:
# they are hashes of files that are about to change, so the Release describes
# a tree that no longer exists.
STALE="$REPO/dists/$SUITE/Release"
rm -f "$STALE" "$REPO/dists/$SUITE/Release.gpg" "$REPO/dists/$SUITE/InRelease"
RELEASE=$(mktemp "${TMPDIR:-/tmp}/zenith-release.XXXXXX")
trap 'rm -f "$RELEASE"' EXIT

# Which writer produces the Release file. "auto" takes apt-ftparchive when it
# is installed and the hand-written path when it is not.
#
# The two are not interchangeable, so the choice is a value rather than an
# accident of what happens to be installed: the hand-written path is what a
# machine without apt-utils gets, and it has to be exercised on purpose, on a
# machine that has apt-ftparchive, or it is only ever tested by the machines
# that cannot run the test at all.
FORMAT=${ZENITH_RELEASE_FORMAT:-auto}
if [ "$FORMAT" = "auto" ]; then
    if command -v apt-ftparchive >/dev/null 2>&1; then
        FORMAT=tool
    else
        FORMAT=hand
    fi
fi

if [ "$FORMAT" = "tool" ]; then
    apt-ftparchive \
        -o APT::FTPArchive::Release::Origin="Zenith OS" \
        -o APT::FTPArchive::Release::Label="Zenith OS" \
        -o APT::FTPArchive::Release::Suite="$SUITE" \
        -o APT::FTPArchive::Release::Codename="$SUITE" \
        -o APT::FTPArchive::Release::Components="$COMPONENT" \
        -o APT::FTPArchive::Release::Architectures="$ARCH" \
        release "$REPO/dists/$SUITE" > "$RELEASE"
else
    # Without apt-ftparchive the Release file is assembled by hand from the same
    # checksums apt would verify. It is less pretty, and it is only exactly as
    # good if it carries what apt requires of a Release file:
    #
    #   Date is not decoration, but it is hygiene rather than the cause of
    #   anything. A Release without a parsable Date makes apt warn — "Invalid
    #   'Date' entry in Release file" — and Debian's own policy is that a
    #   Release carries one, because an archive whose metadata has no date
    #   cannot say when it was true. apt proceeds all the same. The fault that
    #   made this repository unreadable was a different one, and it is
    #   described above, where the index is written.
    #
    #   Checksum entries name paths relative to the suite directory with no
    #   leading "./", and name the uncompressed index as well as the
    #   compressed one. That is what apt-ftparchive emits and what apt looks
    #   up when it decides an index is covered by the hashes it just verified.
    #
    # Codename is not required of a hand-written file, and is carried because
    # every apt-ftparchive Release has one and a reader comparing suites
    # expects to find it.
    echo "apt-ftparchive is absent; writing the Release file by hand"
    ( cd "$REPO/dists/$SUITE" && \
      { echo "Origin: Zenith OS"; echo "Label: Zenith OS"; echo "Suite: $SUITE"; \
        echo "Codename: $SUITE"; \
        echo "Date: $(date -R -u)"; \
        echo "Components: $COMPONENT"; echo "Architectures: $ARCH"; \
        echo "Description: Zenith OS signed archive"; echo "MD5Sum:"; \
        find . -type f -printf '%P\n' | sort | while read -r f; do
            printf ' %s %16d %s\n' "$(md5sum <"$f" | cut -d' ' -f1)" "$(stat -c%s "$f")" "$f"
        done; echo "SHA256:"; \
        find . -type f -printf '%P\n' | sort | while read -r f; do
            printf ' %s %16d %s\n' "$(sha256sum <"$f" | cut -d' ' -f1)" "$(stat -c%s "$f")" "$f"
        done; } > "$RELEASE" )
fi

# 0644: the file apt and a mirror have to read. mktemp creates it 0600, and
# moving it into place would carry that mode with it.
chmod 0644 "$RELEASE"
mv "$RELEASE" "$STALE"

# The signatures are written under temporary names and moved into place
# afterwards, so a failure while signing leaves a repository with a Release
# and no signature — which apt refuses loudly — rather than a signature over
# nothing.
gpg --batch --yes --quiet --armor --detach-sign \
    --output "$REPO/dists/$SUITE/Release.gpg.tmp" \
    "$REPO/dists/$SUITE/Release"

gpg --batch --yes --quiet --clearsign \
    --output "$REPO/dists/$SUITE/InRelease.tmp" \
    "$REPO/dists/$SUITE/Release"

chmod 0644 "$REPO/dists/$SUITE/Release.gpg.tmp" "$REPO/dists/$SUITE/InRelease.tmp"
mv "$REPO/dists/$SUITE/Release.gpg.tmp" "$REPO/dists/$SUITE/Release.gpg"
mv "$REPO/dists/$SUITE/InRelease.tmp" "$REPO/dists/$SUITE/InRelease"

cat > "$REPO/zenith.sources" <<SOURCES
# Zenith OS signed repository.
#
# signed-by names the keyring file, so this line adds exactly one trusted key and
# nothing else. It does not widen the machine's trust to a whole keyserver.
Types: deb
URIs: file://$REPO
Suites: $SUITE
Components: $COMPONENT
Architectures: $ARCH
Signed-By: $KEYRING
SOURCES

if [ "${CREATED_HOME:-0}" -eq 1 ]; then
    rm -rf "$GNUPGHOME"
fi

echo
echo "repository at $REPO (suite $SUITE, component $COMPONENT)"
echo "key $KEYID"
echo "source line: cp $REPO/zenith.sources /etc/apt/sources.list.d/"
