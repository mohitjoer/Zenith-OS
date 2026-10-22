pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

// QML view of the daemon `osd` topic. The volume and mic OSDs read PipeWire
// directly (Zenith owns the audio graph in QML), but brightness has no
// QML-observable source: the media keys and the brightness verb both drive the
// backlight directly, so the daemon watches the primary backlight and pushes
// {brightness:{seq,value}} here. The brightness OSD shows when `brightnessSeq`
// advances and binds `brightness` (0..1) to its bar. Contract 12 sec 3, sec 9.
//
// Fallback (this lane): when the daemon is absent (no frames after ~10 s), read
// current/max from /sys/class/backlight/<first>/ directly so the OSD and the
// bar widget still show live values. Daemon feed remains first-class when present.
Singleton {
    id: root

    property real brightness: 0
    property int brightnessSeq: 0
    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/zenith-shell.sock"

    // --- sysfs fallback -------------------------------------------------------
    property string blDevice: ""
    property string blPath: ""
    property int sysfsBrightness: 0
    property int sysfsMaxBrightness: 0
    property bool sysfsValid: false
    property bool daemonSeen: false
    property int retryCount: 0

    function apply(line) {
        try {
            const b = JSON.parse(line).brightness;
            if (b) {
                if (typeof b.value === "number")
                    root.brightness = Math.max(0, Math.min(1, b.value));
                if (typeof b.seq === "number")
                    root.brightnessSeq = b.seq;
                if (!root.daemonSeen)
                    root.daemonSeen = true;
            }
        } catch (e) {
            // A malformed frame must never wedge the OSD; keep the last value.
        }
    }

    function refreshSysfs() {
        if (!root.blPath || root.daemonSeen)
            return;
        briFile.reload();
        maxFile.reload();
    }

    function onBriLoaded() {
        var cur = parseInt((briFile.text() || "").trim(), 10);
        var max = parseInt((maxFile.text() || "").trim(), 10);
        if (!isNaN(cur) && !isNaN(max) && max > 0) {
            root.sysfsBrightness = cur;
            root.sysfsMaxBrightness = max;
            root.sysfsValid = true;
            root.brightness = Math.max(0, Math.min(1, cur / max));
        } else {
            root.sysfsValid = false;
        }
    }

    // Detect the first backlight device on startup.
    Process {
        id: detectBl
        command: ["bash", "-c", "ls -1 /sys/class/backlight 2>/dev/null | head -n1"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                var bl = this.text.trim()
                if (bl !== "") {
                    root.blDevice = bl;
                    root.blPath = "/sys/class/backlight/" + bl;
                    // Trigger initial read
                    briFile.path = root.blPath + "/brightness";
                    maxFile.path = root.blPath + "/max_brightness";
                    root.refreshSysfs();
                }
            }
        }
    }

    FileView {
        id: briFile
        path: ""
        blockLoading: true
        printErrors: false
        onLoaded: root.onBriLoaded()
    }
    FileView {
        id: maxFile
        path: ""
        blockLoading: true
        printErrors: false
        onLoaded: root.onBriLoaded()
    }

    // Subscribe once for the whole shell; every screen's brightness OSD reads
    // this singleton rather than opening its own socket.
    Socket {
        id: sub
        path: root.sockPath
        parser: SplitParser { onRead: line => root.apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe osd\n");
                flush();
            } else {
                retry.restart();
            }
        }
    }

    // The daemon may be down at load or restart under the shell; retry quietly.
    // Count retries so we can delay sysfs fallback until ~10 s (5 retries @ 2 s).
    Timer {
        id: retry
        interval: 2000
        onTriggered: {
            if (!sub.connected) {
                if (root.retryCount < 5)
                    root.retryCount++;
                sub.connected = true;
            }
        }
    }

    // Daemon-absence detector: if no frame arrived after ~10 s (5 retries),
    // start polling sysfs every 2 s. Stops automatically when a frame arrives.
    Timer {
        id: fallbackTimer
        interval: 2000
        running: !root.daemonSeen && root.blPath !== "" && root.retryCount >= 5
        repeat: true
        triggeredOnStart: true
        onTriggered: {
            if (!root.daemonSeen)
                root.refreshSysfs();
        }
    }

    // Keep sysfs fresh while daemon is absent (OSD and bar both read `brightness`).
    Timer {
        id: sysfsPoll
        interval: 2000
        running: root.sysfsValid && !root.daemonSeen
        repeat: true
        onTriggered: root.refreshSysfs()
    }
}