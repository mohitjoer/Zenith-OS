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
printf '%s\n' "\$@" > "$WORK/seen.args"
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

# The compositor is given Zenith's configuration, because a compositor started
# without it runs on Hyprland's own defaults — a desktop with no keybindings
# at all, where the launcher cannot be opened and the switcher cannot be driven.
grep -qx -- '-c' "$WORK/seen.args"
check "the compositor is started with a config" $?

grep -qx -- '/usr/share/zenith/hyprland/hyprland.conf' "$WORK/seen.args"
check "the compositor is started with Zenith's configuration" $?

# The user's own configuration is created, because Hyprland treats a source
# that resolves to nothing as a configuration error and refuses to start.
[ -f "$WORK/.config/zenith/hyprland/zenith.conf" ]
check "the user's compositor configuration is created" $?

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

# ------------------------------------------------------------------- units
# A unit with a key in the wrong section is not a unit that fails loudly. systemd
# logs "Unknown key ... ignoring" at the level that nobody reads, the limiter it
# was meant to configure is silently not applied, and the service restarts
# forever instead of giving up. This caught StartLimitIntervalSec sitting in
# [Service] rather than [Unit].
#
# Skipped when systemd-analyze is absent, because a packaging machine without it
# should not fail over a check it cannot run.
echo
echo "checking the units"
if command -v systemd-analyze >/dev/null 2>&1; then
    for unit in "$REPO_ROOT"/session/units/*.service "$REPO_ROOT"/session/units/*.target; do
        # Errors that are about the tree rather than about this unit: hyprland
        # is not installed on a packaging machine, and the two binaries are M6's.
        #
        # The targets are missing too, because the unit path is the repository
        # and systemd resolves references against the installed units. That is
        # what the session's own target is for: it is installed, and the
        # references resolve there.
        PROBLEMS=$(systemd-analyze verify "$unit" 2>&1 \
            | grep -vE "(zenith-session|graphical-session)\.target not found|not executable" || true)

        rc=0; [ -z "$PROBLEMS" ] || rc=$?
        check "$(basename "$unit") has no unknown keys or bad directives" $rc

        if [ -n "$PROBLEMS" ]; then
            echo "$PROBLEMS" | sed 's/^/      /'
        fi
    done

    # The start order is the part of the session that is easiest to get wrong and
    # quietest when it is wrong, so it is asserted rather than eyeballed.
    #
    # Each dependency is checked as a member of an After= list rather than as a
    # line of its own: the spec puts all three on one line, and a check that
    # only ever matched a lone line would pass against the wrong unit.
    # An After=/Requires= value can be one unit or several. The first one shares
    # a field with the key ("After=hyprland-session.target"), the rest are separate
    # fields, so both have to be scanned or a single-dependency unit silently
    # matches nothing.
    has_dep() {
        awk -v key="$2" -v want="$3" '
            index($0, key "=") == 1 {
                split($1, head, "=")
                for (i = 2; i <= length(head); i++)
                    if (head[i] == want) found = 1
                for (i = 2; i <= NF; i++)
                    if ($i == want) found = 1
            }
            END { exit !found }' "$1"
    }
    after() { has_dep "$1" After "$2"; }
    requires() { has_dep "$1" Requires "$2"; }

    SHELL_UNIT="$REPO_ROOT/session/units/zenith-shell.service"
    PORTAL_UNIT="$REPO_ROOT/session/units/zenith-portal.service"

    # The shell waits for the dispatcher: the dispatcher resolves its backends
    # once at startup and never re-reads them, so a shell that starts first asks
    # the bus for a portal that has not been decided on yet.
    rc=0; after "$SHELL_UNIT" zenith-portal.service || rc=$?
    check "the shell is ordered after the portal dispatcher" $rc

    rc=0; after "$SHELL_UNIT" zenith-session.target || rc=$?
    check "the shell is ordered after the session" $rc

    # Audio: the shell's volume widget has nothing to talk to without it.
    rc=0; after "$SHELL_UNIT" wireplumber.service || rc=$?
    check "the shell is ordered after the audio policy" $rc

    # Requires, not just After. Ordered-only leaves the shell running against a
    # display that is no longer there.
    rc=0; requires "$SHELL_UNIT" zenith-session.target || rc=$?
    check "the shell requires the session's target" $rc

    rc=0; after "$PORTAL_UNIT" zenith-session.target || rc=$?
    check "the portal dispatcher is ordered after the session" $rc

    # Type=dbus is what makes "started" mean the bus name is held, which is the
    # only moment the portal is genuinely usable. Type=exec would leave the shell
    # ordered against a process that has not yet chosen its backends.
    rc=0; grep -q '^Type=dbus$' "$PORTAL_UNIT" || rc=$?
    check "the portal dispatcher is typed dbus, not exec" $rc

    rc=0; grep -q '^BusName=org.freedesktop.portal.Desktop$' "$PORTAL_UNIT" || rc=$?
    check "the portal dispatcher claims the portal bus name" $rc

    for unit in zenith-shell zenith-portal; do
        rc=0; grep -q '^PartOf=graphical-session.target$' \
            "$REPO_ROOT/session/units/$unit.service" || rc=$?
        check "$unit is part of the graphical session" $rc

        # WantedBy the session's own target, not graphical-session.target: the
        # session target is what the session actually pulls up.
        rc=0; grep -q '^WantedBy=zenith-session.target$' \
            "$REPO_ROOT/session/units/$unit.service" || rc=$?
        check "$unit is wanted by the session's target" $rc
    done

    # The session's target is what pulls either unit up at all, and it is
    # Zenith's own because Ubuntu's Hyprland ships no systemd units: a unit
    # that requires a target nobody has cannot start.
    rc=0; grep -q 'zenith-shell.service' \
        "$REPO_ROOT/session/units/zenith-session.target" || rc=$?
    check "the session target wants the shell" $rc

    rc=0; grep -q 'zenith-portal.service' \
        "$REPO_ROOT/session/units/zenith-session.target" || rc=$?
    check "the session target wants the portal dispatcher" $rc

    # And something has to start it, at a moment when the display exists.
    rc=0; grep -q 'exec-once = /usr/share/zenith/session/zenith-activate' \
        "$REPO_ROOT/session/hyprland/hyprland.conf" || rc=$?
    check "the compositor's configuration starts the session" $rc

    rc=0; [ -x "$REPO_ROOT/session/zenith-activate" ] || rc=$?
    check "the activation script is executable" $rc

    # The ordering cycle, checked in the user manager where the units live.
    #
    # `systemd-analyze verify` on a file resolves references against the
    # installed units and says nothing about a cycle that only exists once
    # they are all installed, and a cycle is resolved by systemd by dropping a
    # job — which means the portal dispatcher silently does not start.
    CYCLES=$(systemd-analyze --user verify \
        "$REPO_ROOT"/session/units/*.service "$REPO_ROOT"/session/units/*.target 2>&1 \
        | grep -E "ordering cycle" | grep -E "zenith" || true)
    rc=0; [ -z "$CYCLES" ] || rc=$?
    check "the units have no ordering cycle" $rc
    if [ -n "$CYCLES" ]; then
        echo "$CYCLES" | sed 's/^/      /'
    fi
else
    echo "skip  — systemd-analyze is not installed"
fi

# ------------------------------------------------------------- the settings app
#
# zenith-hub lives in this directory and has no test of its own, and the one
# thing it exists to do is export ZENITH_SCHEMA_DIR — which settingsCatalogue
# reads. It pointed at $PREFIX/schemas, a directory the package never creates,
# so the wrapper set an override onto a path that resolves nowhere, and the Hub
# started against an empty settings catalogue.
#
# The two paths are written in two languages, which is exactly how they came
# apart, so they are compared rather than trusted.
hub_schema=$(grep -o 'ZENITH_SCHEMA_DIR:-[^}]*' "$REPO_ROOT/session/zenith-hub" |
    sed 's/.*:-//')
tool_schema=$(grep -o 'const settingsDir = "[^"]*"'     "$REPO_ROOT/desktop/cmd/zenith/config.go" | sed 's/.*"\(.*\)"/\1/')
rc=0
if [ -z "$hub_schema" ] || [ "$hub_schema" != "$tool_schema" ]; then
    echo "  hub exports:   ${hub_schema:-<none>}"
    echo "  zenith reads:  ${tool_schema:-<none>}"
    rc=1
fi
check "the hub exports the schema directory zenith reads" $rc

# ------------------------------------------------------- the compositor's files
# The configuration is several files that hyprland.conf sources. Hyprland's own
# parser is the only judge of a rule's syntax, so it is asked, with an empty home
# so none of the optional state files under ~/.config/zenith/hyprland exist.
if command -v Hyprland >/dev/null 2>&1; then
    rc=0
    HOME="$WORK/empty-home" mkdir -p "$WORK/empty-home"
    HOME="$WORK/empty-home" Hyprland --verify-config -c "$REPO_ROOT/session/hyprland/hyprland.conf" 2>&1 \
        | grep -q 'config ok' || rc=$?
    check "the compositor's configuration parses" $rc
else
    echo "skip  — Hyprland is not installed"
fi

# The helper scripts hyprland.conf binds, tested against stubs and fixtures.
for t in workspace_test.sh monitor_test.sh; do
    rc=0; bash "$REPO_ROOT/session/scripts/$t" >/dev/null 2>&1 || rc=$?
    check "session/scripts/$t passes" $rc
done

echo
if [ "$fail" -eq 0 ]; then
    echo "all session checks passed"
else
    echo "$fail session check(s) failed"
fi
exit $fail
