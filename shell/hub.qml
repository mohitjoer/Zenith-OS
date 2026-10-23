import QtQuick
import Quickshell
import Quickshell.Io
import "hub"
import qs.Ui.Singletons

// Zenith Settings entry point, from Ryoku's hub/quickshell/shell.qml. A normal
// floating window (not a layer-shell surface); the Hyprland window rule floats
// and centres it. The file is at the shell root because a Quickshell
// configuration's root is the directory of its entry file, and `qs.*` only
// resolves from there. `session/zenith-hub` starts it; `ZENITH_HUB_PAGE` names
// the page to open on.
ShellRoot {
    FloatingWindow {
        id: win
        // The literal, not I18n.tr: the compositor's window rule matches this
        // exact title, and a translated one would not match.
        title: "Zenith Settings"
        // Zenith Settings is a page, not a dialog: it opens at 99% of the screen
        // width, filling the height the shell bar leaves, so the settings get the
        // room the layout is designed for instead of a fixed 1200px strip. The
        // size is requested here and honoured by the Hyprland rule, which floats
        // it at the same 99% and centres it; maximumSize is the same request, so
        // the window can never open larger than the screen and a manual resize
        // has a sane ceiling.
        // Guard on a positive width/height, not just a non-null screen: during a
        // monitor switch Quickshell hides/reshows the window and the screen object
        // briefly dangles -- still non-null, but reporting size 0. Unguarded, fitW/
        // fitH go negative and Quickshell forwards them to the compositor as
        // set_min_size/set_max_size, sizing the toplevel to ~0: invisible, no input
        // region (click-dead), while qs keeps running and pins the single-instance
        // lock so reopening no-ops.
        readonly property int fitW: (win.screen && win.screen.width > 0) ? Math.round(win.screen.width * 0.99) : 1200
        readonly property int fitH: (win.screen && win.screen.height > 0) ? Math.round((win.screen.height - 56) * 0.99) : 880
        minimumSize: Qt.size(Math.min(1120, win.fitW), Math.min(820, win.fitH))
        maximumSize: Qt.size(win.fitW, win.fitH)
        color: Tokens.paper

        // Honour this monitor's Interface scale (Displays page): scale the whole
        // settings UI via Tokens. The window fills its screen, so a smaller scale
        // packs the chrome tighter rather than leaving a margin.
        Binding {
            target: Tokens
            property: "uiScale"
            value: Tokens.uiScaleFor(win.screen && win.screen.name ? win.screen.name : "")
        }

        // The launcher guards against a second instance with a lock held for the
        // life of this process. The in-app dismissals (Escape, close button)
        // already route requestQuit(); closing the window through the compositor
        // (Super+Q) only hides it while qs keeps running, which would pin the
        // lock and make Super+, silently no-op until the orphan is killed. Quit
        // on every close so the lock always releases -- through requestQuit, so
        // unsaved live Bar Studio edits are walked back off the desktop first.
        onClosed: hubItem.requestQuit()

        Hub {
            id: hubItem
            anchors.fill: parent
            // Right-to-left languages (Arabic, Hebrew, Persian) mirror the whole
            // settings UI from here: Qt flips anchors, rows, layouts and text
            // alignment for every descendant, so this is the one place that has
            // to know, instead of every component. Live, like the language
            // itself, because I18n.rtl is a binding.
            LayoutMirroring.enabled: I18n.rtl
            LayoutMirroring.childrenInherit: true
        }
    }

    // `zenith-hub PAGE` hands the page over in the environment: Quickshell takes
    // no arguments of its own for a configuration.
    Component.onCompleted: {
        const page = Quickshell.env("ZENITH_HUB_PAGE");
        if (page) hubItem.navigate(page);
    }

    // drive navigation from the CLI (`quickshell ipc -p .../hub.qml call nav open
    // <key>`): scripts jump straight to a section without a relaunch.
    IpcHandler {
        target: "nav"
        function open(section: string): void { hubItem.navigate(section); }
        function section(): string { return hubItem.section; }
    }

    // the daemon's `hub close` verb: same exit as the in-app dismissals (the
    // unsaved-restore then quit), so the lock releases and the next open starts
    // clean.
    IpcHandler {
        target: "hub"
        function close(): void { hubItem.requestQuit(); }
    }
}
