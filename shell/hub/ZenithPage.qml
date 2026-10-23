pragma ComponentBehavior: Bound

// The Zenith group: the settings the GSettings schemas declare
// (org.zenith.shell, org.zenith.desktop), drawn from the page registry that
// `zenith config export` writes, and changed only through `zenith config set`.
//
// This is the Hub as it was before Ryoku's was ported in, kept as the one extra
// page because the rule in docs/architecture.md still holds: every setting that
// has a command also has a page. The pages are not listed here. A page added to
// the registry in Go (desktop/internal/hub) shows up below with nothing in this
// directory edited, which is the whole point of reading the registry rather than
// repeating it. The registry is read once, when the page is first opened.

import Quickshell.Io
import QtQuick
import QtQuick.Layouts
import qs.modules.common as Common
import qs.modules.common.widgets

Item {
    id: root

    // Set by the Hub when it loads the page.
    property var hub

    property var document: ({ pages: [] })
    property bool loaded: false
    property string loadError: ""
    readonly property var pages: root.document.pages

    // The registry is read when all three parts are in -- the exit code and both
    // streams -- and not when the process exits. They arrive separately and none
    // of them waits for another: handling the exit first reads an output that has
    // not arrived yet, and the page reports a parse failure for a document that
    // was on its way.
    property int registryCode: -1
    property bool registryOut: false
    property bool registryErr: false
    property string registryOutput: ""
    property string registryError: ""

    function finishRegistry() {
        if (root.loaded) return
        if (root.registryCode < 0 || !root.registryOut || !root.registryErr) return
        if (root.registryCode !== 0) {
            root.loadError = root.registryError.trim()
        } else {
            try {
                root.document = JSON.parse(root.registryOutput)
            } catch (error) {
                root.loadError = "" + error
            }
        }
        root.loaded = true
    }

    Process {
        id: registry
        command: ["zenith", "config", "export"]
        running: true

        stdout: StdioCollector {
            waitForEnd: true
            onStreamFinished: {
                root.registryOutput = text
                root.registryOut = true
                root.finishRegistry()
            }
        }
        stderr: StdioCollector {
            waitForEnd: true
            onStreamFinished: {
                root.registryError = text
                root.registryErr = true
                root.finishRegistry()
            }
        }

        onExited: (code) => {
            root.registryCode = code
            root.finishRegistry()
        }

        // A tool that cannot be started never exits, so the exit never comes.
        // That is a failure to read the settings, reported as one.
        onRunningChanged: if (!running && root.registryCode < 0) {
            root.registryError =
                "zenith could not be started, and the settings are read with "
                + "`zenith config export`."
            root.registryErr = true
            root.registryCode = 1
            root.finishRegistry()
        }
    }

    // All four states say something true: loading, failed, empty and normal.
    StyledText {
        anchors.centerIn: parent
        visible: !root.loaded
        text: "Loading settings…"
        color: Common.Appearance.colors.colSubtext
        font.pixelSize: Common.Appearance.font.pixelSize.normal
    }

    ColumnLayout {
        anchors.centerIn: parent
        width: Math.min(root.width - 120, 560)
        spacing: 8
        visible: root.loaded && root.loadError !== ""

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
            text: root.loadError
            color: Common.Appearance.colors.colSubtext
            font.pixelSize: Common.Appearance.font.pixelSize.small
        }
    }

    StyledText {
        anchors.centerIn: parent
        visible: root.loaded && root.loadError === "" && root.pages.length === 0
        text: "Zenith has no settings"
        color: Common.Appearance.colors.colSubtext
        font.pixelSize: Common.Appearance.font.pixelSize.normal
    }

    ContentPage {
        anchors.fill: parent
        visible: root.loaded && root.loadError === "" && root.pages.length > 0
        baseWidth: root.width - 48
        forceWidth: true

        Repeater {
            model: root.pages

            PageView {
                required property var modelData

                Layout.fillWidth: true
                page: modelData
            }
        }
    }
}
