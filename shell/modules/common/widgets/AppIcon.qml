import Quickshell
import Quickshell.Widgets
import QtQuick

// An application icon.
//
// Upstream's version of this widget is a KDE Frameworks `Kirigami.Icon`,
// and the base layer carries no KDE Frameworks: one widget is not a reason
// to put a whole application framework on a desktop whose manifest
// deliberately excludes it.
//
// So this is Zenith's, at upstream's path, with the same two properties the
// one consumer uses — a source and a size — and nothing else.
//
// `IconImage` takes the same source a `Kirigami.Icon` does: a URL, not a
// theme name. The caller resolves it with `Quickshell.iconPath`, which is
// where the freedesktop icon theme is read from.
IconImage {
    id: root

    // The size of the icon.
    //
    // Named `implicitSize` because the caller sets it that way, and a
    // property renamed for style is a property a caller cannot set.
    property real implicitSize: 26

    implicitWidth: implicitSize
    implicitHeight: implicitSize

    // The image is drawn at the size it is given, not rounded down to a
    // theme's nominal size.
    fillMode: Image.PreserveAspectFit
    smooth: true
    cache: true
}
