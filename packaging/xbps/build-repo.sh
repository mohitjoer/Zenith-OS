#!/bin/sh
# Build Zenith XBPS packages and repository index using void-packages / xbps-src
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
BUILD_DIR="${1:-$REPO_ROOT/build/xbps-repo}"
VOID_PACKAGES_DIR="${2:-/tmp/void-packages}"

echo "build-repo: updating XBPS tools in container..."
xbps-install -Syu -y xbps
xbps-install -yu -y
xbps-install -y git curl tar xz ca-certificates make gcc go

if [ ! -d "$VOID_PACKAGES_DIR" ]; then
	echo "build-repo: cloning void-packages..."
	git config --global http.postBuffer 524288000
	git config --global core.compression 0
	git clone --depth 1 --single-branch https://github.com/void-linux/void-packages.git "$VOID_PACKAGES_DIR" || \
	(sleep 5 && git clone --depth 1 https://github.com/void-linux/void-packages.git "$VOID_PACKAGES_DIR")
fi

cd "$VOID_PACKAGES_DIR"
echo "XBPS_ALLOW_CHROOT_BREAKOUT=yes" >> etc/conf

echo "build-repo: bootstrapping xbps-src..."
./xbps-src binary-bootstrap

echo "build-repo: staging Zenith XBPS templates..."
for pkgdir in "$REPO_ROOT/packaging/xbps"/*; do
	[ -d "$pkgdir" ] || continue
	pkgname=$(basename "$pkgdir")
	echo "  staging srcpkgs/$pkgname..."
	rm -rf "srcpkgs/$pkgname"
	cp -r "$pkgdir" "srcpkgs/$pkgname"
done

# Stage workspace checkout into masterdir for zenith package
mkdir -p masterdir/builddir/zenith-0.1
cp -r "$REPO_ROOT/." masterdir/builddir/zenith-0.1/

echo "build-repo: building package zenith..."
./xbps-src -N pkg zenith

echo "build-repo: indexing repository..."
mkdir -p "$BUILD_DIR"
cp -r hostdir/binpkgs/* "$BUILD_DIR/" 2>/dev/null || true
cd "$BUILD_DIR"
xbps-rindex -a *.xbps 2>/dev/null || true
ls -la

echo "build-repo: repository ready at $BUILD_DIR"
