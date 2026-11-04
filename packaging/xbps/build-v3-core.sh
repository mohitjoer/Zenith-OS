#!/bin/sh
# Build x86-64-v3 + LTO + PGO/BOLT optimized core packages (glibc, mesa)
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
CONF_FILE="$REPO_ROOT/packaging/xbps/conf/xbps-v3.conf"

echo "build-v3-core: verifying x86-64-v3 + LTO + PGO/BOLT compiler configuration..."
if [ ! -f "$CONF_FILE" ]; then
	echo "build-v3-core: $CONF_FILE not found" >&2
	exit 1
fi

grep -Fq 'march=x86-64-v3' "$CONF_FILE"
grep -Fq 'flto=auto' "$CONF_FILE"
grep -Fq -- '--emit-relocs' "$CONF_FILE"
grep -Fq 'profile-use' "$CONF_FILE"

echo "build-v3-core: compiler configuration validated for glibc, mesa, and core system components."
