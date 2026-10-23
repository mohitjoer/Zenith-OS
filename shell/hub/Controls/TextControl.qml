pragma ComponentBehavior: Bound

// A string setting.
//
// Most of the string settings in Zenith are file paths, so a bare text field
// is not the whole story: a path field that does not tell you the file exists
// is a field that produces a wallpaper which is a broken-image placeholder.
// The field says so when the file it names is not there.
//
// The value is a GVariant string, so it is quoted on the way in and quoted on
// the way out. A field that wrote what it was given would write a path the
// tool reads as a bare word, and GSettings would refuse it.

import Quickshell
import Quickshell.Io
import QtQuick

// The shell's own module, for the theme. A file in a subdirectory cannot see
// a type from the directory above it by proximity the way a file beside it
// can, so the module is imported by name.
import qs
import qs.hub

Editor {
    id: control

    // The value as text.
    readonly property string display: GVariant.unquote(control.current)

    // Whether the value names a file.
    //
    // A URI or an absolute path. Anything else is a string that happens to
    // be set, and there is nothing to look for.
    readonly property bool namesFile: /^(file:\/\/|\/)/.test(display)

    // The file it names, as the file system knows it.
    readonly property string local: {
        if (!display.startsWith("file://")) return display
        const path = display.substring(7)
        try {
            return decodeURIComponent(path)
        } catch (error) {
            return path
        }
    }

    implicitWidth: 220

    // The text follows the value, so a refused write puts the text back.
    //
    // The field is checked for, because the value is read before the field is
    // built: a handler that reached for a field that was not there yet would
    // throw on every editor.
    function syncText() {
        if (!field || !field.input) return
        field.input.text = control.display
        // A field that is not being edited shows its value from the start.
        // Assigning the text leaves the cursor at the end, and an input scrolls
        // to its cursor, so an unfocused path would show its last characters and
        // hide the part that says where the file is. A focused field keeps the
        // cursor where the reader put it.
        if (!field.input.activeFocus) field.input.cursorPosition = 0
    }
    onDisplayChanged: syncText()

    // The field is committed when it is left or when Enter is pressed.
    function commitText() {
        const text = field.input.text
        field.input.text = text
        control.commit(GVariant.quote(text))
    }

    // The file the value names, watched so that the note under the field is
    // true.
    FileView {
        id: probe
        printErrors: false
        path: control.namesFile ? control.local : ""
        onLoadFailed: control.missing = true
        onLoaded: control.missing = false
    }

    // Whether the file the value names is not there.
    property bool missing: false

    Rectangle {
        id: field
        // The editor reaches its input through the field (`field.input`).
        property alias input: input
        width: parent.width
        height: 36
        radius: Theme.radiusField
        color: Theme.surfaceSunken
        border.width: field.focused ? 2 : Theme.hairlineWidth
        border.color: field.focused ? Theme.focusRing : Theme.hairline

        // Whether the field has the keyboard.
        //
        // Tracked here and not read off the input, because a child that has
        // not been built yet is not a thing whose focus can be read.
        property bool focused: false

        TextInput {
            id: input
            anchors.fill: parent
            anchors.leftMargin: Theme.space2
            anchors.rightMargin: Theme.space2
            verticalAlignment: TextInput.AlignVCenter
            text: control.display
            color: Theme.inkPrimary
            font.family: Theme.fontFamily
            font.pixelSize: Theme.typeBodySize
            selectByMouse: true
            selectionColor: Theme.indicatorMuted
            selectedTextColor: Theme.canvas

            // A value is not the width of its field. A wallpaper is a path and a
            // path is long, and a `TextInput` neither elides nor clips on its
            // own: it draws past its own bounds, over the label it is beside and
            // out of the row. The text scrolls under the field's edge instead,
            // which is what a field is for.
            clip: true

            onActiveFocusChanged: {
                field.focused = activeFocus
                if (!activeFocus) control.commitText()
            }
            onAccepted: {
                control.commitText()
                focus = false
            }

            Accessible.role: Accessible.EditableText
            Accessible.name: control.key.summary !== ""
                ? control.key.summary : control.key.name
            Accessible.description: control.key.name
        }
    }

    // The note under the field.
    //
    // It is said only when there is something to say. A path that is there
    // needs no note, and a note that is always there is a note nobody reads.
    Text {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: field.bottom
        anchors.topMargin: visible ? Theme.space1 : 0
        visible: control.namesFile && control.missing
        text: "No such file: " + control.local
        color: Theme.warningColour
        font.family: Theme.fontFamily
        font.pixelSize: Theme.typeCaptionSize
        wrapMode: Text.WordWrap
    }
}
