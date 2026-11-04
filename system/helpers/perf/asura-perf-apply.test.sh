#!/bin/sh
# Runs asura-perf-apply against throwaway config and cache dirs and checks which
# hyprctl calls it would make. Needs no compositor: --dry-run only prints them.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/cfg/asura" "$tmp/cache/asura"

run() { XDG_CONFIG_HOME=$tmp/cfg XDG_CACHE_HOME=$tmp/cache sh "$here/asura-perf-apply" --dry-run; }
check() { [ "$(run)" = "$2" ] || { echo "FAIL $1: got '$(run)'" >&2; exit 1; }; }

check "nothing set changes nothing" ""
printf '{"disableBlur": true}' >"$tmp/cfg/asura/performance.json"
check "disableBlur" "hyprctl keyword decoration:blur:enabled false"
printf '{"lowPowerMode": true}' >"$tmp/cfg/asura/performance.json"
check "lowPowerMode" "hyprctl keyword decoration:blur:enabled false
hyprctl keyword decoration:shadow:enabled false"
printf '{}' >"$tmp/cfg/asura/performance.json"
printf '{"saver": true}' >"$tmp/cache/asura/hypr-perf.json"
check "power saver" "hyprctl keyword decoration:blur:enabled false
hyprctl keyword decoration:shadow:enabled false"
echo "ok"
