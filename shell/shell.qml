pragma ComponentBehavior: Bound

// The shell's entry point.
//
// `quickshell -p /usr/share/zenith/shell` runs this file. Nothing else names it,
// which is why the packaging check can assert it exists: a missing entry point
// is a shell that starts and immediately exits, with nothing in the journal that
// says why.
//
// Everything below is loaded once, here, rather than on demand. A bar that
// assembles itself from a Loader per widget means the bar is absent until every
// widget has constructed, and a widget that fails to construct leaves a hole in
// it with no indication which one.

import Quickshell
import Quickshell.Io
import Quickshell.Services.Notifications
import Quickshell.Widgets
import QtQuick

ShellRoot {
    id: root

    // The session's colour scheme is read once and bound, not polled. A shell
    // that re-reads settings on a timer is a shell that redraws on a clock
    // rather than on a change. The value itself lives on the wrapper, because
    // that is what every widget asks for it through.
    readonly property bool dark: Config.dark

    // Popovers are children of the root rather than of the surface that opened
    // them, so closing a popover is closing one object instead of tracking which
    // surface owns it. Two popovers open at once is two independent objects.
    //
    // Exactly one is open at a time, and that is the point of `currentPopover`
    // rather than three separate booleans: with three booleans, opening the
    // third leaves the first two open and the user gets two orphaned surfaces
    // with no keybinding that closes them.
    property string currentPopover: ""

    function togglePopover(name) {
        currentPopover = currentPopover === name ? "" : name
        launcherPopover.opened = currentPopover === "launcher"
        quickSettingsPopover.opened = currentPopover === "quick"
        cheatsheetPopover.opened = currentPopover === "cheatsheet"
    }

    // A popover that closes itself tells the root, so that the next press of
    // its key opens it rather than swallowing the press.
    //
    // The popover cannot say so itself, because the file that draws it is a
    // surface and the thing that decides what is open is the root. Without
    // this, a surface that closed itself — with Escape, or with a click
    // somewhere else — leaves the root believing it is still open, and the
    // press that should open it is spent clearing a decision that is no
    // longer true.
    function forget(name: string) {
        if (root.currentPopover === name) {
            root.currentPopover = ""
        }
    }

    Connections {
        target: launcherPopover
        function onVisibleChanged() { root.forget("launcher") }
    }

    Connections {
        target: quickSettingsPopover
        function onVisibleChanged() { root.forget("quick") }
    }

    Connections {
        target: cheatsheetPopover
        function onVisibleChanged() { root.forget("cheatsheet") }
    }

    // The popovers hang off the dock, because the dock is what opens them and
    // a popup placed against the dock is a popup placed against the edge of
    // the screen the dock is on. The launcher and the quick settings hang from
    // the side of the dock that faces the screen, so they open beside it
    // rather than over it.
    Popover {
        id: launcherPopover
        implicitWidth: 640
        implicitHeight: 480
        anchorItem: dock.contentItem
        edgeTop: Config.dockPosition === Config.edgeBottom
        edgeBottom: Config.dockPosition === Config.edgeTop
        edgeLeft: Config.dockPosition === Config.edgeRight
        edgeRight: Config.dockPosition === Config.edgeLeft
        Launcher {}
    }

    Popover {
        id: quickSettingsPopover
        implicitWidth: 360
        implicitHeight: 320
        anchorItem: dock.contentItem
        edgeTop: Config.dockPosition === Config.edgeBottom
        edgeBottom: Config.dockPosition === Config.edgeTop
        edgeLeft: Config.dockPosition === Config.edgeRight
        edgeRight: Config.dockPosition === Config.edgeLeft
        QuickSettings {}
    }

    // The cheat sheet.
    //
    // It hangs off the dock like the other two, because the dock is what the
    // popovers hang off and where a surface sits is the dock's business and
    // not the sheet's.
    Popover {
        id: cheatsheetPopover
        // The size is the sheet's, because the sheet is as tall as the
        // catalogue needs it to be.
        implicitWidth: cheatsheet.implicitWidth
        implicitHeight: cheatsheet.implicitHeight
        anchorItem: dock.contentItem
        edgeTop: Config.dockPosition === Config.edgeBottom
        edgeBottom: Config.dockPosition === Config.edgeTop
        edgeLeft: Config.dockPosition === Config.edgeRight
        edgeRight: Config.dockPosition === Config.edgeLeft
        // The window's own background is white, and the sheet draws its own
        // card over it, so the window has to be transparent for the card to
        // have corners. The sheet is dark whatever the window is.
        color: "transparent"
        Shell {
            id: cheatsheet
            width: implicitWidth
            height: implicitHeight
        }
    }

    // The two always-present surfaces. The bar and the dock are siblings of the
    // popovers rather than children of them: a popover that closes must not take
    // the bar with it.
    // The wallpaper first, because QML draws siblings in declaration order and
    // nothing else. A wallpaper declared after the bar is a wallpaper the bar
    // is drawn over.
    Wallpaper {}

    Bar {}
    Dock {
        id: dock
    }

    // The sidebar is a sibling of the bar and the dock rather than a popover
    // child, for the same reason: a panel that opens must not take the surfaces
    // it was opened from down with it when it closes.
    Sidebar {
        id: sidebar
        visible: root.currentPopover === "sidebar"
    }

    // The switcher is independent of the popovers. It is driven by a keybind
    // rather than by togglePopover, and it closes itself, so keeping it out of
    // currentPopover is what stops a Super+Tab from also closing an open
    // launcher.
    Switcher {
        id: switcher
    }

    // Idle owns the timers and the overlay draws what they mean. The overlay is
    // declared last so it is on top of everything, which is the whole point of
    // it.
    //
    // The overlay is told what the idle object knows rather than asking it:
    // they are two files, and a file cannot see another file's ids.
    Idle {
        id: idleState
    }
    IdleOverlay {
        dimmed: idleState.dimmed
        blanked: idleState.blanked
    }

    // The notification server. One for the session, because one bus name for
    // the interface is all there is: a second server loses the registration
    // and the notifications go to a surface nobody is looking at.
    NotificationServer {
        id: notificationServer
    }

    // The notification popup. It is a child of the root and not of the popovers,
    // for the reason the popovers are: a notification that arrives while the
    // quick settings are open must not be dismissed by them closing, and a
    // popup that owns its own lifetime cannot be taken down by someone else's.
    Notifications {
        anchorItem: dock.contentItem
        server: notificationServer
    }

    // The shell's external surface: what the compositor's keybinds call.
    //
    // A keybind is a string in one file and the surface it names is a string
    // in another, and nothing in either file checks the other. A rename on one
    // side leaves a keybind that runs, succeeds, and does nothing — the
    // quietest possible failure, and the one a user reports as "the shortcut
    // stopped working" rather than as a bug.
    //
    // So the target name is written once, here, and `zenith doctor` checks the
    // compositor's configuration against it.
    //
    // The functions are the shell's own, not wrappers around them: togglePopover
    // is what the shell's own surfaces call, and a keybind that took a
    // different path would be a second opinion about what Super+Space means.
    // ---------------------------------------------------------------- readers
    //
    // The shell's three inputs from the machine: the settings, the catalogue of
    // applications, and the palette. All three are read here, in the root,
    // rather than in the singletons that hold them.
    //
    // A process inside a singleton is a process that never runs. That is the
    // one thing about a singleton the engine does not make work, and it is
    // silent: the shell carries on and the value never arrives. A shell that
    // reads its settings in a singleton is a shell with no settings, and it
    // looks like a shell whose settings are all empty rather than like a shell
    // that never asked.
    //
    // So the root reads, and pushes the answers into the singletons that own
    // them. The singletons stay what they are good at: the vocabulary the
    // surfaces read through.
    Process {
        id: settingsReader
        command: ["zenith", "config", "values"]
        running: true

        stdout: StdioCollector {
            id: settingsOut
            waitForEnd: true
            onStreamFinished: {
                try {
                    Config.values = JSON.parse(settingsOut.text);
                } catch (error) {
                    console.warn("Config: the settings could not be read:", error);
                }
            }
        }
    }

    Process {
        id: appsReader
        command: ["zenith", "apps"]
        running: true

        stdout: StdioCollector {
            id: appsOut
            waitForEnd: true
            onStreamFinished: {
                try {
                    Apps.applications = JSON.parse(appsOut.text);
                } catch (error) {
                    console.warn("Apps: the application list could not be read:", error);
                }
            }
        }
    }

    Process {
        id: paletteReader
        command: ["cat", Theme.path]
        running: true

        stdout: StdioCollector {
            id: paletteOut
            waitForEnd: true
            onStreamFinished: {
                try {
                    Theme.palette = JSON.parse(paletteOut.text);
                } catch (error) {
                    // A broken palette leaves the fallback in place rather than
                    // throwing, because a shell that dies on a parse error is
                    // a shell with no desktop at all.
                }
            }
        }
    }

    // Read the three again.
    //
    // The settings are read once and held, because a shell that re-reads them
    // on a timer redraws on a clock rather than on a change. A write says so
    // over the IPC target, and the tool that made the write says so too, so
    // nothing is polled.
    function reloadSettings() {
        settingsReader.running = false;
        settingsReader.running = true;
        appsReader.running = false;
        appsReader.running = true;
        paletteReader.running = false;
        paletteReader.running = true;
    }

    IpcHandler {
        id: ipc
        target: "zenith"

        // Open or close a surface by name: "launcher", "quick", "sidebar".
        function togglePopover(name: string): void {
            root.togglePopover(name)
        }

        // The window switcher. It has its own lifetime — it is not a popover —
        // and it closes itself, so the two verbs are separate on purpose.
        function advanceSwitcher(): void {
            switcher.advance()
        }

        function closeSwitcher(): void {
            switcher.close()
        }

        // The cheat sheet: the list of what the keyboard does.
        //
        // Its own verb rather than a name for togglePopover, because the
        // compositor's configuration names it as a thing in its own right —
        // the catalogue is the shell's list and the sheet is where that list
        // is drawn, and a keybind that called it by the popover's name would
        // be calling a surface by the name of the thing that opens surfaces.
        function toggleCheatsheet(): void {
            root.togglePopover("cheatsheet")
        }

        // The settings changed. The shell reads them once and holds what it
        // read, so a change has to be told to it rather than waited for. The
        // tool that writes a setting says so, and so does the Hub.
        function reloadSettings(): void {
            root.reloadSettings()
        }
    }
}
