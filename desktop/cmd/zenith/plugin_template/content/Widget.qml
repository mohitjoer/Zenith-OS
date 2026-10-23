import QtQuick
import qs.PluginKit.Singletons

// content/Widget.qml is the one view the host mounts (on the bar, this is the
// glyph). It reads live state from the service (pluginApi.mainInstance) and its
// only click action toggles the plugin's panel: a widget click NEVER mutates
// anything. The host sets pluginApi, density, s, widthBudget and active; read
// them, never assign.
Item {
    id: root

    property var pluginApi
    property var screen
    property bool active: false
    property string density: "glyph"
    property real s: 1
    property real widthBudget: 0

    readonly property var service: pluginApi ? pluginApi.mainInstance : null
    readonly property int count: service ? service.count : 0

    // A setting is read behind its default: a plugin enabled before the setting
    // existed has no value for it, and undefined must not decide the layout.
    readonly property bool showCount: {
        const settings = pluginApi ? pluginApi.pluginSettings : null;
        return settings && settings.showCount !== undefined ? settings.showCount : true;
    }

    implicitWidth: row.implicitWidth
    implicitHeight: Math.max(row.implicitHeight, 18 * root.s)

    Row {
        id: row
        anchors.verticalCenter: parent.verticalCenter
        spacing: 6 * root.s

        // The plugin's mark, a diamond drawn from a square so it needs no font.
        // Ink, not colour: the active state is brighter, the idle state dimmer.
        Item {
            anchors.verticalCenter: parent.verticalCenter
            width: 12 * root.s
            height: 12 * root.s

            Rectangle {
                anchors.centerIn: parent
                width: 7 * root.s
                height: 7 * root.s
                rotation: 45
                color: root.active ? Theme.accent : Theme.dim
            }
        }

        // The label beside the mark, elided to the width the host allows.
        Text {
            visible: root.showCount
            anchors.verticalCenter: parent.verticalCenter
            text: root.count
            color: Theme.bright
            font.family: Theme.font
            font.pixelSize: 13 * root.s
            elide: Text.ElideRight
            width: root.widthBudget > 0 ? Math.min(implicitWidth, root.widthBudget) : implicitWidth
        }
    }

    MouseArea {
        anchors.fill: parent
        onClicked: if (root.pluginApi) root.pluginApi.togglePanel()
    }
}
