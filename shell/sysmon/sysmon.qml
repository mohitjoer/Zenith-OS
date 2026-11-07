import QtQuick
import Quickshell
import Quickshell.Io

ShellRoot {
    FloatingWindow {
        id: win
        title: "Resource Monitor — Asura OS"
        readonly property int fitW: (win.screen && win.screen.width > 0) ? Math.min(940, win.screen.width - 40) : 940
        readonly property int fitH: (win.screen && win.screen.height > 0) ? Math.min(640, win.screen.height - 60) : 640

        implicitWidth: fitW
        implicitHeight: fitH
        minimumSize: Qt.size(760, 500)
        color: "transparent"

        App {
            id: app
            anchors.fill: parent
        }
    }
}
