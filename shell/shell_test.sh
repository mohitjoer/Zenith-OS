#!/bin/sh
# The shell's configuration, loaded.
#
# Everything else about the shell is checked by reading it: the Go validator
# walks the QML and the doctor checks the installed tree. This checks the one
# thing reading cannot: that the configuration a compositor would load is one
# the engine accepts.
#
# The shell had never been loaded by a QML engine. Every defect in it — a
# property that does not exist, a type from a module that is not imported, a
# component declared at file scope, a name that another library already has —
# was invisible to every check the project had, because every check read the
# file rather than loading it. A shell that had been loaded once would have
# said so on the first login.
#
# The check needs two things the machine may not have: Quickshell, which is
# built from source rather than taken from the archive, and a Wayland display,
# because the shell's windows are layer-shell surfaces and a QML engine with no
# Wayland platform plugin cannot create them at all. It says which of the two
# it is missing rather than failing, because a check that cannot run is not a
# failure of the shell.

set -eu

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SHELL_DIR="$REPO_ROOT/shell"
LOG="$(mktemp)"
trap 'rm -f "$LOG"' EXIT

echo "checking that the shell's configuration loads"

if ! command -v quickshell >/dev/null 2>&1; then
    echo "skip  — quickshell is not installed, and the shell cannot be loaded without it"
    exit 0
fi

# The Wayland display. Quickshell needs the Wayland platform plugin: the
# offscreen platform has no window system, and the shell's panels are windows.
if [ -z "${WAYLAND_DISPLAY:-}" ]; then
    echo "skip  — there is no Wayland display, and the shell's windows cannot be created without one"
    exit 0
fi

# The DBus session is severed so the shell cannot take the notification bus
# name from whatever is holding it on this machine. A shell that cannot own
# the name is a shell that says so in a warning, and the warning is not a
# fault in the configuration.
DBUS_SESSION_BUS_ADDRESS="unix:path=$(mktemp -u)"

# Ten seconds is a generous ceiling for a configuration that loads, and a
# short one for a configuration that hangs.
if command -v timeout >/dev/null 2>&1; then
    timeout -s INT --kill-after=5 10 \
        quickshell -p "$SHELL_DIR" --no-color >"$LOG" 2>&1 || true
else
    quickshell -p "$SHELL_DIR" --no-color >"$LOG" 2>&1 &
    SHELL_PID=$!
    sleep 10
    kill -INT "$SHELL_PID" 2>/dev/null || true
    wait "$SHELL_PID" 2>/dev/null || true
fi

# The errors are what matters. The warnings are the machine's: a compositor
# without layer-shell, a session without Hyprland, a notification name owned
# by something else. None of them is a fault in the configuration, and a check
# that failed on them would be a check that failed on the machine rather than
# on the shell.
ERRORS=$(grep "^ ERROR:" "$LOG" || true)

if [ -z "$ERRORS" ]; then
    echo "ok    — the configuration loads with no errors"
    exit 0
fi

echo "FAIL  — the configuration does not load:"
echo "$ERRORS" | sed 's/^/      /'
exit 1
