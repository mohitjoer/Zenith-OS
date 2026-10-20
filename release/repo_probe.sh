#!/bin/sh
# A probe, not a test: it publishes several candidate repository shapes and
# asks apt, once per shape, whether it can read them.
#
# The bug being chased is a signed repository that apt fetches, verifies,
# commits and then silently takes no packages from. Each variant below changes
# one thing about the shape, so a run that reports which variants apt can read
# says which thing was the cause — in one CI run, rather than one run per
# hypothesis.
#
# Usage: sh release/repo_probe.sh <path-to-a-deb>
set -eu

DEB=${1:?usage: repo_probe.sh <path-to-a-deb>}
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

VERSION=$(dpkg-deb -f "$DEB" Version)
ARCH=$(dpkg --print-architecture)
GNUPGHOME="$WORK/gnupg"
export GNUPGHOME
mkdir -p "$GNUPGHOME"
chmod 700 "$GNUPGHOME"

gpg --batch --quiet --passphrase '' \
    --quick-generate-key "Zenith OS probe <zenith@localhost>" rsa3072 sign never \
    >/dev/null 2>&1
KEYID=$(gpg --batch --with-colons --list-keys | awk -F: '/^fpr:/ {print $10; exit}')
gpg --batch --quiet --export "$KEYID" > "$WORK/keyring.gpg"

# build_repo <dir> <pool> <uncompressed> <binary-all>
#
# pool=yes         the .deb lives in pool/main/ and the index names that path
# pool=no          the .deb lives at the repository root
# uncompressed=yes the Release lists the index under both names
# uncompressed=no  the Release lists only the compressed one
# binary-all=yes   an (empty) binary-all index is published
build_repo() {
    dir=$1; pool=$2; uncompressed=$3; binaryall=$4
    index="$dir/dists/stable/main/binary-$ARCH"

    mkdir -p "$index"
    if [ "$pool" = yes ]; then
        mkdir -p "$dir/pool/main"
        cp "$DEB" "$dir/pool/main/"
        ( cd "$dir/pool/main" && dpkg-scanpackages --multiversion . /dev/null ) \
            > "$index/Packages" || true
        sed -i 's|^Filename: \./|Filename: pool/main/|' "$index/Packages"
    else
        cp "$DEB" "$dir/"
        ( cd "$dir" && dpkg-scanpackages --multiversion . /dev/null ) \
            > "$index/Packages" || true
    fi
    if [ ! -s "$index/Packages" ]; then
        echo "probe: the index for $dir came out empty" >&2
        exit 1
    fi
    gzip -9n -c "$index/Packages" > "$index/Packages.gz"

    if [ "$binaryall" = yes ]; then
        mkdir -p "$dir/dists/stable/main/binary-all"
        : > "$dir/dists/stable/main/binary-all/Packages"
        gzip -9n -c "$dir/dists/stable/main/binary-all/Packages" \
            > "$dir/dists/stable/main/binary-all/Packages.gz"
    fi

    if [ "$uncompressed" = no ]; then
        rm -f "$index/Packages"
    fi
}

# write_release <dir> <tool|hand>
write_release() {
    dir=$1; writer=$2
    suite="$dir/dists/stable"
    if [ "$writer" = tool ]; then
        apt-ftparchive \
            -o APT::FTPArchive::Release::Origin="Zenith OS" \
            -o APT::FTPArchive::Release::Label="Zenith OS" \
            -o APT::FTPArchive::Release::Suite="stable" \
            -o APT::FTPArchive::Release::Codename="stable" \
            -o APT::FTPArchive::Release::Components="main" \
            -o APT::FTPArchive::Release::Architectures="$ARCH" \
            release "$suite" > "$suite/Release"
    else
        ( cd "$suite" && \
          { echo "Origin: Zenith OS"; echo "Label: Zenith OS"; echo "Suite: stable"; \
            echo "Codename: stable"; echo "Date: $(date -R -u)"; \
            echo "Components: main"; echo "Architectures: $ARCH"; \
            echo "Description: Zenith OS probe"; echo "MD5Sum:"; \
            find . -type f -path './main/*' -printf '%P\n' | sort | while read -r f; do
                printf ' %s %16d %s\n' "$(md5sum <"$f" | cut -d' ' -f1)" \
                    "$(stat -c%s "$f")" "$f"
            done; echo "SHA256:"; \
            find . -type f -path './main/*' -printf '%P\n' | sort | while read -r f; do
                printf ' %s %16d %s\n' "$(sha256sum <"$f" | cut -d' ' -f1)" \
                    "$(stat -c%s "$f")" "$f"
            done; } > "$suite/Release" )
    fi

    gpg --batch --yes --quiet --armor --detach-sign \
        --output "$suite/Release.gpg" "$suite/Release"
    gpg --batch --yes --quiet --clearsign \
        --output "$suite/InRelease" "$suite/Release"
}

# probe <name> <dir> <source-style> <signed>
#
# source-style: deb822 | oneline
# signed:       yes | trusted
probe() {
    name=$1; dir=$2; style=$3; trust=$4
    lists="$WORK/$name.lists"
    parts="$WORK/$name.parts"
    mkdir -p "$lists" "$parts"

    if [ "$style" = deb822 ]; then
        # A deb822 file is a .sources file, and apt reads those from
        # sourceparts. Handing one to sourcelist — which is a one-line
        # sources.list — makes apt reject it as an unknown type, and the
        # variant looks like a repository apt cannot read when it never read
        # it at all.
        {
            echo "Types: deb"
            echo "URIs: file://$dir"
            echo "Suites: stable"
            echo "Components: main"
            echo "Architectures: $ARCH"
            if [ "$trust" = yes ]; then
                echo "Signed-By: $WORK/keyring.gpg"
            else
                echo "Trusted: yes"
            fi
        } > "$parts/$name.sources"
        sourcelist=/dev/null
        sourceparts="$parts"
    else
        if [ "$trust" = yes ]; then
            echo "deb [signed-by=$WORK/keyring.gpg] file://$dir stable main" > "$WORK/$name.list"
        else
            echo "deb [trusted=yes] file://$dir stable main" > "$WORK/$name.list"
        fi
        sourcelist="$WORK/$name.list"
        sourceparts=/dev/null
    fi

    rc=0
    apt-get \
        -o Dir::Etc::sourcelist="$sourcelist" \
        -o Dir::Etc::sourceparts="$sourceparts" \
        -o Dir::State::lists="$lists" \
        -o APT::Get::List-Cleanup=0 \
        update > "$WORK/$name.update.log" 2>&1 || rc=$?

    candidate=$(apt-cache \
        -o Dir::Etc::sourcelist="$sourcelist" \
        -o Dir::Etc::sourceparts="$sourceparts" \
        -o Dir::State::lists="$lists" \
        policy zenith 2>/dev/null | sed -n 's/^ *Candidate: //p')

    if [ -n "$candidate" ] && [ "$candidate" != "(none)" ]; then
        echo "READ   $name (candidate $candidate)"
    else
        echo "IGNORE $name (apt saw no candidate; update exit $rc)"
        sed -n '1,6p' "$WORK/$name.update.log" | sed 's/^/         | /'
    fi
}

# ---------------------------------------------------------------- the variants
V="$WORK/variants"
mkdir -p "$V"

build_repo "$V/v1" yes yes yes;  write_release "$V/v1" tool
build_repo "$V/v2" yes yes yes;  write_release "$V/v2" hand
build_repo "$V/v3" yes yes no;   write_release "$V/v3" tool
build_repo "$V/v4" yes no yes;   write_release "$V/v4" tool
build_repo "$V/v5" no  yes yes;  write_release "$V/v5" tool
build_repo "$V/v6" yes yes yes;  write_release "$V/v6" tool

probe v1-pool-uncompressed-binaryall-tool-signed  "$V/v1" deb822 yes
probe v2-pool-uncompressed-binaryall-hand-signed  "$V/v2" deb822 yes
probe v3-no-binaryall-tool-signed                 "$V/v3" deb822 yes
probe v4-compressed-index-only-tool-signed        "$V/v4" deb822 yes
probe v5-no-pool-tool-signed                      "$V/v5" deb822 yes
probe v6-trusted-no-signature                     "$V/v6" deb822 trusted
probe v7-oneline-sources-signed                   "$V/v1" oneline yes

echo
echo "--- the Release each variant published (the first lines) ---"
for v in v1 v2 v3 v4; do
    echo "== $v"
    sed -n '1,12p' "$V/$v/dists/stable/Release" | sed 's/^/   /'
done

echo
echo "--- the machine's own apt sources are untouched ---"
ls -1 /etc/apt/sources.list.d/ | sed 's/^/   /'
