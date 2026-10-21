pragma Singleton
pragma ComponentBehavior: Bound

// The one way this window changes a setting.
//
// Every control calls this rather than writing through GSettings. That is the
// architecture record's rule — the shell daemon and zenith-hub are the only
// writers, and hand-editing dconf causes drift — but the deeper reason is that
// `zenith config set` validates against the key's type, range and enum before
// it writes. A control that wrote dconf itself could put 999999 in a bar height
// that declares a maximum of 96, and nothing would report it until the shell
// laid out its bar using it.
//
// One place to write from also means one place to report from. A write that is
// refused is announced here, and the editor that asked for it shows the message
// and goes back to the value the tool last accepted. A write that failed quietly
// is a setting that looks changed and is not.
//
// The error is kept rather than thrown: a QML exception in a settings window
// takes the window down, and the user who just changed a setting is left with
// no way to find out why it did not change.

import Quickshell
import Quickshell.Io

Singleton {
    id: zenith

    // A write has been tried and has reported back.
    //
    // The value that was written goes with it, so an editor can tell its own
    // write from another editor's and does not revert a change it did not
    // make.
    signal settled(string qualified, string value, bool failed, string message)

    // The write in flight.
    //
    // One at a time. A process that is given a second command while it is
    // running belongs to neither write, and the value on screen would be one
    // of two.
    property bool writing: false
    property var current: null
    property bool reported: false

    // The writes waiting for the one in flight.
    property var queue: []

    // The two halves of the outcome, and the moment both are in.
    //
    // The exit code says whether the setting changed. The error stream says
    // why it did not. Neither is enough on its own: a failure with no message
    // is a refusal the user cannot act on, and a message with no code says
    // nothing about whether the write happened.
    //
    // They are collected separately because they arrive separately, and a
    // write that is reported before its message has been read is a write that
    // is reported as a failure with nothing to say.
    property int exitCode: -1
    property bool errorRead: false

    // set writes a value.
    //
    // The value is the tool's spelling of it, not GSettings': an enum's nick
    // is bare, because the tool compares it against the schema's nicks.
    function set(key, value) {
        queue.push({
            schema: key.schema,
            name: key.name,
            qualified: key.schema + "." + key.name,
            value: value
        })
        pump()
    }

    // pump starts the next write if there is one and nothing is in flight.
    function pump() {
        if (writing || queue.length === 0) return

        const job = queue.shift()

        // The process is stopped before it is given a command, so the change
        // of command cannot restart a process that is still finishing.
        process.running = false

        current = job
        writing = true
        reported = false
        exitCode = -1
        errorRead = false

        process.command = ["zenith", "config", "set", job.schema, job.name, job.value]
        process.running = true
    }

    // The write has finished and the error stream has been read.
    //
    // Both, and in whatever order they arrive: the exit comes from the
    // process and the message comes from the pipe, and neither of them waits
    // for the other.
    function settle() {
        if (reported || !current) return
        if (zenith.exitCode < 0 || !zenith.errorRead) return

        reported = true

        const job = current
        current = null
        writing = false

        const failed = zenith.exitCode !== 0
        const message = failed ? process.stderr.text.trim() : ""

        settled(job.qualified, job.value, failed, message)
        pump()
    }

    Process {
        id: process

        stdout: StdioCollector {
            waitForEnd: true
        }

        stderr: StdioCollector {
            // The message is read when the process has ended rather than when
            // the stream closed, so the text is whole.
            waitForEnd: true
            onStreamFinished: {
                zenith.errorRead = true
                zenith.settle()
            }
        }

        onExited: (code) => {
            zenith.exitCode = code
            zenith.settle()
        }

        // A process that cannot be started at all never exits, so the exit
        // never comes. The setting tool missing is a refusal like any other
        // and has to be reported as one.
        onRunningChanged: if (!running && zenith.exitCode < 0) {
            zenith.exitCode = 1
            zenith.errorRead = true
            zenith.settle()
        }
    }
}
