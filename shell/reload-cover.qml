pragma ComponentBehavior: Bound

// The reload cover's window: a black curtain that closes over every output before
// the shell restarts and opens again once it is back.
//
// Ported from the reference desktop's reload cover. The entry file sits at the shell's
// root and the window's own code is in `reload-cover/`, for the reason `hub.qml`
// gives: a Quickshell configuration's root is the directory of the file it is
// started on, and the `qs.*` imports the cover draws with resolve only from the
// shell's root. It is started by `scripts/zenith-reload-cover begin`, which hands
// it a token in the environment; nothing else should start it.
//
// The handshake is three calls on the `reload-cover` IPC target, every one of
// them carrying the token so that a stale cover and a fresh one cannot be
// mistaken for each other:
//
//   mapped / covered   the curtain is up on every output. `begin` waits for
//                      this, and only then lets the reload go ahead.
//   finish             the shell is back. The curtain opens and the process ends.
//   fail               the reload went wrong. The curtain says so and opens.
//
// The shell calls `finish` itself (shell.qml, `reloadHold`), so the daemon never
// has to hold the token. If nothing answers at all the cover gives up on its
// own after sixteen and a half seconds and shows the failure, because a curtain
// that stays down on a shell that never came back hides the one thing the
// user needs to see.

import QtQuick
import Quickshell
import Quickshell.Io
import "reload-cover"

ShellRoot {
    id: root

    readonly property string token: Quickshell.env("ZENITH_RELOAD_COVER_TOKEN") || ""
    property string phase: "closing"
    property bool startClose: false
    property bool finishQueued: false
    property int mappedCount: 0
    property var mappedOutputs: ({})
    readonly property string brandPath: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/zenith/brand.json"
    readonly property var reloadCover: brandAdapter.reloadCover || ({ path: "", name: "", kind: "default", bytes: 0 })

    // The media the Hub's Reload cover page chose, if any. Read once: the cover
    // lives for a second or two, and a descriptor changing under it would be a
    // curtain that swaps its picture halfway down.
    FileView {
        path: root.brandPath
        blockLoading: true
        printErrors: false
        JsonAdapter {
            id: brandAdapter
            property var reloadCover: ({ path: "", name: "", kind: "default", bytes: 0 })
        }
    }

    function mapped(name: string): void {
        if (!mappedOutputs[name]) {
            mappedOutputs[name] = true;
            mappedCount += 1;
        }
        if (mappedCount >= Quickshell.screens.length)
            startClose = true;
    }
    function finish(value: string): bool {
        if (value !== token || phase === "opening" || phase === "failed")
            return false;
        if (phase === "hold")
            phase = "opening";
        else
            finishQueued = true;
        return true;
    }
    function fail(value: string): bool {
        if (value !== token || phase === "opening")
            return false;
        phase = "failed";
        return true;
    }

    // The iris takes 520 ms to close. Holding the phase until then is what makes
    // `covered` true only when no pixel of the old desktop is still showing.
    Timer {
        interval: 520
        running: root.startClose && root.phase === "closing"
        onTriggered: {
            root.phase = "hold";
            if (root.finishQueued)
                root.phase = "opening";
        }
    }
    Timer {
        interval: 16500
        running: root.phase !== "opening" && root.phase !== "failed"
        onTriggered: root.phase = "failed"
    }
    Timer {
        interval: 1800
        running: root.phase === "failed"
        onTriggered: root.phase = "opening"
    }
    Timer {
        interval: 560
        running: root.phase === "opening"
        onTriggered: Qt.quit()
    }

    IpcHandler {
        target: "reload-cover"
        function mapped(value: string): bool {
            return value === root.token && root.mappedCount >= Quickshell.screens.length;
        }
        function covered(value: string): bool { return value === root.token && root.phase === "hold"; }
        function status(value: string): string { return value === root.token ? root.phase : ""; }
        function finish(value: string): bool { return root.finish(value); }
        function fail(value: string): bool { return root.fail(value); }
    }

    Variants {
        model: Quickshell.screens
        delegate: Component {
            ReloadCover {
                required property var modelData
                targetScreen: modelData
                phase: root.phase
                startClose: root.startClose
                reloadCover: root.reloadCover
                onMapped: root.mapped(targetScreen.name)
            }
        }
    }
}
