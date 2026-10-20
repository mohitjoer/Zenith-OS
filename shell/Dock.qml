pragma ComponentBehavior: Bound

// The dock.
//
// A floating panel: a rounded surface held a little way off the edge of the
// screen, carrying the applications the user reaches for. It is the one
// surface in the shell that is drawn over the desktop rather than beside it,
// so it is also the one surface that has to say where it belongs — a hairline
// around it and a shadow under it, both thrown away from the edge, so that
// the panel reads as floating over the wallpaper rather than as a strip of it.
//
// favourites are desktop file IDs, not paths, because that is what
// org.gnome.shell favorite-apps holds and adoption imports them across
// unchanged. A path here would have to be rewritten during adoption, and a
// rewritten desktop file ID is one nobody can compare against what GNOME had.
//
// An empty list of favourites is not an empty dock. A dock with nothing in it
// is a strip of nothing, and the machine's own catalogue is right there: when
// the user has not chosen anything, the dock offers a small set of
// applications the machine actually has.

import Quickshell
import Quickshell.Hyprland
import Quickshell.Widgets
import QtQuick

PanelWindow {
    id: dock

    readonly property int edge: Config.dockPosition

    // The dock stands on its side on the left and the right and lies flat on
    // the top and the bottom. Everything below is written once and read
    // through this, because the two layouts differ in nearly every
    // measurement and a dock that had to be written twice would be two docks
    // that had drifted apart.
    readonly property bool vertical: edge === Config.edgeLeft || edge === Config.edgeRight

    // ------------------------------------------------------------------ sizes
    //
    // A cell is the space one application takes, the icon is what is drawn
    // inside it, and the gap is what separates one cell from the next. The
    // padding is the space between the cells and the edge of the panel.
    readonly property int cellSize: 48
    readonly property int iconSize: 30
    readonly property int cellGap: Theme.space3
    readonly property int padding: Theme.space2

    // Where the icon and the mark sit inside a cell.
    //
    // The icon is a little above the middle, because the mark needs room
    // under it, and the mark is a little above the bottom of the cell,
    // because a mark flush with the edge of the panel is a mark that belongs
    // to the panel and not to the application.
    readonly property int iconTop: 5
    readonly property int markTop: 39

    // The space kept between the panel and the edge of the screen.
    //
    // A dock flush with the edge is a bar. A dock held off it is a panel
    // floating over the desktop, and the difference between the two is the
    // whole of the look.
    readonly property int inset: Theme.space2

    // The band the shadow is given.
    //
    // A shadow is a blur and an offset, so it needs a room of its own on the
    // side away from the screen edge. Without one it would fall off the
    // window and be cut in half by the edge of the screen.
    readonly property int shadowBand: Theme.shadowBlur + Theme.shadowOffset

    // The panel's own measurements.
    readonly property int panelThickness: cellSize + padding * 2
    readonly property int panelWidth: vertical
        ? panelThickness
        : items.length * cellSize + Math.max(0, items.length - 1) * cellGap + padding * 2
    readonly property int panelHeight: vertical
        ? items.length * cellSize + Math.max(0, items.length - 1) * cellGap + padding * 2
        : panelThickness

    // The window is larger than the panel: the shadow is given its band on
    // the side away from the screen edge, and the panel is held off the edge
    // by the inset on the other.
    implicitWidth: vertical ? panelThickness + inset + shadowBand : panelWidth
    implicitHeight: vertical ? panelHeight : panelThickness + inset + shadowBand

    // The edge the dock stands on.
    anchors {
        top: edge === Config.edgeTop
        bottom: edge === Config.edgeBottom
        left: edge === Config.edgeLeft
        right: edge === Config.edgeRight
    }

    // The panel's place in the window, and the direction the shadow is thrown.
    //
    // The shadow always falls away from the edge of the screen, so it falls on
    // the desktop and never on the neighbour.
    readonly property int panelX: edge === Config.edgeLeft ? inset
        : edge === Config.edgeRight ? shadowBand
        : 0
    readonly property int panelY: edge === Config.edgeTop ? inset
        : edge === Config.edgeBottom ? shadowBand
        : 0
    readonly property int shadowOffsetX: edge === Config.edgeLeft ? Theme.shadowOffset
        : edge === Config.edgeRight ? -Theme.shadowOffset
        : 0
    readonly property int shadowOffsetY: edge === Config.edgeTop ? Theme.shadowOffset
        : edge === Config.edgeBottom ? -Theme.shadowOffset
        : 0

    // ---------------------------------------------------------------- autohide
    //
    // Autohide is a property of the panel's geometry, not an animation played
    // over a fixed window: an animated hide leaves a strip the pointer has to
    // find, and the strip is the thing autohide is supposed to remove.
    //
    // What is left is three pixels of panel at the edge of the screen, and
    // the strip of screen between the panel and the edge is the way the dock
    // is reached: the pointer arriving there is what brings the dock back.
    readonly property int travel: revealed ? 0 : inset + panelThickness - 3

    readonly property int groupX: edge === Config.edgeLeft ? panelX - travel
        : edge === Config.edgeRight ? panelX + travel
        : panelX
    readonly property int groupY: edge === Config.edgeTop ? panelY - travel
        : edge === Config.edgeBottom ? panelY + travel
        : panelY

    // Whether the dock is on screen.
    //
    // It is a decision and not a reading, because the dock is told to go by
    // the pointer leaving it and is told to come back by the pointer arriving
    // at the edge, and those are two different things at two different times.
    property bool revealed: true

    // Whether the pointer is at the edge of the screen, and whether it is on
    // the panel.
    //
    // Both are read from the gutter and the panel rather than from the
    // window, because the window's input region is fixed and a region that
    // moved under the pointer would be a region the compositor took the
    // pointer away from.
    property bool atEdge: false
    property bool panelHover: false
    readonly property bool onPanel: dock.panelHover || dock.hoveredIndex >= 0
    readonly property bool pointerHere: dock.atEdge || dock.onPanel

    // The pointer arriving at the edge brings the dock back, and the pointer
    // leaving the dock sends it away.
    //
    // The going is late, because the panel slides out of the screen over the
    // pointer's own position, and a dock that hid itself under the pointer
    // would be a dock that came straight back.
    onPointerHereChanged: {
        if (dock.atEdge) {
            dock.revealed = true;
            hiding.stop();
            return;
        }
        if (dock.onPanel) {
            hiding.stop();
            return;
        }
        hiding.restart();
    }

    Timer {
        id: hiding
        interval: 300
        onTriggered: dock.revealed = !Config.dockAutohide
    }

    // The setting decides whether the dock is on screen at all.
    Component.onCompleted: dock.revealed = !Config.dockAutohide

    Connections {
        target: Config
        function onDockAutohideChanged() {
            dock.revealed = !Config.dockAutohide;
        }
    }

    // The strip of screen between the panel and the edge of the screen.
    readonly property int gutterX: edge === Config.edgeRight ? implicitWidth - inset : 0
    readonly property int gutterY: edge === Config.edgeBottom ? implicitHeight - inset : 0
    readonly property int gutterWidth: vertical ? inset : implicitWidth
    readonly property int gutterHeight: vertical ? implicitHeight : inset

    color: "transparent"
    // Excluded from the layout when it is hidden, and a reserved strip of
    // screen when it is not: a pinned dock is a thing the desktop is built
    // around, and an autohiding one is a thing the desktop is not.
    exclusionMode: ExclusionMode.Normal
    exclusiveZone: Config.dockAutohide ? 0 : panelThickness + inset

    // The window's input region.
    //
    // The panel is what the pointer can touch, and so is the strip of screen
    // between the panel and the edge. The band the shadow falls in is not: a
    // shadow that swallowed clicks would be a shadow with a hole in the
    // desktop under it.
    //
    // The region is the panel's place and not the panel itself, and it does
    // not move when the panel slides out of the screen. A region that moves
    // under the pointer is a region the compositor takes the pointer away
    // from, and the dock that was on its way back would go out again.
    mask: Region {
        item: footprint
        Region {
            item: gutter
        }
    }

    // The place the panel occupies when it is on screen.
    Item {
        id: footprint

        x: dock.panelX
        y: dock.panelY
        width: dock.panelWidth
        height: dock.panelHeight
    }

    // The strip of screen between the panel and the edge.
    Item {
        id: gutter

        x: dock.gutterX
        y: dock.gutterY
        width: dock.gutterWidth
        height: dock.gutterHeight

        MouseArea {
            anchors.fill: parent
            hoverEnabled: true
            onContainsMouseChanged: dock.atEdge = containsMouse
        }
    }

    // ------------------------------------------------------------------- items
    //
    // The applications the dock offers.
    readonly property var items: dock.resolveItems()

    // The item the pointer is on, and the name that follows it.
    property int hoveredIndex: -1
    property Item hoveredItem: null

    Timer {
        id: dwell
        interval: 400
        onTriggered: label.opened = true
    }

    Item {
        id: group
        x: dock.groupX
        y: dock.groupY

        Behavior on x {
            NumberAnimation {
                duration: 250
                easing.type: Easing.OutCubic
            }
        }
        Behavior on y {
            NumberAnimation {
                duration: 250
                easing.type: Easing.OutCubic
            }
        }

        // The shadow.
        //
        // A blur without a shader is a stack of rounded rectangles, each a
        // little larger and a little fainter than the one inside it, which is
        // what a blur of a rounded shape amounts to. The weights fall away
        // towards the outside so the shadow is heaviest where it touches the
        // panel and gone by the time it reaches the blur's radius.
        Repeater {
            model: 6

            delegate: Rectangle {
                required property int index

                readonly property int grow: (index + 1) * Math.ceil(dock.shadowBand / 6)
                readonly property real weight: (6 - index) / 21

                x: -grow + dock.shadowOffsetX
                y: -grow + dock.shadowOffsetY
                width: dock.panelWidth + grow * 2
                height: dock.panelHeight + grow * 2
                radius: Theme.radiusLarge + grow
                color: Theme.shadow
                opacity: weight
            }
        }

        // The panel.
        Rectangle {
            id: panel

            width: dock.panelWidth
            height: dock.panelHeight
            radius: Theme.radiusLarge
            color: Theme.panelFill
            border.width: Theme.hairlineWidth
            border.color: Theme.hairline

            // The panel itself, under the cells.
            //
            // It is here so that the pointer is known to be on the panel even
            // in the gap between two cells, and an autohiding dock that hid
            // itself under the pointer would be a dock that could not be
            // reached.
            MouseArea {
                anchors.fill: parent
                hoverEnabled: true
                onContainsMouseChanged: dock.panelHover = containsMouse
            }

            Grid {
                id: cells
                anchors.centerIn: parent
                columns: dock.vertical ? 1 : Math.max(1, dock.items.length)
                spacing: dock.cellGap

                Repeater {
                    model: dock.items

                    delegate: Item {
                        id: cell

                        required property var modelData
                        required property int index

                        width: dock.cellSize
                        height: dock.cellSize

                        // Whether the application has a window open, and
                        // whether that window is the one the compositor has
                        // focused.
                        readonly property bool running: dock.isRunning(cell.modelData.id)
                        readonly property bool focused: dock.focusedClass === cell.modelData.id

                        property bool hovered: false

                        // The state layer, under the icon.
                        Rectangle {
                            anchors.fill: parent
                            radius: Theme.radiusMedium
                            color: Theme.stateLayer
                            visible: cell.hovered
                        }

                        // The application's own icon.
                        IconImage {
                            x: (cell.width - dock.iconSize) / 2
                            y: dock.iconTop
                            width: dock.iconSize
                            height: dock.iconSize
                            source: dock.iconSource(cell.modelData)
                        }

                        // The mark that says the application is running.
                        //
                        // A dot for a window that is open, and a bar for the
                        // window the compositor has focused: the focused one
                        // is the window the pointer will come back to, and it
                        // should not look like the rest.
                        //
                        // The ink is the theme's text colour and the weight is
                        // carried by the mark itself, because a colour and an
                        // alpha are not the same thing and a mark that asked
                        // a string for its channels would get none.
                        Rectangle {
                            anchors.horizontalCenter: parent.horizontalCenter
                            y: dock.markTop
                            height: 3
                            radius: 1.5
                            width: cell.focused ? cell.width : 3
                            visible: cell.running || cell.focused
                            color: Theme.text
                            opacity: 0.6
                        }

                        MouseArea {
                            anchors.fill: parent
                            hoverEnabled: true
                            onEntered: {
                                cell.hovered = true;
                                dock.enter(cell.index, cell);
                            }
                            onExited: {
                                cell.hovered = false;
                                dock.leave(cell.index);
                            }
                            onClicked: dock.launch(cell.modelData)
                        }
                    }
                }
            }
        }
    }

    // The name of the application under the pointer.
    //
    // It is a window of its own because the panel is not wide enough to hold
    // it: a name is wider than a cell, and a label that ran off the end of
    // the window would be a name cut in half.
    Popover {
        id: label

        anchorItem: dock.hoveredItem
        margin: Theme.space2
        edgeTop: dock.edge === Config.edgeBottom
        edgeBottom: dock.edge === Config.edgeTop
        edgeLeft: dock.edge === Config.edgeRight
        edgeRight: dock.edge === Config.edgeLeft

        implicitWidth: labelBox.width
        implicitHeight: labelBox.height

        Rectangle {
            id: labelBox

            width: labelText.width + Theme.space2 * 2
            height: labelText.height + Theme.space1 * 2
            radius: Theme.radiusSmall
            color: Theme.panelFill
            border.width: Theme.hairlineWidth
            border.color: Theme.hairline

            Text {
                id: labelText
                anchors.centerIn: parent
                text: dock.hoveredIndex >= 0 ? dock.items[dock.hoveredIndex].name : ""
                color: Theme.text
                font.pixelSize: 12
            }
        }
    }

    // --------------------------------------------------------------- catalogue
    //
    // The applications the dock offers.
    //
    // The user's own favourites when there are any, and a small set of
    // applications the machine has when there are not. Each entry of the
    // starter set is a role the dock wants filled and the applications that
    // can fill it, best first, so a machine with one terminal and a machine
    // with three still get one terminal.
    function resolveItems(): var {
        var chosen = [];

        if (Config.favorites && Config.favorites.length > 0) {
            for (var i = 0; i < Config.favorites.length; i++) {
                var favourite = findApplication(Config.favorites[i]);
                if (favourite) {
                    chosen.push(favourite);
                    continue;
                }
                // An application the machine does not have is still worth a
                // place: it may be installed later, and a hole in the dock is
                // better than a picture of an application that is not there.
                chosen.push({
                    id: Config.favorites[i],
                    name: Config.favorites[i],
                    icon: "",
                    exec: Config.favorites[i],
                    args: [],
                    terminal: false
                });
            }
            return chosen;
        }

        var roles = [
            ["org.gnome.Nautilus"],
            ["com.mitchellh.ghostty", "org.gnome.Ptyxis", "org.gnome.Terminal"],
            ["org.gnome.TextEditor", "org.gnome.gedit"],
            ["brave-browser", "google-chrome", "firefox"],
            ["org.gnome.Settings"]
        ];

        for (var role = 0; role < roles.length; role++) {
            for (var candidate = 0; candidate < roles[role].length; candidate++) {
                var application = findApplication(roles[role][candidate]);
                if (application) {
                    chosen.push(application);
                    break;
                }
            }
        }
        if (chosen.length > 0) return chosen;

        // A machine with none of the usual applications still gets a dock.
        var catalogue = Apps.applications || [];
        for (var other = 0; other < catalogue.length && chosen.length < 4; other++) {
            chosen.push(catalogue[other]);
        }
        return chosen;
    }

    function findApplication(id: string): var {
        var catalogue = Apps.applications || [];
        for (var i = 0; i < catalogue.length; i++) {
            if (catalogue[i].id === id) return catalogue[i];
        }
        return null;
    }

    // An application's icon, as something an image can open.
    //
    // A desktop entry's icon is either a name in the icon theme or a path to
    // a file, and the two are not the same thing to an image: a name is looked
    // up in the theme, and a path has to be a URL. The tool that reads the
    // machine's desktop files already knows the difference, so the shell only
    // has to carry it through.
    function iconSource(app: var): string {
        var icon = app && app.icon ? app.icon : "";
        if (icon === "") return "";
        if (icon.indexOf("file://") === 0) return icon;
        if (icon.charAt(0) === "/") return "file://" + icon;
        return "image://icon/" + icon;
    }

    // Run one.
    //
    // An application that wants a terminal gets one, through the desktop's
    // own helper: which terminal that is, is the user's decision and the
    // helper's business. An application run without a terminal opens a window
    // and then exits, which is an application that looks broken.
    function launch(app: var): void {
        var command = [app.exec].concat(app.args || []);
        if (app.terminal) {
            Quickshell.execDetached(["xdg-terminal-exec"].concat(command));
            return;
        }
        Quickshell.execDetached(command);
    }

    // ---------------------------------------------------------------- hovering
    //
    // The pointer has arrived at an item. The name waits for the pointer to
    // settle before it appears, because a pointer crossing the dock on its
    // way somewhere else should not leave a name behind it.
    function enter(index: int, item: Item): void {
        dock.hoveredIndex = index;
        dock.hoveredItem = item;
        label.opened = false;
        dwell.restart();
    }

    function leave(index: int): void {
        if (dock.hoveredIndex !== index) return;
        dock.hoveredIndex = -1;
        dock.hoveredItem = null;
        label.opened = false;
        dwell.stop();
    }

    // ----------------------------------------------------------------- windows
    //
    // The windows the compositor knows about.
    readonly property var windows: Hyprland.toplevels ? Hyprland.toplevels.values : []

    // The application the compositor has focused.
    //
    // The compositor says which window it focused when that changes, so a
    // shell that started after the window was opened has not been told. The
    // focus history is what the compositor already knows, and it is right
    // until the next focus change tells the shell something newer.
    readonly property string focusedClass: {
        var active = Hyprland.activeToplevel;
        if (active) return windowClass(active);
        for (var i = 0; i < windows.length; i++) {
            var ipc = windows[i].lastIpcObject;
            if (ipc && ipc["focusHistoryID"] === 0) return windowClass(windows[i]);
        }
        return "";
    }

    function windowClass(toplevel: var): string {
        if (!toplevel || !toplevel.lastIpcObject) return "";
        var ipc = toplevel.lastIpcObject;
        if (ipc["class"]) return ipc["class"];
        if (ipc["initialClass"]) return ipc["initialClass"];
        return "";
    }

    // Whether an application has a window open.
    function isRunning(id: string): bool {
        for (var i = 0; i < windows.length; i++) {
            var ipc = windows[i].lastIpcObject;
            if (!ipc) continue;
            if (ipc["class"] === id || ipc["initialClass"] === id) return true;
        }
        return false;
    }
}
