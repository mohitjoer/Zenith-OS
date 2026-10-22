pragma ComponentBehavior: Bound

// The sidebar.
//
// A slide-in panel for audio, network, bluetooth, display and media. It is a
// renderer and nothing else: it runs `zenith state` and draws the JSON it gets
// back. Every decision about what a tool's output means is in Go, where there
// are tests for it — wpctl reporting a volume of 1.53, nmcli escaping the
// colons in a MAC address, bluetoothctl exiting 0 while printing an error. Those
// are five parsers that would have no tests here and five places to be wrong.
//
// The Hub window sets the precedent and this follows it exactly: QML reads
// through a Go command and never interprets a raw tool's output itself.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick
// The vendored component kit. The sidebar's surface, its text and its layers
// are the kit's, so a change to how a surface step looks reaches this panel
// without this panel being edited.
import qs.modules.common as Kit
import qs.modules.common.widgets

PanelWindow {
    id: sidebar

    // Four small pieces, inline rather than in four files. Each is a few lines
    // and each is used by more than one section; promoting one to a file is a
    // small change later, not a refactor now.
    //
    // They are members of this object because `component` is a member keyword:
    // an inline component declared at file scope is a syntax error, which is
    // what this was before the shell had ever been run by an engine.
    component SectionTitle: StyledText {
        color: Kit.Appearance.colors.colOnSurfaceVariant
        font.pixelSize: Theme.typeCaptionSize
        font.capitalization: Font.AllUppercase
        font.letterSpacing: 1.2
    }

    component Dim: StyledText {
        color: Kit.Appearance.colors.colOnSurfaceVariant
        font.pixelSize: 12
        width: parent ? parent.width : 0
        elide: Text.ElideRight
    }

    component Glyph: StyledText {
        font.family: Icons.iconFamily
        font.pixelSize: 13
        color: Kit.Appearance.m3colors.m3onBackground
    }

    // A hidden section collapses rather than showing an empty header. A sidebar
    // that says "no audio" on a machine that has audio, because one probe timed
    // out, is a sidebar people stop trusting.
    component Section: Column {
        property string title: ""
        property bool shown: true

        spacing: 6
        width: parent ? parent.width : 0
        leftPadding: 16
        rightPadding: 16
        visible: shown
        height: shown ? implicitHeight : 0

        SectionTitle { text: parent.title }
    }

    // An icon and a value, which is what two of the sections are made of.
    component Stat: Row {
        property string icon: ""
        property string value: ""

        spacing: 8

        Glyph { text: parent.icon }
        StyledText {
            anchors.verticalCenter: parent.verticalCenter
            text: parent.value
            color: Kit.Appearance.m3colors.m3onBackground
            font.pixelSize: 13
        }
    }

    // The panel is always this wide. One that resized to its contents would
    // reflow the desktop every time a bluetooth device appeared, which is a
    // desktop that moves under the pointer while you are clicking something.
    implicitWidth: 320
    implicitHeight: screen.height

    anchors { top: true; right: true }

    color: Theme.canvas
    exclusionMode: ExclusionMode.Ignore

    // Polled, not watched, because there is nothing to watch: none of these five
    // report state through anything this shell owns. The timer runs only while
    // the panel is open, so a sidebar nobody is looking at costs nothing.
    property var document: ({})

    readonly property var audio: document.audio ?? ({})
    readonly property var network: document.network ?? ({})
    readonly property var bluetooth: document.bluetooth ?? ({})
    readonly property var display: document.display ?? ({})
    readonly property var media: document.media ?? ({})

    Process {
        id: probe
        command: ["zenith", "state"]
        running: sidebar.refreshToken !== ""

        stdout: StdioCollector {
            onStreamFinished: {
                // Exit codes are not settled when the stream closes (the exit
                // signal lands after this collector), so the guard is content,
                // not exit code: `exitedWithCode` is still undefined here.
                // A failed probe leaves the previous document in place rather
                // than blanking the panel. And it does not fail on absent
                // hardware anyway — it reports it as absent, which is the point.
                if (text.trim() === "")
                    return
                try {
                    sidebar.document = JSON.parse(text)
                } catch (error) {
                    // Left as-is. A shell that throws on a parse error is a
                    // shell with no desktop.
                }
            }
        }
    }

    // The token restarts a finished Process, and is the idiom Theme.qml
    // already uses for the same reason.
    property string refreshToken: ""

    Timer {
        interval: 2000
        repeat: true
        running: sidebar.visible
        onTriggered: sidebar.refreshToken = Date.now()
    }

    onVisibleChanged: sidebar.refreshToken = Date.now()

    StyledRectangle {
        anchors.fill: parent
        contentLayer: StyledRectangle.ContentLayer.Pane

        Rectangle {
            anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
            width: 1
            color: Theme.lineDivider
        }

        Flickable {
            anchors { left: parent.left; right: parent.right; top: parent.top; bottom: parent.bottom }
            contentHeight: column.implicitHeight + 32
            clip: true

            Column {
                id: column
                width: parent.width
                topPadding: 16
                bottomPadding: 16

                Section {
                    title: "audio"
                    shown: sidebar.audio.available === true

                    Stat {
                        icon: sidebar.audio.muted ? Icons.glyph("volume-mute")
                            : sidebar.audio.volume > 0.5
                                ? Icons.glyph("volume-high")
                                : Icons.glyph("volume-low")
                        value: sidebar.audio.muted
                            ? "muted" : (sidebar.audio.percent ?? 0) + "%"
                    }
                    Dim { text: sidebar.audio.sink ?? "" }
                }

                Section {
                    title: "network"
                    shown: sidebar.network.available === true

                    Stat {
                        icon: Icons.glyph("wifi")
                        value: sidebar.network.wifi_on === true ? "on" : "off"
                    }

                    Repeater {
                        // The interfaces nobody connected are filtered here
                        // rather than in Go, because this is presentation and
                        // not a fact about the machine.
                        model: (sidebar.network.devices ?? []).filter(function (device) {
                            return device.type !== "loopback"
                                && device.type !== "bridge"
                                && device.type !== "bt"
                                && device.type !== "wifi-p2p"
                        })

                        Row {
                            required property var modelData
                            width: parent.width
                            spacing: 8

                            StyledText {
                                width: 92
                                text: modelData.name
                                color: Kit.Appearance.colors.colOnSurfaceVariant
                                font.pixelSize: 12
                            }
                            StyledText {
                                text: modelData.state
                                color: modelData.state.indexOf("connected") === 0
                                    ? Kit.Appearance.m3colors.m3onBackground
                                    : Kit.Appearance.colors.colOnSurfaceVariant
                                font.pixelSize: 12
                            }
                        }
                    }
                }

                Section {
                    title: "bluetooth"
                    shown: sidebar.bluetooth.present === true

                    Stat {
                        value: sidebar.bluetooth.powered === true
                            ? "on · " + (sidebar.bluetooth.devices ?? []).length + " device(s)"
                            : "off"
                    }
                    Repeater {
                        model: sidebar.bluetooth.devices ?? []
                        Dim { text: modelData.name }
                    }
                }

                Section {
                    title: "display"
                    shown: sidebar.display.available === true

                    Stat { value: (sidebar.display.percent ?? 0) + "%" }

                    // Read only, and deliberately. The sysfs node is 0644
                    // root:root, so a slider here would fail on every laptop. A
                    // control that does not work is worse than no control; see
                    // docs/architecture.md for the udev rule that would fix it.
                    Dim { text: "read only"; visible: sidebar.display.writable !== true }
                }

                Section {
                    title: "media"
                    shown: sidebar.media.available === true

                    Dim {
                        text: sidebar.media.title !== undefined && sidebar.media.title !== ""
                            ? sidebar.media.artist + " · " + sidebar.media.status
                            : sidebar.media.player + " · " + sidebar.media.status
                    }
                }
            }
        }
    }
}