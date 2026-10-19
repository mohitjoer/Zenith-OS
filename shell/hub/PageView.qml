pragma ComponentBehavior: Bound

// One page of the Hub.
//
// It receives a page object from the export and draws a row per key. It does not
// know what any key means, what it is called, or which page it is on — all three
// come in — so a setting added to the registry shows up here with the control its
// type maps to and nothing else in this file changes.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

Item {
    id: view

    required property var page

    readonly property var keys: page ? page.keys : []

    // A row is chosen from a name rather than built with a chain of conditionals,
    // so the mapping between a control type and a widget is one lookup and it is
    // the same lookup `zenith doctor` checks has a control for every type.
    function controlFor(key) {
        switch (key.control) {
        case "switch": return "Controls/SwitchControl.qml";
        case "spin": return "Controls/SpinControl.qml";
        case "choice": return "Controls/ChoiceControl.qml";
        case "list": return "Controls/ListControl.qml";
        default: return "Controls/TextControl.qml";
        }
    }

    Column {
        anchors.fill: parent
        anchors.margins: 28
        spacing: 20

        Text {
            text: view.page ? view.page.title : ""
            color: config.dark ? "#e8eaed" : "#202124"
            font.pixelSize: 22
            font.bold: true
        }

        Text {
            width: parent.width
            text: view.page ? view.page.summary : ""
            color: config.dark ? "#9aa0a6" : "#5f6368"
            font.pixelSize: 13
        }

        ListView {
            width: parent.width
            height: parent.height - 120
            spacing: 4
            model: view.keys

            delegate: Rectangle {
                required property var modelData

                width: ListView.view.width
                height: Math.max(64, row.implicitHeight + 16)
                radius: 8
                color: config.dark ? "#1b1e22" : "#f8f9fa"

                Row {
                    id: row
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.margins: 12
                    spacing: 16

                    Column {
                        width: (row.width - 220) / 2
                        spacing: 2

                        Text {
                            width: parent.width
                            text: parent.parent.modelData.name
                            color: config.dark ? "#e8eaed" : "#202124"
                            font.pixelSize: 13
                        }

                        // The schema's own summary, not a sentence written here.
                        // A description living next to its key is one that gets
                        // updated when the key changes.
                        Text {
                            width: parent.width
                            visible: text !== ""
                            text: parent.parent.modelData.summary
                            color: config.dark ? "#9aa0a6" : "#5f6368"
                            font.pixelSize: 11
                            wrapMode: Text.WordWrap
                        }
                    }

                    Loader {
                        width: 220
                        source: view.controlFor(row.parent.modelData)
                        onLoaded: if (item) item.key = row.parent.modelData
                    }
                }
            }
        }
    }
}
