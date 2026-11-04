pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Effects
import qs.Services
import "Singletons"

// One stage layer: an alpha PNG drawn back-to-front inside the desktop surface
// (docs/stage.md). Edge softness, drop shadow and its angle are the global Look
// every layer inherits (Config). The still cut sits pixel-locked over the
// wallpaper's own subject and there is never a second, ghosted subject.
Item {
    id: root

    property int layerIndex: 1
    property string wallPath: ""
    property string url: ""
    property string fit: "Cover"
    readonly property int i: root.layerIndex - 1
    readonly property bool layerShown: root.url !== ""
        && StageBackend.isActiveFor(root.wallPath)
        && StageBackend.layerEnabled(root.wallPath, root.i)

    anchors.fill: parent
    visible: root.layerShown

    readonly property real _edge: Config.edge
    readonly property real _shStrength: Config.shadow
    readonly property real _shAngle: Config.shadowAngle * Math.PI / 180

    function _fillMode(im) {
        switch (root.fit) {
        case "Contain": return Image.PreserveAspectFit;
        case "Fill": return Image.Stretch;
        case "ScaleDown":
            return (im.sourceSize.width <= root.width && im.sourceSize.height <= root.height) ? Image.Pad : Image.PreserveAspectFit;
        default: return Image.PreserveAspectCrop;
        }
    }

    // Cutting progress rides the subject itself, not a panel (docs/stage.md):
    // the subject layer (index 1) dims and a ring with the percent floats over it.
    readonly property bool _busyHere: root.layerIndex === 1 && StageBackend.busy && root.url !== ""

    Image {
        id: img
        anchors.fill: parent
        source: root.url
        cache: false
        asynchronous: true
        fillMode: root._fillMode(img)
        sourceSize.width: root.width
        sourceSize.height: root.height
        opacity: (status === Image.Ready ? 1 : 0) * (root._busyHere ? 0.45 : 1)
        Behavior on opacity { NumberAnimation { duration: 320; easing.type: Easing.OutCubic } }

        // Edge feather and the angled cast shadow set the cut-out into the scene;
        // the FBO stays off when both are zero.
        layer.enabled: root._edge > 0.001 || root._shStrength > 0.001
        layer.effect: MultiEffect {
            blurEnabled: root._edge > 0.001
            blurMax: 16
            blur: root._edge
            shadowEnabled: root._shStrength > 0.001
            shadowColor: Qt.rgba(0, 0, 0, 0.72 * root._shStrength)
            shadowBlur: 0.55 + 0.45 * root._shStrength
            shadowHorizontalOffset: Math.round(Math.cos(root._shAngle) * 20 * root._shStrength)
            shadowVerticalOffset: Math.round(Math.sin(root._shAngle) * 20 * root._shStrength)
        }
    }

    // Progress ring centred on the subject; tap to cancel the cut.
    Item {
        anchors.centerIn: parent
        width: 96
        height: 96
        visible: root._busyHere
        Canvas {
            id: ring
            anchors.fill: parent
            property int pct: StageBackend.percent
            onPctChanged: ring.requestPaint()
            onVisibleChanged: ring.requestPaint()
            Component.onCompleted: ring.requestPaint()
            onPaint: {
                const ctx = getContext("2d");
                const w = width, cx = w / 2, cy = height / 2, r = w / 2 - 6;
                ctx.reset();
                ctx.lineWidth = 6;
                ctx.lineCap = "round";
                ctx.strokeStyle = Qt.rgba(1, 1, 1, 0.25);
                ctx.beginPath();
                ctx.arc(cx, cy, r, 0, 2 * Math.PI);
                ctx.stroke();
                ctx.strokeStyle = Theme.primary;
                ctx.beginPath();
                ctx.arc(cx, cy, r, -Math.PI / 2, -Math.PI / 2 + 2 * Math.PI * Math.max(0, Math.min(1, ring.pct / 100)));
                ctx.stroke();
            }
        }
        Text {
            anchors.centerIn: parent
            text: StageBackend.percent + "%"
            color: "white"
            font.family: Theme.fontPrimary
            font.pixelSize: Theme.fontSm
            font.weight: Font.DemiBold
        }
        MouseArea {
            anchors.fill: parent
            cursorShape: Qt.PointingHandCursor
            onClicked: StageBackend.cancel()
        }
    }
}
