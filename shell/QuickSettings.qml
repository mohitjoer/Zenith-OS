pragma ComponentBehavior: Bound

// Quick settings.
//
// Every control here writes a key that is already declared in a schema, which is
// the rule that keeps the shell and the Hub from becoming two settings
// applications: a toggle in here and a switch in the Hub are the same key, so
// they cannot disagree.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

Item {
    id: quick

    implicitWidth: 360
    implicitHeight: column.implicitHeight + 32

    Column {
        id: column
        anchors { left: parent.left; right: parent.right; top: parent.top }
        anchors.margins: 16
        spacing: 12

        Row {
            spacing: 12

            // A switch bound to a key is a switch that is correct at load and
            // correct after a Hub change, because both of them wrote the same
            // value to the same place.
            Switch {
                text: "dock auto-hide"
                checked: config.dockAutohide
                onToggled: config.setDockAutohide(checked)
            }

            Button {
                text: "lock"
                onClicked: lockScreen.request()
            }
        }

        Row {
            spacing: 12

            Button {
                text: config.dark ? "light" : "dark"
                onClicked: config.setColorScheme(config.dark ? 2 : 1)
            }

            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: "Zenith"
                color: Palette.textDim
                font.pixelSize: 12
            }
        }
    }
}
