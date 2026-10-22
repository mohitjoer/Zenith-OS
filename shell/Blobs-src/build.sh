#!/usr/bin/env bash
# build the Zenith.Blobs QML plugin, install the module onto a QML import path.
#
#   build.sh <qml-import-dir>
#
# drops <qml-import-dir>/Zenith/Blobs/{libzenith-blobs.so, qmldir, *.qsb, ...}.
# Quickshell picks it up once that dir is on QML2_IMPORT_PATH (zenith-shell does
# that for the surfaces it supervises). target has no build toolchain, so this
# runs on the dev box (deploy.sh) and the ISO build host (iso/build.sh), never
# on the installed machine. build deps: cmake, ninja, qt6-shadertools, a c++20
# compiler, Qt6 dev files.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
dest="${1:?usage: build.sh <qml-import-dir>}"
build="${ZENITH_BLOBS_BUILD:-$here/build}"

for tool in cmake ninja; do
  command -v "$tool" >/dev/null 2>&1 || {
    printf 'build.sh: error: %s is required (pacman -S cmake ninja qt6-shadertools)\n' "$tool" >&2
    exit 1
  }
done

cmake -S "$here" -B "$build" -G Ninja -DCMAKE_BUILD_TYPE=Release
cmake --build "$build"

# self-contained module under <build>/qml/Zenith/Blobs. nuke any prior copy first
# so a renamed / removed file doesn't linger on the import path.
mkdir -p "$dest/Zenith"
rm -rf "$dest/Zenith/Blobs"
cp -a "$build/qml/Zenith/Blobs" "$dest/Zenith/Blobs"

# drop the generated .qrc dir-map: build byproduct, not a runtime file.
rm -f "$dest/Zenith/Blobs"/*.qrc
