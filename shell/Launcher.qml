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

    // A Grid rather than a Flow, because a grid has columns and launcher-columns
    // is a setting about columns. A Flow with a fixed width gives you however
    // many fit, which is not what the key says and makes the key a lie.
    Grid {
        id: grid

        anchors.centerIn: parent
        // columns: 0 in the schema means "choose", and an unset column count on
        // a Grid means one column, which is the worst possible reading of a key
        // that exists to be set.
        columns: config.launcherColumns > 0 ? config.launcherColumns : 5
        spacing: 16

        Repeater {
            model: launcher.apps

            delegate: IconImage {
                required property var modelData

                implicitSize: 64
                // The name is the icon name, not a path to one. Building a
                // path from a desktop file ID means a settings value decides
                // what file the shell reads.
                source: modelData.icon
                colorize: true
            }
        }
    }

    // Resolved when the launcher opens rather than bound to a process that is
    // always running. Scanning every .desktop file is a filesystem walk, and a
    // filesystem walk on a timer is a shell that is never idle — which matters
    // more than usual here, because this shell owns the idle policy.
    property var apps: []

    function refresh() {
        apps = launcherApps()
    }

    // An empty setting means everything on the machine, and that is the default
    // rather than a fallback: a launcher whose default is empty would ship
    // showing nothing, which is what the empty-means-nothing reading produces.
    function launcherApps() {
        if (config.launcherApps.length > 0) {
            return config.launcherApps.map(function (id) {
                return { id: id, icon: id }
            })
        }
        return Quickshell.apps.map(function (app) {
            return {
                id: app.id,
                icon: app.icon,
                name: app.name,
                run: function () { app.launch() }
            }
        })
    }

    Component.onCompleted: refresh()
}
