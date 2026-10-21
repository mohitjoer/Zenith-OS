pragma ComponentBehavior: Bound

// Load and memory, from procfs.
//
// Read from procfs rather than by running a helper: a widget that spawns a
// process to read two numbers is a process per widget per refresh, on every
// desktop.
//
// The files are read again every few seconds because they are the only place
// the numbers are, and a load average that never changes is not a load
// average. procfs does not report changes, so the read is on a timer — and it
// is a timer on the read rather than a timer on the drawing: the widget only
// redraws when the number it shows has changed.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

// The shell's own module, for the singletons this file reads. A file in a
// subdirectory cannot see a type from the directory above it by proximity the
// way a file beside it can, so the module is imported by name.
import qs

Item {
    id: system

    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    // How often the numbers are read. Five seconds is often enough for the
    // load average to mean something and rare enough that nobody is measuring
    // it.
    readonly property int refreshSeconds: 5

    // The first three fields of /proc/loadavg: the one, five and fifteen
    // minute averages.
    readonly property string load: {
        var fields = loadFile.text().trim().split(/\s+/);
        return fields.length >= 3 ? fields.slice(0, 3).join(" ") : "";
    }

    // The memory in use, as a percentage of the memory there is.
    //
    // Read from MemAvailable rather than from MemFree: the free memory on a
    // machine with a page cache is a small number that means nothing, and
    // "80% used" is a statement about the cache rather than about pressure.
    readonly property int memoryPercent: {
        var available = 0;
        var total = 0;
        var lines = memoryFile.text().split("\n");
        for (var i = 0; i < lines.length; i++) {
            if (lines[i].indexOf("MemTotal:") === 0) {
                total = parseInt(lines[i].split(/\s+/)[1]);
            } else if (lines[i].indexOf("MemAvailable:") === 0) {
                available = parseInt(lines[i].split(/\s+/)[1]);
            }
        }
        if (!total) {
            return 0;
        }
        return Math.round((1 - available / total) * 100);
    }

    FileView {
        id: loadFile
        path: "/proc/loadavg"
        watchChanges: false
    }

    FileView {
        id: memoryFile
        path: "/proc/meminfo"
        watchChanges: false
    }

    Timer {
        interval: system.refreshSeconds * 1000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: {
            loadFile.reload();
            memoryFile.reload();
        }
    }

    Row {
        id: row
        spacing: 8

        Text {
            text: "load " + system.load
            color: Theme.inkPrimary
            font.pixelSize: 12
        }

        Text {
            text: "mem " + system.memoryPercent + "%"
            color: Theme.inkPrimary
            font.pixelSize: 12
        }
    }
}
