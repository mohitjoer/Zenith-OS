#!/usr/bin/env bash
# One runnable check for the recorder chain on a box that has wf-recorder and no
# gpu-screen-recorder, which is every stock Ubuntu install: the backend must be
# wf, the clip must land where Recorder.qml and Paths.qml look, last-recording
# must point at it, and the post-save hook must be the one next to the script.
# wf-recorder is a stub that writes its output file and exits, so nothing is
# captured and nothing is signalled.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
scripts="$here/../../../../shell/scripts"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

bin="$tmp/bin"; mkdir -p "$bin"
cat >"$bin/wf-recorder" <<EOF
#!/usr/bin/env bash
echo "wf-recorder \$*" >>"$tmp/wf.log"
while [ \$# -gt 0 ]; do [ "\$1" = -f ] && { shift; out="\$1"; }; shift; done
printf 'x' >"\$out"
sleep 2
EOF
cat >"$bin/notify-send" <<EOF
#!/bin/sh
echo "notify \$*" >>"$tmp/notify.log"
EOF
chmod +x "$bin"/*

export PATH="$bin:$PATH"
export ZENITH_STATE_PATH="$tmp/state"
export ZENITH_SHELL_RECORDINGS_DIR="$tmp/rec"
export XDG_CONFIG_HOME="$tmp/cfg"
export ZENITH_SCREENRECORD_RECORDER="$tmp/no-gsr"
export ZENITH_SCREENRECORD_WF="$bin/wf-recorder"

fail() { echo "FAIL: $1" >&2; exit 1; }

info="$("$scripts/zenith-cmd-screenrecord" --info)"
grep -q '"backend":"wf"' <<<"$info" || fail "backend is not wf without gpu-screen-recorder: $info"

"$scripts/zenith-cmd-screenrecord" --fullscreen >/dev/null 2>&1 || fail "wf start exited non-zero"
clip="$(cat "$ZENITH_STATE_PATH/last-recording")"
[[ $clip == "$ZENITH_SHELL_RECORDINGS_DIR"/*.mp4 && -s $clip ]] || fail "last-recording does not name a clip in the recordings dir: $clip"
grep -q -- '-r 60' "$tmp/wf.log" || fail "wf-recorder was not given the default 60 fps: $(cat "$tmp/wf.log")"
for _ in $(seq 20); do grep -q 'Recording saved' "$tmp/notify.log" 2>/dev/null && break; sleep 0.2; done
grep -q 'Recording saved' "$tmp/notify.log" || fail "post-save hook did not announce the clip"

# Without wf-recorder either, the message names the apt package, and exits 127.
rc=0
ZENITH_SCREENRECORD_WF="$tmp/none" ZENITH_SCREENRECORD_BACKEND=wf "$scripts/zenith-cmd-screenrecord" --fullscreen >/dev/null 2>&1 || rc=$?
[ "$rc" -eq 127 ] || fail "missing wf-recorder exited $rc, want 127"
grep -q 'sudo apt install wf-recorder' "$tmp/notify.log" || fail "missing-tool notification does not name the package"

echo "screenrecord: all checks passed"
