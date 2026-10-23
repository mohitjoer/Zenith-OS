#!/bin/sh
# Contract checks: does the tree still hold together after many lanes edited it at once?
#
#   sh tests/contract/run.sh            gate: run every check, print failures, exit 1 if any failed
#   sh tests/contract/run.sh --report   print every finding in full and exit 0 (for triage)
#   sh tests/contract/run.sh --no-go    skip go vet / go test, the slow part
#
# The checks, each also runnable alone:
#   commands.py  every external command the QML, confs, scripts and Go call must come from a
#                build line, a package, the base layer or optional.txt. Prints UNRESOLVED
#                commands with file:line; a trailing * means a file probes for the command
#                (likely gated: a candidate for optional.txt, not a missing dependency).
#                `commands.py --selftest` is the one runnable check of its parsers.
#   daemon.py    zenith / zenith-shell / zenith-hubd / zenith-wallpaper verbs, and
#                `ipc call TARGET FN`, exist in the Go source / the shell's IpcHandlers.
#   qml.sh       qmllint syntax-class errors against qml.baseline (its header says why only
#                those; `qml.sh --update` re-baselines).
#   load.sh      loads the shell tree in a sandbox; fails on module-resolution errors.
#   units.sh     session/units verify, and their Exec targets are installed by something.
#   go           go vet and go test in desktop/ and system/hardware/.
#
# What the lanes hand off and the checks read:
#   release/build.d/*.build       `SRC -> /usr/bin/NAME` (SRC: a desktop/cmd dir or a repo path)
#   release/deps.d/*.deps         `depends|recommends|suggests PACKAGE  # why`, or a
#                                 `# unavailable: TOOL (fallback)` note
#   tests/contract/optional.txt   `command  # how the caller is gated`
root=$(cd "$(dirname "$0")/../.." && pwd)
here=$root/tests/contract
report=0 go=1 fail=0
for a in "$@"; do
    case $a in --report) report=1 ;; --no-go) go=0 ;; esac
done
out=$(mktemp -d) && trap 'rm -rf "$out"' EXIT

step() {    # step NAME COMMAND...: log it, show the log when asked or when it failed
    name=$1; shift
    "$@" >"$out/$name" 2>&1; rc=$?
    [ $rc = 0 ] || fail=1
    if [ $report = 1 ] || [ $rc != 0 ]; then echo "== $name"; cat "$out/$name"; fi
    printf '%-12s %s  %s\n' "$name" "$([ $rc = 0 ] && echo PASS || echo FAIL)" "$(tail -n 1 "$out/$name" | cut -c1-100)" >>"$out/summary"
}
commands() { python3 "$here/commands.py" --selftest && python3 "$here/commands.py"; }
gotest() { (cd "$1" && go vet ./... && go test ./...); }

step commands commands
step daemon python3 "$here/daemon.py"
step qml sh "$here/qml.sh"
step load sh "$here/load.sh"
step dock sh "$here/dock_probe.sh"
step units sh "$here/units.sh"
if [ $go = 1 ]; then
    step go-desktop gotest "$root/desktop"
    step go-hardware gotest "$root/system/hardware"
fi
echo; echo "contract summary"; cat "$out/summary"
[ $report = 1 ] || exit $fail
