pragma ComponentBehavior: Bound

// The one way this window changes a setting.
//
// Every control calls this rather than writing through GSettings. That is the
// architecture record's rule — the shell daemon and zenith-hub are the only
// writers, and hand-editing dconf causes drift — but the deeper reason is that
// `zenith config set` validates against the key's type, range and enum before it
// writes. A control that wrote dconf itself could put 999999 in a bar height that
// declares a maximum of 96, and nothing would report it until the shell laid out
// its bar using it.
//
// One place to write from also means one place to add reporting later.

import Quickshell
import Quickshell.Io

Singleton {
    id: zenith

    property string lastError: ""
    property string lastWritten: ""

    // set writes a value and records what happened.
    //
    // The error is kept rather than thrown: a QML exception in a settings window
    // takes the window down, and the user who just changed a setting is left with
    // no way to find out why it did not change.
    function set(key, value) {
        lastError = ""
        lastWritten = ""

        const write = writeProcess
        write.command = ["zenith", "config", "set", key.schema, key.name, value]
        write.running = true
    }

    readonly property Process writeProcess: Process {
        id: process

        stderr: StdioCollector {
            onStreamFinished: zenith.lastError = text
        }
        stdout: StdioCollector {
            onStreamFinished: if (zenith.lastError === "") zenith.lastWritten = text
        }
    }
}
