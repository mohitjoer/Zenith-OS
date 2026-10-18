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
            // Four states rather than two, because a muted sink at full volume
            // and an unmuted one at full volume are different states, and an
            // icon that cannot tell them apart is decoration.
            glyph: volume.muted ? "volume-mute"
                : volume.level < 0.001 ? "volume-mute"
                : volume.level < 0.5 ? "volume-low"
                : "volume-high"
        }

        StatusIcon {
            label: "net"
            glyph: network.connected ? "wifi" : ""
        }

        StatusIcon {
            label: "bat"
            visible: battery.present
            glyph: battery.charging ? "battery-charging"
                : battery.charge < 0.1 ? "battery-empty"
                : battery.charge < 0.5 ? "battery-quarter"
                : battery.charge < 0.9 ? "battery-half"
                : "battery-full"
        }

        StatusIcon {
            label: "bt"
            visible: bluetooth.powered
            glyph: bluetooth.connected ? "bluetooth" : ""
        }

        // The lock. Only while the session is locked, which is the only time it
        // says anything — an always-present padlock is a claim that the machine
        // is locked, and it is not.
        StatusIcon {
            visible: idle.locked
            glyph: "lock"
        }
    }
}
