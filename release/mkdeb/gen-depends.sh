#!/bin/sh
# The package's Depends, Recommends and Suggests, emitted from release/deps.d.
#
# Every lane that needs a program from the base layer says so once, in its own
# release/deps.d/<lane>.deps, as `depends|recommends|suggests PACKAGE  # why`.
# A relations list kept by hand in control.in is a second list, and it goes the
# way every second list goes: a lane adds a tool, the package that has to carry
# it is not updated, and the desktop works on the machine that built it.
#
# Three rules decide what a line becomes:
#
#   - The strongest word wins. A package one lane depends on and another only
#     recommends is a dependency; a recommendation and a suggestion is a
#     recommendation. Nothing is listed twice.
#   - Recommends is where the desktop's features come from, because apt installs
#     Recommends by default: one `apt install zenith` brings the screenshot,
#     clipboard and recorder tools, and `--no-install-recommends` gives the
#     bare session. Depends is only what the session cannot start without, or
#     what is small enough that a missing one is not worth a degraded mode.
#   - Heavy or disruptive things are Suggests, whatever the lane thought: the
#     VM engine (quickemu pulls in QEMU, over a hundred megabytes), a login
#     manager (installing sddm next to gdm3 asks which one is the default, and a
#     wrong answer is a machine that boots to a login screen with no theme), and
#     a second icon theme. The deps files say so themselves; this script does
#     not carry a list of exceptions.
#
# Quickshell is not here. It is not in the archive: Zenith builds it, and
# system/packages.yaml records that under `zenith_built`.
#
# Hyprland is pinned to the series the configuration was written against. Window
# and layer rule syntax changed between 0.53 and 0.54, so a newer Hyprland is a
# deliberate bump of this line and of session/hyprland/, not something apt does
# on a Tuesday.
#
# Usage: release/mkdeb/gen-depends.sh [--verify]
#
# --verify resolves every name against the machine's apt index and fails on a
# name the archive does not have, because a dependency that does not exist makes
# the package uninstallable, and nothing else would notice until someone
# installs it. A machine whose index does not carry the target suite (it cannot
# find hyprland) says so and skips the check instead of failing every build.
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# Always in Depends. The names without their versions are what keeps the lane
# files from listing them a second time.
FIXED='libc6
git
hyprland (>= 0.53.3)
hyprland (<< 0.54)'
FIXED_NAMES='libc6
git
hyprland'

for deps in "$REPO_ROOT"/release/deps.d/*.deps; do
    awk -v file="release/deps.d/$(basename "$deps")" '
        { sub(/[ \t]*#.*$/, "") }
        NF == 0 { next }
        ($1 == "depends" || $1 == "recommends" || $1 == "suggests") && NF == 2 && $2 ~ /^[a-z0-9][a-z0-9+.-]+$/ {
            print $1 "\t" $2
            next
        }
        {
            printf "%s: not `depends|recommends|suggests PACKAGE`: %s\n", file, $0 > "/dev/stderr"
            bad = 1
        }
        END { exit bad }
    ' "$deps"
done > "$WORK/declared"

declared() { awk -F'\t' -v kind="$1" '$1 == kind { print $2 }' "$WORK/declared" | sort -u; }

printf '%s\n' "$FIXED_NAMES" > "$WORK/claimed"
declared depends | grep -vxFf "$WORK/claimed" > "$WORK/depends" || true
cat "$WORK/depends" >> "$WORK/claimed"
declared recommends | grep -vxFf "$WORK/claimed" > "$WORK/recommends" || true
cat "$WORK/recommends" >> "$WORK/claimed"
declared suggests | grep -vxFf "$WORK/claimed" > "$WORK/suggests" || true

if [ "${1:-}" = "--verify" ]; then
    sort -u "$WORK/claimed" "$WORK/suggests" > "$WORK/names"
    # shellcheck disable=SC2046
    apt-cache policy $(cat "$WORK/names") 2>/dev/null | awk '
        /^[^ ].*:$/ { name = $0; sub(/:$/, "", name); next }
        /^  Candidate: / && $2 != "(none)" { print name }
    ' | sort -u > "$WORK/indexed" || true
    if grep -qx hyprland "$WORK/indexed"; then
        grep -vxFf "$WORK/indexed" "$WORK/names" > "$WORK/missing" || true
        if [ -s "$WORK/missing" ]; then
            printf 'gen-depends: not in this machine'"'"'s apt index (see release/deps.d): %s\n' \
                "$(tr '\n' ' ' < "$WORK/missing")" >&2
            exit 1
        fi
    else
        echo "gen-depends: this machine's apt index does not carry hyprland, so the" \
             "package names were not checked against the archive" >&2
    fi
fi

# A relation field: the first package after the field name, the rest on
# continuation lines, one each, so `dpkg-deb -I` and a diff both read.
field() {
    awk -v field="$1" '
        NR == 1 { printf "%s: %s", field, $0; next }
        { printf ",\n %s", $0 }
        END { if (NR) printf "\n" }
    ' "$2"
}

printf '%s\n' "$FIXED" "$(cat "$WORK/depends")" | grep -v '^$' > "$WORK/depends.all"
field Depends "$WORK/depends.all"
[ ! -s "$WORK/recommends" ] || field Recommends "$WORK/recommends"
[ ! -s "$WORK/suggests" ] || field Suggests "$WORK/suggests"
