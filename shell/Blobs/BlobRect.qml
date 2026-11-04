import QtQuick

// One rectangle in a blob field.
//
// The geometry is native to this item, so the caller lays it out just as it
// would any item. What is special is where the body reads its paint from: the
// blob field it has been placed in, through `group`. The field's color and
// border are shared by every member, which is what makes separate rects read
// as one object rather than a pile of parts.
//
// The compiled module also deforms the body toward the field it grows from,
// using the spring parameters. The shim exposes the same knobs and settles at
// an undelivered identity deform, so callers that read `deformMatrix` (the
// content warp) stay valid until the compiled module replaces this.
Item {
    id: rect

    // Every caller assigns this explicitly, because members of one field can
    // be declared in different subtrees and reach it by reference. It is a
    // var, so sites that bind to an optional parent still type-check.
    property var group: null
    property real topLeftRadius: 0
    property real topRightRadius: 0
    property real bottomLeftRadius: 0
    property real bottomRightRadius: 0
    property real deformScale: 0.000015
    property bool sinks: true
    property real burial: 0

    // Spring parameters of the grow and retract. Undelivered in the shim: the
    // identity matrix is what the frame curves at rest, minus the wobble.
    property real stiffness: 170
    property real damping: 26
    readonly property var deformMatrix: Qt.matrix4x4()

    Rectangle {
        anchors.fill: parent
        radius: Math.max(rect.topLeftRadius, rect.topRightRadius, rect.bottomLeftRadius, rect.bottomRightRadius)
        color: rect.group ? rect.group.color : "transparent"
        border.width: rect.group ? rect.group.borderWidth : 0
        border.color: rect.group && rect.group.borderColor !== "transparent" ? rect.group.borderColor : Qt.rgba(0, 0, 0, 0)
        visible: rect.visible && width > 0 && height > 0 && radius >= 0
        antialiasing: true
    }
}
