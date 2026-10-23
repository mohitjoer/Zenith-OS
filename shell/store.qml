import QtQuick
import Quickshell
import Quickshell.Io
import qs.Ui.Singletons
import "store"

// The store window. A normal floating window, started by session/zenith-store
// with `quickshell -p` on this file: the file sits at the shell root because a
// Quickshell configuration's root is the directory of its entry file, and the
// kit under qs.Ui only resolves from there.
ShellRoot {
    FloatingWindow {
        id: win
        // Literal, not translated: the compositor's float rule matches this title.
        title: "Zenith Store"
        // A screen object that dangles during a monitor switch reports size 0;
        // sizing from it would hand the compositor a zero-size window that is
        // invisible and click-dead while its lock stays held.
        readonly property int fitW: (win.screen && win.screen.width > 0) ? Math.min(1180, win.screen.width - Tokens.s5) : 1180
        readonly property int fitH: (win.screen && win.screen.height > 0) ? Math.min(760, win.screen.height - Tokens.s7) : 760
        readonly property bool cramped: win.fitW < 1180 || win.fitH < 760
        minimumSize: Qt.size(Math.min(980, win.fitW), Math.min(640, win.fitH))
        maximumSize: win.cramped ? Qt.size(win.fitW, win.fitH) : Qt.size(16777215, 16777215)
        color: Tokens.paper
        onClosed: app.requestQuit()

        App {
            id: app
            anchors.fill: parent
        }
    }

    IpcHandler {
        target: "nav"
        function open(section: string): void { app.openRoute(section); }
        function section(): string { return app.categoryID !== "" ? app.categoryID : app.view; }
    }
}
