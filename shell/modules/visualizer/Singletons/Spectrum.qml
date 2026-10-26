pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import qs.Services

// high-resolution playback spectrum for the desktop visualiser. mirrors the
// pill's AudioBars but reads the PipeWire playback monitor at 64 bands / 60fps
// so the whole desktop sweep stays smooth. `analysing` gates the cava process;
// levels settle to a flat rest when cava stops emitting (system silent, or a
// restart gap) so the spectrum never freezes on the last peak.
//
// It mirrors AudioBars' policy as well as its shape, which it previously did
// not: `running` hung off surface visibility alone, so this analyser ignored the
// visualiser's own freeze switch, Power Saver, lowPowerMode and Game Mode, and
// kept a second cava alive beside the pill's whenever the surface existed. Both
// were measured running for hours against silence.
Singleton {
    id: root

    property bool active: false
    property int bars: 64
    property int fps: 30
    // Instances being placed (Super+Alt+M) hold this up so the spectrum keeps
    // running while it is aimed: you cannot position a frozen, invisible line.
    property int placementHolds: 0

    // 0..1 per band (length == bars) + mean energy across all bands.
    property var levels: root.flat(0.02)
    property real energy: 0
    property real lastReadMs: 0

    function flat(v) {
        var a = [];
        for (var i = 0; i < root.bars; i++)
            a.push(v);
        return a;
    }

    readonly property bool analysing: false

    Process {
        id: cavaProc
        command: ["true"]
        running: false
        property bool backoff: false
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: (line) => root.readBars(line)
        }
        onExited: if (root.analysing) {
            cavaProc.backoff = true;
            restartTimer.restart();
        }
    }

    Timer {
        id: restartTimer
        interval: 1200
        onTriggered: cavaProc.backoff = false
    }

    // cava reads its band count and framerate once, at startup, so changing
    // either has to cycle the process. Closing the backoff window is all this
    // does; whether cava actually comes back is still the binding's call.
    Timer {
        id: barsRestart
        interval: 300
        onTriggered: cavaProc.backoff = false
    }

    // settle to a flat resting line when no frame has arrived in a bit.
    Timer {
        interval: 120
        running: root.analysing
        repeat: true
        onTriggered: if (Date.now() - root.lastReadMs > 260) {
            root.levels = root.flat(0.02);
            root.energy = 0;
        }
    }

    onActiveChanged: {
        levels = flat(0.02);
        energy = 0;
        if (active)
            lastReadMs = 0;
    }

    // The settle Timer above only runs while analysing, so when analysis stops the
    // last frame's levels would otherwise stay put forever. Motion reads those
    // levels to decide whether it is "sounding", so stale peaks kept the whole
    // visualiser animating long after the music ended: measured at 6% of a core
    // for the shell, indefinitely, with the analyser already gone. Flatten on the
    // way down, exactly as AudioBars does.
    onAnalysingChanged: {
        levels = flat(0.02);
        energy = 0;
        if (analysing)
            lastReadMs = 0;
    }

    onBarsChanged: {
        levels = flat(0.02);
        cavaProc.backoff = true;
        barsRestart.restart();
    }

    onFpsChanged: {
        cavaProc.backoff = true;
        barsRestart.restart();
    }

    function norm(v) {
        var n = parseInt(v);
        if (isNaN(n))
            return 0;
        return Math.max(0, Math.min(1, n / 100));
    }

    function readBars(line) {
        var t = line.trim();
        if (!t)
            return;
        var parts = t.split(/[;\s]+/);
        if (parts.length < root.bars)
            return;
        var out = [];
        var sum = 0;
        for (var i = 0; i < root.bars; i++) {
            var v = root.norm(parts[i]);
            out.push(v);
            sum += v;
        }
        root.levels = out;
        root.energy = sum / root.bars;
        root.lastReadMs = Date.now();
    }
}
