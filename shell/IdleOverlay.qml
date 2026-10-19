pragma ComponentBehavior: Bound

// What is drawn while the session is idle, before it locks.
//
// This answers a question that was open rather than implementing a feature:
// does Zenith want its own surface while idle, given that hyprlock covers the
// screen a moment later? The answer is a dim, and it earns its place — the gap
// between idle and lock is exactly the window where someone walks past.
//
// It does not blank the display and does not claim to. Power saving is the
// compositor's job, and a shell that drew an opaque black rectangle would be
// making the screen look off while the panel kept drawing at full power. What it
// does is stop the content being legible, which is the actual concern.

import Quickshell
import Quickshell.Widgets
import QtQuick

Item {
    id: idleOverlay

    // Covers the whole screen and takes no input. A scrim that eats clicks
    // would make an idle session feel broken to someone touching the mouse; one
    // that lets clicks through is a scrim, which is the point.
    anchors.fill: parent
    z: 1000

    property bool dimmed: idle.dimmed
    property bool blanked: idle.blanked

    readonly property real opacityFor: blanked ? 1.0 : dimmed ? 0.6 : 0.0

    Rectangle {
        anchors.fill: parent
        // Black on purpose, and not Palette.background. A scrim darkens
        // whatever is behind it, so it has to be independent of the palette's
        // own background — a tinted scrim would tint the wallpaper too.
        color: "#000000"
        opacity: idleOverlay.opacityFor
        // Animated rather than snapped: a screen that blanks instantly is a
        // flicker, and a flicker on every idle is a fault report.
        Behavior on opacity {
            NumberAnimation {
                duration: 400
                easing.type: Easing.OutQuad
            }
        }
    }

    // Clicking or pressing a key wakes the session, and re-arms every timer.
    // Without this the only way back is to wait out the lock timeout and
    // authenticate against a locker nobody asked for.
    TapHandler {
        onTapped: idle.bump()
    }

    Keys.onPressed: idle.bump()
}
