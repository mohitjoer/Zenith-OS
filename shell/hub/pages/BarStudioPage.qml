pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import qs.Ui
import qs.Ui.Singletons
import qs.Services as Services
import ".."
import "../schema/BarStudioPage.js" as BarStudioSchema
import "../Singletons"

Item {
    id: pg
    property var hub

    readonly property string pTitle: I18n.tr("Bar Studio")
    readonly property string pEyebrow: I18n.tr("BAR & DOCK")
    readonly property string pBlurb: I18n.tr("Configure Control Bar layout, surface, widgets, gaps and the desktop dock.")
    function focusKey(k) { sp.focusKey(k); }

    component IconText: Text {
        renderType: Text.QtRendering
        font.family: "Material Symbols Rounded"
    }

    // ── Layout lanes state ───────────────────────────────────────────────────
    readonly property var currentLayout: {
        Settings.revision;
        var l = (pg.hub && pg.hub.val ? pg.hub.val("qsbar.layout") : null) || Settings.get("qsbar.layout");
        if (l && l.left !== undefined) return l;
        return {
            version: 1,
            left: ["launcher", "workspaces", "status", "cpu", "volume", "memory", "ai"],
            center: ["clock"],
            right: ["media", "quick", "network", "power", "battery", "brightness", "cputemp", "storage", "gpu", "bluetooth", "layout"]
        };
    }

    function moveWidget(widgetId, targetLane, targetIndex) {
        var cur = pg.currentLayout;
        var next = {
            version: 1,
            left: cur.left ? cur.left.slice() : [],
            center: cur.center ? cur.center.slice() : [],
            right: cur.right ? cur.right.slice() : []
        };
        next.left = next.left.filter(x => x !== widgetId);
        next.center = next.center.filter(x => x !== widgetId);
        next.right = next.right.filter(x => x !== widgetId);
        if (!next[targetLane]) next[targetLane] = [];
        if (targetIndex !== undefined && targetIndex >= 0 && targetIndex < next[targetLane].length) {
            next[targetLane].splice(targetIndex, 0, widgetId);
        } else {
            next[targetLane].push(widgetId);
        }
        if (pg.hub) pg.hub.stageLive("qsbar.layout", next);
        else Settings.patch("qsbar.layout", next);
    }

    function resetLayout() {
        var def = {
            version: 1,
            left: ["launcher", "workspaces", "status", "cpu", "volume", "memory", "ai"],
            center: ["clock"],
            right: ["media", "quick", "network", "power", "battery", "brightness", "cputemp", "storage", "gpu", "bluetooth", "layout"]
        };
        if (pg.hub) pg.hub.stageLive("qsbar.layout", def);
        else Settings.patch("qsbar.layout", def);
    }

    // ── Pinned apps helpers ──────────────────────────────────────────────────
    function removePin(cls) {
        var cur = Services.Dock.pinnedOrStarter();
        var next = [];
        for (var i = 0; i < cur.length; i++) {
            if (cur[i] !== cls) next.push(cur[i]);
        }
        Services.Dock.setPinned(next);
    }

    function addPin(cls) {
        var cur = Services.Dock.pinnedOrStarter();
        if (cur.indexOf(cls) < 0) {
            Services.Dock.setPinned(cur.concat([cls]));
        }
    }

    property bool appPickerOpen: false
    function availableApps() {
        var src = DesktopEntries.applications ? DesktopEntries.applications.values : [];
        var pinned = Services.Dock.pinnedOrStarter();
        var out = [];
        for (var i = 0; i < src.length; i++) {
            var e = src[i];
            if (!e || e.noDisplay) continue;
            var cls = e.id ? String(e.id) : String(e.name || "");
            if (cls === "" || pinned.indexOf(cls) >= 0) continue;
            out.push({ name: e.name || cls, id: cls, icon: Services.Dock.iconFor(cls) });
        }
        out.sort(function (a, b) { return String(a.name).localeCompare(String(b.name)); });
        return out;
    }

    SchemaPage {
        id: sp
        anchors.fill: parent
        schema: BarStudioSchema.rows
        draft: pg.hub ? pg.hub.draft : null
        defaults: pg.hub ? pg.hub.committed : ({})
        advanced: pg.hub ? pg.hub.advanced : false
        title: pg.pTitle
        eyebrow: pg.pEyebrow
        blurb: pg.pBlurb
        query: pg.hub ? pg.hub.query : ""
        tailTabs: ["Layout"]
        onEdited: (k, v) => {
            if (!pg.hub) return;
            pg.hub.stageLive(k, v);
        }
        onPickRequested: (r) => { if (pg.hub) pg.hub.openPick(r); }

        // ── LAYOUT TAB: Interactive Lanes ────────────────────────────────────
        SettingCard {
            id: layoutCard
            anchors.left: parent.left
            width: sp.cardWidth
            title: I18n.tr("BAR LANES")
            visible: sp.tab === "Layout"

            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                label: I18n.tr("Reset layout")
                desc: I18n.tr("Restore shipped arrangement across Left, Center, and Right zones.")
                controlWidth: 100
                Btn {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("RESET")
                    onAct: pg.resetLayout()
                }
            }

            // Left Lane
            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                divider: true
                block: true
                label: I18n.tr("Left Lane")
                desc: I18n.tr("Widgets anchored to the start of the bar.")

                Flow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    spacing: Tokens.s2
                    Repeater {
                        model: pg.currentLayout.left || []
                        delegate: Rectangle {
                            id: lChip
                            required property string modelData
                            required property int index
                            height: Tokens.ctlH
                            implicitWidth: lRow.implicitWidth + Tokens.s3 * 2
                            radius: Tokens.radius
                            color: Tokens.tint5
                            border.width: Tokens.border
                            border.color: Tokens.line
                            Row {
                                id: lRow
                                anchors.centerIn: parent
                                spacing: Tokens.s2
                                Text {
                                    text: lChip.modelData
                                    color: Tokens.inkDim
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fSmall
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                IconText {
                                    text: "arrow_forward"
                                    color: Tokens.inkFaint
                                    font.pixelSize: 12
                                    anchors.verticalCenter: parent.verticalCenter
                                    MouseArea {
                                        anchors.fill: parent
                                        hoverEnabled: true
                                        cursorShape: Qt.PointingHandCursor
                                        onClicked: pg.moveWidget(lChip.modelData, "center", 0)
                                    }
                                }
                            }
                        }
                    }
                }
            }

            // Center Lane
            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                divider: true
                block: true
                label: I18n.tr("Center Lane")
                desc: I18n.tr("Widgets centered on the bar.")

                Flow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    spacing: Tokens.s2
                    Repeater {
                        model: pg.currentLayout.center || []
                        delegate: Rectangle {
                            id: cChip
                            required property string modelData
                            required property int index
                            height: Tokens.ctlH
                            implicitWidth: cRow.implicitWidth + Tokens.s3 * 2
                            radius: Tokens.radius
                            color: Tokens.tint5
                            border.width: Tokens.border
                            border.color: Tokens.line
                            Row {
                                id: cRow
                                anchors.centerIn: parent
                                spacing: Tokens.s2
                                IconText {
                                    text: "arrow_back"
                                    color: Tokens.inkFaint
                                    font.pixelSize: 12
                                    anchors.verticalCenter: parent.verticalCenter
                                    MouseArea {
                                        anchors.fill: parent
                                        hoverEnabled: true
                                        cursorShape: Qt.PointingHandCursor
                                        onClicked: pg.moveWidget(cChip.modelData, "left", -1)
                                    }
                                }
                                Text {
                                    text: cChip.modelData
                                    color: Tokens.inkDim
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fSmall
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                IconText {
                                    text: "arrow_forward"
                                    color: Tokens.inkFaint
                                    font.pixelSize: 12
                                    anchors.verticalCenter: parent.verticalCenter
                                    MouseArea {
                                        anchors.fill: parent
                                        hoverEnabled: true
                                        cursorShape: Qt.PointingHandCursor
                                        onClicked: pg.moveWidget(cChip.modelData, "right", 0)
                                    }
                                }
                            }
                        }
                    }
                }
            }

            // Right Lane
            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                divider: true
                block: true
                label: I18n.tr("Right Lane")
                desc: I18n.tr("Widgets anchored to the end of the bar.")

                Flow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    spacing: Tokens.s2
                    Repeater {
                        model: pg.currentLayout.right || []
                        delegate: Rectangle {
                            id: rChip
                            required property string modelData
                            required property int index
                            height: Tokens.ctlH
                            implicitWidth: rRow.implicitWidth + Tokens.s3 * 2
                            radius: Tokens.radius
                            color: Tokens.tint5
                            border.width: Tokens.border
                            border.color: Tokens.line
                            Row {
                                id: rRow
                                anchors.centerIn: parent
                                spacing: Tokens.s2
                                IconText {
                                    text: "arrow_back"
                                    color: Tokens.inkFaint
                                    font.pixelSize: 12
                                    anchors.verticalCenter: parent.verticalCenter
                                    MouseArea {
                                        anchors.fill: parent
                                        hoverEnabled: true
                                        cursorShape: Qt.PointingHandCursor
                                        onClicked: pg.moveWidget(rChip.modelData, "center", -1)
                                    }
                                }
                                Text {
                                    text: rChip.modelData
                                    color: Tokens.inkDim
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fSmall
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                            }
                        }
                    }
                }
            }
        }

        // ── DOCK TAB: Pinned Applications ────────────────────────────────────
        SettingCard {
            id: pinnedCard
            anchors.left: parent.left
            width: sp.cardWidth
            title: I18n.tr("PINNED APPS")
            visible: sp.tab === "Dock"

            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                block: true
                label: I18n.tr("Dock items")
                desc: I18n.tr("Applications always kept on the dock band.")

                Column {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    spacing: Tokens.s3

                    Flow {
                        width: parent.width
                        spacing: Tokens.s2
                        Repeater {
                            model: Services.Dock.pinnedOrStarter()
                            delegate: Rectangle {
                                id: pChip
                                required property string modelData
                                readonly property string iconSrc: Services.Dock.iconFor(pChip.modelData)
                                readonly property string appName: {
                                    var e = DesktopEntries.heuristicLookup(pChip.modelData);
                                    return (e && e.name) ? e.name : pChip.modelData;
                                }
                                height: Tokens.ctlH + 4
                                implicitWidth: pChipRow.implicitWidth + Tokens.s3 * 2
                                radius: Tokens.radius
                                color: pChipMa.containsMouse ? Tokens.tint10 : Tokens.tint5
                                border.width: Tokens.border
                                border.color: pChipMa.containsMouse ? Tokens.lineStrong : Tokens.line

                                Row {
                                    id: pChipRow
                                    anchors.centerIn: parent
                                    spacing: Tokens.s2
                                    Image {
                                        width: 16
                                        height: 16
                                        source: pChip.iconSrc
                                        visible: pChip.iconSrc !== ""
                                        fillMode: Image.PreserveAspectFit
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                    Text {
                                        text: pChip.appName
                                        color: Tokens.inkDim
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fSmall
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                    IconText {
                                        text: "close"
                                        color: pChipMa.containsMouse ? Tokens.ink : Tokens.inkFaint
                                        font.pixelSize: 12
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                }
                                MouseArea {
                                    id: pChipMa
                                    anchors.fill: parent
                                    hoverEnabled: true
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: pg.removePin(pChip.modelData)
                                }
                            }
                        }
                    }

                    Row {
                        spacing: Tokens.s2
                        Btn {
                            text: pg.appPickerOpen ? I18n.tr("CLOSE LIST") : I18n.tr("ADD PINNED APP")
                            onAct: pg.appPickerOpen = !pg.appPickerOpen
                        }
                    }

                    // Expandable list of installed apps to pin
                    Column {
                        width: parent.width
                        visible: pg.appPickerOpen
                        spacing: Tokens.s1

                        Text {
                            text: I18n.tr("Click any application to pin it:")
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                        }

                        Flow {
                            width: parent.width
                            spacing: Tokens.s2
                            Repeater {
                                model: pg.availableApps()
                                delegate: Rectangle {
                                    id: addChip
                                    required property var modelData
                                    height: Tokens.ctlH
                                    implicitWidth: addRow.implicitWidth + Tokens.s3 * 2
                                    radius: Tokens.radius
                                    color: addMa.containsMouse ? Tokens.tint10 : Tokens.tint5
                                    border.width: Tokens.border
                                    border.color: addMa.containsMouse ? Tokens.lineStrong : Tokens.line

                                    Row {
                                        id: addRow
                                        anchors.centerIn: parent
                                        spacing: Tokens.s2
                                        Image {
                                            width: 14
                                            height: 14
                                            source: addChip.modelData.icon || ""
                                            fillMode: Image.PreserveAspectFit
                                            visible: !!addChip.modelData.icon
                                            anchors.verticalCenter: parent.verticalCenter
                                        }
                                        Text {
                                            text: addChip.modelData.name
                                            color: Tokens.inkDim
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fSmall
                                            anchors.verticalCenter: parent.verticalCenter
                                        }
                                    }
                                    MouseArea {
                                        id: addMa
                                        anchors.fill: parent
                                        hoverEnabled: true
                                        cursorShape: Qt.PointingHandCursor
                                        onClicked: {
                                            pg.addPin(addChip.modelData.id);
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}