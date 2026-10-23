#!/usr/bin/env bash
# zenith-workspace against a stub hyprctl: Super+N must land on the Nth workspace
# of the CURRENT desktop (a desktop is a block of 10 ids), never on desktop 1.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

cat >"$tmp/hyprctl" <<'STUB'
#!/usr/bin/env bash
case "$1" in
  activeworkspace) printf '{"id": %s}\n' "$ACTIVE_WS" ;;
  dispatch) shift; printf '%s\n' "$*" >>"$LOG" ;;
esac
STUB
chmod +x "$tmp/hyprctl"

fail() { echo "FAIL: $1" >&2; exit 1; }
run() { # active workspace, then the zenith-workspace arguments; prints the dispatch
  : >"$tmp/log"
  PATH="$tmp:$PATH" ACTIVE_WS="$1" LOG="$tmp/log" "$here/zenith-workspace" "${@:2}"
  cat "$tmp/log"
}

[[ "$(run 12 focus 3)" == "workspace 13" ]] || fail "focus 3 on desktop 1 did not go to workspace 13"
[[ "$(run 5 focus 10)" == "workspace 10" ]] || fail "focus 10 on desktop 0 did not go to workspace 10"
[[ "$(run 25 move 1)" == "movetoworkspace 21" ]] || fail "move 1 on desktop 2 did not go to workspace 21"
[[ "$(run 25 movesilent 4)" == "movetoworkspacesilent 24" ]] || fail "movesilent 4 on desktop 2 did not go to workspace 24"
[[ "$(PATH="$tmp:$PATH" ACTIVE_WS=31 "$here/zenith-workspace" desktop)" == "3" ]] || fail "workspace 31 is not desktop 3"
"$here/zenith-workspace" bogus 2>/dev/null && fail "an unknown command exited 0" || true

echo "zenith-workspace: all checks passed"
