#!/bin/sh
# Build a bootable Zenith live ISO from the Ubuntu Server base and the reviewed
# Zenith package manifest.
#
# Usage: installation/build-iso.sh [build-directory]
#
# The script deliberately does not partition disks or install anything. The
# installer is a later M4 slice; this produces the bootable image that can be
# tested independently.
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BUILD_DIR=${1:-"$REPO_ROOT/build/iso"}
ARCH=${ZENITH_ARCH:-amd64}
SUITE=${ZENITH_SUITE:-resolute}
MIRROR=${ZENITH_MIRROR:-http://archive.ubuntu.com/ubuntu}
BASE_FLAVOR=ubuntu-server
ISO_NAME=zenith-os-beta-001
ISO_LABEL=ZENITH_OS_BETA_001
ISO_APPLICATION="Zenith OS"
ISO_PUBLISHER="Zenith OS Project"

for tool in lb python3; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "build-iso: required tool not found: $tool" >&2
		exit 2
	fi
done

if ! python3 -c 'import yaml' >/dev/null 2>&1; then
	echo "build-iso: PyYAML is required (python3 -c 'import yaml')" >&2
	exit 2
fi

mkdir -p "$BUILD_DIR"
BUILD_DIR=$(CDPATH= cd -- "$BUILD_DIR" && pwd)
DEB_DIR="$BUILD_DIR/debs"
mkdir -p "$DEB_DIR"

"$REPO_ROOT/release/build-debs.sh" "$DEB_DIR" >/dev/null
DEB=$(find "$DEB_DIR" -maxdepth 1 -type f -name 'zenith_*.deb' -print -quit)
if [ -z "$DEB" ]; then
	echo "build-iso: release/build-debs.sh produced no Zenith package" >&2
	exit 1
fi

mkdir -p "$BUILD_DIR/config/package-lists" "$BUILD_DIR/config/hooks/normal" \
	"$BUILD_DIR/config/includes.chroot/etc" \
	"$BUILD_DIR/config/includes.chroot/tmp" \
	"$BUILD_DIR/config/includes.chroot/usr/share/zenith/branding"
cp "$DEB" "$BUILD_DIR/config/includes.chroot/tmp/zenith.deb"
cp "$REPO_ROOT/public/zenith-logo.svg" \
	"$BUILD_DIR/config/includes.chroot/usr/share/zenith/branding/"
cp "$REPO_ROOT/public/zenith-logo-on-dark.svg" \
	"$BUILD_DIR/config/includes.chroot/usr/share/zenith/branding/"

# These are the live system's public identity. The Ubuntu base remains recorded
# in the repository manifest and package documentation; it must not leak into
# the installed OS name shown to users.
cat >"$BUILD_DIR/config/includes.chroot/etc/os-release" <<'EOF'
NAME="Zenith OS"
PRETTY_NAME="Zenith OS Beta 001"
ID=zenith
ID_LIKE=linux
VERSION_ID="0.1"
VERSION_CODENAME=beta-001
HOME_URL="https://zenithos.dev/"
SUPPORT_URL="https://zenithos.dev/support"
BUG_REPORT_URL="https://github.com/mohitjoer/Zenith-OS/issues"
EOF
printf 'zenith-os\n' >"$BUILD_DIR/config/includes.chroot/etc/hostname"
cat >"$BUILD_DIR/config/includes.chroot/etc/motd" <<'EOF'
Zenith OS Beta 001

Welcome to Zenith OS.
EOF

# Modern Ubuntu archive does not publish gfxboot-theme-ubuntu or
# syslinux-themes-ubuntu-oneiric, causing live-build's lb_binary_syslinux stage
# to fail with exit code 100 when it attempts to apt-get install them.  The
# dummy stubs below are kept for older live-build versions that check file
# existence, but the real fix is forcing the syslinux theme to "live-build"
# (via --syslinux-theme) so lb_binary_syslinux never requests those obsolete
# Ubuntu theme packages in the first place.
mkdir -p "$BUILD_DIR/config/includes.chroot/usr/share/gfxboot-theme-ubuntu"
tar -czf "$BUILD_DIR/config/includes.chroot/usr/share/gfxboot-theme-ubuntu/bootlogo.tar.gz" -T /dev/null
mkdir -p "$BUILD_DIR/config/includes.chroot/usr/share/syslinux/themes/ubuntu-server/isolinux-live" \
	"$BUILD_DIR/config/includes.chroot/usr/share/syslinux/themes/ubuntu/isolinux-live"
# Resolute's syslinux-common moved vesamenu.c32 to
# /usr/lib/syslinux/modules/bios/ and isolinux package uses
# /usr/lib/ISOLINUX/isolinux.bin, but live-build's bootloaders/isolinux
# symlinks still point at /usr/lib/syslinux/*.  Without compat links the
# lb_binary_syslinux "cp -aL /root/isolinux" dereference fails with
# "cannot stat isolinux.bin/vesamenu.c32" (run 40095280254).
cat >"$BUILD_DIR/config/hooks/normal/0000-fix-syslinux-compat.chroot" <<'HOOK_EOF'
#!/bin/sh
set -eu
mkdir -p /usr/lib/syslinux
if [ ! -e /usr/lib/syslinux/isolinux.bin ]; then
	if [ -e /usr/lib/ISOLINUX/isolinux.bin ]; then
		ln -sf /usr/lib/ISOLINUX/isolinux.bin /usr/lib/syslinux/isolinux.bin
	elif [ -e /usr/lib/syslinux/modules/bios/isolinux.bin ]; then
		ln -sf /usr/lib/syslinux/modules/bios/isolinux.bin /usr/lib/syslinux/isolinux.bin
	fi
fi
if [ ! -e /usr/lib/syslinux/vesamenu.c32 ]; then
	if [ -e /usr/lib/syslinux/modules/bios/vesamenu.c32 ]; then
		ln -sf /usr/lib/syslinux/modules/bios/vesamenu.c32 /usr/lib/syslinux/vesamenu.c32
	elif [ -e /usr/lib/syslinux/vesamenu.c32 ]; then
		:
	fi
fi
# Ensure rsvg wrapper exists in chroot for live-build theme conversions
if [ -e /usr/bin/rsvg-convert ] && [ ! -e /usr/bin/rsvg ]; then
	cat > /usr/bin/rsvg <<'RSVG_CHROOT_EOF'
#!/bin/bash
if [ $# -ge 2 ]; then
	last="${!#}"
	prev="${@: -2:1}"
	case "$last" in *.png)
		case "$prev" in *.svg)
			out="$last"
			exec /usr/bin/rsvg-convert "${@:1:$#-1}" -o "$out"
			;;
		esac
		;;
	esac
fi
exec /usr/bin/rsvg-convert "$@"
RSVG_CHROOT_EOF
	chmod 0755 /usr/bin/rsvg
fi
HOOK_EOF
chmod 0755 "$BUILD_DIR/config/hooks/normal/0000-fix-syslinux-compat.chroot"
# Provide a corrected config/bootloaders/isolinux so lb_binary_syslinux's
# _SOURCE already points at resolute-correct targets (belt-and-suspenders
# with the chroot hook above).
if [ -d /usr/share/live/build/bootloaders/isolinux ]; then
	mkdir -p "$BUILD_DIR/config/bootloaders"
	cp -r /usr/share/live/build/bootloaders/isolinux "$BUILD_DIR/config/bootloaders/"
	# Fix dangling symlinks to match resolute layout
	if [ -L "$BUILD_DIR/config/bootloaders/isolinux/isolinux.bin" ]; then
		ln -sf /usr/lib/ISOLINUX/isolinux.bin "$BUILD_DIR/config/bootloaders/isolinux/isolinux.bin"
	fi
	if [ -L "$BUILD_DIR/config/bootloaders/isolinux/vesamenu.c32" ]; then
		ln -sf /usr/lib/syslinux/modules/bios/vesamenu.c32 "$BUILD_DIR/config/bootloaders/isolinux/vesamenu.c32"
	fi
else
	# Fallback if host has no live-build bootloaders (e.g. local test)
	mkdir -p "$BUILD_DIR/config/bootloaders/isolinux"
	ln -sf /usr/lib/ISOLINUX/isolinux.bin "$BUILD_DIR/config/bootloaders/isolinux/isolinux.bin"
	ln -sf /usr/lib/syslinux/modules/bios/vesamenu.c32 "$BUILD_DIR/config/bootloaders/isolinux/vesamenu.c32"
fi
# Keep dummy stubs for LB_BUILD_WITH_CHROOT=false fallback, but the real
# fix is --syslinux-theme live-build passed to lb config below.

python3 - "$REPO_ROOT/system/packages.yaml" \
	"$BUILD_DIR/config/package-lists/zenith.list.chroot" <<'PY'
import sys
from pathlib import Path
import yaml

manifest = yaml.safe_load(Path(sys.argv[1]).read_text())
if manifest.get("base", {}).get("flavor") != "ubuntu-server":
    raise SystemExit("build-iso: system/packages.yaml must declare flavor: ubuntu-server")
packages = []
for group, body in manifest.items():
    if group in {"version", "base"} or not isinstance(body, dict):
        continue
    entries = body.get("packages")
    if isinstance(entries, list):
        packages.extend(entries)
    elif isinstance(entries, dict):
        for values in entries.values():
            if isinstance(values, list):
                packages.extend(values)

# live-build verifies the chroot's APT metadata with gpg during package
# installation; installing it only on the host does not provide it here.
packages.append("gnupg")
# Syslinux/isolinux modules, bootloaders, and zenith runtime dependencies.
packages.extend([
    "isolinux", "syslinux-common",
    "grub-pc-bin", "grub-efi-amd64-bin",
    "curl", "file", "imagemagick", "jq", "libnotify-bin",
    "openssh-client", "pkexec", "procps", "qml6-module-qtmultimedia", "util-linux",
])
out = sorted(dict.fromkeys(str(package) for package in packages))
Path(sys.argv[2]).write_text("\n".join(out) + "\n")
PY

cat >"$BUILD_DIR/config/hooks/normal/9999-install-zenith.chroot" <<'EOF'
#!/bin/sh
set -eu
if [ ! -s /etc/resolv.conf ] || grep -q '127.0.0.53' /etc/resolv.conf; then
	printf 'nameserver 8.8.8.8\nnameserver 1.1.1.1\n' >/etc/resolv.conf
fi
DEBIAN_FRONTEND=noninteractive apt-get update || true
DEBIAN_FRONTEND=noninteractive apt-get install -y linux-image-generic /tmp/zenith.deb
rm -f /tmp/zenith.deb
EOF
chmod 0755 "$BUILD_DIR/config/hooks/normal/9999-install-zenith.chroot"

# librsvg2-bin on resolute ships /usr/bin/rsvg-convert, but live-build's
# lb_binary_syslinux calls "rsvg --format png ... in.svg out.png" (run
# 40099593317: env: 'rsvg': No such file). rsvg-convert requires "-o out.png"
# instead of positional output, so a symlink alone fails with
# "Multiple SVG files are only allowed for PDF and (E)PS output."
# This wrapper runs on the HOST because lb_binary_syslinux runs there.
if [ -e /usr/bin/rsvg-convert ] && [ ! -e /usr/bin/rsvg ]; then
	cat > /usr/bin/rsvg <<'RSVG_EOF'
#!/bin/bash
# Compat wrapper: translate "rsvg [opts] in.svg out.png" -> "rsvg-convert [opts] in.svg -o out.png"
if [ $# -ge 2 ]; then
	last="${!#}"
	prev="${@: -2:1}"
	case "$last" in *.png)
		case "$prev" in *.svg)
			out="$last"
			exec /usr/bin/rsvg-convert "${@:1:$#-1}" -o "$out"
			;;
		esac
		;;
	esac
fi
exec /usr/bin/rsvg-convert "$@"
RSVG_EOF
	chmod 0755 /usr/bin/rsvg
fi

(
	cd "$BUILD_DIR"
	lb config \
		--architectures "$ARCH" \
		--distribution "$SUITE" \
		--archive-areas "main universe multiverse" \
		--parent-mirror-bootstrap "$MIRROR" \
		--parent-mirror-chroot "$MIRROR" \
		--parent-mirror-binary "$MIRROR" \
		--linux-packages linux-image \
		--linux-flavours generic \
		--initramfs live-boot \
		--initsystem systemd \
		--binary-images iso-hybrid \
		--bootloader syslinux \
		--syslinux-theme live-build \
		--iso-application "$ISO_APPLICATION" \
		--iso-publisher "$ISO_PUBLISHER" \
		--iso-volume "$ISO_LABEL" \
		--apt-recommends false \
		--security false \
		--memtest none
	lb build
)

ISO_PATH=$(find "$BUILD_DIR" -maxdepth 1 -type f \
	\( -name '*.iso' -o -name '*.hybrid.iso' \) -print -quit)
if [ -z "$ISO_PATH" ]; then
	echo "build-iso: live-build produced no ISO" >&2
	exit 1
fi
FINAL_ISO="$BUILD_DIR/$ISO_NAME.iso"
if [ "$ISO_PATH" != "$FINAL_ISO" ]; then
	mv "$ISO_PATH" "$FINAL_ISO"
fi
echo "build-iso: image written to $FINAL_ISO"
