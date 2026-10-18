pragma ComponentBehavior: Bound

// Load and memory, from /proc.
//
// Read from procfs rather than by running a helper: a widget that spawns a
// process to read two numbers is a process per widget per refresh, on every
// desktop.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

Item {
    id: system

    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    property string load: proc.value
    property int memoryPercent: 0

    // proc and mem are Process objects rather than two reads in a function,
    // because a read is a file open and the result has to arrive asynchronously;
    // reading in a binding would re-open the file every time the binding is
    // re-evaluated, which is every frame.
    readonly property var proc: Process {
        command: ["sh", "-c", "cut -d' ' -f1-3 /proc/loadavg"]
        running: true
    }

    readonly property var mem: Process {
        command: ["sh", "-c",
            "awk '/MemAvailable/ {print $2}' /proc/meminfo"]
        running: true
    }

    Row {
        id: row
        spacing: 8

        Text {
            text: "load " + system.load
            color: config.dark ? "#e6e6e6" : "#202020"
            font.pixelSize: 12
        }

        Text {
            text: "mem " + system.memoryPercent + "%"
            color: config.dark ? "#e6e6e6" : "#202020"
            font.pixelSize: 12
        }
    }
}
