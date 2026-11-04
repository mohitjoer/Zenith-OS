pragma ComponentBehavior: Bound
import QtQuick
import "../Singletons"
import "lib/clock.js" as Clk
import qs.Services as Svc

// Compact modern date card: a sleek monochrome pill with two-stop elevation
// (subtle background fill + crisp 1px contact border).
// Displays the day-of-week in tracked uppercase alongside the month and date.
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

    Rectangle {
        id: pill
        implicitWidth: content.implicitWidth + Math.round(24 * date.s)
        implicitHeight: content.implicitHeight + Math.round(12 * date.s)
        radius: Math.round(10 * date.s)
        color: Qt.rgba(date.ink.r, date.ink.g, date.ink.b, 0.08)
        border.width: Math.max(1, Math.round(date.s))
        border.color: Qt.rgba(date.ink.r, date.ink.g, date.ink.b, 0.16)

        Row {
            id: content
            anchors.centerIn: parent
            spacing: Math.round(10 * date.s)

            // Day of week
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: date.dp.weekdayShort.toUpperCase()
                color: date.accent
                font.family: Theme.font
                font.pixelSize: Math.round(13 * date.s)
                font.weight: Font.DemiBold
                font.letterSpacing: Math.round(1.5 * date.s)
            }

            // Divider dot
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: "\u00b7"
                color: date.inkDim
                font.family: Theme.font
                font.pixelSize: Math.round(14 * date.s)
                font.weight: Font.Bold
            }

            // Month and Day
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: (date.dp.monthShort || date.dp.month).toUpperCase() + " " + date.dp.dom
                color: date.ink
                font.family: Theme.font
                font.pixelSize: Math.round(13 * date.s)
                font.weight: Font.Medium
                font.letterSpacing: Math.round(0.5 * date.s)
            }
        }
    }
}
