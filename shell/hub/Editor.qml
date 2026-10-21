pragma ComponentBehavior: Bound

// The base every editor in the Hub is built on.
//
// It is in the Hub's own directory rather than beside the editors in
// `Controls/`, and that is a fact about the engine rather than about the
// design. A file in a subdirectory cannot name a type from the directory
// beside it by relative import: the editors reach their base through the
// module that stands for the Hub's directory, which is `qs.hub`, and a module
// is named by its directory and not by the files in it. The base being at the
// top of that directory is what makes it a type of the module, so the editors
// get it with one import and the load order does not matter.
//
// Three things live here because all five editors need them and none of them
// belongs to any one of them: the value, the write, and the error.
//
// The value first. Every setting the export carries is a GVariant literal in
// text form, and the two directions do not spell the same value the same way.
// An enum's nick arrives quoted and has to be written bare, because the tool
// compares it against the schema's nicks and a quoted nick is not a nick.
//
// The write second, because a refused write that is not shown is a refused
// write that looks like a change. The editor shows the message and goes back
// to the value the tool last accepted, because a value on screen is a claim
// about the setting and the setting did not change.
//
// The error last, and it is only a line of text under the control. The error
// colour is one of the three exceptions to the monochrome rule, because a
// refused write is one of the three states a grey cannot carry.

import Quickshell
import QtQuick
import qs

Item {
    id: editor

    // The setting this editor is for.
    //
    // Required, because a required property has to be initialised while the
    // object that declares it is being created, and the page does that: it
    // gives the editor its key as it loads it. An editor whose key arrives
    // afterwards is an editor that has already read an undefined key, and
    // every binding below it is wrong for a moment.
    required property var key

    // The setting's identity, as the tool names it.
    readonly property string qualified: key.schema + "." + key.name

    // The value the tool last accepted, in the tool's spelling.
    //
    // Writable, because the export is read once: a value written after it was
    // read is not in it, and an editor that reverted to the export after every
    // write would show the setting as it was before the user touched it.
    property string accepted: spell(key.value)

    // The value being offered, in the same spelling.
    //
    // Empty means nothing is pending, which is what a refused write leaves
    // behind: the editor falls back to the accepted value.
    property string proposed: ""

    // What the editor shows.
    readonly property string current: proposed !== "" ? proposed : accepted

    // The message from the last refused write.
    property string error: ""

    // The value of the write that has not reported back yet.
    property string inFlight: ""

    // Whether the setting is an enum.
    //
    // An enum is not a type of its own: it is a string with named values. The
    // export quotes the name and the tool wants it bare, and every other type
    // is spelled the same in both directions, so the rule is the type itself
    // rather than which editor is showing it.
    readonly property bool enumeration: key.type !== "s" && key.type !== "i"
        && key.type !== "b" && key.type !== "as"

    // The value as the tool wants it.
    function spell(text) {
        return enumeration ? GVariant.unquote(text) : text
    }

    // A write is proposed.
    //
    // One at a time per setting. A second edit made while a write is still in
    // flight waits for the first to report back, so the two do not race for
    // the same value and the second does not inherit the first's outcome.
    function commit(value) {
        proposed = value
        if (inFlight !== "") return
        write(value)
    }

    function write(value) {
        inFlight = value
        Zenith.set(key, value)
    }

    // The write reported back.
    //
    // The value is compared before anything is changed, because an editor that
    // reverts on a write it did not make throws away what the user is typing.
    function settled(value, failed, message) {
        if (value !== inFlight) return
        inFlight = ""

        if (failed) {
            error = message
            if (proposed === value) proposed = ""
        } else {
            error = ""
            accepted = value
            if (proposed === value) proposed = ""
        }

        // Anything the user changed while the write was in flight is written
        // now, so the setting ends up where the user last left it.
        if (proposed !== "" && proposed !== accepted) write(proposed)
    }

    Connections {
        target: Zenith
        function onSettled(qualified, value, failed, message) {
            if (qualified !== editor.qualified) return
            editor.settled(value, failed, message)
        }
    }

    // The editor's own content.
    default property alias content: body.data

    Item {
        id: body
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: childrenRect.height
    }

    // The message under the control.
    //
    // It is under the control rather than over it, because a message that
    // covers the value the user is looking at is a message about a value
    // they cannot check.
    Text {
        id: message
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: body.bottom
        anchors.topMargin: visible ? Theme.space1 : 0
        visible: editor.error !== ""
        text: editor.error
        color: Theme.error
        font.family: Theme.fontFamily
        font.pixelSize: Theme.typeCaptionSize
        wrapMode: Text.WordWrap
    }

    implicitHeight: body.height
        + (message.visible ? message.height + Theme.space1 : 0)
}
