#!/bin/sh
# Build a bootable Zenith live ISO using Void Linux x86_64 (glibc) base and void-mklive.
#
# Usage: installation/build-iso-void.sh [build-directory]
#
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BUILD_DIR=${1:-"$REPO_ROOT/build/iso-void"}
ARCH=${ZENITH_ARCH:-x86_64}
MIRROR=${ZENITH_MIRROR:-https://repo-fastly.voidlinux.org/current}
ISO_NAME=zenith-os-beta-001
ISO_TITLE="Zenith OS Beta 001"

for tool in python3 git curl; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "build-iso-void: required tool not found: $tool" >&2
		exit 2
	fi
done

if ! python3 -c 'import yaml' >/dev/null 2>&1; then
	echo "build-iso-void: PyYAML is required (python3 -c 'import yaml')" >&2
	exit 2
fi

mkdir -p "$BUILD_DIR"
BUILD_DIR=$(CDPATH= cd -- "$BUILD_DIR" && pwd)
INCLUDE_DIR="$BUILD_DIR/include"
mkdir -p "$INCLUDE_DIR/etc/sudoers.d" \
	"$INCLUDE_DIR/etc/udev/rules.d" \
	"$INCLUDE_DIR/etc/modprobe.d" \
	"$INCLUDE_DIR/etc/sysctl.d" \
	"$INCLUDE_DIR/etc/turnstile" \
	"$INCLUDE_DIR/etc/xbps.d" \
	"$INCLUDE_DIR/etc/plymouth" \
	"$INCLUDE_DIR/etc/dracut.conf.d" \
	"$INCLUDE_DIR/usr/bin" \
	"$INCLUDE_DIR/usr/share/zenith/branding" \
	"$INCLUDE_DIR/usr/share/zenith/session" \
	"$INCLUDE_DIR/usr/share/zenith/runit/services" \
	"$INCLUDE_DIR/usr/share/zenith/hyprland" \
	"$INCLUDE_DIR/usr/share/zenith/shell" \
	"$INCLUDE_DIR/usr/share/wayland-sessions"

# Configure Plymouth boot splash, mini log window, and progress
mkdir -p "$INCLUDE_DIR/usr/share/plymouth/themes/zenith"
cp -r "$REPO_ROOT/system/plymouth/zenith"/* "$INCLUDE_DIR/usr/share/plymouth/themes/zenith/"
ln -sf /usr/share/plymouth/themes/zenith/zenith.plymouth "$INCLUDE_DIR/usr/share/plymouth/themes/default.plymouth"

cat >"$INCLUDE_DIR/etc/plymouth/plymouthd.conf" <<'EOF'
[Daemon]
Theme=zenith
ShowDelay=0
DeviceTimeout=5
EOF

cat >"$INCLUDE_DIR/etc/dracut.conf.d/50-plymouth.conf" <<'EOF'
add_dracutmodules+=" plymouth "
install_items+=" /usr/lib/plymouth/label.so /etc/fonts/fonts.conf /etc/fonts/conf.d "
install_optional_items+=" /usr/share/fonts/* "
EOF

# Setup OS identity
cat >"$INCLUDE_DIR/etc/os-release" <<'EOF'
NAME="Zenith OS"
PRETTY_NAME="Zenith OS Beta 001"
ID=zenith
ID_LIKE="linux"
VERSION_ID="0.1"
VERSION_CODENAME=beta-001
HOME_URL="https://zenithos.dev/"
SUPPORT_URL="https://zenithos.dev/support"
BUG_REPORT_URL="https://github.com/mohitjoer/Zenith-OS/issues"
EOF

printf 'zenith-os\n' >"$INCLUDE_DIR/etc/hostname"

cat >"$INCLUDE_DIR/etc/issue" <<'EOF'
     ▄▄▄▄▄▄▄▄▄▄▄      Zenith OS Beta 001 (\m)
         ███          Kernel \r on an \m (\l)
    █████████████
     ███ ███ ███      天 — Robust. Minimal. Kinetic.
    ███  ███  ███     Live Session: user 'zenith' (nopasswd sudo)
   ███   ███   ███    Run 'zenith install' or 'zenith-installer' to install

EOF

cat >"$INCLUDE_DIR/etc/issue.net" <<'EOF'
     ▄▄▄▄▄▄▄▄▄▄▄      Zenith OS Beta 001
         ███          https://zenithos.dev
    █████████████
     ███ ███ ███      天 — Robust. Minimal. Kinetic.
    ███  ███  ███     Live Session: user 'zenith' (nopasswd sudo)
   ███   ███   ███    Run 'zenith install' or 'zenith-installer' to install

EOF

cat >"$INCLUDE_DIR/etc/motd" <<'EOF'
     ▄▄▄▄▄▄▄▄▄▄▄      Zenith OS Beta 001
         ███          https://zenithos.dev
    █████████████
     ███ ███ ███      天 — Robust. Minimal. Kinetic.
    ███  ███  ███     Live Session: user 'zenith' (nopasswd sudo)
   ███   ███   ███    Run 'zenith install' or 'zenith-installer' to install

EOF

# Copy branding assets
cp "$REPO_ROOT/public/zenith-logo.svg" \
	"$INCLUDE_DIR/usr/share/zenith/branding/"
cp "$REPO_ROOT/public/zenith-logo-on-dark.svg" \
	"$INCLUDE_DIR/usr/share/zenith/branding/"
if [ -f "$REPO_ROOT/public/branding/boot-splash.png" ]; then
	cp "$REPO_ROOT/public/branding/boot-splash.png" \
		"$INCLUDE_DIR/usr/share/zenith/branding/boot-splash.png"
fi

# Ensure strict permissions on sudoers.d directory (0750) and wheel rule (0440)
chmod 0750 "$INCLUDE_DIR/etc/sudoers.d"
echo '%wheel ALL=(ALL:ALL) NOPASSWD: ALL' > "$INCLUDE_DIR/etc/sudoers.d/wheel"
chmod 0440 "$INCLUDE_DIR/etc/sudoers.d/wheel"

# Copy session files, keybind catalogue, and desktop sessions
cp "$REPO_ROOT/session/zenith-session" "$INCLUDE_DIR/usr/share/zenith/session/zenith-session"
cp "$REPO_ROOT/session/zenith-session" "$INCLUDE_DIR/usr/bin/zenith-session"
cp "$REPO_ROOT/session/zenith-activate" "$INCLUDE_DIR/usr/share/zenith/session/zenith-activate"
cp "$REPO_ROOT/session/zenith-activate" "$INCLUDE_DIR/usr/bin/zenith-activate"
cp "$REPO_ROOT/session/zenith.desktop" "$INCLUDE_DIR/usr/share/zenith/session/zenith.desktop"
cp "$REPO_ROOT/session/zenith.desktop" "$INCLUDE_DIR/usr/share/wayland-sessions/zenith.desktop"
cp "$REPO_ROOT/session/binds.json" "$INCLUDE_DIR/usr/share/zenith/session/binds.json"
cp "$REPO_ROOT/session/runit/zenith-user-services" "$INCLUDE_DIR/usr/share/zenith/session/zenith-user-services"
cp "$REPO_ROOT/session/runit/zenith-user-services" "$INCLUDE_DIR/usr/bin/zenith-user-services"

# Copy session store wrapper, portal configuration, and session CLI scripts
cp "$REPO_ROOT/session/zenith-store" "$INCLUDE_DIR/usr/share/zenith/session/zenith-store"
mkdir -p "$INCLUDE_DIR/usr/share/xdg-desktop-portal"
cp "$REPO_ROOT/session/zenith-portals.conf" "$INCLUDE_DIR/usr/share/xdg-desktop-portal/zenith-portals.conf"

for s in zenith-hub zenith-idle zenith-pin zenith-reload-cover zenith-shot zenith-vm zenith-wallpaper-ui zenith-welcome zenith-greeter; do
	if [ -f "$REPO_ROOT/session/$s" ]; then
		cp "$REPO_ROOT/session/$s" "$INCLUDE_DIR/usr/bin/"
	fi
done
if [ -d "$REPO_ROOT/session/scripts" ]; then
	cp "$REPO_ROOT/session/scripts"/* "$INCLUDE_DIR/usr/bin/"
fi

# Copy all shell scripts (zenith-app, zenith-cmd-*, zenith-summon, etc.) to /usr/bin/
if [ -d "$REPO_ROOT/shell/scripts" ]; then
	cp "$REPO_ROOT/shell/scripts"/* "$INCLUDE_DIR/usr/bin/"
fi

# Copy all system helpers (zenith-volume, zenith-restart-audio, zenith-hw-backlight, etc.)
for hdir in audio display input perf pkg; do
	if [ -d "$REPO_ROOT/system/helpers/$hdir" ]; then
		for f in "$REPO_ROOT/system/helpers/$hdir"/*; do
			if [ -f "$f" ] && [ "${f##*.}" != "sh" -o "${f##*/}" = "zenith-installer.sh" ]; then
				cp "$f" "$INCLUDE_DIR/usr/bin/"
			fi
		done
	fi
done
cp "$REPO_ROOT/system/helpers/prime-run" "$INCLUDE_DIR/usr/bin/prime-run"
cp "$REPO_ROOT/system/helpers/fan-profile" "$INCLUDE_DIR/usr/bin/fan-profile"
cp "$REPO_ROOT/installation/zenith-installer.sh" "$INCLUDE_DIR/usr/bin/zenith-installer"

# Copy GLib GSettings schemas
mkdir -p "$INCLUDE_DIR/usr/share/glib-2.0/schemas"
cp "$REPO_ROOT/shell"/org.zenith.*.gschema.xml "$INCLUDE_DIR/usr/share/glib-2.0/schemas/"

# Copy session unit definitions (for reference/documentation only; runit handles supervision)
mkdir -p "$INCLUDE_DIR/usr/share/zenith/session/units"
if [ -d "$REPO_ROOT/session/units" ]; then
	cp "$REPO_ROOT/session/units"/* "$INCLUDE_DIR/usr/share/zenith/session/units/" 2>/dev/null || true
fi

# Copy applications .desktop entries
mkdir -p "$INCLUDE_DIR/usr/share/applications"
if [ -d "$REPO_ROOT/session/applications" ]; then
	cp "$REPO_ROOT/session/applications"/*.desktop "$INCLUDE_DIR/usr/share/applications/" 2>/dev/null || true
fi
if [ -d "$REPO_ROOT/apps/mimeapps" ]; then
	cp "$REPO_ROOT/apps/mimeapps"/zenith-mimeapps.list "$INCLUDE_DIR/usr/share/applications/" 2>/dev/null || true
fi

# Copy Hyprland configuration, runit services, and shell tree
cp -r "$REPO_ROOT/session/hyprland"/* "$INCLUDE_DIR/usr/share/zenith/hyprland/"
if [ -d "$REPO_ROOT/session/sddm" ]; then
	cp -r "$REPO_ROOT/session/sddm" "$INCLUDE_DIR/usr/share/zenith/session/" 2>/dev/null || true
fi
cp -r "$REPO_ROOT/shell"/* "$INCLUDE_DIR/usr/share/zenith/shell/"
cp -r "$REPO_ROOT/system/runit/user-services"/* "$INCLUDE_DIR/usr/share/zenith/runit/services/"

# Stage native Void Linux Hyprland compositor stack and libraries
echo "build-iso-void: staging native Void Linux Hyprland stack..."
HYPR_CACHE="$BUILD_DIR/hyprland-pkgs"
mkdir -p "$HYPR_CACHE"

# Dynamically resolve latest Makrennel package filenames from repo metadata to prevent version drift
RESOLVED_PKGS=$(python3 - <<'PY' 2>/dev/null || true
import urllib.request, plistlib, subprocess
wanted = ['tomlplusplus', 'hyprutils', 'aquamarine', 'hyprlang', 'hyprcursor', 'hyprgraphics', 'libspng', 'sdbus-cpp', 'hyprland', 'xdg-desktop-portal-hyprland']
url = 'https://raw.githubusercontent.com/Makrennel/hyprland-void/repository-x86_64-glibc/x86_64-repodata'
try:
    req = urllib.request.Request(url, headers={'User-Agent': 'Zenith-ISO-Builder'})
    data = urllib.request.urlopen(req, timeout=10).read()
    p = subprocess.Popen(['tar', '--zstd', '-xO', 'index.plist'], stdin=subprocess.PIPE, stdout=subprocess.PIPE)
    plist_bytes, _ = p.communicate(data)
    plist = plistlib.loads(plist_bytes)
    res = [f"{plist[pkg]['pkgver']}.x86_64.xbps" for pkg in wanted if pkg in plist]
    if len(res) == len(wanted):
        print(" ".join(res))
except Exception:
    pass
PY
)

HYPR_PKGS="${RESOLVED_PKGS:-tomlplusplus-3.4.0_1.x86_64.xbps hyprutils-0.7.1_1.x86_64.xbps aquamarine-0.8.0_2.x86_64.xbps hyprlang-0.6.3_1.x86_64.xbps hyprcursor-0.1.12_1.x86_64.xbps hyprgraphics-0.1.3_2.x86_64.xbps libspng-0.7.4_1.x86_64.xbps sdbus-cpp-2.1.0_1.x86_64.xbps hyprland-0.49.0_1.x86_64.xbps xdg-desktop-portal-hyprland-1.3.9_2.x86_64.xbps}"
for hpkg in $HYPR_PKGS; do
	if [ ! -f "$HYPR_CACHE/$hpkg" ]; then
		curl -sSL -o "$HYPR_CACHE/$hpkg" "https://raw.githubusercontent.com/Makrennel/hyprland-void/repository-x86_64-glibc/$hpkg" || true
	fi
	if [ -f "$HYPR_CACHE/$hpkg" ]; then
		tar -xf "$HYPR_CACHE/$hpkg" -C "$INCLUDE_DIR" 2>/dev/null || true
	fi
done
rm -f "$INCLUDE_DIR/props.plist" "$INCLUDE_DIR/files.plist"
if [ -f "$INCLUDE_DIR/usr/libexec/xdg-desktop-portal-hyprland" ]; then
	cp "$INCLUDE_DIR/usr/libexec/xdg-desktop-portal-hyprland" "$INCLUDE_DIR/usr/bin/xdg-desktop-portal-hyprland" 2>/dev/null || true
fi
# Compatibility soname symlinks for Void libraries with newer ABI bumps
mkdir -p "$INCLUDE_DIR/usr/lib"
ln -sf libdisplay-info.so.3 "$INCLUDE_DIR/usr/lib/libdisplay-info.so.2"
ln -sf libjxl.so.0.12 "$INCLUDE_DIR/usr/lib/libjxl.so.0.11"
ln -sf libjxl_threads.so.0.12 "$INCLUDE_DIR/usr/lib/libjxl_threads.so.0.11"
# libseat is installed by XBPS from packages-void.yaml; no host-path staging needed

# Pre-trust Makrennel repository public key in /var/db/xbps/keys
mkdir -p "$INCLUDE_DIR/var/db/xbps/keys"
cat >"$INCLUDE_DIR/var/db/xbps/keys/1293d0c27380d795a88fdbd007356dae84090282b5bd4123678ee9a682d06dd3.plist" <<'KEYEOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>public-key</key>
	<data>LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUlJQm9qQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FZOEFNSUlCaWdLQ0FZRUFtNWk4VnhQdEpJRVU5ZERuSlNmSAo5by9PNCt1THFYc1hqemY0emZxTXFXM0RIQTJjM2pHeDE2dFVUT0dHbUdzVk1pb3l6TXl0TjUwSTZwb0J0ODBZCjlWQ25MRVBmVFFmbjlNaHV1ZGFPNjA5OGZVb2srejA4WjJsZ205NVg5Z3JDblkraEhwMC8yZ2VxNGJ5Zy9mZG0KTjlCbGVqaTlNd2dJdHFpRllJM0J4MXdWUVkzekVzWEUyeXlseHpycTFpS2p0S2dYMXAxWDFnWDBadzR2ZEIzMwo4b1luSDNNYkNMUGlobWxocm5BMTRhWm1UaHFuNFNRU1A2czAzdStQSHNuaHVKQ3EvS1N4S3AyRFhVM0JpY1dmCnY0YmdPQmV2TmYrZWhKbDJyRHdTSk9lS0NrOFhnZVYyVXNKQW4xaTM3UUpjdm9ZNWZUamFNUmxYSnRISERvN04KVzNXZHVPcWxwMHVHVVlXZVNxLzFMRVlJUDE2bGJ1dXd1a1ZrWXF5QzZMbWY0VXovOVZOYkc1anp0UUdmVkZJdgoxZmppZGg0cEpzNWEyeGx6QVpOTVpoT1VMdElnM2U5bkorYUc5dGt5K09kSjRiS1lja2NicWxRMlNiME5QTmF4CnQ4Q1Q2c2E4MHNJRU1oZWRvemUvVkhmdlNaZitLZ3FPc25BaGMvY1ZEUmhsQWdNQkFBRT0KLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0tCg==</data>
	<key>public-key-size</key>
	<integer>3072</integer>
	<key>signature-by</key>
	<string>hyprland-void-github-action</string>
	<key>signature-type</key>
	<string>rsa</string>
</dict>
</plist>
KEYEOF

# Copy host quickshell binary if available
if [ -x /usr/bin/quickshell ]; then
	cp /usr/bin/quickshell "$INCLUDE_DIR/usr/bin/quickshell"
fi

# Copy udev power rules (99-zenith-power.rules, 80-nvidia-pm.rules, 99-battery-threshold.rules)
cp "$REPO_ROOT/system/udev"/*.rules "$INCLUDE_DIR/etc/udev/rules.d/"
if [ -d "$REPO_ROOT/system/modprobe.d" ]; then
	cp "$REPO_ROOT/system/modprobe.d"/*.conf "$INCLUDE_DIR/etc/modprobe.d/"
fi
cp "$REPO_ROOT/system/sysctl.d/99-zenith-performance.conf" "$INCLUDE_DIR/etc/sysctl.d/99-zenith-performance.conf"
cp "$REPO_ROOT/session/turnstile/turnstile.conf" "$INCLUDE_DIR/etc/turnstile/turnstile.conf"
cp "$REPO_ROOT/system/xbps.d/zenith.conf" "$INCLUDE_DIR/etc/xbps.d/zenith.conf"

# Default environment for seatd & Wayland
cat >"$INCLUDE_DIR/etc/environment" <<'EOF'
LIBSEAT_BACKEND=seatd
XDG_SESSION_TYPE=wayland
QT_QPA_PLATFORM=wayland
GDK_BACKEND=wayland
MOZ_ENABLE_WAYLAND=1
EOF

# Build and copy Zenith Go control binaries into include tree
echo "build-iso-void: building Zenith Go binaries..."
(
	cd "$REPO_ROOT/desktop"
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/zenith" ./cmd/zenith
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/zenith-sysd" ./cmd/zenith-sysd
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/zenith-userd" ./cmd/zenith-userd
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/zenith-hubd" ./cmd/zenith-hubd
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/zenith-shell" ./cmd/zenith-shell
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/zenith-wallpaper" ./cmd/zenith-wallpaper
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/zpm" ./cmd/zpm
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/zenith-store" ./cmd/zenith-store
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/zenith-agent" ./cmd/zenith-agent
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/zenith-vm-mon" ./cmd/zenith-vm/mon
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/zenith-vm-ssh" ./cmd/zenith-vm/remote
	cp -f "$REPO_ROOT/desktop/cmd/zenith-vm/zenith-vmd" "$INCLUDE_DIR/usr/bin/zenith-vmd"
)
find "$INCLUDE_DIR/usr/bin" -type f -exec chmod 0755 {} +
# Symlinks
ln -sf zenith "$INCLUDE_DIR/usr/bin/os-ctl"

# Stage system runit services (zenith-sysd)
mkdir -p "$INCLUDE_DIR/etc/sv/zenith-sysd"
cp "$REPO_ROOT/system/runit/services/zenith-sysd/run" "$INCLUDE_DIR/etc/sv/zenith-sysd/run"
chmod +x "$INCLUDE_DIR/etc/sv/zenith-sysd/run"

# Extract package list from system/packages-void.yaml
PACKAGES=$(python3 - "$REPO_ROOT/system/packages-void.yaml" "$MIRROR" <<'PY'
import sys, yaml
from pathlib import Path

manifest = yaml.safe_load(Path(sys.argv[1]).read_text())

if manifest.get("base", {}).get("distribution") != "void-linux":
    raise SystemExit("build-iso-void: system/packages-void.yaml must declare distribution: void-linux")

packages = []
for group, body in manifest.items():
    if group in {"version", "base"} or not isinstance(body, dict):
        continue
    entries = body.get("packages")
    if isinstance(entries, list):
        packages.extend(entries)

filtered = [p for p in packages if p != "base-system"]
all_requested = sorted(dict.fromkeys(str(p) for p in filtered))
custom_pkgs = {"hyprland", "xdg-desktop-portal-hyprland"}
out = [p for p in all_requested if p not in custom_pkgs]
print(" ".join(out))
PY
)

# Acquire void-mklive
MKLIVE_DIR="$BUILD_DIR/void-mklive"
if [ ! -d "$MKLIVE_DIR" ]; then
	echo "build-iso-void: cloning void-mklive..."
	git clone --depth 1 https://github.com/void-linux/void-mklive.git "$MKLIVE_DIR"
fi

# Overwrite upstream mklive issue, motd, and splash data files to eliminate upstream branding
if [ -d "$MKLIVE_DIR/data" ]; then
	cp "$INCLUDE_DIR/etc/issue" "$MKLIVE_DIR/data/issue"
	cp "$INCLUDE_DIR/etc/motd" "$MKLIVE_DIR/data/motd"
	if [ -f "$REPO_ROOT/public/branding/boot-splash.png" ]; then
		cp "$REPO_ROOT/public/branding/boot-splash.png" "$MKLIVE_DIR/data/splash.png"
	fi
fi

# Clean up void-mklive bootstrap repository flags and force-include plymouth in initramfs
if [ -f "$MKLIVE_DIR/mklive.sh" ]; then
	sed -i 's/--force-add "vmklive autoinstaller"/--force-add "vmklive autoinstaller plymouth"/g' "$MKLIVE_DIR/mklive.sh"
	sed -i 's|--repository=https://repo-default.voidlinux.org/current/musl||g' "$MKLIVE_DIR/mklive.sh"
	sed -i "s|https://repo-default.voidlinux.org/current|$MIRROR|g" "$MKLIVE_DIR/mklive.sh"
fi
if [ -f "$MKLIVE_DIR/lib.sh" ]; then
	sed -i 's|--repository=https://repo-default.voidlinux.org/current/musl||g' "$MKLIVE_DIR/lib.sh"
	sed -i "s|https://repo-default.voidlinux.org/current|$MIRROR|g" "$MKLIVE_DIR/lib.sh"
fi

# Patch dracut vmklive module to use zenith-os hostname, zenith user, and include _seatd/input groups
if [ -f "$MKLIVE_DIR/dracut/vmklive/adduser.sh" ]; then
	sed -i 's/echo void-live >/echo zenith-os >/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
	sed -i 's/-G audio,video,wheel/-G audio,video,wheel,_seatd,input,tty,storage,network/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
	sed -i 's/USERNAME=anon/USERNAME=zenith/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
	sed -i 's/voidlinux/zenith/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
	sed -i 's/99-void-live/99-zenith-live/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
	sed -i 's/void-live.rules/zenith-live.rules/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
fi

# Replace cluttered multi-option isolinux menu with a clean, streamlined Zenith OS boot menu
if [ -d "$MKLIVE_DIR/isolinux" ]; then
	cat >"$MKLIVE_DIR/isolinux/isolinux.cfg.in" <<'BOOTCFG'
UI vesamenu.c32
PROMPT 0
TIMEOUT 30
ONTIMEOUT linux

MENU TABMSG Press ENTER to boot or TAB to edit boot options
MENU AUTOBOOT Zenith OS starts in # second{,s}...
MENU BACKGROUND @@SPLASHIMAGE@@
MENU WIDTH 78
MENU MARGIN 4
MENU ROWS 6
MENU VSHIFT 2
MENU TIMEOUTROW 10
MENU TABMSGROW 2
MENU CMDLINEROW 13

MENU COLOR title        * #ffffffff *
MENU COLOR border       * #00000000 #00000000 none
MENU COLOR sel          * #00000000 #ffffffff *
MENU COLOR unsel        * #ffffffff #00000000 none
MENU COLOR timeout_msg  * #88888888 #00000000 none
MENU COLOR timeout      * #ffffffff #00000000 none
MENU COLOR tabmsg       * #66666666 #00000000 none

LABEL linux
MENU LABEL Zenith OS (Live Session)
KERNEL /boot/vmlinuz
APPEND initrd=/boot/initrd root=live:CDLABEL=VOID_LIVE init=/sbin/init ro rd.luks=0 rd.md=0 rd.dm=0 loglevel=0 rd.udev.log_level=0 vconsole.unicode=1 vconsole.keymap=@@KEYMAP@@ locale.LANG=@@LOCALE@@ vga=792 video=1024x768 @@BOOT_CMDLINE@@

LABEL linuxram
MENU LABEL Zenith OS (Copy to RAM)
KERNEL /boot/vmlinuz
APPEND initrd=/boot/initrd root=live:CDLABEL=VOID_LIVE init=/sbin/init ro rd.luks=0 rd.md=0 rd.dm=0 loglevel=0 rd.udev.log_level=0 vconsole.unicode=1 vconsole.keymap=@@KEYMAP@@ locale.LANG=@@LOCALE@@ vga=792 video=1024x768 @@BOOT_CMDLINE@@ rd.live.ram

LABEL reboot
MENU LABEL Reboot System
COM32 reboot.c32

LABEL poweroff
MENU LABEL Power Off
COM32 poweroff.c32
BOOTCFG
fi

# Prune cluttered extra speech and nogfx entries from GRUB configuration
if [ -f "$MKLIVE_DIR/grub/grub_void.cfg.pre" ]; then
	sed -i 's/timeout=15/timeout=3/g' "$MKLIVE_DIR/grub/grub_void.cfg.pre"
fi

# Ensure XBPS tools are available (if running on a non-Void host, fetch xbps-static)
if ! command -v xbps-install >/dev/null 2>&1; then
	echo "build-iso-void: xbps-install not found on host, setting up xbps-static..."
	XBPS_STATIC_DIR="$BUILD_DIR/xbps-static"
	if [ ! -x "$XBPS_STATIC_DIR/usr/bin/xbps-install" ]; then
		mkdir -p "$XBPS_STATIC_DIR"
		curl -sSL "https://repo-fastly.voidlinux.org/static/xbps-static-latest.x86_64-musl.tar.xz" | tar -xJ -C "$XBPS_STATIC_DIR"
	fi
	export PATH="$XBPS_STATIC_DIR/usr/bin:$PATH"
fi

# Post-setup script to configure users and seat permissions inside ROOTFS
POSTSETUP_SCRIPT="$BUILD_DIR/postsetup.sh"
cat >"$POSTSETUP_SCRIPT" <<'EOF'
#!/bin/sh
ROOTFS="$1"

# Enable core system runit services (dbus manages elogind on-demand via D-Bus activation)
mkdir -p "$ROOTFS/etc/runit/runsvdir/default"
for svc in dbus seatd udevd NetworkManager acpid turnstiled tlp zramen zenith-sysd; do
	if [ -d "$ROOTFS/etc/sv/$svc" ]; then
		ln -sf "/etc/sv/$svc" "$ROOTFS/etc/runit/runsvdir/default/$svc"
	fi
done

# Ensure runtime library compatibility symlinks exist inside rootfs
mkdir -p "$ROOTFS/usr/lib"
[ -e "$ROOTFS/usr/lib/libdisplay-info.so.3" ] && ln -sf libdisplay-info.so.3 "$ROOTFS/usr/lib/libdisplay-info.so.2"
[ -e "$ROOTFS/usr/lib/libjxl.so.0.12" ] && ln -sf libjxl.so.0.12 "$ROOTFS/usr/lib/libjxl.so.0.11"
[ -e "$ROOTFS/usr/lib/libjxl_threads.so.0.12" ] && ln -sf libjxl_threads.so.0.12 "$ROOTFS/usr/lib/libjxl_threads.so.0.11"
ldconfig -r "$ROOTFS" 2>/dev/null || true

# Hook turnstile into PAM session stack if present
if [ -f "$ROOTFS/etc/pam.d/system-login" ] && ! grep -q 'pam_turnstile.so' "$ROOTFS/etc/pam.d/system-login"; then
	echo "session optional pam_turnstile.so" >> "$ROOTFS/etc/pam.d/system-login"
fi

# Configure seat permissions and groups for default user
for user in anon zenith; do
	if chroot "$ROOTFS" id "$user" >/dev/null 2>&1; then
		chroot "$ROOTFS" usermod -aG _seatd,video,audio,input,wheel,storage,network "$user" || true
	fi
done

# Ensure user zenith exists and configure autologin
if ! chroot "$ROOTFS" id zenith >/dev/null 2>&1; then
	chroot "$ROOTFS" useradd -m -s /bin/bash -U -G _seatd,video,audio,input,wheel,storage,network zenith || true
fi
echo "zenith:zenith" | chroot "$ROOTFS" chpasswd 2>/dev/null || true

# Reset root and anon passwords to eliminate upstream voidlinux defaults
echo "root:zenith" | chroot "$ROOTFS" chpasswd 2>/dev/null || true
if chroot "$ROOTFS" id anon >/dev/null 2>&1; then
	echo "anon:zenith" | chroot "$ROOTFS" chpasswd 2>/dev/null || true
fi

# Neutralize upstream installer and docs commands
if [ -f "$ROOTFS/usr/bin/void-installer" ]; then
	rm -f "$ROOTFS/usr/bin/void-installer"
	ln -sf /usr/bin/zenith-installer "$ROOTFS/usr/bin/void-installer"
fi

cat >"$ROOTFS/usr/bin/void-docs" <<'DOCS'
#!/bin/sh
echo "Zenith OS documentation is available at: https://zenithos.dev"
DOCS
chmod 0755 "$ROOTFS/usr/bin/void-docs"

# Sudo permissions for wheel group (strict 0750 dir, 0440 file)
chmod 0750 "$ROOTFS/etc/sudoers.d"
echo '%wheel ALL=(ALL:ALL) NOPASSWD: ALL' > "$ROOTFS/etc/sudoers.d/wheel"
chmod 0440 "$ROOTFS/etc/sudoers.d/wheel"
chmod 0440 "$ROOTFS/etc/sudoers.d"/* 2>/dev/null || true
grep -q '%wheel ALL=(ALL:ALL) NOPASSWD: ALL' "$ROOTFS/etc/sudoers" 2>/dev/null || echo '%wheel ALL=(ALL:ALL) NOPASSWD: ALL' >> "$ROOTFS/etc/sudoers"

# Compile GSettings schemas for Zenith Shell and Settings
if command -v glib-compile-schemas >/dev/null 2>&1; then
	glib-compile-schemas "$ROOTFS/usr/share/glib-2.0/schemas" 2>/dev/null || true
fi

# Enable automatic console login on tty1
mkdir -p "$ROOTFS/etc/sv/agetty-tty1"
echo 'GETTY_ARGS="--autologin zenith --noclear"' > "$ROOTFS/etc/sv/agetty-tty1/conf"

# Overwrite OS identity inside ROOTFS
cat >"$ROOTFS/etc/os-release" <<'OSREL'
NAME="Zenith OS"
PRETTY_NAME="Zenith OS Beta 001"
ID=zenith
ID_LIKE="linux"
VERSION_ID="0.1"
VERSION_CODENAME=beta-001
HOME_URL="https://zenithos.dev/"
SUPPORT_URL="https://zenithos.dev/support"
BUG_REPORT_URL="https://github.com/mohitjoer/Zenith-OS/issues"
OSREL

# Branding replacement: Change Welcome to Void! to Welcome to Zenith OS! in the boot sequence
for runit_file in "$ROOTFS/etc/runit/1" "$ROOTFS/etc/runit/2" "$ROOTFS/etc/runit/3" "$ROOTFS/etc/rc.local"; do
	if [ -f "$runit_file" ]; then
		sed -i 's/Welcome to Void!/Welcome to Zenith OS!/g' "$runit_file"
		sed -i 's/Welcome to Void Linux!/Welcome to Zenith OS!/g' "$runit_file"
		sed -i 's/Void Linux/Zenith OS/g' "$runit_file"
	fi
done

# Set 80% battery charge threshold fallback in /etc/rc.local
if [ -f "$ROOTFS/etc/rc.local" ]; then
	if ! grep -q 'charge_control_end_threshold' "$ROOTFS/etc/rc.local"; then
		echo 'for b in /sys/class/power_supply/BAT*/charge_control_end_threshold; do [ -f "$b" ] && echo 80 > "$b" 2>/dev/null || true; done' >> "$ROOTFS/etc/rc.local"
	fi
fi

# Set Zenith OS hostname and network identity
printf 'zenith-os\n' > "$ROOTFS/etc/hostname"
if [ -f "$ROOTFS/etc/rc.conf" ]; then
	sed -i 's/^#\?HOSTNAME=.*/HOSTNAME="zenith-os"/' "$ROOTFS/etc/rc.conf"
	if ! grep -q '^HOSTNAME=' "$ROOTFS/etc/rc.conf"; then
		echo 'HOSTNAME="zenith-os"' >> "$ROOTFS/etc/rc.conf"
	fi
fi
if [ -f "$ROOTFS/etc/hosts" ]; then
	sed -i 's/void-live/zenith-os/g' "$ROOTFS/etc/hosts"
fi

# Update all TTY greeting messages (/etc/issue, /etc/issue.net, /etc/motd) to show Zenith OS wordmark and logo
cat >"$ROOTFS/etc/issue" <<'ISSUE'
     ▄▄▄▄▄▄▄▄▄▄▄      Zenith OS Beta 001 (\m)
         ███          Kernel \r on an \m (\l)
    █████████████
     ███ ███ ███      天 — Robust. Minimal. Kinetic.
    ███  ███  ███     Live Session: user 'zenith' (nopasswd sudo)
   ███   ███   ███    Run 'zenith install' or 'zenith-installer' to install

ISSUE

cat >"$ROOTFS/etc/issue.net" <<'ISSUENET'
     ▄▄▄▄▄▄▄▄▄▄▄      Zenith OS Beta 001
         ███          https://zenithos.dev
    █████████████
     ███ ███ ███      天 — Robust. Minimal. Kinetic.
    ███  ███  ███     Live Session: user 'zenith' (nopasswd sudo)
   ███   ███   ███    Run 'zenith install' or 'zenith-installer' to install

ISSUENET

cat >"$ROOTFS/etc/motd" <<'MOTD'
     ▄▄▄▄▄▄▄▄▄▄▄      Zenith OS Beta 001
         ███          https://zenithos.dev
    █████████████
     ███ ███ ███      天 — Robust. Minimal. Kinetic.
    ███  ███  ███     Live Session: user 'zenith' (nopasswd sudo)
   ███   ███   ███    Run 'zenith install' or 'zenith-installer' to install

MOTD

# Auto-start Zenith Wayland desktop on tty1 login (with safe fallback if compositor exits)
mkdir -p "$ROOTFS/etc/profile.d"
cat >"$ROOTFS/etc/profile.d/zenith-autologin.sh" <<'PROFILE'
if [ -z "$WAYLAND_DISPLAY" ] && [ -z "$DISPLAY" ] && [ "$(tty 2>/dev/null)" = "/dev/tty1" ]; then
	echo "Starting Zenith OS graphical session..."
	if ! /usr/share/zenith/session/zenith-session; then
		echo
		echo "================================================================"
		echo "  Zenith graphical desktop session exited."
		echo "  Log available at: ~/.local/state/zenith/session.log"
		echo
		if [ -f "$HOME/.local/state/zenith/session.log" ]; then
			echo "--- Last 20 lines of session.log ---"
			tail -n 20 "$HOME/.local/state/zenith/session.log"
			echo "------------------------------------"
		fi
		echo "  To launch the installer directly, run: sudo zenith install"
		echo "  To retry launching the desktop, run: zenith-session"
		echo "================================================================"
		echo
	fi
fi
PROFILE
chmod 0755 "$ROOTFS/etc/profile.d/zenith-autologin.sh"
exit 0
EOF
chmod +x "$POSTSETUP_SCRIPT"

FINAL_ISO="$BUILD_DIR/$ISO_NAME.iso"

# Run mklive to produce the image with boot splash & hybrid graphics
echo "build-iso-void: generating ISO with void-mklive..."
(
	cd "$MKLIVE_DIR"
	export SPLASH_IMAGE="$REPO_ROOT/public/branding/boot-splash.png"
	./mklive.sh \
		-a "$ARCH" \
		-b base-system \
		-r "$MIRROR" \
		-p "$PACKAGES" \
		-I "$INCLUDE_DIR" \
		-s xz \
		-S "dbus seatd NetworkManager zramen" \
		-C "live.user=zenith live.autologin splash quiet loglevel=0 rd.udev.log_level=0 vt.global_cursor_default=0 fbcon=nodefer vga=792 video=1024x768 nvidia-drm.modeset=1 nvidia-drm.fbdev=1 NVreg_DynamicPowerManagement=0x02 nowatchdog mitigations=off preempt=full" \
		-x "$POSTSETUP_SCRIPT" \
		-T "$ISO_TITLE" \
		-o "$FINAL_ISO"
)

if [ ! -f "$FINAL_ISO" ]; then
	echo "build-iso-void: void-mklive produced no ISO" >&2
	exit 1
fi

chmod -R a+rwX "$BUILD_DIR" || true
echo "build-iso-void: image written to $FINAL_ISO"
