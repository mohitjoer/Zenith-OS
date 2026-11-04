pragma ComponentBehavior: Bound
import QtQuick
import "../Singletons"
import "lib/clock.js" as Clk

// Zen clock face: an ultra-clean, modern architectural layout.
// Emphasizes refined typography, strict monochrome hierarchy, and zero-overhead
// rendering. Digits are rendered with precise kerning; hours are set in a clean
// light weight and minutes in demibold, separated by a slender breath mark.
Item {
    id: face

    property real underL: Scheme.wallLstar
    property string inkColorA: ""
    readonly property color ink:     Theme.inkOn2(face.underL, face.inkColorA)
    readonly property color inkDim:  Theme.inkDimOn2(face.underL, face.inkColorA)
    readonly property color inkSoft: Theme.inkSoftOn2(face.underL, face.inkColorA)

    readonly property var t: Clk.parts(Now.date, Config.clock24h)
    readonly property color accent: Clk.pickAccent(Config.clockAccent, Theme.accentOn2(face.underL, face.inkColorA), Theme.brand, face.ink)
    readonly property real px: Math.round(84 * Config.clockScale)
    readonly property bool hasCaption: Config.clockSeconds || !Config.clock24h

    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    Row {
        id: row
        spacing: Math.round(12 * Config.clockScale)
        anchors.verticalCenter: parent ? parent.verticalCenter : undefined

        // Hours
        Text {
            text: face.t.hh
            color: face.ink
            font.family: Theme.font
            font.pixelSize: face.px
            font.weight: Font.Light
            font.letterSpacing: Math.round(-1 * Config.clockScale)
        }

        // Slender Zen separator
        Item {
            width: Math.max(2, Math.round(4 * Config.clockScale))
            height: face.px * 0.72
            anchors.verticalCenter: parent.verticalCenter

            Rectangle {
                anchors.centerIn: parent
                width: Math.max(1, Math.round(2 * Config.clockScale))
                height: parent.height
                radius: width / 2
                color: face.inkDim
                opacity: 0.45
            }
        }

        // Minutes
        Text {
            text: face.t.mm
            color: face.ink
            font.family: Theme.font
            font.pixelSize: face.px
            font.weight: Font.DemiBold
            font.letterSpacing: Math.round(-1 * Config.clockScale)
        }

        // Optional seconds or AM/PM pill
        Column {
            visible: face.hasCaption
            anchors.verticalCenter: parent.verticalCenter
            spacing: Math.round(3 * Config.clockScale)

            Text {
                visible: Config.clockSeconds
                text: face.t.ss
                color: face.accent
                font.family: Theme.mono
                font.pixelSize: Math.round(face.px * 0.28)
                font.weight: Font.Medium
            }

            Text {
                visible: !Config.clock24h
                text: face.t.ampm.toUpperCase()
                color: face.inkDim
                font.family: Theme.font
                font.pixelSize: Math.round(face.px * 0.22)
                font.weight: Font.DemiBold
                font.letterSpacing: Math.round(1.5 * Config.clockScale)
            }
        }
    }
}
