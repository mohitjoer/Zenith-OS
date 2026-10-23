pragma ComponentBehavior: Bound

// The window switcher.
//
// Bound to Alt+Tab (Super+Tab is the workspace overview), and Alt+Shift+Tab
// walks the list the other way. Each press moves to the next window in
// most-recently-used order and the overlay shows what happened.
//
// The MRU order comes from `zenith windows`, never from here. Hyprland already
// tracks it in `focusHistoryID`, and the id is inverted from what it looks like
// — lower is more recent — so it is parsed and sorted in Go where there are
// tests. A sort in QML that is one line and backwards is the worst kind.
//
// It focuses on press rather than on the modifier's release, which is the
// deviation from macOS and it is deliberate. Releasing a modifier after a Tab
// press does not reliably deliver a keybind in Hyprland, and every third-party
// switcher works around it with raw keyboard-event or key-state polling.
// Committing on press needs one bind and no polling. What is given up is
// picking from the whole list before committing, which needs a release the
// compositor does not reliably send.
//
// The release only starts a dwell. The compositor reports the press and the
// release a few milliseconds apart, and an overlay hidden on the release was on
// screen for one frame; every press and release restarts the timer, so the card
// outlives the last of them by `dwell` and a lost release still ends it.
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
    // which is the one case that reads as "the overlay is broken". Reverse starts
    // at -1, "the last window", which is only known once the list has arrived.
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
                // Exit codes are not settled when the stream closes (the exit
                // signal lands after this collector), so the guard is content,
                // not exit code: `exitedWithCode` is still undefined here.
                // No compositor, or nothing to switch to: close rather than show
                // an empty card. The keybind fired; there was simply no window
                // behind it.
                var t = text.trim()
                if (t === "" || t === "null" || t === "[]") {
                    switcher.dismiss()
                    return
                }
                try {
                    switcher.windows = JSON.parse(text)
                } catch (error) {
                    switcher.dismiss()
                    return
                }
                var last = switcher.windows.length - 1
                switcher.selected = switcher.selected < 0 ? last : Math.min(switcher.selected, last)
                // The dwell counts from the card having something to show.
                dwell.restart()
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

    Timer {
        id: dwell
        interval: 450
        onTriggered: switcher.visible = false
    }

    // advance and reverse are what the keybinds call. A press loads the list on
    // the press that opens the overlay — and focuses once the list arrives, since
    // the target cannot be known before then — then walks the frozen snapshot on
    // every press after, forwards or backwards.
    function advance() { press(1) }
    function reverse() { press(-1) }

    function press(direction) {
        dwell.restart()
        if (!switcher.visible) {
            switcher.windows = []
            switcher.selected = direction > 0 ? 1 : -1
            switcher.token = Date.now()
            switcher.visible = true
            return
        }
        if (switcher.windows.length > 1) {
            switcher.selected = (switcher.selected + direction + switcher.windows.length) % switcher.windows.length
            switcher.focusToken = Date.now()
        }
    }

    // The key came up: the card stays for the dwell, see the header.
    function close() {
        if (switcher.visible)
            dwell.restart()
    }

    // Escape and click-away leave at once.
    function dismiss() {
        dwell.stop()
        switcher.visible = false
    }

    Rectangle {
        anchors.centerIn: parent
        width: Math.min(560, switcher.width - 80)
        height: Math.min(list.implicitHeight + 32, switcher.height - 80)
        radius: 12
        color: Theme.surface
        border.width: 1
        border.color: Theme.lineDivider

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
                    color: index === switcher.selected ? Theme.indicator : "transparent"

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
                        color: parent.picked ? Theme.inkInverse : Theme.inkPrimary
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
                        color: parent.picked ? Theme.inkInverse : Theme.inkSecondary
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
        onActivated: switcher.dismiss()
    }

    TapHandler {
        onTapped: switcher.dismiss()
    }
}