#!/bin/sh
# One check for zenith-touchpad against a fake hyprctl: what it asks Hyprland, and
# what it remembers between calls.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/bin" "$tmp/run"
export XDG_RUNTIME_DIR="$tmp/run" PATH="$tmp/bin:/usr/bin:/bin" CALLS="$tmp/calls"
: >"$CALLS"

# The shape `hyprctl devices -j` has on 0.53, trimmed. The mouse that shares the
# touchpad's hardware id must not be touched.
cat >"$tmp/devices.json" <<'EOF'
{
"mice": [
    {
        "address": "0x1",
        "name": "asup1205:00-093a:2003-mouse",
        "defaultSpeed": 0.00000
    },    {
        "address": "0x2",
        "name": "asup1205:00-093a:2003-touchpad",
        "defaultSpeed": 0.00000
    }
],
"keyboards": [
    {
        "address": "0x3",
        "name": "at-translated-set-2-keyboard"
    },    {
        "address": "0x4",
        "name": "asup1205:00-093a:2003-touchpad-keys"
    }
]
}
EOF
cat >"$tmp/bin/hyprctl" <<'EOF'
#!/bin/sh
echo "$*" >>"$CALLS"
[ -z "${FAKE_DOWN:-}" ] || exit 1
case "$*" in
"devices -j") cat "$DEVICES" ;;
*keyword*) echo "${FAKE_ANSWER:-ok}" ;;
esac
EOF
chmod +x "$tmp/bin/hyprctl"
export DEVICES="$tmp/devices.json"

fail() { echo "FAIL: $1" >&2; exit 1; }
keywords() { grep -c keyword "$CALLS" || true; }
pad='-r keyword device[asup1205:00-093a:2003-touchpad]:enabled'

[ "$("$here/zenith-touchpad" status)" = on ] || fail "a fresh session is not on"

"$here/zenith-touchpad" off
grep -qxF -- "$pad false" "$CALLS" || fail "off did not disable the touchpad: $(cat "$CALLS")"
[ "$(keywords)" = 1 ] || fail "off touched something besides the touchpad"
[ "$("$here/zenith-touchpad" status)" = off ] || fail "status did not remember off"

: >"$CALLS"; "$here/zenith-touchpad" restore
grep -qxF -- "$pad false" "$CALLS" || fail "restore did not push the stored off back"

"$here/zenith-touchpad" toggle
grep -qxF -- "$pad true" "$CALLS" || fail "toggle from off did not enable"
[ "$("$here/zenith-touchpad" status)" = on ] || fail "toggle did not clear the flag"

: >"$CALLS"; "$here/zenith-touchpad" restore
[ "$(keywords)" = 0 ] || fail "restore with the pad on still called hyprctl keyword"

"$here/zenith-touchpad" toggle
[ "$("$here/zenith-touchpad" status)" = off ] || fail "toggle from on did not disable"

# Hyprland refusing the keyword must not look like success.
rm -f "$XDG_RUNTIME_DIR/zenith/touchpad-off"
if FAKE_ANSWER='no such keyword' "$here/zenith-touchpad" off 2>/dev/null; then fail "a refused keyword exited 0"; fi
[ "$("$here/zenith-touchpad" status)" = on ] || fail "a refused keyword was remembered as off"

# No compositor to ask.
if FAKE_DOWN=1 "$here/zenith-touchpad" off 2>/dev/null; then fail "an unreachable Hyprland exited 0"; fi

# A machine without a touchpad.
sed -i 's/touchpad/pointer/' "$tmp/devices.json"
: >"$CALLS"; "$here/zenith-touchpad" off
[ "$(keywords)" = 0 ] || fail "flipped a device on a machine with no touchpad"
[ "$("$here/zenith-touchpad" status)" = on ] || fail "claimed off with no touchpad"

echo "zenith-touchpad: ok"
