import QtQuick
import Quickshell
import qs

// The vendored components' theme, drawn in Zenith's tokens.
//
// This is an overlay: it sits at upstream's path, under upstream's name, and
// speaks upstream's property names — `m3colors.m3background`,
// `colors.colLayer1`, `rounding.normal`, `font.pixelSize.small` — so a
// vendored file that reads a colour reads Zenith's colour without being
// edited. That is the whole of the re-theme.
//
// Two things follow from that, and they are the reason this file is shaped
// the way it is.
//
// First, every value here is a Zenith token. There is no wallpaper palette,
// no quantiser, no Material colour scheme: the upstream file derives a
// palette from the wallpaper with `ColorQuantizer`, and a derived palette is
// a second theme that changes when the wallpaper does. This desktop is
// monochrome by the design system's first rule, so the ramp is ink — a
// surface step is a lightness step, and a state is a weight of ink, not a
// tint.
//
// Second, the names are upstream's and the values are ours, so the whole set
// is present rather than only the names something reads today. A component
// that reads a name which is missing here gets `undefined`, and `undefined`
// assigned to a colour is a binding error in the journal — a failure, but a
// quiet one, on a surface nobody may be looking at. A complete set costs one
// line per name and cannot fail that way.
//
// The three colour exceptions in the design system are carried through as
// themselves: error is red, warning is amber, and the traffic lights are the
// traffic lights. Everything else is black, grey and white.
pragma Singleton
pragma ComponentBehavior: Bound

Singleton {
    id: root

    // Where upstream's components ask for a colour.
    property QtObject m3colors: QtObject {
        property bool darkmode: true
        property bool transparent: false

        // Surfaces. The order is the meaning: background is the root, and
        // each container step is a thing above the last.
        property color m3background: Theme.canvas
        property color m3onBackground: Theme.inkPrimary
        property color m3surface: Theme.canvas
        property color m3surfaceDim: Theme.surfaceSunken
        property color m3surfaceBright: Theme.surface
        property color m3surfaceContainerLowest: Theme.surfaceSunken
        property color m3surfaceContainerLow: Theme.canvas
        property color m3surfaceContainer: Theme.surface
        property color m3surfaceContainerHigh: Theme.surfaceRaised
        property color m3surfaceContainerHighest: Theme.surfaceOverlay
        property color m3onSurface: Theme.inkPrimary
        property color m3surfaceVariant: Theme.surfaceRaised
        property color m3onSurfaceVariant: Theme.inkSecondary
        property color m3inverseSurface: Theme.surfaceInverse
        property color m3inverseOnSurface: Theme.inkInverse

        // Lines.
        property color m3outline: Theme.lineDivider
        property color m3outlineVariant: Theme.lineHairline

        // Recess and shadow.
        property color m3shadow: "#000000"
        property color m3scrim: "#000000"
        property color m3surfaceTint: Theme.inkPrimary

        // The one accent the design system allows a surface to carry is ink
        // at full strength, so `primary` is the indicator's ink. A component
        // that paints itself "primary" paints itself the colour of a selected
        // thing, which is what primary means.
        property color m3primary: Theme.indicator
        property color m3onPrimary: Theme.inkInverse
        property color m3primaryContainer: Theme.surfaceRaised
        property color m3onPrimaryContainer: Theme.inkPrimary
        property color m3inversePrimary: Theme.inkTertiary
        property color m3secondary: Theme.inkSecondary
        property color m3onSecondary: Theme.inkInverse
        property color m3secondaryContainer: Theme.surfaceOverlay
        property color m3onSecondaryContainer: Theme.inkPrimary
        property color m3tertiary: Theme.inkSecondary
        property color m3onTertiary: Theme.inkInverse
        property color m3tertiaryContainer: Theme.surfaceRaised
        property color m3onTertiaryContainer: Theme.inkSecondary

        // The exceptions.
        property color m3error: Theme.error
        property color m3onError: Theme.onErrorColour
        property color m3errorContainer: Theme.errorContainer
        property color m3onErrorContainer: Theme.onErrorContainerColour

        // Fixed roles are the same roles at a fixed lightness; in a
        // monochrome palette there is nothing for them to vary, so they are
        // the ramp again.
        property color m3primaryFixed: Theme.inkPrimary
        property color m3primaryFixedDim: Theme.inkSecondary
        property color m3onPrimaryFixed: Theme.inkInverse
        property color m3onPrimaryFixedVariant: Theme.surfaceRaised
        property color m3secondaryFixed: Theme.inkPrimary
        property color m3secondaryFixedDim: Theme.inkSecondary
        property color m3onSecondaryFixed: Theme.inkInverse
        property color m3onSecondaryFixedVariant: Theme.surfaceRaised
        property color m3tertiaryFixed: Theme.inkPrimary
        property color m3tertiaryFixedDim: Theme.inkSecondary
        property color m3onTertiaryFixed: Theme.inkInverse
        property color m3onTertiaryFixedVariant: Theme.surfaceRaised

        // Success is not one of the design system's exceptions, so it is ink.
        // A confirmation that is green and an error that is red makes colour
        // carry a meaning the design system says only weight carries; a
        // confirmed state is a filled shape and a failed one is the red.
        property color m3success: Theme.inkSecondary
        property color m3onSuccess: Theme.inkInverse
        property color m3successContainer: Theme.surfaceRaised
        property color m3onSuccessContainer: Theme.inkPrimary

        // Terminal colours. The vendored tree carries an ANSI palette for a
        // terminal widget Zenith does not adopt; it is mapped to the ink ramp
        // so that a terminal drawn with it would still be monochrome.
        property color term0: Theme.inkPrimary
        property color term1: Theme.error
        property color term2: Theme.inkSecondary
        property color term3: Theme.warningColour
        property color term4: Theme.inkSecondary
        property color term5: Theme.inkSecondary
        property color term6: Theme.inkTertiary
        property color term7: Theme.inkPrimary
        property color term8: Theme.inkDisabled
        property color term9: Theme.error
        property color term10: Theme.inkSecondary
        property color term11: Theme.warningColour
        property color term12: Theme.inkSecondary
        property color term13: Theme.inkSecondary
        property color term14: Theme.inkTertiary
        property color term15: Theme.surfaceInverse
    }

    // Where upstream's components ask for a surface, an ink, or a state.
    //
    // The layers are a depth ramp: a component selects a layer by how far
    // above the desktop it sits, and every layer's hover and active step is
    // the next surface up. A state expressed by position in the ramp rather
    // than by a tint is a state that survives a greyscale screenshot.
    property QtObject colors: QtObject {
        property color colSubtext: Theme.inkTertiary

        property color colLayer0Base: Theme.canvas
        property color colLayer0: Theme.canvas
        property color colOnLayer0: Theme.inkPrimary
        property color colLayer0Hover: Theme.surface
        property color colLayer0Active: Theme.surfaceRaised
        property color colLayer0Border: Theme.hairline

        property color colLayer1Base: Theme.surface
        property color colLayer1: Theme.surface
        property color colOnLayer1: Theme.inkPrimary
        property color colOnLayer1Inactive: Theme.inkTertiary
        property color colLayer1Hover: Theme.surfaceRaised
        property color colLayer1Active: Theme.surfaceOverlay

        property color colLayer2Base: Theme.surfaceRaised
        property color colLayer2: Theme.surfaceRaised
        property color colLayer2Hover: Theme.surfaceOverlay
        property color colLayer2Active: Theme.surfaceHighest
        property color colLayer2Disabled: Theme.surface
        property color colOnLayer2: Theme.inkPrimary
        property color colOnLayer2Disabled: Theme.inkDisabled

        property color colLayer3Base: Theme.surfaceOverlay
        property color colLayer3: Theme.surfaceOverlay
        property color colLayer3Hover: Theme.surfaceHighest
        property color colLayer3Active: Theme.surfaceHighest
        property color colOnLayer3: Theme.inkPrimary

        property color colLayer4Base: Theme.surfaceHighest
        property color colLayer4: Theme.surfaceHighest
        property color colLayer4Hover: Theme.surfaceHighest
        property color colLayer4Active: Theme.surfaceHighest
        property color colOnLayer4: Theme.inkPrimary

        property color colPrimary: Theme.indicator
        property color colOnPrimary: Theme.inkInverse
        property color colPrimaryHover: Theme.inkPrimary
        property color colPrimaryActive: Theme.inkPrimary
        property color colPrimaryContainer: Theme.surfaceRaised
        property color colPrimaryContainerHover: Theme.surfaceOverlay
        property color colPrimaryContainerActive: Theme.surfaceHighest
        property color colOnPrimaryContainer: Theme.inkPrimary

        property color colSecondary: Theme.inkSecondary
        property color colSecondaryHover: Theme.inkPrimary
        property color colSecondaryActive: Theme.inkPrimary
        property color colOnSecondary: Theme.inkInverse
        property color colSecondaryContainer: Theme.surfaceRaised
        property color colSecondaryContainerHover: Theme.surfaceOverlay
        property color colSecondaryContainerActive: Theme.surfaceHighest
        property color colOnSecondaryContainer: Theme.inkPrimary

        property color colTertiary: Theme.inkSecondary
        property color colTertiaryHover: Theme.inkPrimary
        property color colTertiaryActive: Theme.inkPrimary
        property color colTertiaryContainer: Theme.surfaceRaised
        property color colTertiaryContainerHover: Theme.surfaceOverlay
        property color colTertiaryContainerActive: Theme.surfaceHighest
        property color colOnTertiary: Theme.inkInverse
        property color colOnTertiaryContainer: Theme.inkPrimary

        property color colBackgroundSurfaceContainer: Theme.panelFill
        property color colSurfaceContainerLow: Theme.canvas
        property color colSurfaceContainer: Theme.surface
        property color colSurfaceContainerHigh: Theme.surfaceRaised
        property color colSurfaceContainerHighest: Theme.surfaceOverlay
        property color colSurfaceContainerHighestHover: Theme.surfaceHighest
        property color colSurfaceContainerHighestActive: Theme.surfaceHighest
        property color colOnSurface: Theme.inkPrimary
        property color colOnSurfaceVariant: Theme.inkSecondary

        property color colTooltip: Theme.surfaceInverse
        property color colOnTooltip: Theme.inkInverse
        property color colScrim: Theme.scrim
        property color colShadow: Theme.shadow
        property color colOutline: Theme.lineDivider
        property color colOutlineVariant: Theme.lineHairline

        property color colError: Theme.error
        property color colErrorHover: Theme.error
        property color colErrorActive: Theme.error
        property color colOnError: Theme.onErrorColour
        property color colErrorContainer: Theme.errorContainer
        property color colErrorContainerHover: Theme.errorContainer
        property color colErrorContainerActive: Theme.errorContainer
        property color colOnErrorContainer: Theme.onErrorContainerColour
    }

    // Corner radii, on the design system's scale.
    //
    // Upstream's own values are its taste — 17 and 23 are in its ramp. This
    // desktop's ramp is 6, 7, 8, 10, 12, 14, 18, 22, and a vendored corner is
    // a corner like any other, so it lands on the nearest step.
    property QtObject rounding: QtObject {
        property int unsharpen: 6
        property int unsharpenmore: 7
        property int verysmall: 8
        property int small: 10
        property int normal: 12
        property int large: 14
        property int verylarge: 18
        property int full: 9999
        property int screenRounding: 22
        property int windowRounding: 22
    }

    // Type.
    //
    // The families are the desktop's, which means a vendored widget cannot
    // name a font the image does not have.
    //
    // `iconMaterial` is the exception, and it is a known gap rather than a
    // mapping. Upstream draws its icons as Material Symbols ligatures — the
    // glyph is selected by spelling its name — and no package on Ubuntu
    // 26.04 carries that font. Pointing it at Font Awesome, which is what the
    // image does have, would render the words `chevron_right` instead of an
    // arrow, so the family stays named and a widget that asks for a glyph
    // draws nothing until a provider exists or the widget is replaced. No
    // adopted surface uses one.
    property QtObject font: QtObject {
        property QtObject family: QtObject {
            property string main: Theme.fontFamily
            property string numbers: Theme.fontMonoFamily
            property string title: Theme.fontFamily
            property string iconMaterial: "Material Symbols Rounded"
            property string iconNerd: Theme.fontFamily
            property string monospace: Theme.fontMonoFamily
            property string reading: Theme.fontFamily
            property string expressive: Theme.fontFamily
        }
        // Zenith's faces are not variable fonts, so there are no axes to set.
        property QtObject variableAxes: QtObject {
            property var main: ({})
            property var numbers: ({})
            property var title: ({})
        }
        property QtObject pixelSize: QtObject {
            property int smallest: Theme.typeCaptionSize
            property int smaller: 12
            property int smallie: Theme.typeLabelSize
            property int small: Theme.typeBodySize
            property int normal: Theme.typeBodySize
            property int large: Theme.typeHeadlineSize
            property int larger: 19
            property int huge: Theme.typeTitleSize
            property int hugeass: 24
            property int title: Theme.typeTitleSize
        }
    }

    // Motion.
    //
    // The curves are upstream's: they are the components' own easing, and a
    // curve is a shape, not a colour or a corner. The durations are Zenith's,
    // because the design system says how long a state change takes, and a
    // vendored animation is a state change.
    property QtObject animationCurves: QtObject {
        readonly property list<real> expressiveFastSpatial: [0.42, 1.67, 0.21, 0.90, 1, 1]
        readonly property list<real> expressiveDefaultSpatial: [0.38, 1.21, 0.22, 1.00, 1, 1]
        readonly property list<real> expressiveSlowSpatial: [0.39, 1.29, 0.35, 0.98, 1, 1]
        readonly property list<real> expressiveEffects: [0.34, 0.80, 0.34, 1.00, 1, 1]
        readonly property list<real> emphasized: [0.05, 0, 2 / 15, 0.06, 1 / 6, 0.4, 5 / 24, 0.82, 0.25, 1, 1, 1]
        readonly property list<real> emphasizedFirstHalf: [0.05, 0, 2 / 15, 0.06, 1 / 6, 0.4, 5 / 24, 0.82]
        readonly property list<real> emphasizedLastHalf: [5 / 24, 0.82, 0.25, 1, 1, 1]
        readonly property list<real> emphasizedAccel: [0.3, 0, 0.8, 0.15, 1, 1]
        readonly property list<real> emphasizedDecel: [0.05, 0.7, 0.1, 1, 1, 1]
        readonly property list<real> standard: [0.2, 0, 0, 1, 1, 1]
        readonly property list<real> standardAccel: [0.3, 0, 1, 1, 1, 1]
        readonly property list<real> standardDecel: [0, 0, 0, 1, 1, 1]
        readonly property real expressiveFastSpatialDuration: Theme.motionSmall
        readonly property real expressiveDefaultSpatialDuration: Theme.motionMedium
        readonly property real expressiveSlowSpatialDuration: Theme.motionLarge
        readonly property real expressiveEffectsDuration: Theme.motionMicro
    }

    property QtObject animation: QtObject {
        property QtObject elementMove: QtObject {
            property int duration: Theme.motionMedium
            property int type: Easing.BezierSpline
            property list<real> bezierCurve: root.animationCurves.expressiveDefaultSpatial
            property int velocity: 650
            property Component numberAnimation: Component {
                NumberAnimation {
                    alwaysRunToEnd: true
                    duration: root.animation.elementMove.duration
                    easing.type: root.animation.elementMove.type
                    easing.bezierCurve: root.animation.elementMove.bezierCurve
                }
            }
        }
        property QtObject elementMoveSmall: QtObject {
            property int duration: Theme.motionSmall
            property int type: Easing.BezierSpline
            property list<real> bezierCurve: root.animationCurves.expressiveFastSpatial
            property int velocity: 650
            property Component numberAnimation: Component {
                NumberAnimation {
                    alwaysRunToEnd: true
                    duration: root.animation.elementMoveSmall.duration
                    easing.type: root.animation.elementMoveSmall.type
                    easing.bezierCurve: root.animation.elementMoveSmall.bezierCurve
                }
            }
        }
        property QtObject elementMoveEnter: QtObject {
            property int duration: Theme.motionMedium
            property int type: Easing.BezierSpline
            property list<real> bezierCurve: root.animationCurves.emphasizedDecel
            property int velocity: 650
            property Component numberAnimation: Component {
                NumberAnimation {
                    alwaysRunToEnd: true
                    duration: root.animation.elementMoveEnter.duration
                    easing.type: root.animation.elementMoveEnter.type
                    easing.bezierCurve: root.animation.elementMoveEnter.bezierCurve
                }
            }
        }
        property QtObject elementMoveExit: QtObject {
            property int duration: Theme.motionSmall
            property int type: Easing.BezierSpline
            property list<real> bezierCurve: root.animationCurves.emphasizedAccel
            property int velocity: 650
            property Component numberAnimation: Component {
                NumberAnimation {
                    alwaysRunToEnd: true
                    duration: root.animation.elementMoveExit.duration
                    easing.type: root.animation.elementMoveExit.type
                    easing.bezierCurve: root.animation.elementMoveExit.bezierCurve
                }
            }
        }
        property QtObject elementMoveFast: QtObject {
            property int duration: Theme.motionMicro
            property int type: Easing.BezierSpline
            property list<real> bezierCurve: root.animationCurves.expressiveEffects
            property int velocity: 850
            property Component colorAnimation: Component {
                ColorAnimation {
                    duration: root.animation.elementMoveFast.duration
                    easing.type: root.animation.elementMoveFast.type
                    easing.bezierCurve: root.animation.elementMoveFast.bezierCurve
                }
            }
            property Component numberAnimation: Component {
                NumberAnimation {
                    alwaysRunToEnd: true
                    duration: root.animation.elementMoveFast.duration
                    easing.type: root.animation.elementMoveFast.type
                    easing.bezierCurve: root.animation.elementMoveFast.bezierCurve
                }
            }
        }
        property QtObject elementResize: QtObject {
            property int duration: Theme.motionMedium
            property int type: Easing.BezierSpline
            property list<real> bezierCurve: root.animationCurves.emphasized
            property int velocity: 650
            property Component numberAnimation: Component {
                NumberAnimation {
                    alwaysRunToEnd: true
                    duration: root.animation.elementResize.duration
                    easing.type: root.animation.elementResize.type
                    easing.bezierCurve: root.animation.elementResize.bezierCurve
                }
            }
        }
        property QtObject clickBounce: QtObject {
            property int duration: Theme.motionMedium
            property int type: Easing.BezierSpline
            property list<real> bezierCurve: root.animationCurves.expressiveDefaultSpatial
            property int velocity: 850
            property Component numberAnimation: Component {
                NumberAnimation {
                    alwaysRunToEnd: true
                    duration: root.animation.clickBounce.duration
                    easing.type: root.animation.clickBounce.type
                    easing.bezierCurve: root.animation.clickBounce.bezierCurve
                }
            }
        }
        property QtObject scroll: QtObject {
            property int duration: Theme.motionSmall
            property int type: Easing.BezierSpline
            property list<real> bezierCurve: root.animationCurves.standardDecel
        }
        property QtObject menuDecel: QtObject {
            property int duration: Theme.motionMedium
            property int type: Easing.OutExpo
        }
    }

    // Geometry.
    //
    // These are sizes, not colours: upstream's components are laid out to
    // these numbers, and a bar that is 8 pixels tall because the token it
    // read did not exist is worse than a bar that is upstream's height. The
    // two that Zenith's own design system pins — the bar's height and the
    // body's type size — are the tokens, and the rest are upstream's.
    property QtObject sizes: QtObject {
        property real baseBarHeight: 40
        property real barHeight: baseBarHeight
        property real barCenterSideModuleWidth: 140
        property real barCenterSideModuleWidthShortened: 280
        property real barCenterSideModuleWidthHellaShortened: 190
        property real barShortenScreenWidthThreshold: 1200
        property real barHellaShortenScreenWidthThreshold: 1000
        property real elevationMargin: 10
        property real fabShadowRadius: 5
        property real fabHoveredShadowRadius: 7
        property real hyprlandGapsOut: 5
        property real mediaControlsWidth: 440
        property real mediaControlsHeight: 160
        property real notificationPopupWidth: 410
        property real osdWidth: 180
        property real searchWidthCollapsed: 210
        property real searchWidth: 360
        property real sidebarWidth: 460
        property real sidebarWidthExtended: 750
        property real baseVerticalBarWidth: 46
        property real verticalBarWidth: baseVerticalBarWidth
        property real wallpaperSelectorWidth: 1200
        property real wallpaperSelectorHeight: 690
        property real wallpaperSelectorItemMargins: 8
        property real wallpaperSelectorItemPadding: 6
    }

    // Panels are drawn with Zenith's own fill — an opaque surface with a
    // hairline — rather than by compositing a partially transparent panel
    // over the wallpaper. So nothing here is transparent, and the two
    // properties upstream's components read to ask "how transparent is this
    // surface" answer zero rather than a value derived from the wallpaper's
    // vibrancy.
    property real backgroundTransparency: 0
    property real contentTransparency: 0
}
