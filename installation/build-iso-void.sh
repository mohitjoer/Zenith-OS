#!/bin/sh
# Build a bootable Asura live ISO using Void Linux x86_64 (glibc) base and void-mklive.
#
# Usage: installation/build-iso-void.sh [build-directory]
#
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BUILD_DIR=${1:-"$REPO_ROOT/build/iso-void"}
ARCH=${ASURA_ARCH:-x86_64}
MIRROR=${ASURA_MIRROR:-https://repo-fastly.voidlinux.org/current}
ISO_NAME=asura-os-x86_64
ISO_TITLE="Asura OS x86_64"
export GOWORK=off

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
rm -rf "$INCLUDE_DIR"
mkdir -p "$INCLUDE_DIR/etc/sudoers.d" \
	"$INCLUDE_DIR/etc/udev/rules.d" \
	"$INCLUDE_DIR/etc/modprobe.d" \
	"$INCLUDE_DIR/etc/sysctl.d" \
	"$INCLUDE_DIR/etc/turnstile" \
	"$INCLUDE_DIR/etc/xbps.d" \
	"$INCLUDE_DIR/etc/plymouth" \
	"$INCLUDE_DIR/etc/dracut.conf.d" \
	"$INCLUDE_DIR/usr/bin" \
	"$INCLUDE_DIR/usr/share/asura/branding" \
	"$INCLUDE_DIR/usr/share/asura/session" \
	"$INCLUDE_DIR/usr/share/asura/runit/services" \
	"$INCLUDE_DIR/usr/share/asura/hyprland" \
	"$INCLUDE_DIR/usr/share/asura/shell" \
	"$INCLUDE_DIR/usr/share/wayland-sessions"

# Configure Plymouth boot splash with logo and loading progress bar animation
mkdir -p "$INCLUDE_DIR/usr/share/plymouth/themes/asura"
cp -r "$REPO_ROOT/system/plymouth/asura"/* "$INCLUDE_DIR/usr/share/plymouth/themes/asura/"
ln -sf /usr/share/plymouth/themes/asura/asura.plymouth "$INCLUDE_DIR/usr/share/plymouth/themes/default.plymouth"

# Pre-stage TrueType fonts and fontconfig configuration for Plymouth label rendering
mkdir -p "$INCLUDE_DIR/usr/share/fonts/TTF" "$INCLUDE_DIR/etc/fonts"
rm -rf "$INCLUDE_DIR/usr/lib/plymouth"
rm -f "$INCLUDE_DIR/usr/lib/libseat.so"*
for font in DejaVuSans.ttf DejaVuSans-Bold.ttf DejaVuSansMono.ttf; do
	for src in "/usr/share/fonts/truetype/dejavu/$font" "/usr/share/fonts/TTF/$font"; do
		if [ -f "$src" ]; then
			cp -L "$src" "$INCLUDE_DIR/usr/share/fonts/TTF/$font"
			break
		fi
	done
done
if [ -d "$REPO_ROOT/public/fonts" ]; then
	cp -L "$REPO_ROOT/public/fonts"/* "$INCLUDE_DIR/usr/share/fonts/TTF/"
fi

cat >"$INCLUDE_DIR/etc/fonts/fonts.conf" <<'EOF'
<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "urn:fontconfig:fonts.dtd">
<fontconfig>
  <dir>/usr/share/fonts</dir>
  <dir>/usr/share/fonts/TTF</dir>
  <cachedir>/var/cache/fontconfig</cachedir>
  <alias>
    <family>sans-serif</family>
    <prefer>
      <family>Inter</family>
      <family>Space Grotesk</family>
      <family>Noto Sans</family>
      <family>DejaVu Sans</family>
    </prefer>
  </alias>
  <alias>
    <family>sans</family>
    <prefer>
      <family>Inter</family>
      <family>Space Grotesk</family>
      <family>Noto Sans</family>
      <family>DejaVu Sans</family>
    </prefer>
  </alias>
  <alias>
    <family>monospace</family>
    <prefer>
      <family>JetBrainsMono Nerd Font</family>
      <family>JetBrains Mono</family>
      <family>DejaVu Sans Mono</family>
    </prefer>
  </alias>
</fontconfig>
EOF

cat >"$INCLUDE_DIR/etc/fonts/local.conf" <<'EOF'
<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "urn:fontconfig:fonts.dtd">
<fontconfig>
  <dir>/usr/share/fonts</dir>
  <dir>/usr/share/fonts/TTF</dir>
  <alias>
    <family>sans-serif</family>
    <prefer>
      <family>Inter</family>
      <family>Space Grotesk</family>
      <family>Noto Sans</family>
      <family>DejaVu Sans</family>
    </prefer>
  </alias>
  <alias>
    <family>sans</family>
    <prefer>
      <family>Inter</family>
      <family>Space Grotesk</family>
      <family>Noto Sans</family>
      <family>DejaVu Sans</family>
    </prefer>
  </alias>
  <alias>
    <family>monospace</family>
    <prefer>
      <family>JetBrainsMono Nerd Font</family>
      <family>JetBrains Mono</family>
      <family>DejaVu Sans Mono</family>
    </prefer>
  </alias>
</fontconfig>
EOF

cat >"$INCLUDE_DIR/etc/plymouth/plymouthd.conf" <<'EOF'
[Daemon]
Theme=asura
ShowDelay=0
DeviceTimeout=2
EOF

cat >"$INCLUDE_DIR/etc/dracut.conf.d/50-plymouth.conf" <<'EOF'
add_dracutmodules+=" plymouth "
add_drivers+=" vboxvideo vmwgfx bochs virtio_gpu qxl simpledrm "
install_optional_items+=" /usr/lib/plymouth/script.so /usr/lib/plymouth/label.so /usr/lib/plymouth/label-pango.so /usr/lib/plymouth/renderers/drm.so /usr/lib/plymouth/renderers/frame-buffer.so /etc/fonts/fonts.conf /etc/fonts/local.conf /usr/share/fonts/TTF/DejaVuSans.ttf /usr/share/fonts/TTF/DejaVuSans-Bold.ttf /usr/share/fonts/TTF/DejaVuSansMono.ttf "
EOF

# Setup OS identity
cat >"$INCLUDE_DIR/etc/os-release" <<'EOF'
NAME="Asura OS"
PRETTY_NAME="Asura OS Beta 001"
ID=asura
ID_LIKE="linux"
VERSION_ID="0.1"
VERSION_CODENAME=beta-001
HOME_URL="https://asuraos.dev/"
SUPPORT_URL="https://asuraos.dev/support"
BUG_REPORT_URL="https://github.com/mohitjoer/Asura-OS/issues"
EOF

printf 'asura-os\n' >"$INCLUDE_DIR/etc/hostname"

cat >"$INCLUDE_DIR/etc/issue" <<'EOF'
       #                    #         Asura OS Beta 001 (\m)
     %##                    ##%       Kernel \r on an \m (\l)
    ####   %%%%%%%%%%%%%%   ####
  ######% %%%%%%%%%%%%%%%% %######    Robust. Minimal. Kinetic.
%###########% %%%%%%%%  ###########%  Live Session: user 'asura' (nopasswd sudo)
 %###########          ###########%   Run 'asura install' or 'asura-installer' to install
     ########  @@@@@@  ########
   %% ########%      %######## %%%    https://asuraos.dev
   %%  %%##################%%% %%%
   %% ##     %########%     ## %%%
   %% ####%%%%%      %%%%%#### %%
       %####################%
      %%  %##############%  %%
       ### ############## %##
       %##  ############  ###
        ###  %        %  ###
         %################%
           %############%
             ##########

EOF

cat >"$INCLUDE_DIR/etc/issue.net" <<'EOF'
       #                    #         Asura OS Beta 001
     %##                    ##%       https://asuraos.dev
    ####   %%%%%%%%%%%%%%   ####
  ######% %%%%%%%%%%%%%%%% %######    Robust. Minimal. Kinetic.
%###########% %%%%%%%%  ###########%  Live Session: user 'asura' (nopasswd sudo)
 %###########          ###########%   Run 'asura install' or 'asura-installer' to install
     ########  @@@@@@  ########
   %% ########%      %######## %%%
   %%  %%##################%%% %%%
   %% ##     %########%     ## %%%
   %% ####%%%%%      %%%%%#### %%
       %####################%
      %%  %##############%  %%
       ### ############## %##
       %##  ############  ###
        ###  %        %  ###
         %################%
           %############%
             ##########

EOF

cat >"$INCLUDE_DIR/etc/motd" <<'EOF'
       #                    #         Asura OS Beta 001
     %##                    ##%       https://asuraos.dev
    ####   %%%%%%%%%%%%%%   ####
  ######% %%%%%%%%%%%%%%%% %######    Robust. Minimal. Kinetic.
%###########% %%%%%%%%  ###########%  Live Session: user 'asura' (nopasswd sudo)
 %###########          ###########%   Run 'asura install' or 'asura-installer' to install
     ########  @@@@@@  ########
   %% ########%      %######## %%%
   %%  %%##################%%% %%%
   %% ##     %########%     ## %%%
   %% ####%%%%%      %%%%%#### %%
       %####################%
      %%  %##############%  %%
       ### ############## %##
       %##  ############  ###
        ###  %        %  ###
         %################%
           %############%
             ##########

EOF



# Copy branding assets
mkdir -p "$INCLUDE_DIR/usr/share/asura/branding"
cp "$REPO_ROOT/public/asura-logo.svg" \
	"$INCLUDE_DIR/usr/share/asura/branding/"
cp "$REPO_ROOT/public/asura-logo-on-dark.svg" \
	"$INCLUDE_DIR/usr/share/asura/branding/"
if [ -f "$REPO_ROOT/public/branding/boot-splash.png" ]; then
	cp "$REPO_ROOT/public/branding/boot-splash.png" \
		"$INCLUDE_DIR/usr/share/asura/branding/boot-splash.png"
fi
if [ -d "$REPO_ROOT/public/branding" ]; then
	cp -r "$REPO_ROOT/public/branding"/* \
		"$INCLUDE_DIR/usr/share/asura/branding/" 2>/dev/null || true
fi

# Ensure strict permissions on sudoers.d directory (0750) and wheel rule (0440)
chmod 0750 "$INCLUDE_DIR/etc/sudoers.d"
echo '%wheel ALL=(ALL:ALL) NOPASSWD: ALL' > "$INCLUDE_DIR/etc/sudoers.d/wheel"
chmod 0440 "$INCLUDE_DIR/etc/sudoers.d/wheel"

# Copy session files, keybind catalogue, and desktop sessions
cp "$REPO_ROOT/session/asura-session" "$INCLUDE_DIR/usr/share/asura/session/asura-session"
cp "$REPO_ROOT/session/asura-session" "$INCLUDE_DIR/usr/bin/asura-session"
cp "$REPO_ROOT/session/asura-activate" "$INCLUDE_DIR/usr/share/asura/session/asura-activate"
cp "$REPO_ROOT/session/asura-activate" "$INCLUDE_DIR/usr/bin/asura-activate"
cp "$REPO_ROOT/session/asura.desktop" "$INCLUDE_DIR/usr/share/asura/session/asura.desktop"
cp "$REPO_ROOT/session/asura.desktop" "$INCLUDE_DIR/usr/share/wayland-sessions/asura.desktop"
cp "$REPO_ROOT/session/binds.json" "$INCLUDE_DIR/usr/share/asura/session/binds.json"
cp "$REPO_ROOT/session/runit/asura-user-services" "$INCLUDE_DIR/usr/share/asura/session/asura-user-services"
cp "$REPO_ROOT/session/runit/asura-user-services" "$INCLUDE_DIR/usr/bin/asura-user-services"

# Copy session store wrapper, portal configuration, and session CLI scripts
cp "$REPO_ROOT/session/asura-store" "$INCLUDE_DIR/usr/share/asura/session/asura-store"
mkdir -p "$INCLUDE_DIR/usr/share/xdg-desktop-portal" "$INCLUDE_DIR/usr/share/asura/session"
cp "$REPO_ROOT/session/asura-portals.conf" "$INCLUDE_DIR/usr/share/xdg-desktop-portal/asura-portals.conf"
cp "$REPO_ROOT/session/asura-portals.conf" "$INCLUDE_DIR/usr/share/asura/session/asura-portals.conf"

# Stage session units into /usr/share/asura/units and systemd user directory
mkdir -p "$INCLUDE_DIR/usr/share/asura/units" "$INCLUDE_DIR/usr/share/asura/session/units" "$INCLUDE_DIR/usr/lib/systemd/user"
if [ -d "$REPO_ROOT/session/units" ]; then
	cp "$REPO_ROOT/session/units"/* "$INCLUDE_DIR/usr/share/asura/units/"
	cp "$REPO_ROOT/session/units"/* "$INCLUDE_DIR/usr/share/asura/session/units/"
	cp "$REPO_ROOT/session/units"/* "$INCLUDE_DIR/usr/lib/systemd/user/"
fi

# Copy default wallpapers to /usr/share/asura/backgrounds/ and /etc/skel/Pictures/Wallpapers/
mkdir -p "$INCLUDE_DIR/usr/share/asura/backgrounds"
mkdir -p "$INCLUDE_DIR/etc/skel/Pictures/Wallpapers"
if [ -d "$REPO_ROOT/public/wallpapers" ]; then
	cp "$REPO_ROOT/public/wallpapers"/* "$INCLUDE_DIR/usr/share/asura/backgrounds/"
	cp "$REPO_ROOT/public/wallpapers"/* "$INCLUDE_DIR/etc/skel/Pictures/Wallpapers/"
fi

for s in asura-hub asura-idle asura-pin asura-reload-cover asura-shot asura-vm asura-wallpaper-ui asura-welcome asura-greeter; do
	if [ -f "$REPO_ROOT/session/$s" ]; then
		cp "$REPO_ROOT/session/$s" "$INCLUDE_DIR/usr/bin/"
	fi
done
if [ -d "$REPO_ROOT/session/scripts" ]; then
	cp "$REPO_ROOT/session/scripts"/* "$INCLUDE_DIR/usr/bin/"
fi

# Copy all shell scripts (asura-app, asura-cmd-*, asura-summon, etc.) to /usr/bin/
if [ -d "$REPO_ROOT/shell/scripts" ]; then
	cp "$REPO_ROOT/shell/scripts"/* "$INCLUDE_DIR/usr/bin/"
fi

# Copy all system helpers (asura-volume, asura-restart-audio, asura-hw-backlight, etc.)
for hdir in audio display input perf pkg; do
	if [ -d "$REPO_ROOT/system/helpers/$hdir" ]; then
		for f in "$REPO_ROOT/system/helpers/$hdir"/*; do
			if [ -f "$f" ] && [ "${f##*.}" != "sh" -o "${f##*/}" = "asura-installer.sh" ]; then
				cp "$f" "$INCLUDE_DIR/usr/bin/"
			fi
		done
	fi
done
cp "$REPO_ROOT/system/helpers/prime-run" "$INCLUDE_DIR/usr/bin/prime-run"
cp "$REPO_ROOT/system/helpers/fan-profile" "$INCLUDE_DIR/usr/bin/fan-profile"
cp "$REPO_ROOT/installation/asura-installer.sh" "$INCLUDE_DIR/usr/bin/asura-installer"
chmod 755 "$INCLUDE_DIR/usr/bin/asura-installer"
touch "$INCLUDE_DIR/etc/asura-live"



# Copy GLib GSettings schemas
mkdir -p "$INCLUDE_DIR/usr/share/glib-2.0/schemas"
cp "$REPO_ROOT/shell"/org.asura.*.gschema.xml "$INCLUDE_DIR/usr/share/glib-2.0/schemas/"

# Copy applications .desktop entries
mkdir -p "$INCLUDE_DIR/usr/share/applications"
if [ -d "$REPO_ROOT/session/applications" ]; then
	cp "$REPO_ROOT/session/applications"/*.desktop "$INCLUDE_DIR/usr/share/applications/" 2>/dev/null || true
fi
# Install Zed code editor desktop entry on ISO
cat > "$INCLUDE_DIR/usr/share/applications/dev.zed.Zed.desktop" <<'EOF'
[Desktop Entry]
Name=Zed
Comment=A high-performance, multiplayer code editor.
GenericName=Text Editor
Exec=zed %F
Icon=zed
Type=Application
StartupNotify=true
StartupWMClass=dev.zed.Zed
Categories=TextEditor;Development;IDE;
MimeType=text/plain;text/x-chdr;text/x-csrc;text/x-c++hdr;text/x-c++src;text/x-rust;text/x-python;text/x-go;application/json;application/xml;text/markdown;
Keywords=Text;Editor;Development;
EOF

# Download and install official standalone Zed binary if not already cached
ZED_CACHE="$BUILD_DIR/zed"
mkdir -p "$ZED_CACHE"
if [ ! -f "$ZED_CACHE/zed-linux-x86_64.tar.gz" ]; then
	echo "build-iso-void: downloading Zed editor..."
	curl -sSL -o "$ZED_CACHE/zed-linux-x86_64.tar.gz" "https://github.com/zed-industries/zed/releases/latest/download/zed-linux-x86_64.tar.gz" || true
fi
if [ -f "$ZED_CACHE/zed-linux-x86_64.tar.gz" ]; then
	mkdir -p "$INCLUDE_DIR/usr/lib/zed" "$INCLUDE_DIR/usr/bin"
	tar -xzf "$ZED_CACHE/zed-linux-x86_64.tar.gz" --strip-components=1 -C "$INCLUDE_DIR/usr/lib/zed" 2>/dev/null || true
	if [ -x "$INCLUDE_DIR/usr/lib/zed/bin/zed" ]; then
		ln -sf /usr/lib/zed/bin/zed "$INCLUDE_DIR/usr/bin/zed"
		mkdir -p "$INCLUDE_DIR/usr/share/icons/hicolor"
		if [ -d "$INCLUDE_DIR/usr/lib/zed/share/icons/hicolor" ]; then
			cp -rn "$INCLUDE_DIR/usr/lib/zed/share/icons/hicolor"/* "$INCLUDE_DIR/usr/share/icons/hicolor/" 2>/dev/null || true
		fi
	fi
fi

# Download and install official standalone OpenCode CLI binary if not already cached
OPENCODE_CACHE="$BUILD_DIR/opencode"
mkdir -p "$OPENCODE_CACHE"
if [ ! -f "$OPENCODE_CACHE/opencode-linux-x64.tar.gz" ]; then
	echo "build-iso-void: downloading OpenCode CLI..."
	curl -sSL -o "$OPENCODE_CACHE/opencode-linux-x64.tar.gz" "https://github.com/anomalyco/opencode/releases/latest/download/opencode-linux-x64.tar.gz" || true
fi
if [ -f "$OPENCODE_CACHE/opencode-linux-x64.tar.gz" ]; then
	mkdir -p "$INCLUDE_DIR/usr/bin"
	tar -xzf "$OPENCODE_CACHE/opencode-linux-x64.tar.gz" -C "$INCLUDE_DIR/usr/bin" 2>/dev/null || true
	chmod 755 "$INCLUDE_DIR/usr/bin/opencode" 2>/dev/null || true
fi
if [ -d "$REPO_ROOT/apps/mimeapps" ]; then
	cp "$REPO_ROOT/apps/mimeapps"/asura-mimeapps.list "$INCLUDE_DIR/usr/share/applications/" 2>/dev/null || true
fi

# Copy Asura apps (ghostty, starship, zsh configurations)
if [ -d "$REPO_ROOT/apps" ]; then
	mkdir -p "$INCLUDE_DIR/usr/share/asura/apps"
	cp -r "$REPO_ROOT/apps"/* "$INCLUDE_DIR/usr/share/asura/apps/"
fi

# Copy Hyprland configuration, runit services, and shell tree
cp -r "$REPO_ROOT/session/hyprland"/* "$INCLUDE_DIR/usr/share/asura/hyprland/"
if [ -d "$REPO_ROOT/session/sddm" ]; then
	cp -r "$REPO_ROOT/session/sddm" "$INCLUDE_DIR/usr/share/asura/session/" 2>/dev/null || true
fi
cp -r "$REPO_ROOT/shell"/* "$INCLUDE_DIR/usr/share/asura/shell/"
cp -r "$REPO_ROOT/system/runit/user-services"/* "$INCLUDE_DIR/usr/share/asura/runit/services/"

# Stage native Void Linux Hyprland compositor stack and libraries
echo "build-iso-void: staging native Void Linux Hyprland stack..."
HYPR_CACHE="$BUILD_DIR/hyprland-pkgs"
mkdir -p "$HYPR_CACHE"

# Dynamically resolve latest Makrennel package filenames from repo metadata to prevent version drift
RESOLVED_PKGS=$(python3 - <<'PY' 2>/dev/null || true
import urllib.request, plistlib, subprocess
wanted = ['tomlplusplus', 'hyprutils', 'aquamarine', 'hyprlang', 'hyprcursor', 'hyprgraphics', 'libspng', 'sdbus-cpp', 'hyprland', 'xdg-desktop-portal-hyprland', 'hyprland-qtutils']
url = 'https://raw.githubusercontent.com/Makrennel/hyprland-void/repository-x86_64-glibc/x86_64-repodata'
try:
    req = urllib.request.Request(url, headers={'User-Agent': 'Asura-ISO-Builder'})
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

HYPR_PKGS="${RESOLVED_PKGS:-tomlplusplus-3.4.0_1.x86_64.xbps hyprutils-0.7.1_1.x86_64.xbps aquamarine-0.8.0_2.x86_64.xbps hyprlang-0.6.3_1.x86_64.xbps hyprcursor-0.1.12_1.x86_64.xbps hyprgraphics-0.1.3_2.x86_64.xbps libspng-0.7.4_1.x86_64.xbps sdbus-cpp-2.1.0_1.x86_64.xbps hyprland-0.49.0_1.x86_64.xbps xdg-desktop-portal-hyprland-1.3.9_2.x86_64.xbps hyprland-qtutils-0.1.4_1.x86_64.xbps}"
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
# libsystemd compatibility symlinks redirecting to Void's native libelogind
ln -sf libelogind.so.0 "$INCLUDE_DIR/usr/lib/libsystemd.so.0"
ln -sf libelogind.so.0 "$INCLUDE_DIR/usr/lib/libsystemd.so"
# libseat is installed natively by XBPS from packages-void.yaml; purge any foreign host binaries
rm -f "$INCLUDE_DIR/usr/lib/libseat.so"*
rm -rf "$INCLUDE_DIR/usr/lib/plymouth"

# Pre-trust Makrennel repository public key in /var/db/xbps/keys
mkdir -p "$INCLUDE_DIR/var/db/xbps/keys"
cat >"$INCLUDE_DIR/var/db/xbps/keys/30:93:47:9e:1f:55:c5:c5:07:b1:9a:e6:8b:10:5b:4b.plist" <<'KEYEOF'
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
cp "$INCLUDE_DIR/var/db/xbps/keys/30:93:47:9e:1f:55:c5:c5:07:b1:9a:e6:8b:10:5b:4b.plist" \
   "$INCLUDE_DIR/var/db/xbps/keys/00649994a2d04d020cdb1119ff53e712be6b5c3e726accfb04c664d1200d748d.plist"
cp "$INCLUDE_DIR/var/db/xbps/keys/30:93:47:9e:1f:55:c5:c5:07:b1:9a:e6:8b:10:5b:4b.plist" \
   "$INCLUDE_DIR/var/db/xbps/keys/1293d0c27380d795a88fdbd007356dae84090282b5bd4123678ee9a682d06dd3.plist"

# Ensure foreign host quickshell binary is never copied into Void rootfs;
# native quickshell package is installed via packages-void.yaml
rm -f "$INCLUDE_DIR/usr/bin/quickshell"

# Copy udev power rules (99-asura-power.rules, 80-nvidia-pm.rules, 99-battery-threshold.rules)
cp "$REPO_ROOT/system/udev"/*.rules "$INCLUDE_DIR/etc/udev/rules.d/"
if [ -d "$REPO_ROOT/system/modprobe.d" ]; then
	cp "$REPO_ROOT/system/modprobe.d"/*.conf "$INCLUDE_DIR/etc/modprobe.d/"
fi
cp "$REPO_ROOT/system/sysctl.d/99-asura-performance.conf" "$INCLUDE_DIR/etc/sysctl.d/99-asura-performance.conf"
cp "$REPO_ROOT/session/turnstile/turnstile.conf" "$INCLUDE_DIR/etc/turnstile/turnstile.conf"
cp "$REPO_ROOT/system/xbps.d/asura.conf" "$INCLUDE_DIR/etc/xbps.d/asura.conf"
mkdir -p "$ROOTFS/etc/xbps.d"
cp "$REPO_ROOT/system/xbps.d/asura.conf" "$ROOTFS/etc/xbps.d/asura.conf" 2>/dev/null || true

# Default environment for seatd & Wayland
cat >"$INCLUDE_DIR/etc/environment" <<'EOF'
LIBSEAT_BACKEND=seatd
XDG_SESSION_TYPE=wayland
QT_QPA_PLATFORM=wayland
GDK_BACKEND=wayland
MOZ_ENABLE_WAYLAND=1
EOF

# Build and copy Asura Go control binaries into include tree
echo "build-iso-void: building Asura Go binaries..."
(
	export GOWORK=off
	cd "$REPO_ROOT/desktop"
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/asura" ./cmd/asura
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/asura-sysd" ./cmd/asura-sysd
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/asura-userd" ./cmd/asura-userd
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/asura-hubd" ./cmd/asura-hubd
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/asura-shell" ./cmd/asura-shell
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/asura-wallpaper" ./cmd/asura-wallpaper
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/apm" ./cmd/apm
	# Build Asura Store (prioritizing standalone repository https://github.com/ASURA-OS/asura-store)
	STORE_SRC="${ASURA_STORE_SRC:-}"
	if [ -z "$STORE_SRC" ]; then
		if [ -d "$REPO_ROOT/../asura-store" ]; then
			STORE_SRC="$REPO_ROOT/../asura-store"
		elif [ -d "/home/mohit/code/asura-store" ]; then
			STORE_SRC="/home/mohit/code/asura-store"
		elif [ -d "/home/mohit/code/asura/asura-store" ]; then
			STORE_SRC="/home/mohit/code/asura/asura-store"
		fi
	fi
	if [ -n "$STORE_SRC" ] && [ -d "$STORE_SRC/cmd/asura-store" ]; then
		echo "build-iso-void: building asura-store from standalone repo ($STORE_SRC)..."
		( cd "$STORE_SRC/cmd/asura-store" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$INCLUDE_DIR/usr/bin/asura-store" . )
		if [ -d "$STORE_SRC/catalog" ]; then
			mkdir -p "$INCLUDE_DIR/usr/share/asura/store/catalog"
			cp -r "$STORE_SRC/catalog"/* "$INCLUDE_DIR/usr/share/asura/store/catalog/" 2>/dev/null || true
		fi
	else
		echo "build-iso-void: building asura-store from desktop tree..."
		CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/asura-store" ./cmd/asura-store
	fi

	# Build Asura Resource Monitor (prioritizing standalone repository https://github.com/ASURA-OS/asura-sysmon)
	SYSMON_SRC="${ASURA_SYSMON_SRC:-}"
	if [ -z "$SYSMON_SRC" ]; then
		if [ -d "$REPO_ROOT/../asura-sysmon" ]; then
			SYSMON_SRC="$REPO_ROOT/../asura-sysmon"
		elif [ -d "/home/mohit/code/asura-sysmon" ]; then
			SYSMON_SRC="/home/mohit/code/asura-sysmon"
		elif [ -d "/home/mohit/code/asura/asura-sysmon" ]; then
			SYSMON_SRC="/home/mohit/code/asura/asura-sysmon"
		fi
	fi
	if [ -n "$SYSMON_SRC" ] && [ -d "$SYSMON_SRC/cmd/asura-sysmon" ]; then
		echo "build-iso-void: building asura-sysmon from standalone repo ($SYSMON_SRC)..."
		( cd "$SYSMON_SRC/cmd/asura-sysmon" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$INCLUDE_DIR/usr/bin/asura-sysmon" . )
		if [ -d "$SYSMON_SRC/ui" ]; then
			mkdir -p "$INCLUDE_DIR/usr/share/asura/sysmon"
			cp -r "$SYSMON_SRC/ui"/* "$INCLUDE_DIR/usr/share/asura/sysmon/" 2>/dev/null || true
		fi
	else
		echo "build-iso-void: building asura-sysmon from desktop tree..."
		CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/asura-sysmon" ./cmd/asura-sysmon
		mkdir -p "$INCLUDE_DIR/usr/share/asura/sysmon"
		cp -r "$REPO_ROOT/shell/sysmon"/* "$INCLUDE_DIR/usr/share/asura/sysmon/" 2>/dev/null || true
	fi
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/asura-vm-mon" ./cmd/asura-vm/mon
	CGO_ENABLED=0 go build -o "$INCLUDE_DIR/usr/bin/asura-vm-ssh" ./cmd/asura-vm/remote
	cp -f "$REPO_ROOT/desktop/cmd/asura-vm/asura-vmd" "$INCLUDE_DIR/usr/bin/asura-vmd"
)
find "$INCLUDE_DIR/usr/bin" -type f -exec chmod 0755 {} +
# Symlinks
ln -sf asura "$INCLUDE_DIR/usr/bin/os-ctl"

# Pre-seed user skeleton configuration and XDG user directories
mkdir -p "$INCLUDE_DIR/etc/skel/.config/asura" \
         "$INCLUDE_DIR/etc/skel/.config/ghostty" \
         "$INCLUDE_DIR/etc/skel/.config/fastfetch" \
         "$INCLUDE_DIR/etc/skel/Desktop" \
         "$INCLUDE_DIR/etc/skel/Documents" \
         "$INCLUDE_DIR/etc/skel/Downloads" \
         "$INCLUDE_DIR/etc/skel/Music" \
         "$INCLUDE_DIR/etc/skel/Pictures" \
         "$INCLUDE_DIR/etc/skel/Videos"
cp -a "$INCLUDE_DIR/usr/share/asura"/* "$INCLUDE_DIR/etc/skel/.config/asura/"

# Pre-seed Ghostty terminal configuration in skeleton
cat >"$INCLUDE_DIR/etc/skel/.config/ghostty/config" <<'EOF'
config-file = ?~/.config/asura/apps/ghostty/config
working-directory = home
EOF

# Pre-seed zsh configuration in skeleton
cat >"$INCLUDE_DIR/etc/skel/.zshrc" <<'EOF'
# Asura OS Zsh configuration
[ -r "$HOME/.config/asura/apps/zsh/asura.zsh" ] && . "$HOME/.config/asura/apps/zsh/asura.zsh"
EOF

# Pre-seed fastfetch ASCII logo and configuration
mkdir -p "$INCLUDE_DIR/usr/share/asura/branding" "$INCLUDE_DIR/etc/skel/.config/fastfetch"
if [ -f "$REPO_ROOT/public/branding/asura-logo.txt" ]; then
	cp "$REPO_ROOT/public/branding/asura-logo.txt" "$INCLUDE_DIR/usr/share/asura/branding/asura-logo.txt"
	cp "$REPO_ROOT/public/branding/asura-logo.txt" "$INCLUDE_DIR/usr/share/asura/branding/fastfetch-ascii.txt"
fi
if [ -f "$REPO_ROOT/public/branding/asura-logo-mono.txt" ]; then
	cp "$REPO_ROOT/public/branding/asura-logo-mono.txt" "$INCLUDE_DIR/usr/share/asura/branding/asura-logo-mono.txt"
fi
if [ -f "$REPO_ROOT/public/branding/fastfetch-emblem.png" ]; then
	cp "$REPO_ROOT/public/branding/fastfetch-emblem.png" "$INCLUDE_DIR/usr/share/asura/branding/fastfetch-emblem.png"
fi

cat >"$INCLUDE_DIR/etc/skel/.config/fastfetch/config.jsonc" <<'EOF'
{
  "$schema": "https://github.com/fastfetch-cli/fastfetch/raw/dev/doc/json_schema.json",
  "logo": {
    "type": "file-raw",
    "source": "/usr/share/asura/branding/asura-logo.txt",
    "padding": {
      "left": 2,
      "right": 4,
      "top": 1
    }
  },
  "display": {
    "separator": " ── ",
    "color": {
      "keys": "38;2;219;20;49"
    }
  },
  "modules": [
    {
      "format": "\u001b[38;2;219;20;49m■\u001b[0m \u001b[1;38;2;240;240;240mASURA OS\u001b[0m",
      "type": "custom"
    },
    "break",
    {
      "format": "\u001b[38;2;219;20;49m──\u001b[0m \u001b[1;38;2;240;240;240mSYSTEM\u001b[0m \u001b[38;2;80;80;80m──────────────────────────\u001b[0m",
      "type": "custom"
    },
    {
      "key": "OS",
      "type": "os"
    },
    {
      "key": "KERNEL",
      "type": "kernel"
    },
    {
      "key": "UPTIME",
      "type": "uptime"
    },
    {
      "key": "PACKAGES",
      "type": "packages"
    },
    {
      "key": "SHELL",
      "type": "shell"
    },
    {
      "key": "WM",
      "type": "wm"
    },
    "break",
    {
      "format": "\u001b[38;2;219;20;49m──\u001b[0m \u001b[1;38;2;240;240;240mHARDWARE\u001b[0m \u001b[38;2;80;80;80m────────────────────────\u001b[0m",
      "type": "custom"
    },
    {
      "key": "HOST",
      "type": "host"
    },
    {
      "key": "CPU",
      "type": "cpu"
    },
    {
      "key": "GPU",
      "type": "gpu"
    },
    {
      "key": "MEMORY",
      "type": "memory"
    },
    {
      "key": "DISPLAY",
      "type": "display"
    },
    "break",
    {
      "type": "colors",
      "symbol": "circle"
    }
  ]
}
EOF

# Stage system runit services (asura-sysd, scx-lavd, ananicy-cpp)
mkdir -p "$INCLUDE_DIR/etc/sv/asura-sysd"
for svc_dir in "$REPO_ROOT/system/runit/services"/*; do
	if [ -d "$svc_dir" ]; then
		svc_name=$(basename "$svc_dir")
		mkdir -p "$INCLUDE_DIR/etc/sv/$svc_name"
		cp -r "$svc_dir"/* "$INCLUDE_DIR/etc/sv/$svc_name/"
		chmod +x "$INCLUDE_DIR/etc/sv/$svc_name/run" 2>/dev/null || true
	fi
done

# Stage Layer 3: zramen zstd RAM compression configuration
mkdir -p "$INCLUDE_DIR/etc/sv/zramen"
cat >"$INCLUDE_DIR/etc/sv/zramen/conf" <<'EOF'
export ZRAM_COMP_ALGORITHM=zstd
export ZRAM_SIZE=50
export ZRAM_PRIORITY=32767
EOF

# Stage Layer 3: mimalloc global lock-free allocator preload
echo "/usr/lib/libmimalloc.so.3" > "$INCLUDE_DIR/etc/ld.so.preload"

# Stage Layer 3: ananicy-cpp auto-nice rules for compilers, scanners, UI, and games
mkdir -p "$INCLUDE_DIR/etc/ananicy.d" "$INCLUDE_DIR/etc/ananicy-cpp"
if [ -d "$REPO_ROOT/system/ananicy" ]; then
	cp "$REPO_ROOT/system/ananicy"/* "$INCLUDE_DIR/etc/ananicy.d/" 2>/dev/null || true
	cp "$REPO_ROOT/system/ananicy"/* "$INCLUDE_DIR/etc/ananicy-cpp/" 2>/dev/null || true
fi

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

# Clean up void-mklive bootstrap repository flags and force-include plymouth and VM drivers in initramfs
if [ -f "$MKLIVE_DIR/mklive.sh" ]; then
	sed -i 's/VOID_LIVE/ASURA_LIVE/g' "$MKLIVE_DIR/mklive.sh"
	sed -i 's/--force-add "vmklive autoinstaller"/--force-add "vmklive autoinstaller plymouth"/g' "$MKLIVE_DIR/mklive.sh"
	sed -i 's/--add-drivers "ahci"/--add-drivers "ahci vboxvideo vmwgfx virtio_gpu bochs qxl simpledrm"/g' "$MKLIVE_DIR/mklive.sh"
	sed -i 's|--repository=https://repo-default.voidlinux.org/current/musl||g' "$MKLIVE_DIR/mklive.sh"
	sed -i "s|https://repo-default.voidlinux.org/current|$MIRROR|g" "$MKLIVE_DIR/mklive.sh"
	sed -i 's/loglevel=4/loglevel=4 quiet splash rd.plymouth=1 plymouth.enable=1 console=tty0/g' "$MKLIVE_DIR/mklive.sh"
fi
if [ -f "$MKLIVE_DIR/lib.sh" ]; then
	sed -i 's|--repository=https://repo-default.voidlinux.org/current/musl||g' "$MKLIVE_DIR/lib.sh"
	sed -i "s|https://repo-default.voidlinux.org/current|$MIRROR|g" "$MKLIVE_DIR/lib.sh"
fi

# Patch dracut vmklive module to use asura-os hostname, asura user, and include _seatd/input groups
if [ -f "$MKLIVE_DIR/dracut/vmklive/adduser.sh" ]; then
	sed -i 's/echo void-live >/echo asura-os >/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
	sed -i 's/-G audio,video,wheel/-G audio,video,wheel,_seatd,input,tty,storage,network/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
	sed -i 's/USERNAME=anon/USERNAME=asura/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
	sed -i 's/voidlinux/asura/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
	sed -i 's/99-void-live/99-asura-live/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
	sed -i 's/void-live.rules/asura-live.rules/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
	sed -i 's/-s \/bin\/bash/-s \/bin\/zsh/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
	sed -i 's/chroot ${NEWROOT} useradd/chroot ${NEWROOT} id "$USERNAME" >\/dev\/null 2>\&1 || chroot ${NEWROOT} useradd/g' "$MKLIVE_DIR/dracut/vmklive/adduser.sh"
fi

# Replace cluttered multi-option isolinux menu with a clean, streamlined Asura OS boot menu
if [ -d "$MKLIVE_DIR/isolinux" ]; then
	cat >"$MKLIVE_DIR/isolinux/isolinux.cfg.in" <<'BOOTCFG'
UI vesamenu.c32
PROMPT 0
TIMEOUT 30
ONTIMEOUT linux

MENU TABMSG Press ENTER to boot or TAB to edit boot options
MENU AUTOBOOT Asura OS starts in # second{,s}...
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
MENU LABEL Asura OS (Live Session)
KERNEL /boot/vmlinuz
APPEND initrd=/boot/initrd root=live:CDLABEL=ASURA_LIVE init=/sbin/init ro rd.luks=0 rd.md=0 rd.dm=0 vconsole.unicode=1 vconsole.keymap=@@KEYMAP@@ locale.LANG=@@LOCALE@@ @@BOOT_CMDLINE@@

LABEL linuxram
MENU LABEL Asura OS (Copy to RAM)

KERNEL /boot/vmlinuz
APPEND initrd=/boot/initrd root=live:CDLABEL=ASURA_LIVE init=/sbin/init ro rd.luks=0 rd.md=0 rd.dm=0 vconsole.unicode=1 vconsole.keymap=@@KEYMAP@@ locale.LANG=@@LOCALE@@ @@BOOT_CMDLINE@@ rd.live.ram

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

# Enable core system runit services (Layer 1 seatd, Layer 3 scx-lavd, ananicy-cpp, zramen)
mkdir -p "$ROOTFS/etc/runit/runsvdir/default"
for svc in dbus seatd udevd NetworkManager acpid turnstiled tlp zramen asura-sysd asura-boot-guard; do
	if [ -d "$ROOTFS/etc/sv/$svc" ]; then
		ln -sf "/etc/sv/$svc" "$ROOTFS/etc/runit/runsvdir/default/$svc"
	fi
done
if [ -x "$ROOTFS/usr/bin/scx_lavd" ] && [ -d "$ROOTFS/etc/sv/scx-lavd" ]; then
	ln -sf /etc/sv/scx-lavd "$ROOTFS/etc/runit/runsvdir/default/scx-lavd"
fi
if [ -x "$ROOTFS/usr/bin/ananicy-cpp" ] && [ -d "$ROOTFS/etc/sv/ananicy-cpp" ]; then
	ln -sf /etc/sv/ananicy-cpp "$ROOTFS/etc/runit/runsvdir/default/ananicy-cpp"
fi

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
for user in anon asura; do
	if chroot "$ROOTFS" id "$user" >/dev/null 2>&1; then
		chroot "$ROOTFS" usermod -aG _seatd,video,audio,input,wheel,storage,network "$user" || true
	fi
done

# Ensure user asura exists with zsh login shell and configure autologin
if ! chroot "$ROOTFS" id asura >/dev/null 2>&1; then
	chroot "$ROOTFS" useradd -m -s /bin/zsh -U -G _seatd,video,audio,input,wheel,storage,network asura || true
else
	chroot "$ROOTFS" usermod -s /bin/zsh asura || true
fi
echo "asura:asura" | chroot "$ROOTFS" chpasswd 2>/dev/null || true

# Reset root and anon passwords to eliminate upstream voidlinux defaults
echo "root:asura" | chroot "$ROOTFS" chpasswd 2>/dev/null || true
if chroot "$ROOTFS" id anon >/dev/null 2>&1; then
	echo "anon:asura" | chroot "$ROOTFS" chpasswd 2>/dev/null || true
fi

# Neutralize upstream installer and docs commands
if [ -f "$ROOTFS/usr/bin/void-installer" ]; then
	rm -f "$ROOTFS/usr/bin/void-installer"
	ln -sf /usr/bin/asura-installer "$ROOTFS/usr/bin/void-installer"
fi

cat >"$ROOTFS/usr/bin/void-docs" <<'DOCS'
#!/bin/sh
echo "Asura OS documentation is available at: https://asuraos.dev"
DOCS
chmod 0755 "$ROOTFS/usr/bin/void-docs"

# Sudo permissions for wheel group (strict 0750 dir, 0440 file)
chmod 0750 "$ROOTFS/etc/sudoers.d"
echo '%wheel ALL=(ALL:ALL) NOPASSWD: ALL' > "$ROOTFS/etc/sudoers.d/wheel"
chmod 0440 "$ROOTFS/etc/sudoers.d/wheel"
chmod 0440 "$ROOTFS/etc/sudoers.d"/* 2>/dev/null || true
grep -q '%wheel ALL=(ALL:ALL) NOPASSWD: ALL' "$ROOTFS/etc/sudoers" 2>/dev/null || echo '%wheel ALL=(ALL:ALL) NOPASSWD: ALL' >> "$ROOTFS/etc/sudoers"

# Compile GSettings schemas for Asura Shell and Settings
if command -v glib-compile-schemas >/dev/null 2>&1; then
	glib-compile-schemas "$ROOTFS/usr/share/glib-2.0/schemas" 2>/dev/null || true
fi

# Enable automatic silent console login on tty1 for seamless graphical boot
mkdir -p "$ROOTFS/etc/sv/agetty-tty1" "$ROOTFS/home/asura"
echo 'GETTY_ARGS="--autologin asura --skip-login --nonewline --noissue"' > "$ROOTFS/etc/sv/agetty-tty1/conf"
touch "$ROOTFS/home/asura/.hushlogin"
touch "$INCLUDE_DIR/etc/skel/.hushlogin"
# Pre-materialize user configuration for asura user, skeleton, and XDG directories
mkdir -p "$ROOTFS/home/asura/.config/asura" "$ROOTFS/etc/skel/.config/asura" \
         "$ROOTFS/home/asura/.config/ghostty" "$ROOTFS/home/asura/.config/fastfetch" \
         "$ROOTFS/home/asura/Desktop" "$ROOTFS/home/asura/Documents" \
         "$ROOTFS/home/asura/Downloads" "$ROOTFS/home/asura/Music" \
         "$ROOTFS/home/asura/Pictures" "$ROOTFS/home/asura/Videos" \
         "$INCLUDE_DIR/etc/skel/Desktop"
if [ -f "$REPO_ROOT/session/applications/asura-installer.desktop" ]; then
	cp "$REPO_ROOT/session/applications/asura-installer.desktop" "$ROOTFS/home/asura/Desktop/"
	cp "$REPO_ROOT/session/applications/asura-installer.desktop" "$INCLUDE_DIR/etc/skel/Desktop/"
	chmod 755 "$ROOTFS/home/asura/Desktop/asura-installer.desktop" 2>/dev/null || true
	chmod 755 "$INCLUDE_DIR/etc/skel/Desktop/asura-installer.desktop" 2>/dev/null || true
fi
if [ -f "$REPO_ROOT/session/applications/asura-store.desktop" ]; then
	cp "$REPO_ROOT/session/applications/asura-store.desktop" "$ROOTFS/home/asura/Desktop/"
	cp "$REPO_ROOT/session/applications/asura-store.desktop" "$INCLUDE_DIR/etc/skel/Desktop/"
	chmod 755 "$ROOTFS/home/asura/Desktop/asura-store.desktop" 2>/dev/null || true
	chmod 755 "$INCLUDE_DIR/etc/skel/Desktop/asura-store.desktop" 2>/dev/null || true
fi
if [ -f "$REPO_ROOT/session/applications/asura-sysmon.desktop" ]; then
	mkdir -p "$INCLUDE_DIR/usr/share/applications"
	cp "$REPO_ROOT/session/applications/asura-sysmon.desktop" "$INCLUDE_DIR/usr/share/applications/"
	cp "$REPO_ROOT/session/applications/asura-sysmon.desktop" "$ROOTFS/usr/share/applications/" 2>/dev/null || true
fi
if [ -f "$REPO_ROOT/session/applications/opencode.desktop" ]; then
	mkdir -p "$INCLUDE_DIR/usr/share/applications"
	cp "$REPO_ROOT/session/applications/opencode.desktop" "$INCLUDE_DIR/usr/share/applications/"
	cp "$REPO_ROOT/session/applications/opencode.desktop" "$ROOTFS/usr/share/applications/" 2>/dev/null || true
fi
if [ -d "$ROOTFS/usr/share/asura" ]; then
	cp -a "$ROOTFS/usr/share/asura"/* "$ROOTFS/home/asura/.config/asura/"
	cp -a "$ROOTFS/usr/share/asura"/* "$ROOTFS/etc/skel/.config/asura/"
fi
cp -a "$ROOTFS/etc/skel/.config/ghostty"/* "$ROOTFS/home/asura/.config/ghostty/" 2>/dev/null || true
cp -a "$ROOTFS/etc/skel/.config/fastfetch"/* "$ROOTFS/home/asura/.config/fastfetch/" 2>/dev/null || true
cp -a "$ROOTFS/etc/skel/.zshrc" "$ROOTFS/home/asura/.zshrc" 2>/dev/null || true
chroot "$ROOTFS" su - asura -c "asura materialize" 2>/dev/null || true
chroot "$ROOTFS" su - asura -c "xdg-user-dirs-update" 2>/dev/null || true
chroot "$ROOTFS" chown -R asura:asura /home/asura 2>/dev/null || true

# Ensure Plymouth theme and configuration are active in ROOTFS
mkdir -p "$ROOTFS/usr/share/plymouth/themes"
ln -sf /usr/share/plymouth/themes/asura/asura.plymouth "$ROOTFS/usr/share/plymouth/themes/default.plymouth" 2>/dev/null || true
if [ -f "$ROOTFS/etc/plymouth/plymouthd.conf" ]; then
	sed -i 's/^Theme=.*/Theme=asura/' "$ROOTFS/etc/plymouth/plymouthd.conf"
fi

# Silence runit boot messages and sysctl dump while Plymouth splash is active
if [ -f "$ROOTFS/etc/runit/functions" ]; then
	cat >"$ROOTFS/etc/runit/functions" <<'FNEOF'
msg() {
	if command -v plymouth >/dev/null 2>&1 && plymouth --ping 2>/dev/null; then
		plymouth update --status="$*" 2>/dev/null || true
	else
		printf "=> %s\n" "$*"
	fi
}
msg_warn() {
	if command -v plymouth >/dev/null 2>&1 && plymouth --ping 2>/dev/null; then
		plymouth update --status="WARNING: $*" 2>/dev/null || true
	else
		printf "=> WARNING: %s\n" "$*"
	fi
}
msg_error() {
	if command -v plymouth >/dev/null 2>&1 && plymouth --ping 2>/dev/null; then
		plymouth update --status="ERROR: $*" 2>/dev/null || true
	else
		printf "=> ERROR: %s\n" "$*"
	fi
}
FNEOF
fi

if [ -f "$ROOTFS/etc/runit/core-services/08-sysctl.sh" ]; then
	sed -i 's/sysctl -p/sysctl -q -p/g' "$ROOTFS/etc/runit/core-services/08-sysctl.sh"
	sed -i 's/sysctl --system/sysctl -q --system/g' "$ROOTFS/etc/runit/core-services/08-sysctl.sh"
fi

# Overwrite OS identity inside ROOTFS
cat >"$ROOTFS/etc/os-release" <<'OSREL'
NAME="Asura OS"
PRETTY_NAME="Asura OS Beta 001"
ID=asura
ID_LIKE="linux"
VERSION_ID="0.1"
VERSION_CODENAME=beta-001
HOME_URL="https://asuraos.dev/"
SUPPORT_URL="https://asuraos.dev/support"
BUG_REPORT_URL="https://github.com/mohitjoer/Asura-OS/issues"
OSREL

# Branding replacement: Change Welcome to Void! to Welcome to Asura OS! in the boot sequence
for runit_file in "$ROOTFS/etc/runit/1" "$ROOTFS/etc/runit/2" "$ROOTFS/etc/runit/3" "$ROOTFS/etc/rc.local"; do
	if [ -f "$runit_file" ]; then
		sed -i 's/Welcome to Void!/Welcome to Asura OS!/g' "$runit_file"
		sed -i 's/Welcome to Void Linux!/Welcome to Asura OS!/g' "$runit_file"
		sed -i 's/Void Linux/Asura OS/g' "$runit_file"
	fi
done

# Configure fontconfig cache permissions and hardware battery threshold in /etc/rc.local
mkdir -p "$ROOTFS/var/cache/fontconfig" "$INCLUDE_DIR/var/cache/fontconfig"
chmod 1777 "$ROOTFS/var/cache/fontconfig" "$INCLUDE_DIR/var/cache/fontconfig" 2>/dev/null || true
mkdir -p "$ROOTFS/usr/share/fonts/TTF"
if [ -d "$REPO_ROOT/public/fonts" ]; then
	cp -L "$REPO_ROOT/public/fonts"/* "$ROOTFS/usr/share/fonts/TTF/" 2>/dev/null || true
fi
chroot "$ROOTFS" fc-cache -f 2>/dev/null || true
if [ -f "$ROOTFS/etc/rc.local" ]; then
	if ! grep -q 'charge_control_end_threshold' "$ROOTFS/etc/rc.local"; then
		echo 'for b in /sys/class/power_supply/BAT*/charge_control_end_threshold; do [ -f "$b" ] && echo 80 > "$b" 2>/dev/null || true; done' >> "$ROOTFS/etc/rc.local"
	fi
fi

# Set Asura OS hostname and network identity
printf 'asura-os\n' > "$ROOTFS/etc/hostname"
if [ -f "$ROOTFS/etc/rc.conf" ]; then
	sed -i 's/^#\?HOSTNAME=.*/HOSTNAME="asura-os"/' "$ROOTFS/etc/rc.conf"
	if ! grep -q '^HOSTNAME=' "$ROOTFS/etc/rc.conf"; then
		echo 'HOSTNAME="asura-os"' >> "$ROOTFS/etc/rc.conf"
	fi
fi
if [ -f "$ROOTFS/etc/hosts" ]; then
	sed -i 's/void-live/asura-os/g' "$ROOTFS/etc/hosts"
fi

# Update all TTY greeting messages (/etc/issue, /etc/issue.net, /etc/motd) to show Asura OS wordmark and logo
cat >"$ROOTFS/etc/issue" <<'ISSUE'
       #                    #         Asura OS Beta 001 (\m)
     %##                    ##%       Kernel \r on an \m (\l)
    ####   %%%%%%%%%%%%%%   ####
  ######% %%%%%%%%%%%%%%%% %######    Robust. Minimal. Kinetic.
%###########% %%%%%%%%  ###########%  Live Session: user 'asura' (nopasswd sudo)
 %###########          ###########%   Run 'asura install' or 'asura-installer' to install
     ########  @@@@@@  ########
   %% ########%      %######## %%%    https://asuraos.dev
   %%  %%##################%%% %%%
   %% ##     %########%     ## %%%
   %% ####%%%%%      %%%%%#### %%
       %####################%
      %%  %##############%  %%
       ### ############## %##
       %##  ############  ###
        ###  %        %  ###
         %################%
           %############%
             ##########

ISSUE

cat >"$ROOTFS/etc/issue.net" <<'ISSUENET'
       #                    #         Asura OS Beta 001
     %##                    ##%       https://asuraos.dev
    ####   %%%%%%%%%%%%%%   ####
  ######% %%%%%%%%%%%%%%%% %######    Robust. Minimal. Kinetic.
%###########% %%%%%%%%  ###########%  Live Session: user 'asura' (nopasswd sudo)
 %###########          ###########%   Run 'asura install' or 'asura-installer' to install
     ########  @@@@@@  ########
   %% ########%      %######## %%%
   %%  %%##################%%% %%%
   %% ##     %########%     ## %%%
   %% ####%%%%%      %%%%%#### %%
       %####################%
      %%  %##############%  %%
       ### ############## %##
       %##  ############  ###
        ###  %        %  ###
         %################%
           %############%
             ##########

ISSUENET

cat >"$ROOTFS/etc/motd" <<'MOTD'
       #                    #         Asura OS Beta 001
     %##                    ##%       https://asuraos.dev
    ####   %%%%%%%%%%%%%%   ####
  ######% %%%%%%%%%%%%%%%% %######    Robust. Minimal. Kinetic.
%###########% %%%%%%%%  ###########%  Live Session: user 'asura' (nopasswd sudo)
 %###########          ###########%   Run 'asura install' or 'asura-installer' to install
     ########  @@@@@@  ########
   %% ########%      %######## %%%
   %%  %%##################%%% %%%
   %% ##     %########%     ## %%%
   %% ####%%%%%      %%%%%#### %%
       %####################%
      %%  %##############%  %%
       ### ############## %##
       %##  ############  ###
        ###  %        %  ###
         %################%
           %############%
             ##########

MOTD


# Auto-start Asura Wayland desktop on tty1 login (with safe fallback if compositor exits)
mkdir -p "$ROOTFS/etc/profile.d"
cat >"$ROOTFS/etc/profile.d/asura-autologin.sh" <<'PROFILE'
if [ -z "$WAYLAND_DISPLAY" ] && [ -z "$DISPLAY" ] && [ "$(tty 2>/dev/null)" = "/dev/tty1" ]; then
	# Suppress text console cursor and clear screen to eliminate terminal flicker before GUI
	setterm -cursor off 2>/dev/null || true
	clear 2>/dev/null || true
	command -v xdg-user-dirs-update >/dev/null 2>&1 && xdg-user-dirs-update 2>/dev/null || true
	if [ ! -d "$HOME/.config/asura/shell" ] && command -v asura >/dev/null 2>&1; then
		asura materialize 2>/dev/null || true
	fi
	if ! /usr/share/asura/session/asura-session; then
		setterm -cursor on 2>/dev/null || true
		if command -v plymouth >/dev/null 2>&1; then
			sudo -n plymouth --quit 2>/dev/null || plymouth --quit 2>/dev/null || sudo -n killall -9 plymouthd 2>/dev/null || true
		fi
		echo
		echo "================================================================"
		if grep -q "No 3D enabled" "$HOME/.local/state/asura/session.log" 2>/dev/null || \
		   grep -q "failed to create dri2 screen" "$HOME/.local/state/asura/session.log" 2>/dev/null; then
			echo "  [Virtual Machine Display Notice]"
			echo "  VirtualBox / VMware 3D Acceleration is currently DISABLED."
			echo "  Hyprland Wayland compositor requires hardware 3D acceleration."
			echo
			echo "  To run the full Wayland graphical desktop in VirtualBox:"
			echo "    1. Power off this virtual machine."
			echo "    2. Go to VirtualBox Settings -> Display -> Screen."
			echo "    3. Check 'Enable 3D Acceleration' and set Video Memory >= 128MB."
			echo
		else
			echo "  Asura graphical desktop session exited."
			echo "  Log available at: ~/.local/state/asura/session.log"
			echo
		fi
		echo "  To launch the installer directly, run: sudo asura install"
		echo "  To retry launching the desktop, run: asura-session"
		echo "================================================================"
		echo
	fi
fi
PROFILE
chmod 0755 "$ROOTFS/etc/profile.d/asura-autologin.sh"
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
		-k linux \
		-r "$MIRROR" \
		-p "$PACKAGES" \
		-I "$INCLUDE_DIR" \
		-i gzip \
		-s zstd \
		-S "dbus seatd NetworkManager zramen" \
		-C "loglevel=4 quiet splash rd.plymouth=1 plymouth.enable=1 console=tty0 live.user=asura live.autologin nvidia-drm.modeset=1 nvidia-drm.fbdev=1 NVreg_DynamicPowerManagement=0x02 nowatchdog mitigations=off preempt=full" \
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
