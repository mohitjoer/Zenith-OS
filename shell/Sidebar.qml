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

// The three text styles and the section frame, inline rather than in four files.
//
// A section header, a dimmed line and an icon are each four lines of QML and
// each used exactly once. As separate files they are four more paths for the
// packaging check to carry and four more names to collide with; as inline
// components they are visible at the point of use and gone from the catalogue.
// The check that this fails — a section header used in three panels becoming a
// shared component — has not happened yet, and promoting one then is a small
// change rather than a refactor.
component SectionTitle: Text {
    color: Palette.textDim
    font.pixelSize: 11
    font.capitalization: Font.AllUppercase
    font.letterSpacing: 1.2
}

component Dim: Text {
    color: Palette.textDim
    font.pixelSize: 12
    width: parent ? parent.width : 0
    elide: Text.ElideRight
}

component Glyph: Text {
    font.family: Icons.iconFamily
    font.pixelSize: 13
    color: Palette.text
}

component Section: Column {
    property string title: ""
    spacing: 6
    width: parent ? parent.width : 0

    SectionTitle { text: parent.title }
}

PanelWindow {
    id: sidebar

    // The panel is always this wide and slides. A panel that resized to its
    // contents would make the whole desktop reflow every time a bluetooth device
    // appeared, which is a desktop that moves under the pointer while you are
    // trying to click something.
    implicitWidth: 320
    implicitHeight: screen.height

    anchors {
        top: true
        right: true
    }

    color: Palette.background
    exclusionMode: ExclusionMode.Ignore

    // Read once per interval rather than watched.
    //
    // There is no watch here because there is nothing to watch. Every one of
    // these five tools reports state through a daemon Zenith's shell does not
    // own, so the only honest trigger is a poll. What makes that affordable is
    // that it only runs while the panel is open: a sidebar spawning five
    // subprocesses every two seconds while nobody is looking at it is five forks
    // a minute for a number that does not change.
    property var document: ({})
    property bool loaded: false
    property string refreshToken: ""

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
                // A failed probe leaves the previous document in place rather
                // than blanking the panel. Half a sidebar that briefly shows
                // nothing is worse than a stale section that is right often
                // enough — and `zenith state` does not fail on absent hardware
                // anyway, it reports it as absent, which is the whole point.
                if (probe.exitedWithCode !== 0) return
                try {
                    sidebar.document = JSON.parse(text)
                    sidebar.loaded = true
                } catch (error) {
                    // Left as-is. A shell that throws on a parse error is a
                    // shell with no desktop.
                }
            }
        }
    }

    // The token is how a finished Process is restarted, and it is the idiom
    // Palette.qml already uses for the same reason.
    Timer {
        interval: 2000
        repeat: true
        running: sidebar.visible
        onTriggered: sidebar.refreshToken = Date.now()
    }

    onVisibleChanged: sidebar.refreshToken = Date.now()

    Rectangle {
        anchors.fill: parent
        color: Palette.surface

        Rectangle {
            anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
            width: 1
            color: Palette.border
        }

        Flickable {
            anchors {
                left: parent.left
                right: parent.right
                top: parent.top
                bottom: parent.bottom
                margins: 1
            }
            contentHeight: column.implicitHeight + 32
            clip: true

            Column {
                id: column
                width: parent.width
                topPadding: 16
                bottomPadding: 16

                // ---- audio -------------------------------------------------
                //
                // Hidden when the section is unavailable rather than shown as an
                // error. A sidebar that reports "no audio" on a machine that
                // has audio, because one probe timed out, is a sidebar people
                // stop trusting.
                Section {
                    title: "audio"
                    visible: sidebar.audio.available === true
                    height: visible ? implicitHeight : 0
                    leftPadding: 16
                    rightPadding: 16

                    Row {
                        spacing: 8
                        Glyph {
                            text: sidebar.audio.muted ? Icons.glyph("volume-mute")
                                : sidebar.audio.volume > 0.5
                                    ? Icons.glyph("volume-high")
                                    : Icons.glyph("volume-low")
                        }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: sidebar.audio.muted
                                ? "muted"
                                : (sidebar.audio.percent ?? 0) + "%"
                            color: Palette.text
                            font.pixelSize: 13
                        }
                    }

                    Dim { text: sidebar.audio.sink ?? "" }
                }

                // ---- network -----------------------------------------------
                Section {
                    title: "network"
                    visible: sidebar.network.available === true
                    height: visible ? implicitHeight : 0
                    leftPadding: 16
                    rightPadding: 16

                    Row {
                        spacing: 8
                        Glyph { text: Icons.glyph("wifi") }
                        Text {
                            text: sidebar.network.wifi_on === true ? "on" : "off"
                            color: Palette.text
                            font.pixelSize: 13
                        }
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

                            Text {
                                width: 92
                                text: modelData.name
                                color: Palette.textDim
                                font.pixelSize: 12
                            }
                            Text {
                                text: modelData.state
                                color: modelData.state.indexOf("connected") === 0
                                    ? Palette.text : Palette.textDim
                                font.pixelSize: 12
                            }
                        }
                    }
                }

                // ---- bluetooth ---------------------------------------------
                Section {
                    title: "bluetooth"
                    visible: sidebar.bluetooth.present === true
                    height: visible ? implicitHeight : 0
                    leftPadding: 16
                    rightPadding: 16

                    Text {
                        text: sidebar.bluetooth.powered === true
                            ? "on · " + (sidebar.bluetooth.devices ?? []).length + " device(s)"
                            : "off"
                        color: Palette.text
                        font.pixelSize: 13
                    }

                    Repeater {
                        model: sidebar.bluetooth.devices ?? []
                        Dim { text: modelData.name }
                    }
                }

                // ---- display -----------------------------------------------
                Section {
                    title: "display"
                    visible: sidebar.display.available === true
                    height: visible ? implicitHeight : 0
                    leftPadding: 16
                    rightPadding: 16

                    Text {
                        text: (sidebar.display.percent ?? 0) + "%"
                        color: Palette.text
                        font.pixelSize: 13
                    }

                    // The brightness level is shown and cannot be changed.
                    //
                    // /sys/class/backlight/*/brightness is 0644 root:root on a
                    // stock image, so a slider here would fail every time it was
                    // dragged. A control that does not work is worse than no
                    // control, so there is none until Zenith lays down a udev
                    // rule — see docs/architecture.md.
                    Dim {
                        text: "read only"
                        visible: sidebar.display.writable !== true
                    }
                }

                // ---- media -------------------------------------------------
                Section {
                    title: "media"
                    visible: sidebar.media.available === true
                    height: visible ? implicitHeight : 0
                    leftPadding: 16
                    rightPadding: 16

                    Text {
                        width: parent.width
                        text: sidebar.media.title ?? sidebar.media.player ?? ""
                        color: Palette.text
                        font.pixelSize: 13
                        elide: Text.ElideRight
                    }
                    Dim {
                        text: [sidebar.media.artist ?? "", sidebar.media.status ?? ""]
                            .filter(function (part) { return part !== "" })
                            .join(" · ")
                        visible: (sidebar.media.title ?? "") !== ""
                    }
                }
            }
        }
    }
}