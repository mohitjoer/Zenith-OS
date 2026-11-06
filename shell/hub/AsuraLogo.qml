import QtQuick

Item {
    id: root
    property real size: 28
    property string variant: "white" // "white" (default) or "dark"
    implicitWidth: size
    implicitHeight: size

    Image {
        id: img
        anchors.fill: parent
        source: {
            if (root.variant === "dark") {
                return Qt.resolvedUrl("../assets/asura-logo.svg");
            }
            return Qt.resolvedUrl("../assets/asura-logo-on-dark.svg");
        }
        sourceSize.width: Math.round(root.size * 2)
        sourceSize.height: Math.round(root.size * 2)
        fillMode: Image.PreserveAspectFit
        smooth: true
        mipmap: true
    }
}
