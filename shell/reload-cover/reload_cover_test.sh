#!/usr/bin/env bash
# The reload cover's launcher, tested against a stand-in for Quickshell.
#
# The cover is a window, and a window cannot be opened by a test that runs on
# someone's desktop: this one would draw a black curtain over their screen. So
# the launcher is given a Quickshell that is a script. It answers the same
# command line the real one does (`-p ENTRY` starts the cover, `ipc --path ENTRY
# call reload-cover FN TOKEN` talks to it) and keeps the cover's phases in a
# file, which is enough to run the handshake end to end: begin waits for the
# curtain, finish opens it, a second begin is refused while one is down.
#
# What this does not check is the QML. `shell/shell_test.sh` loads the windows
# in a real engine, and says so when the machine cannot.
#
# Nothing here touches the real runtime directory or the real sockets: every
# path is under a temporary directory.
set -u

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT="$REPO_ROOT/shell/scripts/zenith-reload-cover"
WRAPPER="$REPO_ROOT/session/zenith-reload-cover"

TMP="$(mktemp -d)"
FAILED=0
cleanup() {
    # Any stand-in cover still alive is this test's; the pid files name them.
    for f in "$TMP"/fake*/pid; do
        [ -f "$f" ] && kill "$(cat "$f")" 2>/dev/null
    done
    rm -rf "$TMP"
}
trap cleanup EXIT

pass() { printf 'ok    - %s\n' "$1"; }
fail() { printf 'FAIL  - %s\n' "$1"; FAILED=1; }
check() {
    # check "what" command...
    what=$1
    shift
    if "$@"; then pass "$what"; else fail "$what"; fi
}

# ---------------------------------------------------------------- the stand-in
mkdir -p "$TMP/bin"
cat >"$TMP/bin/quickshell" <<'FAKE'
#!/usr/bin/env bash
# Stand-in for quickshell. FAKE_DIR holds the cover's state; FAKE_MODE changes
# how the cover behaves: normal, stuck (never covers), crash (exits at once).
d="${FAKE_DIR:?}"
mkdir -p "$d"

if [[ ${1:-} == ipc ]]; then
    shift
    while [[ ${1:-} != call && $# -gt 0 ]]; do shift; done
    shift                                   # call
    target=$1 fn=$2 token=${3:-}
    [[ $target == reload-cover ]] || exit 1
    pid=$(cat "$d/pid" 2>/dev/null) || { echo "No running instances" >&2; exit 1; }
    kill -0 "$pid" 2>/dev/null || { echo "No running instances" >&2; exit 1; }
    if [[ $token != "$(cat "$d/token")" ]]; then
        [[ $fn == status ]] && echo "" || echo false
        exit 0
    fi
    phase=$(cat "$d/phase")
    case $fn in
        status)  echo "$phase" ;;
        covered) [[ $phase == hold ]] && echo true || echo false ;;
        mapped)  echo true ;;
        finish)
            if [[ $phase == hold || $phase == closing ]]; then echo opening >"$d/phase"; echo true; else echo false; fi ;;
        fail)
            if [[ $phase != opening ]]; then echo failed >"$d/phase"; echo true; else echo false; fi ;;
    esac
    exit 0
fi

# started as the cover: `-p ENTRY`
[[ ${FAKE_MODE:-normal} == crash ]] && { echo "module \"qs.Ui\" is not installed" >&2; exit 3; }
echo $$ >"$d/pid"
printf '%s' "${ZENITH_RELOAD_COVER_TOKEN:-}" >"$d/token"
echo closing >"$d/phase"
echo "$*" >"$d/argv"
if [[ ${FAKE_MODE:-normal} == stuck ]]; then sleep 30; exit 0; fi
sleep 0.3
echo hold >"$d/phase"
for _ in $(seq 1 200); do
    [[ $(cat "$d/phase") == opening ]] && { sleep 0.2; exit 0; }
    [[ $(cat "$d/phase") == failed ]] && { sleep 0.2; exit 0; }
    sleep 0.05
done
FAKE
chmod +x "$TMP/bin/quickshell"

# One run of the launcher in its own runtime directory and its own fake.
export WAYLAND_DISPLAY=wayland-test
export ZENITH_SHELL_DIR="$REPO_ROOT/shell"
export ZENITH_QUICKSHELL="$TMP/bin/quickshell"

fresh() {
    n=${n:-0}; n=$((n + 1))
    export XDG_RUNTIME_DIR="$TMP/run$n"
    export FAKE_DIR="$TMP/fake$n"
    export FAKE_MODE=${1:-normal}
    mkdir -p "$XDG_RUNTIME_DIR"
}
statefile() { echo "$XDG_RUNTIME_DIR/zenith-reload-cover.json"; }
# Read one field of the state file the way the QML does: as JSON.
field() { python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))[sys.argv[2]])' "$(statefile)" "$1"; }
gone() {
    # gone PID: true when the process has ended, within two seconds
    for _ in $(seq 1 40); do kill -0 "$1" 2>/dev/null || return 0; sleep 0.05; done
    return 1
}

# ---------------------------------------------------------------- the syntax
check "the launcher parses" bash -n "$SCRIPT"
check "the wrapper parses" sh -n "$WRAPPER"
check "the launcher is executable" test -x "$SCRIPT"
check "the wrapper is executable" test -x "$WRAPPER"

# ------------------------------------------------------- begin, then finish
fresh
token=$("$SCRIPT" begin 2>"$TMP/err1")
rc=$?
check "begin succeeds" test "$rc" = 0
check "begin prints a 32-character hex token (the shell accepts no other)" \
    test "$(printf '%s' "$token" | grep -cE '^[0-9a-f]{32}$')" = 1
check "the cover was started on the entry file at the shell's root" \
    grep -q -- "-p $REPO_ROOT/shell/reload-cover.qml" "$FAKE_DIR/argv"
check "the cover received the token in its environment" \
    test "$(cat "$FAKE_DIR/token")" = "$token"
check "the state file is private" test "$(stat -c %a "$(statefile)")" = 600
check "the state file is JSON the shell can read" \
    test "$(field token)" = "$token"
check "the state file names the cover's pid" \
    test "$(field pid)" = "$(cat "$FAKE_DIR/pid")"
check "the state file records the start time with the pid" \
    test -n "$(field pidStart)"
check "begin returned only when the curtain was down (phase hold)" \
    test "$(cat "$FAKE_DIR/phase")" = hold

# A second begin while one is down must not start a second curtain.
"$SCRIPT" begin >"$TMP/second" 2>"$TMP/second.err"
check "a second begin is refused while a cover is down" test $? -ne 0
check "the refused begin printed no token" test ! -s "$TMP/second"
check "the first cover is still the one in the state file" \
    test "$(field token)" = "$token"

# A finish for some other token is not an error and changes nothing.
"$SCRIPT" finish 00000000000000000000000000000000
check "finish with the wrong token is a quiet no-op" test $? = 0
check "...and leaves the real state alone" test "$(field token)" = "$token"
check "...and does not open the cover" test "$(cat "$FAKE_DIR/phase")" = hold

pid=$(cat "$FAKE_DIR/pid")
"$SCRIPT" finish "$token"
check "finish with the token succeeds" test $? = 0
check "finish opens the cover" test "$(cat "$FAKE_DIR/phase")" = opening
check "the cover process ends" gone "$pid"
for _ in $(seq 1 60); do [ -e "$(statefile)" ] || break; sleep 0.05; done
check "the state file is removed once the cover is gone" test ! -e "$(statefile)"
check "no lock outlives begin (the next begin is not refused)" \
    flock -n "$XDG_RUNTIME_DIR/zenith-reload-cover.lock" true

# ------------------------------------------------- two begins at the same time
fresh
("$SCRIPT" begin >"$TMP/a" 2>/dev/null) &
("$SCRIPT" begin >"$TMP/b" 2>/dev/null) &
wait
check "of two simultaneous begins exactly one gets a token" \
    test "$(cat "$TMP/a" "$TMP/b" | grep -cE '^[0-9a-f]{32}$')" = 1
"$SCRIPT" finish "$(field token)"

# ------------------------------------------------------------ fail, then retry
fresh
token=$("$SCRIPT" begin 2>/dev/null)
"$SCRIPT" fail "$token"
check "fail puts the cover in its failed phase" test "$(cat "$FAKE_DIR/phase")" = failed
old=$(cat "$FAKE_DIR/pid")
second=$("$SCRIPT" begin 2>/dev/null)
check "a failed cover is replaced by the next begin" \
    test "$(printf '%s' "$second" | grep -cE '^[0-9a-f]{32}$')" = 1
check "...with a different token" test "$second" != "$token"
check "...and the failed cover was stopped" gone "$old"
"$SCRIPT" finish "$second"

# ------------------------------------------------------------ a stale state file
fresh
printf '{"token":"%s","pid":999999,"pidStart":"1"}\n' "$(printf 'ab%.0s' $(seq 1 16))" >"$(statefile)"
"$SCRIPT" finish "$(printf 'ab%.0s' $(seq 1 16))"
check "finish on a state whose cover is gone succeeds" test $? = 0
check "...and removes the stale state file" test ! -e "$(statefile)"

fresh
printf '{"token":"%s","pid":999999,"pidStart":"1"}\n' "$(printf 'cd%.0s' $(seq 1 16))" >"$(statefile)"
token=$("$SCRIPT" begin 2>/dev/null)
check "begin replaces a stale state file" \
    test "$(printf '%s' "$token" | grep -cE '^[0-9a-f]{32}$')" = 1
"$SCRIPT" finish "$token"

# ------------------------------------------------------- the failure modes
fresh crash
"$SCRIPT" begin >"$TMP/c.out" 2>"$TMP/c.err"
check "begin fails fast when the cover cannot start" test $? -ne 0
check "...says why" grep -q "exited before it was up" "$TMP/c.err"
check "...prints no token" test ! -s "$TMP/c.out"
check "...and leaves no state behind" test ! -e "$(statefile)"

fresh
ZENITH_QUICKSHELL=/nonexistent/quickshell "$SCRIPT" begin >"$TMP/d.out" 2>"$TMP/d.err"
check "begin fails when Quickshell is missing" test $? -ne 0
check "...and says which program" grep -q "nonexistent/quickshell" "$TMP/d.err"

fresh
env -u WAYLAND_DISPLAY "$SCRIPT" begin >"$TMP/e.out" 2>"$TMP/e.err"
check "begin fails with no display, rather than waiting" test $? -ne 0
check "...and says there is no display" grep -q "no Wayland display" "$TMP/e.err"

fresh
ZENITH_SHELL_DIR="$TMP/empty" "$SCRIPT" begin >"$TMP/f.out" 2>"$TMP/f.err"
check "begin fails when the entry file is missing" test $? -ne 0

# A cover that never reaches the curtain stage is stopped, after the deadline.
fresh stuck
start=$(date +%s)
"$SCRIPT" begin >"$TMP/g.out" 2>"$TMP/g.err"
rc=$?
elapsed=$(( $(date +%s) - start ))
check "begin gives up on a cover that never covers" test "$rc" -ne 0
check "...within the deadline (4s) and not much after" test "$elapsed" -ge 3 -a "$elapsed" -le 7
check "...and stops that cover" gone "$(cat "$FAKE_DIR/pid")"
check "...and leaves no state behind" test ! -e "$(statefile)"

# ----------------------------------------------------------- usage and wrapper
"$SCRIPT" bogus >/dev/null 2>"$TMP/h.err"
check "an unknown verb is a usage error (exit 2)" test $? = 2

fresh
token=$("$WRAPPER" begin 2>/dev/null)
check "the PATH wrapper reaches the launcher" \
    test "$(printf '%s' "$token" | grep -cE '^[0-9a-f]{32}$')" = 1
"$WRAPPER" finish "$token"

if [ "$FAILED" = 0 ]; then
    echo "reload cover launcher: PASS"
else
    echo "reload cover launcher: FAIL"
fi
exit "$FAILED"
