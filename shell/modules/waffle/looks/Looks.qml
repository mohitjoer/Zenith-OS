pragma Singleton
import QtQuick
import Quickshell
import qs.modules.common

// The one name a vendored widget still reaches for from upstream's Windows-style
// "waffle" look. CalendarView animates its scroll through
// Looks.transition.scroll, and its `import qs.modules.waffle.looks` has to
// resolve or Quickshell's import scanner reports it on every start.
//
// Zenith does not ship that look, so this is an overlay at the import path the
// vendored file expects: the same motion every other vendored widget uses, taken
// from Appearance rather than upstream's separate waffle curves.
Singleton {
    readonly property QtObject transition: QtObject {
        readonly property Component scroll: Appearance.animation.elementMove.numberAnimation
    }
}
