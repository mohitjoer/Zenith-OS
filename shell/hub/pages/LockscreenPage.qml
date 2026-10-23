pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import qs.Ui
import qs.Ui.Singletons

// Lockscreen. hyprlock draws the lock screen and hypridle starts it, so the
// lock has no options of its own to set here: when it comes up is the Idle and
// Locking page, and how it looks is session/hyprland/hyprlock.conf. This page
// says whether the pieces are installed and running, and locks on request.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    // What the probe below found missing: "hyprlock", "hypridle", or "idle-off"
    // for an installed hypridle whose service is not running.
    property var missing: []
    readonly property bool canLock: pg.missing.indexOf("hyprlock") < 0

    Process {
        running: true
        command: ["sh", "-c",
            "for t in hyprlock hypridle; do command -v $t >/dev/null || echo $t; done; "
            + "systemctl --user is-active --quiet zenith-idle.service || echo idle-off"]
        stdout: StdioCollector {
            onStreamFinished: pg.missing = text.trim().split("\n").filter(s => s !== "")
        }
    }

    Column {
        x: Tokens.s6
        y: Tokens.s6
        width: Math.min(pg.width - Tokens.s6 * 2, 720)
        spacing: Tokens.s3

        Text {
            text: I18n.tr("Lockscreen")
            color: Tokens.ink
            font.family: Tokens.display
            font.pixelSize: Tokens.fTitle
        }
        Text {
            width: parent.width
            wrapMode: Text.WordWrap
            text: I18n.tr("hyprlock draws the lock screen and checks your password with PAM. hypridle starts it when you have been idle; set the times under Idle and Locking.")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
        }
        Text {
            width: parent.width
            wrapMode: Text.WordWrap
            visible: !pg.canLock
            text: I18n.tr("Not available: install hyprlock to lock the screen.")
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            font.weight: Font.Medium
        }
        Text {
            width: parent.width
            wrapMode: Text.WordWrap
            visible: pg.missing.indexOf("hypridle") >= 0
            text: I18n.tr("Not available: install hypridle to lock after being idle.")
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            font.weight: Font.Medium
        }
        Text {
            width: parent.width
            wrapMode: Text.WordWrap
            visible: pg.missing.indexOf("idle-off") >= 0 && pg.missing.indexOf("hypridle") < 0
            text: I18n.tr("The idle lock is not running, so the screen will not lock by itself.")
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            font.weight: Font.Medium
        }
        Btn {
            text: I18n.tr("LOCK NOW")
            armed: pg.canLock
            onAct: Quickshell.execDetached(["zenith-shell", "lock"])
        }
    }
}
