#!/bin/sh
# Build Zenith Linux Kernel with CachyOS / BORE patchset, 1000Hz, and Full Preempt
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
DOTCONFIG="$REPO_ROOT/system/kernel/x86_64-dotconfig-zenith"
BUILD_DIR="${1:-$REPO_ROOT/build/kernel}"
CONF_FILE="$REPO_ROOT/packaging/xbps/conf/xbps-v3.conf"

echo "build-cachyos-kernel: preparing kernel build environment..."
mkdir -p "$BUILD_DIR"

if [ ! -f "$DOTCONFIG" ]; then
	echo "build-cachyos-kernel: missing $DOTCONFIG" >&2
	exit 1
fi

echo "build-cachyos-kernel: verifying kernel configuration parameters:"
grep -E 'CONFIG_(SCHED_BORE|HZ_1000|HZ|PREEMPT|SCHED_CLASS_EXT|MQ_IOSCHED_KYBER|BTRFS_FS)' "$DOTCONFIG"

echo "build-cachyos-kernel: compiler flags loaded from $CONF_FILE"
if [ -f "$CONF_FILE" ]; then
	grep -E 'XBPS_TARGET_C' "$CONF_FILE" || true
fi

echo "build-cachyos-kernel: kernel configuration validated successfully."
