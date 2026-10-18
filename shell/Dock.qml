pragma ComponentBehavior: Bound

// The dock.
//
// favourites are desktop file IDs, not paths, because that is what
// org.gnome.shell favorite-apps holds and adoption imports them across
// unchanged. A path here would have to be rewritten during adoption, and a
// rewritten desktop file ID is one nobody can compare against what GNOME had.

import Quickshell
import Quickshell.Widgets
import QtQuick

PanelWindow {
    id: dock

    readonly property int edge: config.dockPosition

    implicitWidth: edge === config.edgeLeft || edge === config.edgeRight
        ? implicitHeight
        : implicitHeight
    implicitHeight: {
        var count = Math.max(1, config.favorites.length)
        return count * 64 + 16
    }

    anchors {
        top: edge === config.edgeTop
        bottom: edge === config.edgeBottom
        left: edge === config.edgeLeft
        right: edge === config.edgeRight
    }

    color: "transparent"
    exclusionMode: ExclusionMode.Ignore
    exclusiveZone: config.dockAutohide ? 0 : implicitWidth

    // Autohide is a property of the window's geometry, not an animation played
    // over a fixed window: an animated hide leaves a strip the pointer has to
    // find, and the strip is the thing autohide is supposed to remove.
    visible: !config.dockAutohide || pointerInside

    property bool pointerInside: false

    HoverHandler {
        id: dockHover
        onHoveredChanged: dock.pointerInside = hovered
    }

    Flow {
        anchors.fill: parent
        spacing: 8

        Repeater {
            model: config.favorites

            delegate: IconImage {
                required property var modelData

                // The desktop file ID is resolved to an icon by name; the shell
                // never builds a path to an icon out of it.
                readonly property string iconName: modelData

                implicitSize: 48
                source: iconName
                colorize: true
            }
        }
    }
}
