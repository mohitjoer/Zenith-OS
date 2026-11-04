pragma ComponentBehavior: Bound

// The launcher.
//
// launcher-apps empty means every launchable application on the machine, not
// none. An empty list is a policy decision, and the common one is not to make
// it — a launcher that shows nothing because nothing was configured is the worst
// of both readings.

import Quickshell
import Quickshell.Io
import Quickshell.Widgets
import QtQuick

Item {
    id: launcher

    implicitWidth: 640
    implicitHeight: 480

    // The card the launcher is drawn on. A popup with no background of its own
    // is a white rectangle, which is the one surface in the shell that cannot
    // be mistaken for anything else and is always wrong.
    Rectangle {
        anchors.fill: parent
        radius: Theme.radiusLarge
        color: Theme.panelFill
        border.width: Theme.hairlineWidth
        border.color: Theme.hairline
    }

    // A Grid rather than a Flow, because a grid has columns and launcher-columns
    // is a setting about columns. A Flow with a fixed width gives you however
    // many fit, which is not what the key says and makes the key a lie.
    Grid {
        id: grid

        anchors.centerIn: parent
        // columns: 0 in the schema means "choose", and an unset column count on
        // a Grid means one column, which is the worst possible reading of a key
        // that exists to be set.
        columns: Config.launcherColumns > 0 ? Config.launcherColumns : 5
        spacing: 16

        Repeater {
            model: launcher.apps

            delegate: Item {
                required property var modelData

                implicitWidth: 64
                implicitHeight: 64

                IconImage {
                    anchors.fill: parent
                    // The name is the icon name, not a path to one. Building a
                    // path from a desktop file ID means a settings value decides
                    // what file the shell reads.
                    source: modelData.icon
                }

                // The whole icon is the button. A launcher whose applications
                // are launched by clicking a 4-pixel label is a launcher
                // nobody can use.
                MouseArea {
                    anchors.fill: parent
                    hoverEnabled: true
                    onClicked: launcher.launch(modelData)
                }
            }
        }
    }

    // The applications, resolved from the catalogue as it arrives.
    //
    // A binding rather than a snapshot, because the catalogue is read by a
    // process and a process finishes after the launcher is built: a launcher
    // that took its list once, at construction, would be empty until it was
    // reopened. The catalogue is read once, not on a timer, so this is not a
    // shell that is never idle.
    property var apps: launcherApps()

    // The applications the launcher offers.
    //
    // The list on the machine, unless the user has named a list of their own:
    // launcher-apps is a list of desktop file ids, and a list of ids is a
    // choice, not a catalogue.
    function launcherApps() {
        if (Config.launcherApps.length > 0) {
            return Config.launcherApps.map(function (id) {
                var found = findApplication(id);
                if (found) {
                    return found;
                }
                // An id the machine does not have is an id that is still
                // worth offering: the dock's favourites are ids too, and an
                // application installed later should appear without the
                // setting being rewritten.
                return { id: id, name: id, icon: id, exec: id, args: [], terminal: false };
            })
        }
        return Apps.applications;
    }

    function findApplication(id: string) {
        for (var i = 0; i < Apps.applications.length; i++) {
            if (Apps.applications[i].id === id) {
                return Apps.applications[i];
            }
        }
        return null;
    }

    // Run one.
    //
    // An application that wants a terminal gets one, through the desktop's
    // own helper: which terminal that is, is the user's decision and the
    // helper's business. An application run without a terminal opens a window
    // and then exits, which is an application that looks broken.
    function launch(app: var) {
        var command = [app.exec].concat(app.args || []);
        if (app.terminal) {
            Quickshell.execDetached(["xdg-terminal-exec"].concat(command));
            return;
        }
        Quickshell.execDetached(command);
    }
}
