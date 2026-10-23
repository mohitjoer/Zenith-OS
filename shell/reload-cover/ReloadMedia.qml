import QtQuick
import Quickshell
import qs.Ui.Singletons

// What the curtain shows: the Zenith mark by default, or the still, animated
// image or muted video the user chose on the Hub's Reload cover page.
//
// This file is also the Hub's live preview of that page, loaded into a frame
// there, so its three inputs are its whole interface and must keep their names:
//
//   descriptor    { path, name, kind, bytes, enabled } as brand.json stores it
//   active        decoders run only while this is true
//   forceDefault  show the mark whatever the descriptor says (the failed phase)
//
// A custom picture that cannot be drawn falls back to the mark rather than to
// an empty curtain: the cover exists to hide a restart, and a blank black screen
// with no mark on it reads as a crash.
Item {
    id: root

    property var descriptor: ({ path: "", name: "", kind: "default", bytes: 0, enabled: true })
    property bool active: true
    property bool forceDefault: false
    onMediaPathChanged: {
        orientationKnown = false
        imageTall = false
    }

    readonly property string mediaPath: descriptor && typeof descriptor.path === "string" ? descriptor.path : ""
    readonly property string mediaKind: descriptor && typeof descriptor.kind === "string" ? descriptor.kind : "default"
    readonly property bool customEnabled: !descriptor || descriptor.enabled !== false
    readonly property bool wantsVideo: customEnabled && mediaPath !== "" && mediaKind === "video"
    readonly property bool wantsImage: customEnabled && mediaPath !== "" && (mediaKind === "image" || mediaKind === "animated")
    // brand.json stores an absolute path today, but a hand-edited config may
    // carry a ~ (the shell expands it for the brand mark too); a bare
    // "file://~/..." resolves to nothing and silently falls back to the default
    // cover, so expand it here before building the URL.
    readonly property string homeDir: Quickshell.env("HOME") || ""
    readonly property string resolvedPath: mediaPath === "~"
        ? root.homeDir
        : (mediaPath.indexOf("~/") === 0 ? root.homeDir + mediaPath.substring(1) : mediaPath)
    readonly property string imageSource: resolvedPath.indexOf("://") >= 0 ? resolvedPath : "file://" + resolvedPath
    readonly property size orientationProbeSize: Qt.size(64, 64)
    property bool orientationKnown: false
    property bool imageTall: false
    readonly property bool probeReady: imageProbeLoader.status === Loader.Ready
        && imageProbeLoader.item
        && imageProbeLoader.item.status === Image.Ready
    readonly property bool probeFailed: imageProbeLoader.status === Loader.Ready
        && imageProbeLoader.item
        && imageProbeLoader.item.status === Image.Error
    readonly property bool probeReleased: wantsImage && orientationKnown
        && imageProbeLoader.status === Loader.Null
    readonly property bool imageReady: wantsImage
        && customImageLoader.status === Loader.Ready
        && customImageLoader.item
        && customImageLoader.item.status === Image.Ready
    readonly property bool videoReady: videoLoader.status === Loader.Ready && videoLoader.item && videoLoader.item.ready
    readonly property bool customReady: customEnabled && !forceDefault && (imageReady || videoReady)
    readonly property bool mediaError: wantsImage
        ? probeFailed
            || (orientationKnown && (customImageLoader.status === Loader.Error
                || (customImageLoader.status === Loader.Ready
                    && customImageLoader.item
                    && customImageLoader.item.status === Image.Error)))
        : (wantsVideo && (videoLoader.status === Loader.Error
            || (videoLoader.status === Loader.Ready && videoLoader.item && videoLoader.item.failed)))
    readonly property string mediaErrorText: wantsVideo && videoLoader.status === Loader.Ready && videoLoader.item
        ? videoLoader.item.errorText
        : ""
    readonly property bool showingDefault: forceDefault || !customReady

    // The mark is square, so it is sized by the shorter reading of the frame: a
    // fifth of its height, capped so a very tall screen does not get a banner.
    // The caption in ReloadCover.qml hangs off defaultLogoHeight, so the two
    // dimensions stay separate names even though they are equal.
    readonly property real defaultLogoHeight: Math.min(root.height * 0.2, 200)
    readonly property real defaultLogoWidth: root.defaultLogoHeight

    // Where the mark is found, in the order it is looked for: the packaged
    // branding directory, then the checkout's `public/` for a tree run from
    // source, then lettering. The SVG is the one copy of the mark, so it is not
    // duplicated into this directory where it could stop matching the original.
    readonly property var markSources: [
        "file:///usr/share/zenith/branding/zenith-logo-on-dark.svg",
        Qt.resolvedUrl("../../public/zenith-logo-on-dark.svg")
    ]
    property int markTry: 0
    readonly property bool markLettering: root.markTry >= root.markSources.length

    Image {
        id: defaultMark
        anchors.centerIn: parent
        visible: root.showingDefault && !root.markLettering
        source: root.markLettering ? "" : root.markSources[root.markTry]
        sourceSize.width: Math.round(root.defaultLogoWidth)
        sourceSize.height: Math.round(root.defaultLogoHeight)
        fillMode: Image.PreserveAspectFit
        width: root.defaultLogoWidth
        height: root.defaultLogoHeight
        onStatusChanged: if (status === Image.Error) root.markTry += 1
    }

    // Lettering is the last resort, for a machine with no branding directory
    // and no checkout. It names the product in Latin letters rather than with
    // the 天 glyph, because a glyph needs a CJK font and this is the path taken
    // when the machine is short of files.
    Text {
        anchors.centerIn: parent
        visible: root.showingDefault && root.markLettering
        text: "ZENITH"
        color: Tokens.keycapOnDark
        font.family: Tokens.ui
        font.weight: Font.DemiBold
        font.pixelSize: Math.max(12, Math.round(root.defaultLogoHeight * 0.4))
        font.letterSpacing: Math.round(root.defaultLogoHeight * 0.12)
    }

    Component {
        id: orientationProbe
        Image {
            objectName: "imageProbe"
            source: root.imageSource
            fillMode: Image.PreserveAspectFit
            sourceSize: root.orientationProbeSize
            asynchronous: true
            cache: false
            onStatusChanged: {
                if (status === Image.Ready) {
                    root.imageTall = implicitHeight > implicitWidth
                    root.orientationKnown = true
                }
            }
        }
    }

    Loader {
        id: imageProbeLoader
        objectName: "imageProbeLoader"
        active: root.wantsImage && !root.orientationKnown && !root.forceDefault
        sourceComponent: orientationProbe
    }

    Component {
        id: wideImage
        AnimatedImage {
            objectName: "customImage"
            anchors.fill: parent
            source: root.imageSource
            sourceSize.width: Math.max(1, Math.ceil(width))
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            cache: false
            playing: root.active && root.imageReady && !root.forceDefault && status === Image.Ready
        }
    }

    Component {
        id: tallImage
        AnimatedImage {
            objectName: "customImage"
            anchors.fill: parent
            source: root.imageSource
            sourceSize.height: Math.max(1, Math.ceil(height))
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            cache: false
            playing: root.active && root.imageReady && !root.forceDefault && status === Image.Ready
        }
    }

    Loader {
        id: customImageLoader
        anchors.fill: parent
        active: root.wantsImage && root.orientationKnown && !root.forceDefault
        visible: root.imageReady && !root.forceDefault
        sourceComponent: root.imageTall ? tallImage : wideImage
    }

    Loader {
        id: videoLoader
        anchors.fill: parent
        active: root.active && root.wantsVideo && !root.forceDefault
        source: active ? Qt.resolvedUrl("ReloadVideo.qml") : ""
        onLoaded: {
            if (status === Loader.Ready && item) {
                item.path = Qt.binding(() => root.resolvedPath)
                item.active = Qt.binding(() => root.active && !root.forceDefault)
            }
        }
    }
}
