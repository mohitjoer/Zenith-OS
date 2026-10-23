pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import qs.Ui
import qs.Ui.Singletons
import qs.FrameBars
import qs.Services as Services
import "../Singletons"
import "../barstudio"
import "../barstudio/BarStudioModel.js" as Model

// Bar Studio (DESKTOP). Choose which bar the desktop draws, tune the built-in
// styles, and turn the app dock on. QS Bar is a folder style that keeps its own
// layout, widgets and form in QS Bar Settings (the bar logo opens it, or the OPEN
// QS BAR SETTINGS button here); this page shows only a live summary of its
// order. Kairos keeps its island settings behind the gear on its own clock. Sumi
// is edited in place: the frame's draw toggle and opacity, each rail's on/off and
// thickness, and the widgets in its three zones (add via a per-zone drawer,
// remove, reorder). Obi and Nacre are store styles and carry their own small
// editors when they are installed. The dock belongs to no style, so its card is
// always here.
//
// Everything stages through the shared draft (hub.stageLive), which applies to
// the RUNNING desktop as you work and rides the Hub's Save and Revert like every
// other framed page. Every key here is one the shell already reads from
// shell.json (Services/Config.qml, Services/Dock.qml); the daemon is its only
// writer, so nothing in this file opens that file for writing.
Item {
    id: page
    property var hub

    // which rail is on the bench
    property string edge: "left"

    // Always normalize what the editor reads. A stored value that lost a subtree
    // (a legacy config, a hand edit, a partial write) would otherwise leave the
    // editor reading undefined and a rail edit cloning the gap straight back to
    // disk. Normalizing restores every subtree from the schema default, so the
    // editor is always whole and the first staged edit heals the store. The
    // probe harness's bare hub has no val(); normalize(null) still yields a
    // complete default, so the page always loads.
    readonly property var config: {
        const v = page.hub && page.hub.val ? page.hub.val("frameBars") : null;
        return FrameBars.normalize(v, BarCatalog, MenuCatalog);
    }
    // the on-disk config, normalized to the same shape so the changed marks and
    // struck defaults compare like against like.
    readonly property var committedBars: {
        const c = page.hub && page.hub.committed ? page.hub.committed.frameBars : null;
        return c ? FrameBars.normalize(c, BarCatalog, MenuCatalog) : null;
    }

    // the selected rail and its on-disk twin, for the rail cells' changed marks
    readonly property var rail: page.config.rails[page.edge]
    readonly property var railWas: page.committedBars && page.committedBars.rails ? page.committedBars.rails[page.edge] : null
    readonly property bool horizontal: page.edge === "top" || page.edge === "bottom"

    // ── the style shelf ──────────────────────────────────────────────────────
    // Three styles ship in the shell tree and need nothing installed: QS Bar and
    // Kairos are folder scenes (Services/BarProducts.qml), Sumi is the frame the
    // shell paints itself. They are listed here directly, so the shelf is whole
    // on a machine with no store. Anything else is a store style that
    // BarProducts has already indexed from the store's own state directory, plus
    // whatever the store reports once the user asks it (CHECK FOR UPDATES): the
    // store is never asked on its own, because asking can reach the network.
    readonly property var builtinStyles: [
        { id: "qsbar", name: "QS Bar", desc: I18n.tr("Top bar with its own widgets, dock and settings") },
        { id: "kairos", name: "Kairos", desc: I18n.tr("One island at the top that carries the clock") },
        { id: "sumi", name: "Sumi", desc: I18n.tr("Painted frame with a rail down the left edge") }
    ]
    property var catalogStyles: []
    property bool storePresent: false
    property bool checking: false
    property string updatingId: ""    // the style whose update is in flight
    property string note: ""          // the last thing that went wrong, in words

    function titleOf(id) {
        return String(id).split("-").map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(" ");
    }

    readonly property var shelf: {
        const out = [];
        const seen = ({});
        for (const b of page.builtinStyles) {
            out.push({
                id: b.id, name: b.name, desc: b.desc, builtin: true, installed: true,
                version: "", installedVersion: "", updateAvailable: false,
                unavailable: false, unavailableReason: "", requiredWindowManager: "",
                downloadPaused: false, downloadPauseReason: ""
            });
            seen[b.id] = true;
        }
        const local = ({});
        for (const row of Services.BarProducts.rows)
            local[row.id] = row;
        for (const item of page.catalogStyles) {
            if (seen[item.id])
                continue;
            seen[item.id] = true;
            out.push(Object.assign({ builtin: false }, item, { installed: item.installed || !!local[item.id] }));
        }
        for (const id in local) {
            if (seen[id])
                continue;
            out.push({
                id: id, name: page.titleOf(id), desc: "", builtin: false, installed: true,
                version: String(local[id].version || ""), installedVersion: String(local[id].version || ""),
                updateAvailable: false, unavailable: false, unavailableReason: "",
                requiredWindowManager: "", downloadPaused: false, downloadPauseReason: ""
            });
        }
        return out;
    }

    // The running bar style. Config.qml's default is QS Bar, so a stored value
    // that is absent reads as that, never as the frame style.
    readonly property string activeStyle: page.fval("barStyle", "qsbar")
    readonly property bool sumiActive: page.activeStyle === "sumi"
    readonly property string activeName: {
        for (let i = 0; i < page.shelf.length; i++)
            if (page.shelf[i].id === page.activeStyle) return page.shelf[i].name;
        return page.titleOf(page.activeStyle);
    }

    // The store is an optional companion: capability-detect it once and hide
    // every control that needs it when it is not there.
    Process {
        id: storeProbe
        command: ["sh", "-c", "command -v zenith-store >/dev/null 2>&1"]
        running: true
        onExited: code => page.storePresent = (code === 0)
    }

    function browseStyles() {
        if (page.storePresent)
            Spawn.run(["zenith-store", "open", "barstyles"]);
    }

    // The catalogue is the store's own account of what exists and what has an
    // update. Asking it is a user action (a button), never a side effect of
    // opening this page.
    function checkStore() {
        if (!page.storePresent || page.checking)
            return;
        page.note = "";
        page.checking = true;
        catalogProc.running = true;
    }
    Process {
        id: catalogProc
        command: ["zenith-store", "catalog", "--category", "barstyles"]
        stdout: StdioCollector { id: catalogOut }
        stderr: StdioCollector { id: catalogErr }
        onExited: code => {
            page.checking = false;
            if (code !== 0) {
                page.note = catalogErr.text.trim() || I18n.tr("The store did not answer.");
                return;
            }
            try {
                const catalog = JSON.parse(catalogOut.text || "{}");
                // Every bar style the catalogue carries, not only the ones already
                // installed: a style you have yet to fetch still belongs on the
                // shelf so you can see it exists. The one style hidden is one
                // written for another compositor that you have not installed --
                // it can neither run nor be fetched.
                page.catalogStyles = (catalog.items || [])
                    .filter(item => item.category === "barstyles"
                        && !(item.unavailable === true && item.installed !== true))
                    .map(item => ({
                        id: item.id,
                        name: item.name || item.id,
                        desc: item.summary || item.description || "",
                        installed: item.installed === true,
                        version: item.version || "",
                        installedVersion: item.installedVersion || "",
                        updateAvailable: item.updateAvailable === true,
                        unavailable: item.unavailable === true,
                        unavailableReason: item.unavailableReason || "",
                        requiredWindowManager: item.requiredWindowManager || "",
                        downloadPaused: item.downloadPaused === true,
                        downloadPauseReason: item.downloadPauseReason || ""
                    }));
            } catch (e) {
                page.note = I18n.tr("The store's answer could not be read.");
            }
        }
    }

    // The store only installs; this re-runs an install over the receipt-owned
    // tree, which replaces the style in place. Install never activates, and only
    // a removal rewrites the bar selection, so updating the style you are wearing
    // is safe: the shell keys style URLs to the store revision and swaps the new
    // content without a reload.
    function updateStyle(id) {
        if (!id || page.updatingId !== "" || !page.storePresent)
            return;
        page.note = "";
        page.updatingId = id;
        updateProc.command = ["zenith-store", "install", "barstyles", id];
        updateProc.running = true;
    }
    Process {
        id: updateProc
        stderr: StdioCollector { id: updateErr }
        onExited: code => {
            page.updatingId = "";
            if (code !== 0)
                page.note = updateErr.text.trim() || I18n.tr("The update did not finish.");
            page.checkStore();
        }
    }

    // A command run under a guard: a tool that is not installed is a sentence the
    // user can read, not a click that does nothing.
    function guarded(argv) {
        return ["sh", "-c",
            'command -v "$1" >/dev/null 2>&1 || { echo "$1 is not installed" >&2; exit 127; }; exec "$@"',
            "sh"].concat(argv);
    }
    Process {
        id: actionProc
        property string label: ""
        stderr: StdioCollector { id: actionErr }
        onExited: code => {
            if (code !== 0)
                page.note = I18n.tr("%1 could not be opened: %2")
                    .arg(actionProc.label).arg(actionErr.text.trim() || I18n.tr("the shell did not answer."));
        }
    }
    // The Hub is its own Quickshell instance, so a call to the desktop's IPC is
    // addressed to the shell tree by path; the shell lives in this Hub's own root.
    function openQsBarSettings(route) {
        if (actionProc.running)
            return;
        page.note = "";
        actionProc.label = I18n.tr("QS Bar Settings");
        actionProc.command = page.guarded(["quickshell", "ipc", "--path", Quickshell.shellDir,
            "call", "qsbar", "settings", route || ""]);
        actionProc.running = true;
    }

    // The Obi bar's widgets, for the per-widget show/hide toggles below. Mirrors
    // barstyles/obi/Scene.qml; Workspaces is the bar's identity and has no toggle.
    readonly property var obiWidgets: [
        { id: "activeWindow", label: I18n.tr("Active window"), desc: I18n.tr("The focused window's title, far left.") },
        { id: "resources", label: I18n.tr("Resources"), desc: I18n.tr("CPU and memory rings.") },
        { id: "media", label: I18n.tr("Media"), desc: I18n.tr("Now playing with a music visualizer.") },
        { id: "audio", label: I18n.tr("Audio"), desc: I18n.tr("Output and input volume, with a mixer.") },
        { id: "clock", label: I18n.tr("Clock"), desc: I18n.tr("Time and date.") },
        { id: "connectivity", label: I18n.tr("Connections"), desc: I18n.tr("Wi-Fi and Bluetooth.") },
        { id: "battery", label: I18n.tr("Battery"), desc: I18n.tr("Charge and power profile.") },
        { id: "tray", label: I18n.tr("Tray"), desc: I18n.tr("System tray icons.") },
        { id: "weather", label: I18n.tr("Weather"), desc: I18n.tr("Current conditions.") }
    ]

    // Stage AND apply: edits ride the shared draft like every page, and the
    // hub's stageLive coalesces a settings.patch to the daemon so the running
    // desktop repaints as you work. The probe harness's bare hub has neither;
    // fall back so the page still loads and stages inertly.
    function stage(next) {
        if (!next || !page.hub) return;
        if (page.hub.stageLive) page.hub.stageLive("frameBars", next);
        else if (page.hub.edit) page.hub.edit("frameBars", next);
    }

    // The frame chrome keys live beside frameBars in shell.json: the running
    // shell reads frameEnabled for the draw toggle and frameOpacity for how solid
    // the frame paints, so they belong on the frame's own studio and stage
    // through the same live channel.
    function fval(key, fall) {
        if (!page.hub || !page.hub.val) return fall;
        const v = page.hub.val(key);
        return v === undefined || v === null ? fall : v;
    }
    function fnum(key, fall) {
        const v = Number(page.fval(key, fall));
        return isFinite(v) ? v : fall;
    }
    function fedit(key, value) {
        if (!page.hub) return;
        if (page.hub.stageLive) page.hub.stageLive(key, value);
        else if (page.hub.edit) page.hub.edit(key, value);
    }
    function fwas(key) {
        return page.hub && page.hub.committed ? page.hub.committed[key] : undefined;
    }

    // Obi's per-widget visibility lives in the `obi` map in shell.json (an absent
    // key reads as shown). Toggling stages the whole map live like every edit.
    function obiShown(id) {
        const o = page.fval("obi", ({}));
        return !o || o[id] !== false;
    }
    function obiSet(id, on) {
        const o = Object.assign({}, page.fval("obi", ({})));
        o[id] = on;
        page.fedit("obi", o);
    }

    // ── the dock ─────────────────────────────────────────────────────────────
    // The first-class dock (modules/dock) is a surface of its own, so its look
    // and pins are the top-level `dock` object in shell.json and every bar style
    // shares it. The fallbacks below are the ones Services/Dock.qml applies to a
    // key the object does not carry, so an absent key reads here the way the
    // dock reads it. A toggle writes the whole object, pins included.
    readonly property var dockDefaults: ({
        "enabled": true, "edge": "auto", "style": "islands", "autohide": true,
        "magnify": true, "media": false, "frost": true, "shadow": true, "labels": true
    })
    readonly property var dockEdgeLabels: ({
        "auto": I18n.tr("Auto"), "top": I18n.tr("Top"), "bottom": I18n.tr("Bottom"),
        "left": I18n.tr("Left"), "right": I18n.tr("Right")
    })
    // Dock.qml's styleOptions, by key; the dock only changes what its band draws.
    readonly property var dockStyleLabels: ({
        "islands": I18n.tr("Islands"), "rail": I18n.tr("Rail"), "ledger": I18n.tr("Ledger"),
        "tanzaku": I18n.tr("Tanzaku"), "seal": I18n.tr("Seal")
    })
    function dockObject() {
        const d = page.fval("dock", ({}));
        return (d && typeof d === "object" && !Array.isArray(d)) ? d : ({});
    }
    function dockVal(key) {
        const d = page.dockObject();
        return d[key] !== undefined && d[key] !== null ? d[key] : page.dockDefaults[key];
    }
    function dockOn(key) { return !!page.dockVal(key); }
    function dockSet(key, value) {
        const next = Object.assign({}, page.dockObject());
        next[key] = value;
        page.fedit("dock", next);
    }
    // The saved state at open: a live edit has already rebased `committed`, so
    // the unsaved distance is measured against the Hub's live baseline.
    function dockWas(key) {
        const b = page.hub && page.hub.liveBaseline && page.hub.liveBaseline.dock ? page.hub.liveBaseline.dock : null;
        return (b && b[key] !== undefined && b[key] !== null) ? b[key] : page.dockDefaults[key];
    }
    function dockChanged(key) { return page.dockVal(key) !== page.dockWas(key); }
    function dockWasSwitch(key) { return page.dockWas(key) ? I18n.tr("ON") : I18n.tr("OFF"); }
    readonly property int dockPinned: {
        const p = page.dockObject().pinned;
        return Array.isArray(p) ? p.length : 0;
    }

    // ── QS Bar layout summary ─────────────────────────────────────────────────
    // The QS Bar's layout, widgets, form and dock are arranged in QS Bar Settings
    // (the bar logo opens it). This page keeps a read-only summary of the order,
    // read from the daemon's settings topic so it tracks a move made from the
    // panel or the CLI without a Hub reload.
    readonly property string qsbarLayoutSummary: {
        Settings.revision;
        const layout = Settings.get("qsbar.layout") || ({});
        const lane = a => Array.isArray(a) ? a.join(" \u00b7 ") : "";
        const lanes = [lane(layout.left), lane(layout.center), lane(layout.right)].filter(s => s.length > 0);
        return lanes.join("  |  ");
    }

    CatalogLabels { id: labels }

    // ── head: the eyebrow band, the title, the blurb ─────────────────────────
    Column {
        id: head
        anchors.top: parent.top
        // the head sits on the body's grid: full body width from the left, so
        // the title starts over the first card column instead of floating centred
        x: 0
        width: page.width - 14
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Item {
            width: parent.width
            height: 14
            Row {
                // the register row holds a fixed box, so the rule and the seal keep
                // their distance from the title on every page
                height: Tokens.s5
                id: ebrow
                spacing: Tokens.s2
                anchors.verticalCenter: parent.verticalCenter
                Rectangle { width: 16; height: 1; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                Text {
                    text: "天"; color: Tokens.ink; font.family: Tokens.jp
                    font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: I18n.tr("DESKTOP"); color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
            Rectangle {
                anchors { left: ebrow.right; right: crossMark.left; verticalCenter: parent.verticalCenter }
                anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3
                height: 1; color: Tokens.lineSoft
            }
            Text {
                id: crossMark
                anchors { right: slashMark.left; rightMargin: Tokens.s2; verticalCenter: parent.verticalCenter }
                text: "+"; color: Tokens.inkFaint
                font.family: Tokens.mono; font.pixelSize: 10
            }
            Text {
                id: slashMark
                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                text: "///"; color: Tokens.inkFaint
                font.family: Tokens.mono; font.pixelSize: 10
            }
        }
        Text {
            text: I18n.tr("Bar Studio")
            color: Tokens.ink
            font.family: Tokens.display
            font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Which bar the desktop draws, how it looks, and the dock.")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            wrapMode: Text.WordWrap
        }
    }

    // ── the sheet: STYLE, DOCK, FRAME, RAILS, WIDGETS in one scroll ──────────
    Flickable {
        id: flick
        anchors { left: parent.left; right: parent.right; top: head.bottom; bottom: parent.bottom; topMargin: Tokens.s5 }
        contentWidth: width
        contentHeight: Math.max(col.height, flick.height)
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        CardColumns {
            id: col
            // a body of cards fills the measure and splits into balanced columns
            width: flick.width - 14
            spacing: Tokens.s5
            fillTo: flick.height

            // ── BAR STYLE: which bar the desktop draws ───────────────────────
            SettingCard {
                id: styleSect
                // The primary choice of the page takes the band across the
                // columns: the style tiles need the room, and a column-width
                // card crams them two to a row while the page's other half
                // sits empty.
                property bool fullWidth: true
                width: col.colWidth
                title: I18n.tr("BAR STYLE")

                Item {
                    width: parent.width
                    height: styleBody.height + Tokens.s3 + Tokens.s4
                    Column {
                        id: styleBody
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
                        spacing: Tokens.s3
                        // A gallery, not a filmstrip: with a bar style per tile the
                        // captions are what tell them apart, so the tiles wrap into
                        // as many rows as they need.
                        // The height is computed, not measured: a Flow reports its
                        // post-wrap height a polish pass late, and CardColumns would
                        // place the next card over this one's second row.
                        Flow {
                            id: styleRow
                            width: parent.width
                            spacing: Tokens.s2
                            readonly property int perRow: Math.max(2, Math.floor((width + Tokens.s2) / 210))
                            readonly property int tileH: 64
                            readonly property int rowCount: Math.max(1, Math.ceil(styleRep.count / perRow))
                            height: rowCount * tileH + (rowCount - 1) * spacing
                            Repeater {
                                id: styleRep
                                model: page.shelf
                                delegate: Rectangle {
                                    id: styleCard
                                    required property var modelData
                                    // A style applies only when it is installed
                                    // and can run on this compositor. A paused
                                    // style stays applyable once installed (the
                                    // pause blocks only new downloads); a
                                    // not-installed or wm-gated one cannot.
                                    readonly property bool applyable: styleCard.modelData.installed && !styleCard.modelData.unavailable
                                    readonly property bool on: styleCard.applyable && page.activeStyle === styleCard.modelData.id
                                    // The store refuses an install over a paused download or a
                                    // product written for another compositor, so an update is
                                    // offered only where re-running the install can succeed --
                                    // never a button that is guaranteed to do nothing.
                                    readonly property bool updatable: page.storePresent
                                        && styleCard.modelData.updateAvailable === true
                                        && styleCard.modelData.version.length > 0
                                        && !styleCard.modelData.unavailable && !styleCard.modelData.downloadPaused
                                    // The sub line reads the style's own blurb
                                    // when it is yours to apply, otherwise the
                                    // honest reason it is not: the compositor it
                                    // wants, that it is under construction, or
                                    // where to fetch it. A style with an update
                                    // shows the version it would move to, so the
                                    // UPDATE button next to it is self-explanatory.
                                    readonly property string subText: {
                                        if (styleCard.updatable) {
                                            const cur = styleCard.modelData.installedVersion.length > 0
                                                ? styleCard.modelData.installedVersion : styleCard.modelData.version;
                                            return I18n.tr("%1 \u2192 %2").arg(cur).arg(styleCard.modelData.version);
                                        }
                                        if (styleCard.applyable)
                                            return styleCard.modelData.desc.length > 0
                                                ? I18n.tr(styleCard.modelData.desc) : I18n.tr("Installed");
                                        if (styleCard.modelData.unavailable) {
                                            const wm = ("" + styleCard.modelData.requiredWindowManager).toUpperCase();
                                            const tag = wm.length > 0 ? I18n.tr("%1 only").arg(wm) : I18n.tr("Unavailable");
                                            return styleCard.modelData.unavailableReason.length > 0
                                                ? tag + " \u00b7 " + styleCard.modelData.unavailableReason : tag;
                                        }
                                        if (styleCard.modelData.downloadPaused)
                                            return styleCard.modelData.downloadPauseReason.length > 0
                                                ? I18n.tr("Under construction") + " \u00b7 " + styleCard.modelData.downloadPauseReason
                                                : I18n.tr("Under construction");
                                        return page.storePresent
                                            ? I18n.tr("Available \u00b7 install from the Store")
                                            : I18n.tr("Not installed");
                                    }

                                    objectName: "bar-style-" + styleCard.modelData.id
                                    // fill the row evenly: as many ~210px tiles as the
                                    // measure holds, stretched to close the last gap.
                                    width: Math.floor((styleRow.width - (styleRow.perRow - 1) * Tokens.s2) / styleRow.perRow)
                                    height: styleRow.tileH
                                    radius: Tokens.radius
                                    // dim a style you cannot apply from here
                                    opacity: styleCard.applyable ? 1.0 : 0.55
                                    color: styleCard.on ? Tokens.bone : (sma.containsMouse ? Tokens.tint5 : "transparent")
                                    border.width: Tokens.border
                                    border.color: styleCard.on ? Tokens.bone : Tokens.line
                                    Behavior on color { ColorAnimation { duration: Tokens.snap } }

                                    Column {
                                        anchors {
                                            left: parent.left
                                            right: updateBtn.visible ? updateBtn.left : parent.right
                                            margins: Tokens.s3; rightMargin: updateBtn.visible ? Tokens.s2 : Tokens.s3
                                            verticalCenter: parent.verticalCenter
                                        }
                                        spacing: 3
                                        Text {
                                            text: styleCard.modelData.name.toUpperCase()
                                            color: styleCard.on ? Tokens.inkOnBone : Tokens.inkDim
                                            font.family: Tokens.ui
                                            font.pixelSize: 12
                                            font.weight: Font.Medium
                                            font.letterSpacing: Tokens.trackLabel
                                        }
                                        Text {
                                            width: parent.width
                                            text: styleCard.subText
                                            color: styleCard.on ? Tokens.inkOnBoneDim : Tokens.inkFaint
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fTiny
                                            elide: Text.ElideRight
                                        }
                                    }
                                    MouseArea {
                                        id: sma
                                        anchors.fill: parent
                                        hoverEnabled: true
                                        preventStealing: true
                                        cursorShape: styleCard.applyable || page.storePresent ? Qt.PointingHandCursor : Qt.ArrowCursor
                                        onClicked: styleCard.applyable ? page.fedit("barStyle", styleCard.modelData.id) : page.browseStyles()
                                    }
                                    // Declared after the MouseArea so it stacks
                                    // above it: the tile applies on click, and the
                                    // update must not be stolen by that handler.
                                    Btn {
                                        id: updateBtn
                                        anchors { right: parent.right; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                                        visible: styleCard.updatable
                                        compact: true
                                        armed: page.updatingId === ""
                                        text: page.updatingId === styleCard.modelData.id
                                            ? I18n.tr("UPDATING")
                                            : I18n.tr("UPDATE")
                                        objectName: "bar-style-update-" + styleCard.modelData.id
                                        onAct: page.updateStyle(styleCard.modelData.id)
                                    }
                                }
                            }
                        }
                        Row {
                            visible: page.storePresent
                            spacing: Tokens.s2
                            Btn {
                                text: I18n.tr("BROWSE STORE")
                                onAct: page.browseStyles()
                            }
                            Btn {
                                text: page.checking ? I18n.tr("CHECKING") : I18n.tr("CHECK FOR UPDATES")
                                armed: !page.checking
                                onAct: page.checkStore()
                            }
                        }
                        Text {
                            width: parent.width
                            visible: !page.storePresent
                            text: I18n.tr("The Zenith Store is not installed, so only the built-in styles and the ones already on this machine are listed.")
                            color: Tokens.inkFaint
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                        Text {
                            width: parent.width
                            visible: page.note !== ""
                            text: page.note
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                    }
                }
            }

            // ── QS BAR: its layout, widgets and form live in QS Bar Settings ──
            SettingCard {
                id: qsbarSect
                width: col.colWidth
                visible: page.activeStyle === "qsbar"
                title: I18n.tr("QS BAR")
                kana: "帯"

                Item {
                    width: parent.width
                    height: qsbarBody.height + Tokens.s3 + Tokens.s4
                    Column {
                        id: qsbarBody
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
                        spacing: Tokens.s3
                        Text {
                            width: parent.width
                            text: I18n.tr("The QS Bar arranges its own layout, widgets and form in QS Bar Settings. The bar logo opens it, or the button below.")
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fBody
                            wrapMode: Text.WordWrap
                        }
                        Text {
                            width: parent.width
                            visible: page.qsbarLayoutSummary.length > 0
                            text: page.qsbarLayoutSummary
                            color: Tokens.inkDim
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                        Btn {
                            text: I18n.tr("OPEN QS BAR SETTINGS")
                            armed: !actionProc.running
                            onAct: page.openQsBarSettings("")
                        }
                    }
                }
            }

            // ── KAIROS: its island settings live on the island ────────────────
            SettingCard {
                id: kairosSect
                width: col.colWidth
                visible: page.activeStyle === "kairos"
                title: I18n.tr("KAIROS")

                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s4
                    text: I18n.tr("Kairos keeps its own settings: hover the clock island to open it, then use the gear in its top-right corner. The same island is the launcher on Super+Space.")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fBody
                    wrapMode: Text.WordWrap
                }
            }

            // A store style owns its own frame, rails and widgets inside its
            // folder, so the Sumi editors below stand down.
            SettingCard {
                id: folderNote
                width: col.colWidth
                visible: !page.sumiActive && ["qsbar", "kairos", "obi", "nacre"].indexOf(page.activeStyle) < 0
                title: I18n.tr("LAYOUT")

                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s4
                    text: I18n.tr("The %1 style manages its own layout.").arg(page.activeName)
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fBody
                    wrapMode: Text.WordWrap
                }
            }

            // OBI WIDGETS: show or hide each widget on the Obi bar.
            SettingCard {
                id: obiSect
                width: col.colWidth
                visible: page.activeStyle === "obi"
                title: I18n.tr("OBI WIDGETS")

                Repeater {
                    model: page.obiWidgets
                    delegate: SettingRow {
                        id: obiRow
                        required property var modelData
                        required property int index
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: obiRow.index > 0
                        controlWidth: 54
                        label: I18n.tr(obiRow.modelData.label)
                        desc: I18n.tr(obiRow.modelData.desc)
                        source: "shell.json"
                        Sw {
                            objectName: "obi-" + obiRow.modelData.id
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: page.obiShown(obiRow.modelData.id)
                            onToggled: value => page.obiSet(obiRow.modelData.id, value)
                        }
                    }
                }
            }

            SettingCard {
                id: nacreSect
                width: col.colWidth
                visible: page.activeStyle === "nacre"
                title: I18n.tr("NACRE LAYOUT")

                Item {
                    width: parent.width
                    height: nacreEd.height + Tokens.s3 + Tokens.s4
                    NacreEditor {
                        id: nacreEd
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
                        config: page.fval("nacre", ({}))
                        onStaged: value => page.fedit("nacre", value)
                    }
                }
            }

            // ── DOCK: the app dock, on any bar style ─────────────────────────
            SettingCard {
                id: dockSect
                width: col.colWidth
                title: I18n.tr("DOCK")
                kana: "台"

                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    controlWidth: 54
                    label: I18n.tr("Show the dock")
                    desc: I18n.tr("App icons on an edge of their own, whichever bar style is on.")
                    def: page.dockWasSwitch("enabled")
                    changed: page.dockChanged("enabled")
                    source: "shell.json"
                    Sw {
                        objectName: "dock-enabled"
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        on: page.dockOn("enabled")
                        onToggled: value => page.dockSet("enabled", value)
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    block: true
                    enabled: page.dockOn("enabled")
                    label: I18n.tr("Edge")
                    desc: I18n.tr("Auto puts it opposite the bar.")
                    def: page.dockEdgeLabels[page.dockWas("edge")] || ""
                    changed: page.dockChanged("edge")
                    source: "shell.json"
                    Chips {
                        objectName: "dock-edge"
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        options: ["auto", "top", "bottom", "left", "right"]
                        labels: page.dockEdgeLabels
                        current: String(page.dockVal("edge"))
                        onChose: key => page.dockSet("edge", key)
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    block: true
                    enabled: page.dockOn("enabled")
                    label: I18n.tr("Style")
                    desc: I18n.tr("How the dock band is drawn.")
                    def: page.dockStyleLabels[page.dockWas("style")] || ""
                    changed: page.dockChanged("style")
                    source: "shell.json"
                    Chips {
                        objectName: "dock-style"
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        options: ["islands", "rail", "ledger", "tanzaku", "seal"]
                        labels: page.dockStyleLabels
                        current: String(page.dockVal("style"))
                        onChose: key => page.dockSet("style", key)
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    enabled: page.dockOn("enabled")
                    label: I18n.tr("Pinned apps")
                    value: String(page.dockPinned)
                    desc: page.dockPinned > 0
                        ? I18n.tr("Right-click an icon on the dock to pin or unpin it; drag to reorder.")
                        : I18n.tr("None yet: the dock shows a starter set. Right-click a running app to pin it.")
                    source: "shell.json"
                }
            }

            SettingCard {
                id: dockLookSect
                width: col.colWidth
                title: I18n.tr("DOCK BEHAVIOUR")
                kana: "振舞"

                Repeater {
                    model: [
                        { key: "autohide", label: I18n.tr("Auto-hide"), desc: I18n.tr("A thin strip stays until the pointer reaches the edge.") },
                        { key: "magnify", label: I18n.tr("Magnify"), desc: I18n.tr("Grow the icon under the pointer.") },
                        { key: "media", label: I18n.tr("Media chip"), desc: I18n.tr("Show now playing on the dock, only while audio plays.") },
                        { key: "frost", label: I18n.tr("Frost"), desc: I18n.tr("Blur what is behind the dock.") },
                        { key: "shadow", label: I18n.tr("Depth"), desc: I18n.tr("A soft shadow under the dock.") },
                        { key: "labels", label: I18n.tr("Hover labels"), desc: I18n.tr("Name an app when the pointer rests on it.") }
                    ]
                    delegate: SettingRow {
                        id: dockRow
                        required property var modelData
                        required property int index
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: dockRow.index > 0
                        enabled: page.dockOn("enabled")
                        controlWidth: 54
                        label: dockRow.modelData.label
                        desc: dockRow.modelData.desc
                        def: page.dockWasSwitch(dockRow.modelData.key)
                        changed: page.dockChanged(dockRow.modelData.key)
                        source: "shell.json"
                        Sw {
                            objectName: "dock-" + dockRow.modelData.key
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: page.dockOn(dockRow.modelData.key)
                            onToggled: value => page.dockSet(dockRow.modelData.key, value)
                        }
                    }
                }
            }

            // ── FRAME: the chrome the shell draws around the desktop ─────────
            SettingCard {
                id: frameSect
                width: col.colWidth
                title: I18n.tr("FRAME")
                visible: page.sumiActive

                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    controlWidth: 54
                    label: I18n.tr("Draw frame")
                    def: page.fwas("frameEnabled") === undefined ? "" : (page.fwas("frameEnabled") ? I18n.tr("ON") : I18n.tr("OFF"))
                    changed: page.fwas("frameEnabled") !== undefined && !!page.fval("frameEnabled", true) !== !!page.fwas("frameEnabled")
                    desc: I18n.tr("The bounded band drawn around the desktop.")
                    source: "shell.json"
                    Sw {
                        objectName: "frame-enabled"
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        on: !!page.fval("frameEnabled", true)
                        onToggled: value => page.fedit("frameEnabled", value)
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    controlWidth: Math.min(240, Math.max(160, Math.round(frameSect.width * 0.34)))
                    label: I18n.tr("Opacity")
                    unit: "%"
                    value: String(Math.round(page.fnum("frameOpacity", 1) * 100))
                    def: page.fwas("frameOpacity") === undefined ? "" : String(Math.round(Number(page.fwas("frameOpacity")) * 100))
                    changed: page.fwas("frameOpacity") !== undefined && page.fnum("frameOpacity", 1) !== Number(page.fwas("frameOpacity"))
                    desc: I18n.tr("How solid the frame draws.")
                    source: "shell.json"
                    Slid {
                        objectName: "frame-opacity"
                        anchors.fill: parent
                        from: 0.5; to: 1.0
                        value: page.fnum("frameOpacity", 1)
                        onModified: value => page.fedit("frameOpacity", value)
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    controlWidth: 58
                    label: I18n.tr("Thickness")
                    unit: "px"
                    value: String(page.fnum("frameThickness", 2))
                    def: page.fwas("frameThickness") === undefined ? "" : String(page.fwas("frameThickness"))
                    changed: page.fwas("frameThickness") !== undefined && page.fnum("frameThickness", 2) !== Number(page.fwas("frameThickness"))
                    desc: I18n.tr("How far the band stands into the screen.")
                    source: "shell.json"
                    Step {
                        objectName: "frame-thickness"
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        from: 0; to: 24
                        value: page.fnum("frameThickness", 2)
                        onModified: value => page.fedit("frameThickness", value)
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    controlWidth: 58
                    label: I18n.tr("Corner radius")
                    unit: "px"
                    value: String(page.fnum("frameCorner", 8))
                    def: page.fwas("frameCorner") === undefined ? "" : String(page.fwas("frameCorner"))
                    changed: page.fwas("frameCorner") !== undefined && page.fnum("frameCorner", 8) !== Number(page.fwas("frameCorner"))
                    desc: I18n.tr("How round the frame cuts the screen's corners.")
                    source: "shell.json"
                    Step {
                        objectName: "frame-corner"
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        from: 0; to: 40
                        value: page.fnum("frameCorner", 8)
                        onModified: value => page.fedit("frameCorner", value)
                    }
                }
            }

            // ── RAILS: pick an edge, then its own switches ───────────────────
            SettingCard {
                id: railSect
                width: col.colWidth
                title: I18n.tr("RAILS")
                visible: page.sumiActive

                Item {
                    width: parent.width
                    height: railRow.height + Tokens.s3 + Tokens.s3
                    Row {
                        id: railRow
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
                        spacing: Tokens.s2
                        Repeater {
                            model: ["left"]
                            delegate: Rectangle {
                                id: plate
                                required property string modelData
                                readonly property var pRail: page.config.rails[plate.modelData]
                                readonly property int count: {
                                    const zs = plate.modelData === "top" || plate.modelData === "bottom" ? ["start", "center", "end"] : ["top", "center", "bottom"];
                                    let n = 0;
                                    for (const zone of zs) n += (plate.pRail[zone] || []).length;
                                    return n;
                                }
                                readonly property bool on: page.edge === plate.modelData

                                objectName: "rail-edge-" + plate.modelData
                                width: (railRow.width - 3 * Tokens.s2) / 4
                                height: 48
                                radius: Tokens.radius
                                color: plate.on ? Tokens.bone : (pma.containsMouse ? Tokens.tint5 : "transparent")
                                border.width: Tokens.border
                                border.color: plate.on ? Tokens.bone : Tokens.line
                                Behavior on color { ColorAnimation { duration: Tokens.snap } }

                                Column {
                                    anchors { left: parent.left; leftMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                                    spacing: 2
                                    Text {
                                        text: labels.edge(plate.modelData).toUpperCase()
                                        color: plate.on ? Tokens.inkOnBone : Tokens.inkDim
                                        font.family: Tokens.ui
                                        font.pixelSize: 11
                                        font.weight: Font.Medium
                                        font.letterSpacing: Tokens.trackLabel
                                    }
                                    Text {
                                        text: plate.pRail.enabled ? I18n.tr("on · %1").arg(plate.count) : I18n.tr("off")
                                        color: plate.on ? Tokens.inkOnBoneDim : Tokens.inkFaint
                                        font.family: Tokens.mono
                                        font.pixelSize: Tokens.fTiny
                                    }
                                }
                                MouseArea {
                                    id: pma
                                    anchors.fill: parent
                                    hoverEnabled: true
                                    preventStealing: true
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: page.edge = plate.modelData
                                }
                            }
                        }
                    }
                }

                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    controlWidth: 54
                    label: I18n.tr("Show this rail")
                    def: page.railWas ? (page.railWas.enabled ? I18n.tr("ON") : I18n.tr("OFF")) : ""
                    changed: !!page.railWas && page.rail.enabled !== page.railWas.enabled
                    desc: I18n.tr("Draw the %1 rail on the frame.").arg(labels.edge(page.edge).toLowerCase())
                    source: "shell.json"
                    Sw {
                        objectName: "rail-enabled"
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        on: page.rail.enabled
                        onToggled: value => page.stage(Model.setRail(page.config, page.edge, { enabled: value }))
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    controlWidth: Math.min(240, Math.max(160, Math.round(railSect.width * 0.34)))
                    label: I18n.tr("Thickness")
                    unit: "px"
                    value: String(page.rail.size)
                    def: page.railWas ? String(page.railWas.size) : ""
                    changed: !!page.railWas && page.rail.size !== page.railWas.size
                    desc: I18n.tr("How far the %1 rail stands into the screen.").arg(labels.edge(page.edge).toLowerCase())
                    source: "shell.json"
                    Slid {
                        objectName: "rail-thickness"
                        anchors.fill: parent
                        from: page.horizontal ? 16 : 24
                        to: page.horizontal ? 96 : 112
                        value: page.rail.size
                        onModified: value => page.stage(Model.setRail(page.config, page.edge, { size: value }))
                    }
                }
            }

            // ── WIDGETS: the selected rail's three zones and its add drawers ──
            SettingCard {
                id: zoneSect
                width: col.colWidth
                title: I18n.tr("WIDGETS ON THE %1 RAIL").arg(labels.edge(page.edge).toUpperCase())
                visible: page.sumiActive

                Item {
                    width: parent.width
                    height: zoneEd.height + Tokens.s3 + Tokens.s4
                    ZoneEditor {
                        id: zoneEd
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
                        config: page.config
                        edge: page.edge
                        catalog: BarCatalog
                        onStaged: next => page.stage(next)
                    }
                }
            }
        }
    }

}
