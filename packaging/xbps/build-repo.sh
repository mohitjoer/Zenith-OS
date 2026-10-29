#!/bin/sh
# Build Zenith XBPS binary package and repository index using xbps-create and xbps-rindex
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
BUILD_DIR="${1:-$REPO_ROOT/build/xbps-repo}"
VERSION="${ZENITH_VERSION:-0.1}"
BIN_DIR="${2:-$REPO_ROOT/build/bin}"

STAGE=$(mktemp -d /tmp/zenith-stage.XXXXXX)
cleanup() {
	rm -rf "$STAGE"
}
trap cleanup EXIT

echo "build-repo: staging Zenith files into $STAGE..."
mkdir -p \
	"$STAGE/usr/bin" \
	"$STAGE/usr/share/zenith/session" \
	"$STAGE/usr/share/zenith/runit/services" \
	"$STAGE/usr/share/applications" \
	"$STAGE/usr/share/wayland-sessions" \
	"$STAGE/usr/lib/udev/rules.d" \
	"$STAGE/etc/sv/zenith-sysd" \
	"$STAGE/usr/share/licenses/zenith"

# Build binaries if not provided in build/bin
if [ ! -f "$BIN_DIR/zenith" ]; then
	echo "build-repo: compiling Go binaries..."
	mkdir -p "$BIN_DIR"
	(
		cd "$REPO_ROOT/desktop"
		for cmd in zenith zenith-sysd zenith-userd zenith-hubd zenith-shell zenith-wallpaper zpm; do
			CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$BIN_DIR/$cmd" "./cmd/$cmd"
		done
	)
fi

# Install Go control plane daemons and utilities
for cmd in zenith zenith-sysd zenith-userd zenith-hubd zenith-shell zenith-wallpaper zpm; do
	install -m 755 "$BIN_DIR/$cmd" "$STAGE/usr/bin/$cmd"
done

# Install scripts and helpers
install -m 755 "$REPO_ROOT/system/helpers/prime-run" "$STAGE/usr/bin/prime-run"
install -m 755 "$REPO_ROOT/session/runit/zenith-user-services" "$STAGE/usr/bin/zenith-user-services"
install -m 755 "$REPO_ROOT/installation/zenith-installer.sh" "$STAGE/usr/bin/zenith-installer"
install -m 755 "$REPO_ROOT/session/zenith-session" "$STAGE/usr/share/zenith/session/zenith-session"
install -m 755 "$REPO_ROOT/session/zenith-activate" "$STAGE/usr/share/zenith/session/zenith-activate"

# Install desktop and wayland session entries
install -m 644 "$REPO_ROOT/session/applications/zenith-installer.desktop" "$STAGE/usr/share/applications/zenith-installer.desktop"
install -m 644 "$REPO_ROOT/session/zenith.desktop" "$STAGE/usr/share/wayland-sessions/zenith.desktop"

# Install user-level runit service templates
cp -r "$REPO_ROOT/system/runit/user-services/"* "$STAGE/usr/share/zenith/runit/services/"
find "$STAGE/usr/share/zenith/runit/services" -name "run" -exec chmod 755 {} +

# Install system runit service (zenith-sysd)
install -m 755 "$REPO_ROOT/system/runit/services/zenith-sysd/run" "$STAGE/etc/sv/zenith-sysd/run"

# Install udev rules and license
install -m 644 "$REPO_ROOT/system/udev/99-zenith-power.rules" "$STAGE/usr/lib/udev/rules.d/99-zenith-power.rules"
install -m 644 "$REPO_ROOT/LICENSE" "$STAGE/usr/share/licenses/zenith/LICENSE"

echo "build-repo: creating XBPS binary package..."
mkdir -p "$BUILD_DIR"
xbps-create \
	-A x86_64 \
	-n "zenith-${VERSION}_1" \
	-s "Zenith OS control plane, session daemons, and system integration" \
	-m "Zenith OS Project <dev@zenithos.dev>" \
	-l "MIT" \
	-H "https://zenithos.dev" \
	-D "hyprland quickshell seatd turnstile ghostty papirus-icon-theme font-awesome" \
	"$STAGE"

mv "zenith-${VERSION}_1.x86_64.xbps" "$BUILD_DIR/"

echo "build-repo: indexing repository at $BUILD_DIR..."
xbps-rindex -a "$BUILD_DIR"/zenith-*.xbps

echo "build-repo: repository ready at $BUILD_DIR:"
ls -lh "$BUILD_DIR"
