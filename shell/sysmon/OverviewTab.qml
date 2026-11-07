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

    readonly property string fontUi: app ? app.fontUi : "Inter, system-ui, sans-serif"
    readonly property string fontMono: app ? app.fontMono : "JetBrains Mono, monospace"

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

                    Item {
                        width: parent.width
                        height: 18
                        Text {
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            text: "Processor (CPU)"
                            color: "#9ca3af"
                            font.family: tab.fontUi
                            font.pixelSize: 12
                            font.weight: Font.Medium
                        }
                        Text {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            text: tab.cpuTemp > 0 ? Math.round(tab.cpuTemp) + " °C" : ""
                            color: "#e5e7eb"
                            font.family: tab.fontMono
                            font.pixelSize: 11
                        }
                    }

                    Row {
                        spacing: 14
                        anchors.horizontalCenter: parent.horizontalCenter
                        Text {
                            text: Math.round(tab.cpuPct) + "%"
                            color: "#ffffff"
                            font.family: tab.fontUi
                            font.pixelSize: 38
                            font.weight: Font.Bold
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        Column {
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: 3
                            Text {
                                text: "Load: " + (tab.stats && tab.stats.loadAvg ? tab.stats.loadAvg.map(function(v) { return v.toFixed(2); }).join(", ") : "0.00")
                                color: "#d1d5db"
                                font.family: tab.fontMono
                                font.pixelSize: 11
                            }
                            Text {
                                text: (tab.stats && tab.stats.cpu && tab.stats.cpu.cores ? tab.stats.cpu.cores.length : "0") + " Logical Cores"
                                color: "#9ca3af"
                                font.family: tab.fontUi
                                font.pixelSize: 11
                            }
                        }
                    }

                    // Usage Progress Bar
                    Rectangle {
                        width: parent.width
                        height: 6
                        radius: 3
                        color: "#1e212b"
                        Rectangle {
                            width: parent.width * Math.min(1.0, Math.max(0.01, tab.cpuPct / 100.0))
                            height: parent.height
                            radius: 3
                            color: "#ffffff"
                        }
                    }

                    // Per-Core Visual Bars
                    Text {
                        text: "Core Distribution"
                        color: "#9ca3af"
                        font.family: tab.fontUi
                        font.pixelSize: 11
                    }

                    Flow {
                        width: parent.width
                        spacing: 4
                        Repeater {
                            model: tab.stats && tab.stats.cpu && tab.stats.cpu.cores ? tab.stats.cpu.cores : []
                            delegate: Rectangle {
                                required property real modelData
                                required property int index
                                width: Math.max(28, (parent.width - 32) / 8)
                                height: 26
                                radius: 4
                                color: "#181b24"
                                border.color: "#242836"
                                border.width: 1

                                Rectangle {
                                    anchors.bottom: parent.bottom
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    height: Math.max(2, parent.height * Math.min(1.0, modelData / 100.0))
                                    color: "#e5e7eb"
                                    opacity: 0.85
                                    radius: 3
                                }

                                Text {
                                    anchors.centerIn: parent
                                    text: String(index + 1)
                                    color: modelData > 60 ? "#000000" : "#9ca3af"
                                    font.family: tab.fontMono
                                    font.pixelSize: 9
                                    font.weight: Font.DemiBold
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

                    Item {
                        width: parent.width
                        height: 18
                        Text {
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            text: "Memory (RAM) & Swap"
                            color: "#9ca3af"
                            font.family: tab.fontUi
                            font.pixelSize: 12
                            font.weight: Font.Medium
                        }
                    }

                    Row {
                        spacing: 14
                        anchors.horizontalCenter: parent.horizontalCenter
                        Text {
                            text: Math.round(tab.memPct) + "%"
                            color: "#ffffff"
                            font.family: tab.fontUi
                            font.pixelSize: 38
                            font.weight: Font.Bold
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        Column {
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: 3
                            Text {
                                text: (tab.stats && tab.stats.memory ? app.fmtBytes(tab.stats.memory.usedBytes) + " used of " + app.fmtBytes(tab.stats.memory.totalBytes) : "")
                                color: "#e5e7eb"
                                font.family: tab.fontMono
                                font.pixelSize: 11
                            }
                            Text {
                                text: "Available: " + (tab.stats && tab.stats.memory ? app.fmtBytes(tab.stats.memory.availableBytes) : "")
                                color: "#9ca3af"
                                font.family: tab.fontUi
                                font.pixelSize: 11
                            }
                        }
                    }

                    // Memory Progress Bar
                    Rectangle {
                        width: parent.width
                        height: 6
                        radius: 3
                        color: "#1e212b"
                        Rectangle {
                            width: parent.width * Math.min(1.0, Math.max(0.01, tab.memPct / 100.0))
                            height: parent.height
                            radius: 3
                            color: "#ffffff"
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
                                Text { text: "Cached"; color: "#6b7280"; font.pixelSize: 10; font.family: tab.fontUi }
                                Text { text: tab.stats && tab.stats.memory ? app.fmtBytes(tab.stats.memory.cachedBytes) : "0 B"; color: "#ffffff"; font.pixelSize: 12; font.weight: Font.DemiBold; font.family: tab.fontMono }
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
                                Text { text: "Swap"; color: "#6b7280"; font.pixelSize: 10; font.family: tab.fontUi }
                                Text {
                                    text: tab.stats && tab.stats.swap && tab.stats.swap.totalBytes > 0 ? app.fmtBytes(tab.stats.swap.usedBytes) + " (" + Math.round(tab.swapPct) + "%)" : "Not configured"
                                    color: "#ffffff"
                                    font.pixelSize: 12
                                    font.weight: Font.DemiBold
                                    font.family: tab.fontMono
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

                    Item {
                        width: parent.width
                        height: 18
                        Text {
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            text: "Network Activity"
                            color: "#9ca3af"
                            font.family: tab.fontUi
                            font.pixelSize: 12
                            font.weight: Font.Medium
                        }
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
                                Text { text: "↓ Download"; color: "#ffffff"; font.pixelSize: 11; font.weight: Font.DemiBold; font.family: tab.fontUi }
                                Text { text: tab.stats && tab.stats.network ? app.fmtRate(tab.stats.network.rxRateSec) : "0 B/s"; color: "#ffffff"; font.pixelSize: 15; font.weight: Font.Bold; font.family: tab.fontMono }
                                Text { text: "Total: " + (tab.stats && tab.stats.network ? app.fmtBytes(tab.stats.network.rxBytes) : "0 B"); color: "#9ca3af"; font.pixelSize: 10; font.family: tab.fontMono }
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
                                Text { text: "↑ Upload"; color: "#9ca3af"; font.pixelSize: 11; font.weight: Font.DemiBold; font.family: tab.fontUi }
                                Text { text: tab.stats && tab.stats.network ? app.fmtRate(tab.stats.network.txRateSec) : "0 B/s"; color: "#ffffff"; font.pixelSize: 15; font.weight: Font.Bold; font.family: tab.fontMono }
                                Text { text: "Total: " + (tab.stats && tab.stats.network ? app.fmtBytes(tab.stats.network.txBytes) : "0 B"); color: "#9ca3af"; font.pixelSize: 10; font.family: tab.fontMono }
                            }
                        }
                    }

                    Text {
                        text: "Active Interfaces: " + (tab.stats && tab.stats.network && tab.stats.network.interfaces ? tab.stats.network.interfaces.map(function(i){ return i.name; }).join(", ") : "None")
                        color: "#6b7280"
                        font.family: tab.fontMono
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

                    Item {
                        width: parent.width
                        height: 18
                        Text {
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            text: "Storage & Disk I/O"
                            color: "#9ca3af"
                            font.family: tab.fontUi
                            font.pixelSize: 12
                            font.weight: Font.Medium
                        }
                        Text {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            text: tab.stats && tab.stats.diskIO && (tab.stats.diskIO.readIOPS + tab.stats.diskIO.writeIOPS > 0) ? Math.round(tab.stats.diskIO.readIOPS + tab.stats.diskIO.writeIOPS) + " IOPS" : ""
                            color: "#6b7280"
                            font.family: tab.fontMono
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
                                Text { text: "↓ Disk Read"; color: "#9ca3af"; font.pixelSize: 10; font.family: tab.fontUi }
                                Text { text: tab.stats && tab.stats.diskIO ? app.fmtRate(tab.stats.diskIO.readBytesSec) : "0 B/s"; color: "#ffffff"; font.pixelSize: 13; font.weight: Font.Bold; font.family: tab.fontMono }
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
                                Text { text: "↑ Disk Write"; color: "#9ca3af"; font.pixelSize: 10; font.family: tab.fontUi }
                                Text { text: tab.stats && tab.stats.diskIO ? app.fmtRate(tab.stats.diskIO.writeBytesSec) : "0 B/s"; color: "#ffffff"; font.pixelSize: 13; font.weight: Font.Bold; font.family: tab.fontMono }
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

                            Item {
                                width: parent.width
                                height: 16
                                Text {
                                    anchors.left: parent.left
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: "Root (/) " + (parent.parent.rootDisk ? app.fmtBytes(parent.parent.rootDisk.usedBytes) + " / " + app.fmtBytes(parent.parent.rootDisk.totalBytes) : "")
                                    color: "#9ca3af"
                                    font.family: tab.fontMono
                                    font.pixelSize: 11
                                }
                                Text {
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: parent.parent.rootDisk ? Math.round(parent.parent.rootDisk.usedPercent) + "%" : ""
                                    color: "#ffffff"
                                    font.family: tab.fontMono
                                    font.pixelSize: 11
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

            // ── Dynamic Card 5: GPU / Graphics ──
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

                        Item {
                            width: parent.width
                            height: 18
                            Text {
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                text: modelData.isVirtual ? "Virtual Display Adapter" : "Graphics Processor"
                                color: "#9ca3af"
                                font.family: tab.fontUi
                                font.pixelSize: 12
                                font.weight: Font.Medium
                            }
                            Text {
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                text: modelData.tempC > 0 ? Math.round(modelData.tempC) + " °C" : ""
                                color: "#e5e7eb"
                                font.family: tab.fontMono
                                font.pixelSize: 11
                            }
                        }

                        Text {
                            text: modelData.name
                            color: "#ffffff"
                            font.family: tab.fontUi
                            font.pixelSize: 13
                            font.weight: Font.DemiBold
                            elide: Text.ElideRight
                            width: parent.width
                        }

                        // If virtual machine adapter (VMware, VirtIO, VirtualBox)
                        Column {
                            visible: modelData.isVirtual === true
                            width: parent.width
                            spacing: 6

                            Rectangle {
                                width: parent.width
                                height: 50
                                radius: 6
                                color: "#181b24"
                                border.color: "#242836"
                                border.width: 1
                                Column {
                                    anchors.centerIn: parent
                                    spacing: 3
                                    Text { text: "Virtual Machine Display"; color: "#ffffff"; font.pixelSize: 11; font.weight: Font.DemiBold; font.family: tab.fontUi }
                                    Text { text: "Graphics acceleration provided by host hypervisor"; color: "#9ca3af"; font.pixelSize: 10; font.family: tab.fontUi }
                                }
                            }
                        }

                        // If hardware GPU (NVIDIA, AMD, Intel)
                        Column {
                            visible: modelData.isVirtual !== true
                            width: parent.width
                            spacing: 10

                            Row {
                                spacing: 14
                                Text {
                                    text: Math.round(modelData.utilizationPct) + "%"
                                    color: "#ffffff"
                                    font.family: tab.fontUi
                                    font.pixelSize: 32
                                    font.weight: Font.Bold
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                Column {
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: 2
                                    Text {
                                        text: modelData.memoryTotalMB > 0 ? "VRAM: " + modelData.memoryUsedMB + " MB / " + (modelData.memoryTotalMB >= 1024 ? (modelData.memoryTotalMB / 1024).toFixed(1) + " GB" : modelData.memoryTotalMB + " MB") : "Shared System Memory"
                                        color: "#e5e7eb"
                                        font.family: tab.fontMono
                                        font.pixelSize: 11
                                    }
                                    Text {
                                        text: "Vendor: " + modelData.vendor.toUpperCase()
                                        color: "#9ca3af"
                                        font.family: tab.fontUi
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
                                    width: parent.width * Math.min(1.0, Math.max(0.01, modelData.utilizationPct / 100.0))
                                    height: parent.height
                                    radius: 3
                                    color: "#ffffff"
                                }
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

                        Item {
                            width: parent.width
                            height: 18
                            Text {
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                text: "Power & Battery (" + modelData.name + ")"
                                color: "#9ca3af"
                                font.family: tab.fontUi
                                font.pixelSize: 12
                                font.weight: Font.Medium
                            }
                            Text {
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                text: modelData.status
                                color: "#e5e7eb"
                                font.family: tab.fontUi
                                font.pixelSize: 11
                            }
                        }

                        Row {
                            spacing: 14
                            Text {
                                text: Math.round(modelData.percentage) + "%"
                                color: "#ffffff"
                                font.family: tab.fontUi
                                font.pixelSize: 32
                                font.weight: Font.Bold
                                anchors.verticalCenter: parent.verticalCenter
                            }
                            Column {
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: 2
                                Text {
                                    text: modelData.watts > 0 ? modelData.watts.toFixed(1) + " W" + (modelData.voltageV > 0 ? " (" + modelData.voltageV.toFixed(1) + " V)" : "") : (modelData.voltageV > 0 ? modelData.voltageV.toFixed(1) + " V" : "AC Line Connected")
                                    color: "#e5e7eb"
                                    font.family: tab.fontMono
                                    font.pixelSize: 11
                                }
                                Text {
                                    text: modelData.healthPct > 0 ? "Battery Health: " + Math.round(modelData.healthPct) + "%" : "Power Source: " + modelData.status
                                    color: "#9ca3af"
                                    font.family: tab.fontUi
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
                                width: parent.width * Math.min(1.0, Math.max(0.01, modelData.percentage / 100.0))
                                height: parent.height
                                radius: 3
                                color: "#ffffff"
                            }
                        }
                    }
                }
            }
        }
    }
}
