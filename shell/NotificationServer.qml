pragma ComponentBehavior: Bound

// The notification server.
//
// org.freedesktop.Notifications is a session-scoped bus name, and nothing else
// on this machine holds it: there is no notification daemon anywhere in the
// image. If this object is not alive, every application that sends a
// notification gets an error back and the user is told nothing.
//
// It lives inside the shell rather than as its own daemon for the same reason
// the lock screen and quick settings do: a daemon with no UI is not a reason to
// ship a second process, and the UI has to be in the same process as the bus
// name anyway or the notifications arrive at a surface that may not be running.

import Quickshell
import Quickshell.Services.Notifications

NotificationServer {
    id: server

    // Reload is the most common thing anyone does to a shell, and a reload that
    // drops whatever the user has not looked at yet is a reload that loses
    // things. The Notification objects from the previous generation are
    // re-emitted with lastGeneration set, so the handler below can tell them
    // apart from new ones.
    keepOnReload: true

    // Actions are the point of a notification server to a user — "Reply",
    // "Archive", "Mark read" — and an application that offers them is entitled
    // to know they will be offered.
    actionsSupported: true
    actionIconsSupported: false
    bodySupported: true

    // Markup is off. The spec is explicit that a server without the body-markup
    // capability must have markup shown to the user as plain text, and the
    // alternative is that an application sends <b> and the user's name gets
    // rendered in bold, or worse, that it sends markup the renderer treats as
    // rich text and something in it is interpreted.
    //
    // Turning it on later means handling a body that is genuinely markup, which
    // is a decision to make when there is a decision to make.
    bodyMarkupSupported: false
    bodyHyperlinksSupported: false
    bodyImagesSupported: false

    // Images are the app icon and the image hint, both of which Quickshell
    // resolves. An image in the body is markup, which is off above.
    imageSupported: true

    // Persistence claims the server keeps notifications after they go offscreen
    // and until the user acts on them, so that clients may stop showing a
    // status icon of their own. There is no notification centre yet, so this
    // stays false: a client that believes it does not need its own reminder is a
    // client with no reminder and no history.
    persistenceSupported: false

    // How many are kept. A server that grows without bound is a server that
    // eventually runs the shell out of memory during the one event that
    // generates a lot of notifications, which is exactly when the machine is
    // already struggling.
    property int historyLimit: 32

    property var history: []

    // A notification arrives here rather than straight into the model.
    //
    // The handler's whole job is one line, and that line is the difference
    // between a working notification server and one that shows nothing: a
    // Notification is discarded unless something sets `tracked`. Do nothing
    // here and every notification is accepted off the bus and dropped on the
    // floor, with no error anywhere, because discarding is the default.
    Connections {
        target: server
        function onNotification(notification) {
            server.remember(notification)
        }
    }

    function remember(notification) {
        notification.tracked = true

        // Replaces: the client sent a replacesId, and Quickshell has already
        // updated the existing object in place. Re-appending it would show the
        // same notification twice, which is the visible symptom of ignoring
        // replaces_id — so only genuinely new notifications are appended.
        if (!notification.lastGeneration) {
            var next = history.slice()
            next.push(notification)
            if (next.length > historyLimit) {
                // The oldest go first. Dropping the newest would be dropping
                // the one the user is most likely to be looking at.
                next = next.slice(next.length - historyLimit)
            }
            history = next
        }
    }

    function dismiss(id) {
        for (var i = 0; i < history.length; i++) {
            if (history[i].id === id) {
                history[i].dismiss()
                var next = history.slice()
                next.splice(i, 1)
                history = next
                return
            }
        }
    }

    function clear() {
        for (var i = 0; i < history.length; i++) {
            history[i].dismiss()
        }
        history = []
    }

    // The one being shown, or undefined. A separate property rather than
    // history[0] at every use site, because the "which one is showing" question
    // is asked from three places and answering it three ways is how the popup
    // and the history end up disagreeing.
    property var current: history.length > 0 ? history[history.length - 1] : undefined
}
