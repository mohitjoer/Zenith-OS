pragma ComponentBehavior: Bound

// A boolean setting.
//
// Writes through `zenith config set`, not through GSettings, so every write to
// dconf on this desktop goes through the one tool that validates it. A control
// that writes dconf itself can put a value in that is outside the key's range,
// and nothing would report it until the shell acted on it.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

Switch {
    id: control

    required property var key

    // The value from the export is a GVariant literal — "'true'" — not a
    // boolean. Stripping the quotes is what makes this work, and getting it
    // wrong gives a switch that renders from the inverse of the truth.
    readonly property bool current: key.value === "'true'"

    checked: current
    onToggled: {
        const next = (!checked) ? "false" : "true"
        Zenith.set(key, next)
    }
}
