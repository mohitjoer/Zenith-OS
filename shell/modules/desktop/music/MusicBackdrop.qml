import QtQuick
import Quickshell.Widgets

// A looping visual backdrop for the now-playing surfaces: the track's Spotify
// Canvas, a chosen video/gif, or a still, whichever the widget hands it. Clips
// already play muted through QtMultimedia in the upstream design, but the
// QtMultimedia module is absent here, so the video path is trimmed (the same
// trim as modules/wallpaper/Backdrop.qml) and a .mp4/.webm/... source renders
// nothing; gifs run through AnimatedImage and stills through Image. The
// `source`/`active`/`radius` API is unchanged, so callers keep binding clip
// paths and they simply fall back to the still underneath. Everything fills by
// aspect-crop and clips to the given radius, and playback is gated on `active`
// so a hidden or idle surface spins no decoder.
Item {
    id: root

    property string source: ""     // path/url: .mp4/.webm/.mkv/.mov/.m4v, .gif, or an image
    property bool active: true      // play gate (surface visible)
    property real radius: 0

    readonly property string lc: root.source.toLowerCase()
    readonly property bool hasSource: root.source.length > 0
    readonly property bool isVideo: /\.(mp4|webm|mkv|mov|m4v|avi)(\?|$)/.test(root.lc)
    readonly property bool isGif: /\.gif(\?|$)/.test(root.lc)

    // a bare filesystem path becomes a file URL; http(s)/file/qrc pass through.
    readonly property url srcUrl: !root.hasSource ? ""
        : (/^(https?|file|qrc):/.test(root.source) ? root.source : "file://" + root.source)

    ClippingRectangle {
        anchors.fill: parent
        radius: root.radius
        color: "transparent"

        Loader {
            anchors.fill: parent
            active: root.isVideo && root.hasSource
            sourceComponent: videoComp
        }
        Component {
            id: videoComp
            // QtMultimedia-unavailable degrade: in-shell clip playback is
            // trimmed (MediaPlayer/VideoOutput removed; same trim as
            // modules/wallpaper/Backdrop.qml), so a video source draws nothing
            // here — use a .gif or a still for a live backdrop.
            Item {
                anchors.fill: parent
            }
        }

        AnimatedImage {
            anchors.fill: parent
            visible: root.isGif && root.hasSource
            source: root.isGif ? root.srcUrl : ""
            playing: root.active && visible
            cache: true
            smooth: true
            asynchronous: true
            fillMode: Image.PreserveAspectCrop
        }

        Image {
            anchors.fill: parent
            visible: root.hasSource && !root.isVideo && !root.isGif
            source: (root.hasSource && !root.isVideo && !root.isGif) ? root.srcUrl : ""
            asynchronous: true
            cache: true
            smooth: true
            mipmap: true
            fillMode: Image.PreserveAspectCrop
        }
    }
}
