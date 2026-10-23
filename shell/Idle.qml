pragma ComponentBehavior: Bound

// The shell's part of the idle policy: it draws the dim scrim and nothing else.
//
// hypridle owns the clock. It holds off for D-Bus and systemd inhibitors (a
// browser playing a video, Keep Awake), which the compositor's own idle signal
// does not see, so a second clock here would dim a screen the lock then
// ignores. hypridle tells the shell a stage began or ended by writing its name
// to $XDG_RUNTIME_DIR/zenith-idle (session/hyprland/hypridle.conf); this
// watches that file.
//
// It also keeps ~/.config/zenith/current/background pointing at the wallpaper in
// use. The lock screen's config can only name an image path, not read the
// wallpaper state file, and that link is the path it names.

import Quickshell
import Quickshell.Io
import QtQuick
import qs.Services

Scope {
    id: idle

    property bool dimmed: false
    property bool ready: false
    readonly property string flag: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/zenith-idle"

    // A file that does not exist cannot be watched, and the shell may start
    // before zenith-idle does.
    Process {
        running: true
        command: ["sh", "-c", ": >> \"$0\"", idle.flag]
        onExited: idle.ready = true
    }

    FileView {
        path: idle.ready ? idle.flag : ""
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: idle.dimmed = text().trim() === "dim"
    }

    Variants {
        model: ShellState.screens
        IdleOverlay { dimmed: idle.dimmed }
    }

    function syncBackground() {
        // A video has no still for the lock screen to draw; the link keeps the
        // last image and hyprlock falls back to its colour when there is none.
        if (Session.wallpaper === "" || Session.wallIsVideo)
            return;
        Quickshell.execDetached(["ln", "-sfn", Session.wallpaper,
            Quickshell.env("HOME") + "/.config/zenith/current/background"]);
    }

    Connections {
        target: Session
        function onWallpaperChanged() { idle.syncBackground(); }
    }

    Component.onCompleted: syncBackground()
}
