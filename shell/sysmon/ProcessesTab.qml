import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io

Item {
    id: tab
    property var processes: []
    property var app
    property string filterText: ""
    property int selectedPID: -1
    property string statusMsg: ""

    Process {
        id: killProc
        property int targetPid: -1
        command: ["asura-sysmon", "kill", String(targetPid)]
        onExited: (code) => {
            if (code === 0) {
                tab.statusMsg = "Process " + killProc.targetPid + " terminated";
                tab.selectedPID = -1;
                // Trigger refresh
                procRefreshProc.running = true;
            } else {
                tab.statusMsg = "Failed to terminate PID " + killProc.targetPid;
            }
        }
    }

    Process {
        id: procRefreshProc
        command: ["asura-sysmon", "proc", "--limit", "120"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var parsed = JSON.parse(text);
                    if (parsed && parsed.processes) {
                        tab.app.processList = parsed.processes;
                    }
                } catch(e) {}
            }
        }
    }

    Column {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        // Filter & Action Toolbar
        Item {
            width: parent.width
            height: 36

            Row {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                height: 36
                spacing: 12

                Rectangle {
                    width: 260
                    height: 36
                    radius: 6
                    color: "#13151c"
                    border.color: searchInput.activeFocus ? "#4b5563" : "#222530"
                    border.width: 1

                    Row {
                        anchors.fill: parent
                        anchors.leftMargin: 10
                        anchors.rightMargin: 10
                        spacing: 8

                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: "\u2315"
                            color: "#6b7280"
                            font.pixelSize: 14
                        }

                        TextInput {
                            id: searchInput
                            anchors.verticalCenter: parent.verticalCenter
                            width: parent.width - 30
                            color: "#ffffff"
                            font.family: "Inter, sans-serif"
                            font.pixelSize: 12
                            clip: true
                            onTextChanged: tab.filterText = text.toLowerCase().trim()

                            Text {
                                anchors.fill: parent
                                visible: !searchInput.text && !searchInput.activeFocus
                                text: "Search processes..."
                                color: "#4b5563"
                                font.family: "Inter, sans-serif"
                                font.pixelSize: 12
                            }
                        }
                    }
                }

                Rectangle {
                    width: 100
                    height: 36
                    radius: 6
                    color: tab.selectedPID > 0 ? (killMouse.containsMouse ? "#3b1515" : "#2a1212") : "#161820"
                    border.color: tab.selectedPID > 0 ? "#7f1d1d" : "#242836"
                    border.width: 1
                    opacity: tab.selectedPID > 0 ? 1.0 : 0.4

                    Row {
                        anchors.centerIn: parent
                        spacing: 6
                        Text {
                            text: "End Task"
                            color: tab.selectedPID > 0 ? "#f87171" : "#6b7280"
                            font.family: "Inter, sans-serif"
                            font.pixelSize: 12
                            font.weight: Font.SemiBold
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }

                    MouseArea {
                        id: killMouse
                        anchors.fill: parent
                        enabled: tab.selectedPID > 0
                        cursorShape: tab.selectedPID > 0 ? Qt.PointingHandCursor : Qt.ArrowCursor
                        hoverEnabled: true
                        onClicked: {
                            if (tab.selectedPID > 0) {
                                killProc.targetPid = tab.selectedPID;
                                killProc.running = true;
                            }
                        }
                    }
                }

                Text {
                    text: tab.statusMsg
                    color: "#9ca3af"
                    font.family: "JetBrainsMono Nerd Font, monospace"
                    font.pixelSize: 11
                    anchors.verticalCenter: parent.verticalCenter
                }
            }

            Text {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: (filteredModel.count) + " processes"
                color: "#6b7280"
                font.family: "JetBrainsMono Nerd Font, monospace"
                font.pixelSize: 11
            }
        }

        // Table Header
        Rectangle {
            width: parent.width
            height: 30
            color: "#151821"
            radius: 4

            Row {
                anchors.fill: parent
                anchors.leftMargin: 12
                anchors.rightMargin: 12

                Text { width: 80; text: "PID"; color: "#6b7280"; font.pixelSize: 11; font.family: "JetBrainsMono Nerd Font, monospace"; font.weight: Font.SemiBold; anchors.verticalCenter: parent.verticalCenter }
                Text { width: 220; text: "PROCESS NAME"; color: "#6b7280"; font.pixelSize: 11; font.family: "JetBrainsMono Nerd Font, monospace"; font.weight: Font.SemiBold; anchors.verticalCenter: parent.verticalCenter }
                Text { width: 120; text: "USER"; color: "#6b7280"; font.pixelSize: 11; font.family: "JetBrainsMono Nerd Font, monospace"; font.weight: Font.SemiBold; anchors.verticalCenter: parent.verticalCenter }
                Text { width: 110; text: "MEMORY"; color: "#6b7280"; font.pixelSize: 11; font.family: "JetBrainsMono Nerd Font, monospace"; font.weight: Font.SemiBold; anchors.verticalCenter: parent.verticalCenter }
                Text { width: 70; text: "THREADS"; color: "#6b7280"; font.pixelSize: 11; font.family: "JetBrainsMono Nerd Font, monospace"; font.weight: Font.SemiBold; anchors.verticalCenter: parent.verticalCenter }
                Text { width: 60; text: "STATE"; color: "#6b7280"; font.pixelSize: 11; font.family: "JetBrainsMono Nerd Font, monospace"; font.weight: Font.SemiBold; anchors.verticalCenter: parent.verticalCenter }
            }
        }

        // Process List
        ListView {
            id: procListView
            width: parent.width
            height: parent.height - 86
            clip: true
            spacing: 2
            boundsBehavior: Flickable.StopAtBounds

            model: ListModel {
                id: filteredModel
            }

            function recompute() {
                filteredModel.clear();
                var list = tab.processes || [];
                var f = tab.filterText;
                for (var i = 0; i < list.length; i++) {
                    var p = list[i];
                    if (!f || p.name.toLowerCase().indexOf(f) >= 0 || String(p.pid).indexOf(f) >= 0 || p.user.toLowerCase().indexOf(f) >= 0) {
                        filteredModel.append({
                            pid: p.pid,
                            name: p.name,
                            user: p.user,
                            memoryBytes: p.memoryBytes,
                            threads: p.threads,
                            state: p.state
                        });
                    }
                }
            }

            Component.onCompleted: recompute()

            Connections {
                target: tab
                function onProcessesChanged() { procListView.recompute(); }
                function onFilterTextChanged() { procListView.recompute(); }
            }

            delegate: Rectangle {
                required property int pid
                required property string name
                required property string user
                required property real memoryBytes
                required property int threads
                required property string state

                width: procListView.width
                height: 32
                radius: 4
                color: tab.selectedPID === pid ? "#222736" : (rowMouse.containsMouse ? "#161922" : "transparent")
                border.color: tab.selectedPID === pid ? "#3b4255" : "transparent"
                border.width: 1

                Row {
                    anchors.fill: parent
                    anchors.leftMargin: 12
                    anchors.rightMargin: 12

                    Text { width: 80; text: String(pid); color: "#9ca3af"; font.pixelSize: 11; font.family: "JetBrainsMono Nerd Font, monospace"; anchors.verticalCenter: parent.verticalCenter }
                    Text { width: 220; text: name; color: "#ffffff"; font.pixelSize: 12; font.family: "Inter, sans-serif"; font.weight: Font.Medium; elide: Text.ElideRight; anchors.verticalCenter: parent.verticalCenter }
                    Text { width: 120; text: user; color: "#9ca3af"; font.pixelSize: 11; font.family: "Inter, sans-serif"; anchors.verticalCenter: parent.verticalCenter }
                    Text { width: 110; text: tab.app.fmtBytes(memoryBytes); color: "#e5e7eb"; font.pixelSize: 11; font.family: "JetBrainsMono Nerd Font, monospace"; anchors.verticalCenter: parent.verticalCenter }
                    Text { width: 70; text: String(threads); color: "#9ca3af"; font.pixelSize: 11; font.family: "JetBrainsMono Nerd Font, monospace"; anchors.verticalCenter: parent.verticalCenter }
                    Text { width: 60; text: state; color: state === "R" ? "#22c55e" : "#6b7280"; font.pixelSize: 11; font.family: "JetBrainsMono Nerd Font, monospace"; anchors.verticalCenter: parent.verticalCenter }
                }

                MouseArea {
                    id: rowMouse
                    anchors.fill: parent
                    hoverEnabled: true
                    onClicked: tab.selectedPID = pid
                }
            }
        }
    }
}
