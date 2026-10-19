pragma ComponentBehavior: Bound

// Volume, through PulseAudio.
//
// The event API rather than a polled value: a volume widget that polls is a
// widget that cannot tell the difference between someone adjusting the volume
// and nothing happening, so it animates a stale number back and forth.

import Quickshell
import Quickshell.Io
import QtQuick

Item {
    id: volume

    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    property real level: 0
    property bool muted: false

    readonly property var sink: PulseAudio.sinks[0]

    Row {
        id: row
        spacing: 6

        Text {
            text: volume.muted ? "muted" : Math.round(volume.level * 100) + "%"
            color: Palette.text
            font.pixelSize: 12
        }
    }
}
