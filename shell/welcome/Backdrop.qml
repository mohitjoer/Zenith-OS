import QtQuick
import qs.Ui
import qs.Ui.Singletons
import "Singletons"

// The ambient frame the whole walkthrough sits in: the 天 mark huge and faint on
// the left (no shipped art; the Ui kit's Watermark draws it), a black wash to
// seat it on the paper, an edge vignette that melts it into the window edge, and
// registration ticks. Content scrims live in Welcome.qml so each step can tune
// how much of the backdrop shows through.
Item {
    id: backdrop

    Watermark {
        anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
        width: parent.width * 0.62
        text: "\u5929"
        strength: 0.08
    }

    // seat the plate on the paper: a faint pure-black wash over all.
    Rectangle {
        anchors.fill: parent
        color: Tokens.paper
        opacity: 0.18
    }

    // edge vignette: darken the corners so content holds the eye and the plate
    // melts into the paper at the window edge.
    Canvas {
        anchors.fill: parent
        onWidthChanged: requestPaint()
        onHeightChanged: requestPaint()
        onPaint: {
            let ctx = getContext("2d");
            ctx.clearRect(0, 0, width, height);
            let r = Math.max(width, height) * 0.86;
            let g = ctx.createRadialGradient(width * 0.5, height * 0.46, r * 0.34, width * 0.5, height * 0.46, r);
            g.addColorStop(0, "rgba(0,0,0,0)");
            g.addColorStop(1, "rgba(0,0,0,0.42)");
            ctx.fillStyle = g;
            ctx.fillRect(0, 0, width, height);
        }
    }

    // registration ticks bracketing the sheet, the house corner vocabulary.
    Ticks {
        anchors.margins: 16
        arm: 22
        color: Tokens.line
    }
}
