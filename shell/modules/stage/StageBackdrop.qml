pragma ComponentBehavior: Bound
import QtQuick

// Retired Parallax backdrop stub.
Item {
    id: root
    property var screen
    property string wallpaperPath: ""
    property string wallpaperFit: "Cover"
    anchors.fill: parent
    visible: false
}
