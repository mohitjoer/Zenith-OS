pragma ComponentBehavior: Bound
import QtQuick
import "Singletons"
import qs.Ui.Singletons

/**
 * A live, plain-QML preview of the desktop clock widget for the Desktop Widgets
 * section, so the chosen face, date design, format and accent show at a glance
 * without leaning over the hub window to the wallpaper. It mirrors the live faces
 * in zenith/shell/quickshell/shell/modules/desktop/clock; the accent follows your real
 * palette (Scheme singleton), the rest is bright ink as on the wallpaper.
 */
Item {
    id: preview

    property string design: "zen"
    property bool is24: true
    property bool seconds: false
    property string accentChoice: "palette"
    property bool dateShow: true
    property string dateDesign: "hud"

    readonly property color ink: "#f5f3ff"
    readonly property color inkSoft: "#d2d7ef"
    readonly property color inkDim: "#9aa3c8"
    readonly property color accent: preview.accentChoice === "brand" ? "#F25623"
        : preview.accentChoice === "mono" ? preview.ink : Scheme.accent

    property var now: new Date()
    Timer { interval: 1000; running: true; repeat: true; triggeredOnStart: true; onTriggered: preview.now = new Date() }

    function pad2(n) { return (n < 10 ? "0" : "") + n; }
    readonly property int h: now.getHours()
    readonly property int mins: now.getMinutes()
    readonly property int secs: now.getSeconds()
    readonly property int h12: (h % 12) === 0 ? 12 : (h % 12)
    readonly property string hh: preview.is24 ? pad2(h) : String(h12)
    readonly property string mm: pad2(mins)
    readonly property string ss: pad2(secs)
    readonly property string ampm: h < 12 ? I18n.tr("AM") : I18n.tr("PM")

    readonly property var weekdays: ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"]
    readonly property var weekdaysShort: ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]
    readonly property var months: ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"]
    readonly property int dow: now.getDay()
    readonly property int dom: now.getDate()
    readonly property int monIdx: now.getMonth()

    // report the content's real size: the host card scales against this, and a
    // face plus a date line is taller than the registry's bare-face guess.
    implicitWidth: contentCol.implicitWidth
    implicitHeight: contentCol.implicitHeight

    Column {
        id: contentCol
        anchors.centerIn: parent
        spacing: 14

        Loader {
            anchors.horizontalCenter: parent.horizontalCenter
            sourceComponent: preview.faceFor()
        }
        Loader {
            anchors.horizontalCenter: parent.horizontalCenter
            active: preview.dateShow
            visible: preview.dateShow
            sourceComponent: preview.dateShow ? preview.dateFor() : null
        }
    }

    function faceFor() {
        switch (preview.design) {
        case "zen":     return zenC;
        case "minimal": return minimalC;
        default:        return digitalC;
        }
    }
    function dateFor() {
        switch (preview.dateDesign) {
        case "card":    return cardC;
        case "hud":     return hudC;
        default:        return hudC;
        }
    }

    // --- faces -------------------------------------------------------------
    Component {
        id: zenC
        Row {
            spacing: 10
            Text { text: preview.hh; color: preview.ink; font.family: "Inter"; font.pixelSize: 56; font.weight: Font.Light; font.letterSpacing: -1 }
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: 2; height: 38; radius: 1
                color: preview.inkDim; opacity: 0.45
            }
            Text { text: preview.mm; color: preview.ink; font.family: "Inter"; font.pixelSize: 56; font.weight: Font.DemiBold; font.letterSpacing: -1 }
            Column {
                visible: preview.seconds || !preview.is24
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2
                Text { visible: preview.seconds; text: preview.ss; color: preview.accent; font.family: "JetBrainsMono Nerd Font"; font.pixelSize: 16; font.weight: Font.Medium }
                Text { visible: !preview.is24; text: preview.ampm; color: preview.inkDim; font.family: "Inter"; font.pixelSize: 13; font.weight: Font.DemiBold; font.letterSpacing: 1 }
            }
        }
    }

    Component {
        id: digitalC
        Row {
            spacing: preview.seconds || !preview.is24 ? 10 : 0
            Row {
                spacing: 0
                Text { text: preview.hh; color: preview.ink; font.family: "JetBrainsMono Nerd Font"; font.pixelSize: 56; font.weight: Font.Bold }
                Text {
                    text: ":"; color: preview.accent; font.family: "JetBrainsMono Nerd Font"; font.pixelSize: 56; font.weight: Font.Bold
                    SequentialAnimation on opacity { loops: Animation.Infinite
                        NumberAnimation { from: 1; to: 0.3; duration: 620; easing.type: Easing.InOutSine }
                        NumberAnimation { from: 0.3; to: 1; duration: 620; easing.type: Easing.InOutSine } }
                }
                Text { text: preview.mm; color: preview.ink; font.family: "JetBrainsMono Nerd Font"; font.pixelSize: 56; font.weight: Font.Bold }
            }
            Column {
                anchors.verticalCenter: parent.verticalCenter
                spacing: 3
                Text { visible: preview.seconds; text: preview.ss; color: preview.accent; font.family: "JetBrainsMono Nerd Font"; font.pixelSize: 17; font.weight: Font.DemiBold }
                Text { visible: !preview.is24; text: preview.ampm; color: preview.inkDim; font.family: "JetBrainsMono Nerd Font"; font.pixelSize: 14; font.weight: Font.DemiBold }
            }
        }
    }

    Component {
        id: minimalC
        Column {
            spacing: 8
            Text { id: mt; text: preview.hh + ":" + preview.mm; color: preview.ink; font.family: "Inter"; font.pixelSize: 54; font.weight: Font.Light; font.letterSpacing: 2 }
            Rectangle { width: mt.implicitWidth * 0.34; height: 3; radius: Theme.radius; color: preview.accent }
            Text {
                visible: preview.seconds || !preview.is24
                text: (preview.seconds ? preview.ss : "") + (preview.seconds && !preview.is24 ? "  " : "") + (!preview.is24 ? preview.ampm.toLowerCase() : "")
                color: preview.inkDim; font.family: "Inter"; font.pixelSize: 13; font.weight: Font.Medium; font.letterSpacing: 3
            }
        }
    }

    // --- date designs ------------------------------------------------------
    Component {
        id: cardC
        Rectangle {
            implicitWidth: cr.implicitWidth + 20; implicitHeight: cr.implicitHeight + 10; radius: 8
            color: Qt.rgba(preview.ink.r, preview.ink.g, preview.ink.b, 0.08)
            border.width: 1; border.color: Qt.rgba(preview.ink.r, preview.ink.g, preview.ink.b, 0.16)
            Row {
                id: cr
                anchors.centerIn: parent; spacing: 8
                Text { anchors.verticalCenter: parent.verticalCenter; text: preview.weekdaysShort[preview.dow].toUpperCase(); color: preview.accent; font.family: "Inter"; font.pixelSize: 13; font.weight: Font.DemiBold; font.letterSpacing: 1 }
                Text { anchors.verticalCenter: parent.verticalCenter; text: "\u00b7"; color: preview.inkDim; font.family: "Inter"; font.pixelSize: 13; font.weight: Font.Bold }
                Text { anchors.verticalCenter: parent.verticalCenter; text: preview.months[preview.monIdx].slice(0, 3).toUpperCase() + " " + preview.dom; color: preview.ink; font.family: "Inter"; font.pixelSize: 13; font.weight: Font.Medium }
            }
        }
    }
    Component {
        id: hudC
        Rectangle {
            implicitWidth: hr.implicitWidth + 20; implicitHeight: hr.implicitHeight + 10; radius: 8
            color: Qt.rgba(preview.ink.r, preview.ink.g, preview.ink.b, 0.08)
            border.width: 1; border.color: Qt.rgba(preview.ink.r, preview.ink.g, preview.ink.b, 0.16)
            Row {
                id: hr
                anchors.centerIn: parent; spacing: 8
                Text { anchors.verticalCenter: parent.verticalCenter; text: preview.weekdaysShort[preview.dow].toUpperCase() + " " + preview.dom; color: preview.accent; font.family: "Inter"; font.pixelSize: 12; font.weight: Font.DemiBold }
                Rectangle { width: 1; height: 12; anchors.verticalCenter: parent.verticalCenter; color: preview.inkDim; opacity: 0.35 }
                Text { anchors.verticalCenter: parent.verticalCenter; text: "94%"; color: preview.ink; font.family: "JetBrainsMono Nerd Font"; font.pixelSize: 12; font.weight: Font.Medium }
                Rectangle { width: 1; height: 12; anchors.verticalCenter: parent.verticalCenter; color: preview.inkDim; opacity: 0.35 }
                Text { anchors.verticalCenter: parent.verticalCenter; text: "12% CPU \u00b7 4.2G"; color: preview.inkSoft; font.family: "JetBrainsMono Nerd Font"; font.pixelSize: 12; font.weight: Font.Normal }
            }
        }
    }
}
