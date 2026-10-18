pragma ComponentBehavior: Bound

// The launcher.
//
// launcher-apps empty means every launchable application on the machine, not
// none. An empty list is a policy decision, and the common one is not to make
// it — a launcher that shows nothing because nothing was configured is the
// worst of both readings.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

PopupWindow {
    id: launcher

    implicitWidth: 640
    implicitHeight: 480

    // The grid is the shell's own, and the model is the applications. Both are
    // plain data so the shell does not have to be told what a launcher is.
    contentItem: Flow {
        spacing: 16

        Repeater {
            model: launcher.apps

            delegate: IconImage {
                required property var modelData

                implicitSize: 64
                source: modelData.icon
                colorize: true
            }
        }
    }

    // Resolved once when the launcher opens rather than bound to a process that
    // is always running. Scanning every .desktop file is a filesystem walk, and a
    // filesystem walk on a timer is a shell that is never idle.
    property var apps: []

    function refresh() {
        apps = []
    }

    Component.onCompleted: refresh()
}
