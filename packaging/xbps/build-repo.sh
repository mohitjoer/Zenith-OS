#!/bin/sh
# Build Asura XBPS binary package and repository index using xbps-create and xbps-rindex
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
BUILD_DIR="${1:-$REPO_ROOT/build/xbps-repo}"
VERSION="${ASURA_VERSION:-0.1}"
BIN_DIR="${2:-$REPO_ROOT/build/bin}"

STAGE=$(mktemp -d /tmp/asura-stage.XXXXXX)
cleanup() {
	rm -rf "$STAGE"
}
trap cleanup EXIT

echo "build-repo: staging Asura files into $STAGE..."
mkdir -p \
	"$STAGE/usr/bin" \
	"$STAGE/usr/share/asura/session" \
	"$STAGE/usr/share/asura/runit/services" \
	"$STAGE/usr/share/applications" \
	"$STAGE/usr/share/wayland-sessions" \
	"$STAGE/usr/lib/udev/rules.d" \
	"$STAGE/etc/sv/asura-sysd" \
	"$STAGE/usr/share/licenses/asura"

# Build binaries if not provided in build/bin
if [ ! -f "$BIN_DIR/asura" ]; then
	echo "build-repo: compiling Go binaries..."
	mkdir -p "$BIN_DIR"
	(
		cd "$REPO_ROOT/desktop"
		for cmd in asura asura-sysd asura-userd asura-hubd asura-shell asura-wallpaper apm; do
			CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$BIN_DIR/$cmd" "./cmd/$cmd"
		done
	)
fi

# Install Go control plane daemons and utilities
for cmd in asura asura-sysd asura-userd asura-hubd asura-shell asura-wallpaper apm; do
	install -m 755 "$BIN_DIR/$cmd" "$STAGE/usr/bin/$cmd"
done

# Install scripts and helpers
install -m 755 "$REPO_ROOT/system/helpers/prime-run" "$STAGE/usr/bin/prime-run"
install -m 755 "$REPO_ROOT/session/runit/asura-user-services" "$STAGE/usr/bin/asura-user-services"
install -m 755 "$REPO_ROOT/installation/asura-installer.sh" "$STAGE/usr/bin/asura-installer"
install -m 755 "$REPO_ROOT/session/asura-session" "$STAGE/usr/share/asura/session/asura-session"
install -m 755 "$REPO_ROOT/session/asura-activate" "$STAGE/usr/share/asura/session/asura-activate"

# Install desktop and wayland session entries
install -m 644 "$REPO_ROOT/session/applications/asura-installer.desktop" "$STAGE/usr/share/applications/asura-installer.desktop"
install -m 644 "$REPO_ROOT/session/asura.desktop" "$STAGE/usr/share/wayland-sessions/asura.desktop"

# Install user-level runit service templates
cp -r "$REPO_ROOT/system/runit/user-services/"* "$STAGE/usr/share/asura/runit/services/"
find "$STAGE/usr/share/asura/runit/services" -name "run" -exec chmod 755 {} +

# Install system runit services (asura-sysd, scx-lavd, ananicy-cpp)
for svc_dir in "$REPO_ROOT/system/runit/services"/*; do
	if [ -d "$svc_dir" ]; then
		svc_name=$(basename "$svc_dir")
		mkdir -p "$STAGE/etc/sv/$svc_name"
		cp -r "$svc_dir"/* "$STAGE/etc/sv/$svc_name/"
		chmod +x "$STAGE/etc/sv/$svc_name/run" 2>/dev/null || true
	fi
done

# Install all system udev rules and license
for rule in "$REPO_ROOT/system/udev"/*.rules; do
	[ -f "$rule" ] && install -m 644 "$rule" "$STAGE/usr/lib/udev/rules.d/$(basename "$rule")"
done
install -m 644 "$REPO_ROOT/LICENSE" "$STAGE/usr/share/licenses/asura/LICENSE"

echo "build-repo: creating XBPS binary package..."
mkdir -p "$BUILD_DIR"
xbps-create \
	-A x86_64 \
	-n "asura-daemon-${VERSION}_1" \
	-s "Asura OS control plane, session daemons, and system integration" \
	-m "Asura OS Project <dev@asuraos.dev>" \
	-l "MIT" \
	-H "https://asuraos.dev" \
	-D "hyprland quickshell-asura seatd turnstile ghostty papirus-icon-theme font-awesome" \
	"$STAGE"

mv "asura-daemon-${VERSION}_1.x86_64.xbps" "$BUILD_DIR/"

echo "build-repo: indexing repository at $BUILD_DIR..."
xbps-rindex -a "$BUILD_DIR"/asura-daemon-*.xbps

echo "build-repo: repository ready at $BUILD_DIR:"
ls -lh "$BUILD_DIR"
