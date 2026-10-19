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

mkdir -p "$REPO/dists/$SUITE/$COMPONENT/binary-$ARCH"

echo "indexing $(ls -1 "$DEBS"/*.deb | wc -l) package(s)"

# Packages first, then the Contents and the Release file that references them.
( cd "$DEBS" && dpkg-scanpackages --multiversion . /dev/null \
    2>/dev/null | gzip -9n > "$REPO/dists/$SUITE/$COMPONENT/binary-$ARCH/Packages.gz" )

# No Contents file. It maps installed file paths to packages, which apt does not
# need for a repository, and a recompression of Packages labelled as Contents is
# worse than none: it is wrong, and nothing reading it would notice.
#
# -n keeps mtimes out of the archive so re-indexing an unchanged set of packages
# produces an identical file. Without it the repository's own contents change
# every time it is rebuilt and a mirror cannot tell a real change from a clock
# tick.

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
if command -v apt-ftparchive >/dev/null 2>&1; then
    apt-ftparchive \
        -o APT::FTPArchive::Release::Origin="Zenith OS" \
        -o APT::FTPArchive::Release::Label="Zenith OS" \
        -o APT::FTPArchive::Release::Suite="$SUITE" \
        -o APT::FTPArchive::Release::Components="$COMPONENT" \
        -o APT::FTPArchive::Release::Architectures="$ARCH" \
        release "$REPO/dists/$SUITE" > "$REPO/dists/$SUITE/Release"
else
    # Without apt-ftparchive the Release file is assembled by hand from the same
    # checksums apt would verify. It is less pretty, and it is only exactly as
    # good if it carries what apt requires of a Release file:
    #
    #   Date is not decoration. apt treats a Release without a parsable Date as
    #   an invalid release: it warns "Invalid 'Date' entry", downloads nothing
    #   the release describes, and exits zero — a repository that gpgv
    #   verifies perfectly and that delivers no packages at all. That failure
    #   was invisible to every check here until update_test.sh ran apt against
    #   a real repository, which is the only place it shows.
    #
    #   Checksum entries name paths relative to the suite directory with no
    #   leading "./". That is what apt-ftparchive emits and what apt looks up
    #   when it decides an index is covered by the hashes it just verified.
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
        find . -name '*.gz' -printf '%P\n' | sort | while read -r f; do
            printf ' %s %16d %s\n' "$(md5sum <"$f" | cut -d' ' -f1)" "$(stat -c%s "$f")" "$f"
        done; echo "SHA256:"; \
        find . -name '*.gz' -printf '%P\n' | sort | while read -r f; do
            printf ' %s %16d %s\n' "$(sha256sum <"$f" | cut -d' ' -f1)" "$(stat -c%s "$f")" "$f"
        done; } > Release )
fi

# The signature is detached so apt can verify the Release file without having to
# trust the transport.
gpg --batch --yes --quiet --armor --detach-sign \
    --output "$REPO/dists/$SUITE/Release.gpg" \
    "$REPO/dists/$SUITE/Release"

gpg --batch --yes --quiet --clearsign \
    --output "$REPO/dists/$SUITE/InRelease" \
    "$REPO/dists/$SUITE/Release"

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
