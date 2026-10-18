pragma ComponentBehavior: Bound

// The status area: one item for every icon the session needs to show.
//
// All of it in one widget rather than one widget per icon, because the status
// area is a single row with a single layout and splitting it means four
// PlaceholderLayouts that each have to know what the other three are doing.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

Item {
    id: status

    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    Row {
        id: row
        spacing: 10

        StatusIcon {
            label: "vol"
            // Three states rather than two, because a muted sink at full volume
            // and an unmuted one at full volume are different states and an icon
            // that cannot tell them apart is decoration.
            state: volume.muted ? "muted"
                : volume.level < 0.001 ? "zero"
                : volume.level < 0.5 ? "low" : "high"
        }

        StatusIcon {
            label: "net"
            state: network.connected ? "on" : "off"
        }

        StatusIcon {
            label: "bat"
            visible: battery.present
            state: battery.charging ? "chg" : "bat"
        }

        StatusIcon {
            label: "bt"
            visible: bluetooth.powered
            state: bluetooth.connected ? "on" : "idle"
        }
    }
}
