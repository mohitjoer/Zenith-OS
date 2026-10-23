pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import qs.Ui.Singletons

// Quick-capture controller for the frame's capture card: pick a delay, a save
// target and a mode, take the shot and be done. The delay / save choices persist
// (capture.json) so the card remembers them across sessions, shared across every
// daemon that reads this singleton. The annotating editor is a separate tool,
// zenith-shot, bound to its own keys.
//
// Screen RECORDING is deliberately NOT owned here. The card's record zone drives
// the existing Recorder singleton (gpu-screen-recorder with a wf-recorder
// fallback, the record island, Studio, Discord, camera). The only recording
// thing this file does is hand a picked target (monitor, window or region) to
// Recorder.start.
//
// Divergence: the capture backend shells out to grim + wl-copy rather than a
// bespoke zwlr_screencopy_v1 client -- grim speaks exactly that protocol, and
// Zenith already shells out to wl-copy / wf-recorder for the same reasons.
Singleton {
    id: root

    // "" while idle; otherwise "region" | "monitor" | "window" naming the active
    // selection-overlay family. Every output's overlay is mapped while this is
    // set, and a commit or Escape on any one clears it, tearing them all down.
    property string selecting: ""

    // Remembered card options, persisted to capture.json (see the FileView below).
    property alias delay: prefs.delay          // seconds: 0 | 1 | 3 | 5 | 10
    property alias save: prefs.save            // "both" | "clipboard" | "file"
    property alias beautify: prefs.beautify    // stored only: the editor has no beautify step to open

    // What the current selection is for: "shot" runs grim, "record" hands the
    // region to Recorder. Set before `selecting`.
    property string _purpose: "shot"
    property int _delayMs: 0
    property string _save: "both"          // save mode latched for the pending shot
    property string _outPath: ""           // resolved PNG path ("" = clipboard-only stream)
    property var _recordAudio: []          // extra Recorder args for a targeted record
    property var _pending: null            // { flag, val } grim target for the pending shot

    // Zenith owns its screenshot location; the filename pattern is the reference
    // UTC stamp. XDG_PICTURES_DIR honoured, else ~/Pictures, matching zenith-shot.
    readonly property string shotsDir: (Quickshell.env("XDG_PICTURES_DIR") || (Quickshell.env("HOME") + "/Pictures")) + "/Screenshots"

    // Persisted card options, shared across daemons and surviving a restart. Same
    // shape as Flags: watch for outside edits, write back on any change, seed only
    // on a real first run so a slow load never clobbers a present file.
    FileView {
        id: prefsFile
        path: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/zenith/capture.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: reload()
        onAdapterUpdated: writeAdapter()
        JsonAdapter {
            id: prefs
            property int delay: 0
            property string save: "both"
            property bool beautify: false
        }
    }
    Component.onCompleted: if (!prefsFile.text()) prefsFile.writeAdapter();

    // Screenshot entry from the card. mode: "all" | "monitor" | "window" | "region".
    // "all" captures immediately after a floor delay (500ms minimum so the card is
    // gone from the frame before the whole desktop is grabbed). The other three
    // raise the selection overlays and apply the delay AFTER the selection.
    function shoot(mode) {
        root._purpose = "shot";
        root._save = root.save;
        if (mode === "all") {
            root._pending = { flag: "", val: "" };
            fireTimer.interval = Math.max(root.delay * 1000, 500);
            fireTimer.restart();
        } else {
            root._delayMs = root.delay * 1000;
            root.selecting = mode;
        }
    }

    // Record-target entry from the card: raise the same selection overlay a
    // screenshot uses for this family, then hand the picked target to the
    // existing Recorder (no capture of our own). mode: "monitor" | "window" |
    // "region"; a window and a region both record their plain rect.
    function recordTarget(mode, audioArgs) {
        root._purpose = "record";
        root._recordAudio = audioArgs || [];
        root.selecting = mode;
    }

    // Called by an overlay when a selection commits. result carries the output
    // name, its logical layout origin (monX/monY) and, for region/window, the
    // output-local rect, so we resolve global logical coordinates for grim.
    function commit(result) {
        root.selecting = "";
        if (root._purpose === "record") {
            var rgx = Math.round(result.monX + result.x);
            var rgy = Math.round(result.monY + result.y);
            var geom = Math.round(result.w) + "x" + Math.round(result.h) + "+" + rgx + "+" + rgy;
            // A monitor becomes -w <NAME> on the KMS backend and degrades to this
            // rect on the portal backend, so it carries both; a window is captured
            // by its area, so window and region both record the plain --region rect.
            var args = result.mode === "monitor"
                ? ["--monitor", result.output, "--geometry", geom]
                : ["--region", "--geometry", geom];
            // The delay counts from the selection, exactly as it does for a shot:
            // you frame the target first, then get the beat to clear the frame.
            Recorder.startAfter(args.concat(root._recordAudio), root.delay);
            return;
        }
        if (result.mode === "monitor") {
            root._pending = { flag: "-o", val: result.output };
        } else {
            var gx = Math.round(result.monX + result.x);
            var gy = Math.round(result.monY + result.y);
            root._pending = { flag: "-g", val: gx + "," + gy + " " + Math.round(result.w) + "x" + Math.round(result.h) };
        }
        fireTimer.interval = root._delayMs;
        fireTimer.restart();
    }

    function cancel() {
        root.selecting = "";
    }

    // Delay gate: 500ms floor for All (set in shoot), else the picked delay.
    Timer {
        id: fireTimer
        onTriggered: root._run()
    }

    // UTC capture stamp, matching zenith-shot's filename pattern.
    function _stamp() {
        var d = new Date();
        function p(n) { return (n < 10 ? "0" : "") + n; }
        return d.getUTCFullYear() + "_" + p(d.getUTCMonth() + 1) + "_" + p(d.getUTCDate())
            + "_" + p(d.getUTCHours()) + "_" + p(d.getUTCMinutes()) + "_" + p(d.getUTCSeconds());
    }

    function _run() {
        if (!root._pending)
            return;
        // Save to the Screenshots folder for file / both; a pure clipboard shot
        // writes a throwaway.
        var saveToFolder = root._save !== "clipboard";
        root._outPath = saveToFolder
            ? (root.shotsDir + "/" + root._stamp() + "_screenshot.png")
            : "/tmp/zenith-shot-clipboard.png";
        // Detached so wl-copy's clipboard daemon outlives this call: a managed
        // Process reaps its child tree on exit, which kills that daemon and the
        // selection vanishes even though grim's file is written. grim, the
        // clipboard write, the shutter cue and the toast all run in one detached
        // shell. A missing grim or wl-copy is said in a toast, not swallowed.
        // $1 flag, $2 value, $3 out, $4 mode, $5 clip.
        var script = [
            'FLAG="$1"; VAL="$2"; OUT="$3"; MODE="$4"; CLIP="$5"',
            'mkdir -p "$(dirname "$OUT")" 2>/dev/null || true',
            'if [ -n "$FLAG" ]; then grim "$FLAG" "$VAL" "$OUT"; else grim "$OUT"; fi || {',
            '  notify-send -a zenith "Screenshots need grim" "Install it with: sudo apt install grim" >/dev/null 2>&1',
            '  exit 0',
            '}',
            'if [ -n "$CLIP" ] && ! { zenith-shell clip-copy image/png "$OUT" || wl-copy --type image/png < "$OUT"; } >/dev/null 2>&1; then',
            '  notify-send -a zenith "Screenshot not copied" "Install wl-clipboard: sudo apt install wl-clipboard" >/dev/null 2>&1',
            '  [ "$MODE" = clipboard ] && exit 0',
            '  MODE=file',
            'fi',
            'zenith-shell sound shutter >/dev/null 2>&1 || true',
            'case "$MODE" in',
            'clipboard) notify-send -a zenith "Screenshot copied to clipboard" >/dev/null 2>&1 || true ;;',
            'file) notify-send -a zenith "Screenshot saved" "Saved to $OUT" >/dev/null 2>&1 || true ;;',
            '*) notify-send -a zenith "Screenshot saved & copied" "Saved to $OUT" >/dev/null 2>&1 || true ;;',
            'esac'
        ].join("\n");
        Spawn.run(["sh", "-c", script, "sh",
            root._pending.flag, root._pending.val, root._outPath, root._save,
            (root._save === "both" || root._save === "clipboard") ? "1" : ""]);
    }
}
