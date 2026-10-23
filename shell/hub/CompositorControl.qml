pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import qs.Ui
import qs.Ui.Singletons
import "Singletons"

// The compositor Zenith is running, and which version of it. Zenith has one
// compositor, Hyprland, so this is a statement and not a picker: there is no
// switch to offer and no second store for settings to be carried across to. The
// version is read from the compositor that is running rather than from a package
// name, because the binary the settings apply to is the one on the screen.
Column {
    id: ctl

    readonly property string name: Settings.provider !== "" ? Settings.provider : "hyprland"
    property string version: ""
    property bool probed: false

    function cap(name) { return name.length ? name.charAt(0).toUpperCase() + name.slice(1) : name; }

    spacing: 0

    Process {
        command: ["hyprctl", "version", "-j"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var v = JSON.parse(this.text);
                    if (v && typeof v.version === "string")
                        ctl.version = v.version;
                } catch (e) {
                    ctl.version = "";
                }
                ctl.probed = true;
            }
        }
    }

    Section {
        id: sect
        width: ctl.width
        title: I18n.tr("COMPOSITOR")

        Column {
            width: sect.width
            spacing: Tokens.s2

            Rectangle {
                width: parent.width
                height: 62
                radius: Tokens.radius
                color: Tokens.tint5
                border.width: Tokens.border
                border.color: Tokens.lineStrong

                Column {
                    anchors { left: parent.left; leftMargin: Tokens.s4; right: chip.left; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                    spacing: 3
                    Text {
                        text: ctl.cap(ctl.name)
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fRow
                        font.weight: Font.Medium
                    }
                    Text {
                        text: ctl.version !== ""
                            ? I18n.tr("RUNNING NOW  \u00b7  %1").arg(ctl.version)
                            : (ctl.probed ? I18n.tr("RUNNING NOW") : "")
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fTiny
                        font.letterSpacing: 0.6
                    }
                }

                Rectangle {
                    id: chip
                    anchors { right: parent.right; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                    width: curLabel.width + Tokens.s3
                    height: 20
                    radius: Tokens.radius
                    color: Tokens.bone
                    Text {
                        id: curLabel
                        anchors.centerIn: parent
                        text: I18n.tr("CURRENT")
                        color: Tokens.inkOnBone
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny
                        font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackLabel
                    }
                }
            }

            Text {
                width: parent.width
                wrapMode: Text.WordWrap
                text: I18n.tr("Zenith runs on Hyprland. Window rules, animations and keybinds are written for it into your Zenith config, and your edits survive updates.")
                color: Tokens.inkFaint
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                lineHeight: 1.3
            }
        }
    }
}
