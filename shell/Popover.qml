pragma ComponentBehavior: Bound

// A surface that appears anchored to an edge and can be closed from inside.
//
// Nothing here redirects children. PopupWindow already puts them in its
// contentItem, and an earlier version of this file added a `default property
// alias` onto an Item it never sized or anchored — so every child of every
// popover landed at 0,0 in a zero-sized parent, and the popover rendered empty
// on a machine that had working QML. Adding an indirection that does nothing is
// worse than adding nothing.

import Quickshell
import Quickshell.Widgets
import QtQuick

PopupWindow {
    id: popover

    // Escape closes from the inside as well as by the caller from the outside,
    // because a surface that can only be closed by the thing that opened it is
    // a surface a keyboard cannot get rid of.
    Keys.onEscapePressed: close()

    function close() {
        if (popover.opened) {
            popover.opened = false
        }
    }

    // The single source of truth for "is this open", so the shell tracks one
    // property rather than each popover holding a boolean nobody else can set.
    //
    // Written as an explicit property with a change handler rather than
    // `readonly property bool opened: visible`: the root decides what is open,
    // and a popover that derived it from its own visibility would make the
    // decision local to each one.
    property bool opened: false

    onOpenedChanged: visible = opened

    // Closing the surface closes the decision that made it open, rather than
    // leaving the root thinking a popover is still up.
    onVisibleChanged: if (!visible) opened = false
}
