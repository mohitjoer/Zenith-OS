import QtQuick
import Quickshell
import "welcome"

// Zenith first-run welcome. A normal floating window (not a layer-shell surface);
// the Hyprland window rule floats and centres it by its title. Entry file at the
// shell root so the qs.* imports resolve (see hub.qml); started by
// session/zenith-welcome, which runs it once on first login, holds the lock so it
// never double-fires, and writes the done-marker after this process exits -- so
// the tour shows exactly once. Quit on every close (Finish, Skip, Escape, or the
// compositor) so the lock always releases and the marker write runs.
ShellRoot {
    FloatingWindow {
        id: win
        // Not translated: the window rule matches this exact string, and a
        // translated title would leave the tour tiled in every other language.
        title: "Welcome to Zenith"
        // Same fit-clamp as the hub window: the rule floats this at 1180x760,
        // which a 720p-class screen cannot hold. Hyprland clamps the rule's
        // size into maximumSize and centres the result, so a small screen gets
        // a window that fits; minimumSize shrinks with it.
        readonly property int fitW: win.screen ? Math.min(1180, win.screen.width - 24) : 1180
        readonly property int fitH: win.screen ? Math.min(760, win.screen.height - 56) : 760
        readonly property bool cramped: win.fitW < 1180 || win.fitH < 760
        minimumSize: Qt.size(Math.min(980, win.fitW), Math.min(640, win.fitH))
        maximumSize: win.cramped ? Qt.size(win.fitW, win.fitH) : Qt.size(16777215, 16777215)
        onClosed: Qt.quit()

        Welcome {
            anchors.fill: parent
        }
    }
}
