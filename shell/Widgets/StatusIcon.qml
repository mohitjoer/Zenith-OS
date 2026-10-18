pragma ComponentBehavior: Bound

// One status icon.
//
// A small type rather than a repeated block, because the status area grows and
// each new entry as a copy of the previous one is a place for the three to drift
// apart.
//
// It draws a glyph, and that is the whole reason Icons.qml exists: the
// alternative is a Rectangle, which is what this was before the archive's font
// question was settled. A rectangle is a shape that means nothing, and a status
// area full of shapes is a status area that has told the user nothing.

import Quickshell
import QtQuick

Item {
    id: icon

    implicitWidth: 16
    implicitHeight: 16

    // Two properties rather than one `state` string plus a lookup table at each
    // call site: the caller says what it means and the widget decides what that
    // looks like, so the meaning is not spelled differently in four places.
    property string label: ""
    property string glyph: ""

    // An icon with no glyph is not drawn as a box. Icons.glyph returns "" for a
    // name it does not have, and an absent icon is honest where a wrong one is
    // not.
    readonly property bool drawable: glyph !== "" && Icons.glyph(glyph) !== ""

    Text {
        anchors.fill: parent
        visible: icon.drawable
        text: Icons.glyph(icon.glyph)
        font.family: Icons.iconFamily
        font.pixelSize: 14
        color: config.dark ? "#e6e6e6" : "#202020"
    }
}
