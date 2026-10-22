#!/usr/bin/env bash
# Put the translation catalog where a dev checkout's surfaces look for it, so a
# locally run shell, Hub, app or installer speaks the same languages a packaged
# one does. Mirrors zenith/ui/install.sh; on a packaged system the same files
# land at /usr/share/zenith/i18n from the zenith-desktop PKGBUILD, which is the
# next candidate the runtimes try.
#
#   install.sh [<data-root>]     (default: ~/.local/share)
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
root="${1:-${XDG_DATA_HOME:-$HOME/.local/share}}"
dest="$root/zenith/i18n"

rm -rf "$dest"
mkdir -p "$dest"
install -m0644 "$here/langs.json" "$dest/langs.json"
install -m0644 "$here"/catalog/*.json "$dest/"
echo "installed the Zenith i18n catalog -> $dest"
