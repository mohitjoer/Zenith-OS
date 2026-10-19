pragma ComponentBehavior: Bound

// Network state.
//
// The state, not the interface name. A widget that shows "wlan0" is showing a
// name that changes with hardware; a widget that shows "connected" is showing
// the thing the user cares about.

import Quickshell
import Quickshell.Io
import QtQuick

Item {
    id: network

    implicitWidth: label.implicitWidth
    implicitHeight: label.implicitHeight

    property bool connected: NetworkManager.status === 2

    Text {
        id: label
        anchors.fill: parent
        text: network.connected ? "on" : "off"
        color: Palette.text
        font.pixelSize: 12
    }
}
