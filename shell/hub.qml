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
//
// The chrome is the vendored component kit's: the navigation rail is built from
// the kit's buttons, the content pane is the kit's page, and the settings are a
// kit group of rows. That is why this file sits at the shell root rather than in
// `hub/` — a Quickshell configuration's root is the directory of its entry
// file, and the kit's own files reach each other through `qs.modules.*`. A Hub
// whose root was `hub/` could not see the kit at all, and a second copy of the
// kit beside it would be a second thing to keep in step. Two entry points, one
// tree: `shell.qml` is the desktop, `hub.qml` is this window, and both are
// loaded with `quickshell -p` on the file.
//
// The window's own code — the pages, the controls, the writes — is in `hub/`.

import Quickshell
import Quickshell.Io
import QtQuick
import QtQuick.Layouts
import "hub"
import qs.modules.common as Common
import qs.modules.common.widgets

FloatingWindow {
    id: window

    implicitWidth: 1040
    implicitHeight: 700
    visible: true
    title: "Zenith Settings"

    // The desktop's own root, not a panel's: the Hub is a window like any
    // other, and it is drawn on the same canvas the rest of the desktop is.
    color: Common.Appearance.colors.colLayer0

    // The registry, as `zenith config export` writes it.
    //
    // Read once at startup rather than watched. The registry changes when the
    // package is upgraded, and a settings window that has to be told about it is
    // a settings window that can be running against a stale list.
    property var document: ({ pages: [] })

    property bool loaded: false
    property string loadError: ""

    // The registry as it came back, and how it ended.
    property string registryOutput: ""
    property string registryError: ""

    // Which page is open, as an index into the registry.
    //
    // An index rather than the page object, so there is one answer to the
    // question and it is the same one in the navigation and in the pane below:
    // both resolve it against `pages`. It is clamped where it is read rather
    // than where it is written, because a registry that shrank under a window
    // that was already open would otherwise leave the pane on a page that is
    // not there.
    property int currentPage: 0

    readonly property var pages: window.document.pages

    // The page the pane below draws.
    //
    // Named `openPage` because `page` is the name the pane takes it under, and a
    // property the pane declares is not one this file can redeclare for it.
    readonly property var openPage: window.loaded && window.pages.length > 0
        ? window.pages[Math.min(window.currentPage, window.pages.length - 1)]
        : null

    // The registry is read, and the result is taken apart.
    //
    // Read when all three parts are in — the exit code and both streams — and
    // not when the process exits. They arrive separately and none of them waits
    // for another: handling the exit first reads an output that has not arrived
    // yet, and the window reports a parse failure for a document that was on its
    // way. `waitForEnd` holds each collector's text until its stream has ended,
    // and the flags say when that has happened to both.
    function finishRegistry() {
        if (window.loaded) return
        if (window.registryCode < 0 || !window.registryOut
            || !window.registryErr) return
        window.readRegistry(window.registryCode)
    }

    function readRegistry(code) {
        if (code !== 0) {
            window.loadError = window.registryError.trim()
            window.loaded = true
            return
        }
        try {
            window.document = JSON.parse(window.registryOutput)
        } catch (error) {
            // A parse failure is shown rather than swallowed. An empty
            // window with no explanation looks like a Hub with no
            // settings, which is a different and much worse claim.
            window.loadError = "" + error
        }
        window.loaded = true
    }

    // The three parts of the read.
    property int registryCode: -1
    property bool registryOut: false
    property bool registryErr: false

    Process {
        id: registry
        command: ["zenith", "config", "export"]
        running: true

        stdout: StdioCollector {
            waitForEnd: true
            onStreamFinished: {
                window.registryOutput = text
                window.registryOut = true
                window.finishRegistry()
            }
        }
        stderr: StdioCollector {
            waitForEnd: true
            onStreamFinished: {
                window.registryError = text
                window.registryErr = true
                window.finishRegistry()
            }
        }

        onExited: (code) => {
            window.registryCode = code
            window.finishRegistry()
        }

        // A tool that cannot be started at all never exits, so the exit never
        // comes. That is a failure to read the settings, and it is reported as
        // one rather than as a window that loads for ever — with a reason, so
        // it does not read as a desktop that has no settings.
        onRunningChanged: if (!running && window.registryCode < 0) {
            window.registryError =
                "zenith could not be started, and the settings are read with "
                + "`zenith config export`."
            window.registryErr = true
            window.registryCode = 1
            window.finishRegistry()
        }
    }

    // Show something true in all four states: loading, failed, empty and normal.
    // A settings application whose failure state is a blank rectangle has
    // converted a bug into an impression.
    StyledText {
        anchors.centerIn: parent
        visible: !window.loaded
        text: "Loading settings…"
        color: Common.Appearance.colors.colSubtext
        font.pixelSize: Common.Appearance.font.pixelSize.normal
    }

    ColumnLayout {
        anchors.centerIn: parent
        width: Math.min(window.width - 120, 560)
        spacing: 8
        visible: window.loaded && window.loadError !== ""

        StyledText {
            Layout.fillWidth: true
            horizontalAlignment: Text.AlignHCenter
            text: "Zenith's settings could not be read"
            font.pixelSize: Common.Appearance.font.pixelSize.large
            font.weight: Font.Medium
        }

        StyledText {
            Layout.fillWidth: true
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.WordWrap
            text: window.loadError
            color: Common.Appearance.colors.colSubtext
            font.pixelSize: Common.Appearance.font.pixelSize.small
        }
    }

    StyledText {
        anchors.centerIn: parent
        visible: window.loaded && window.loadError === ""
            && window.pages.length === 0
        text: "Zenith has no settings"
        color: Common.Appearance.colors.colSubtext
        font.pixelSize: Common.Appearance.font.pixelSize.normal
    }

    RowLayout {
        anchors.fill: parent
        spacing: 0
        visible: window.loaded && window.loadError === ""
            && window.pages.length > 0

        // The navigation rail.
        //
        // One step up from the canvas, so the rail reads as a column of the
        // window rather than as a stripe painted on it, and the page it opens
        // reads as the sheet on top. Depth is the only thing separating them,
        // because there is no colour to separate them with.
        Rectangle {
            Layout.preferredWidth: 236
            Layout.fillHeight: true
            color: Common.Appearance.colors.colLayer1

            ColumnLayout {
                anchors.fill: parent
                anchors.margins: 16
                spacing: 16

                // The mark, at the top of the navigation. Loaded from the
                // packaged SVG rather than embedded here: a logo pasted into
                // QML is a logo that stops matching the file anyone edits, and
                // the shell check cannot compare two copies it cannot find.
                Image {
                    id: mark
                    sourceSize.width: 88
                    asynchronous: true
                    Layout.leftMargin: 6

                    // The logo ships in both variants, and which one is correct
                    // depends on the panel the mark is drawn on rather than on
                    // the desktop's colour scheme: a logo in the wrong variant
                    // is a black blob on a dark panel. The canvas is what the
                    // mark ends up against, so the canvas is what decides.
                    source: (Common.Appearance.colors.colLayer1.r * 0.299
                            + Common.Appearance.colors.colLayer1.g * 0.587
                            + Common.Appearance.colors.colLayer1.b * 0.114) < 0.5
                        ? "/usr/share/zenith/branding/zenith-logo-on-dark.svg"
                        : "/usr/share/zenith/branding/zenith-logo.svg"
                }

                // The pages. Built from the document, so a page added to the
                // registry appears here without this file changing.
                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 4

                    Repeater {
                        model: window.pages

                        delegate: RippleButton {
                            id: entry

                            required property int index
                            required property var modelData

                            // The open page is a filled pill and the others are
                            // not. Fill and weight, because there is no colour
                            // to say "this one" with.
                            toggled: entry.index === window.currentPage
                            buttonRadius: Common.Appearance.rounding.full
                            buttonText: entry.modelData.title
                            implicitHeight: 40

                            // The kit's own navigation rail paints the open
                            // entry in the container colour rather than in the
                            // indicator ink, which is what keeps a selected row
                            // quiet enough to sit in a column of them.
                            colBackgroundToggled:
                                Common.Appearance.colors.colSecondaryContainer
                            colBackgroundToggledHover:
                                Common.Appearance.colors.colSecondaryContainerHover
                            colRippleToggled:
                                Common.Appearance.colors.colSecondaryContainerActive

                            Layout.fillWidth: true
                            onClicked: window.currentPage = entry.index

                            contentItem: StyledText {
                                text: entry.modelData.title
                                leftPadding: 16
                                verticalAlignment: Text.AlignVCenter
                                elide: Text.ElideRight
                                font.pixelSize: Common.Appearance.font.pixelSize.small
                                font.weight: entry.toggled ? Font.Medium : Font.Normal
                                color: entry.toggled
                                    ? Common.Appearance.colors.colOnSecondaryContainer
                                    : Common.Appearance.colors.colOnLayer1
                            }

                            Accessible.role: Accessible.Link
                            Accessible.name: entry.modelData.title
                            Accessible.description: entry.modelData.summary
                        }
                    }
                }

                // Nothing under the pages. A page's summary belongs with the
                // page, where the rows it describes are, and repeated in the
                // rail it is one sentence in two places — the rail's job is to
                // be scanned rather than read.
                //
                // The empty item is what holds the pages up against the mark.
                // A layout with nothing in it that grows gives its spare height
                // to what is left, and the navigation would drift to the middle
                // of the window the first time a page was added.
                Item {
                    Layout.fillHeight: true
                }
            }
        }

        // The page.
        //
        // Wider than the pane by nothing, and narrower by a gutter: the page
        // draws its own margins, so the width it is given is the width its rows
        // have, and a row that ran edge to edge against the rail would read as a
        // strip of the rail rather than as a row of the page.
        ContentPage {
            Layout.fillWidth: true
            Layout.fillHeight: true
            baseWidth: window.width - 236 - 48
            forceWidth: true

            PageView {
                Layout.fillWidth: true
                page: window.openPage
            }
        }
    }
}
