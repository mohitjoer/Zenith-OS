pragma ComponentBehavior: Bound

// The clock.
//
// The format is built from the locale rather than hard-coded, because a shell
// that prints "14:32" to someone whose locale says otherwise is a shell that
// has decided what their language is.

import Quickshell
import Quickshell.Widgets
import QtQuick

// The shell's own module, for the settings singleton. A file in a subdirectory
// cannot see a type from the directory above it by proximity the way a file
// beside it can, so the module is imported by name.
import qs

Item {
    id: clock

    implicitWidth: label.implicitWidth
    implicitHeight: label.implicitHeight

    // A Date and not a DateTime: the QML value type for the date and time is
    // not one this shell can count on, and the JavaScript one is the same
    // clock to the second.
    property string format: Qt.formatTime(new Date(), Qt.locale().timeFormat(Locale.ShortFormat))

    // Rebuilt on a timer rather than on a one-shot. A one-shot taken once at
    // start-up is a clock that stops at whatever time the session began.
    Timer {
        interval: 1000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: clock.format = Qt.formatTime(
            new Date(), Qt.locale().timeFormat(Locale.ShortFormat))
    }

    Text {
        id: label
        anchors.fill: parent
        text: clock.format
        color: Config.dark ? Theme.text : Theme.textDim
        font.pixelSize: 13
    }
}
