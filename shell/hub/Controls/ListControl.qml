pragma ComponentBehavior: Bound

// A string-list setting.
//
// Edited as one comma-separated line rather than as a list widget, and that is a
// deliberate reduction: this project has a launcher, a dock and a widget
// catalogue to build first, and a text field that edits the same value correctly
// is better than a list widget that edits it badly.
//
// The value is a GVariant tuple — "['a', 'b']" — so it is shown without the
// brackets and the quotes, and written back with them.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

Rectangle {
    id: control

    required property var key

    implicitHeight: 36
    radius: 6
    color: Palette.surface
    border.width: 1
    border.color: Palette.border

    readonly property string shown: key.value
        .replace(/^\[/, "")
        .replace(/\]$/, "")
        .replace(/'/g, "")

    TextInput {
        id: input
        anchors.fill: parent
        anchors.margins: 8
        verticalAlignment: TextInput.AlignVCenter
        text: control.shown
        color: Palette.text
        font.pixelSize: 13
        selectByMouse: true

        onActiveFocusChanged: if (!activeFocus) {
            // Reassembled as a GVariant tuple. An empty field writes "[]" rather
            // than "", because an empty string is not a list and GSettings would
            // reject it.
            const parts = text.split(",")
                .map(function (p) { return p.trim() })
                .filter(function (p) { return p.length > 0 })
            Zenith.set(control.key, "[" + parts.map(function (p) {
                return "'" + p + "'"
            }).join(", ") + "]")
        }
    }
}
