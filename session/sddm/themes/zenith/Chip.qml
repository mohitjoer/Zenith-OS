import QtQuick

// One tappable label: a user, a session or a power action. 48 units high, the
// finger-sized floor, scaled with the screen like the rest of the login screen.
Rectangle {
    id: chip

    property string text
    property url face
    property bool active: false
    signal clicked()

    // The greeter runs at the compositor's scale, which is 1 even on a 4K panel.
    readonly property real u: Math.max(1, Window.height / 900)

    implicitWidth: row.implicitWidth + 32 * u
    implicitHeight: 48 * u
    radius: 12 * u
    color: active ? "#ffffff" : (area.containsMouse ? "#33ffffff" : "#1fffffff")
    border.width: activeFocus ? 2 : 1
    border.color: activeFocus ? "#ffffff" : "#66ffffff"
    activeFocusOnTab: true

    Row {
        id: row
        anchors.centerIn: parent
        spacing: 8 * chip.u

        Image {
            width: 32 * chip.u
            height: width
            visible: status === Image.Ready
            source: chip.face
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: chip.text
            color: chip.active ? "#000000" : "#ffffff"
            font.pixelSize: 16 * chip.u
        }
    }

    MouseArea {
        id: area
        anchors.fill: parent
        hoverEnabled: true
        onClicked: chip.clicked()
    }

    Keys.onReturnPressed: clicked()
    Keys.onSpacePressed: clicked()
}
