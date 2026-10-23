import QtQuick
import Quickshell
import qs.Ui.Singletons
import "vm"

// The VM manager's window: an entry file at the shell root with its code in vm/,
// for the same reason hub.qml is. Quickshell's configuration root is the directory
// of the file it is started on, and qs.Ui resolves only from there. Started with
// `quickshell -p` by session/zenith-vm, which also holds the single-instance lock.
ShellRoot {
    FloatingWindow {
        id: win
        // The Hyprland float rule (session/hyprland/window-rules.conf) and the
        // dock's title-to-launcher map both key on this exact string.
        title: "zenith-vm"
        minimumSize: Qt.size(1180, 760)
        // Opaque paper from the first frame, so the compositor never flashes its
        // uncleared buffer before the QML paints.
        color: Tokens.paper
        onClosed: Qt.quit()

        App { anchors.fill: parent }
    }
}
