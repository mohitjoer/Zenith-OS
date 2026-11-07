import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io

Rectangle {
    id: root
    color: "#0c0d12"

    FontLoader { id: flUi; source: Qt.resolvedUrl("assets/fonts/SpaceGrotesk-Regular.ttf") }
    FontLoader { id: flUiMedium; source: Qt.resolvedUrl("assets/fonts/SpaceGrotesk-Medium.ttf") }
    FontLoader { id: flUiBold; source: Qt.resolvedUrl("assets/fonts/SpaceGrotesk-Bold.ttf") }
    FontLoader { id: flMono; source: Qt.resolvedUrl("assets/fonts/JetBrainsMonoNerdFont-Regular.ttf") }

    readonly property string fontUi: flUi.status === FontLoader.Ready ? flUi.name : "Inter, system-ui, -apple-system, sans-serif"
    readonly property string fontMono: flMono.status === FontLoader.Ready ? flMono.name : "JetBrains Mono, monospace"

    property string currentTab: "performance" // performance, processes, storage, system
    property var stats: ({
        cpu: { usagePercent: 0, cores: [], tempC: 0 },
        memory: { totalBytes: 0, usedBytes: 0, availableBytes: 0, usedPercent: 0, cachedBytes: 0 },
        swap: { totalBytes: 0, usedBytes: 0, usedPercent: 0 },
        network: { rxBytes: 0, txBytes: 0, rxRateSec: 0, txRateSec: 0, interfaces: [] },
        disks: [],
        diskIO: { readBytesSec: 0, writeBytesSec: 0, readIOPS: 0, writeIOPS: 0, devices: [] },
        gpus: [],
        batteries: [],
        uptimeSecs: 0,
        loadAvg: [0, 0, 0]
    })
    property var processList: []
    property bool isPolling: true

    function fmtBytes(b) {
        if (!b || b <= 0) return "0 B";
        var k = 1024;
        var sizes = ["B", "KB", "MB", "GB", "TB"];
        var i = Math.floor(Math.log(b) / Math.log(k));
        if (i < 0) i = 0;
        if (i >= sizes.length) i = sizes.length - 1;
        return (b / Math.pow(k, i)).toFixed(i > 1 ? 1 : 0) + " " + sizes[i];
    }

    function fmtRate(b) {
        if (!b || b <= 0) return "0 B/s";
        return fmtBytes(b) + "/s";
    }

    function fmtUptime(secs) {
        if (!secs || secs <= 0) return "0m";
        var h = Math.floor(secs / 3600);
        var m = Math.floor((secs % 3600) / 60);
        if (h > 0) return h + "h " + m + "m";
        return m + "m";
    }

    // Polling for live system stats
    Process {
        id: statsProc
        command: ["asura-sysmon", "stats", "--interval", "150"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var parsed = JSON.parse(text);
                    if (parsed && parsed.cpu) {
                        root.stats = parsed;
                    }
                } catch(e) {}
            }
        }
    }

    // Polling for active processes
    Process {
        id: procProc
        command: ["asura-sysmon", "proc", "--limit", "120"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var parsed = JSON.parse(text);
                    if (parsed && parsed.processes) {
                        root.processList = parsed.processes;
                    }
                } catch(e) {}
            }
        }
    }

    Timer {
        id: pollTimer
        interval: 1800
        repeat: true
        running: root.isPolling
        triggeredOnStart: true
        onTriggered: {
            if (!statsProc.running) statsProc.running = true;
            if (root.currentTab === "processes" && !procProc.running) {
                procProc.running = true;
            }
        }
    }

    onCurrentTabChanged: {
        if (root.currentTab === "processes" && !procProc.running) {
            procProc.running = true;
        }
    }

    // ── Header Bar ──────────────────────────────────────────────────────────
    Rectangle {
        id: header
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: 52
        color: "#12141a"
        border.color: "#1e212b"
        border.width: 1

        Row {
            anchors.left: parent.left
            anchors.leftMargin: 16
            anchors.verticalCenter: parent.verticalCenter
            spacing: 12

            Image {
                width: 20
                height: 20
                anchors.verticalCenter: parent.verticalCenter
                source: Qt.resolvedUrl("assets/asura-logo-on-dark.svg")
                fillMode: Image.PreserveAspectFit
                smooth: true
                // Fallback text if image not accessible
                Text {
                    visible: parent.status !== Image.Ready
                    anchors.centerIn: parent
                    text: "\u25c8"
                    color: "#ffffff"
                    font.pixelSize: 16
                }
            }

            Text {
                text: "Resource Monitor"
                color: "#ffffff"
                font.family: root.fontUi
                font.pixelSize: 15
                font.weight: Font.DemiBold
                anchors.verticalCenter: parent.verticalCenter
            }

            Rectangle {
                width: 1
                height: 18
                color: "#2a2d39"
                anchors.verticalCenter: parent.verticalCenter
            }

            Text {
                text: "ASURA OS"
                color: "#6b7280"
                font.family: root.fontMono
                font.pixelSize: 10
                font.weight: Font.Medium
                font.letterSpacing: 1.5
                anchors.verticalCenter: parent.verticalCenter
            }
        }

        // Navigation Tabs
        Row {
            anchors.centerIn: parent
            spacing: 4

            Repeater {
                model: [
                    { id: "performance", label: "Performance" },
                    { id: "processes", label: "Processes" },
                    { id: "storage", label: "Storage" },
                    { id: "system", label: "System" }
                ]

                delegate: Rectangle {
                    required property var modelData
                    width: tabText.implicitWidth + 24
                    height: 32
                    radius: 6
                    color: root.currentTab === modelData.id ? "#222634" : (tabMouse.containsMouse ? "#181b24" : "transparent")
                    border.color: root.currentTab === modelData.id ? "#374151" : "transparent"
                    border.width: 1

                    Text {
                        id: tabText
                        anchors.centerIn: parent
                        text: modelData.label
                        color: root.currentTab === modelData.id ? "#ffffff" : "#9ca3af"
                        font.family: root.fontUi
                        font.pixelSize: 12
                        font.weight: root.currentTab === modelData.id ? Font.DemiBold : Font.Normal
                    }

                    MouseArea {
                        id: tabMouse
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.currentTab = modelData.id
                    }
                }
            }
        }

        // Window Controls / Refresh Pill
        Row {
            anchors.right: parent.right
            anchors.rightMargin: 16
            anchors.verticalCenter: parent.verticalCenter
            spacing: 10

            Rectangle {
                width: 60
                height: 24
                radius: 12
                color: "#181a22"
                border.color: "#282b37"
                border.width: 1

                Row {
                    anchors.centerIn: parent
                    spacing: 5
                    Rectangle {
                        width: 6
                        height: 6
                        radius: 3
                        color: statsProc.running ? "#38bdf8" : "#22c55e"
                        anchors.verticalCenter: parent.verticalCenter
                    }
                    Text {
                        text: "LIVE"
                        color: "#9ca3af"
                        font.family: "JetBrainsMono Nerd Font, monospace"
                        font.pixelSize: 9
                        font.weight: Font.Bold
                        anchors.verticalCenter: parent.verticalCenter
                    }
                }
            }

            Rectangle {
                width: 28
                height: 28
                radius: 6
                color: closeMouse.containsMouse ? "#2d1616" : "#181a22"
                border.color: closeMouse.containsMouse ? "#7f1d1d" : "#282b37"
                border.width: 1

                Text {
                    anchors.centerIn: parent
                    text: "\u2715"
                    color: closeMouse.containsMouse ? "#ef4444" : "#9ca3af"
                    font.pixelSize: 12
                }

                MouseArea {
                    id: closeMouse
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: Qt.quit()
                }
            }
        }
    }

    // ── Tab Views ───────────────────────────────────────────────────────────
    Item {
        id: viewHost
        anchors.top: header.bottom
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right

        OverviewTab {
            anchors.fill: parent
            visible: root.currentTab === "performance"
            stats: root.stats
            app: root
        }

        ProcessesTab {
            anchors.fill: parent
            visible: root.currentTab === "processes"
            processes: root.processList
            app: root
        }

        DisksTab {
            anchors.fill: parent
            visible: root.currentTab === "storage"
            disks: root.stats.disks || []
            diskIO: root.stats.diskIO || { readBytesSec: 0, writeBytesSec: 0, readIOPS: 0, writeIOPS: 0, devices: [] }
            app: root
        }

        SystemTab {
            anchors.fill: parent
            visible: root.currentTab === "system"
            stats: root.stats
            app: root
        }
    }
}
