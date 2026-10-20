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
import QtQuick.Controls

Item {
    id: quick

    // The card. A popup with no background of its own is a white rectangle,
    // and the quick settings are the surface a user reaches for when something
    // is wrong, so it is the last surface that should look like a blank page.
    Rectangle {
        anchors.fill: parent
        radius: Theme.radiusLarge
        color: Theme.panelFill
        border.width: Theme.hairlineWidth
        border.color: Theme.hairline
    }

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
                checked: Config.dockAutohide
                onToggled: Config.setDockAutohide(checked)
            }

            Button {
                text: "lock"
                onClicked: lockScreen.request()
            }
        }

        Row {
            spacing: 12

            Button {
                text: Config.dark ? "light" : "dark"
                onClicked: Config.setColorScheme(Config.dark ? 2 : 1)
            }

            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: "Zenith"
                color: Theme.textDim
                font.pixelSize: 12
            }
        }
    }
}
