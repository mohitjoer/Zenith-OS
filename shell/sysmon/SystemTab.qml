import QtQuick
import QtQuick.Controls

Item {
    id: tab
    property var stats
    property var app

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
                        source: "/usr/share/asura/branding/asura-logo-on-dark.svg"
                        fillMode: Image.PreserveAspectFit
                        smooth: true
                        anchors.verticalCenter: parent.verticalCenter
                    }

                    Column {
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 4
                        Text {
                            text: "Asura OS"
                            color: "#ffffff"
                            font.family: "Space Grotesk, sans-serif"
                            font.pixelSize: 22
                            font.weight: Font.Bold
                        }
                        Text {
                            text: "Pure Monochrome Desktop Operating System · Beta 001"
                            color: "#9ca3af"
                            font.family: "Inter, sans-serif"
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
                        text: "SYSTEM SPECIFICATIONS"
                        color: "#9ca3af"
                        font.family: "JetBrainsMono Nerd Font, monospace"
                        font.pixelSize: 11
                        font.weight: Font.SemiBold
                        font.letterSpacing: 1.0
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
                            Text { text: "OPERATING SYSTEM"; color: "#6b7280"; font.pixelSize: 10; font.family: "JetBrainsMono Nerd Font, monospace" }
                            Text { text: "Asura OS"; color: "#ffffff"; font.pixelSize: 13; font.family: "Inter, sans-serif"; font.weight: Font.Medium }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "SYSTEM UPTIME"; color: "#6b7280"; font.pixelSize: 10; font.family: "JetBrainsMono Nerd Font, monospace" }
                            Text { text: tab.app.fmtUptime(tab.stats ? tab.stats.uptimeSecs : 0); color: "#ffffff"; font.pixelSize: 13; font.family: "Inter, sans-serif"; font.weight: Font.Medium }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "TOTAL MEMORY"; color: "#6b7280"; font.pixelSize: 10; font.family: "JetBrainsMono Nerd Font, monospace" }
                            Text { text: tab.app.fmtBytes(tab.stats && tab.stats.memory ? tab.stats.memory.totalBytes : 0); color: "#ffffff"; font.pixelSize: 13; font.family: "Inter, sans-serif"; font.weight: Font.Medium }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "HARDWARE THREADS"; color: "#6b7280"; font.pixelSize: 10; font.family: "JetBrainsMono Nerd Font, monospace" }
                            Text { text: (tab.stats && tab.stats.cpu && tab.stats.cpu.cores ? tab.stats.cpu.cores.length : "0") + " Logical Cores"; color: "#ffffff"; font.pixelSize: 13; font.family: "Inter, sans-serif"; font.weight: Font.Medium }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "WINDOW COMPOSITOR"; color: "#6b7280"; font.pixelSize: 10; font.family: "JetBrainsMono Nerd Font, monospace" }
                            Text { text: "Hyprland (Wayland)"; color: "#ffffff"; font.pixelSize: 13; font.family: "Inter, sans-serif"; font.weight: Font.Medium }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "LOAD AVERAGE"; color: "#6b7280"; font.pixelSize: 10; font.family: "JetBrainsMono Nerd Font, monospace" }
                            Text { text: tab.stats && tab.stats.loadAvg ? tab.stats.loadAvg[0].toFixed(2) + " (1m), " + tab.stats.loadAvg[1].toFixed(2) + " (5m)" : "0.00"; color: "#ffffff"; font.pixelSize: 13; font.family: "Inter, sans-serif"; font.weight: Font.Medium }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "GRAPHICS ACCELERATORS"; color: "#6b7280"; font.pixelSize: 10; font.family: "JetBrainsMono Nerd Font, monospace" }
                            Text {
                                text: tab.stats && tab.stats.gpus && tab.stats.gpus.length > 0 ? tab.stats.gpus.map(function(g){ return g.name; }).join(" · ") : "Integrated / Host GPU"
                                color: "#ffffff"
                                font.pixelSize: 12
                                font.family: "Inter, sans-serif"
                                font.weight: Font.Medium
                                elide: Text.ElideRight
                                width: parent.width
                            }
                        }

                        Column {
                            width: (parent.width - 20) / 2
                            spacing: 2
                            Text { text: "POWER & BATTERY"; color: "#6b7280"; font.pixelSize: 10; font.family: "JetBrainsMono Nerd Font, monospace" }
                            Text {
                                text: tab.stats && tab.stats.batteries && tab.stats.batteries.length > 0 ? tab.stats.batteries.map(function(b){ return b.name + " (" + Math.round(b.percentage) + "% · " + (b.healthPct > 0 ? Math.round(b.healthPct) + "% Health" : b.status) + ")"; }).join(", ") : "AC Mains Power"
                                color: "#ffffff"
                                font.pixelSize: 12
                                font.family: "Inter, sans-serif"
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
