pragma ComponentBehavior: Bound

// Battery, over UPower's D-Bus interface.
//
// A device that has no battery reports nothing rather than showing 100%. A
// desktop widget that claims a full charge on a desktop machine is a widget
// lying, and users read it.

import Quickshell
import Quickshell.Io
import QtQuick

Item {
    id: battery

    // Devices can arrive late and go away when they are unplugged, so both are
    // tracked rather than indexed. Indexing a device list that can change length
    // is how a widget ends up displaying another machine's battery.
    property var devices: []

    property bool present: devices.length > 0
    property real charge: present ? devices[0].charge : 0
    property bool charging: present && devices[0].state === "charging"

    visible: present

    Text {
        anchors.fill: parent
        text: battery.charging ? "+" : ""
            + Math.round(battery.charge * 100) + "%"
        color: config.dark ? "#e6e6e6" : "#202020"
        font.pixelSize: 12
    }
}
