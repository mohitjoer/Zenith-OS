#!/bin/sh
# Sync the vendored upstream component tree.
#
# The vendored files are byte-identical to a pinned upstream commit. That is
# the whole point: a vendored file is a shipped file, and the project's rule
# is that a shipped file is never edited in place. A change to a vendored
# component is a configuration value, an overlay file Zenith owns, or a patch
# in shell/vendor/patches/ — never a hand edit of a vendored file.
#
# This script is the only thing that writes into the vendored paths, and it
# writes exactly what upstream has. `zenith doctor` verifies the result
# against the hashes in the manifest, so a hand edit cannot survive.
#
# Usage:
#   sh shell/vendor/vendor.sh sync     # fetch upstream and write the tree
#   sh shell/vendor/vendor.sh verify   # check the tree against the manifest
#   sh shell/vendor/vendor.sh update ii <commit>   # move one pin, then sync
#
# The upstream repositories are cloned into a cache directory outside the
# tree, so nothing upstream owns ever lands in the repository except the
# files this script copies.

set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
VENDOR_DIR="$REPO_ROOT/shell/vendor"
CACHE="${ZENITH_VENDOR_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/zenith/vendor}"

# ---------------------------------------------------------------- the pins
#
# One line per component: name, repository, commit, path inside the
# repository, where it lands in the shell, and the licence file's name.
#
# The licence file's name is not always `LICENSE`, because the repository's
# own check requires every file named LICENSE to be byte-identical to the
# root licence. A component under a different licence therefore carries its
# text under a name that says which licence it is.
#
#   name|repo|url|commit|path-in-repo|destination|licence-file-name
#
II_NAME="ii"
II_REPO="end4"
II_URL="https://github.com/end-4/dots-hyprland.git"
II_COMMIT="2f2a376fdb2ad738bf55f693dd0b474b64bb0ab8"
II_PATH="dots/.config/quickshell/ii"
II_DEST="."
II_LICENCE="COPYING"

SHAPES_NAME="shapes"
SHAPES_REPO="rpoly"
SHAPES_URL="https://github.com/end-4/rounded-polygon-qmljs.git"
SHAPES_COMMIT="e31ec4cb4ebf6a46b267f5c42eabf6874916fa16"
SHAPES_PATH="."
SHAPES_DEST="modules/common/widgets/shapes"
SHAPES_LICENCE="LICENSE.apache-2.0"

# The paths taken from the "ii" component, and where each one lands in the
# shell.
#
# The destination is relative to shell/. Upstream's component files import
# each other with absolute module paths (`import qs.modules.common.widgets`),
# so the tree has to sit at the shell root's own `modules/` for those imports
# to resolve without a single edit to a vendored file.
#
# `modules/common/Appearance.qml` and `modules/common/Config.qml` are NOT in
# this list on purpose. They are the two files upstream's components read for
# their colours and their settings, and Zenith supplies its own at those same
# paths — a monochrome token bridge and a GSettings-backed configuration.
# That is the whole of the re-theme and the re-bind: no vendored file is
# modified, and upstream's components read Zenith's palette and Zenith's
# settings because they were told to.
II_PATHS="
modules/common
modules/ii/bar
modules/ii/dock
modules/ii/sidebarRight
modules/ii/overview
modules/ii/notificationPopup
modules/ii/onScreenDisplay
modules/ii/mediaControls
modules/ii/cheatsheet
modules/ii/background
modules/ii/lock
modules/ii/verticalBar
services
GlobalStates.qml
"

# Files inside a vendored path that Zenith supplies instead.
#
# These four are the whole of the re-theme and the re-bind, and they are the
# reason no vendored file needs a patch:
#
#   Appearance.qml  upstream's theme singleton. Zenith's version exposes the
#                   same property names, sourced from Zenith's monochrome
#                   tokens, so upstream's components draw in Zenith's palette
#                   without being edited.
#   Config.qml      upstream's settings singleton, which reads a JSON file of
#                   its own. Zenith's version exposes the same `options.*`
#                   shape, sourced from GSettings, so upstream's components
#                   read Zenith's settings store — and there is still exactly
#                   one settings store in the desktop.
#   GlobalStates.qml upstream's shell-root state singleton. It sits at the
#                   shell root, which is Zenith's, and it imports the whole
#                   service graph, which closes a cycle through the modules
#                   directory: the services reach back into the modules, and
#                   the modules reach back into the root. Zenith's version
#                   carries the same surface-open flags and nothing else.
#   AppIcon.qml     upstream's application-icon widget, which is a KDE
#                   Frameworks `Kirigami.Icon`. The base layer carries no
#                   KDE Frameworks, and one widget is not a reason to put an
#                   application framework on the desktop. Zenith's version
#                   has the same two properties — a source and a size — and
#                   nothing else.
#
# They are Zenith's files, in Zenith's tree, at upstream's paths. That is the
# overlay: the vendored components are upstream's, the context they run in is
# ours.
EXCLUDE_FILES="
modules/common/Appearance.qml
modules/common/Config.qml
modules/common/widgets/AppIcon.qml
GlobalStates.qml
"

log() { printf '%s\n' "$*" >&2; }

ensure_clone() {
    url=$1
    dir=$2
    if [ ! -d "$CACHE/$dir/.git" ]; then
        log "cloning $url into $CACHE/$dir"
        mkdir -p "$CACHE"
        git clone --filter=blob:none --no-checkout "$url" "$CACHE/$dir"
    fi
    git -C "$CACHE/$dir" remote set-url origin "$url" 2>/dev/null || true
}

fetch_pin() {
    ensure_clone "$II_URL" "$II_REPO"
    ensure_clone "$SHAPES_URL" "$SHAPES_REPO"
    log "fetching $II_COMMIT and $SHAPES_COMMIT"
    git -C "$CACHE/$II_REPO" fetch --depth 1 origin "$II_COMMIT" >/dev/null 2>&1 ||
        git -C "$CACHE/$II_REPO" fetch origin >/dev/null 2>&1
    git -C "$CACHE/$SHAPES_REPO" fetch --depth 1 origin "$SHAPES_COMMIT" >/dev/null 2>&1 ||
        git -C "$CACHE/$SHAPES_REPO" fetch origin >/dev/null 2>&1
    git -C "$CACHE/$II_REPO" cat-file -e "$II_COMMIT^{commit}"
    git -C "$CACHE/$SHAPES_REPO" cat-file -e "$SHAPES_COMMIT^{commit}"
}

is_excluded() {
    for e in $EXCLUDE_FILES; do
        [ "$e" = "$1" ] && return 0
    done
    return 1
}

# The list of upstream files a component consists of, one path per line,
# relative to the shell root. This is the manifest's file list and the input
# to the hash check.
#
# Gitlinks (mode 160000) are skipped: a submodule is not a file, and the
# one upstream carries is vendored as a component in its own right below.
ii_file_list() {
    for p in $II_PATHS; do
        git -C "$CACHE/$II_REPO" ls-tree -r --format='%(objectmode) %(path)' "$II_COMMIT" "$II_PATH/$p" 2>/dev/null |
        while read -r mode f; do
            [ -n "$f" ] || continue
            [ "$mode" = "160000" ] && continue
            rel=${f#"$II_PATH"/}
            is_excluded "$rel" && continue
            printf '%s\n' "$rel"
        done
    done | sort -u
}

shapes_file_list() {
    git -C "$CACHE/$SHAPES_REPO" ls-tree -r --name-only "$SHAPES_COMMIT" 2>/dev/null |
    while IFS= read -r f; do
        case "$f" in
            LICENSE|README.md|.gitignore|example*.qml) continue ;;
        esac
        printf 'modules/common/widgets/shapes/%s\n' "$f"
    done | sort -u
}

cmd_sync() {
    fetch_pin
    staging=$(mktemp -d)
    trap 'rm -rf "$staging"' EXIT

    # Zenith supplies the files that carry the design, the settings and the
    # shell's own state. They live at upstream's paths, so they are set aside
    # BEFORE the copy and put back after it — otherwise the copy overwrites
    # the very files that make the copy work.
    overlay=$(mktemp -d)
    for e in $EXCLUDE_FILES; do
        if [ -f "$REPO_ROOT/shell/$e" ]; then
            mkdir -p "$overlay/$(dirname "$e")"
            cp -a "$REPO_ROOT/shell/$e" "$overlay/$e"
        fi
    done

    # --- the "ii" component ------------------------------------------------
    #
    # Upstream lives at dots/.config/quickshell/ii/, so four path components
    # are stripped and the rest lands at the shell root's own layout.
    git -C "$CACHE/$II_REPO" archive --format=tar "$II_COMMIT" \
        $(for p in $II_PATHS; do printf '%s ' "$II_PATH/$p"; done) |
        tar -x -C "$staging" --strip-components=4

    for p in $II_PATHS; do
        src="$staging/$p"
        [ -e "$src" ] || continue
        dest="$REPO_ROOT/shell/$p"
        rm -rf "$dest"
        mkdir -p "$(dirname "$dest")"
        cp -a "$src" "$dest"
    done

    # The shapes component. Upstream's own submodule, vendored as a
    # component in its own right because `MaterialShape.qml` — one of the
    # most used widgets in the kit — imports it.
    mkdir -p "$staging/shapes"
    git -C "$CACHE/$SHAPES_REPO" archive --format=tar "$SHAPES_COMMIT" |
        tar -x -C "$staging/shapes"
    rm -rf "$REPO_ROOT/shell/$SHAPES_DEST"
    mkdir -p "$(dirname "$REPO_ROOT/shell/$SHAPES_DEST")"
    cp -a "$staging/shapes" "$REPO_ROOT/shell/$SHAPES_DEST"
    rm -f "$REPO_ROOT/shell/$SHAPES_DEST/LICENSE" \
          "$REPO_ROOT/shell/$SHAPES_DEST/README.md" \
          "$REPO_ROOT/shell/$SHAPES_DEST/.gitignore" \
          "$REPO_ROOT/shell/$SHAPES_DEST/example.qml" \
          "$REPO_ROOT/shell/$SHAPES_DEST/example-squircle.qml"

    # Put Zenith's own files back.
    for e in $EXCLUDE_FILES; do
        if [ -f "$overlay/$e" ]; then
            mkdir -p "$(dirname "$REPO_ROOT/shell/$e")"
            cp -a "$overlay/$e" "$REPO_ROOT/shell/$e"
        fi
    done
    rm -rf "$overlay"

    # --- the licences ------------------------------------------------------
    #
    # A vendored file is upstream's, and upstream's licence travels with it.
    mkdir -p "$REPO_ROOT/shell/vendor"
    git -C "$CACHE/$II_REPO" show "$II_COMMIT:LICENSE" > "$REPO_ROOT/shell/vendor/$II_LICENCE"
    git -C "$CACHE/$SHAPES_REPO" show "$SHAPES_COMMIT:LICENSE" > "$REPO_ROOT/shell/vendor/$SHAPES_LICENCE"

    write_manifest
    log "vendored $(ii_file_list | wc -l) + $(shapes_file_list | wc -l) files"
}

# The manifest. Machine-readable, and the input to `zenith doctor`'s
# byte-identity check: if a vendored file is edited by hand, its hash stops
# matching and the doctor reports it.
write_manifest() {
    out="$VENDOR_DIR/manifest.yaml"
    {
        echo "# The vendored upstream component tree."
        echo "#"
        echo "# Generated by shell/vendor/vendor.sh. Do not edit by hand: the"
        echo "# hashes below are what \`zenith doctor\` checks the tree against."
        echo "schema: 1"
        echo "components:"
        echo "  - name: $II_NAME"
        echo "    upstream_name: end-4/dots-hyprland"
        echo "    upstream_url: $II_URL"
        echo "    upstream_path: $II_PATH"
        echo "    pinned_commit: $II_COMMIT"
        echo "    licence: GPL-3.0-only"
        echo "    licence_file: shell/vendor/$II_LICENCE"
        echo "    copyright: Copyright (C) the end-4/dots-hyprland contributors"
        echo "    spdx: GPL-3.0-only"
        echo "    notices: shell/vendor/notices.md"
        echo "    files:"
        ii_file_list | while IFS= read -r f; do
            blob=$(git -C "$CACHE/$II_REPO" rev-parse "$II_COMMIT:$II_PATH/$f" 2>/dev/null || echo "-")
            echo "      - path: $f"
            echo "        blob: $blob"
        done
        echo "  - name: $SHAPES_NAME"
        echo "    upstream_name: end-4/rounded-polygon-qmljs"
        echo "    upstream_url: $SHAPES_URL"
        echo "    upstream_path: $SHAPES_PATH"
        echo "    pinned_commit: $SHAPES_COMMIT"
        echo "    licence: Apache-2.0"
        echo "    licence_file: shell/vendor/$SHAPES_LICENCE"
        echo "    copyright: Copyright the end-4/rounded-polygon-qmljs contributors"
        echo "    spdx: Apache-2.0"
        echo "    notices: shell/vendor/notices.md"
        echo "    files:"
        shapes_file_list | while IFS= read -r f; do
            rel=${f#"modules/common/widgets/shapes/"}
            blob=$(git -C "$CACHE/$SHAPES_REPO" rev-parse "$SHAPES_COMMIT:$rel" 2>/dev/null || echo "-")
            echo "      - path: $f"
            echo "        blob: $blob"
        done
    } > "$out"
    log "wrote $out"
}

cmd_verify() {
    if [ ! -f "$VENDOR_DIR/manifest.yaml" ]; then
        log "no manifest; nothing to verify"
        return 1
    fi
    bad=0
    total=0
    # shellcheck disable=SC2162
    while read -r path blob; do
        [ -n "$path" ] || continue
        total=$((total + 1))
        f="$REPO_ROOT/shell/$path"
        if [ ! -f "$f" ]; then
            log "MISSING  $path"
            bad=$((bad + 1))
            continue
        fi
        # git hash-object gives the same blob id git stores.
        have=$(git hash-object "$f")
        if [ "$have" != "$blob" ]; then
            log "MODIFIED $path"
            bad=$((bad + 1))
        fi
    done <<EOF
$(awk '
    /^      - path: / { p=$3; getline b; sub(/.*blob: /,"",b); print p, b }
' "$VENDOR_DIR/manifest.yaml")
EOF
    if [ "$bad" -eq 0 ]; then
        log "ok - $total vendored files are byte-identical to the pinned commits"
        return 0
    fi
    log "FAIL - $bad of $total vendored files differ from the pinned commits"
    return 1
}

cmd_update() {
    [ -n "${1:-}" ] || { log "usage: vendor.sh update <component> <commit>"; exit 2; }
    case "$1" in
        "$II_NAME") II_COMMIT=$2 ;;
        "$SHAPES_NAME") SHAPES_COMMIT=$2 ;;
        *) log "unknown component: $1"; exit 2 ;;
    esac
    fetch_pin
    cmd_sync
}

case "${1:-}" in
    sync)   cmd_sync ;;
    verify) cmd_verify ;;
    update) cmd_update "${2:-}" "${3:-}" ;;
    *)      log "usage: vendor.sh sync|verify|update <component> <commit>"; exit 2 ;;
esac
