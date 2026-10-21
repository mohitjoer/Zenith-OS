pragma ComponentBehavior: Bound

// One page of the Hub.
//
// It receives a page object from the export and draws a row per key. It does not
// know what any key means, what it is called, or which page it is on — all three
// come in — so a setting added to the registry shows up here with the control its
// type maps to and nothing else in this file changes.
//
// The rows are the vendored kit's: the group is one `StyledRectangle` on the
// raised layer, and each row is a label and a control separated from the next by
// a hairline. That is the shape the kit's own settings pages have, and the shape
// is the whole of what "looks like the rest of the desktop" means for a list of
// settings — the alternative is a card per key, which turns a page of settings
// into a page of boxes and makes the eye re-find the left edge on every row.

import QtQuick
import QtQuick.Layouts
import qs.modules.common as Common
import qs.modules.common.widgets

ColumnLayout {
    id: view

    required property var page

    readonly property var keys: view.page ? view.page.keys : []

    // Wide enough for the longest control in the kit and stable between rows, so
    // the controls line up in a column of their own rather than stepping in and
    // out with the width of each label.
    readonly property int controlWidth: 240

    // A row is chosen from a name rather than built with a chain of conditionals,
    // so the mapping between a control type and a widget is one lookup and it is
    // the same lookup `zenith doctor` checks has a control for every type.
    function editorFor(key) {
        switch (key.control) {
        case "switch": return "Controls/SwitchControl.qml";
        case "spin": return "Controls/SpinControl.qml";
        case "choice": return "Controls/ChoiceControl.qml";
        case "list": return "Controls/ListControl.qml";
        default: return "Controls/TextControl.qml";
        }
    }

    Layout.fillWidth: true
    spacing: 20

    // The page's own heading.
    //
    // The rail names the page and this says what is on it. Both, because a rail
    // entry that is one word is a promise about a page the reader has not seen
    // yet, and the sentence underneath is what makes the promise checkable.
    ColumnLayout {
        Layout.fillWidth: true
        spacing: 4

        StyledText {
            Layout.fillWidth: true
            text: view.page ? view.page.title : ""
            color: Common.Appearance.colors.colOnLayer0
            font.pixelSize: Common.Appearance.font.pixelSize.huge
            font.weight: Font.Medium
        }

        StyledText {
            Layout.fillWidth: true
            visible: text !== ""
            wrapMode: Text.WordWrap
            text: view.page ? view.page.summary : ""
            color: Common.Appearance.colors.colSubtext
            font.pixelSize: Common.Appearance.font.pixelSize.small
        }
    }

    // The settings, as one group.
    //
    // One group and not one per key, because the rows are all the same kind of
    // thing: a name, a key and a control. A box around each would say each was
    // its own subject, and the hairlines already say where one ends.
    StyledRectangle {
        Layout.fillWidth: true
        contentLayer: StyledRectangle.ContentLayer.Group
        radius: Common.Appearance.rounding.large
        implicitHeight: rows.implicitHeight + rows.anchors.margins * 2

        ColumnLayout {
            id: rows

            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.margins: 8
            spacing: 0

            Repeater {
                model: view.keys

                delegate: ColumnLayout {
                    id: row

                    required property var modelData
                    required property int index

                    Layout.fillWidth: true
                    spacing: 0

                    // The rule between two rows, inset so it reads as a
                    // separator inside the group rather than as an edge of it.
                    Rectangle {
                        Layout.fillWidth: true
                        Layout.leftMargin: 12
                        Layout.rightMargin: 12
                        implicitHeight: 1
                        color: Common.Appearance.colors.colOutlineVariant
                        visible: row.index > 0
                    }

                    RowLayout {
                        Layout.fillWidth: true
                        Layout.topMargin: 12
                        Layout.bottomMargin: 12
                        Layout.leftMargin: 12
                        Layout.rightMargin: 12
                        spacing: 16

                        // What the setting is, and the key it is stored under.
                        //
                        // The summary is the label and the key is the identity,
                        // and the key is what `zenith config set` takes, so it
                        // is shown rather than kept to itself: a reader who has
                        // to guess the name of a setting cannot be told to set
                        // it from a terminal.
                        //
                        // The minimums are zero rather than the texts' own
                        // widths, because a layout will not shrink an item below
                        // its implicit size unless it is told to. Left at its
                        // natural width the label pushes the row wider than the
                        // page, and the control and the right half of every value
                        // go off the window's edge — which is what the label with
                        // the fewest words does.
                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 2

                            StyledText {
                                Layout.fillWidth: true
                                Layout.minimumWidth: 0
                                wrapMode: Text.WordWrap
                                text: row.modelData.summary !== ""
                                    ? row.modelData.summary
                                    : row.modelData.name
                                color: Common.Appearance.colors.colOnLayer2
                                font.pixelSize: Common.Appearance.font.pixelSize.small
                            }

                            StyledText {
                                Layout.fillWidth: true
                                Layout.minimumWidth: 0
                                elide: Text.ElideRight
                                text: row.modelData.name
                                color: Common.Appearance.colors.colSubtext
                                font.family: Common.Appearance.font.family.monospace
                                font.pixelSize: Common.Appearance.font.pixelSize.smaller
                            }
                        }

                        // The control.
                        //
                        // One width, and it is fixed rather than preferred: the
                        // rows are read by scanning down the right edge, and a
                        // control that took part of the slack would sit a little
                        // further from the edge on every row with a shorter
                        // label than the last.
                        //
                        // The key is given to the editor as it is loaded,
                        // because a required property has to be initialised
                        // while the object that declares it is being created.
                        // Setting it after the load is too late: the editor has
                        // already failed to be created by then, and the loader
                        // ends in Loader.Error with nothing in it.
                        Loader {
                            id: control

                            Layout.minimumWidth: view.controlWidth
                            Layout.preferredWidth: view.controlWidth
                            Layout.maximumWidth: view.controlWidth
                            Layout.alignment: Qt.AlignVCenter
                            Layout.preferredHeight: item ? item.implicitHeight : 0

                            Component.onCompleted: control.setSource(
                                view.editorFor(row.modelData),
                                { key: row.modelData })
                        }
                    }
                }
            }
        }
    }
}
