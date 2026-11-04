pragma ComponentBehavior: Bound

import QtQuick
import "../.." as Pill
import qs.Services
import "../../../../components"
import qs.Ui.Singletons

// One quick-settings toggle tile: clean, borderless surface without nested boxes.
// When active, inverts cleanly to a luminous plate with dark ink.
Item {
    id: root

    property string icon: "circle"
    property string label: ""
    property string sub: ""
    property bool on: false
    property bool hasPage: false
    property string pageTip: ""

    // A tile whose action cannot be taken right now.
    property bool available: true

    signal toggled()
    signal pageRequested()

    implicitHeight: 74

    // Active inverts to Theme.primary, inactive is a soft quiet surface (no harsh box border)
    readonly property color tileBg: root.on
        ? (face.pressed ? Qt.darker(Theme.primary, 1.1) : (face.containsMouse ? Qt.lighter(Theme.primary, 1.05) : Theme.primary))
        : (face.containsMouse ? Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.08)
                              : Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.04))

    readonly property color inkMain: root.on
        ? Theme.inkOn(Theme.primary, Theme.onPrimary)
        : Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)

    readonly property color inkSub: root.on
        ? Qt.rgba(Theme.onPrimary.r, Theme.onPrimary.g, Theme.onPrimary.b, 0.75)
        : Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)

    // Sleek single card surface — no nested boxes
    Rectangle {
        id: cardBg
        anchors.fill: parent
        radius: 12
        color: root.tileBg
        border.width: root.on ? 0 : 1
        border.color: Qt.rgba(Theme.outline.r, Theme.outline.g, Theme.outline.b, face.containsMouse ? 0.20 : 0.08)

        Behavior on color { ColorAnimation { duration: Motion.fast } }
        Behavior on border.color { ColorAnimation { duration: Motion.fast } }
    }

    MouseArea {
        id: face
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: root.available ? Qt.PointingHandCursor : Qt.ArrowCursor
        onClicked: if (root.available) root.toggled()
    }

    Item {
        anchors.fill: parent
        anchors.margins: 10
        opacity: root.available ? 1 : 0.45
        Behavior on opacity { NumberAnimation { duration: Motion.crossfade; easing.type: Motion.crossfadeCurve } }

        // ── Top Row: Direct Icon (no inner box) and Chevron/Status ──
        Item {
            id: topRow
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            height: 24

            MaterialIcon {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                font.pixelSize: 18
                fill: root.on ? 1 : 0
                text: root.icon
                color: root.inkMain
                scale: face.pressed ? 0.88 : 1
                Behavior on scale { NumberAnimation { duration: Motion.fast; easing.type: Easing.OutBack; easing.overshoot: 2.0 } }
                Behavior on color { ColorAnimation { duration: Motion.fast } }
            }

            // Right side: chevron or status dot (no nested box!)
            Row {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 6

                // Detail-page chevron
                MaterialIcon {
                    visible: root.hasPage
                    anchors.verticalCenter: parent.verticalCenter
                    font.pixelSize: 15
                    text: "chevron_right"
                    color: root.inkSub
                    opacity: pageTap.containsMouse ? 1 : 0.6

                    MouseArea {
                        id: pageTap
                        anchors.fill: parent
                        anchors.margins: -4
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.pageRequested()
                    }
                    QsTip {
                        text: root.pageTip
                        align: "right"
                        hovered: pageTap.containsMouse && !pageTap.pressed
                    }
                }
            }
        }

        // ── Bottom Area: Clean typography with full width (no truncation) ──
        Column {
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            spacing: 2

            Text {
                width: parent.width
                elide: Text.ElideRight
                text: I18n.tr(root.label)
                color: root.inkMain
                font.family: Theme.fontPrimary
                font.pixelSize: Theme.fontSm
                font.weight: Font.DemiBold
                Behavior on color { ColorAnimation { duration: Motion.fast } }
            }

            Text {
                width: parent.width
                elide: Text.ElideRight
                visible: text.length > 0
                text: root.sub
                color: root.inkSub
                font.family: Theme.fontPrimary
                font.pixelSize: Theme.fontSm - 2
                font.weight: Font.Normal
                Behavior on color { ColorAnimation { duration: Motion.fast } }
            }
        }
    }
}
