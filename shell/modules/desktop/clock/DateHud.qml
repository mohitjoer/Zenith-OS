pragma ComponentBehavior: Bound
import QtQuick
import "../Singletons"
import "lib/clock.js" as Clk
import qs.Services as Svc

// System & Battery HUD companion widget:
// Combines date, power status, and lightweight hardware vitals into an
// energy-efficient, monochrome HUD bar.
// Claims Sysinfo polling only while visible and active, ensuring zero background cost.
Item {
    id: date

    property real underL: Scheme.wallLstar
    property string inkColorA: ""
    readonly property color ink:     Theme.inkOn2(date.underL, date.inkColorA)
    readonly property color inkDim:  Theme.inkDimOn2(date.underL, date.inkColorA)
    readonly property color inkSoft: Theme.inkSoftOn2(date.underL, date.inkColorA)

    readonly property var dp: Clk.dateParts(Now.date, Svc.Config.formatLoc)
    readonly property color accent: Clk.pickAccent(Config.clockAccent, Theme.accentOn2(date.underL, date.inkColorA), Theme.brand, date.ink)
    readonly property real s: Config.clockScale

    implicitWidth: pill.implicitWidth
    implicitHeight: pill.implicitHeight

    // Active lifecycle management for Sysinfo poll:
    readonly property bool wanted: date.visible && date.opacity > 0
    onWantedChanged: Svc.Sysinfo.setActive(date, date.wanted)
    Component.onCompleted: Svc.Sysinfo.setActive(date, date.wanted)
    Component.onDestruction: Svc.Sysinfo.setActive(date, false)

    Rectangle {
        id: pill
        implicitWidth: row.implicitWidth + Math.round(24 * date.s)
        implicitHeight: row.implicitHeight + Math.round(12 * date.s)
        radius: Math.round(10 * date.s)
        color: Qt.rgba(date.ink.r, date.ink.g, date.ink.b, 0.08)
        border.width: Math.max(1, Math.round(date.s))
        border.color: Qt.rgba(date.ink.r, date.ink.g, date.ink.b, 0.16)

        Row {
            id: row
            anchors.centerIn: parent
            spacing: Math.round(10 * date.s)

            // Date section
            Row {
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(6 * date.s)

                Text {
                    text: date.dp.weekdayShort.toUpperCase()
                    color: date.accent
                    font.family: Theme.font
                    font.pixelSize: Math.round(12 * date.s)
                    font.weight: Font.DemiBold
                    font.letterSpacing: Math.round(1 * date.s)
                }

                Text {
                    text: date.dp.dom + " " + (date.dp.monthShort || date.dp.month).toUpperCase()
                    color: date.ink
                    font.family: Theme.font
                    font.pixelSize: Math.round(12 * date.s)
                    font.weight: Font.Medium
                }
            }

            // Divider
            Rectangle {
                width: 1
                height: Math.round(14 * date.s)
                anchors.verticalCenter: parent.verticalCenter
                color: date.inkDim
                opacity: 0.35
            }

            // Battery section
            Row {
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(4 * date.s)

                Text {
                    text: Svc.Battery.present
                        ? (Svc.Battery.charging ? "\u26a1 " : "") + Svc.Battery.pct + "%"
                        : "AC"
                    color: Svc.Battery.low ? date.accent : date.ink
                    font.family: Theme.mono
                    font.pixelSize: Math.round(12 * date.s)
                    font.weight: Font.Medium
                }
            }

            // Divider
            Rectangle {
                width: 1
                height: Math.round(14 * date.s)
                anchors.verticalCenter: parent.verticalCenter
                color: date.inkDim
                opacity: 0.35
            }

            // System Load section (CPU & RAM)
            Row {
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(6 * date.s)

                Text {
                    text: Math.round(Svc.Sysinfo.cpu * 100) + "% CPU"
                    color: date.inkSoft
                    font.family: Theme.mono
                    font.pixelSize: Math.round(12 * date.s)
                    font.weight: Font.Normal
                }

                Text {
                    text: "\u00b7"
                    color: date.inkDim
                    font.family: Theme.font
                    font.pixelSize: Math.round(12 * date.s)
                    font.weight: Font.Bold
                }

                Text {
                    text: Svc.Sysinfo.memUsedGiB.toFixed(1) + "G RAM"
                    color: date.inkSoft
                    font.family: Theme.mono
                    font.pixelSize: Math.round(12 * date.s)
                    font.weight: Font.Normal
                }
            }
        }
    }
}
