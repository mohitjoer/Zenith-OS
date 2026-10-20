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

    // The window paints nothing. A window that paints its own background is a
    // rectangle with square corners, and every surface in this shell is a
    // rectangle with rounded ones — so the window is the frame and the
    // surface inside it is the card, and the two are not the same thing.
    color: "transparent"

    // Escape closes from the inside as well as by the caller from the outside,
    // because a surface that can only be closed by the thing that opened it is
    // a surface a keyboard cannot get rid of.
    //
    // A shortcut rather than the Keys attached property, because a popup is a
    // window and not an item, and Keys needs an item. The shortcut lives in
    // the window's content, which is where a key press lands anyway.
    Shortcut {
        sequence: "Escape"
        onActivated: popover.close()
    }

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

    // Where the surface sits, in the shell's own words.
    //
    // The bar and the dock name the edge of the screen they belong to, and
    // every other surface in the shell names one the same way. Quickshell
    // names it differently: a point on a rectangle, and a direction to grow in
    // from that point. The translation lives here, once, so that a surface
    // says where it goes in the same terms as every other surface and no
    // surface has to know the compositor's vocabulary.
    //
    // The rectangle is the surface this one belongs to — the dock, for
    // everything the dock opens — because that is the only rectangle the
    // compositor can place a popup against, and because a popup placed against
    // the dock is a popup placed against the edge of the screen the dock is
    // on. `anchorItem` is that surface's content; without it the popup has
    // nothing to be placed against and lands wherever the compositor decides.
    property Item anchorItem: null

    // The edge of the screen this surface belongs to. One vertical and one
    // horizontal at most: the compositor refuses opposing edges, and a surface
    // that asked for the top and the bottom of the same thing meant neither.
    //
    // Four properties rather than one object with four, because a property
    // holding an object cannot be written to in a binding, and a surface whose
    // edge is fixed at the moment it is created is a surface that does not
    // follow the dock when the dock moves.
    property bool edgeTop: false
    property bool edgeBottom: false
    property bool edgeLeft: false
    property bool edgeRight: false

    // How far from that edge the surface sits. It is an inset of the rectangle
    // it hangs off, which is the only direction a margin can take that keeps
    // the surface on the screen.
    property int margin: 12

    // The edges the compositor attaches the popup to, and the direction it
    // grows in.
    //
    // The direction is the one toward the middle of the screen from the
    // surface the popup hangs off, because that is the direction that keeps
    // the popup on the screen: a popup that grew away from the screen would be
    // a popup off it. It is read from where the dock is rather than guessed
    // from the edge named above, because the dock's screen edge and the side
    // of it that faces the screen are two different edges and a popup hanging
    // from either grows the same way.
    readonly property bool verticalAnchor: edgeTop || edgeBottom
    readonly property int inwardVertical: Config.dockPosition === Config.edgeTop
        ? Edges.Bottom
        : Edges.Top
    readonly property int inwardHorizontal: Config.dockPosition === Config.edgeLeft
        ? Edges.Right
        : Edges.Left

    anchor.item: anchorItem
    anchor.edges: (edgeTop ? Edges.Top : Edges.None)
        | (edgeBottom ? Edges.Bottom : Edges.None)
        | (edgeLeft ? Edges.Left : Edges.None)
        | (edgeRight ? Edges.Right : Edges.None)
    anchor.gravity: verticalAnchor ? inwardVertical : inwardHorizontal
    anchor.margins.left: margin
    anchor.margins.right: margin
    anchor.margins.top: margin
    anchor.margins.bottom: margin
}
