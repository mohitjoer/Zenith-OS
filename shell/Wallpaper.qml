pragma ComponentBehavior: Bound

// The background.
//
// This exists because adoption imports a wallpaper into
// org.zenith.shell/wallpaper-path and org.gnome.shell does not exist on this
// desktop to draw it. A setting that an import writes and nothing draws is an
// import that succeeds and does nothing, which is worse than not importing: the
// adoption report says the wallpaper was carried over, and the machine is bare.
//
// It is the bottom of the stack, so it is a child of the root declared before
// everything else. QML draws siblings in declaration order and nothing else,
// so a wallpaper declared after the bar is a wallpaper behind it.

import Quickshell
import Quickshell.Widgets
import QtQuick

Item {
    id: wallpaper

    // Anchors to the whole screen rather than sized to it, because the screen
    // can change — a monitor plugged in, a lid opened — and a wallpaper sized for
    // the screen that was there at login is a wallpaper with grey edges.
    anchors.fill: parent

    // config.wallpaper is already resolved to the light or dark URI by the
    // wrapper, so nothing here knows that two settings exist.
    //
    // The empty case is a drawn background rather than nothing: an unset
    // wallpaper has to be something, and the colour scheme is already decided,
    // so it follows it.
    Rectangle {
        anchors.fill: parent
        color: config.dark ? "#111114" : "#fdfdfe"
        visible: config.wallpaper === ""
    }

    Image {
        anchors.fill: parent
        visible: config.wallpaper !== ""
        source: config.wallpaper
        asynchronous: true
        cache: true
        // Scale rather than crop. Cropping silently cuts off the edges of
        // whatever the user set, and a wallpaper whose subject is half a face
        // looks like a rendering bug rather than a choice.
        fillMode: Image.PreserveAspectCrop
        // A wallpaper path that does not load falls back to the rectangle
        // behind it rather than to a broken-image placeholder over nothing.
        onStatusChanged: if (status === Image.Error) source = ""
    }
}
