pragma ComponentBehavior: Bound

// The network.
//
// The connectivity the shell reports is the connectivity check rather than
// the state of a link: a machine attached to a network that cannot reach
// anything is not online, and an icon that says it is would be lying.

import Quickshell
import Quickshell.Io
import Quickshell.Networking
import QtQuick

// The shell's own module, for the singletons this file reads. A file in a
// subdirectory cannot see a type from the directory above it by proximity the
// way a file beside it can, so the module is imported by name.
import qs

Item {
    id: network

    implicitWidth: label.implicitWidth
    implicitHeight: label.implicitHeight

    // Connected means the internet is reachable. A captive portal is a
    // network the machine is on and cannot use, and the two are different
    // sentences: one is "on", the other is "on a network that wants a
    // password".
    property bool connected: Networking.connectivity === NetworkConnectivity.Full
        || Networking.connectivity === NetworkConnectivity.Limited

    property bool captive: Networking.connectivity === NetworkConnectivity.Portal

    Text {
        id: label
        anchors.fill: parent
        text: network.captive ? "portal" : (network.connected ? "on" : "off")
        color: Theme.inkPrimary
        font.pixelSize: 12
    }
}
