pragma ComponentBehavior: Bound

// The bar.
//
// Widgets come from bar-widgets as a list of names, and each name is looked up
// in the catalogue below. A name that is not in the catalogue is skipped, and
// `zenith doctor` reports it — which is the point: a typo in a settings key
// should leave a visible gap and a diagnostic, not a bar that is quietly one
// widget shorter.
//
// The bar is a panel and not a strip. A strip glued to the edge of the screen
// has no edge of its own, so it cannot be told apart from the wallpaper
// behind it, and the desktop reads as one flat picture with some text on it.
// A panel has a border, corners and a shadow, and the shadow says which way
// is up: it is always thrown away from the edge of the screen, so it falls on
// the desktop and never on the panel's neighbour.
//
// The panel is inset from the edges of the screen, so the wallpaper shows
// around it, and it still asks the compositor for the space it needs. The
// space it asks for is its own thickness and the inset in front of it,
// because the compositor counts the reservation from the edge of the screen
// and the panel is no longer at the edge: a window allowed up to the edge
// would slide underneath the panel and sit half under it.

import Quickshell
import Quickshell.Widgets
import QtQuick
import QtQuick.Effects

PanelWindow {
    id: bar

    // The bar follows the edge key rather than choosing one. Four layouts for
    // one setting, and no second place where the choice is made.
    readonly property int edge: Config.barPosition

    // These four read `edge`, the bar's own property, and not
    // `Config.edge`. The wrapper has barPosition; `edge` is what this object
    // resolved it to. Reading Config.edge reads a property that does not exist,
    // which in QML is undefined rather than an error — so every comparison is
    // false and the bar picks up whichever anchor block it happens to reach.
    readonly property bool onTop: edge === Config.edgeTop
    readonly property bool onBottom: edge === Config.edgeBottom
    readonly property bool onLeft: edge === Config.edgeLeft
    readonly property bool onRight: edge === Config.edgeRight

    // Whether the bar runs along the top and bottom of the screen or along the
    // left and right of it. Nearly everything else about the bar follows from
    // this one answer: which way the chips run, which end of the bar is the
    // start of it, and which way the shadow falls.
    readonly property bool horizontal: onTop || onBottom

    // The gap between the panel and the edge of the screen.
    readonly property int inset: Theme.space2

    // The gap between the panel and the chips inside it. The chips take the
    // rest of the panel's thickness, so a bar set to a greater height has
    // taller chips rather than a taller panel with the same chips floating in
    // the middle of it.
    readonly property int padding: Theme.space1

    // The room the shadow needs on the side of the panel that faces the
    // desktop. A layer-shell surface is exactly as large as its window, so a
    // shadow drawn past the edge of the window would be clipped away; the
    // window carries the room instead.
    readonly property int shadowRoom: Theme.shadowOffset + Theme.shadowBlur

    // The panel's thickness, across the screen.
    readonly property int thickness: Config.barHeight

    // The chip's thickness: the panel's, less the gap on either side of it.
    readonly property int innerThickness: thickness - padding * 2

    // The bar is an island, not a strip.
    //
    // A strip that runs the whole width of the screen is a piece of the
    // desktop with some things on it; an island in the middle of the edge is a
    // thing on the desktop. The island is as wide as the widgets it holds and
    // no wider, which means the wallpaper is wallpaper again on either side
    // of it rather than a bar with a strip of picture under it.
    //
    // The width is the content's, and the content's width is the zones'. The
    // zones' *positions* depend on this width and their widths do not, so
    // there is no loop: the bar is as wide as it needs to be and no wider.
    readonly property int gap: Theme.space3
    readonly property int panelWidth: horizontal
        ? startZone.width + centreZone.width + endZone.width + padding * 2 + gap * 2
        : thickness
    readonly property int panelHeight: horizontal
        ? thickness
        : startZone.height + centreZone.height + endZone.height + padding * 2 + gap * 2

    // Where the island sits in the window: centred along the edge it is on.
    readonly property int panelX: horizontal
        ? (bar.width - panelWidth) / 2
        : (onRight ? bar.width - thickness : 0)
    readonly property int panelY: horizontal
        ? (onBottom ? bar.height - thickness : 0)
        : (bar.height - panelHeight) / 2

    // The way the shadow falls: away from the edge of the screen, always.
    readonly property int shadowDx: onLeft ? 1 : (onRight ? -1 : 0)
    readonly property int shadowDy: onTop ? 1 : (onBottom ? -1 : 0)

    // The size of the window. The dimension that runs along the screen is the
    // compositor's to decide, because it knows the screen and the margins
    // better than the shell does; the other is the panel and the room its
    // shadow needs.
    implicitWidth: horizontal ? Screen.width : thickness + shadowRoom
    implicitHeight: horizontal ? thickness + shadowRoom : Screen.height

    anchors {
        top: onTop || onLeft || onRight
        bottom: onBottom || onLeft || onRight
        left: onTop || onBottom || onLeft
        right: onTop || onBottom || onRight
    }

    // The inset, on every edge the bar does not lie along. A bar on the top of
    // the screen is inset from the top and from both sides, so that the
    // wallpaper shows at the corners; the margin on the opposite edge is
    // nothing, because the bar's shadow is what belongs there.
    margins {
        top: onTop || onLeft || onRight ? bar.inset : 0
        bottom: onBottom || onLeft || onRight ? bar.inset : 0
        left: onTop || onBottom || onLeft ? bar.inset : 0
        right: onTop || onBottom || onRight ? bar.inset : 0
    }

    color: "transparent"
    // Excluded from the layout on purpose: a shell that appears in the
    // compositor's own switcher cannot be moved out of the way by it.
    exclusionMode: ExclusionMode.Ignore
    // The space the bar takes from the desktop. The compositor counts it from
    // the edge of the screen, and the panel is a further inset in, so the
    // zone is the panel's own thickness and the gap in front of it.
    exclusiveZone: bar.thickness + bar.inset

    // ---------------------------------------------------------------- widgets
    //
    // The setting names the widgets; it does not say where they go. A list
    // read left to right as a bar layout is a list that has to be rewritten
    // every time a widget is added, and the wrong order is a bar with the
    // clock at one end of it and the workspaces in the middle. The zones are
    // therefore decided by what a widget is: the workspaces and the launcher
    // belong at the start of the bar, the clock belongs in the middle where
    // the eye looks for it, and the status widgets belong at the end, out of
    // the way of both.
    //
    // The order within a zone is the order of the setting, so a user who
    // wants the network before the volume still gets it.
    readonly property var startWidgets: bar.widgetsInZone("start")
    readonly property var centreWidgets: bar.widgetsInZone("centre")
    readonly property var endWidgets: bar.widgetsInZone("end")

    function zoneOf(name) {
        if (name === "clock") return "centre"
        if (name === "workspaces" || name === "launcher") return "start"
        return "end"
    }

    function widgetsInZone(zone) {
        var names = [];
        var configured = Config.barWidgets ?? [];
        for (var i = 0; i < configured.length; i++) {
            if (bar.zoneOf(configured[i]) === zone) {
                names.push(configured[i]);
            }
        }
        return names;
    }

    // A widget, in a chip.
    //
    // The chip is the whole of the widget's own surface: a rounded box, a
    // state layer when the pointer is over it, and a hairline that arrives
    // with it. The widget inside is not restyled — the chip is a frame around
    // the widget, not a second opinion about how a widget should look.
    component Chip: Rectangle {
        id: chip

        required property var modelData

        // Which way the bar runs, and how thick it is. A chip is as thick as
        // the bar across the screen and as long as its widget along it, or
        // the other way round.
        property bool vertical: false
        property int thickness: 0

        property bool hovered: false

        // A widget with nothing to show is not there: a machine with no
        // battery reports nothing rather than a chip around nothing, and a
        // name the catalogue does not know has no widget to put in a chip at
        // all. The chip goes away with it, rather than leaving a hole in the
        // bar that reads as a widget that failed.
        readonly property bool present: chip.vertical
            ? content.height > 0
            : content.width > 0

        visible: chip.present

        radius: Theme.radiusSmall
        color: chip.hovered ? Theme.stateLayer : "transparent"
        border.width: chip.hovered ? Theme.hairlineWidth : 0
        border.color: Theme.hairline

        // The widget, centred in the chip. The padding along the bar is the
        // space grid's eight; across the bar the chip is as thick as the bar
        // itself, because a chip whose height came from its own contents
        // would be a different height from the chip beside it, and a row of
        // chips of different heights is a row of chips that does not line up.
        Loader {
            id: content
            anchors.centerIn: parent

            // The source is chosen from a lookup rather than interpolated
            // into a path. Building a URL out of a settings string means a
            // settings value decides what file the shell reads.
            source: bar.catalogue[chip.modelData]
                ? "Widgets/" + bar.catalogue[chip.modelData] + ".qml"
                : ""

            // A widget that fails to load leaves a zero-sized hole rather
            // than taking the bar down. One broken widget should cost one
            // widget, not the session.
            width: item ? item.implicitWidth : 0
            height: item ? item.implicitHeight : 0
        }

        width: chip.vertical ? chip.thickness : content.width + Theme.space2 * 2
        height: chip.vertical ? content.height + Theme.space2 * 2 : chip.thickness

        HoverHandler {
            onHoveredChanged: chip.hovered = hovered
        }
    }

    // A zone of the bar: the chips in it, laid along the bar.
    //
    // A Grid rather than a Row or a Column, because the bar runs in two
    // directions and a layout that knows only one of them is a layout that
    // has to be written twice. One row for a bar along the top and the
    // bottom, one column for a bar along the left and the right.
    component Zone: Grid {
        id: zone

        property var entries: []

        columns: bar.horizontal ? 0 : 1
        rows: bar.horizontal ? 1 : 0
        spacing: Theme.space3

        Repeater {
            model: zone.entries
            delegate: Chip {
                vertical: !bar.horizontal
                thickness: bar.innerThickness
            }
        }
    }

    // The shadow under the panel.
    //
    // A real one, and the reason it can be a real one is that the module is
    // in the image: the shell's surfaces are translucent and they sit over
    // the wallpaper, and a translucent panel with a stack of rectangles under
    // it looks like a panel with a stack of rectangles under it.
    //
    // The shadow falls away from the edge of the screen, so it falls on the
    // desktop and never on the panel's neighbour.
    RectangularShadow {
        x: bar.panelX
        y: bar.panelY
        width: bar.panelWidth
        height: bar.panelHeight
        radius: bar.panelHeight / 2

        offset: Qt.point(
            bar.shadowDx * Theme.shadowOffset,
            bar.shadowDy * Theme.shadowOffset
        )
        blur: Theme.shadowBlur
        spread: 0
        color: Theme.shadow
        opacity: 0.55
    }

    // The panel itself. The window is transparent and the panel is drawn
    // inside it, because a surface is a rectangle and the panel has corners.
    Rectangle {
        id: panel

        x: bar.panelX
        y: bar.panelY
        width: bar.panelWidth
        height: bar.panelHeight

        color: Theme.panelFill
        // A stadium: the island's ends are as round as the bar is thick, which
        // is what makes it an island and not a rectangle that happens to be
        // short.
        radius: height / 2
        border.width: Theme.hairlineWidth
        border.color: Theme.hairline

        // The start of the bar. The left on a bar along the top or the
        // bottom, the top on a bar along the left or the right: the bar is
        // read in the order the screen is read.
        Zone {
            id: startZone
            entries: bar.startWidgets

            width: implicitWidth
            height: implicitHeight
            x: bar.horizontal ? bar.padding : (bar.panelWidth - width) / 2
            y: bar.horizontal ? (bar.panelHeight - height) / 2 : bar.padding
        }

        // The middle of the bar, where the clock lives. It is centred on the
        // middle of the panel, which is the middle of the screen, on every
        // edge the bar can be on.
        Zone {
            id: centreZone
            entries: bar.centreWidgets

            width: implicitWidth
            height: implicitHeight
            x: (bar.panelWidth - width) / 2
            y: (bar.panelHeight - height) / 2
        }

        // The end of the bar, which is where the status widgets go: the
        // things that are read rather than acted on.
        Zone {
            id: endZone
            entries: bar.endWidgets

            width: implicitWidth
            height: implicitHeight
            x: bar.horizontal
                ? bar.panelWidth - width - bar.padding
                : (bar.panelWidth - width) / 2
            y: bar.horizontal
                ? (bar.panelHeight - height) / 2
                : bar.panelHeight - height - bar.padding
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
