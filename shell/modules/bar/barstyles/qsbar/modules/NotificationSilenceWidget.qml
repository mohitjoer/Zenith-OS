import QtQuick
import Quickshell
import Quickshell.Io
import qs.Ui.Singletons

Item {
    id: rootMod
    required property var root
    readonly property color contentColor: root.widgetContentColor("G8", root.seal)

    readonly property bool silenced: root.notifSilenced

    visible: silenced
    implicitWidth: silenced ? 20 : 0
    implicitHeight: 28


    readonly property string tooltipText: I18n.tr("Notifications silenced")

    IconText {
        anchors.centerIn: parent
        text: "\uE7F6"   // notifications_off
        color: rootMod.contentColor
        font.pixelSize: 14
    }

    Process {
        id: toggleProc
        command: ["true"]
        onExited: root.refreshStatusIndicators()
    }

    TooltipMixin { id: tip; root: rootMod.root; owner: rootMod; text: rootMod.tooltipText }

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true; cursorShape: Qt.PointingHandCursor
        onEntered: tip.show()
        onExited:  { tip.hide() }
        onClicked: {
            tip.hide()
            toggleProc.running = false; toggleProc.running = true
        }
    }
}
