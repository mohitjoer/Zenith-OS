pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import qs.Ui.Singletons

// Welcome-local values only. The look -- paper, ink, type, geometry, motion --
// is qs.Ui's Tokens and nothing here duplicates it (the one-look rule; the
// eleven drifted Theme copies are the cautionary tale). What remains is the
// brand-identity plumbing this once-per-login window reads for itself, and the
// one scrim tint the threshold art needs.
Singleton {
    // content backing over the art: the paper flooding back in so copy stays
    // legible while the backdrop breathes around it. Derived from Tokens.paper,
    // never a second black.
    readonly property color panel: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.66)

    // brand mark + name, user-overridable via ~/.config/zenith/brand.json (Shell ->
    // Global). defaults to the 天 seal / "Zenith". BrandMark renders `mark`, or
    // `markSource` (an image) when set. Zenith's own apps never read these.
    readonly property string mark: brandAdapter.markText.length > 0 ? brandAdapter.markText : "\u5929"
    readonly property string markSource: brandAdapter.markImage
    readonly property bool markTint: brandAdapter.markTint
    readonly property string brandName: brandAdapter.name.length > 0 ? brandAdapter.name : "Zenith"

    // brand identity master (mark + name), the same ~/.config/zenith/brand.json the
    // rest of the shell and the Hub read. read-only here: the always-on
    // pill seeds the file, so this once-per-login window just falls back to the
    // adapter defaults when it is absent.
    FileView {
        id: brandFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/zenith/brand.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        JsonAdapter {
            id: brandAdapter
            property string markText: "天"
            property string markImage: ""
            property bool markTint: true
            property string name: "Zenith"
        }
    }
}
