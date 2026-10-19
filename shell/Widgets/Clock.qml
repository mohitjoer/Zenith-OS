pragma ComponentBehavior: Bound

// The clock.
//
// The format is built from the locale rather than hard-coded, because a shell
// that prints "14:32" to someone whose locale says otherwise is a shell that
// has decided what their language is.

import Quickshell
import Quickshell.Widgets
import QtQuick

Item {
    id: clock

    implicitWidth: label.implicitWidth
    implicitHeight: label.implicitHeight

    property string format: Qt.formatTime(DateTime.currentDate(), Qt.locale().timeFormat(Locale.ShortFormat))

    // Rebuilt on a timer rather than on a one-shot. A one-shot taken once at
    // start-up is a clock that stops at whatever time the session began.
    Timer {
        interval: 1000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: clock.format = Qt.formatTime(
            DateTime.currentDate(), Qt.locale().timeFormat(Locale.ShortFormat))
    }

    Text {
        id: label
        anchors.fill: parent
        text: clock.format
        color: config.dark ? Palette.text : Palette.textDim
        font.pixelSize: 13
    }
}
