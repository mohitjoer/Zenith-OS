pragma ComponentBehavior: Bound
import QtQuick
import "../Singletons"

// clock widget. picks one face + (if enabled) stacks the date strip centred
// underneath. purely structural -- faces and date strips read time, palette
// and size knobs from the singletons themselves (same pattern as the
// visualiser's renderer). this only chooses which to show and lays them out.
// implicit size drives the WidgetSlot around it.
Item {
    id: clock

    // pushed in by WidgetSlot, forwarded to the face and date strip so both pick
    // their ink against the same patch of wallpaper.
    property real underL: Scheme.wallLstar
    // pinned per-widget colour, forwarded to the face and date strip so a solid
    // colour reaches the glyphs (gradient is applied by the slot over the whole).
    property string inkColorA: ""

    readonly property var faceItem: faceLoader.item
    readonly property var dateItem: dateLoader.item
    readonly property real fw: faceItem ? faceItem.implicitWidth : 0
    readonly property real fh: faceItem ? faceItem.implicitHeight : 0
    readonly property bool hasDate: Config.dateShow && dateItem !== null
    readonly property real dw: clock.hasDate ? dateItem.implicitWidth : 0
    readonly property real dh: clock.hasDate ? dateItem.implicitHeight : 0
    readonly property real gap: (clock.hasDate && clock.dh > 0) ? Math.round(14 * Config.clockScale) : 0

    implicitWidth: Math.max(1, Math.max(clock.fw, clock.dw))
    implicitHeight: Math.max(1, clock.fh + clock.gap + clock.dh)

    Loader {
        id: faceLoader
        x: (clock.implicitWidth - clock.fw) / 2
        y: 0
        sourceComponent: clock.faceFor(Config.clockDesign)
    }

    Loader {
        id: dateLoader
        x: (clock.implicitWidth - clock.dw) / 2
        y: clock.fh + clock.gap
        active: Config.dateShow
        visible: Config.dateShow
        sourceComponent: Config.dateShow ? clock.dateFor(Config.dateDesign) : null
    }

    function faceFor(d) {
        switch (d) {
        case "zen":     return zenComp;
        case "minimal": return minimalComp;
        default:        return digitalComp;
        }
    }
    function dateFor(d) {
        switch (d) {
        case "card":    return dateCardComp;
        case "hud":     return dateHudComp;
        default:        return dateHudComp;
        }
    }

    Component { id: zenComp;      ClockZen { underL: clock.underL; inkColorA: clock.inkColorA } }
    Component { id: digitalComp;  ClockDigital { underL: clock.underL; inkColorA: clock.inkColorA } }
    Component { id: minimalComp;  ClockMinimal { underL: clock.underL; inkColorA: clock.inkColorA } }

    Component { id: dateCardComp; DateCard { underL: clock.underL; inkColorA: clock.inkColorA } }
    Component { id: dateHudComp;  DateHud { underL: clock.underL; inkColorA: clock.inkColorA } }
}
