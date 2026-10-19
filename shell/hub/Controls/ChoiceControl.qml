pragma ComponentBehavior: Bound

// An enum-typed setting.
//
// The choices come from the export, so the list is the schema's and not a copy
// of it. An enum whose values were added to the schema would otherwise not
// appear here until somebody remembered to add them, which is the same
// second-copy problem the window was built to avoid.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

ComboBox {
    id: control

    required property var key

    // GVariant strings arrive quoted, so the value is "'top'" and the model entry
    // is "top". Comparing them unquoted would silently match nothing and leave
    // the control showing the first entry whatever the real value is.
    readonly property string current: key.value.replace(/^'(.*)'$/, "$1")

    model: key.choices
    currentIndex: Math.max(0, key.choices.indexOf(current))

    onActivated: Zenith.set(key, "'" + key.choices[index] + "'")
}
