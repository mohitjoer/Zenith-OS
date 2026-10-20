pragma ComponentBehavior: Bound

// The background.
//
// This exists because adoption imports a wallpaper into
// org.zenith.shell/wallpaper-path and org.gnome.shell does not exist on this
// desktop to draw it. A setting that an import writes and nothing draws is an
// import that succeeds and does nothing, which is worse than not importing: the
// adoption report says the wallpaper was carried over, and the machine is bare.
//
// It is a window rather than an item, because an item has no size of its own:
// the shell's root is not an item, so an item inside it is an item with no
// width and no height, which is a wallpaper that is never seen. The bar and
// the dock are windows for the same reason.
//
// It is below everything else, so it is on the compositor's bottom layer
// rather than its top one. A wallpaper on the top layer would be a wallpaper
// over every window on the desktop, and a wallpaper that is over the windows
// is a wallpaper the user cannot click past.

import Quickshell
import Quickshell.Widgets
import QtQuick

PanelWindow {
    id: wallpaper

    // The whole screen, on every edge. A wallpaper sized for the screen that
    // was there at login is a wallpaper with grey edges when a monitor is
    // plugged in.
    anchors {
        top: true
        bottom: true
        left: true
        right: true
    }

    // Below the windows, and taking no space of its own.
    aboveWindows: false
    exclusionMode: ExclusionMode.Ignore
    exclusiveZone: 0

    // Transparent, so the image below is the wallpaper and not a rectangle
    // with a picture on it.
    color: "transparent"

    // Config.wallpaper is already resolved to the light or dark URI by the
    // wrapper, so nothing here knows that two settings exist.
    //
    // The empty case is a drawn background rather than nothing: an unset
    // wallpaper has to be something, and the colour scheme is already decided,
    // so it follows it.
    Rectangle {
        anchors.fill: parent
        color: Theme.background
        visible: Config.wallpaper === ""
    }

    Image {
        anchors.fill: parent
        visible: Config.wallpaper !== ""
        source: Config.wallpaper
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
