import QtQuick
import QtQuick.Controls

Item {
    id: tab
    property var disks: []
    property var app

    Flickable {
        anchors.fill: parent
        anchors.margins: 16
        contentWidth: width
        contentHeight: Math.max(flow.implicitHeight + 20, height)
        clip: true

        Column {
            id: flow
            width: parent.width
            spacing: 12

            Text {
                text: "STORAGE PARTITIONS & MOUNTPOINTS"
                color: "#9ca3af"
                font.family: "JetBrainsMono Nerd Font, monospace"
                font.pixelSize: 11
                font.weight: Font.SemiBold
                font.letterSpacing: 1.0
            }

            Repeater {
                model: tab.disks
                delegate: Rectangle {
                    required property var modelData
                    width: flow.width
                    height: 84
                    radius: 8
                    color: "#13151c"
                    border.color: "#1f222d"
                    border.width: 1

                    Column {
                        anchors.fill: parent
                        anchors.margins: 14
                        spacing: 8

                        Row {
                            width: parent.width
                            Text {
                                text: modelData.mountPoint
                                color: "#ffffff"
                                font.family: "Inter, sans-serif"
                                font.pixelSize: 14
                                font.weight: Font.SemiBold
                            }
                            Text {
                                text: "  (" + modelData.device + " · " + modelData.fsType + ")"
                                color: "#6b7280"
                                font.family: "JetBrainsMono Nerd Font, monospace"
                                font.pixelSize: 11
                                anchors.baseline: parent.children[0].baseline
                            }
                            Item { Layout.fillWidth: true; width: 1; height: 1 }
                            Text {
                                text: tab.app.fmtBytes(modelData.usedBytes) + " / " + tab.app.fmtBytes(modelData.totalBytes) + " (" + Math.round(modelData.usedPercent) + "%)"
                                color: "#e5e7eb"
                                font.family: "JetBrainsMono Nerd Font, monospace"
                                font.pixelSize: 12
                                font.weight: Font.Medium
                            }
                        }

                        Rectangle {
                            width: parent.width
                            height: 8
                            radius: 4
                            color: "#1e212b"
                            Rectangle {
                                width: parent.width * Math.min(1.0, modelData.usedPercent / 100.0)
                                height: parent.height
                                radius: 4
                                color: modelData.usedPercent > 90 ? "#ef4444" : "#ffffff"
                            }
                        }

                        Row {
                            width: parent.width
                            Text {
                                text: "Free: " + tab.app.fmtBytes(modelData.freeBytes)
                                color: "#9ca3af"
                                font.family: "Inter, sans-serif"
                                font.pixelSize: 11
                            }
                        }
                    }
                }
            }

            Text {
                visible: tab.disks.length === 0
                text: "No mounted storage disks detected"
                color: "#6b7280"
                font.family: "Inter, sans-serif"
                font.pixelSize: 12
            }
        }
    }
}
