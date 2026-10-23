pragma ComponentBehavior: Bound

import QtQuick

// The login screen: a user, a password, a session, and power. SDDM injects
// `sddm`, `userModel` and `sessionModel`; everything else is plain QtQuick, so
// the screen needs nothing beyond the packages sddm itself depends on.
Rectangle {
    id: root

    color: "#000000"

    readonly property real u: Math.max(1, height / 900)
    // SDDM has no daemon to talk to under --test-mode, so login is a no-op there.
    property bool testMode: Qt.application.arguments.indexOf("--test-mode") >= 0

    property string userName: ""
    property var sessions: []
    property int sessionPos: 0
    property string message: ""
    property bool failed: false

    // SDDM lists sessions alphabetically. Zenith goes first and is preselected,
    // so the greeter never makes the user hunt for the desktop they installed.
    function rebuild() {
        var zenith = []
        var rest = []
        for (var i = 0; i < probe.count; ++i) {
            var s = probe.itemAt(i)
            if (!s)
                continue
            if (/(^|\/)zenith\.desktop$/.test(s.file))
                zenith.push({ row: i, name: s.name })
            else
                rest.push({ row: i, name: s.name })
        }
        sessions = zenith.concat(rest)
        sessionPos = 0
    }

    function login() {
        failed = true
        if (userName === "")
            message = qsTr("Choose a user")
        else if (sessions.length === 0)
            message = qsTr("No desktop session is installed")
        else if (testMode)
            message = qsTr("Test mode: nothing was sent")
        else {
            failed = false
            message = qsTr("Signing in…")
            sddm.login(userName, password.text, sessions[sessionPos].row)
        }
    }

    Keys.onEscapePressed: {
        if (testMode && Window.window)
            Window.window.close()
    }

    Connections {
        target: sddm
        function onLoginFailed() {
            // Clear the field first: editing it clears the message.
            password.text = ""
            root.failed = true
            root.message = qsTr("Incorrect password")
            password.forceActiveFocus()
        }
    }

    Image {
        anchors.fill: parent
        source: "file:///usr/share/zenith/backgrounds/galaxy.png"
        fillMode: Image.PreserveAspectCrop
        asynchronous: true
    }
    Rectangle {
        anchors.fill: parent
        color: "#000000"
        opacity: 0.6
    }

    Repeater {
        id: probe
        model: sessionModel
        delegate: Item {
            required property string name
            required property string file
        }
        onItemAdded: Qt.callLater(root.rebuild)
        onItemRemoved: Qt.callLater(root.rebuild)
    }

    Column {
        anchors.centerIn: parent
        width: Math.min(root.width - 64 * root.u, 480 * root.u)
        spacing: 16 * root.u

        Flow {
            width: parent.width
            spacing: 8 * root.u
            Repeater {
                model: userModel
                delegate: Chip {
                    required property int index
                    required property string name
                    required property string realName
                    required property string icon
                    text: realName || name
                    face: icon
                    active: name === root.userName
                    onClicked: {
                        root.userName = name
                        password.forceActiveFocus()
                    }
                    // The last user, else the first one listed.
                    Component.onCompleted: {
                        if (name === userModel.lastUser || (index === 0 && root.userName === ""))
                            root.userName = name
                    }
                }
            }
        }

        Row {
            width: parent.width
            spacing: 8 * root.u

            Rectangle {
                width: parent.width - parent.spacing - go.width
                height: 56 * root.u
                radius: 14 * root.u
                color: "#c7000000"
                border.width: password.activeFocus ? 2 : 1
                border.color: password.activeFocus ? "#ffffff" : "#66ffffff"

                TextInput {
                    id: password
                    anchors.fill: parent
                    anchors.margins: 16 * root.u
                    verticalAlignment: TextInput.AlignVCenter
                    echoMode: TextInput.Password
                    inputMethodHints: Qt.ImhSensitiveData
                    color: "#ffffff"
                    font.pixelSize: 18 * root.u
                    focus: true
                    clip: true
                    onAccepted: root.login()
                    // Typing again clears the last error.
                    onTextChanged: if (root.failed) root.message = ""
                }
                Text {
                    anchors.fill: password
                    verticalAlignment: Text.AlignVCenter
                    visible: password.text === ""
                    text: qsTr("Password")
                    color: "#9a9a9a"
                    font.pixelSize: 18 * root.u
                }
            }
            Chip {
                id: go
                height: 56 * root.u
                text: qsTr("Log in")
                onClicked: root.login()
            }
        }

        Text {
            width: parent.width
            height: 20 * root.u
            text: root.message
            color: root.failed ? "#ff6b61" : "#d0d0d0"
            font.pixelSize: 15 * root.u
        }

        Text {
            text: qsTr("Session")
            color: "#d0d0d0"
            font.pixelSize: 14 * root.u
        }
        Flow {
            width: parent.width
            spacing: 8 * root.u
            Repeater {
                model: root.sessions
                delegate: Chip {
                    required property int index
                    required property var modelData
                    text: modelData.name
                    active: index === root.sessionPos
                    onClicked: {
                        root.sessionPos = index
                        password.forceActiveFocus()
                    }
                }
            }
        }
    }

    Row {
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: 32 * root.u
        spacing: 8 * root.u

        Chip {
            visible: sddm.canSuspend
            text: qsTr("Sleep")
            onClicked: sddm.suspend()
        }
        Chip {
            visible: sddm.canReboot
            text: qsTr("Restart")
            onClicked: sddm.reboot()
        }
        Chip {
            visible: sddm.canPowerOff
            text: qsTr("Shut down")
            onClicked: sddm.powerOff()
        }
    }
}
