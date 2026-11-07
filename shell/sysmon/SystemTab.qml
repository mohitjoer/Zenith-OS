import QtQuick
import QtQuick.Controls

Item {
    id: tab
    property var stats
    property var app

    readonly property string fontUi: app ? app.fontUi : "Inter, system-ui, sans-serif"
    readonly property string fontMono: app ? app.fontMono : "JetBrains Mono, monospace"

    Flickable {
        anchors.fill: parent
        anchors.margins: 16
        contentWidth: width
        contentHeight: Math.max(col.implicitHeight + 20, height)
        clip: true

        Column {
            id: col
            width: parent.width
            spacing: 16

            Rectangle {
                width: parent.width
                height: 110
                radius: 8
                color: "#13151c"
                border.color: "#1f222d"
                border.width: 1

                Row {
                    anchors.fill: parent
                    anchors.margins: 20
                    spacing: 20

                    Image {
                        width: 56
                        height: 56
                        source: Qt.resolvedUrl("assets/asura-logo-on-dark.svg")
                        fillMode: Image.PreserveAspectFit
                        smooth: true
                        anchors.verticalCenter: parent.verticalCenter
                    }

                    Column {
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 4
                        Text {
                            text: "ASURA OS"
                            color: "#ffffff"
                            font.family: tab.fontUi
                            font.pixelSize: 22
                            font.weight: Font.Bold
                        }
                        Text {
                            text: "Pure Monochrome Desktop Operating System"
                            color: "#9ca3af"
                            font.family: tab.fontUi
                            font.pixelSize: 12
                        }
                    }
                }
            }

            Rectangle {
                width: parent.width
                height: 310
                radius: 8
                color: "#13151c"
                border.color: "#1f222d"
                border.width: 1

                Column {
                    anchors.fill: parent
                    anchors.margins: 16
                    spacing: 12

                    Text {
                        text: "System Specifications"
                        color: "#9ca3af"
                        font.family: tab.fontUi
                        font.pixelSize: 12
                        font.weight: Font.Medium
                    }

                    Rectangle { width: parent.width; height: 1; color: "#1e212b" }

                    Grid {
                        width: parent.width
                        columns: 2
                        rowSpacing: 12
                        columnSpacing: 20

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "Operating System"; color: "#9ca3af"; font.pixelSize: 11; font.family: tab.fontUi }
                            Text { text: "ASURA OS"; color: "#ffffff"; font.pixelSize: 13; font.family: tab.fontUi; font.weight: Font.Medium }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "System Uptime"; color: "#9ca3af"; font.pixelSize: 11; font.family: tab.fontUi }
                            Text { text: tab.app.fmtUptime(tab.stats ? tab.stats.uptimeSecs : 0); color: "#ffffff"; font.pixelSize: 13; font.family: tab.fontMono; font.weight: Font.Medium }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "Total Memory"; color: "#9ca3af"; font.pixelSize: 11; font.family: tab.fontUi }
                            Text { text: tab.app.fmtBytes(tab.stats && tab.stats.memory ? tab.stats.memory.totalBytes : 0); color: "#ffffff"; font.pixelSize: 13; font.family: tab.fontMono; font.weight: Font.Medium }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "Processor Cores"; color: "#9ca3af"; font.pixelSize: 11; font.family: tab.fontUi }
                            Text { text: (tab.stats && tab.stats.cpu && tab.stats.cpu.cores ? tab.stats.cpu.cores.length : "0") + " Logical Cores"; color: "#ffffff"; font.pixelSize: 13; font.family: tab.fontUi; font.weight: Font.Medium }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "Window Compositor"; color: "#9ca3af"; font.pixelSize: 11; font.family: tab.fontUi }
                            Text { text: "Hyprland (Wayland)"; color: "#ffffff"; font.pixelSize: 13; font.family: tab.fontUi; font.weight: Font.Medium }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "Load Average"; color: "#9ca3af"; font.pixelSize: 11; font.family: tab.fontUi }
                            Text { text: tab.stats && tab.stats.loadAvg ? tab.stats.loadAvg[0].toFixed(2) + " (1m), " + tab.stats.loadAvg[1].toFixed(2) + " (5m)" : "0.00"; color: "#ffffff"; font.pixelSize: 13; font.family: tab.fontMono; font.weight: Font.Medium }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "Graphics Adapters"; color: "#9ca3af"; font.pixelSize: 11; font.family: tab.fontUi }
                            Text {
                                text: tab.stats && tab.stats.gpus && tab.stats.gpus.length > 0 ? tab.stats.gpus.map(function(g){ return g.name; }).join(" · ") : "Virtual / Host Framebuffer"
                                color: "#ffffff"
                                font.pixelSize: 12
                                font.family: tab.fontUi
                                font.weight: Font.Medium
                                elide: Text.ElideRight
                                width: parent.width
                            }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "Power Source"; color: "#9ca3af"; font.pixelSize: 11; font.family: tab.fontUi }
                            Text {
                                text: tab.stats && tab.stats.batteries && tab.stats.batteries.length > 0 ? tab.stats.batteries.map(function(b){ return b.name + " (" + Math.round(b.percentage) + "% · " + (b.healthPct > 0 ? Math.round(b.healthPct) + "% Health" : b.status) + ")"; }).join(", ") : "AC Mains Power"
                                color: "#ffffff"
                                font.pixelSize: 12
                                font.family: tab.fontUi
                                font.weight: Font.Medium
                                elide: Text.ElideRight
                                width: parent.width
                            }
                        }
                    }
                }
            }
        }
    }
}
