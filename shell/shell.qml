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
import Quickshell.Widgets
import QtQuick

ShellRoot {
    id: root

    // The session's colour scheme is read once and bound, not polled. A shell
    // that re-reads settings on a timer is a shell that redraws on a clock
    // rather than on a change. The value itself lives on the wrapper, because
    // that is what every widget asks for it through.
    readonly property bool dark: config.dark

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
    }

    Popover {
        id: launcherPopover
        width: 640
        height: 480
        Launcher {}
    }

    Popover {
        id: quickSettingsPopover
        width: 360
        height: 320
        QuickSettings {}
    }

    // The two always-present surfaces. The bar and the dock are siblings of the
    // popovers rather than children of them: a popover that closes must not take
    // the bar with it.
    // The wallpaper first, because QML draws siblings in declaration order and
    // nothing else. A wallpaper declared after the bar is a wallpaper the bar
    // is drawn over.
    Wallpaper {}

    Bar {}
    Dock {}

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
    //
    // Nothing calls advance() yet — see the keybind note in Switcher.qml.
    Switcher {}

    // Idle owns the timers and the overlay draws what they mean. The overlay is
    // declared last so it is on top of everything, which is the whole point of
    // it.
    Idle {}
    IdleOverlay {}

    // The notification popup. It is a child of the root and not of the popovers,
    // for the reason the popovers are: a notification that arrives while the
    // quick settings are open must not be dismissed by them closing, and a
    // popup that owns its own lifetime cannot be taken down by someone else's.
    Notifications {}
}
