#!/bin/sh
# The shell's configurations, loaded.
#
# Everything else about the shell is checked by reading it: the Go validator
# walks the QML and the doctor checks the installed tree. This checks the one
# thing reading cannot: that a configuration a compositor would load is one the
# engine accepts.
#
# Both windows are checked, because both are QML configurations and both had
# defects that no reader saw. The shell's had never been loaded by an engine at
# all — a property that does not exist, a type from a module that is not
# imported, a component declared at file scope, a name another library already
# has. The Hub's had been loaded, and it still assigned a readonly property,
# read a property off an object that is not a property, and loaded its editors
# by URL, which gives a file no import of its own directory — so every settings
# control failed to appear and the window looked like an empty page.
#
# Each window is named by its entry file rather than by its directory, because
# the two windows share one tree: the Hub is drawn with the same component kit
# as the desktop, and the kit's files reach each other through `qs.modules.*`,
# which only resolves when the configuration's root is the directory both entry
# files are in.
#
# The check needs two things the machine may not have: Quickshell, which is
# built from source rather than taken from the archive, and a Wayland display,
# because the windows are layer-shell surfaces and a QML engine with no Wayland
# platform plugin cannot create them at all. It says which of the two it is
# missing rather than failing, because a check that cannot run is not a failure
# of the shell.

set -eu

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOG="$(mktemp)"
trap 'rm -f "$LOG"' EXIT

if ! command -v quickshell >/dev/null 2>&1; then
    echo "skip  — quickshell is not installed, and neither window can be loaded without it"
    exit 0
fi

# The Wayland display. Quickshell needs the Wayland platform plugin: the
# offscreen platform has no window system, and these are windows.
if [ -z "${WAYLAND_DISPLAY:-}" ]; then
    echo "skip  — there is no Wayland display, and the windows cannot be created without one"
    exit 0
fi

# The DBus session is severed so a window cannot take the notification bus name
# from whatever is holding it on this machine. A window that cannot own the name
# says so in a warning, and the warning is not a fault in the configuration.
# Exported, because an assignment that is not exported is not an environment for
# the child and severs nothing.
DBUS_SESSION_BUS_ADDRESS="unix:path=$(mktemp -u)"
export DBUS_SESSION_BUS_ADDRESS

# A module the machine does not have is reported as a chain of types that could
# not be resolved, one per file in the import graph, with the module itself
# named at the very end. The chain says the same thing forty times and names the
# package nowhere, so it is recognised and said once.
missing_modules() {
    sed -n 's/.*module "\([^"]*\)" is not installed.*/\1/p' "$LOG" | sort -u
}

# check_load <what> <directory>
#
# Ten seconds is a generous ceiling for a configuration that loads, and a short
# one for a configuration that hangs.
check_load() {
    what=$1
    dir=$2

    if command -v timeout >/dev/null 2>&1; then
        timeout -s INT --kill-after=5 10 \
            quickshell -p "$dir" --no-color >"$LOG" 2>&1 || true
    else
        quickshell -p "$dir" --no-color >"$LOG" 2>&1 &
        window_pid=$!
        sleep 10
        kill -INT "$window_pid" 2>/dev/null || true
        wait "$window_pid" 2>/dev/null || true
    fi

    missing=$(missing_modules)
    if [ -n "$missing" ]; then
        echo "FAIL  — $what imports a QML module this machine does not have:"
        for module in $missing; do
            echo "      $module"
        done
        echo
        echo "      The modules the shell needs are listed in system/packages.yaml."
        echo "      On Ubuntu the package for a module is qml6-module- followed by"
        echo "      the module's name in lower case; on an Arch development machine"
        echo "      it is qt6- followed by the module's name in lower case."
        return 1
    fi

    # The errors are what matters. The warnings are the machine's: a compositor
    # without layer-shell, a session without Hyprland, a notification name owned
    # by something else, a branding file that is installed rather than checked
    # out. None of them is a fault in the configuration, and a check that failed
    # on them would be a check that failed on the machine.
    errors=$(grep "^ ERROR:" "$LOG" || true)
    if [ -n "$errors" ]; then
        echo "FAIL  — $what does not load:"
        printf '%s\n' "$errors" | sed 's/^/      /'
        return 1
    fi

    echo "ok    — $what loads with no errors"
    return 0
}

rc=0
check_load "the shell" "$REPO_ROOT/shell/shell.qml" || rc=$?
check_load "the Hub" "$REPO_ROOT/shell/hub.qml" || rc=$?
exit $rc
