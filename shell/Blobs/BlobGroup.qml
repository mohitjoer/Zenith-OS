import QtQuick

// The group that owns one blob field.
//
// In the compiled module this coordinates a single mesh out of every member
// rect. The shim keeps the group as the holder of the field's color and the
// per-rect styling parameters, so a member reads the field it belongs to,
// rather than carrying duplicates of its values.
Item {
    id: group

    default property alias content: content.data

    property color color: "white"
    property real borderWidth: 0
    property color borderColor: "transparent"
    property real smoothing: 0
    property real shadowStrength: 0
    property real shadowSize: 0

    Item {
        id: content
        anchors.fill: parent
    }
}
