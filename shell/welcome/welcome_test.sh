#!/bin/sh
# The welcome wrapper's first-run gate, against a stand-in for Quickshell:
# the marker is written after a clean close, never after a failed one, and
# --if-first-run does nothing once it exists.
set -u
WRAPPER="$(cd "$(dirname "$0")/../.." && pwd)/session/zenith-welcome"
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
export XDG_STATE_HOME="$T/state" XDG_RUNTIME_DIR="$T/run"; mkdir -p "$XDG_RUNTIME_DIR"
printf '#!/bin/sh\necho ran >> "%s/ran"\nexit "${FAKE_EXIT:-0}"\n' "$T" > "$T/qs"; chmod +x "$T/qs"
export ZENITH_QUICKSHELL="$T/qs"
M="$XDG_STATE_HOME/zenith/welcome-done"
fail=0; ok() { [ "$1" = 0 ] || { echo "FAIL: $2"; fail=1; }; }

FAKE_EXIT=1 "$WRAPPER" --if-first-run 2>/dev/null; test ! -e "$M"; ok $? "a failed tour must not write the marker"
"$WRAPPER" --if-first-run; test -e "$M"; ok $? "a clean close writes the marker"
rm -f "$T/ran"; "$WRAPPER" --if-first-run; test ! -e "$T/ran"; ok $? "--if-first-run does nothing once the marker exists"
"$WRAPPER"; test -e "$T/ran"; ok $? "without the flag the tour opens regardless"
[ "$fail" = 0 ] && echo "welcome wrapper: PASS"; exit "$fail"
