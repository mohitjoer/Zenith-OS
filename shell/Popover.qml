pragma ComponentBehavior: Bound

// A surface that appears anchored to an edge and closes itself.
//
// One implementation rather than a base class plus a subclass per surface,
// because every property of a popover — which edge, how wide, whether it is
// open — is used by all of them, and a base class here would be a file whose
// body is a list of `property` declarations and nothing else.

import Quickshell
import Quickshell.Widgets
import QtQuick

PopupWindow {
    id: popover

    // The surface's own content. Assigned by whoever opens it, so adding a
    // surface does not mean adding a file that exists only to hold a child.
    default property alias content: surface.data

    // Closed on Escape from the inside as well as by the caller from the
    // outside, because a surface that can only be closed by the thing that
    // opened it is a surface a keyboard cannot get rid of.
    onVisibleChanged: if (!visible) surface.opened = false

    Item {
        id: surface
        property bool opened: false
    }
}
