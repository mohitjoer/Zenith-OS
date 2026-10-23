import QtQuick
import QtTest

// Runs the real theme against stand-ins for the objects SDDM injects, because
// the greeter itself cannot run without a display manager. Run by
// session/greeter_test.sh when qmltestrunner is installed.
Item {
    id: top
    width: 1600
    height: 900

    QtObject {
        id: sddm
        property bool canSuspend: true
        property bool canReboot: true
        property bool canPowerOff: true
        property var calls: []
        signal loginFailed()
        function login(user, pw, row) { calls.push(["login", user, pw, row]) }
        function powerOff() { calls.push(["powerOff"]) }
        function suspend() { calls.push(["suspend"]) }
        function reboot() { calls.push(["reboot"]) }
    }
    ListModel {
        id: userModel
        property string lastUser: "mohit"
        ListElement { name: "guest"; realName: ""; icon: "" }
        ListElement { name: "mohit"; realName: "Mohit"; icon: "" }
    }
    // Alphabetical, as SDDM lists them: Zenith arrives last.
    ListModel {
        id: sessionModel
        ListElement { file: "/usr/share/wayland-sessions/gnome.desktop"; name: "GNOME" }
        ListElement { file: "/usr/share/wayland-sessions/hyprland.desktop"; name: "Hyprland" }
        ListElement { file: "/usr/share/wayland-sessions/zenith.desktop"; name: "Zenith" }
    }

    Loader {
        id: loader
        anchors.fill: parent
        focus: true
        source: Qt.resolvedUrl("themes/zenith/Main.qml")
    }

    TestCase {
        name: "theme"
        when: windowShown

        // Each test gets a fresh theme: QML runs test functions alphabetically,
        // and a click in one must not leak into the next.
        function init() {
            sddm.calls = []
            loader.active = false
            loader.active = true
            tryVerify(function() { return loader.item.sessions.length === 3 })
            wait(100)
        }

        function chip(item, text) {
            if (item.text === text && item.clicked !== undefined)
                return item
            for (var i = 0; i < item.children.length; ++i) {
                var found = chip(item.children[i], text)
                if (found)
                    return found
            }
            return null
        }

        function type(text) {
            for (var i = 0; i < text.length; ++i)
                keyClick(text[i])
        }

        function test_zenith_is_listed_first_and_preselected() {
            compare(loader.item.sessions[0].name, "Zenith")
            compare(loader.item.sessions[0].row, 2)
            compare(loader.item.sessionPos, 0)
            compare(loader.item.userName, "mohit")
        }

        function test_login_sends_user_password_and_the_models_row() {
            type("secret")
            keyClick(Qt.Key_Return)
            compare(sddm.calls.length, 1)
            compare(sddm.calls[0], ["login", "mohit", "secret", 2])
        }

        // Clearing the field after a failure used to wipe the message too.
        function test_failure_keeps_its_message_and_empties_the_field() {
            type("wrong")
            sddm.loginFailed()
            compare(loader.item.message, "Incorrect password")
            type("x")
            compare(loader.item.message, "")
            keyClick(Qt.Key_Return)
            compare(sddm.calls[0][2], "x")
        }

        function test_choosing_a_session_and_a_user_changes_the_login() {
            mouseClick(chip(loader.item, "Hyprland"))
            mouseClick(chip(loader.item, "guest"))
            keyClick(Qt.Key_Return)
            compare(sddm.calls[0][1], "guest")
            compare(sddm.calls[0][3], 1)
        }

        function test_power_follows_what_the_daemon_allows() {
            mouseClick(chip(loader.item, "Shut down"))
            compare(sddm.calls[0], ["powerOff"])
            sddm.canReboot = false
            verify(!chip(loader.item, "Restart").visible)
            sddm.canReboot = true
        }

        function test_test_mode_sends_nothing() {
            loader.item.testMode = true
            keyClick(Qt.Key_Return)
            compare(sddm.calls.length, 0)
            loader.item.testMode = false
        }
    }
}
