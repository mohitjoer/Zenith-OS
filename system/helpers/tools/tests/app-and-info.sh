#!/usr/bin/env bash
# Runnable check for the two things other lanes call: `zenith-app terminal -- CMD`
# and `zenith-app editor -- FILE` must run the command in the terminal / open the
# file in the editor, and `zenith-cmd-screenrecord --info` must print one JSON
# object with backend and encoder, both empty when no recorder is installed.
# Every launched program is a stub that records its argv, so no window opens.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
scripts="$here/../../../../shell/scripts"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

bin="$tmp/bin"; mkdir -p "$bin"
for t in ghostty nvim notify-send; do
	cat >"$bin/$t" <<EOF
#!/bin/sh
echo "$t \$*" >>"$tmp/argv.log"
EOF
done
cat >"$bin/wf-recorder" <<EOF
#!/bin/sh
exit 0
EOF
chmod +x "$bin"/*

# A PATH of the stubs plus the coreutils the scripts need, with no real ghostty,
# nvim, wf-recorder or gpu-screen-recorder anywhere on it.
safe="$tmp/safe"; mkdir -p "$safe"
for t in sh bash env jq cat sed awk grep tr cut head tail date mkdir dirname readlink basename md5sum pgrep wc sort stat rm mktemp printf seq sleep; do
	p="$(command -v "$t" 2>/dev/null || true)"; [ -n "$p" ] && ln -s "$p" "$safe/$t"
done
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/cfg" ZENITH_STATE_PATH="$tmp/state"
mkdir -p "$HOME"
export PATH="$bin:$safe"

fail() { echo "FAIL: $1" >&2; exit 1; }
last() { tail -n1 "$tmp/argv.log"; }

: >"$tmp/argv.log"
"$scripts/zenith-app" terminal -- sh -c 'echo hi'
[ "$(last)" = "ghostty -e sh -c echo hi" ] || fail "terminal -- CMD: got '$(last)'"

: >"$tmp/argv.log"
"$scripts/zenith-app" editor -- /tmp/some/file.txt
[ "$(last)" = "ghostty -e nvim /tmp/some/file.txt" ] || fail "editor -- FILE: got '$(last)'"

: >"$tmp/argv.log"
"$scripts/zenith-app" terminal
[ "$(last)" = "ghostty " ] || fail "bare terminal: got '$(last)'"

# a stored editor that carries its own terminal wrapper still gets the file last
mkdir -p "$XDG_CONFIG_HOME/zenith"
echo '{"desktop":{"apps":{"editor":"ghostty --class=notes -e nvim"}}}' >"$XDG_CONFIG_HOME/zenith/desktop.json"
: >"$tmp/argv.log"
"$scripts/zenith-app" editor -- "a file.txt"
[ "$(last)" = "ghostty --class=notes -e nvim a file.txt" ] || fail "stored editor -- FILE: got '$(last)'"

# --info: wf-recorder present
export ZENITH_SCREENRECORD_RECORDER="$tmp/no-gsr" ZENITH_SCREENRECORD_WF="$bin/wf-recorder"
info="$("$scripts/zenith-cmd-screenrecord" --info)"
jq -e '.backend == "wf" and (.encoder | length > 0)' <<<"$info" >/dev/null || fail "--info with wf-recorder: $info"

# --info: nothing installed
export ZENITH_SCREENRECORD_WF="$tmp/no-wf"
info="$("$scripts/zenith-cmd-screenrecord" --info)"
jq -e '.backend == "" and .encoder == ""' <<<"$info" >/dev/null || fail "--info with no recorder: $info"
[ "$(wc -l <<<"$info")" -eq 1 ] || fail "--info is not a single line: $info"

echo "app-and-info: all checks passed"
