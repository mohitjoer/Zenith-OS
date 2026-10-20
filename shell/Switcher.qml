pragma ComponentBehavior: Bound

// The window switcher.
//
// Bound to Super+Tab, the way Ryoku and macOS bind it. Each press moves to the
// next window in most-recently-used order and the overlay shows what happened.
//
// The MRU order comes from `zenith windows`, never from here. Hyprland already
// tracks it in `focusHistoryID`, and the id is inverted from what it looks like
// — lower is more recent — so it is parsed and sorted in Go where there are
// tests. A sort in QML that is one line and backwards is the worst kind.
//
// It focuses on press rather than on Super's release, which is the deviation
// from macOS and it is deliberate. Releasing a modifier after a Tab press does
// not reliably deliver a keybind in Hyprland, and every third-party switcher
// works around it with raw keyboard-event or key-state polling. Committing on
// press needs one bind and no polling. What is given up is picking from the
// whole list before committing, which needs a release the compositor does not
// reliably send.
//
// The list is snapshotted once, on the press that opens the overlay. Re-reading
// each press would reshuffle it under the user, because focusing a window
// mutates focusHistoryID — this press would change what the next press means.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

PanelWindow {
    id: switcher

    implicitWidth: Math.min(560, screen.width - 80)
    implicitHeight: Math.min(420, screen.height - 80)

    anchors { top: true; left: true; right: true; bottom: true }

    color: "transparent"
    visible: false

    // Not the most recent window. focusHistoryID 0 is the window you are already
    // in, so starting there would mean the first press appears to do nothing,
    // which is the one case that reads as "the overlay is broken".
    property int selected: 1

    property var windows: []
    property string token: ""
    property string focusToken: ""

    Process {
        id: load
        command: ["zenith", "windows"]
        running: switcher.token !== ""

        stdout: StdioCollector {
            onStreamFinished: {
                // No compositor, or nothing to switch to: close rather than show
                // an empty card. The keybind fired; there was simply no window
                // behind it.
                if (load.exitedWithCode !== 0 || text.trim() === "null" || text.trim() === "[]") {
                    switcher.close()
                    return
                }
                try {
                    switcher.windows = JSON.parse(text)
                } catch (error) {
                    switcher.close()
                    return
                }
                switcher.focusToken = Date.now()
            }
        }
    }

    // Focus is a one-line dispatch, not a CLI subcommand. Unlike the parsers
    // there is nothing to interpret, so nothing to test and nothing to wrap.
    // `address:` is hyprlang syntax and Zenith ships Hyprland 0.53.3; 0.55 and
    // later wrap dispatch differently under a Lua Config.
    Process {
        id: focus
        command: {
            var target = switcher.windows[switcher.selected]
            return target
                ? ["hyprctl", "dispatch", "focuswindow", "address:" + target.address]
                : []
        }
        running: switcher.focusToken !== ""
    }

    // advance is what the keybind calls. It loads on the press that opens the
    // overlay — and focuses once the list arrives, since the target cannot be
    // known before then — then walks the frozen snapshot on every press after.
    function advance() {
        if (!visible) {
            windows = []
            selected = 1
            token = Date.now()
            visible = true
            return
        }
        if (windows.length > 1) {
            selected = (selected + 1) % windows.length
            focusToken = Date.now()
        }
    }

    function close() {
        visible = false
    }

    Rectangle {
        anchors.centerIn: parent
        width: Math.min(560, switcher.width - 80)
        height: Math.min(list.implicitHeight + 32, switcher.height - 80)
        radius: 12
        color: Theme.surface
        border.width: 1
        border.color: Theme.border

        Column {
            id: list
            anchors.centerIn: parent
            width: parent.width - 32
            spacing: 2

            Repeater {
                model: switcher.windows

                Rectangle {
                    required property var modelData
                    required property int index

                    width: list.width
                    height: 36
                    radius: 6
                    color: index === switcher.selected ? Theme.accent : "transparent"

                    // `class` is a reserved word, so it is read by key and not by
                    // attribute. A window with no class is listed by its title,
                    // which is the only thing it has.
                    readonly property string name: modelData["class"] || modelData.title
                    readonly property bool picked: index === switcher.selected

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.left: parent.left
                        anchors.right: workspaceLabel.left
                        anchors.leftMargin: 12
                        anchors.rightMargin: 12
                        text: parent.name
                        color: parent.picked ? Theme.accentForeground : Theme.text
                        font.pixelSize: 13
                        elide: Text.ElideRight
                    }

                    Text {
                        id: workspaceLabel
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.right: parent.right
                        anchors.rightMargin: 12
                        // The workspace number is what makes a cross-workspace
                        // switch legible: without it, focusing a window that is
                        // not on screen looks like nothing happened.
                        text: (modelData.workspace ?? {}).name ?? ""
                        color: parent.picked ? Theme.accentForeground : Theme.textDim
                        font.pixelSize: 11
                    }
                }
            }
        }
    }

    // Escape and click-away, because an overlay dismissed only by pressing the
    // same key again is one the user is stuck in when the list is not what they
    // expected.
    // A shortcut rather than the Keys attached property, because a panel is a
    // window and not an item, and Keys needs an item. The shortcut lives in the
    // window's content, which is where a key press lands anyway.
    Shortcut {
        sequence: "Escape"
        onActivated: switcher.close()
    }

    TapHandler {
        onTapped: switcher.close()
    }
}