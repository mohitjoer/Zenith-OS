pragma ComponentBehavior: Bound

// Every settings read in the shell happens here, once.
//
// One Settings object rather than one per surface, because a per-surface object
// means each widget binds and reloads independently, and a widget created after
// a settings change shows the value from before the change. One object, bound
// once, has no window in which two widgets disagree.

import Quickshell
import Quickshell.Io

Singleton {
    id: config

    // The only writer of org.zenith.shell is the shell daemon or zenith-hub.
    // This object reads; it never writes.
    readonly property Settings shell: Settings {
        id: shellSettings
        schemaId: "org.zenith.shell"
    }

    // org.zenith.desktop owns the colour scheme, not org.zenith.shell: the shell
    // is a consumer of the desktop's decision, and a second copy of that key
    // would be a second decision that could disagree with the first.
    readonly property Settings desktop: Settings {
        id: desktopSettings
        schemaId: "org.zenith.desktop"
    }

    readonly property int colorScheme: desktop.colorScheme

    // Derived here rather than on the root. Every widget asks the wrapper
    // "config.dark", so the answer has to be on the wrapper; leaving it on the
    // root means a widget reads a property the wrapper does not have, and
    // GSettings-returning-defaults quietly rather than failing.
    readonly property bool dark: colorScheme === 1

    readonly property var barWidgets: shell.barWidgets
    readonly property int barHeight: shell.barHeight
    readonly property string barPosition: shell.barPosition
    readonly property var favorites: shell.favorites
    readonly property string dockPosition: shell.dockPosition
    readonly property bool dockAutohide: shell.dockAutohide
    readonly property var launcherApps: shell.launcherApps
    readonly property int launcherColumns: shell.launcherColumns
    readonly property string wallpaper: dark
        ? shell.wallpaperPathDark
        : shell.wallpaperPath

    // Edge is stored as the enum's numeric value because the schema uses a
    // GSettings enum. Comparing against the schema's nick names here would mean
    // repeating the mapping in three places and getting one of them wrong.
    readonly property int edgeTop: 0
    readonly property int edgeBottom: 1
    readonly property int edgeLeft: 2
    readonly property int edgeRight: 3
}
