#!/bin/sh
# Check the session entry point's properties.
#
# The session script cannot be unit-tested in the ordinary sense — it has to be
# run, because the properties that matter are about how /bin/sh behaves under
# `set -e`. So it is run, with the two things it needs stubbed: a log it can
# write to, and a compositor that is not a compositor.
#
# Usage: session/session_test.sh
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SCRIPT="$REPO_ROOT/session/zenith-session"
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

fail=0
check() {
    if [ "$2" = "0" ]; then
        echo "ok    — $1"
    else
        echo "FAIL  — $1"
        fail=$((fail + 1))
    fi
}

# A stub compositor that records the environment it was handed, which is the
# only way to assert what the session exported.
cat > "$WORK/compositor" <<STUB
#!/bin/sh
env | sort > "$WORK/seen.env"
STUB
chmod +x "$WORK/compositor"

# systemctl and dbus-update-activation-environment are stubbed for the same
# reason, and both are on a stub PATH that comes before the real one.
#
# They are not stubbed to hide a failure. Under `set -e` a session script that
# cannot publish its identity aborts before the compositor, and that is the
# intended behaviour: a session whose units and bus activations do not inherit
# XDG_CURRENT_DESKTOP is exactly the silent portal failure this milestone
# exists to prevent, so bouncing to the login screen beats starting a desktop
# whose file chooser is wrong. A stub is what lets that be asserted rather than
# assumed.
mkdir -p "$WORK/bin"
for tool in systemctl dbus-update-activation-environment; do
    cat > "$WORK/bin/$tool" <<STUB
#!/bin/sh
exit 0
STUB
    chmod +x "$WORK/bin/$tool"
done

run_session() {
    env -i \
        HOME="$WORK" \
        PATH="$WORK/bin:$PATH" \
        XDG_STATE_HOME="$WORK/state" \
        XDG_CONFIG_HOME="$WORK/config" \
        XDG_SESSION_ID="test-session" \
        ZENITH_COMPOSITOR="$WORK/compositor" \
        sh "$SCRIPT"
}

# ----------------------------------------------------------------- the basics
run_session
[ -f "$WORK/seen.env" ]
check "the script reaches the compositor" $?

# The two desktop names are different strings on purpose: the dispatcher
# lower-cases XDG_CURRENT_DESKTOP to derive its config file name, and the login
# manager records the lower-case session name.
grep -qx 'XDG_CURRENT_DESKTOP=Zenith' "$WORK/seen.env"
check "XDG_CURRENT_DESKTOP is Zenith, capitalised" $?

grep -qx 'XDG_SESSION_DESKTOP=zenith' "$WORK/seen.env"
check "XDG_SESSION_DESKTOP is zenith, lower-case" $?

grep -qx 'DESKTOP_SESSION=zenith' "$WORK/seen.env"
check "DESKTOP_SESSION is set" $?

grep -qx 'XDG_SESSION_TYPE=wayland' "$WORK/seen.env"
check "XDG_SESSION_TYPE is wayland" $?

# ------------------------------------------------------- the toolkit backends
# Set unconditionally rather than inherited: a GDK_BACKEND left in
# /etc/environment by a GNOME-era tweak is how the GNOME portal backend decides it
# is not on a real GNOME session and silently drops interfaces.
for name in GDK_BACKEND MOZ_ENABLE_WAYLAND QT_QPA_PLATFORM; do
    grep -q "^$name=" "$WORK/seen.env"
    check "$name is set" $?
done

grep -qx 'GDK_BACKEND=wayland' "$WORK/seen.env"
check "GDK_BACKEND is wayland even with nothing inherited" $?

# -------------------------------------------- the set -e AND-list trap
# This is the one the design record calls out: under `set -e`, a false test in
# an AND-list aborts the script, so `[ -r f ] && . f` for a file that does not
# exist kills the session before the compositor starts — a confusing way to fail,
# on a machine that is merely unconfigured.
echo "--- checking that a missing overlay file does not abort the session"
rm -f "$WORK/seen.env"
run_session
if [ -f "$WORK/seen.env" ]; then
    check "a missing session env file does not abort the session" 0
else
    check "a missing session env file does not abort the session" 1
fi

# ------------------------------------------------ the overlay is actually read
echo "--- checking that the overlay is sourced when it exists"
mkdir -p "$WORK/config/zenith/session"
printf 'export ZENITH_OVERLAY_WAS_READ=yes\n' \
    > "$WORK/config/zenith/session/zenith-session.env"
rm -f "$WORK/seen.env"
run_session
grep -qx 'ZENITH_OVERLAY_WAS_READ=yes' "$WORK/seen.env"
check "the user's session overlay is sourced" $?

# An overlay that fails must fail loudly rather than being swallowed.
printf 'exit 3\n' > "$WORK/config/zenith/session/zenith-session.env"
rm -f "$WORK/seen.env"
if run_session >/dev/null 2>&1; then
    check "a failing overlay aborts the session before the compositor" 1
else
    check "a failing overlay aborts the session before the compositor" 0
fi
if [ -f "$WORK/seen.env" ]; then
    check "the compositor is not started when the overlay fails" 1
else
    check "the compositor is not started when the overlay fails" 0
fi

# --------------------------------------------------------------- the log line
grep -q 'zenith-session' "$WORK/state/zenith/session.log"
check "the session logs what it did" $?

echo
if [ "$fail" -eq 0 ]; then
    echo "all session checks passed"
else
    echo "$fail session check(s) failed"
fi
exit $fail
