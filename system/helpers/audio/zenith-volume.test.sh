#!/bin/sh
# Runs zenith-volume against a fake wpctl and a throwaway config dir, and checks
# the level and cap it would ask for. Needs no PipeWire.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/cfg/zenith"
cat >"$tmp/bin/wpctl" <<'EOF'
#!/bin/sh
case "$1" in
get-volume) echo "Volume: $FAKE_VOL" ;;
set-volume) shift; echo "$*" >"$FAKE_LOG" ;;
esac
EOF
chmod +x "$tmp/bin/wpctl"

check() { # name, volume, direction, expected set-volume arguments
	FAKE_VOL=$2 FAKE_LOG=$tmp/log XDG_CONFIG_HOME=$tmp/cfg PATH=$tmp/bin:$PATH \
		bash "$here/zenith-volume" "$3"
	[ "$(cat "$tmp/log")" = "$4" ] || { echo "FAIL $1: got '$(cat "$tmp/log")'" >&2; exit 1; }
}

check "step up" 0.40 up "-l 1 @DEFAULT_AUDIO_SINK@ 45%"
check "capped at 100" 1.00 up "-l 1 @DEFAULT_AUDIO_SINK@ 100%"
check "floored at 0" 0.03 down "-l 1 @DEFAULT_AUDIO_SINK@ 0%"
check "65 skips to 67" 0.65 up "-l 1 @DEFAULT_AUDIO_SINK@ 67%"
check "70 steps back to 67" 0.70 down "-l 1 @DEFAULT_AUDIO_SINK@ 67%"
printf '{"qsbar": {"audioBoost": true}}' >"$tmp/cfg/zenith/shell.json"
check "boost passes 100" 1.00 up "-l 1.5 @DEFAULT_AUDIO_SINK@ 105%"
check "boost capped at 150" 1.48 up "-l 1.5 @DEFAULT_AUDIO_SINK@ 150%"
echo "ok"
