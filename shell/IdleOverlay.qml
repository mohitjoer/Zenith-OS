pragma ComponentBehavior: Bound

// The dim scrim for one screen: black, over everything, ignoring input.
//
// Black on purpose and not a theme colour. A scrim darkens whatever is behind
// it, and one tinted by the palette would tint the wallpaper with it.

import Quickshell
import Quickshell.Wayland
import QtQuick

PanelWindow {
    id: scrim

    required property var modelData
    property bool dimmed: false

    screen: modelData
    // Mapped only while there is something to see, so an awake desktop carries
    // no extra surface.
    visible: fill.opacity > 0
    color: "transparent"
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "zenith-idle-scrim"
    WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
    anchors { top: true; bottom: true; left: true; right: true }

    // Input goes through: moving the mouse is how the user wakes the machine.
    mask: Region { item: null }

    Rectangle {
        id: fill
        anchors.fill: parent
        color: "#000000"
        opacity: scrim.dimmed ? 0.6 : 0
        // In slowly, out fast: waking should not feel like waiting.
        Behavior on opacity {
            NumberAnimation {
                duration: scrim.dimmed ? 400 : 120
                easing.type: Easing.OutQuad
            }
        }
    }
}
