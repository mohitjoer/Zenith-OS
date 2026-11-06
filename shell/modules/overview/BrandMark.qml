pragma ComponentBehavior: Bound
import QtQuick
import Qt5Compat.GraphicalEffects
import "Singletons"

// The desktop brand seal, A by default. Users swap it globally (their own
// glyph/text, or an image) from Asura Settings -> Shell -> Global; stored in
// ~/.config/asura/brand.json. Asura's own apps (the Hub, ryo* apps) never use
// this and keep the Asura brand. Drop-in for the old
// `Text { text: "A"; font.family: Theme.fontJp; color: Theme.brand }` seal:
// pass the old pixelSize as `size` and the old colour as `color`.
Item {
    id: mark

    property real size: 13
    property color color: Theme.brand
    property int weight: Font.Medium
    // recolour a single-colour image mark to `color` (matches the tinted Asura mark
    // idiom); off shows a full-colour logo as-is. no effect in text mode.
    property bool tint: Theme.markTint

    readonly property bool hasCustomImage: Theme.markSource.length > 0
    readonly property bool hasCustomText: Theme.markSource.length === 0 && Theme.mark.length > 0 && Theme.mark !== "A"
    readonly property bool isDefaultLogo: !hasCustomImage && !hasCustomText

    readonly property string imageSource: hasCustomImage
        ? (/^(file|qrc|image|https?):/.test(Theme.markSource) ? Theme.markSource : "file://" + Theme.markSource)
        : Qt.resolvedUrl("../../../assets/asura-logo-on-dark.svg")

    implicitWidth:  !mark.hasCustomText ? mark.size : glyph.implicitWidth
    implicitHeight: !mark.hasCustomText ? mark.size : glyph.implicitHeight

    Text {
        id: glyph
        visible: mark.hasCustomText
        anchors.centerIn: parent
        text: Theme.mark
        color: mark.color
        font.family: Theme.fontJp
        font.weight: mark.weight
        font.pixelSize: mark.size
    }

    // custom image mark or default Asura logo.
    Image {
        id: img
        anchors.centerIn: parent
        width: mark.size
        height: mark.size
        visible: !mark.hasCustomText && (!mark.tint || mark.isDefaultLogo)
        source: mark.imageSource
        sourceSize.width: Math.round(mark.size * 2)
        sourceSize.height: Math.round(mark.size * 2)
        fillMode: Image.PreserveAspectFit
        smooth: true
    }

    ColorOverlay {
        anchors.fill: img
        visible: !mark.hasCustomText && mark.tint && !mark.isDefaultLogo
        source: img
        color: mark.color
    }
}
