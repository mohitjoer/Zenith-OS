pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import qs.Ui
import qs.Ui.Singletons

// Zenith Agent (ADVANCED). The optional local assistant: a machine-generated
// vault, memory, skills, agents and chat. The agent engine that answers is the
// user's own, named by one config key (engine.command), and this page says plainly
// when there is none. It keeps the master switch and the dashboard link (the web
// dashboard at 127.0.0.1:3600, served from desktop/cmd/zenith-agent/web). Live
// state comes from `zenith-agent status --json`. The assistant is off until you
// switch it on.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    // Monochrome like the rest of the Hub: the accent names stay so the page's
    // rules do not change, and they resolve to ink and its dimmer tiers.
    QtObject {
        id: hx
        readonly property color paper: Tokens.paper
        readonly property color paper2: Tokens.paperLift
        readonly property color ink: Tokens.ink
        readonly property color inkDim: Tokens.inkDim
        readonly property color red: Tokens.ink
        readonly property color redDeep: Tokens.inkMuted
        readonly property color teal: Tokens.inkMuted
        readonly property color orange: Tokens.ink
        readonly property color slate: Tokens.inkDim
        readonly property color tan: Tokens.inkDim
        readonly property color line: Tokens.line
        readonly property color lineSoft: Tokens.lineSoft
    }

    readonly property string fDisplay: Tokens.display
    readonly property string fMono: Tokens.mono
    readonly property string fJp: Tokens.jp

    // centered body column; the hero and every section share this measure.
    readonly property real bodyW: Math.min(pg.width - Tokens.s5 * 2, 1080)

    // ── status snapshot (page readouts, not stored config) ───────────────────
    property bool installed: true
    property bool daemonEnabled: false
    property bool running: false
    // the port the daemon reports, not a literal: a user who changed it in
    // ~/.config/zenith/agent/config.json still gets a link that resolves.
    property int port: 3600
    property bool vaultExists: false
    property int vaultFiles: 0
    // the engine, as `status --json` reports it; engineReason is the plain
    // sentence to show when it cannot answer.
    property bool engineConfigured: false
    property bool engineAvailable: false
    property string engineCommand: ""
    property string engineReason: ""
    property string engineKey: "engine.command"
    property string engineConfig: "~/.config/zenith/agent/config.json"
    property int agentsPresent: 0
    property int agentsWired: 0
    // manifest: the full agent surface (paths + agents), from
    // `zenith-agent paths --json`. Drives the Agents and Connect sections.
    property var agentList: []
    property string skillPath: ""
    property string prowlPath: ""
    property var vaultItems: []

    readonly property string wiredSummary: pg.agentsPresent > 0
        ? I18n.tr("%1 / %2 wired").arg(pg.agentsWired).arg(pg.agentsPresent)
        : I18n.tr("none yet")

    Component.onCompleted: pg.refresh()
    function refresh() { statusProc.running = true; manifestProc.running = true; }

    Process {
        id: statusProc
        // sh-wrapped so a missing zenith-agent still closes stdout and fires
        // onStreamFinished; empty/unparseable output reads as "not installed".
        command: ["sh", "-c", "zenith-agent status --json"]
        stderr: StdioCollector {}
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var o = JSON.parse(this.text);
                    pg.installed = true;
                    pg.daemonEnabled = o.enabled === true;
                    pg.running = o.running === true;
                    if (typeof o.port === "number" && o.port > 0)
                        pg.port = o.port;
                    var v = o.vault || ({});
                    pg.vaultExists = v.exists === true;
                    pg.vaultFiles = (typeof v.files === "number") ? v.files : 0;
                    var e = o.engine || ({});
                    pg.engineConfigured = e.configured === true;
                    pg.engineAvailable = e.available === true;
                    pg.engineCommand = (e.command || []).join(" ");
                    pg.engineReason = e.reason || "";
                    pg.engineKey = e.key || "engine.command";
                    pg.engineConfig = e.config || pg.engineConfig;
                    var ags = o.agents || [];
                    var present = 0, wired = 0;
                    for (var i = 0; i < ags.length; i++) {
                        if (ags[i].present) {
                            present++;
                            if (ags[i].wired)
                                wired++;
                        }
                    }
                    pg.agentsPresent = present;
                    pg.agentsWired = wired;
                    return;
                } catch (e) {
                    // empty output (binary missing) or malformed JSON.
                }
                pg.installed = false;
                pg.daemonEnabled = false;
                pg.running = false;
            }
        }
    }

    Process { id: enableProc; onExited: pg.refresh() }
    Process { id: disableProc; onExited: pg.refresh() }
    function setEnabled(on) {
        var p = on ? enableProc : disableProc;
        p.command = ["zenith-agent", on ? "enable" : "disable"];
        p.running = true;
    }

    // Open the dashboard the daemon actually serves. Two things used to send a
    // user to a browser error they read as a 404: the port was hardcoded here
    // while the daemon reads it from its own config, and the button opened the
    // URL even with nothing listening. Take the port from `status --json` and
    // start the daemon first when it is down; `zenith-agent serve` returns once
    // the socket is up, so the browser never races it.
    function openDashboard() {
        var url = "http://127.0.0.1:" + pg.port;
        if (pg.running) {
            Spawn.run(["xdg-open", url]);
            return;
        }
        openProc.command = ["sh", "-c",
            "zenith-agent serve --if-enabled >/dev/null 2>&1 & "
            + "for i in 1 2 3 4 5 6 7 8 9 10; do "
            + "zenith-agent status --json 2>/dev/null | grep -q '\"running\":true' && break; sleep 0.3; done; "
            + "xdg-open " + url];
        openProc.running = true;
    }
    Process { id: openProc; onExited: pg.refresh() }

    // the agent surface, refreshed with status. Best effort: a missing binary
    // leaves the sections empty rather than erroring.
    Process {
        id: manifestProc
        command: ["sh", "-c", "zenith-agent paths --json"]
        stderr: StdioCollector {}
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var m = JSON.parse(this.text);
                    pg.skillPath = (m.skill && m.skill.path) || "";
                    pg.prowlPath = (m.prowl && m.prowl.path) || "";
                    pg.vaultItems = m.vault || [];
                    pg.agentList = m.agents || [];
                } catch (e) {}
            }
        }
    }
    Process { id: wireProc; onExited: pg.refresh() }
    // Wiring drops the pointer + the zenith skill + prowl's skills into an
    // agent (or every present one).
    function wireAgent(id) { wireProc.command = ["zenith-agent", "wire", id]; wireProc.running = true; }
    function wireAll() { wireProc.command = ["zenith-agent", "wire"]; wireProc.running = true; }
    // Copy the paste snippet for an agent the daemon does not wire directly.
    function copySnippet() { Quickshell.execDetached(["sh", "-c", "zenith-agent paths --snippet | wl-copy"]); }
    // ── reusable poster parts ────────────────────────────────────────────────

    // a stamped-ink status chip: hairline box, tracked mono caps, tinted.
    component Stamp: Rectangle {
        id: stamp
        property string label: ""
        property color tint: hx.inkDim
        implicitWidth: stampT.implicitWidth + 18
        implicitHeight: 22
        color: "transparent"
        border.width: 1
        border.color: Qt.rgba(stamp.tint.r, stamp.tint.g, stamp.tint.b, 0.55)
        Text {
            id: stampT
            anchors.centerIn: parent
            text: I18n.tr(stamp.label)
            color: stamp.tint
            font.family: pg.fMono
            font.pixelSize: 9
            font.letterSpacing: 2
        }
    }

    // a section header: kanji + tracked caps title + hairline leader.
    component Head: Item {
        id: hd
        property string kanji: ""
        property string title: ""
        width: parent ? parent.width : 0
        height: 20
        Row {
            id: hdRow
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Text {
                text: hd.kanji; color: hx.red; font.family: pg.fJp
                font.pixelSize: 13; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: hd.title; color: hx.ink; font.family: pg.fMono
                font.pixelSize: 11; font.letterSpacing: 3; font.weight: Font.Medium
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Rectangle {
            anchors.left: hdRow.right; anchors.leftMargin: Tokens.s3
            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
            height: 1; color: hx.line
        }
    }

    // a function poster card: accent tab, index + kanji, name, one-liner, and an
    // optional live stat brightening to the accent.
    component FnCard: Rectangle {
        id: fc
        property string index: ""
        property string kanji: ""
        property string name: ""
        property string desc: ""
        property string stat: ""
        property color accent: hx.teal
        height: 128
        color: hx.paper2
        border.width: 1
        border.color: hx.line

        Rectangle { x: 0; y: 0; width: 34; height: 3; color: fc.accent }

        Column {
            anchors { left: parent.left; right: parent.right; top: parent.top; margins: 16 }
            spacing: 7
            Row {
                spacing: Tokens.s2
                Text {
                    text: fc.index; color: fc.accent; font.family: pg.fDisplay
                    font.pixelSize: 12; anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: fc.kanji; color: hx.inkDim; font.family: pg.fJp
                    font.pixelSize: 12; anchors.verticalCenter: parent.verticalCenter
                }
            }
            Text {
                text: fc.name; color: hx.ink; font.family: pg.fDisplay; font.pixelSize: 21
            }
            Text {
                width: parent.width
                text: I18n.tr(fc.desc); color: hx.inkDim; font.family: pg.fMono; font.pixelSize: 12
                wrapMode: Text.WordWrap; lineHeight: 1.25
            }
        }
        Text {
            visible: fc.stat !== ""
            anchors { right: parent.right; top: parent.top; margins: 16 }
            text: fc.stat; color: fc.accent; font.family: pg.fMono; font.pixelSize: 11
        }
    }

    // an example row: a mono command in red, then what it does.
    component TryRow: Row {
        id: tr
        property string cmd: ""
        property string note: ""
        width: parent ? parent.width : 0
        spacing: Tokens.s3
        Text {
            width: Math.round(tr.width * 0.34)
            text: tr.cmd; color: hx.red; font.family: pg.fMono; font.pixelSize: 13
            elide: Text.ElideRight
        }
        Text {
            width: tr.width - Math.round(tr.width * 0.34) - tr.spacing
            text: tr.note; color: hx.inkDim; font.family: pg.fMono; font.pixelSize: 13
            wrapMode: Text.WordWrap
        }
    }

    // a poster button: filled ink primary, hairline ghost otherwise.
    component PosterBtn: Item {
        id: bt
        property string label: ""
        property bool primary: false
        property bool on: true
        signal act()
        implicitWidth: btT.implicitWidth + Tokens.s5 * 2
        implicitHeight: 42
        opacity: bt.on ? 1 : 0.4
        Rectangle {
            anchors.fill: parent
            color: bt.primary ? hx.ink : "transparent"
            border.width: 1
            border.color: bt.primary ? hx.ink : hx.line
        }
        Text {
            id: btT
            anchors.centerIn: parent
            text: I18n.tr(bt.label); color: bt.primary ? hx.paper : hx.ink
            font.family: pg.fMono; font.pixelSize: 11; font.letterSpacing: 2; font.weight: Font.Medium
        }
        MouseArea {
            anchors.fill: parent; enabled: bt.on
            cursorShape: Qt.PointingHandCursor
            onClicked: bt.act()
        }
    }

    // one detected coding agent: name, wiring state, skill stamp, and a
    // one-click WIRE (inject the pointer + skill + prowl).
    component AgentRow: Rectangle {
        id: ar
        property var agent: ({})
        width: parent ? parent.width : 0
        height: 56
        color: hx.paper2
        border.width: 1
        border.color: (ar.agent.present && ar.agent.wired) ? Qt.rgba(hx.ink.r, hx.ink.g, hx.ink.b, 0.3) : hx.line
        opacity: ar.agent.present ? 1 : 0.5

        Column {
            anchors { left: parent.left; leftMargin: 16; right: arActions.left; rightMargin: 12; verticalCenter: parent.verticalCenter }
            spacing: 3
            Text {
                width: parent.width
                text: ar.agent.name || ar.agent.id || ""
                color: hx.ink; font.family: pg.fDisplay; font.pixelSize: 16; elide: Text.ElideRight
            }
            Text {
                width: parent.width
                text: !ar.agent.present ? I18n.tr("not installed")
                    : (ar.agent.wired ? I18n.tr("wired to the vault") : I18n.tr("present \u00b7 not wired yet"))
                color: hx.inkDim; font.family: pg.fMono; font.pixelSize: 11; elide: Text.ElideRight
            }
        }
        Row {
            id: arActions
            anchors { right: parent.right; rightMargin: 12; verticalCenter: parent.verticalCenter }
            spacing: Tokens.s2
            Stamp { visible: ar.agent.skillWired === true; label: "SKILL"; tint: hx.teal; anchors.verticalCenter: parent.verticalCenter }
            Rectangle {
                visible: ar.agent.present === true
                anchors.verticalCenter: parent.verticalCenter
                width: wbT.implicitWidth + Tokens.s4 * 2
                height: 30
                color: ar.agent.wired ? "transparent" : hx.ink
                border.width: 1
                border.color: ar.agent.wired ? hx.line : hx.ink
                Text {
                    id: wbT
                    anchors.centerIn: parent
                    text: ar.agent.wired ? I18n.tr("RE-WIRE") : I18n.tr("WIRE")
                    color: ar.agent.wired ? hx.ink : hx.paper
                    font.family: pg.fMono; font.pixelSize: 10; font.letterSpacing: 2; font.weight: Font.Medium
                }
                MouseArea { anchors.fill: parent; cursorShape: Qt.PointingHandCursor; onClicked: pg.wireAgent(ar.agent.id) }
            }
        }
    }

    // one manifest path: label + who owns it (generated/yours/read-only) + the
    // resolved location (dim when absent).
    component PathRow: Row {
        id: pr
        property string label: ""
        property string path: ""
        property string owner: ""
        property bool ok: true
        readonly property int labelW: Math.round(pr.width * 0.22)
        readonly property int ownerW: 76
        width: parent ? parent.width : 0
        spacing: Tokens.s3
        Text {
            width: pr.labelW
            text: pr.label; color: pr.ok ? hx.ink : hx.inkDim
            font.family: pg.fMono; font.pixelSize: 12; elide: Text.ElideRight
        }
        Text {
            width: pr.ownerW
            text: pr.owner
            color: pr.owner === "yours" ? hx.teal : hx.inkDim
            font.family: pg.fMono; font.pixelSize: 10; font.letterSpacing: 1
            anchors.verticalCenter: parent.verticalCenter
        }
        Text {
            width: pr.width - pr.labelW - pr.ownerW - pr.spacing * 2
            text: pr.path === "" ? I18n.tr("not installed") : pr.path
            color: pr.ok ? hx.tan : hx.inkDim
            font.family: pg.fMono; font.pixelSize: 12; elide: Text.ElideLeft
        }
    }

    // ── the page: warm paper, one scrolling poster column ────────────────────
    Rectangle { anchors.fill: parent; color: hx.paper }

    Flickable {
        id: flick
        anchors.fill: parent
        contentWidth: width
        contentHeight: body.height + Tokens.s6 * 2
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Column {
            id: body
            anchors.horizontalCenter: parent.horizontalCenter
            y: Tokens.s6
            width: pg.bodyW
            spacing: Tokens.s6

            // ── HERO: the samurai banner, title + live state overlaid ────────
            Item {
                width: parent.width
                height: Math.round(Math.min(width / 2.357, 320))
                clip: true

                Rectangle { anchors.fill: parent; color: hx.paper2 }
                Rectangle { anchors.fill: parent; color: "transparent"; border.width: 1; border.color: hx.line }

                // bottom scrim so the cap stays legible over the art.
                Rectangle {
                    anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
                    height: parent.height * 0.7
                    gradient: Gradient {
                        GradientStop { position: 0.0; color: "transparent" }
                        GradientStop { position: 1.0; color: Qt.rgba(hx.paper.r, hx.paper.g, hx.paper.b, 0.92) }
                    }
                }

                // title cap, bottom-left.
                Column {
                    anchors { left: parent.left; bottom: parent.bottom; leftMargin: Tokens.s5; bottomMargin: Tokens.s4 }
                    spacing: 3
                    Text {
                        text: I18n.tr("ZENITH AGENT"); color: hx.ink; font.family: pg.fDisplay
                        font.pixelSize: Math.round(Math.min(pg.bodyW * 0.05, 40))
                    }
                    Row {
                        spacing: Tokens.s2
                        Text {
                            text: "羅針"; color: hx.red; font.family: pg.fJp; font.pixelSize: 13
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        Text {
                            text: I18n.tr("THE RESIDENT ASSISTANT"); color: hx.inkDim; font.family: pg.fMono
                            font.pixelSize: 10; font.letterSpacing: 3
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }
                }

                // live state stamp, top-right.
                Stamp {
                    anchors { right: parent.right; top: parent.top; rightMargin: Tokens.s4; topMargin: Tokens.s4 }
                    label: !pg.installed ? I18n.tr("NOT INSTALLED")
                        : (pg.running ? I18n.tr("RUNNING") : (pg.daemonEnabled ? I18n.tr("STARTING") : I18n.tr("OFF")))
                    tint: pg.running ? hx.teal : (pg.installed ? hx.inkDim : hx.redDeep)
                }
            }

            // ── tagline ──────────────────────────────────────────────────────
            Text {
                width: parent.width
                text: I18n.tr("The optional local assistant. It keeps a living map of this machine - hardware, packages, every config beside the binary that owns it - so your coding agents read the terrain instead of rediscovering it. The agent engine that answers is yours to plug in.")
                color: hx.ink; font.family: pg.fMono; font.pixelSize: 14
                wrapMode: Text.WordWrap; lineHeight: 1.5
            }

            // ── MASTER SWITCH + MODEL: what is running, and the control ───────
            Row {
                width: parent.width
                spacing: Tokens.s4
                readonly property bool wide: pg.bodyW >= 640
                property real colW: wide ? (width - Tokens.s4) / 2 : width

                // service switch
                Rectangle {
                    width: parent.colW
                    height: 112
                    color: hx.paper2
                    border.width: 1
                    border.color: pg.daemonEnabled ? Qt.rgba(hx.ink.r, hx.ink.g, hx.ink.b, 0.35) : hx.line
                    opacity: pg.installed ? 1 : 0.5

                    Column {
                        anchors { left: parent.left; right: sw.left; verticalCenter: parent.verticalCenter; leftMargin: 18; rightMargin: 14 }
                        spacing: 6
                        Row {
                            spacing: Tokens.s2
                            Text { text: "羅針"; color: hx.ink; font.family: pg.fJp; font.pixelSize: 15; anchors.verticalCenter: parent.verticalCenter }
                            Text {
                                text: I18n.tr("AGENT SERVICE"); color: hx.ink; font.family: pg.fDisplay
                                font.pixelSize: 13; anchors.verticalCenter: parent.verticalCenter
                            }
                        }
                        Text {
                            width: parent.width
                            text: pg.installed
                                ? (pg.running ? I18n.tr("Running \u00b7 127.0.0.1:%1").arg(pg.port)
                                   : (pg.daemonEnabled ? I18n.tr("Enabled \u00b7 starting\u2026") : I18n.tr("Off \u00b7 switch on to start it with the desktop")))
                                : I18n.tr("Not installed \u00b7 install zenith-agent")
                            color: hx.inkDim; font.family: pg.fMono; font.pixelSize: 11; elide: Text.ElideRight
                        }
                    }
                    Sw {
                        id: sw
                        anchors { right: parent.right; rightMargin: 16; verticalCenter: parent.verticalCenter }
                        enabled: pg.installed
                        on: pg.daemonEnabled
                        onToggled: (c) => { pg.daemonEnabled = c; pg.setEnabled(c); }
                    }
                }

                // the agent engine -- the headline fact, and the plain sentence when there is none.
                Rectangle {
                    width: parent.colW
                    height: 112
                    color: hx.paper2
                    border.width: 1
                    border.color: hx.line
                    Rectangle { x: 0; y: 0; width: 34; height: 3; color: hx.red }

                    Column {
                        anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter; leftMargin: 18; rightMargin: 18 }
                        spacing: 4
                        Row {
                            spacing: Tokens.s2
                            Text { text: "機関"; color: hx.red; font.family: pg.fJp; font.pixelSize: 12; anchors.verticalCenter: parent.verticalCenter }
                            Text {
                                text: I18n.tr("ENGINE"); color: hx.inkDim; font.family: pg.fMono
                                font.pixelSize: 10; font.letterSpacing: 3; anchors.verticalCenter: parent.verticalCenter
                            }
                        }
                        Text {
                            width: parent.width
                            text: pg.engineConfigured ? pg.engineCommand.split(" ")[0].split("/").pop() : "-"
                            color: hx.ink; font.family: pg.fDisplay
                            font.pixelSize: pg.engineConfigured ? 30 : 26
                            elide: Text.ElideRight
                        }
                        Text {
                            width: parent.width
                            text: pg.engineAvailable ? pg.engineCommand
                                : (pg.engineReason !== "" ? pg.engineReason : I18n.tr("no agent engine configured"))
                            color: hx.inkDim; font.family: pg.fMono; font.pixelSize: 11
                            wrapMode: Text.WordWrap; maximumLineCount: 3; elide: Text.ElideRight
                        }
                    }
                }
            }

            // ── FUNCTIONS: the numbered poster index of what it does ──────
            Column {
                width: parent.width
                spacing: Tokens.s4
                Head { kanji: "\u6a5f\u80fd"; title: I18n.tr("FUNCTIONS") }

                Grid {
                    id: fnGrid
                    width: parent.width
                    columns: pg.bodyW >= 860 ? 3 : (pg.bodyW >= 560 ? 2 : 1)
                    columnSpacing: Tokens.s4
                    rowSpacing: Tokens.s4
                    readonly property real cellW: (width - columnSpacing * (columns - 1)) / columns

                    FnCard {
                        width: fnGrid.cellW; index: "01"; kanji: "\u66f8\u5eab"; name: I18n.tr("VAULT"); accent: hx.teal
                        desc: I18n.tr("The living map your agents read, beside each binary.")
                        stat: pg.vaultExists ? I18n.tr("%1 files").arg(pg.vaultFiles) : ""
                    }
                    FnCard {
                        width: fnGrid.cellW; index: "02"; kanji: "\u8a18\u61b6"; name: I18n.tr("MEMORY"); accent: hx.orange
                        desc: I18n.tr("Notes the agent keeps in the vault, across every session.")
                    }
                    FnCard {
                        width: fnGrid.cellW; index: "03"; kanji: "\u6280"; name: I18n.tr("SKILLS"); accent: hx.slate
                        desc: I18n.tr("Skills an agent can load: how to change this desktop safely.")
                    }
                    FnCard {
                        width: fnGrid.cellW; index: "04"; kanji: "\u4e94\u4eba\u8846"; name: I18n.tr("AGENTS"); accent: hx.tan
                        desc: I18n.tr("Your coding agents, wired to one shared map of the machine.")
                        stat: pg.agentsPresent > 0 ? pg.wiredSummary : ""
                    }
                    FnCard {
                        width: fnGrid.cellW; index: "05"; kanji: "\u5bfe\u8a71"; name: I18n.tr("CHAT"); accent: hx.red
                        desc: I18n.tr("Talk to your engine in the sidebar, the dashboard or a terminal.")
                    }
                    FnCard {
                        width: fnGrid.cellW; index: "06"; kanji: "\u7f85\u91dd"; name: I18n.tr("CODE"); accent: hx.teal
                        desc: I18n.tr("Code intelligence: cited answers over your repos.")
                    }
                }
            }

            // ── YOUR AGENTS: one-click wire any coding CLI to the vault ───────
            Column {
                width: parent.width
                spacing: Tokens.s4
                Head { kanji: "\u4e94\u4eba\u8846"; title: I18n.tr("YOUR AGENTS") }
                Text {
                    width: parent.width
                    text: I18n.tr("Wire any coding agent to the same living map of this machine. One click drops a pointer into its instructions, links the zenith skill, and installs prowl's code-intelligence skill.")
                    color: hx.inkDim; font.family: pg.fMono; font.pixelSize: 12; wrapMode: Text.WordWrap; lineHeight: 1.4
                }
                Column {
                    width: parent.width
                    spacing: Tokens.s2
                    Repeater {
                        model: pg.agentList
                        delegate: AgentRow {
                            required property var modelData
                            width: parent.width
                            agent: modelData
                        }
                    }
                }
                PosterBtn { label: I18n.tr("WIRE ALL PRESENT"); on: pg.installed; onAct: pg.wireAll() }
            }

            // ── POINT ANY AGENT: the paths, and a paste snippet for the rest ──
            Column {
                width: parent.width
                spacing: Tokens.s4
                Head { kanji: "\u9023\u643a"; title: I18n.tr("POINT ANY AGENT") }
                Text {
                    width: parent.width
                    text: I18n.tr("On an agent that isn't wired for you? Point it at these yourself, or copy the ready-made instructions and paste them into its config.")
                    color: hx.inkDim; font.family: pg.fMono; font.pixelSize: 12; wrapMode: Text.WordWrap; lineHeight: 1.4
                }
                Column {
                    width: parent.width
                    spacing: Tokens.s2
                    PathRow { label: I18n.tr("skill"); path: pg.skillPath; owner: I18n.tr("read-only"); ok: pg.skillPath !== "" }
                    PathRow { label: I18n.tr("prowl"); path: pg.prowlPath; owner: I18n.tr("tool"); ok: pg.prowlPath !== "" }
                    Repeater {
                        model: pg.vaultItems
                        delegate: PathRow {
                            required property var modelData
                            width: parent.width
                            label: modelData.label
                            path: modelData.path
                            owner: modelData.owner
                            ok: modelData.exists === true
                        }
                    }
                }
                PosterBtn { label: I18n.tr("COPY AGENT SNIPPET"); primary: true; on: pg.installed; onAct: pg.copySnippet() }
            }

            // ── ENGINE: the one config key, and what it holds now ─────────────
            Column {
                width: parent.width
                spacing: Tokens.s3
                Head { kanji: "\u6a5f\u95a2"; title: I18n.tr("ENGINE") }
                Text {
                    width: parent.width
                    text: I18n.tr("The agent that answers the chat is yours to plug in: any program that speaks the Agent Client Protocol over stdio. Nothing is installed or guessed; the daemon starts it the first time the chat is used.")
                    color: hx.inkDim; font.family: pg.fMono; font.pixelSize: 12; wrapMode: Text.WordWrap; lineHeight: 1.4
                }
                Text {
                    width: parent.width
                    text: pg.engineAvailable ? pg.engineCommand
                        : (pg.engineReason !== "" ? pg.engineReason : I18n.tr("no agent engine configured"))
                    color: hx.ink; font.family: pg.fMono; font.pixelSize: 12; wrapMode: Text.WordWrap
                }
                Text {
                    width: parent.width
                    text: pg.engineConfig + "\n{ \"engine\": { \"command\": [\"your-agent\", \"acp\"] } }"
                    color: hx.inkDim; font.family: pg.fMono; font.pixelSize: 11; wrapMode: Text.WordWrap
                }
            }

            // ── TRY: example uses ────────────────────────────────────────────
            Column {
                width: parent.width
                spacing: Tokens.s3
                Head { kanji: "\u4f8b"; title: I18n.tr("TRY") }
                TryRow { cmd: "zenith-agent status"; note: I18n.tr("the daemon, the engine and the vault at a glance") }
                TryRow { cmd: "zenith-agent setup"; note: I18n.tr("where the engine is set, and how") }
                TryRow { cmd: "zenith-agent paths"; note: I18n.tr("every path an agent can read") }
                TryRow { cmd: "prowl overview"; note: I18n.tr("code intelligence on any repo") }
            }

            // ── SET UP + DASHBOARD: the controls ─────────────────────────────
            Column {
                width: parent.width
                spacing: Tokens.s4
                Head { kanji: "\u8d77\u52d5"; title: I18n.tr("GET STARTED") }

                Text {
                    width: parent.width
                    text: I18n.tr("Switch the service on, then set the engine key to your agent. Until there is one, the assistant says that none is configured and does nothing else.")
                    color: hx.inkDim; font.family: pg.fMono; font.pixelSize: 12; wrapMode: Text.WordWrap; lineHeight: 1.4
                }

                PosterBtn {
                    label: I18n.tr("OPEN DASHBOARD")
                    primary: pg.engineAvailable
                    on: pg.installed && pg.daemonEnabled
                    onAct: pg.openDashboard()
                }
            }

            // ── privacy footer ───────────────────────────────────────────────
            Item { width: parent.width; height: Tokens.s2 }
            Row {
                width: parent.width
                spacing: Tokens.s3
                Rectangle { width: 8; height: 8; radius: 4; color: hx.red; anchors.verticalCenter: parent.verticalCenter }
                Text {
                    width: parent.width - Tokens.s3 - 8
                    text: I18n.tr("The daemon serves this machine only: a private socket and 127.0.0.1. Where your engine sends its questions is up to the engine.")
                    color: hx.inkDim; font.family: pg.fMono; font.pixelSize: 11; wrapMode: Text.WordWrap
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
        }
    }
}
