pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import qs.Ui
import qs.Ui.Singletons
import "../Singletons"

// Profile / System Specifications page.
// Shows system specs, Asura OS version, and live update status.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    property string hardwareModel: "Standard PC"
    Process {
        id: hwModelProc
        running: true
        command: ["sh", "-c", "cat /sys/class/dmi/id/product_name 2>/dev/null || cat /sys/devices/virtual/dmi/id/product_name 2>/dev/null || cat /etc/hostname 2>/dev/null || echo 'Standard PC'"]
        stdout: StdioCollector {
            onStreamFinished: {
                var s = text.trim();
                if (s.length > 0)
                    pg.hardwareModel = s;
            }
        }
    }

    // ── Head ────────────────────────────────────────────────────────────────
    Column {
        id: head
        anchors {
            left: parent.left; right: parent.right; top: parent.top
            leftMargin: Tokens.s6; rightMargin: Tokens.s6; topMargin: Tokens.s6
        }
        spacing: Tokens.s3

        Row {
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle {
                width: 16; height: 1; color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
            Image {
                width: 12; height: 12
                anchors.verticalCenter: parent.verticalCenter
                source: Qt.resolvedUrl("../../assets/asura-logo-on-dark.svg")
                sourceSize.width: 24; sourceSize.height: 24
                fillMode: Image.PreserveAspectFit
                smooth: true
            }
            Text {
                text: I18n.tr("OVERVIEW"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }

        Text {
            text: I18n.tr("System Profile"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }

        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Hardware specifications, OS version, and update status.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
    }

    // ── Content ─────────────────────────────────────────────────────────────
    Flickable {
        id: flick
        anchors {
            left: parent.left; right: parent.right
            top: head.bottom; bottom: parent.bottom
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
            topMargin: Tokens.s5; bottomMargin: Tokens.s6
        }
        contentWidth: width
        contentHeight: Math.max(col.height, height)
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        CardColumns {
            id: col
            width: flick.width - Tokens.s3
            spacing: Tokens.s5
            fillTo: flick.height
            minColumnWidth: 320
            maxColumns: 2

            // ── Card 1: ASURA OS ──
            SettingCard {
                width: col.colWidth
                title: I18n.tr("ASURA OS")
                collapsible: false

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: false
                    label: I18n.tr("Operating System")
                    value: "Asura OS"
                    desc: I18n.tr("Pure monochrome desktop OS")
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Version")
                    value: Updates.currentVersion.length > 0 ? Updates.currentVersion : "Beta 001"
                    desc: I18n.tr("Official release edition")
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Kernel")
                    value: SysInfo.sysKernel.length > 0 && SysInfo.sysKernel !== "-" ? SysInfo.sysKernel : "Linux"
                    desc: I18n.tr("Active Linux kernel")
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Architecture")
                    value: SysInfo.sysArch.length > 0 ? SysInfo.sysArch : "x86_64"
                    desc: I18n.tr("Hardware architecture")
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Compositor")
                    value: (SysInfo.sysWM.length > 0 && SysInfo.sysWM !== "-" ? SysInfo.sysWM : "Hyprland") + " (Wayland)"
                    desc: I18n.tr("Window manager and display compositor")
                }
            }

            // ── Card 2: SYSTEM UPDATES ──
            SettingCard {
                width: col.colWidth
                title: I18n.tr("SYSTEM UPDATES")
                collapsible: false

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: false
                    label: Updates.available ? I18n.tr("Update Available") : I18n.tr("System Up to Date")
                    value: Updates.available ? (Updates.behind > 0 ? I18n.tr("%1 Pending").arg(Updates.behind) : I18n.tr("Available")) : I18n.tr("Current")
                    desc: Updates.available
                        ? I18n.tr("A new version is available to install.")
                        : I18n.tr("All components are up to date. Checked %1").arg(Updates.checkedAgo)
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Update Actions")
                    desc: I18n.tr("Check repository for updates or review changelog")
                    controlWidth: 160

                    Row {
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Tokens.s2

                        Btn {
                            text: I18n.tr("CHECK")
                            compact: true
                            onAct: Updates.check()
                        }

                        Btn {
                            text: I18n.tr("VIEW")
                            primary: Updates.available
                            compact: true
                            onAct: if (pg.hub) pg.hub.section = "updates"
                        }
                    }
                }
            }

            // ── Card 3: SYSTEM SPECIFICATIONS ──
            SettingCard {
                width: col.colWidth
                title: I18n.tr("HARDWARE SPECIFICATIONS")
                collapsible: false

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: false
                    label: I18n.tr("Processor (CPU)")
                    value: SysInfo.sysCpuCores.length > 0 && SysInfo.sysCpuCores !== "-" ? SysInfo.sysCpuCores : ""
                    desc: SysInfo.sysCpu.length > 0 && SysInfo.sysCpu !== "-" ? SysInfo.sysCpu : I18n.tr("x86_64 Processor")
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Graphics (GPU)")
                    desc: SysInfo.sysGpu.length > 0 && SysInfo.sysGpu !== "-"
                        ? SysInfo.sysGpu + (SysInfo.sysGpu2.length > 0 ? " / " + SysInfo.sysGpu2 : "")
                        : I18n.tr("Display Controller")
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Memory (RAM)")
                    value: SysInfo.sysRamPct + "% used"
                    desc: SysInfo.sysRam.length > 0 && SysInfo.sysRam !== "- / -" ? SysInfo.sysRam : I18n.tr("System Memory")
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Storage (Root)")
                    value: SysInfo.sysDiskPct + "% used"
                    desc: SysInfo.sysDisk.length > 0 && SysInfo.sysDisk !== "- / -" ? SysInfo.sysDisk : I18n.tr("Root mount filesystem")
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Display Resolution")
                    value: SysInfo.sysRefresh.length > 0 ? SysInfo.sysRefresh : ""
                    desc: SysInfo.sysResolution.length > 0 && SysInfo.sysResolution !== "-" ? SysInfo.sysResolution : I18n.tr("Primary display")
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Machine Model")
                    desc: pg.hardwareModel
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("System Uptime")
                    value: SysInfo.sysUptime.length > 0 && SysInfo.sysUptime !== "-" ? SysInfo.sysUptime : "-"
                    desc: I18n.tr("Host: ") + SysInfo.sysHost
                }
            }
        }
    }
}
