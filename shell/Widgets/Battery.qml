pragma ComponentBehavior: Bound

// Battery, over UPower's D-Bus interface.
//
// A device that has no battery reports nothing rather than showing 100%. A
// desktop widget that claims a full charge on a desktop machine is a widget
// lying, and users read it.

import Quickshell
import Quickshell.Io
import QtQuick

// The shell's own module, for the singletons this file reads. A file in a
// subdirectory cannot see a type from the directory above it by proximity the
// way a file beside it can, so the module is imported by name.
import qs

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

    // The size, from the text it draws. A widget with no size of its own is a
    // widget the bar cannot measure, and a chip around a widget with no size
    // is a chip with nothing in it.
    implicitWidth: label.implicitWidth
    implicitHeight: label.implicitHeight

    Text {
        id: label
        anchors.fill: parent
        text: battery.charging ? "+" : ""
            + Math.round(battery.charge * 100) + "%"
        color: Theme.text
        font.pixelSize: 12
    }
}
