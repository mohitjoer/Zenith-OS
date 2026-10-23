#!/bin/sh
# The dock is built with something in it on a stock Zenith box.
#
#   dock_probe.sh [TREE]    TREE is a shell tree (default shell/); a copy is probed
#
# The bug this pins: `dock.pinned` is [] on a fresh desktop, so DockBand falls back to
# Dock.starterPins(). That list named Arch apps (kitty, chromium, nautilus), none of
# which exists here, and it read the desktop DB through a non-reactive call, so a band
# built before the DB scan (apps=0 at start) stayed empty for good. An empty DockBand
# is 1x46 px of nothing: the dock surface is mapped and the user sees no dock.
#
# Same sandbox as load.sh (env -i, throwaway XDG_*, offscreen Qt, no Wayland, no D-Bus),
# with its own desktop DB so the result does not depend on what is installed. Offscreen
# has no window-manager backend and Quickshell.WindowManager throws bad_function_call
# there, so the copy's Wm.qml has that one binding stubbed. PanelWindow cannot be built
# offscreen, so this probes DockBand, the part that holds the islands, not the surface.
tree=${1:-$(dirname "$0")/../../shell}
command -v quickshell >/dev/null || { echo "dock: skip, no quickshell"; exit 0; }
t=$(mktemp -d) && trap 'rm -rf "$t"' EXIT
mkdir -m 700 "$t/run"
cp -r "$tree" "$t/tree" || exit 2
sed -i 's/WindowManager\.windowsets/[]/' "$t/tree/Ui/Singletons/Wm.qml"
mkdir -p "$t/share/applications"
for id in com.mitchellh.ghostty brave-browser org.gnome.Nautilus; do
    printf '[Desktop Entry]\nType=Application\nName=%s\nExec=true\n' "$id" >"$t/share/applications/$id.desktop"
done
cat >"$t/tree/shell.qml" <<'EOF'
import Quickshell
import QtQuick
import qs.Services
import "modules/dock"

ShellRoot {
    id: probe
    property int ticks: 0
    DockBand { id: band; edge: "bottom"; screenName: "probe" }
    Timer {
        interval: 250; running: true; repeat: true
        onTriggered: if (++probe.ticks >= 32 || band.pins.length >= 3) {
            console.log("PROBE " + JSON.stringify({ pins: band.pins, width: band.implicitWidth }));
            Qt.quit();
        }
    }
}
EOF
env -i PATH="$PATH" LANG=C.UTF-8 HOME="$t" XDG_RUNTIME_DIR="$t/run" XDG_CONFIG_HOME="$t/c" XDG_CACHE_HOME="$t/k" \
    XDG_STATE_HOME="$t/s" XDG_DATA_HOME="$t/d" XDG_DATA_DIRS="$t/share" QT_QPA_PLATFORM=offscreen \
    timeout -k 2 14 quickshell -p "$t/tree" >"$t/log" 2>&1 </dev/null || true
got=$(sed 's/\x1b\[[0-9;]*m//g' "$t/log" | sed -n 's/.*PROBE //p' | head -1)
want='{"pins":["com.mitchellh.ghostty","brave-browser","org.gnome.Nautilus"],"width":170}'
[ "$got" = "$want" ] && { echo "dock: ok, an unconfigured dock holds terminal, browser, files ($tree)"; exit 0; }
echo "dock: FAIL, want $want"
echo "      got  ${got:-nothing (see below)}"
[ -n "$got" ] || sed 's/\x1b\[[0-9;]*m//g' "$t/log" | grep -E 'FATAL|ERROR|TypeError|ReferenceError|is not' | sort -u | head -5
exit 1
