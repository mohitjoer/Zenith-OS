pragma ComponentBehavior: Bound

// An integer setting.
//
// The range comes from the export, so a key's declared bounds and the bounds
// this control enforces are the same numbers rather than two that can disagree.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

SpinBox {
    id: control

    required property var key

    // A GVariant int arrives as a bare number, which parseInt handles; the
    // string trim is for the "0" case where the value is absent and the default
    // stands in.
    readonly property int current: parseInt(key.value, 10)

    from: key.hasRange ? key.minimum : -999999
    to: key.hasRange ? key.maximum : 999999
    value: current

    // Written on release rather than on every keystroke. A spin box that writes
    // on each keystroke commits a value per character, and a user who types 300
    // and then backspaces commits 3, 30 and then 3.
    onActiveFocusChanged: if (!activeFocus) {
        Zenith.set(key, "" + value)
    }
}
