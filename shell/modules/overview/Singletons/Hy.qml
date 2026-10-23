pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Hyprland
import qs.Ui.Singletons

/**
 * The overview's own Hyprland feed.
 *
 * The shell's Wm is capability-shaped around the daemon: caps, window geometry,
 * the workspace join and the focused output all arrive through the
 * $XDG_RUNTIME_DIR/zenith-shell.sock `wm` topic. On a machine where that daemon
 * is absent (a dev box, a rescue boot) the socket never binds, every cap reads
 * false, and the shell's own toggle turns into a silent no-op -- the overview
 * could never open anywhere. This singleton pulls the same facts straight from
 * the compositor (`hyprctl -j`) so the expo still runs daemon-less: window
 * boxes, per-output geometry/scale, workspace occupancy, and one-shot dispatch
 * actions (focus workspace / focus / close / move window). Only the thumbnail
 * capture stays on the shell's foreign-toplevel handles (Wm.toplevels), joined
 * by Hyprland address, because screencopy is a Wayland feature, not a daemon
 * one.
 *
 * Shapes mirror Wm so Overview and WorkspaceCell rebind without caring which
 * feed answered (Wm keeps priority wherever the daemon is publishing; see
 * Overview.daemonFed). Same pattern as the installed shell's
 * services/HyprlandData.qml: three hyprctl -j reads, refreshed on every
 * compositor event (debounced), nothing polled on a timer.
 */
Singleton {
    id: root

    // ---- raw caches -----------------------------------------------------------
    property var _clients: []
    property var _monitors: []
    property var _workspaces: []
    // "a reply ever landed" flags, not "the model is fresh" ones: `live` is the
    // gate the surface reads, so a failed read (no compositor socket, no
    // hyprctl on PATH) leaves the overview hidden instead of showing empty cells.
    property bool _clientsOk: false
    property bool _monitorsOk: false
    property bool _workspacesOk: false
    property bool _warned: false

    // True once the compositor's geometry answered. Enough to place and capture
    // every preview cell, which is all the overview needs.
    readonly property bool live: root._clientsOk && root._monitorsOk

    // Super+Alt+Tab, forwarded by shell.qml's overviewDesktop(). An overview that
    // is up steps its desktop strip on desktopStep; one that is still closed is
    // opened with pendingStep set and applies it once it has seeded.
    signal desktopStep(int delta)
    property int pendingStep: 0

    // ---- window boxes, Wm.windows shape ---------------------------------------
    // hyprctl clients keys windows by monitor ID; the module keys on output
    // NAME (matching Quickshell.screens), so resolve the id -> name once per read.
    function _monitorNameById() {
        const out = ({});
        for (let i = 0; i < root._monitors.length; i++) {
            const m = root._monitors[i];
            if (m)
                out[m.id] = m.name;
        }
        return out;
    }

    // Foreign-toplevel handles, keyed by the bare Hyprland address
    // ("59048b8781f0"). `hyprctl clients` addresses carry the "0x" prefix, the
    // attached property does not, so both sides are stripped once here.
    function _stripAddr(a) {
        return String(a || "").toLowerCase().replace(/^0x/, "");
    }

    readonly property var windows: {
        const byAddr = ({});
        const tls = Wm.toplevels;
        for (let i = 0; i < tls.length; i++) {
            const t = tls[i];
            if (!t || !t.HyprlandToplevel)
                continue;
            const addr = root._stripAddr(t.HyprlandToplevel.address);
            if (addr)
                byAddr[addr] = t;
        }
        const monName = root._monitorNameById();
        const cs = root._clients;
        const out = [];
        for (let j = 0; j < cs.length; j++) {
            const c = cs[j];
            if (!c || c.mapped === false || c.hidden === true)
                continue;
            // special workspaces carry id 0 / negative ids in the client JSON;
            // the desk model counts real workspaces only, so skip them here.
            if (!c.workspace || !(c.workspace.id > 0))
                continue;
            const addr = root._stripAddr(c.address);
            out.push({
                id: addr,
                appId: c.class || "",
                title: c.title || "",
                workspace: String(c.workspace.id),
                output: monName[Number(c.monitor)] || "",
                focusOrder: (typeof c.focusHistoryID === "number") ? c.focusHistoryID : -1,
                floating: c.floating === true,
                x: c.at ? c.at[0] : 0,
                y: c.at ? c.at[1] : 0,
                width: c.size ? c.size[0] : 0,
                height: c.size ? c.size[1] : 0,
                toplevel: byAddr[addr] || null
            });
        }
        return out;
    }

    // ---- outputs, Wm.outputs shape --------------------------------------------
    readonly property var outputs: {
        const ms = root._monitors;
        const out = [];
        for (let i = 0; i < ms.length; i++) {
            const m = ms[i];
            if (!m || m.disabled === true)
                continue;
            out.push({
                name: m.name || "",
                scale: (m.scale > 0) ? m.scale : 1,
                width: m.width || 0,
                height: m.height || 0,
                activeWorkspace: (m.activeWorkspace && m.activeWorkspace.name !== undefined) ? String(m.activeWorkspace.name) : "",
                transform: m.transform || 0
            });
        }
        return out;
    }

    function outputByName(name) {
        const list = root.outputs;
        for (let i = 0; i < list.length; i++)
            if (list[i].name === name)
                return list[i];
        return null;
    }

    // ---- workspaces, Wm.workspaces shape --------------------------------------
    // `hyprctl workspaces` lists only workspaces that exist (visited or
    // occupied); the desk model already treats absent ids as create-on-click, so
    // the filmstrip keeps rendering every SLOT 1..N in order regardless.
    readonly property var workspaces: {
        // a shown special workspace leaves the normal activeWorkspace on the
        // monitor unchanged; the desk model ignores special workspaces either
        // way, so `active` follows the normal activeWorkspace id.
        const activeId = ({});
        for (let j = 0; j < root._monitors.length; j++) {
            const m = root._monitors[j];
            if (m && m.activeWorkspace)
                activeId[m.name] = m.activeWorkspace.id;
        }
        const ws = root._workspaces;
        const out = [];
        for (let i = 0; i < ws.length; i++) {
            const w = ws[i];
            if (!w || !(w.id > 0))
                continue;
            out.push({
                name: String(w.id),
                active: activeId[w.monitor] === w.id,
                urgent: false,
                canActivate: true,
                windows: w.windows || 0,
                occupied: (w.windows || 0) > 0,
                fullscreen: w.hasfullscreen === true,
                special: false,
                output: w.monitor || "",
                layout: ""
            });
        }
        return out;
    }

    function workspaceByName(name) {
        const key = String(name);
        const list = root.workspaces;
        for (let i = 0; i < list.length; i++)
            if (list[i].name === key)
                return list[i];
        return null;
    }

    // ---- reads ----------------------------------------------------------------
    // One refresh per event-burst, not per event: Hyprland lands bursts on
    // openwindow/focus, and three hyprctl spawns per event is the expensive
    // shape the debounce below avoids.
    function refresh() { refreshTimer.restart(); }

    function refreshNow() {
        getClients.running = true;
        getMonitors.running = true;
        getWorkspaces.running = true;
    }

    Timer {
        id: refreshTimer
        interval: 60
        onTriggered: root.refreshNow()
    }

    // The events that never change any of the three reads. The same skips the
    // installed shell's feed makes; layer churn and screencast status are the
    // loud ones.
    Connections {
        target: Hyprland
        function onRawEvent(event) {
            const n = event.name;
            if (n === "openlayer" || n === "closelayer" || n === "screencast")
                return;
            root.refresh();
        }
    }

    Component.onCompleted: root.refreshNow()

    Process {
        id: getClients
        command: ["hyprctl", "clients", "-j"]
        stdout: StdioCollector {
            onStreamFinished: {
                // Exit codes are not settled when the stream closes (the exit
                // signal lands after this collector), so the guard is content,
                // not exit code: `exitedWithCode` is still undefined here.
                if (text.trim() === "")
                    return root.noteFailure("clients");
                try {
                    root._clients = JSON.parse(text) || [];
                } catch (e) {
                    return root.noteFailure("clients");
                }
                root._clientsOk = true;
            }
        }
    }
    Process {
        id: getMonitors
        command: ["hyprctl", "monitors", "-j"]
        stdout: StdioCollector {
            onStreamFinished: {
                if (text.trim() === "")
                    return root.noteFailure("monitors");
                try {
                    root._monitors = JSON.parse(text) || [];
                } catch (e) {
                    return root.noteFailure("monitors");
                }
                root._monitorsOk = true;
            }
        }
    }
    Process {
        id: getWorkspaces
        command: ["hyprctl", "workspaces", "-j"]
        stdout: StdioCollector {
            onStreamFinished: {
                if (text.trim() === "")
                    return root.noteFailure("workspaces");
                try {
                    root._workspaces = JSON.parse(text) || [];
                } catch (e) {
                    return root.noteFailure("workspaces");
                }
                root._workspacesOk = true;
            }
        }
    }

    // One line, once: a feed that never comes up is a feed worth grepping for,
    // but a respawned attempt every event is not.
    function noteFailure(kind) {
        if (root._warned)
            return;
        root._warned = true;
        console.warn("overview: hyprctl feed could not read " + kind + "; the overview stays hidden");
    }

    // ---- actions --------------------------------------------------------------
    // One dispatcher process per commit, re-armed per call, so a whole click
    // sequence (close tile, then switch workspace after the close) runs in
    // order. Dispatch strings are the compositor binds' own syntax; Switcher.qml
    // rides the same `dispatch ... address:` contract, and the same 0.55 note
    // about wrapping then applies.
    property var _cmd: []
    Process {
        id: dispatch
        command: root._cmd
    }

    function runAct(args) {
        // a change to `command` while running restarts the process, but restart
        // it explicitly so back-to-back actions never race the re-arm.
        root._cmd = args;
        dispatch.running = false;
        dispatch.running = true;
    }

    function _addrOf(id) {
        const a = root._stripAddr(id);
        return a ? "address:0x" + a : "";
    }

    function focusWorkspace(ws) {
        const id = Number(ws);
        if (!(id > 0))
            return;
        root.runAct(["hyprctl", "dispatch", "workspace", String(id)]);
    }
    function focusWindow(id) {
        const a = root._addrOf(id);
        if (!a)
            return;
        root.runAct(["hyprctl", "dispatch", "focuswindow", a]);
    }
    function closeWindow(id) {
        const a = root._addrOf(id);
        if (!a)
            return;
        root.runAct(["hyprctl", "dispatch", "closewindow", a]);
    }
    function moveWindowToWorkspace(id, ws) {
        const a = root._addrOf(id);
        const wid = Number(ws);
        if (!a || !(wid > 0))
            return;
        root.runAct(["hyprctl", "dispatch", "movetoworkspacesilent", wid + "," + a]);
    }
}
