pragma ComponentBehavior: Bound

// The notification popup.
//
// PlainText on every body, because the server has bodyMarkupSupported off. The
// Quickshell documentation is explicit that a server without that capability
// must render markup as plain text, and that a Text object renders markup by
// default — so the two settings have to agree or a notification body arrives
// with <b> in it on screen.

import Quickshell
import Quickshell.Services.Notifications
import Quickshell.Widgets
import QtQuick
import QtQuick.Controls

Popover {
    id: surface

    // The server that holds the notifications. It is passed in rather than
    // created here, because there is one server for the session and a second
    // one would be a second bus name for the same interface — the second
    // registration loses, and the notifications go to the wrong surface.
    required property NotificationServer server

    // Anchored to the same edge the dock is on, so the popup never covers the
    // dock and the two cannot disagree about which side of the screen is taken.
    readonly property int edge: Config.dockPosition

    implicitWidth: 380
    implicitHeight: card.implicitHeight + 24

    edgeTop: edge === Config.edgeTop
    edgeBottom: edge === Config.edgeBottom
    edgeLeft: edge === Config.edgeLeft
    edgeRight: edge === Config.edgeRight

    margin: 12

    // Bound to opened, and opened follows visibility, so the popup is shown and
    // hidden through one property rather than two that can disagree.
    opened: server.current !== undefined
        && server.current !== null

    // The popup closes itself, but only for a notification that is allowed to
    // close. A resident notification is one the client asked to keep until
    // something happens to it — a download asking to stay until it is done, a
    // call asking not to be dismissed — and a timer that ignores that is a
    // server that throws away the notifications that mattered most.
    readonly property bool autoDismiss: server.current !== undefined
        && server.current !== null
        && !server.current.resident

    Timer {
        id: dismissTimer
        // The spec's expire_timeout is milliseconds; Quickshell exposes it in
        // seconds. 0 means never expire and -1 means "the server decides", and
        // both of those have to be turned into "do not start a timer at all"
        // rather than into a timer of zero seconds.
        interval: (surface.server.current
            && surface.server.current.expireTimeout > 0)
            ? surface.server.current.expireTimeout * 1000
            : 0
        running: surface.autoDismiss && interval > 0
        repeat: false
        onTriggered: if (surface.server.current) {
            surface.server.current.expire()
        }
    }

    Rectangle {
        id: card
        anchors.fill: parent
        radius: 12
        color: Theme.panelFill
        border.width: 1
        border.color: Theme.lineDivider

        Column {
            id: content
            anchors {
                left: parent.left; right: parent.right; top: parent.top
            }
            anchors.margins: 12
            spacing: 8

            Row {
                spacing: 10
                height: 40

                // Resolved by Quickshell from the client's desktop entry when
                // the client sent only an app name, which is most of the time.
                IconImage {
                    implicitSize: 32
                    source: surface.server.current
                        ? surface.server.current.appIcon
                        : ""
                }

                Column {
                    spacing: 2
                    width: content.width - 42

                    Text {
                        width: parent.width
                        text: surface.server.current
                            ? surface.server.current.summary
                            : ""
                        color: Theme.inkPrimary
                        font.pixelSize: 13
                        font.bold: true
                        elide: Text.ElideRight
                    }

                    Text {
                        width: parent.width
                        text: surface.server.current
                            ? surface.server.current.body
                            : ""
                        color: Theme.inkSecondary
                        font.pixelSize: 12
                        wrapMode: Text.WordWrap
                        // Matches the server's bodyMarkupSupported: false. The
                        // capability is what tells the client we will not render
                        // markup, so the client strips it — and if one sends it
                        // anyway, this is what stops it being interpreted.
                        textFormat: Text.PlainText
                    }
                }
            }

            // Actions are offered because the server advertises the capability.
            // Nothing else: an action that is not rendered is an action the user
            // cannot take, and the client has already committed to waiting for
            // it.
            Flow {
                visible: surface.server.current !== undefined
                    && surface.server.current !== null
                    && surface.server.current.actions.length > 0
                spacing: 8

                Repeater {
                    model: surface.server.current
                        ? surface.server.current.actions
                        : []

                    delegate: Button {
                        required property var modelData

                        text: modelData.label
                        // The identifier is what goes back over the bus. The
                        // label is only what the user reads, and sending it
                        // instead would be a reply the client cannot match to
                        // anything it offered.
                        onClicked: surface.server.current
                            .invokeAction(modelData.identifier)
                    }
                }
            }
        }
    }

    // Dismissed by clicking away as well as by the timer, because a popup with
    // only a timeout is a popup the user cannot get rid of while they are
    // reading it.
    TapHandler {
        onTapped: if (surface.server.current) surface.server.current.dismiss()
    }
}
