pragma ComponentBehavior: Bound

// The Hub's window.
//
// It draws pages it did not write. The navigation, the settings on each page,
// their types, their permitted values and their current values all come from
// `zenith config export`, which reads the same page registry that `zenith doctor`
// checks against the schemas.
//
// That is the whole design. A Hub whose QML lists its own pages is a second copy
// of the registry, and a second copy agrees with the first until somebody edits
// one of them — after which a page shows settings it does not own, or a setting
// has no page at all, which is the exact fault the invariant reports. Adding a
// setting here means adding a key to a schema and putting it on a page in Go.
// Nothing here has to be touched.
//
// The window is also not a writer. It calls `zenith config set`, so dconf is
// only ever written through the control tool, which is what the architecture
// record requires: hand-editing dconf causes drift, and drift is reported.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

FloatingWindow {
    id: window

    implicitWidth: 880
    implicitHeight: 600
    visible: true

    color: Theme.background

    // The registry, as `zenith config export` writes it.
    //
    // Read once at startup rather than watched. The registry changes when the
    // package is upgraded, and a settings window that has to be told about it is
    // a settings window that can be running against a stale list.
    property var document: ({ pages: [] })

    property bool loaded: false
    property string loadError: ""

    Process {
        id: export
        command: ["zenith", "config", "export"]
        running: true

        stdout: StdioCollector {
            onStreamFinished: {
                if (export.exitedWithCode !== 0) {
                    window.loadError = text
                    window.loaded = true
                    return
                }
                try {
                    window.document = JSON.parse(text)
                } catch (error) {
                    // A parse failure is shown rather than swallowed. An empty
                    // window with no explanation looks like a Hub with no
                    // settings, which is a different and much worse claim.
                    window.loadError = "" + error
                }
                window.loaded = true
            }
        }
    }

    // Show something true in all four states: loading, failed, empty and normal.
    // A settings application whose failure state is a blank rectangle has
    // converted a bug into an impression.
    Item {
        anchors.fill: parent

        Text {
            anchors.centerIn: parent
            visible: !window.loaded
            text: "Loading settings…"
            color: Theme.textDim
        }

        Column {
            anchors.centerIn: parent
            spacing: 8
            visible: window.loaded && window.loadError !== ""

            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: "Zenith's settings could not be read"
                color: Theme.accentForeground
                font.pixelSize: 15
                font.bold: true
            }

            Text {
                width: Math.min(window.width - 80, 640)
                horizontalAlignment: Text.AlignHCenter
                wrapMode: Text.WordWrap
                text: window.loadError
                color: Theme.textDim
                font.pixelSize: 12
            }
        }

        Text {
            anchors.centerIn: parent
            visible: window.loaded && window.loadError === ""
                && window.document.pages.length === 0
            text: "Zenith has no settings"
            color: Theme.textDim
        }

        Row {
            anchors.fill: parent
            spacing: 0
            visible: window.loaded && window.loadError === ""
                && window.document.pages.length > 0

            // Navigation. Built from the document, so a page added to the
            // registry appears here without this file changing.
            Rectangle {
                width: 220
                height: parent.height
                color: Theme.surface

                // The mark, at the top of the navigation. Loaded from the
                // packaged SVG rather than embedded here: a logo pasted into
                // QML is a logo that stops matching the file anyone edits, and
                // the shell check cannot compare two copies it cannot find.
                Image {
                    id: mark
                    source: "/usr/share/zenith/branding/zenith-logo.svg"
                    sourceSize.width: 96
                    asynchronous: true
                    anchors.horizontalCenter: parent.horizontalCenter
                    anchors.top: parent.top
                    anchors.topMargin: 16

                    // The logo ships in both variants, and which one is correct
                    // depends on the shell's colour scheme. Binding the source
                    // rather than tinting is what keeps the black-on-white file
                    // from being drawn as a black blob on a dark panel.
                    source: Config.dark
                        ? "/usr/share/zenith/branding/zenith-logo-on-dark.svg"
                        : "/usr/share/zenith/branding/zenith-logo.svg"
                }

                ListView {
                    anchors.fill: parent
                    anchors.topMargin: mark.y + mark.height + 16
                    spacing: 2
                    model: window.document.pages

                    delegate: Rectangle {
                        required property int index
                        required property var modelData

                        width: ListView.view.width
                        height: 40
                        radius: 6
                        readonly property bool current: index === list.currentIndex

                        color: current
                            ? (Theme.border)
                            : "transparent"

                        Text {
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.leftMargin: 12
                            text: parent.modelData.title
                            color: Theme.text
                            font.pixelSize: 13
                        }

                        TapHandler {
                            onTapped: list.currentIndex = parent.index
                        }
                    }
                }

                ListView { id: list; visible: false }
            }

            PageView {
                id: pageView
                width: parent.width - 220
                height: parent.height

                readonly property var page: window.loaded
                    && window.document.pages.length > 0
                    ? window.document.pages[Math.min(list.currentIndex,
                        window.document.pages.length - 1)]
                    : null

                page: pageView.page
            }
        }
    }
}
