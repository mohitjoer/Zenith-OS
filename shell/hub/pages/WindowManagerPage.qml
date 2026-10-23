pragma ComponentBehavior: Bound

import QtQuick
import qs.Ui
import qs.Ui.Singletons
import ".."
import "../Singletons"

// Window Manager (COMPOSITOR). Every window-manager setting on one page: the
// neutral window rows the Hub ships (shape, gaps, borders, motion) and the
// exclusives the active provider ships (ProviderSchema). Zenith runs one
// compositor, Hyprland, so there is nothing to switch: the head names it, and its
// own tab states which version is running beside the honest list of what it
// cannot do (the settings the store carries that it has no equivalent for, so a
// hidden row reads as explained rather than as a bug). Every setting rides the
// shared store (hub.hyprVal/hyprEdit) and the shared schema renderer, so a
// capability the compositor lacks drops its rows rather than showing dead knobs.
Item {
    id: pg
    property var hub

    function cap(s) { s = String(s || ""); return s.length ? s.charAt(0).toUpperCase() + s.slice(1) : s; }
    readonly property string providerName: Settings.provider
    readonly property string pTitle: pg.providerName !== "" ? pg.cap(pg.providerName) : I18n.tr("Window Manager")
    readonly property string pEyebrow: I18n.tr("COMPOSITOR")
    readonly property string pBlurb: I18n.tr("How windows look and behave. Changes preview live and are kept when you save.")

    function hv(path) { return pg.hub ? pg.hub.hyprVal(path) : undefined }
    function cv(path) { return pg.hub ? pg.hub.hyprCommittedVal(path) : undefined }

    // Every window-manager setting on one page: the neutral window rows the Hub
    // ships followed by the active provider's exclusives. effectiveRows is the
    // one source the rail gate reads too, so the page and the gate never diverge.
    // ProviderSchema.revision is read so the list rebuilds when the fetch lands.
    readonly property var rows: {
        ProviderSchema.revision;
        return pg.hub ? pg.hub.effectiveRows("windowmanager") : ProviderSchema.rowsFor("windowmanager");
    }

    // per-row visibility for the neutral rows: a dependent stays hidden until its
    // parent toggle is on. Provider rows carry their own `when` gate through the
    // sheet, so their keys are not named here and pass straight through.
    function gateOk(key, d) {
        switch (key) {
        case "desktop.appearance.dimStrength": return d["desktop.appearance.dimInactive"] === true;
        case "desktop.appearance.wobblyWindows": case "desktop.appearance.windowStyle": return d["desktop.appearance.animations"] === true;
        case "desktop.appearance.glowRange": case "desktop.appearance.glowColor": return d["desktop.appearance.glowEnabled"] === true;
        case "desktop.appearance.borderAngleSpeed": return d["desktop.appearance.animatedBorder"] === true;
        case "desktop.appearance.blurContrast": case "desktop.appearance.blurBrightness": case "desktop.appearance.blurSpecial":
        case "desktop.appearance.blurPopups": case "desktop.appearance.blurIgnoreOpacity": case "desktop.appearance.blurNewOptimizations":
        case "desktop.appearance.blurVibrancyDarkness":
            return d["desktop.appearance.blurEnabled"] === true;
        case "desktop.appearance.shadowSharp": case "desktop.appearance.shadowScale": case "desktop.appearance.shadowColor":
        case "desktop.appearance.shadowSpread": case "desktop.appearance.shadowOffsetX": case "desktop.appearance.shadowOffsetY":
            return d["desktop.appearance.shadowEnabled"] === true;
        }
        return true;
    }

    // draft/committed are flat maps off the store (dotted keys), the shape the
    // settings sheet reads. draft depends on hyprVal, so an edit rebuilds it.
    readonly property var draft: {
        var d = {};
        if (pg.hub)
            for (var i = 0; i < pg.rows.length; i++) { var k = pg.rows[i].key; if (k) d[k] = pg.hv(k); }
        return d;
    }
    readonly property var committed: {
        var d = {};
        if (pg.hub)
            for (var i = 0; i < pg.rows.length; i++) { var k = pg.rows[i].key; if (k) d[k] = pg.cv(k); }
        return d;
    }

    // the neutral gate applied; the shared renderer then drops caps and dead-key
    // rows, so the page and its gate read the same set the rail does.
    readonly property var settingsSchema: {
        var d = pg.draft, out = [];
        for (var i = 0; i < pg.rows.length; i++)
            if (pg.gateOk(pg.rows[i].key, d)) out.push(pg.rows[i]);
        return out;
    }

    // the cannot-do lines: the provider's unhonored reasons, deduped (many
    // keybinds collapse to one "no scratchpad" reason) and shown verbatim.
    readonly property var cannotDo: {
        ProviderSchema.revision;
        var seen = {}, out = [];
        var u = ProviderSchema.unhonored || [];
        for (var i = 0; i < u.length; i++) {
            var r = u[i] ? u[i].reason : "";
            if (r && !seen[r]) { seen[r] = true; out.push(r); }
        }
        return out;
    }

    // the rowless tab that carries the compositor itself
    readonly property string aboutTab: I18n.tr("Compositor")

    function focusKey(k) { sp.focusKey(k) }
    SchemaPage {
        id: sp
        anchors.fill: parent
        schema: pg.settingsSchema
        draft: pg.draft
        defaults: pg.committed
        advanced: pg.hub ? pg.hub.advanced : false
        title: pg.pTitle
        eyebrow: pg.pEyebrow
        blurb: pg.pBlurb
        query: pg.hub ? pg.hub.query : ""
        onEdited: (k, v) => { if (pg.hub) pg.hub.hyprEdit(k, v); }
        onPickRequested: (r) => { if (pg.hub) pg.hub.openPick(r); }
        // Which compositor is running and what it cannot do are about the
        // compositor rather than about any group of settings, so they get a tab
        // of their own instead of repeating above every tab.
        tailTabs: [pg.aboutTab]

        Column {
            width: parent.width
            spacing: Tokens.s5
            visible: sp.tab === pg.aboutTab

            CompositorControl {
                width: parent.width
            }

            // A rarely-touched list: folded, and its header says what it holds, so the
            // tab reads as a page rather than a wall of near-identical lines on
            // first sight. It opens in place when asked.
            SettingCard {
                id: cannotSec
                width: parent.width
                visible: pg.cannotDo.length > 0
                collapsible: true
                expanded: false
                title: I18n.tr("WHAT %1 CANNOT DO").arg(pg.pTitle.toUpperCase())
                summary: pg.cannotDo.length + " " + I18n.tr("SETTINGS")

                Column {
                    width: cannotSec.width
                    spacing: Tokens.s2

                    Text {
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4; topPadding: Tokens.s3
                        wrapMode: Text.WordWrap
                        text: I18n.tr("These settings have no equivalent in Hyprland, so they are ignored.")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        lineHeight: 1.3
                    }
                    Repeater {
                        model: pg.cannotDo
                        Row {
                            required property var modelData
                            width: cannotSec.width
                            spacing: Tokens.s2
                            bottomPadding: Tokens.s1
                            Text {
                                text: "\u00b7"
                                leftPadding: Tokens.s4
                                color: Tokens.inkFaint
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fSmall
                            }
                            Text {
                                width: cannotSec.width - Tokens.s6
                                wrapMode: Text.WordWrap
                                text: parent.modelData
                                color: Tokens.inkMuted
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall
                                lineHeight: 1.3
                            }
                        }
                    }
                    Item { width: 1; height: Tokens.s2 }
                }
            }
        }
    }
}
