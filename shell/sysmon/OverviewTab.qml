import QtQuick
import QtQuick.Controls

Item {
    id: tab
    property var stats
    property var app

    readonly property real cpuPct: stats && stats.cpu ? stats.cpu.usagePercent : 0
    readonly property real memPct: stats && stats.memory ? stats.memory.usedPercent : 0
    readonly property real swapPct: stats && stats.swap ? stats.swap.usedPercent : 0
    readonly property real cpuTemp: stats && stats.cpu ? stats.cpu.tempC : 0

    Flickable {
        anchors.fill: parent
        anchors.margins: 16
        contentWidth: width
        contentHeight: Math.max(grid.implicitHeight + 20, height)
        clip: true

        Grid {
            id: grid
            width: parent.width
            columns: parent.width > 760 ? 2 : 1
            spacing: 14

            // ── Card 1: CPU Performance ──
            Rectangle {
                width: (grid.width - (grid.columns - 1) * grid.spacing) / grid.columns
                height: 250
                radius: 8
                color: "#13151c"
                border.color: "#1f222d"
                border.width: 1

                Column {
                    anchors.fill: parent
                    anchors.margins: 16
                    spacing: 12

                    Row {
                        width: parent.width
                        Text {
                            text: "PROCESSOR (CPU)"
                            color: "#9ca3af"
                            font.family: "JetBrainsMono Nerd Font, monospace"
                            font.pixelSize: 11
                            font.weight: Font.SemiBold
                            font.letterSpacing: 1.0
                        }
                        Item { width: 1; height: 1; Layout.fillWidth: true }
                        Text {
                            text: tab.cpuTemp > 0 ? Math.round(tab.cpuTemp) + "°C" : ""
                            color: "#e5e7eb"
                            font.family: "JetBrainsMono Nerd Font, monospace"
                            font.pixelSize: 11
                        }
                    }

                    Row {
                        spacing: 14
                        anchors.horizontalCenter: parent.horizontalCenter
                        Text {
                            text: Math.round(tab.cpuPct) + "%"
                            color: "#ffffff"
                            font.family: "Space Grotesk, sans-serif"
                            font.pixelSize: 42
                            font.weight: Font.Bold
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        Column {
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: 3
                            Text {
                                text: "Load: " + (tab.stats && tab.stats.loadAvg ? tab.stats.loadAvg[0].toFixed(2) + " · " + tab.stats.loadAvg[1].toFixed(2) + " · " + tab.stats.loadAvg[2].toFixed(2) : "0.00")
                                color: "#9ca3af"
                                font.family: "JetBrainsMono Nerd Font, monospace"
                                font.pixelSize: 10
                            }
                            Text {
                                text: (tab.stats && tab.stats.cpu && tab.stats.cpu.cores ? tab.stats.cpu.cores.length : "0") + " Hardware Threads"
                                color: "#6b7280"
                                font.family: "Inter, sans-serif"
                                font.pixelSize: 11
                            }
                        }
                    }

                    // Usage Progress Bar
                    Rectangle {
                        width: parent.width
                        height: 8
                        radius: 4
                        color: "#1e212b"
                        Rectangle {
                            width: parent.width * Math.min(1.0, tab.cpuPct / 100.0)
                            height: parent.height
                            radius: 4
                            color: "#ffffff"
                        }
                    }

                    // Per-Core Visual Bars
                    Text {
                        text: "Core Distribution"
                        color: "#6b7280"
                        font.family: "Inter, sans-serif"
                        font.pixelSize: 10
                    }

                    Flow {
                        width: parent.width
                        spacing: 4
                        Repeater {
                            model: tab.stats && tab.stats.cpu && tab.stats.cpu.cores ? tab.stats.cpu.cores : []
                            delegate: Rectangle {
                                required property real modelData
                                required property int index
                                width: (parent.width - 32) / 8
                                height: 28
                                radius: 4
                                color: "#181b24"
                                border.color: "#242836"
                                border.width: 1

                                Rectangle {
                                    anchors.bottom: parent.bottom
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    height: parent.height * Math.min(1.0, modelData / 100.0)
                                    color: "#e5e7eb"
                                    opacity: 0.85
                                    radius: 3
                                }

                                Text {
                                    anchors.centerIn: parent
                                    text: "C" + index
                                    color: modelData > 50 ? "#000000" : "#9ca3af"
                                    font.family: "JetBrainsMono Nerd Font, monospace"
                                    font.pixelSize: 8
                                    font.weight: Font.Bold
                                }
                            }
                        }
                    }
                }
            }

            // ── Card 2: Memory & Swap ──
            Rectangle {
                width: (grid.width - (grid.columns - 1) * grid.spacing) / grid.columns
                height: 250
                radius: 8
                color: "#13151c"
                border.color: "#1f222d"
                border.width: 1

                Column {
                    anchors.fill: parent
                    anchors.margins: 16
                    spacing: 12

                    Text {
                        text: "MEMORY (RAM) & SWAP"
                        color: "#9ca3af"
                        font.family: "JetBrainsMono Nerd Font, monospace"
                        font.pixelSize: 11
                        font.weight: Font.SemiBold
                        font.letterSpacing: 1.0
                    }

                    Row {
                        spacing: 14
                        anchors.horizontalCenter: parent.horizontalCenter
                        Text {
                            text: Math.round(tab.memPct) + "%"
                            color: "#ffffff"
                            font.family: "Space Grotesk, sans-serif"
                            font.pixelSize: 42
                            font.weight: Font.Bold
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        Column {
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: 3
                            Text {
                                text: app.fmtBytes(tab.stats.memory.usedBytes) + " used of " + app.fmtBytes(tab.stats.memory.totalBytes)
                                color: "#e5e7eb"
                                font.family: "JetBrainsMono Nerd Font, monospace"
                                font.pixelSize: 11
                            }
                            Text {
                                text: "Available: " + app.fmtBytes(tab.stats.memory.availableBytes)
                                color: "#9ca3af"
                                font.family: "Inter, sans-serif"
                                font.pixelSize: 11
                            }
                        }
                    }

                    // Memory Progress Bar
                    Rectangle {
                        width: parent.width
                        height: 8
                        radius: 4
                        color: "#1e212b"
                        Rectangle {
                            width: parent.width * Math.min(1.0, tab.memPct / 100.0)
                            height: parent.height
                            radius: 4
                            color: "#e5e7eb"
                        }
                    }

                    // Memory Breakdown
                    Row {
                        width: parent.width
                        spacing: 8
                        Rectangle {
                            width: (parent.width - 8) / 2
                            height: 60
                            radius: 6
                            color: "#181b24"
                            border.color: "#242836"
                            border.width: 1
                            Column {
                                anchors.centerIn: parent
                                spacing: 2
                                Text { text: "CACHED"; color: "#6b7280"; font.pixelSize: 9; font.family: "JetBrainsMono Nerd Font, monospace" }
                                Text { text: app.fmtBytes(tab.stats.memory.cachedBytes); color: "#ffffff"; font.pixelSize: 12; font.weight: Font.SemiBold; font.family: "JetBrainsMono Nerd Font, monospace" }
                            }
                        }
                        Rectangle {
                            width: (parent.width - 8) / 2
                            height: 60
                            radius: 6
                            color: "#181b24"
                            border.color: "#242836"
                            border.width: 1
                            Column {
                                anchors.centerIn: parent
                                spacing: 2
                                Text { text: "SWAP"; color: "#6b7280"; font.pixelSize: 9; font.family: "JetBrainsMono Nerd Font, monospace" }
                                Text {
                                    text: tab.stats.swap.totalBytes > 0 ? app.fmtBytes(tab.stats.swap.usedBytes) + " (" + Math.round(tab.swapPct) + "%)" : "None"
                                    color: "#ffffff"
                                    font.pixelSize: 12
                                    font.weight: Font.SemiBold
                                    font.family: "JetBrainsMono Nerd Font, monospace"
                                }
                            }
                        }
                    }
                }
            }

            // ── Card 3: Network Throughput ──
            Rectangle {
                width: (grid.width - (grid.columns - 1) * grid.spacing) / grid.columns
                height: 205
                radius: 8
                color: "#13151c"
                border.color: "#1f222d"
                border.width: 1

                Column {
                    anchors.fill: parent
                    anchors.margins: 16
                    spacing: 12

                    Text {
                        text: "NETWORK ACTIVITY"
                        color: "#9ca3af"
                        font.family: "JetBrainsMono Nerd Font, monospace"
                        font.pixelSize: 11
                        font.weight: Font.SemiBold
                        font.letterSpacing: 1.0
                    }

                    Row {
                        width: parent.width
                        spacing: 12
                        Rectangle {
                            width: (parent.width - 12) / 2
                            height: 75
                            radius: 6
                            color: "#181b24"
                            border.color: "#242836"
                            border.width: 1
                            Column {
                                anchors.centerIn: parent
                                spacing: 3
                                Text { text: "\u2193 DOWNLOAD"; color: "#ffffff"; font.pixelSize: 10; font.weight: Font.Bold; font.family: "JetBrainsMono Nerd Font, monospace" }
                                Text { text: app.fmtRate(tab.stats.network.rxRateSec); color: "#ffffff"; font.pixelSize: 16; font.weight: Font.Bold; font.family: "JetBrainsMono Nerd Font, monospace" }
                                Text { text: "Total: " + app.fmtBytes(tab.stats.network.rxBytes); color: "#6b7280"; font.pixelSize: 9; font.family: "JetBrainsMono Nerd Font, monospace" }
                            }
                        }

                        Rectangle {
                            width: (parent.width - 12) / 2
                            height: 75
                            radius: 6
                            color: "#181b24"
                            border.color: "#242836"
                            border.width: 1
                            Column {
                                anchors.centerIn: parent
                                spacing: 3
                                Text { text: "\u2191 UPLOAD"; color: "#9ca3af"; font.pixelSize: 10; font.weight: Font.Bold; font.family: "JetBrainsMono Nerd Font, monospace" }
                                Text { text: app.fmtRate(tab.stats.network.txRateSec); color: "#ffffff"; font.pixelSize: 16; font.weight: Font.Bold; font.family: "JetBrainsMono Nerd Font, monospace" }
                                Text { text: "Total: " + app.fmtBytes(tab.stats.network.txBytes); color: "#6b7280"; font.pixelSize: 9; font.family: "JetBrainsMono Nerd Font, monospace" }
                            }
                        }
                    }

                    Text {
                        text: "Active Interfaces: " + (tab.stats.network.interfaces ? tab.stats.network.interfaces.map(function(i){ return i.name; }).join(", ") : "None")
                        color: "#6b7280"
                        font.family: "JetBrainsMono Nerd Font, monospace"
                        font.pixelSize: 10
                    }
                }
            }

            // ── Card 4: Storage & Live Disk I/O ──
            Rectangle {
                width: (grid.width - (grid.columns - 1) * grid.spacing) / grid.columns
                height: 205
                radius: 8
                color: "#13151c"
                border.color: "#1f222d"
                border.width: 1

                Column {
                    anchors.fill: parent
                    anchors.margins: 16
                    spacing: 12

                    Row {
                        width: parent.width
                        Text {
                            text: "STORAGE & DISK I/O"
                            color: "#9ca3af"
                            font.family: "JetBrainsMono Nerd Font, monospace"
                            font.pixelSize: 11
                            font.weight: Font.SemiBold
                            font.letterSpacing: 1.0
                        }
                        Item { Layout.fillWidth: true; width: 1; height: 1 }
                        Text {
                            text: tab.stats && tab.stats.diskIO ? Math.round(tab.stats.diskIO.readIOPS + tab.stats.diskIO.writeIOPS) + " IOPS" : ""
                            color: "#6b7280"
                            font.family: "JetBrainsMono Nerd Font, monospace"
                            font.pixelSize: 10
                        }
                    }

                    Row {
                        width: parent.width
                        spacing: 12
                        Rectangle {
                            width: (parent.width - 12) / 2
                            height: 60
                            radius: 6
                            color: "#181b24"
                            border.color: "#242836"
                            border.width: 1
                            Column {
                                anchors.centerIn: parent
                                spacing: 2
                                Text { text: "\u2193 DISK READ"; color: "#9ca3af"; font.pixelSize: 9; font.family: "JetBrainsMono Nerd Font, monospace" }
                                Text { text: app.fmtRate(tab.stats && tab.stats.diskIO ? tab.stats.diskIO.readBytesSec : 0); color: "#ffffff"; font.pixelSize: 13; font.weight: Font.Bold; font.family: "JetBrainsMono Nerd Font, monospace" }
                            }
                        }

                        Rectangle {
                            width: (parent.width - 12) / 2
                            height: 60
                            radius: 6
                            color: "#181b24"
                            border.color: "#242836"
                            border.width: 1
                            Column {
                                anchors.centerIn: parent
                                spacing: 2
                                Text { text: "\u2191 DISK WRITE"; color: "#9ca3af"; font.pixelSize: 9; font.family: "JetBrainsMono Nerd Font, monospace" }
                                Text { text: app.fmtRate(tab.stats && tab.stats.diskIO ? tab.stats.diskIO.writeBytesSec : 0); color: "#ffffff"; font.pixelSize: 13; font.weight: Font.Bold; font.family: "JetBrainsMono Nerd Font, monospace" }
                            }
                        }
                    }

                    // Primary Root Storage Bar
                    Item {
                        width: parent.width
                        height: 50
                        property var rootDisk: {
                            if (tab.stats && tab.stats.disks) {
                                for (var i = 0; i < tab.stats.disks.length; i++) {
                                    if (tab.stats.disks[i].mountPoint === "/") return tab.stats.disks[i];
                                }
                                if (tab.stats.disks.length > 0) return tab.stats.disks[0];
                            }
                            return null;
                        }

                        Column {
                            anchors.fill: parent
                            spacing: 6
                            visible: parent.rootDisk !== null

                            Row {
                                width: parent.width
                                Text {
                                    text: "Root (/) " + (parent.parent.rootDisk ? app.fmtBytes(parent.parent.rootDisk.usedBytes) + " / " + app.fmtBytes(parent.parent.rootDisk.totalBytes) : "")
                                    color: "#9ca3af"
                                    font.family: "JetBrainsMono Nerd Font, monospace"
                                    font.pixelSize: 10
                                }
                                Item { Layout.fillWidth: true; width: 1; height: 1 }
                                Text {
                                    text: parent.parent.rootDisk ? Math.round(parent.parent.rootDisk.usedPercent) + "%" : ""
                                    color: "#ffffff"
                                    font.family: "JetBrainsMono Nerd Font, monospace"
                                    font.pixelSize: 10
                                    font.weight: Font.Bold
                                }
                            }

                            Rectangle {
                                width: parent.width
                                height: 6
                                radius: 3
                                color: "#1e212b"
                                Rectangle {
                                    width: parent.width * Math.min(1.0, (parent.parent.parent.rootDisk ? parent.parent.parent.rootDisk.usedPercent : 0) / 100.0)
                                    height: parent.height
                                    radius: 3
                                    color: "#ffffff"
                                }
                            }
                        }
                    }
                }
            }

            // ── Dynamic Card 5: GPU Acceleration (if present) ──
            Repeater {
                model: tab.stats && tab.stats.gpus ? tab.stats.gpus : []
                delegate: Rectangle {
                    required property var modelData
                    width: (grid.width - (grid.columns - 1) * grid.spacing) / grid.columns
                    height: 205
                    radius: 8
                    color: "#13151c"
                    border.color: "#1f222d"
                    border.width: 1

                    Column {
                        anchors.fill: parent
                        anchors.margins: 16
                        spacing: 12

                        Row {
                            width: parent.width
                            Text {
                                text: "GRAPHICS ACCELERATOR (GPU)"
                                color: "#9ca3af"
                                font.family: "JetBrainsMono Nerd Font, monospace"
                                font.pixelSize: 11
                                font.weight: Font.SemiBold
                                font.letterSpacing: 1.0
                            }
                            Item { Layout.fillWidth: true; width: 1; height: 1 }
                            Text {
                                text: modelData.tempC > 0 ? Math.round(modelData.tempC) + "°C" : ""
                                color: "#e5e7eb"
                                font.family: "JetBrainsMono Nerd Font, monospace"
                                font.pixelSize: 11
                            }
                        }

                        Text {
                            text: modelData.name
                            color: "#ffffff"
                            font.family: "Inter, sans-serif"
                            font.pixelSize: 13
                            font.weight: Font.SemiBold
                            elide: Text.ElideRight
                            width: parent.width
                        }

                        Row {
                            spacing: 14
                            Text {
                                text: Math.round(modelData.utilizationPct) + "%"
                                color: "#ffffff"
                                font.family: "Space Grotesk, sans-serif"
                                font.pixelSize: 32
                                font.weight: Font.Bold
                                anchors.verticalCenter: parent.verticalCenter
                            }
                            Column {
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: 2
                                Text {
                                    text: modelData.memoryTotalMB > 0 ? "VRAM: " + modelData.memoryUsedMB + " MB / " + (modelData.memoryTotalMB >= 1024 ? (modelData.memoryTotalMB / 1024).toFixed(1) + " GB" : modelData.memoryTotalMB + " MB") : "Integrated Architecture"
                                    color: "#e5e7eb"
                                    font.family: "JetBrainsMono Nerd Font, monospace"
                                    font.pixelSize: 11
                                }
                                Text {
                                    text: "Vendor: " + modelData.vendor.toUpperCase()
                                    color: "#6b7280"
                                    font.family: "Inter, sans-serif"
                                    font.pixelSize: 10
                                }
                            }
                        }

                        // Utilization Bar
                        Rectangle {
                            width: parent.width
                            height: 6
                            radius: 3
                            color: "#1e212b"
                            Rectangle {
                                width: parent.width * Math.min(1.0, modelData.utilizationPct / 100.0)
                                height: parent.height
                                radius: 3
                                color: "#ffffff"
                            }
                        }
                    }
                }
            }

            // ── Dynamic Card 6: Power & Battery (if present) ──
            Repeater {
                model: tab.stats && tab.stats.batteries ? tab.stats.batteries : []
                delegate: Rectangle {
                    required property var modelData
                    width: (grid.width - (grid.columns - 1) * grid.spacing) / grid.columns
                    height: 205
                    radius: 8
                    color: "#13151c"
                    border.color: "#1f222d"
                    border.width: 1

                    Column {
                        anchors.fill: parent
                        anchors.margins: 16
                        spacing: 12

                        Row {
                            width: parent.width
                            Text {
                                text: "POWER & BATTERY (" + modelData.name + ")"
                                color: "#9ca3af"
                                font.family: "JetBrainsMono Nerd Font, monospace"
                                font.pixelSize: 11
                                font.weight: Font.SemiBold
                                font.letterSpacing: 1.0
                            }
                            Item { Layout.fillWidth: true; width: 1; height: 1 }
                            Text {
                                text: modelData.status
                                color: "#e5e7eb"
                                font.family: "JetBrainsMono Nerd Font, monospace"
                                font.pixelSize: 11
                                font.weight: Font.Medium
                            }
                        }

                        Row {
                            spacing: 14
                            Text {
                                text: Math.round(modelData.percentage) + "%"
                                color: "#ffffff"
                                font.family: "Space Grotesk, sans-serif"
                                font.pixelSize: 32
                                font.weight: Font.Bold
                                anchors.verticalCenter: parent.verticalCenter
                            }
                            Column {
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: 2
                                Text {
                                    text: modelData.watts > 0 ? modelData.watts.toFixed(1) + " W (" + modelData.voltageV.toFixed(2) + " V)" : (modelData.voltageV > 0 ? modelData.voltageV.toFixed(2) + " V" : "AC Connected")
                                    color: "#e5e7eb"
                                    font.family: "JetBrainsMono Nerd Font, monospace"
                                    font.pixelSize: 11
                                }
                                Text {
                                    text: modelData.healthPct > 0 ? "Battery Health: " + Math.round(modelData.healthPct) + "% (" + modelData.technology + ")" : modelData.technology
                                    color: "#6b7280"
                                    font.family: "Inter, sans-serif"
                                    font.pixelSize: 10
                                }
                            }
                        }

                        // Battery Capacity Bar
                        Rectangle {
                            width: parent.width
                            height: 6
                            radius: 3
                            color: "#1e212b"
                            Rectangle {
                                width: parent.width * Math.min(1.0, modelData.percentage / 100.0)
                                height: parent.height
                                radius: 3
                                color: modelData.percentage <= 15 ? "#ef4444" : "#ffffff"
                            }
                        }
                    }
                }
            }
        }
    }
}
