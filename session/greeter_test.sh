#!/bin/sh
# Check the greeter's properties.
#
# The greeter is the one thing on the machine whose failure is unrecoverable: a
# login screen that does not come up is a machine nobody can get into. So the
# property under test is not "does it pick the right mode" — that is a nicety —
# it is "does it come up anyway". Every branch that fails has to fall back to
# plain Weston rather than exit.
#
# It is run rather than unit-tested, for the same reason zenith-session is: the
# properties are about how /bin/sh behaves, and the only way to know that is to
# run it.
#
# Usage: session/greeter_test.sh
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
GREETER="$REPO_ROOT/session/zenith-greeter"
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

# A stub weston that records how it was called and hands back the config it was
# given, so a check can assert on the file the greeter actually wrote.
cat > "$WORK/weston" <<'STUB'
#!/bin/sh
echo "$@" > "$WESTON_ARGS"
case "$1" in
    --config=*) cat "${1#--config=}" > "$WESTON_CONFIG" ;;
esac
STUB
chmod +x "$WORK/weston"

# A stub modetest whose output is supplied per case.
make_modetest() {
    cat > "$WORK/modetest" <<STUB
#!/bin/sh
cat <<'REPORT'
$1
REPORT
STUB
    chmod +x "$WORK/modetest"
}

run_greeter() {
    WESTON_ARGS="$WORK/args" WESTON_CONFIG="$WORK/weston.ini" \
    ZENITH_WESTON="$WORK/weston" ZENITH_MODETEST="$WORK/modetest" \
    ZENITH_KEYBOARD="${KEYBOARD_CONF:-$WORK/no-keyboard-file}" \
        "$GREETER" >"$WORK/out" 2>&1 || echo "exit=$?" >> "$WORK/out"
    cat "$WORK/out"
}

# ------------------------------------------------------- it always comes up
echo "checking the greeter always comes up"

make_modetest ""
rc=0; run_greeter >/dev/null || rc=$?
check "it runs with modetest producing nothing" $rc

make_modetest "Connector 1 (eDP-1) disconnected"
rc=0; run_greeter >/dev/null || rc=$?
check "it runs when no connector is connected" $rc

make_modetest "this is not a modetest report at all"
rc=0; run_greeter >/dev/null || rc=$?
check "it runs when modetest output cannot be parsed" $rc

# The one that matters most: the tool is missing entirely.
rc=0; WESTON_ARGS="$WORK/args" WESTON_CONFIG="$WORK/weston.ini" \
    ZENITH_WESTON="$WORK/weston" ZENITH_MODETEST="$WORK/does-not-exist" \
    "$GREETER" >/dev/null 2>&1 || rc=$?
check "it runs when modetest is not installed" $rc

rc=0; grep -q -- '--config=' "$WORK/args" || rc=$?
check "weston is always started with a config" $rc

# ------------------------------------------------------------- mode selection
echo
echo "checking mode selection"

make_modetest "Connector 1 (eDP-1) connected (preferred) modes: 1920x1080 1280x720
  1920x1080  60.00 +hsync +vsync"
run_greeter >/dev/null

rc=0; grep -q '^mode=1920x1080$' "$WORK/weston.ini" || rc=$?
check "it takes the first mode of the first connected connector" $rc

# A disconnected first connector must not win over the connected second one,
# which is the laptop-with-an-external-display case.
make_modetest "Connector 1 (DP-1) disconnected modes: 3840x2160
Connector 2 (eDP-1) connected (preferred) modes: 2880x1800 1920x1200"
run_greeter >/dev/null

rc=0; grep -q '^mode=2880x1800$' "$WORK/weston.ini" || rc=$?
check "it skips a disconnected connector" $rc

# Nothing parseable means no mode line at all, and Weston chooses.
make_modetest "Connector 1 (eDP-1) connected (preferred) modes: none"
run_greeter >/dev/null

rc=0; ! grep -q '^mode=' "$WORK/weston.ini" || rc=$?
check "it writes no mode when there is none to read" $rc

rc=0; grep -q 'letting weston choose' "$WORK/out" || rc=$?
check "it says when it is letting weston choose" $rc

# ----------------------------------------------------------------- the config
echo
echo "checking the weston config"

make_modetest "Connector 1 (eDP-1) connected modes: 1920x1080"
run_greeter >/dev/null

rc=0; grep -q '^locker=false$' "$WORK/weston.ini" || rc=$?
check "the session locker is off" $rc

rc=0; grep -q 'command=.*/usr/bin/quickshell -p /usr/share/zenith/greeter$' \
    "$WORK/weston.ini" || rc=$?
check "the login surface is named" $rc

# The greeter has no session to lock. A kiosk Weston that thinks it is a desktop
# is a window manager with no windows.
rc=0; grep -q 'locker=true' "$WORK/weston.ini" && rc=1 || rc=0
check "the locker is not turned on by anything" $rc

# ----------------------------------------------------------------- the keymap
echo
echo "checking the keymap"

printf 'XKBLAYOUT="de"\nXKBVARIANT="nodeadkeys"\nXKBOPTIONS=""\n' > "$WORK/keyboard"
KEYBOARD_CONF="$WORK/keyboard" run_greeter >/dev/null

rc=0
grep -q '^keymap_layout=de$' "$WORK/weston.ini" &&
    grep -q '^keymap_variant=nodeadkeys$' "$WORK/weston.ini" || rc=1
check "the layout in /etc/default/keyboard reaches weston" $rc

rc=0; ! grep -q '^keymap_options=' "$WORK/weston.ini" || rc=$?
check "an empty option is not written" $rc

run_greeter >/dev/null
rc=0; ! grep -q '^keymap_' "$WORK/weston.ini" || rc=$?
check "without a keyboard file weston keeps its default and still starts" $rc

# --------------------------------------------------------------- the SDDM theme
echo
echo "checking the SDDM theme"
THEME="$REPO_ROOT/session/sddm/themes/zenith"

# Without this line SDDM looks for the Qt5 greeter, which 26.04 does not ship,
# and quietly shows its built-in theme instead.
rc=0; grep -q '^QtVersion=6$' "$THEME/metadata.desktop" || rc=$?
check "the theme asks for the Qt6 greeter" $rc

# qmllint cannot see the names SDDM injects (sddm, userModel, sessionModel), so
# it warns about them; only a real error fails.
QMLLINT=$(command -v qmllint6 || command -v qmllint || echo /usr/lib/qt6/bin/qmllint)
if [ -x "$QMLLINT" ]; then
    rc=0; lint=$("$QMLLINT" "$THEME/Main.qml" "$THEME/Chip.qml" 2>&1) || { rc=1; echo "$lint"; }
    check "the theme's QML parses" $rc
else
    echo "skip  — qmllint is not installed, so the theme's QML was not parsed"
fi

# The theme cannot run without a display manager, so it runs here against
# stand-ins for what SDDM injects, on Qt's offscreen platform.
QMLTEST=$(command -v qmltestrunner6 || command -v qmltestrunner || echo /usr/lib/qt6/bin/qmltestrunner)
if [ -x "$QMLTEST" ]; then
    rc=0
    lint=$(QT_QPA_PLATFORM=offscreen QT_QUICK_BACKEND=software "$QMLTEST" \
        -input "$REPO_ROOT/session/sddm/theme_test.qml" 2>&1) || { rc=1; echo "$lint"; }
    check "the theme logs in, reports a failure and orders sessions (mock SDDM)" $rc
else
    echo "skip  — qmltestrunner is not installed, so the theme was not run"
fi

echo
if [ "$fail" -eq 0 ]; then
    echo "all greeter checks passed"
else
    echo "$fail greeter check(s) failed"
    exit $fail
fi
