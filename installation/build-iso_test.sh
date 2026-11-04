#!/bin/sh
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SCRIPT="$REPO_ROOT/installation/build-iso.sh"

if [ ! -x "$SCRIPT" ]; then
	echo "build-iso_test: build-iso.sh is not executable" >&2
	exit 1
fi

if ! sh -n "$SCRIPT"; then
	echo "build-iso_test: shell syntax check failed" >&2
	exit 1
fi

grep -Fq 'system/packages.yaml' "$SCRIPT"
grep -Fq 'BASE_FLAVOR=ubuntu-server' "$SCRIPT"
grep -Fq 'ISO_NAME=asura-os-beta-001' "$SCRIPT"
grep -Fq 'public/asura-logo.svg' "$SCRIPT"
grep -Fq 'PRETTY_NAME="Asura OS Beta 001"' "$SCRIPT"
grep -Fq 'packages.append("gnupg")' "$SCRIPT"
grep -Fq 'config/hooks/normal/9999-install-asura.chroot' "$SCRIPT"
grep -Fq 'apt-get install -y linux-image-generic /tmp/asura.deb' "$SCRIPT"
grep -Fq -- '--parent-mirror-bootstrap "$MIRROR"' "$SCRIPT"
grep -Fq -- '--parent-mirror-chroot "$MIRROR"' "$SCRIPT"
grep -Fq -- '--parent-mirror-binary "$MIRROR"' "$SCRIPT"
grep -Fq -- '--linux-packages linux-image' "$SCRIPT"
grep -Fq -- '--linux-flavours generic' "$SCRIPT"
grep -Fq 'ISO_LABEL=ASURA_OS_BETA_001' "$SCRIPT"
grep -Fq -- '--iso-volume "$ISO_LABEL"' "$SCRIPT"
grep -Fq -- '--binary-images iso-hybrid' "$SCRIPT"
grep -Fq -- '--bootloader syslinux' "$SCRIPT"
grep -Fq -- '--syslinux-theme live-build' "$SCRIPT"
grep -Fq -- '--initramfs live-boot' "$SCRIPT"
grep -Fq -- '--initsystem systemd' "$SCRIPT"
grep -Fq '0000-fix-syslinux-compat.chroot' "$SCRIPT"
grep -Fq -- '--memtest none' "$SCRIPT"
echo "build-iso_test: ok"
