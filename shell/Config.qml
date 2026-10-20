pragma Singleton
pragma ComponentBehavior: Bound

// Every settings read in the shell happens here, once.
//
// One object rather than one per surface, because a per-surface object means
// each widget binds and reloads independently, and a widget created after a
// settings change shows the value from before the change. One object, bound
// once, has no window in which two widgets disagree.
//
// `pragma Singleton` is what makes it reachable from the other files. A QML
// object is visible to the file that declares it and to nothing else, and the
// surfaces that read settings are all in other files. Declared as a singleton,
// the object is exported from the shell's own module and every surface reaches
// it as `Config` — the file's name, which is the only name a QML type can
// have. The id below is the same object seen from inside this file, and the
// two are not two settings objects.
//
// The settings themselves are read by Settings.qml and arrive here as two
// objects: one per schema. This file is the wrapper — the surface every other
// file reads through — and the reading is somewhere else, because a wrapper
// that also owned the machinery would have to justify every one of its
// properties as a bridge to a setting, and a reader is not a bridge.

import Quickshell
import Quickshell.Io

Singleton {
    id: config

    // The two schemas, as the reader reported them.
    //
    // The values are read by the shell's root and pushed in here, rather than
    // read here: a process inside a singleton is a process that never runs,
    // and a shell that reads its settings in a singleton is a shell with no
    // settings. The root owns the reading; the wrapper owns the vocabulary.
    property var values: ({})

    readonly property var shell: values["org.zenith.shell"] || ({})
    readonly property var desktop: values["org.zenith.desktop"] || ({})

    readonly property int colorScheme: desktop.colorScheme ?? 0

    // Idle policy. Seconds, and 0 means never — not "now". A timeout of 0 that
    // means "immediately" is a key whose default locks the machine the moment
    // it is installed, and every "0 means never" key in a desktop exists
    // because somebody got that backwards once.
    //
    // lock-on-suspend is deliberately not here. It was a key the shell could
    // not honour: Quickshell exposes no way to observe a suspend, so the shell
    // would have carried a setting that looked like it worked and did not.
    readonly property int idleDimSeconds: desktop.idleDimSeconds ?? 0
    readonly property int idleBlankSeconds: desktop.idleBlankSeconds ?? 0
    readonly property int idleLockSeconds: desktop.idleLockSeconds ?? 0

    // Derived here rather than on the root. Every widget asks the wrapper
    // "Config.dark", so the answer has to be on the wrapper; leaving it on the
    // root means a widget reads a property the wrapper does not have, and
    // GSettings-returning-defaults quietly rather than failing.
    readonly property bool dark: colorScheme === 1

    readonly property var barWidgets: shell.barWidgets ?? []
    readonly property int barHeight: shell.barHeight ?? 0
    readonly property int barPosition: shell.barPosition ?? 0
    readonly property var favorites: shell.favorites ?? []
    readonly property int dockPosition: shell.dockPosition ?? 0
    readonly property bool dockAutohide: shell.dockAutohide ?? false
    readonly property var launcherApps: shell.launcherApps ?? []
    readonly property int launcherColumns: shell.launcherColumns ?? 0
    readonly property string wallpaper: dark
        ? (shell.wallpaperPathDark ?? "")
        : (shell.wallpaperPath ?? "")

    // Edge is stored as the enum's numeric value because the schema uses a
    // GSettings enum. Comparing against the schema's nick names here would mean
    // repeating the mapping in three places and getting one of them wrong.
    readonly property int edgeTop: 0
    readonly property int edgeBottom: 1
    readonly property int edgeLeft: 2
    readonly property int edgeRight: 3

    // ---------------------------------------------------------------- writers
    //
    // Writes go through here rather than being spread across the surfaces that
    // want them, for two reasons. It keeps Config.qml the only file that knows
    // which schema owns which key — org.zenith.desktop owns color-scheme and
    // org.zenith.shell owns dock-autohide, and a surface that guessed wrong
    // would write the right key to the wrong schema, which is a setting that
    // appears to work.
    //
    // And it keeps the surfaces readable: a switch says what it means
    // ("setDockAutohide"), not how.
    //
    // The shell daemon is one of the two permitted writers of these schemas —
    // the other is zenith-hub — so a write from here is not a rule violation.
    // What would be is a surface reaching past this wrapper.

    //
    // The write goes through the tool, which checks the value against the
    // schema before it stores it, and the tool tells the shell to read its
    // settings again. The round trip is the point: a write that the shell
    // does not hear about is a write that the surfaces do not show.
    function setDockAutohide(value) {
        Quickshell.execDetached([
            "zenith", "config", "set", "org.zenith.shell", "dock-autohide",
            value ? "true" : "false"
        ]);
    }

    function setColorScheme(value) {
        Quickshell.execDetached([
            "zenith", "config", "set", "org.zenith.desktop", "color-scheme",
            "" + value
        ]);
    }

    // ------------------------------------------------------------------- lock
    //
    // The shell decides *when* to lock and hyprlock does the locking. That
    // division is not a compromise on convenience: a QML lock screen cannot
    // authenticate against PAM, because there is no QML PAM API, and the usual
    // workaround is to call something over D-Bus — which produces a lock screen
    // that accepts any password.
    //
    // So the shell never draws a lock surface. It runs the timers and execs a
    // locker that actually authenticates.
    property string locker: "/usr/bin/hyprlock"

    function lock() {
        // The lockers take a --locker command so a compositor-native session
        // can be used instead of the greeter on unlock. Unchanged here: hyprlock
        // restores the session it locked, and a wrong command here is how a
        // machine ends up at the greeter after unlocking.
        exec.lock()
    }
}
