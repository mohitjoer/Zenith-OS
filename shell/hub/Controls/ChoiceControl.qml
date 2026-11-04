pragma ComponentBehavior: Bound

// An enum-typed setting.
//
// The choices come from the export, so the list is the schema's and not a copy
// of it. An enum whose values were added to the schema would otherwise not
// appear here until somebody remembered to add them, which is the same
// second-copy problem the window was built to avoid.
//
// The choices are shown as they are rather than behind a menu that opens on
// top of the page: a popup in a list view is a popup that is clipped by the
// list, and a control that is clipped is a control that cannot be used.
//
// The nick is written bare. The export quotes it, because that is how GVariant
// spells a string, and the tool compares what it is given against the
// schema's nicks, so a quoted nick is not a nick and the write is refused.

import Quickshell
import QtQuick

// The shell's own module, for the theme. A file in a subdirectory cannot see
// a type from the directory above it by proximity the way a file beside it
// can, so the module is imported by name.
import qs
import qs.hub

Editor {
    id: control

    // The choices, in the schema's order.
    readonly property var choices: control.key.choices || []

    // Whether the stored value is one of them.
    readonly property bool known: choices.length === 0
        || choices.indexOf(control.current) >= 0

    implicitWidth: 220

    Column {
        id: options
        width: parent.width
        spacing: Theme.space1
        height: implicitHeight

        Repeater {
            model: control.choices

            delegate: Rectangle {
                id: option

                required property string modelData
                required property int index

                width: options.width
                height: 28
                radius: Theme.radiusSmall
                color: "transparent"

                // The mark that says "this one".
                //
                // Filled when the setting is on it and an outline when it is
                // not. The mark is the state; the label only follows it.
                readonly property bool selected: control.current === modelData

                Rectangle {
                    id: marker
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    width: 12
                    height: 12
                    radius: width / 2
                    color: parent.selected ? Theme.inkPrimary : "transparent"
                    border.width: parent.selected ? 0 : Theme.hairlineWidth
                    border.color: Theme.inkTertiary
                }

                Text {
                    anchors.left: marker.right
                    anchors.leftMargin: Theme.space2
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: option.modelData
                    color: option.selected ? Theme.inkPrimary : Theme.inkSecondary
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.typeLabelSize
                    elide: Text.ElideRight
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

                // The focus ring.
                //
                // Outside the option rather than inside it, so that focusing
                // one does not change the size of the thing being focused.
                Rectangle {
                    anchors.fill: parent
                    anchors.margins: -2
                    radius: parent.radius + 2
                    color: "transparent"
                    border.width: 2
                    border.color: Theme.focusRing
                    visible: option.activeFocus
                }

                HoverHandler {
                    id: hover
                }

                TapHandler {
                    onTapped: control.commit(option.modelData)
                }

                Keys.onSpacePressed: (event) => {
                    control.commit(option.modelData)
                    event.accepted = true
                }
                Keys.onReturnPressed: (event) => {
                    control.commit(option.modelData)
                    event.accepted = true
                }

                focus: true
                activeFocusOnTab: true

                Accessible.role: Accessible.RadioButton
                Accessible.name: option.modelData
                Accessible.description: control.key.name
                Accessible.checkable: true
                Accessible.checked: option.selected
            }
        }

        // The stored value is not one of the choices.
        //
        // It is said rather than hidden. A control that shows the first
        // choice when the real value is not in the list is showing a value the
        // setting does not have, and there is no way to tell that apart from
        // a setting that genuinely is on its first choice.
        Text {
            width: options.width
            visible: !control.known
            text: "\"" + control.current
                + "\" is not one of this setting's choices."
            color: Theme.inkTertiary
            font.family: Theme.fontFamily
            font.pixelSize: Theme.typeCaptionSize
            wrapMode: Text.WordWrap
        }
    }
}
