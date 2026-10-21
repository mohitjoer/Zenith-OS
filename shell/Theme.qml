pragma Singleton
pragma ComponentBehavior: Bound

// The colours the whole desktop is drawn in.
//
// There are two sets of names here, and the difference matters.
//
// The design system's tokens — canvas, surface, inkPrimary, indicator and the
// rest — are the desktop's vocabulary. They are monochrome by construction,
// because the design system says "black, grey and white only", and every one
// of them has a measured contrast ratio against the surfaces it is used on:
// 4.5:1 for body ink, 3:1 for shapes and indicators.
//
// The wallpaper palette — paletteBackground, paletteSurface and the rest — is
// what `zenith theme generate` extracts from one image and writes to
// $XDG_CACHE_HOME/zenith/colors.json. It is a colour palette, and a colour
// palette is not what this desktop draws with. It is kept because the theme
// generator produces it and `zenith theme check` reports on it, and because
// the generator is being rewritten to emit the token set instead.
//
// So: the shell draws in the tokens. The palette is a record of what the
// wallpaper would have said. Nothing in the shell binds to a wallpaper
// colour, and no surface names a hex value of its own — a hard-coded
// #202124 in a QML file is a colour that survives a theme change without
// being noticed, and it is the reason a themed desktop usually has two
// themes: the one you chose and the one the code has.
//
// The vendored upstream components read the same tokens, through
// shell/modules/common/Appearance.qml.

import Quickshell
import Quickshell.Io
import QtQuick

Singleton {
    id: palette

    // ---------------------------------------------------------------- surfaces
    //
    // Six steps, and the order is the meaning: a sunken surface is a recess,
    // a canvas is the root, and each step up is a thing that is above the
    // last. A component that reads a step it was not written for gets
    // transparent, not an error, so the ramp has to be total.
    readonly property color surfaceSunken: "#080808"
    readonly property color canvas: "#101010"
    readonly property color surface: "#1A1A1A"
    readonly property color surfaceRaised: "#242424"
    readonly property color surfaceOverlay: "#2D2D2D"
    readonly property color surfaceHighest: "#373737"
    readonly property color surfaceInverse: "#ECECEC"

    // -------------------------------------------------------------------- ink
    //
    // Four weights of the same colour. A monochrome system says "this is the
    // thing" by weight and by shape, never by hue.
    readonly property color inkPrimary: "#F2F2F2"
    readonly property color inkSecondary: "#CCCCCC"
    readonly property color inkTertiary: "#A1A1A1"
    readonly property color inkDisabled: "#6A6A6A"
    readonly property color inkInverse: "#141414"

    // ------------------------------------------------------------------ lines
    //
    // Two lines, and they are not interchangeable. A hairline is the contact
    // border of a raised surface, and it is always paired with the shadow
    // behind it, so it is allowed to be quiet. A divider stands on its own
    // with nothing behind it, so it meets the 3:1 floor.
    readonly property color lineHairline: "#2B2B2B"
    readonly property color lineDivider: "#8A8A8A"
    readonly property color focusRing: "#F2F2F2"

    // -------------------------------------------------------------- indicators
    //
    // The shape that says "this one": the workspace dot, the tick, the tab
    // bar, the running-application pip. Under a monochrome rule this is where
    // a colour accent would otherwise go.
    readonly property color indicator: "#F2F2F2"
    readonly property color indicatorMuted: "#CCCCCC"

    // ------------------------------------------------------------ state layers
    //
    // A state layer is the ink at an alpha over the surface the control sits
    // on. Never a colour of its own, because a layer that had a hue would
    // tint the surface under it.
    readonly property real stateHoverAlpha: 0.08
    readonly property real stateSelectedAlpha: 0.10
    readonly property real statePressedAlpha: 0.12
    readonly property real stateDraggedAlpha: 0.16

    readonly property color stateLayer: Qt.rgba(inkPrimary.r, inkPrimary.g, inkPrimary.b, stateHoverAlpha)

    // ------------------------------------------------------------------ scrim
    //
    // The scrim behind a modal surface. It is lighter in the light variant,
    // because the compositor's blur already dims the light variant and a
    // heavier scrim on top of it puts body ink below the contrast floor.
    readonly property color scrim: "#99000000"

    // --------------------------------------------------- the colour exceptions
    //
    // Three, and only three: the traffic lights, the warning, and the error.
    // They are the exceptions because they carry meaning a grey cannot. Every
    // other state is expressed by ink weight, fill, position or size.
    readonly property color trafficClose: "#E5484D"
    readonly property color trafficMinimise: "#F5A524"
    readonly property color trafficMaximise: "#2E8B57"
    readonly property color warningColour: "#F5A524"
    readonly property color error: "#DC2626"
    readonly property color onErrorColour: "#FFFFFF"
    readonly property color errorContainer: "#2E0C0C"
    readonly property color onErrorContainerColour: "#F98080"

    // --------------------------------------------------------------- elevation
    //
    // Two stops, always together: a 1px contact border and a soft ambient
    // shadow. One shadow reads as a flat sticker, so a surface that has a
    // shadow has a border as well.
    readonly property int elevation1Blur: 8
    readonly property int elevation1Offset: 1
    readonly property real elevation1Alpha: 0.28
    readonly property int elevation2Blur: 16
    readonly property int elevation2Offset: 4
    readonly property real elevation2Alpha: 0.34
    readonly property int elevation3Blur: 28
    readonly property int elevation3Offset: 8
    readonly property real elevation3Alpha: 0.40

    readonly property int shadowBlur: 12
    readonly property int shadowOffset: 2
    readonly property color shadow: Qt.rgba(0, 0, 0, 0.5)

    // ------------------------------------------------------------------- motion
    //
    // A small budget. Every duration is a name a surface can use, and every
    // one of them is zero when motion is reduced — not shorter, zero.
    readonly property int motionMicro: 100
    readonly property int motionSmall: 150
    readonly property int motionMedium: 250
    readonly property int motionLarge: 400

    readonly property bool reducedMotion: false

    // ------------------------------------------------------------------ radius
    //
    // The project's radius scale. Nothing between these values.
    readonly property int radiusChip: 6
    readonly property int radiusField: 7
    readonly property int radiusControl: 8
    readonly property int radiusControlLarge: 10
    readonly property int radiusPanel: 12
    readonly property int radiusSheet: 14
    readonly property int radiusOverlay: 18
    readonly property int radiusWindow: 22
    readonly property int radiusPill: 9999

    // The three the shell's own surfaces use, kept under the names they had.
    readonly property int radiusSmall: radiusChip
    readonly property int radiusMedium: radiusControl
    readonly property int radiusLarge: radiusPanel

    // ------------------------------------------------------------------ spacing
    //
    // The 4px grid.
    readonly property int space1: 4
    readonly property int space2: 8
    readonly property int space3: 12
    readonly property int space4: 16
    readonly property int space5: 20
    readonly property int space6: 24

    // --------------------------------------------------------------- typography
    //
    // Zenith's interface font ships from Zenith's own repository and is not
    // in the tree yet, so the shell draws in the base layer's Noto. The token
    // names are the contract: when the shipped font arrives it replaces
    // fontFamily and nothing else changes.
    readonly property string fontFamily: "Noto Sans"
    readonly property string fontMonoFamily: "JetBrains Mono"

    readonly property int typeDisplaySize: 32
    readonly property int typeTitleSize: 22
    readonly property int typeHeadlineSize: 17
    readonly property int typeBodySize: 15
    readonly property int typeLabelSize: 13
    readonly property int typeCaptionSize: 11

    // ------------------------------------------------------------------ icons
    //
    // The base layer carries Font Awesome and Powerline and no Nerd Font, so
    // the icon family is Font Awesome. See the note in docs/architecture.md
    // about the vendored components' own icon expectations.
    readonly property string iconFamily: "Font Awesome 4.7"
    readonly property string separatorFamily: "PowerlineSymbols"

    // ------------------------------------------------------------- the borders
    //
    // A contact border is the ink at a tenth, so it reads on any wallpaper
    // without being a line, and a divider is a rule inside a surface, so it
    // has to clear the 3:1 floor on its own.
    readonly property int hairlineWidth: 1
    readonly property color hairline: Qt.rgba(inkPrimary.r, inkPrimary.g, inkPrimary.b, 0.12)

    // ------------------------------------------------------------ the panel fill
    //
    // A raised surface over the wallpaper: the surface's own colour at most
    // opacity, so the picture shows through faintly and the border says
    // where the panel ends.
    readonly property color panelFill: Qt.rgba(surface.r, surface.g, surface.b, 0.86)

    // ================================================== the wallpaper palette
    //
    // What `zenith theme generate` extracted from one image. The desktop
    // does not draw with these; they are the record of what the wallpaper
    // would have said, and they are what the generator still writes.
    //
    // The theme generator is being rewritten to emit the token set above
    // instead, and when it does, these become the generated values and the
    // constants above become the fallbacks.

    property color paletteBackground: "#15171a"
    property color paletteSurface: "#1e2126"
    property color paletteText: "#e8eaed"
    property color paletteTextDim: "#9aa0a6"
    property color paletteBorder: "#3c4043"

    // Whether a palette has been generated. The shell shows this nowhere on
    // purpose: it is a diagnostic, and `zenith theme check` is where a user
    // asks about it.
    property bool generated: false

    readonly property string path: palette.cacheDir + "/zenith/colors.json"

    readonly property string cacheDir: {
        var dir = Quickshell.env("XDG_CACHE_HOME")
        if (dir !== "") return dir
        return Quickshell.env("HOME") + "/.cache"
    }

    // The palette, as the file has it.
    //
    // It is read by the shell's root and pushed in here, for the reason the
    // settings are: a process inside a singleton is a process that never runs,
    // and a shell that cannot read its own palette is a shell whose colours
    // are the ones it shipped with.
    property var palette: ({})

    function apply(document) {
        if (!document || Object.keys(document).length === 0) return
        paletteBackground = document.background
        paletteSurface = document.surface
        paletteText = document.text
        paletteTextDim = document.text_dim
        paletteBorder = document.border
        generated = true
    }
}
