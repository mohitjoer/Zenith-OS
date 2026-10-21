import QtQuick
import Quickshell
import qs as Root

// The vendored components' settings, read from Zenith's settings store.
//
// This is an overlay. Upstream's version of this file reads a JSON file of its
// own — it creates `~/.config/illogical-impulse/config.json` on first use and
// writes back into it whenever a component changes an option. Zenith already
// has a settings store, GSettings, and exactly one writer, `zenith config
// set`. Two stores would mean two answers to "what is the bar's height", and
// the one that wins would depend on which surface you asked.
//
// So this file exposes upstream's shape — `Config.options.bar.height`,
// `Config.options.appearance.fonts.main` — and every value comes from
// `zenith config export`. It has no writer, because there is nothing here
// that owns a setting: a vendored component that wants to change one asks the
// desktop to, and the desktop writes GSettings.
//
// What is here is what a component would read to draw. Upstream's file has
// three hundred and seventy-nine options, most of them for surfaces Zenith
// does not adopt — an AI assistant's model, a booru viewer's tags, a
// cheatsheet's keybindings. Those are not missing by oversight: an option for
// a surface that does not exist is a setting nobody can act on. When one of
// those surfaces is adopted, its options are added here and bound to the
// settings the desktop keeps.
pragma Singleton
pragma ComponentBehavior: Bound

Singleton {
    id: root

    property QtObject options: QtObject {
        property QtObject bar: QtObject {
            // The bar's own height, because the panel reserves exactly this
            // much space and a bar whose settings and geometry disagree is a
            // bar with a gap under it.
            property int height: Root.Config.barHeight
            property bool bottom: Root.Config.barPosition === Root.Config.edgeBottom
            property bool vertical: Root.Config.barPosition === Root.Config.edgeLeft
                || Root.Config.barPosition === Root.Config.edgeRight
            // Upstream calls an edged bar "attached" (0) and a floating one
            // "corned" (1). Zenith's bar is attached: it is the top edge of
            // the screen, not a pill hovering above it.
            property int cornerStyle: 0
            // A verbosity switch that upstream's bar uses to decide whether
            // its centre module shows a clock, a date, or both. Zenith's bar
            // shows both, so it is verbose.
            property bool verbose: true
            property bool showBackground: true
            // Every screen. An empty list means "all of them" upstream, and
            // Zenith has no per-screen bar.
            property var screenList: []
            property QtObject autoHide: QtObject {
                property bool enable: false
                property QtObject showWhenPressingSuper: QtObject {
                    property bool enable: false
                    property int delay: 100
                }
            }
            property QtObject workspaces: QtObject {
                property bool showAppIcons: true
                property bool alwaysShowNumbers: false
            }
        }

        property QtObject dock: QtObject {
            // The dock's pinned applications, which are the desktop's
            // favourites because a dock is what a favourite is.
            property var pinnedApps: Root.Config.favorites
            property bool enabled: true
            property int iconSize: 48
        }

        property QtObject appearance: QtObject {
            property QtObject fonts: QtObject {
                property string main: Root.Theme.fontFamily
                property string numbers: Root.Theme.fontMonoFamily
                property string title: Root.Theme.fontFamily
                property string monospace: Root.Theme.fontMonoFamily
                property string reading: Root.Theme.fontFamily
                property string expressive: Root.Theme.fontFamily
                property string iconNerd: Root.Theme.fontFamily
            }
            property bool extraBackgroundTint: false
            // Panels are opaque. Upstream can composite them over the
            // wallpaper; Zenith draws a surface with a hairline, so there is
            // no transparency to configure.
            property QtObject transparency: QtObject {
                property bool enable: false
                property bool automatic: false
                property real backgroundTransparency: 0
                property real contentTransparency: 0
            }
        }

        property QtObject background: QtObject {
            property string wallpaperPath: Root.Config.wallpaper
        }

        // Scrolling. These are read by the kit's own flickable, so they exist
        // because a component is in use rather than because a surface might be
        // adopted one day. They are constants: the kit multiplies the wheel
        // delta by them to make a mouse wheel feel the same as a touchpad, and
        // the numbers are its own.
        property QtObject interactions: QtObject {
            property QtObject scrolling: QtObject {
                property real touchpadScrollFactor: 100
                property real mouseScrollFactor: 50
                property real mouseScrollDeltaThreshold: 120
                property bool fasterTouchpadScroll: false
            }
        }

        // Upstream's components read this to know whether their settings file
        // had loaded. Zenith's settings are a GSettings read that is either
        // there or is not, and the defaults are the schema's, so there is no
        // half-loaded state to report.
        property bool ready: true
    }
}
