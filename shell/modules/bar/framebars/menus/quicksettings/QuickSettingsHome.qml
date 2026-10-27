pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import "../../.." as Pill
import qs.Services
import "../../../../../components"
import ".." as Menus
import qs.Ui.Singletons

Item {
    id: root

    property real s: 1
    property bool open: false
    property var navigate: null
    property var closePanel: null

    function awakeFor() {
        const since = Flags.keepAwakeSince;
        if (!since || since <= 0)
            return "";
        const mins = Math.floor((Date.now() - since) / 60000);
        if (mins < 1)
            return "";
        if (mins < 60)
            return " " + I18n.tr("for %1m").arg(mins);
        if (mins < 1440)
            return " " + I18n.tr("for %1h").arg(Math.floor(mins / 60));
        return " " + I18n.tr("for %1d").arg(Math.floor(mins / 1440));
    }

    SystemClock {
        id: clock
        precision: SystemClock.Minutes
        enabled: root.open
    }

    Rectangle {
        anchors.fill: parent
        color: Theme.surface
    }

    Item {
        id: header
        anchors.top: parent.top
        anchors.topMargin: 12
        anchors.left: parent.left
        anchors.leftMargin: 12
        anchors.right: parent.right
        anchors.rightMargin: 12
        height: headerColumn.implicitHeight

        Column {
            id: headerColumn
            width: parent.width
            spacing: 6

            // ── Top Title Row: Minimalist Eyebrow ──
            Item {
                width: parent.width
                height: 20

                Row {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 6

                    MaterialIcon {
                        anchors.verticalCenter: parent.verticalCenter
                        font.pixelSize: 13
                        text: "tune"
                        color: Theme.inkOn(Theme.effectiveSurface, Theme.primary, 3.0)
                    }

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("QUICK SETTINGS")
                        color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                        font.family: Theme.fontPrimary
                        font.pixelSize: 10
                        font.weight: Font.Bold
                        font.letterSpacing: 2
                    }
                }

                Row {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 5

                    Rectangle {
                        width: 4
                        height: 4
                        radius: 2
                        anchors.verticalCenter: parent.verticalCenter
                        color: Theme.primary
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("CONTROL")
                        color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                        font.family: Theme.fontPrimary
                        font.pixelSize: 9
                        font.weight: Font.Bold
                        font.letterSpacing: 1.5
                    }
                }
            }

            // ── Floating Clock & Session Buttons (No Heavy Outer Box!) ──
            Item {
                width: parent.width
                height: 44

                Text {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: Qt.formatTime(clock.date, "HH:mm")
                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                    font.family: Theme.fontPrimary
                    font.pixelSize: 36
                    font.weight: Font.Bold
                }

                // Session buttons floating cleanly with no heavy enclosure
                Row {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 4

                    Menus.QsIconButton {
                        icon: "logout"
                        tip: I18n.tr("Log out")
                        tipBelow: true
                        onClicked: SessionActions.run("logout")
                    }
                    Menus.QsIconButton {
                        icon: "lock"
                        tip: I18n.tr("Lock")
                        tipBelow: true
                        onClicked: {
                            Quickshell.execDetached(["zenith-shell", "lock"]);
                            if (root.closePanel)
                                root.closePanel();
                        }
                    }
                    Menus.QsHoldButton {
                        icon: "restart_alt"
                        tip: I18n.tr("Click to reboot")
                        tipBelow: true
                        tipAlign: "right"
                        onActivated: Quickshell.execDetached(["systemctl", "reboot"])
                    }
                    Menus.QsHoldButton {
                        icon: "power_settings_new"
                        tip: I18n.tr("Click to shut down")
                        tipBelow: true
                        tipAlign: "right"
                        onActivated: Quickshell.execDetached(["systemctl", "poweroff"])
                    }
                }
            }

            // ── Date & Battery Indicator (Clean Inline, No Boxes) ──
            Item {
                width: parent.width
                height: 20

                Text {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: Qt.locale().toString(clock.date, "dddd") + " • " + Qt.formatDate(clock.date, "MMM d, yyyy")
                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                    font.family: Theme.fontPrimary
                    font.pixelSize: Theme.fontSm
                    font.weight: Font.Medium
                }

                Row {
                    visible: Battery.present
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 4

                    MaterialIcon {
                        anchors.verticalCenter: parent.verticalCenter
                        font.pixelSize: 13
                        text: Battery.charging ? "bolt" : "battery_full"
                        color: Theme.inkOn(Theme.effectiveSurface, Battery.charging ? Theme.primary : Theme.onSurfaceVariant, 3.0)
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: Battery.pct + "%"
                        color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                        font.family: Theme.fontPrimary
                        font.pixelSize: Theme.fontSm - 1
                        font.weight: Font.Bold
                    }
                }
            }
        }
    }

    Flickable {
        id: homeFlick
        anchors.top: header.bottom
        anchors.topMargin: 12
        anchors.bottom: powerDock.top
        anchors.bottomMargin: 8
        anchors.left: parent.left
        anchors.leftMargin: 12
        anchors.right: parent.right
        anchors.rightMargin: 12
        contentWidth: width
        contentHeight: controls.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        interactive: contentHeight > height

        Column {
            id: controls
            width: parent.width
            spacing: 12

            Menus.QsSection { width: parent.width; label: I18n.tr("Connect") }
            Grid {
                id: tileGrid
                width: parent.width
                columns: 2
                columnSpacing: 8
                rowSpacing: 8
                readonly property real tileWidth: (width - columnSpacing) / 2

                Menus.QsTile {
                    width: tileGrid.tileWidth
                    icon: Network.kind === "ethernet" ? "lan" : "wifi"
                    label: I18n.tr("Wi-Fi")
                    sub: !Toggles.wifiOn ? I18n.tr("Off") : Network.activeSsid !== "" ? Network.activeSsid : I18n.tr("On")
                    on: Toggles.wifiOn
                    hasPage: true
                    pageTip: I18n.tr("Wi-Fi networks")
                    onToggled: Toggles.toggleWifi()
                    onPageRequested: if (root.navigate) root.navigate("network")
                }
                Menus.QsTile {
                    width: tileGrid.tileWidth
                    icon: "bluetooth"
                    label: I18n.tr("Bluetooth")
                    sub: Toggles.btOn ? I18n.tr("On") : I18n.tr("Off")
                    on: Toggles.btOn
                    hasPage: true
                    pageTip: I18n.tr("Bluetooth devices")
                    onToggled: Toggles.toggleBt()
                    onPageRequested: if (root.navigate) root.navigate("bluetooth")
                }
                Menus.QsTile {
                    width: tileGrid.tileWidth
                    icon: "flight"
                    label: I18n.tr("Airplane")
                    sub: Toggles.wifiOn ? I18n.tr("Off") : I18n.tr("On")
                    on: !Toggles.wifiOn
                    onToggled: Toggles.toggleWifi()
                }
                Menus.QsTile {
                    visible: Wm.caps.nightLight === true
                    width: tileGrid.tileWidth
                    icon: "bedtime"
                    label: I18n.tr("Night light")
                    sub: Toggles.nightOn ? I18n.tr("On") : I18n.tr("Off")
                    on: Toggles.nightOn
                    onToggled: Toggles.toggleNight()
                }
                Menus.QsTile {
                    width: tileGrid.tileWidth
                    icon: "coffee"
                    label: I18n.tr("Keep awake")
                    sub: Toggles.keepAwake ? I18n.tr("On") + root.awakeFor() : I18n.tr("Off")
                    on: Toggles.keepAwake
                    onToggled: Toggles.toggleCaffeine()
                }
                Menus.QsTile {
                    width: tileGrid.tileWidth
                    icon: "do_not_disturb_on"
                    label: I18n.tr("Do not disturb")
                    sub: Toggles.dnd ? I18n.tr("On") : I18n.tr("Off")
                    on: Toggles.dnd
                    onToggled: Toggles.toggleDnd()
                }
                Menus.QsTile {
                    visible: Wm.caps.liveConfigEval === true || Toggles.gameMode
                    width: tileGrid.tileWidth
                    icon: "sports_esports"
                    label: I18n.tr("Gaming")
                    available: Toggles.gameMode || Battery.onAc
                    sub: !available ? I18n.tr("Needs AC")
                       : Toggles.gameMode ? I18n.tr("On") : I18n.tr("Off")
                    on: Toggles.gameMode
                    onToggled: Toggles.toggleGame()
                }
            }

            Menus.QsSection { width: parent.width; label: I18n.tr("Sound & display") }
            Column {
                width: parent.width
                spacing: 4

                Menus.QsSlider {
                    width: parent.width
                    icon: "speaker"
                    lit: root.open
                    value: Audio.sink ? Audio.sink.audio.volume : 0
                    muted: Audio.sink ? Audio.sink.audio.muted : false
                    valueLabel: !Audio.sink ? "" : (Audio.sink.audio.muted ? I18n.tr("off") : Math.round(Audio.sink.audio.volume * 100) + "%")
                    peakNode: Audio.sink
                    peakEnabled: root.open && !!Audio.sink
                    hasPage: true
                    onMoved: value => { if (Audio.sink) Audio.sink.audio.volume = value; }
                    onIconTapped: { if (Audio.sink) Audio.sink.audio.muted = !Audio.sink.audio.muted; }
                    onPageRequested: if (root.navigate) root.navigate("audio-out")
                }
                Menus.QsSlider {
                    width: parent.width
                    icon: "mic"
                    lit: root.open
                    value: Audio.source && Audio.source.audio ? Audio.source.audio.volume : 0
                    muted: Audio.source && Audio.source.audio ? Audio.source.audio.muted : false
                    valueLabel: !Audio.source || !Audio.source.audio ? "" : (Audio.source.audio.muted ? I18n.tr("off") : Math.round(Audio.source.audio.volume * 100) + "%")
                    peakNode: Audio.source
                    peakEnabled: root.open && !!Audio.source
                    hasPage: true
                    onMoved: value => { if (Audio.source && Audio.source.audio) Audio.source.audio.volume = value; }
                    onIconTapped: { if (Audio.source && Audio.source.audio) Audio.source.audio.muted = !Audio.source.audio.muted; }
                    onPageRequested: if (root.navigate) root.navigate("audio-in")
                }
                Pill.BrightnessControl {
                    width: parent.width
                    s: 1
                    active: root.open
                }
            }

            Menus.MediaHero {
                width: parent.width
                active: root.open
            }

            Menus.QsSection { width: parent.width; label: I18n.tr("Calendar") }
            Menus.QsCalendarEmbed {
                width: parent.width
                s: 1
                open: root.open
            }

            Menus.QsSection { width: parent.width; label: I18n.tr("System") }
            SysMonitor {
                width: parent.width
                s: 1
                active: root.open
            }
        }
    }

    Column {
        id: powerDock
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 10
        anchors.left: parent.left
        anchors.leftMargin: 12
        anchors.right: parent.right
        anchors.rightMargin: 12
        spacing: 6

        Rectangle {
            width: parent.width
            height: 1
            color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.10)
        }
        Menus.QsSection {
            visible: PowerProfiles.available
            width: parent.width
            label: I18n.tr("Power")
        }
        Menus.QsSeg {
            visible: PowerProfiles.available
            width: parent.width
            current: PowerProfiles.profile
            options: PowerProfiles.profiles.map(profile => ({
                id: profile,
                label: profile === "power-saver" ? I18n.tr("Saver") : profile.charAt(0).toUpperCase() + profile.slice(1)
            }))
            onChose: profile => PowerProfiles.setProfile(profile)
        }
    }
}
