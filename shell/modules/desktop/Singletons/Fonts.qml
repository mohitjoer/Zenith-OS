pragma Singleton
import QtQuick
import Quickshell

// Bundles open-licensed display faces into the shell process so any widget's
// font.family -- and Theme.font, the user-picked widget font -- can name one
// without the family being installed system-wide. Each FontLoader registers its
// file with the font database; `.name` is the family the engine actually
// resolved (authoritative -- never a guessed string), so `families` is exactly
// what the Hub's picker offers and what a widget sets as its font.family.
Singleton {
    // the resolved family names, in bundle order. Used to seed the widget-font
    // picker so the desktop can render any bundled face.
    readonly property var families: [
        spaceGrotesk.name, spaceGroteskMedium.name, spaceGroteskBold.name,
        spaceGroteskLight.name, jetBrainsMono.name
    ]

    FontLoader { id: spaceGrotesk;       source: Qt.resolvedUrl("../../../assets/fonts/SpaceGrotesk-Regular.ttf") }
    FontLoader { id: spaceGroteskMedium; source: Qt.resolvedUrl("../../../assets/fonts/SpaceGrotesk-Medium.ttf") }
    FontLoader { id: spaceGroteskBold;   source: Qt.resolvedUrl("../../../assets/fonts/SpaceGrotesk-Bold.ttf") }
    FontLoader { id: spaceGroteskLight;  source: Qt.resolvedUrl("../../../assets/fonts/SpaceGrotesk-Light.ttf") }
    FontLoader { id: jetBrainsMono;      source: Qt.resolvedUrl("../../../assets/fonts/JetBrainsMonoNerdFont-Regular.ttf") }
}
