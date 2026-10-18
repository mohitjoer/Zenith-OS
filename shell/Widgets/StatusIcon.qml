pragma ComponentBehavior: Bound

// One status icon.
//
// A small type rather than a repeated block, because the status area grows and
// each new entry as a copy of the previous one is a place for the three to
// drift apart.

import Quickshell
import QtQuick

Item {
    id: icon

    implicitWidth: 16
    implicitHeight: 16

    // Two properties rather than one `state` string plus a lookup table at the
    // call site: the caller says what it means and the widget decides what that
    // looks like, so the meaning is not spelled differently at each call site.
    property string label: ""
    property string state: ""

    property color colour: config.dark ? "#e6e6e6" : "#202020"

    Rectangle {
        anchors.fill: parent
        radius: 3
        // The glyph is text rather than an image file. An icon font is already a
        // documented open question in this project, and a widget that depends on
        // one cannot be rendered at all on a machine without it — whereas text
        // always renders to something.
        color: icon.colour
        opacity: icon.state === "off" || icon.state === "zero" || icon.state === "muted"
            ? 0.35
            : 1.0
    }
}
