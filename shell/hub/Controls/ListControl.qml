pragma ComponentBehavior: Bound

// A string-list setting.
//
// Edited as a list of rows rather than as one line of text, and that is the
// point rather than a preference. A GVariant tuple is a list of strings, and
// a line of text separated by commas cannot say where one string ends and the
// next begins: a name with a comma in it is two names, and a name with an
// apostrophe in it loses it. The tuple is parsed and written by the dialect
// in GVariant.qml, so an item that contains a comma or an apostrophe comes
// back as the item that went in.
//
// The value is written whole, because that is what it is: one value, and not
// a list of values the tool has to be told apart.

import Quickshell
import QtQuick

// The shell's own module, for the theme. A file in a subdirectory cannot see
// a type from the directory above it by proximity the way a file beside it
// can, so the module is imported by name.
import qs
import qs.hub

Editor {
    id: control

    // The items, as the value spells them.
    readonly property var items: GVariant.parseList(control.current)

    // The rows on screen.
    //
    // A list of rows rather than a list of strings, so that editing one does
    // not rebuild the others: a model rebuilt on every edit takes the focus
    // with it, and a list of paths edited one at a time becomes unusable.
    property var rows: []

    // The rows are brought into step with the value.
    //
    // Only when they differ, because replacing the rows rebuilds every row
    // and the row being edited is in the middle of it.
    function sync() {
        // The items and the rows are not there yet on the first call, which
        // happens before the properties have been given their values.
        const items = control.items || []

        const rows = control.rows || []
        if (rows.length === items.length) {
            let same = true
            for (let i = 0; i < items.length; i++) {
                if (rows[i].text !== items[i]) {
                    same = false
                    break
                }
            }
            if (same) return
        }

        const built = []
        for (let i = 0; i < items.length; i++) built.push({ text: items[i] })
        control.rows = built
    }

    onCurrentChanged: sync()
    Component.onCompleted: sync()

    // The list is written whole.
    //
    // A row that has never been filled in is not written, because an empty
    // item is a value nobody asked for and the shell reports names it does
    // not recognise.
    function commitRows() {
        const texts = []
        const rows = control.rows || []
        for (let i = 0; i < rows.length; i++) {
            const text = rows[i].text
            if (text !== "") texts.push(text)
        }
        control.commit(GVariant.formatList(texts))
    }

    // A row is added and the field in it takes the focus, so the item is
    // typed rather than clicked at.
    function addRow() {
        const rows = (control.rows || []).slice()
        rows.push({ text: "" })
        control.rows = rows
        commitRows()
        focusNew.start()
    }

    function removeRow(index) {
        const rows = (control.rows || []).slice()
        rows.splice(index, 1)
        control.rows = rows
        commitRows()
    }

    // The focus lands on the new row once it has been built.
    Timer {
        id: focusNew
        interval: 0
        onTriggered: {
            const rows = control.rows || []
            const row = rows[rows.length - 1]
            if (row && row.input) row.input.forceActiveFocus()
        }
    }

    implicitWidth: 220

    Column {
        id: options
        width: parent.width
        spacing: Theme.space1
        height: implicitHeight

        Repeater {
            model: control.rows

            delegate: Rectangle {
                id: row

                required property var modelData
                required property int index

                width: options.width
                height: 36
                radius: Theme.radiusSmall
                color: "transparent"

                // The item.
                Rectangle {
                    id: entry
                    anchors.left: parent.left
                    anchors.right: remove.left
                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    anchors.rightMargin: Theme.space1
                    radius: Theme.radiusField
                    color: Theme.surfaceSunken
                    border.width: entry.focused ? 2 : Theme.hairlineWidth
                    border.color: entry.focused ? Theme.focusRing : Theme.hairline

                    // Whether this row's field has the keyboard.
                    //
                    // Tracked here and not read off the input, because a child
                    // that has not been built yet is not a thing whose focus
                    // can be read.
                    property bool focused: false

                    TextInput {
                        id: input
                        anchors.fill: parent
                        anchors.leftMargin: Theme.space2
                        anchors.rightMargin: Theme.space2
                        verticalAlignment: TextInput.AlignVCenter
                        text: row.modelData.text
                        color: Theme.inkPrimary
                        font.family: Theme.fontFamily
                        font.pixelSize: Theme.typeBodySize
                        selectByMouse: true
                        selectionColor: Theme.indicatorMuted
                        selectedTextColor: Theme.canvas

                        // An item is a word and not the width of the field: an
                        // unclipped input draws over the button beside it.
                        clip: true

                        onAccepted: focus = false
                        onActiveFocusChanged: {
                            entry.focused = activeFocus
                            if (!activeFocus) {
                                row.modelData.text = input.text
                                control.commitRows()
                            }
                        }

                        Component.onCompleted: row.modelData.input = input

                        Accessible.role: Accessible.EditableText
                        Accessible.name: control.key.summary !== ""
                            ? control.key.summary : control.key.name
                        Accessible.description: control.key.name
                    }
                }

                // The button that takes the item out.
                Rectangle {
                    id: remove
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    width: 32
                    height: 32
                    radius: Theme.radiusSmall
                    color: "transparent"
                    border.width: Theme.hairlineWidth
                    border.color: Theme.hairline

                    Text {
                        anchors.centerIn: parent
                        text: "×"
                        color: Theme.inkSecondary
                        font.family: Theme.fontFamily
                        font.pixelSize: Theme.typeBodySize
                    }

                    Rectangle {
                        anchors.fill: parent
                        radius: parent.radius
                        color: Theme.stateLayer
                        opacity: hover.hovered ? 1 : 0
                    }

                    HoverHandler {
                        id: hover
                    }

                    TapHandler {
                        onTapped: control.removeRow(row.index)
                    }

                    Accessible.role: Accessible.Button
                    Accessible.name: "Remove"
                    Accessible.description: row.modelData.text
                }
            }
        }

        // The list is empty.
        Text {
            width: options.width
            visible: control.items.length === 0
            text: "No items."
            color: Theme.inkTertiary
            font.family: Theme.fontFamily
            font.pixelSize: Theme.typeCaptionSize
        }

        // The button that puts one in.
        Rectangle {
            id: add
            width: 72
            height: 28
            radius: Theme.radiusSmall
            color: "transparent"
            border.width: Theme.hairlineWidth
            border.color: Theme.hairline

            Text {
                anchors.centerIn: parent
                text: "Add"
                color: Theme.inkPrimary
                font.family: Theme.fontFamily
                font.pixelSize: Theme.typeLabelSize
            }

            Rectangle {
                anchors.fill: parent
                radius: parent.radius
                color: Theme.stateLayer
                opacity: hover.hovered ? 1 : 0
            }

            HoverHandler {
                id: hover
            }

            TapHandler {
                onTapped: control.addRow()
            }

            Accessible.role: Accessible.Button
            Accessible.name: "Add"
            Accessible.description: control.key.name
        }
    }
}
