#!/bin/sh
# Check that zenith-idle turns the org.zenith.desktop timeouts into the
# variables hypridle.conf tests, and that it restarts the service only when a
# timeout changed.
#
# It is run rather than unit-tested, like the other session scripts: what matters
# is how /bin/sh behaves. gsettings, hypridle and systemctl are stubs, so nothing
# on the machine is touched.
#
# Usage: session/idle_test.sh
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin" "$WORK/run" "$WORK/home"

fail=0
check() {
    if [ "$2" = "0" ]; then
        echo "ok    — $1"
    else
        echo "FAIL  — $1"
        fail=$((fail + 1))
    fi
}

# gsettings answers from a KEY=VALUE file and fails for a key the file lacks.
cat > "$WORK/bin/gsettings" <<'STUB'
#!/bin/sh
grep "^$3=" "$ZENITH_TEST_VALUES" | cut -d= -f2 | grep .
STUB
# hypridle records what it was started with, and that its default search path
# holds a config (the 0.1.7 abort).
cat > "$WORK/bin/hypridle" <<'STUB'
#!/bin/sh
{ echo "args $*"; env | grep '^ZENITH_IDLE_' | sort; ls "${XDG_CONFIG_DIRS%%:*}/hypr/hypridle.conf"; } > "$ZENITH_TEST_OUT"
STUB
cat > "$WORK/bin/systemctl" <<'STUB'
#!/bin/sh
echo "$*" >> "$ZENITH_TEST_CALLS"
STUB
chmod +x "$WORK/bin/gsettings" "$WORK/bin/hypridle" "$WORK/bin/systemctl"

# idle COMMAND [KEY=VALUE...]
idle() {
    cmd=$1
    shift
    : > "$WORK/values"
    for kv in "$@"; do echo "$kv" >> "$WORK/values"; done
    env -i PATH="$WORK/bin:/usr/bin:/bin" HOME="$WORK/home" XDG_RUNTIME_DIR="$WORK/run" \
        ZENITH_TEST_VALUES="$WORK/values" ZENITH_TEST_OUT="$WORK/out" \
        ZENITH_TEST_CALLS="$WORK/calls" "$REPO_ROOT/session/zenith-idle" "$cmd"
}

echo "checking zenith-idle"

rc=0; idle start || rc=$?
check "it starts hypridle with the schema defaults when gsettings answers nothing" $rc
rc=0; [ "$(grep '^ZENITH_IDLE_' "$WORK/out")" = "ZENITH_IDLE_LOCK=300" ] || rc=$?
check "the defaults are a five minute lock and no dim or blank" $rc

rc=0; idle start idle-dim-seconds=120 idle-blank-seconds=300 idle-lock-seconds=600 || rc=$?
rc2=0; [ "$(grep -c '^ZENITH_IDLE_' "$WORK/out")" = 3 ] && grep -q '^ZENITH_IDLE_DIM=120$' "$WORK/out" || rc2=$?
check "each timeout becomes its variable" $((rc + rc2))

rc=0; idle start idle-dim-seconds=0 idle-blank-seconds=0 idle-lock-seconds=0 || rc=$?
rc2=0; ! grep -q '^ZENITH_IDLE_' "$WORK/out" || rc2=$?
check "a timeout of 0 (never) leaves its variable unset" $((rc + rc2))

rc=0; grep -q "^args -c /usr/share/zenith/hyprland/hypridle.conf$" "$WORK/out" || rc=$?
check "it falls back to the packaged config before the session is materialized" $rc
mkdir -p "$WORK/home/.config/zenith/hyprland"
: > "$WORK/home/.config/zenith/hyprland/hypridle.conf"
idle start
rc=0; grep -q "^args -c $WORK/home/.config/zenith/hyprland/hypridle.conf$" "$WORK/out" || rc=$?
check "the materialized config wins" $rc

# apply: nothing for an unchanged timeout, a restart for a changed one.
idle start idle-lock-seconds=600
rm -f "$WORK/calls"
idle apply idle-lock-seconds=600
rc=0; [ ! -e "$WORK/calls" ] || rc=$?
check "apply leaves the service alone when the timeouts are unchanged" $rc
idle apply idle-lock-seconds=900
rc=0; grep -qx -- '--user try-restart zenith-idle.service' "$WORK/calls" || rc=$?
check "apply restarts the service when a timeout changed" $rc

# hypridle.conf and the script have to agree on the variable names.
rc=0
for var in $(sed -n 's/^# hyprlang if //p' "$REPO_ROOT/session/hyprland/hypridle.conf"); do
    grep -q "export $var=" "$REPO_ROOT/session/zenith-idle" || { echo "      $var is never exported"; rc=1; }
done
check "every variable hypridle.conf tests is one zenith-idle exports" $rc

echo
if [ "$fail" -gt 0 ]; then
    echo "$fail check(s) failed"
    exit 1
fi
echo "all checks passed"
