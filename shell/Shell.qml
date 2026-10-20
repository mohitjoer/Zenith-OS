pragma ComponentBehavior: Bound

// The keyboard shortcuts.
//
// The list of what the keyboard does, grouped by what it is for, with every
// chord drawn as the keys it is made of. It is the catalogue — the same one
// `zenith binds` prints and the same one the compositor's configuration is
// checked against — drawn rather than printed, so that a person who has
// forgotten a key can look it up without opening a terminal.
//
// The name is the shell's, and not the sheet's, because what the file holds
// is the shell's own list of the keybindings: the ones the compositor calls
// the shell's are in it, and so are the ones the compositor keeps for itself.
//
// The catalogue is read through the tool rather than off the file, for the
// reason the launcher reads its applications the same way: a file read twice
// is read by two pieces of code, and the two agree until the day one of them
// is changed. The tool knows the format; the shell asks it.
//
// Read once, at start-up. The catalogue does not change while a session is
// up, and a surface that re-reads it on a timer is a shell that is never
// idle.

import Quickshell
import Quickshell.Io
import QtQuick

Item {
    id: sheet

    // The measurements the sheet is made of.
    //
    // They are properties rather than numbers written into the layout
    // because the sheet's height is worked out from them, and a sheet that
    // is the wrong height is a sheet with its last rows cut off.
    readonly property int rowHeight: 22
    readonly property int rowSpacing: Theme.space1
    readonly property int headingHeight: 16
    readonly property int groupSpacing: Theme.space4
    readonly property int columnSpacing: Theme.space5
    readonly property int margin: Theme.space4
    readonly property int columnWidth: 360

    // The catalogue, as it arrived.
    property var categories: []
    property var binds: []

    // The categories, with the bindings that belong to them.
    readonly property var groups: group(categories, binds)

    // The columns, and where they are split.
    readonly property int splitAt: splitPoint(groups)
    readonly property var columns: [
        lines(groups.slice(0, splitAt)),
        lines(groups.slice(splitAt))
    ]

    implicitWidth: columnWidth * 2 + columnSpacing + margin * 2
    implicitHeight: margin * 2 + Math.max(
        columnHeight(columns[0]), columnHeight(columns[1]))

    Process {
        id: reader
        command: ["zenith", "binds"]
        running: true

        stdout: StdioCollector {
            id: collected
            waitForEnd: true
        }

        onExited: sheet.apply(collected.text)
    }

    // The catalogue, parsed.
    //
    // A catalogue that cannot be read leaves the sheet empty rather than
    // taking the shell down with it. A sheet with nothing in it is a sheet
    // that does not open, and a shell that dies on a parse error is a shell
    // with no desktop at all.
    function apply(text: string) {
        if (!text || text.trim() === "") {
            return;
        }
        try {
            var document = JSON.parse(text);
            sheet.categories = document.categories ?? [];
            sheet.binds = document.binds ?? [];
        } catch (error) {
            console.warn("Shell: the keybinding catalogue could not be read:",
                error);
        }
    }

    // The categories, with the bindings that belong to them.
    //
    // The grouping is done here and not in the catalogue, because a list of
    // bindings has no order a person can read and a list of categories does.
    // The order the categories come in is the one the catalogue gives them,
    // because that is the order somebody chose.
    function group(allCategories: var, allBinds: var): var {
        var listed = [];
        for (var i = 0; i < allCategories.length; i++) {
            var category = allCategories[i];
            var rows = [];
            for (var j = 0; j < allBinds.length; j++) {
                if (allBinds[j].category === category.id) {
                    rows.push(allBinds[j]);
                }
            }
            listed.push({ id: category.id, label: category.label, rows: rows });
        }
        return listed;
    }

    // Where the two columns are split.
    //
    // One column of everything is a column taller than the screen, and a
    // sheet that has to be scrolled is a sheet whose second half nobody
    // reads. The split is by rows rather than by groups, so the two columns
    // come out about the same height whatever the catalogue holds.
    function splitPoint(all: var): int {
        var total = 0;
        for (var i = 0; i < all.length; i++) {
            total += all[i].rows.length;
        }

        var half = total / 2;
        var count = 0;
        var best = 1;
        var closest = -1;
        for (var j = 0; j < all.length - 1; j++) {
            count += all[j].rows.length;
            var difference = Math.abs(count - half);
            if (closest < 0 || difference < closest) {
                closest = difference;
                best = j + 1;
            }
        }
        return best;
    }

    // The lines of one column: a heading for each group, and a row for each
    // binding.
    //
    // The lines are built here rather than by nested layouts, because a
    // layout's size is not reliably the size of its content when the content
    // is made of a repeater's items, and the height of a column is what
    // decides how tall the sheet is.
    function lines(groups: var): var {
        var listed = [];
        for (var i = 0; i < groups.length; i++) {
            var group = groups[i];
            listed.push({
                kind: "heading",
                label: group.label,
                height: headingHeight
            });
            for (var j = 0; j < group.rows.length; j++) {
                // The last row of a group carries the space that separates
                // it from the group below it.
                var last = j === group.rows.length - 1;
                listed.push({
                    kind: "row",
                    label: group.rows[j].label,
                    chord: group.rows[j].chord,
                    height: rowHeight + (last ? groupSpacing : 0)
                });
            }
        }
        return listed;
    }

    // The card.
    //
    // Drawn here rather than left to the popover, because the popover is a
    // window and a window is white unless it is told otherwise, and a sheet
    // of near-white text on white is a sheet nobody can read.
    Rectangle {
        anchors.fill: parent
        radius: Theme.radiusLarge
        color: Theme.panelFill
        border.width: Theme.hairlineWidth
        border.color: Theme.hairline
    }

    // The height of one column: its lines and the gaps between them.
    //
    // It is worked out here rather than read off the column, because a
    // column's own height is not known until the window it is in has been
    // polished, and a window that is the wrong height for a frame is a
    // window the user sees resize under their finger.
    function columnHeight(lines: var): int {
        var height = 0;
        for (var i = 0; i < lines.length; i++) {
            height += lines[i].height;
            if (i > 0) {
                height += rowSpacing;
            }
        }
        return height;
    }

    // One line of a column: a heading, or a binding and its chord.
    //
    // It is a component rather than a delegate written twice, because the
    // two columns are the same list drawn twice and a line drawn two ways is
    // a line that is drawn one way.
    Component {
        id: lineDelegate

        Item {
            id: line
            required property var modelData
            width: sheet.columnWidth
            height: line.modelData.height

            // The category. Dimmed and spaced, so that it reads as a
            // heading rather than as another binding.
            Text {
                visible: line.modelData.kind === "heading"
                anchors.left: parent.left
                anchors.top: parent.top
                text: line.modelData.label
                color: Theme.textDim
                font.pixelSize: 11
                font.letterSpacing: 0.6
            }

            // The binding.
            Text {
                visible: line.modelData.kind === "row"
                anchors.left: parent.left
                anchors.right: caps.left
                anchors.rightMargin: Theme.space3
                anchors.verticalCenter: caps.verticalCenter
                text: line.modelData.label
                color: Theme.text
                font.pixelSize: 12
                elide: Text.ElideRight
            }

            KeyCaps {
                id: caps
                visible: line.modelData.kind === "row"
                anchors.right: parent.right
                anchors.top: parent.top
                chord: line.modelData.kind === "row" ? line.modelData.chord : ""
            }
        }
    }

    Column {
        id: leftColumn
        x: sheet.margin
        y: sheet.margin
        width: sheet.columnWidth
        spacing: sheet.rowSpacing

        Repeater {
            model: sheet.columns[0]
            delegate: lineDelegate
        }
    }

    Column {
        id: rightColumn
        x: sheet.margin + sheet.columnWidth + sheet.columnSpacing
        y: sheet.margin
        width: sheet.columnWidth
        spacing: sheet.rowSpacing

        Repeater {
            model: sheet.columns[1]
            delegate: lineDelegate
        }
    }
}
