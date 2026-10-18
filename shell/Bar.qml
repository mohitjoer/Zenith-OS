pragma ComponentBehavior: Bound

// The bar.
//
// Widgets come from bar-widgets as a list of names, and each name is looked up
// in the catalogue below. A name that is not in the catalogue is skipped, and
// `zenith doctor` reports it — which is the point: a typo in a settings key
// should leave a visible gap and a diagnostic, not a bar that is quietly one
// widget shorter.

import Quickshell
import Quickshell.Widgets
import QtQuick

PanelWindow {
    id: bar

    // The bar follows the edge key rather than choosing one. Four layouts for
    // one setting, and no second place where the choice is made.
    readonly property int edge: config.barPosition

    // These four read `edge`, the bar's own property, and not
    // `config.edge`. The wrapper has barPosition; `edge` is what this object
    // resolved it to. Reading config.edge reads a property that does not exist,
    // which in QML is undefined rather than an error — so every comparison is
    // false and the bar picks up whichever anchor block it happens to reach.
    implicitWidth: edge === config.edgeLeft || edge === config.edgeRight
        ? config.barHeight
        : Screen.width
    implicitHeight: edge === config.edgeLeft || edge === config.edgeRight
        ? Screen.height
        : config.barHeight

    anchors {
        top: edge === config.edgeTop
        bottom: edge === config.edgeBottom
        left: edge === config.edgeLeft
        right: edge === config.edgeRight
    }

    color: "transparent"
    // Excluded from the layout on purpose: a shell that appears in the
    // compositor's own switcher cannot be moved out of the way by it.
    exclusionMode: ExclusionMode.Ignore
    exclusiveZone: config.barHeight

    Row {
        anchors.fill: parent
        anchors.margins: 4
        spacing: 8

        Repeater {
            // model is the names, not the widgets. A Repeater over Component
            // instances would build every widget whether or not the bar shows
            // it, and a widget that cannot be built would take the bar with it.
            model: config.barWidgets

            delegate: Loader {
                id: widgetLoader

                required property int index
                required property var modelData

                // Source is chosen from a lookup rather than interpolated into a
                // path. Building a URL out of a settings string means a
                // settings value decides what file the shell reads.
                readonly property string name: widgetLoader.modelData

                source: Bar.catalogue[name]
                    ? "Widgets/" + Bar.catalogue[name] + ".qml"
                    : ""

                // A widget that fails to load leaves a zero-sized hole rather
                // than taking the bar down. One broken widget should cost one
                // widget, not the session.
                width: item ? item.implicitWidth : 0
                height: item ? item.implicitHeight : 0
            }
        }
    }

    // The catalogue, as a mapping rather than as files discovered at runtime.
    //
    // Discovering widgets from the directory would make the set of widgets
    // depend on what is on the disk, which is the wrong direction: a stray file
    // in the directory would become a widget, and a missing one would be
    // discovered as absent rather than as a mistake.
    readonly property var catalogue: ({
        "workspaces": "Workspaces",
        "clock": "Clock",
        "system": "System",
        "volume": "Volume",
        "battery": "Battery",
        "network": "Network"
    })

    // The names the shell knows about. `zenith doctor` compares this against the
    // schema's default so that a key naming a widget that no longer exists is
    // reported before the user sees a gap.
    function knownWidgets() {
        return Object.keys(catalogue)
    }
}
