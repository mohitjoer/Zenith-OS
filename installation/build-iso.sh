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
out = sorted(dict.fromkeys(str(package) for package in packages))
Path(sys.argv[2]).write_text("\n".join(out) + "\n")
PY

cat >"$BUILD_DIR/config/hooks/normal/9999-install-zenith.chroot" <<'EOF'
#!/bin/sh
set -eu
DEBIAN_FRONTEND=noninteractive apt-get install -y /tmp/zenith.deb
rm -f /tmp/zenith.deb
EOF
chmod 0755 "$BUILD_DIR/config/hooks/normal/9999-install-zenith.chroot"

(
	cd "$BUILD_DIR"
	lb config \
		--architectures "$ARCH" \
		--distribution "$SUITE" \
		--archive-areas "main universe multiverse" \
		--parent-mirror-bootstrap "$MIRROR" \
		--parent-mirror-chroot "$MIRROR" \
		--parent-mirror-binary "$MIRROR" \
		--binary-images iso-hybrid \
		--debian-installer live \
		--iso-application "$ISO_APPLICATION" \
		--iso-publisher "$ISO_PUBLISHER" \
		--iso-volume "$ISO_LABEL" \
		--apt-recommends false \
		--security false
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
