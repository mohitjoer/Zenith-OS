pragma ComponentBehavior: Bound

// One chord, drawn as key caps.
//
// A chord is written the way the catalogue writes it — "SUPER + SHIFT + {n}" —
// and this is where it becomes something a person reads: a small box per key,
// with a hairline around it, and the name the key goes by rather than the
// name the keyboard reports.
//
// The names are here rather than in the catalogue because they are a question
// of display. The catalogue says XF86AudioRaiseVolume because that is what
// the keyboard sends, and "Vol +" is what a person would say about it. The
// catalogue is about what the keys are; this is about what they are called.

import QtQuick

Item {
    id: caps

    // The chord, in the catalogue's spelling.
    property string chord: ""

    // The keys of the chord, in order.
    readonly property var tokens: chord === "" ? [] : chord.split(" + ")

    // The name each key goes by on screen.
    //
    // A key with no name here is drawn with the name the keyboard gave it,
    // which is the right answer for the keys nobody has thought of a name
    // for: the sheet would rather say "XF86Foo" than say nothing.
    readonly property var names: ({
        "SUPER": "Super",
        "SHIFT": "Shift",
        "CTRL": "Ctrl",
        "ALT": "Alt",
        "Return": "Enter",
        "Escape": "Esc",
        "Prior": "Page Up",
        "Next": "Page Down",
        "space": "Space",
        "Space": "Space",
        "comma": ",",
        "period": ".",
        "bracketleft": "[",
        "bracketright": "]",
        "grave": "`",
        "XF86AudioRaiseVolume": "Vol +",
        "XF86AudioLowerVolume": "Vol -",
        "XF86AudioMute": "Mute",
        "XF86AudioPlay": "Play",
        "XF86AudioNext": "Next",
        "XF86AudioPrev": "Prev",
        "XF86MonBrightnessUp": "Brightness +",
        "XF86MonBrightnessDown": "Brightness -"
    })

    // What a token is called on screen.
    //
    // The family of ten keys is one token in the catalogue and one box here,
    // and it says the whole of it: the number row, from one to zero.
    function display(token: string): string {
        if (token === "{n}") {
            return "1 … 0";
        }
        var name = caps.names[token];
        return name !== undefined ? name : token;
    }

    implicitHeight: 22
    implicitWidth: row.implicitWidth

    Row {
        id: row
        anchors.verticalCenter: parent.verticalCenter
        spacing: Theme.space1

        Repeater {
            model: caps.tokens

            delegate: Row {
                id: token
                required property var modelData
                required property int index

                spacing: Theme.space1

                // The plus between two keys, drawn between them rather than
                // after each of them, so that a chord of one key has no
                // trailing plus hanging off it.
                Text {
                    visible: token.index > 0
                    anchors.verticalCenter: parent.verticalCenter
                    text: "+"
                    color: Theme.inkSecondary
                    font.pixelSize: 11
                }

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    width: label.implicitWidth + Theme.space2 * 2
                    height: 20
                    radius: Theme.radiusSmall
                    // The state layer rather than a colour of its own: a cap
                    // is a raised thing, and the layer is the shell's own way
                    // of saying that without inventing a second palette.
                    color: Theme.stateLayer
                    border.width: Theme.hairlineWidth
                    border.color: Theme.hairline

                    Text {
                        id: label
                        anchors.centerIn: parent
                        text: caps.display(token.modelData)
                        color: Theme.inkPrimary
                        font.pixelSize: 12
                    }
                }
            }
        }
    }
}
