pragma ComponentBehavior: Bound

// A boolean setting.
//
// A switch, drawn in ink: the track is an outline over a well when the setting
// is off and a filled bar when it is on, and the knob changes with it. There
// is no colour to say "on" with, so the shape says it — the same rule the
// whole desktop is drawn by.
//
// The value is a boolean in the export and a boolean in the tool, and both
// spell it bare. A switch that looked for "'true'" would show every boolean
// in the desktop as off.

import Quickshell
import QtQuick

// The shell's own module, for the theme. A file in a subdirectory cannot see
// a type from the directory above it by proximity the way a file beside it
// can, so the module is imported by name.
import qs
import qs.hub

Editor {
    id: control

    // The value as a boolean.
    readonly property bool on: control.current === "true"

    implicitWidth: 44

    Rectangle {
        id: track

        anchors.right: parent.right
        width: 44
        height: 26
        radius: Theme.radiusPill
        color: control.on ? Theme.inkPrimary : Theme.surfaceSunken
        border.width: Theme.hairlineWidth
        border.color: control.on ? Theme.inkPrimary : Theme.inkTertiary

        // The knob.
        //
        // Filled in both states, and the colour of the ink changes with the
        // track, because a knob that stayed the same colour would be a knob
        // that said nothing.
        Rectangle {
            id: knob

            width: 18
            height: 18
            radius: width / 2
            color: control.on ? Theme.inkInverse : Theme.inkSecondary
            x: control.on ? track.width - width - 4 : 4
            y: (track.height - height) / 2

            Behavior on x {
                NumberAnimation {
                    duration: Theme.reducedMotion ? 0 : Theme.motionSmall
                    easing.type: Easing.OutCubic
                }
            }
        }

        // The state layer.
        //
        // The ink of the state, over the track. On the filled track the ink
        // has to be the inverse one, because a light layer on a light track
        // is not a layer.
        Rectangle {
            anchors.fill: parent
            radius: parent.radius
            color: control.on
                ? Qt.rgba(Theme.inkInverse.r, Theme.inkInverse.g,
                    Theme.inkInverse.b, Theme.stateHoverAlpha)
                : Theme.stateLayer
            opacity: hover.hovered ? 1 : 0

            Behavior on opacity {
                NumberAnimation {
                    duration: Theme.reducedMotion ? 0 : Theme.motionMicro
                }
            }
        }

        // The focus ring.
        //
        // Outside the track rather than inside it, so that focusing the
        // switch does not change the size of the thing being focused.
        Rectangle {
            anchors.fill: parent
            anchors.margins: -3
            radius: parent.radius + 3
            color: "transparent"
            border.width: 2
            border.color: Theme.focusRing
            visible: track.activeFocus
        }

        HoverHandler {
            id: hover
        }

        TapHandler {
            onTapped: control.commit(control.on ? "false" : "true")
        }

        Keys.onSpacePressed: (event) => {
            control.commit(control.on ? "false" : "true")
            event.accepted = true
        }
        Keys.onReturnPressed: (event) => {
            control.commit(control.on ? "false" : "true")
            event.accepted = true
        }

        focus: true
        activeFocusOnTab: true

        Accessible.role: Accessible.Switch
        Accessible.name: control.key.summary !== ""
            ? control.key.summary : control.key.name
        Accessible.description: control.key.name
        Accessible.checkable: true
        Accessible.checked: control.on
    }
}
