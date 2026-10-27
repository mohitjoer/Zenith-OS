import QtQuick
import Quickshell
import Quickshell.Io
import qs.Services
import "../ZenithPower.js" as ZenithPower
import qs.Ui.Singletons

Item {
    id: rootMod
    required property var root
    readonly property color contentColor: root.widgetContentColor("G13", root.ink)

    property bool   hasBacklight: false
    property int    percent:      0
    property string blDevice:     ""
    property bool   brightnessErrorNotified: false
    property bool   sysfsWriteTried: false
    property bool   sysfsWriteAvailable: false

    readonly property string tooltipText: I18n.tr("Brightness · %1% · Requires elevated helper (brightnessctl/pkexec)").arg(percent)

    readonly property bool shown: hasBacklight && root.modBrightness
    implicitWidth:  shown ? (row.implicitWidth + 18) : 0
    implicitHeight: 28
    visible: implicitWidth > 0.5
    opacity: shown ? 1 : 0

    Behavior on opacity { NumberAnimation { duration: 140; easing.type: Easing.OutCubic } }

    function refreshBrightness() {
        // Prefer daemon-fed OsdFeed.brightness (0..1) when daemon is alive;
        // otherwise fall back to sysfs read via OsdFeed.sysfsValid.
        if (OsdFeed.daemonSeen) {
            rootMod.percent = Math.round(OsdFeed.brightness * 100);
        } else if (OsdFeed.sysfsValid && OsdFeed.sysfsMaxBrightness > 0) {
            rootMod.percent = Math.round((OsdFeed.sysfsBrightness / OsdFeed.sysfsMaxBrightness) * 100);
        } else {
            // Last resort: one-shot brightnessctl read (kept for compatibility).
            briProc.running = false;
            briProc.running = true;
        }
    }

    function writeSysfsBrightness(value) {
        if (!rootMod.blDevice) return false;
        // Try writing directly to sysfs brightness file
        // This will fail without video group membership or elevated privileges
        try {
            sysfsWriteFile.path = "/sys/class/backlight/" + rootMod.blDevice + "/brightness";
            sysfsWriteFile.setText(String(value));
            rootMod.sysfsWriteTried = true;
            // If write succeeds, we'll know on the next poll
            return true;
        } catch (e) {
            return false;
        }
    }

    function runBrightnessStep(up) {
        var step = up ? 5 : -5;
        var target = Math.max(1, Math.min(100, rootMod.percent + step));
        rootMod.percent = target;

        // Try writing to sysfs first
        if (!rootMod.sysfsWriteTried || rootMod.sysfsWriteAvailable) {
            var maxB = OsdFeed.sysfsMaxBrightness > 0 ? OsdFeed.sysfsMaxBrightness : 96000;
            var rawValue = Math.round(target * maxB / 100);
            var ok = writeSysfsBrightness(rawValue);
            if (ok) {
                rootMod.sysfsWriteAvailable = true;
                // Refresh will happen via OsdFeed poll
                return;
            }
        }

        // Degraded: read-only. Surface a non-spammy tooltip hint once.
        if (!rootMod.brightnessErrorNotified) {
            rootMod.brightnessErrorNotified = true;
            brightnessErrNotify.command = ["bash", "-c",
                "notify-send -a 'QS-Shell' 'Brightness control unavailable' 'Write to /sys/class/backlight requires video group or pkexec; configure brightnessctl udev rule or run with elevated helper.' 2>/dev/null || true"];
            brightnessErrNotify.running = false;
            brightnessErrNotify.running = true;
        }
    }

    function notifyBrightnessError(action, exitCode) {
        if (exitCode === 0 || rootMod.brightnessErrorNotified) return;
        rootMod.brightnessErrorNotified = true;
        brightnessErrNotify.command = ["bash", "-c",
            "notify-send -a 'QS-Shell' 'Brightness command failed' '" + action + " failed; brightness backend unavailable.' 2>/dev/null || true"];
        brightnessErrNotify.running = false;
        brightnessErrNotify.running = true;
    }

    Row {
        id: row
        anchors.centerIn: parent
        spacing: 4

        // drawn sun - core + rays that grow/brighten with the level
        Item {
            id: sun
            width: 13
            height: 13
            anchors.verticalCenter: parent.verticalCenter

            readonly property real ratio: Math.max(0, Math.min(1, rootMod.percent / 100))
            // turns theme-red at full brightness, like the battery's full state
            readonly property color sunColor: rootMod.percent >= 100
                ? (root.widgetHasFill("G13") ? rootMod.contentColor : root.seal)
                : rootMod.contentColor

            Rectangle {
                anchors.centerIn: parent
                width: 6.5
                height: 6.5
                radius: 3.25
                color: sun.sunColor
                Behavior on color { ColorAnimation { duration: 200 } }
            }

            Repeater {
                model: 8
                delegate: Item {
                    required property int index
                    anchors.fill: parent
                    rotation: index * 45
                    Rectangle {
                        anchors.horizontalCenter: parent.horizontalCenter
                        anchors.top: parent.top
                        anchors.topMargin: 0.3
                        width: 1.5
                        height: 2 + 1.4 * sun.ratio
                        radius: 0.75
                        color: sun.sunColor
                        opacity: 0.35 + 0.65 * sun.ratio
                        Behavior on height  { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }
                        Behavior on opacity { NumberAnimation { duration: 250 } }
                        Behavior on color   { ColorAnimation  { duration: 200 } }
                    }
                }
            }
        }

        UiText {
            visible: !root.iconOnly("G13")
            anchors.verticalCenter: parent.verticalCenter
            text: rootMod.percent + "%"
            color: rootMod.percent >= 100
                ? (root.widgetHasFill("G13") ? rootMod.contentColor : root.seal)
                : rootMod.contentColor
            font.family: root.mono
            font.pixelSize: 12
            Behavior on color { ColorAnimation { duration: 200 } }
        }
    }

    // detect backlight on startup
    Process {
        id: detectProc
        command: ["bash", "-c", ZenithPower.backlightDetectCmd]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                var bl = this.text.trim()
                if (bl !== "") {
                    rootMod.blDevice = bl
                    rootMod.hasBacklight = true
                    root.hasBacklight = true
                }
            }
        }
    }

    // One-shot brightnessctl read for daemon-absent fallback (kept for compat).
    Process {
        id: briProc
        command: ["bash", "-c", ZenithPower.brightnessPercentCmd]
        running: false
        stdout: StdioCollector {
            onStreamFinished: {
                var v = parseInt(this.text.trim())
                if (!isNaN(v)) rootMod.percent = Math.max(0, Math.min(100, v))
            }
        }
    }

    Timer {
        interval: 3000; running: rootMod.shown; repeat: true; triggeredOnStart: true
        onTriggered: rootMod.refreshBrightness()
    }

    Process { id: brightnessErrNotify }

    // Sysfs write attempt (tried once; if it works we keep using it, else degrade to read-only)
    FileView {
        id: sysfsWriteFile
        path: ""
        blockLoading: false
        printErrors: false
        atomicWrites: true
    }

    // These remain for the elevated-helper path (orchestrator may wire pkexec/brightnessctl).
    // They only run when sysfsWriteAvailable === true (currently never).
    Process {
        id: briUp
        command: ["bash", "-c", ZenithPower.brightnessSetCmd("+5%")]
        onExited: (code) => {
            rootMod.notifyBrightnessError("Brightness up", code)
            rootMod.refreshBrightness()
        }
    }
    Process {
        id: briDown
        command: ["bash", "-c", ZenithPower.brightnessSetCmd("5%-")]
        onExited: (code) => {
            rootMod.notifyBrightnessError("Brightness down", code)
            rootMod.refreshBrightness()
        }
    }

    TooltipMixin { id: tip; root: rootMod.root; owner: rootMod; text: rootMod.tooltipText }

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true; cursorShape: Qt.PointingHandCursor
        onEntered: { if (rootMod.hasBacklight) tip.show() }
        onExited:  { tip.hide() }
        onClicked: { tip.hide(); root.brightnessVisible = !root.brightnessVisible }
        onWheel: (e) => {
            rootMod.runBrightnessStep(e.angleDelta.y > 0)
        }
    }
}