pragma ComponentBehavior: Bound

// A string setting.
//
// Most of the string settings in Zenith are file paths, so a bare text field is
// not the whole story: a path field that does not tell you the file exists is a
// field that produces a wallpaper which is a broken-image placeholder.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

Rectangle {
    id: control

    required property var key

    implicitHeight: 36
    radius: 6
    color: Theme.surface
    border.width: 1
    border.color: Theme.border

    readonly property string shown: key.value.replace(/^'(.*)'$/, "$1")

    // A GVariant string is quoted and a file:// URI is not, so a field that is
    // left as-is would show the user quotes they did not type and would write
    // them back.
    TextInput {
        id: input
        anchors.fill: parent
        anchors.margins: 8
        verticalAlignment: TextInput.AlignVCenter
        text: control.shown
        color: Theme.text
        font.pixelSize: 13
        selectByMouse: true

        onActiveFocusChanged: if (!activeFocus) {
            // An empty field is the schema's empty string rather than "", so a
            // cleared wallpaper means "the shell draws its own" and not "a file
            // called nothing".
            const written = text.length === 0 ? "''" : "'" + text + "'"
            Zenith.set(control.key, written)
        }
    }
}
