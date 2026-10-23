#!/bin/sh
# What in the screenshot tool is not ported code and runs without a display:
# the wrapper's single-instance lock, and the capture card's shell script
# (grim, clipboard, toast) lifted out of Capture.qml and run against stub tools.
set -eu

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
T=$(mktemp -d)
trap 'rm -rf "$T"' EXIT
fail=0
check() { if [ "$2" = ok ]; then echo "ok    $1"; else echo "FAIL  $1"; fail=1; fi; }

# --- the wrapper keeps one instance, and says when Quickshell is missing -------
cat >"$T/fake-qs" <<EOF
#!/bin/sh
: > "$T/up"
sleep 1
EOF
chmod +x "$T/fake-qs"
ZENITH_QUICKSHELL="$T/fake-qs" "$ROOT/session/zenith-shot" &
n=0; while [ ! -e "$T/up" ] && [ "$n" -lt 50 ]; do sleep 0.1; n=$((n + 1)); done
rc=0; ZENITH_QUICKSHELL="$T/fake-qs" "$ROOT/session/zenith-shot" || rc=$?
wait
[ "$rc" -ne 0 ] && check "a second zenith-shot while one is open does not start" ok \
                || check "a second zenith-shot while one is open does not start" no
rc=0; ZENITH_QUICKSHELL="$T/none" "$ROOT/session/zenith-shot" 2>/dev/null || rc=$?
[ "$rc" -ne 0 ] && check "a missing quickshell is an error, not a silent exit" ok \
                || check "a missing quickshell is an error, not a silent exit" no

# --- the capture card's script ---------------------------------------------------
sed -n '/var script = \[/,/\]\.join/p' "$ROOT/shell/Services/Capture.qml" | sed '1d;$d' \
    | sed "s/^ *'//; s/',\{0,1\}\$//" >"$T/card.sh"

mkdir "$T/bin"
ln -s "$(command -v mkdir)" "$(command -v dirname)" "$T/bin/"
cat >"$T/bin/notify-send" <<EOF
#!/bin/sh
shift 2; printf '%s\n' "\$1" >> "$T/notes"
EOF
chmod +x "$T/bin/notify-send"

# stub NAME BODY puts a tool on the test PATH; run MODE CLIP then uses what is there.
stub() { printf '#!/bin/sh\n%s\n' "$2" >"$T/bin/$1"; chmod +x "$T/bin/$1"; }
SH=$(command -v sh)
run() {
    mode=$1; clip=$2; shift 2
    rm -f "$T/out.png" "$T/bin/grim" "$T/bin/zenith-shell" "$T/bin/wl-copy"; : >"$T/notes"
    for t in "$@"; do
        case $t in
        grim)      stub grim 'for a; do o=$a; done; : >"$o"' ;;
        daemon)    stub zenith-shell 'exit 0' ;;
        nodaemon)  stub zenith-shell 'exit 1' ;;
        wl-copy)   stub wl-copy 'exit 0' ;;
        esac
    done
    PATH="$T/bin" "$SH" -c "$(cat "$T/card.sh")" sh "" "" "$T/out.png" "$mode" "$clip" 2>/dev/null
}
said() { grep -q "$1" "$T/notes" && echo ok || echo no; }

run both 1 grim daemon;           check "shot saved and copied through the daemon" "$(said 'saved & copied')"
run both 1 grim nodaemon wl-copy; check "wl-copy stands in when the daemon is not up" "$(said 'saved & copied')"
run both 1 grim;                  check "no clipboard tool is said, and the shot is still saved" "$(said 'not copied')"
grep -q 'Screenshot saved$' "$T/notes" && check "a shot that was saved but not copied is reported as saved" ok \
                                       || check "a shot that was saved but not copied is reported as saved" no
run clipboard 1 grim nodaemon;    check "a clipboard-only shot that cannot be copied is not reported as done" \
                                        "$(grep -q 'copied to clipboard\|saved' "$T/notes" && echo no || echo ok)"
run both 1;                       check "a missing grim says to install it" "$(said 'need grim')"
[ ! -e "$T/out.png" ] && check "a missing grim leaves no file and no success toast" "$(grep -q 'saved' "$T/notes" && echo no || echo ok)"

exit $fail
