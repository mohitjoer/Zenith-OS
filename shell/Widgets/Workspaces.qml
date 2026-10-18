pragma ComponentBehavior: Bound

// Workspace indicators, read from Hyprland.
//
// Read-only. A workspace widget that can switch workspaces is a second way to
// change them, and the compositor is the only place that state lives.

import Quickshell
import Quickshell.Hyprland
import QtQuick

Item {
    id: workspaces

    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    Row {
        id: row
        spacing: 6

        Repeater {
            model: Hyprland.workspaces

            delegate: Rectangle {
                required property var model

                width: 12
                height: 12
                radius: 6
                // The active workspace is the compositor's answer, not a local
                // flag. Two widgets tracking "which one is active" is one too
                // many.
                color: model.active ? "#4c8bf5" : "#707070"
            }
        }
    }
}
