pragma ComponentBehavior: Bound

// Every glyph the shell draws, and the font it comes from.
//
// One map, one file, and the shell check asserts that no glyph is used outside
// it. The reason is that a codepoint a font does not contain renders as a
// hollow box: no error, nothing in the log, and a status area that looks
// decorative rather than broken. Three widgets each naming their own codepoint
// is three places to get one wrong, and nothing would notice.
//
// The families below are Font Awesome 4.7 and Powerline, both from the Ubuntu
// archive. They are not Nerd Fonts, and nothing here pretends otherwise — see
// the reasoning in system/packages.yaml.

import Quickshell
import QtQuick

Singleton {
    id: icons

    // The font families, named so a reader can check them against the manifest
    // rather than against the QML's assumption that they exist.
    readonly property string iconFamily: "Font Awesome 4.7"
    readonly property string separatorFamily: "PowerlineSymbols"

    // Font Awesome 4.7 codepoints, written as \uXXXX escapes rather than as
    // raw characters.
    //
    // The escapes are the whole point. A raw private-use character in source is
    // a sequence of bytes that survives a copy, a paste and a rebase only if
    // every one of them preserves it, and a glyph that changes from U+F8FF to
    // U+F293 by accident is a status area drawing the wrong symbol with nothing
    // to say so. Written as an escape it is reviewable, diffable, and checkable
    // against the font's codepoint list by reading it.
    //
    // An earlier version of this file used raw characters and carried codepoints
    // from the upper private use area, which is not where Font Awesome 4.7 puts
    // anything. Nothing caught it, which is the argument for the range check in
    // the shell validator.
    readonly property var glyphs: ({
        "bluetooth": "\uf293",
        "battery-empty": "\uf244",
        "battery-quarter": "\uf243",
        "battery-half": "\uf242",
        "battery-full": "\uf241",
        "battery-charging": "\uf240",
        "wifi": "\uf1eb",
        "volume-mute": "\uf026",
        "volume-low": "\uf027",
        "volume-high": "\uf028",
        "lock": "\uf023",
        "unlock": "\uf09c",
        "settings": "\uf013",
        "launcher": "\uf187",
        "terminal": "\uf120",
        "camera-off": "\uf030",
    })

    // Powerline separators. These are what a bar is divided with; the archive
    // carries a dedicated package for them and no general icon font includes
    // them, which is the only reason it is installed at all.
    readonly property var separators: ({
        "slash": "\ue0b0",
        "backslash": "\ue0b1",
        "branch": "\ue0bc"
    })

    // Look a glyph up by name, falling back to nothing rather than to a wrong
    // codepoint. A status icon that cannot be drawn is absent; one that shows
    // the wrong glyph is a lie about the machine's state, and the state is the
    // only reason the icon is there.
    function glyph(name) {
        var found = glyphs[name]
        return found === undefined ? "" : found
    }
}
