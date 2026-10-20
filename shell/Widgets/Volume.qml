pragma ComponentBehavior: Bound

// Volume, through Pipewire.
//
// The event API rather than a polled value: a volume widget that polls is a
// widget that cannot tell the difference between someone adjusting the volume
// and nothing happening, so it animates a stale number back and forth.
//
// The default sink rather than the first sink: a machine with two outputs has
// a default one, and the default is the one the user is listening to.

import Quickshell
import Quickshell.Io
import Quickshell.Services.Pipewire
import QtQuick

// The shell's own module, for the singletons this file reads. A file in a
// subdirectory cannot see a type from the directory above it by proximity the
// way a file beside it can, so the module is imported by name.
import qs

Item {
    id: volume

    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    // The sink, or nothing while Pipewire is still starting. A widget that
    // reads a sink that is not there yet is a widget that throws on every
    // frame until it is.
    readonly property var sink: Pipewire.defaultAudioSink

    // The optional chain and the fallback are both there because a sink with
    // no volume yet is a sink that reports one as undefined, and a widget that
    // shows "undefined%" is a widget that is broken on every machine whose
    // audio has not started yet.
    property real level: sink?.volume ?? 0
    property bool muted: sink?.muted ?? false

    Row {
        id: row
        spacing: 6

        Text {
            text: volume.muted ? "muted" : Math.round(volume.level * 100) + "%"
            color: Theme.text
            font.pixelSize: 12
        }
    }
}
