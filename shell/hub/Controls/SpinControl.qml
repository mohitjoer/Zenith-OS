pragma ComponentBehavior: Bound

// An integer setting.
//
// The bounds come from the export, so a key's declared range and the bounds
// this control offers are the same numbers rather than two that can disagree.
// A key that declares no range is given none: the buttons step, the field
// takes what is typed, and the tool decides whether the number is one the
// schema accepts. A control that invented a range would be refusing values
// the schema allows.
//
// The value is written on release rather than on every keystroke. A field that
// writes on each keystroke commits a value per character, and a user who
// types 300 and then backspaces commits 3, 30 and then 3.

import Quickshell
import QtQuick

// The shell's own module, for the theme. A file in a subdirectory cannot see
// a type from the directory above it by proximity the way a file beside it
// can, so the module is imported by name.
import qs
import qs.hub

Editor {
    id: control

    // The value as a number.
    readonly property int value: GVariant.toNumber(control.current, 0)

    // Whether the schema declares bounds for this key.
    readonly property bool bounded: control.key.hasRange === true
    readonly property int minimum: control.bounded ? control.key.minimum : 0
    readonly property int maximum: control.bounded ? control.key.maximum : 0

    implicitWidth: 220

    // The number on screen follows the value, so a refused write puts the
    // number back.
    //
    // The field is checked for, because the value is read before the field is
    // built: a handler that reached for a field that was not there yet would
    // throw on every editor.
    function syncText() {
        if (!field || !field.input) return
        field.input.text = String(control.value)
    }
    onValueChanged: syncText()

    // The field is committed when it is left or when Enter is pressed.
    //
    // A field that is not a number is written as it stands and the tool
    // refuses it, because the tool's message is about the key and this
    // control's would be about the field.
    function commitText() {
        const text = field.input.text.trim()
        if (text === "") {
            field.input.text = String(control.value)
            return
        }
        const number = GVariant.toNumber(text, NaN)
        control.commit(isNaN(number) ? text : String(number))
    }

    // The buttons step.
    //
    // They stop at the declared bounds when there are any, because a button
    // that knows the range cannot produce a value the schema refuses. The
    // field does not, because a field that silently changed what was typed
    // into it is a field that lies about what the user asked for.
    function step(direction) {
        let next = control.value + direction
        if (control.bounded) {
            next = Math.max(control.minimum, Math.min(control.maximum, next))
        }
        control.commit(String(next))
    }

    Row {
        id: row
        spacing: Theme.space1
        height: implicitHeight

        Repeater {
            model: [-1, 1]

            delegate: Rectangle {
                id: step

                required property int modelData

                width: 32
                height: 36
                radius: Theme.radiusSmall
                color: "transparent"
                border.width: Theme.hairlineWidth
                border.color: Theme.hairline

                Text {
                    anchors.centerIn: parent
                    text: step.modelData < 0 ? "−" : "+"
                    color: Theme.inkPrimary
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.typeBodySize
                }

                Rectangle {
                    anchors.fill: parent
                    radius: parent.radius
                    color: Theme.stateLayer
                    opacity: hover.hovered ? 1 : 0

                    Behavior on opacity {
                        NumberAnimation {
                            duration: Theme.reducedMotion ? 0 : Theme.motionMicro
                        }
                    }
                }

                HoverHandler {
                    id: hover
                }

                TapHandler {
                    onTapped: control.step(step.modelData)
                }

                Accessible.role: Accessible.Button
                Accessible.name: step.modelData < 0 ? "Decrease" : "Increase"
                Accessible.description: control.key.name
            }
        }

        // The field.
        //
        // A well rather than a raised surface: the number goes into the
        // surface, and a surface that holds something is not floating above
        // the one it holds.
        Rectangle {
            id: field
            width: 88
            height: 36
            radius: Theme.radiusField
            color: Theme.surfaceSunken
            border.width: field.focused ? 2 : Theme.hairlineWidth
            border.color: field.focused ? Theme.focusRing : Theme.hairline

            // Whether the field has the keyboard.
            //
            // Tracked here and not read off the input, because a child that
            // has not been built yet is not a thing whose focus can be read.
            property bool focused: false

            TextInput {
                id: input
                anchors.fill: parent
                anchors.leftMargin: Theme.space2
                anchors.rightMargin: Theme.space2
                verticalAlignment: TextInput.AlignVCenter
                text: String(control.value)
                color: Theme.inkPrimary
                font.family: Theme.fontFamily
                font.pixelSize: Theme.typeBodySize
                selectByMouse: true
                selectionColor: Theme.indicatorMuted
                selectedTextColor: Theme.canvas

                onActiveFocusChanged: {
                    field.focused = activeFocus
                    if (!activeFocus) control.commitText()
                }
                onAccepted: {
                    control.commitText()
                    focus = false
                }

                Accessible.role: Accessible.SpinBox
                Accessible.name: control.key.summary !== ""
                    ? control.key.summary : control.key.name
                Accessible.description: control.key.name
            }
        }

        // The bounds, when the schema declares them.
        Text {
            anchors.verticalCenter: parent.verticalCenter
            visible: control.bounded
            text: control.minimum + "–" + control.maximum
            color: Theme.inkTertiary
            font.family: Theme.fontFamily
            font.pixelSize: Theme.typeCaptionSize
        }
    }
}
