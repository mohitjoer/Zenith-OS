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
KEYID=${ZENITH_KEYID:-}

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

# A key that already exists is reused: regenerating it would invalidate every
# machine that already trusts it.
#
# GNUPGHOME has to stay alive until the signature is made, not merely until the
# public key is exported — signing needs the secret key, and removing the
# directory first leaves gpg with nothing to sign with.
KEYRING="$REPO/zenith-archive-keyring.gpg"
CREATED_KEY=0

if [ ! -s "$KEYRING" ]; then
    echo "generating a repository signing key"
    CREATED_KEY=1
fi

if [ -z "${GNUPGHOME:-}" ]; then
    GNUPGHOME=$(mktemp -d)
    chmod 700 "$GNUPGHOME"
    export GNUPGHOME
    CREATED_HOME=1
fi

if [ "$CREATED_KEY" -eq 1 ]; then
    gpg --batch --quiet --passphrase '' \
        --quick-generate-key "Zenith OS archive signing key <zenith@localhost>" \
        rsa3072 sign never >/dev/null 2>&1
    KEYID=$(gpg --batch --with-colons --list-keys | awk -F: '/^fpr:/ {print $10; exit}')
    gpg --batch --quiet --export "$KEYID" > "$KEYRING"
    echo "$KEYID" > "$REPO/.keyid"
else
    KEYID=$(cat "$REPO/.keyid" 2>/dev/null || echo "$KEYID")
    echo "reusing the existing signing key"
fi

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
    # checksums apt would verify. It is less pretty and exactly as good.
    echo "apt-ftparchive is absent; writing the Release file by hand"
    ( cd "$REPO/dists/$SUITE" && \
      { echo "Origin: Zenith OS"; echo "Label: Zenith OS"; echo "Suite: $SUITE"; \
        echo "Components: $COMPONENT"; echo "Architectures: $ARCH"; \
        echo "Description: Zenith OS signed archive"; echo "MD5Sum:"; \
        find . -name '*.gz' -printf './%P\n' | sort | while read -r f; do
            printf ' %s %16d %s\n' "$(md5sum <"$f" | cut -d' ' -f1)" "$(stat -c%s "$f")" "$f"
        done; echo "SHA256:"; \
        find . -name '*.gz' -printf './%P\n' | sort | while read -r f; do
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
