pragma Singleton
import QtQuick
import Quickshell

// The plugin kit's design tokens. Colour resolves from the daemon palette
// through the Scheme singleton so a plugin's card and ink follow the active theme
// (a fixed named scheme or the live wallpaper) instead of a hardcoded tint.
// The design system is monochrome, so the accent is ink and every fixed mark
// below is a grey. The names stay as the kit has always spelled them (verm,
// flame, ember ...) so a plugin written against them still loads.
Singleton {
    // accent family: the live system accent (palette primary), with lit/deep
    // variants tinted off it. brand + verm are the accent under their historical
    // names, so existing plugin content follows the theme with no change.
    readonly property color accent:   Scheme.accent
    readonly property color brand:    Scheme.accent
    readonly property color verm:     brand
    readonly property color vermLit:  Qt.lighter(Scheme.accent, 1.22)
    readonly property color vermDeep: Qt.darker(Scheme.accent, 1.3)
    // fixed marks, never themed.
    readonly property color sun:      "#f2f2f2"
    readonly property color gold:     "#cccccc"
    // text ramp, resolved from the palette so a plugin's copy reads on the themed
    // card in light and dark alike.
    readonly property color cream:    Scheme.onSurface
    readonly property color bright:   Scheme.onSurface
    readonly property color dim:      Scheme.onSurfaceVariant
    // card surface, from the palette depth ramp. panelTop/panelBot are the Card
    // gradient stops (aliases of cardTop/cardBot).
    readonly property color cardTop:  Scheme.base
    readonly property color cardBot:  Scheme.deep
    readonly property color panelTop: cardTop
    readonly property color panelBot: cardBot
    readonly property color border:   Scheme.line
    readonly property color lineStrong: Qt.rgba(bright.r, bright.g, bright.b, 0.40)
    readonly property color shadow:   "#000000"
    readonly property color tileBg:   Scheme.elevated
    readonly property color subtle:   Scheme.onSurface
    readonly property color faint:    Scheme.onSurfaceVariant
    readonly property color iconDim:  Scheme.onSurfaceVariant
    readonly property color hair:     Qt.rgba(bright.r, bright.g, bright.b, 0.12)
    readonly property color sheen:    Qt.rgba(bright.r, bright.g, bright.b, 0.06)
    // ember tones for the flame gradients, as greys.
    readonly property color vermDim:  "#8a8a8a"
    readonly property color vermDimDeep: "#4a4a4a"
    readonly property color vermBurn: "#6a6a6a"
    readonly property color tickRest: Scheme.onSurfaceVariant
    readonly property color threadBg: Qt.rgba(accent.r, accent.g, accent.b, 0.13)
    readonly property color flameCore: "#e6e6e6"
    readonly property color flameGlow: "#b0b0b0"

    // flame canvas ramp: literal hex strings (color type breaks here), fed
    // straight into Canvas addColorStop/strokeStyle. a color property serializes
    // to #aarrggbb and corrupts the gradient render.
    readonly property string flameInk:   "#d0d0d0"
    readonly property string flameEmber: "#4a4a4a"
    readonly property string flameBurn:  "#6a6a6a"
    readonly property string flameTip:   "#f0f0f0"
    readonly property color todayWarm: "#cccccc"
    readonly property color ghost:     "#6a6a6a"
    // Rest-card sky: fixed day/night scene tones.
    readonly property color sunGold:  "#e6e6e6"
    readonly property color moonGlow: "#a1a1a1"
    readonly property color moonDisc: "#cccccc"
    readonly property color frameBg:     Qt.rgba(accent.r, accent.g, accent.b, 0.10)
    readonly property color frameBorder: Qt.rgba(bright.r, bright.g, bright.b, 0.18)
    readonly property color creamMenu:   Qt.rgba(bright.r, bright.g, bright.b, 0.82)
    readonly property real shadowOpacity: 0.5
    // type stack and the default control radius (the scale is 6/7/8/10/12/...).
    readonly property string display: "Fraunces"
    readonly property string font: "Space Grotesk"
    readonly property string fontJp: "Noto Sans CJK JP"
    readonly property string mono: "JetBrainsMono Nerd Font"
    readonly property int radius: 8
    readonly property real border2: 1
    readonly property int shadowStep: 6

    // MPRIS trackArtists comes back as a JS array from some players and a
    // plain string from others (Spotify). calling .join on the string throws
    // and kills the whole binding. handle both, fall back to trackArtist.
    function joinArtists(artists, single) {
        if (artists && typeof artists.join === "function" && artists.length > 0)
            return artists.join(", ");
        if (artists && String(artists).length > 0)
            return String(artists);
        return single ? String(single) : "";
    }
}
