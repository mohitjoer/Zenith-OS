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
	"$INCLUDE_DIR/usr/bin" \
	"$INCLUDE_DIR/usr/share/zenith/branding" \
	"$INCLUDE_DIR/usr/share/zenith/session" \
	"$INCLUDE_DIR/usr/share/zenith/runit/services" \
	"$INCLUDE_DIR/usr/share/zenith/hyprland" \
	"$INCLUDE_DIR/usr/share/zenith/shell" \
	"$INCLUDE_DIR/usr/share/wayland-sessions"

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

# Copy session files and runit user services
cp "$REPO_ROOT/session/zenith-session" "$INCLUDE_DIR/usr/share/zenith/session/zenith-session"
cp "$REPO_ROOT/session/zenith-activate" "$INCLUDE_DIR/usr/share/zenith/session/zenith-activate"
cp "$REPO_ROOT/session/runit/zenith-user-services" "$INCLUDE_DIR/usr/share/zenith/session/zenith-user-services"
cp "$REPO_ROOT/session/runit/zenith-user-services" "$INCLUDE_DIR/usr/bin/zenith-user-services"
cp "$REPO_ROOT/system/helpers/prime-run" "$INCLUDE_DIR/usr/bin/prime-run"
cp "$REPO_ROOT/system/helpers/fan-profile" "$INCLUDE_DIR/usr/bin/fan-profile"
cp "$REPO_ROOT/installation/zenith-installer.sh" "$INCLUDE_DIR/usr/bin/zenith-installer"
mkdir -p "$INCLUDE_DIR/usr/share/applications"
cp "$REPO_ROOT/session/applications/zenith-installer.desktop" "$INCLUDE_DIR/usr/share/applications/zenith-installer.desktop"
cp -r "$REPO_ROOT/system/runit/user-services"/* "$INCLUDE_DIR/usr/share/zenith/runit/services/"
cp -r "$REPO_ROOT/session/hyprland"/* "$INCLUDE_DIR/usr/share/zenith/hyprland/"
cp -r "$REPO_ROOT/shell"/* "$INCLUDE_DIR/usr/share/zenith/shell/"
chmod +x "$INCLUDE_DIR/usr/share/zenith/session"/* "$INCLUDE_DIR/usr/bin/zenith-user-services" "$INCLUDE_DIR/usr/bin/prime-run" "$INCLUDE_DIR/usr/bin/fan-profile" "$INCLUDE_DIR/usr/bin/zenith-installer"

# Copy udev power rules (99-zenith-power.rules, 80-nvidia-pm.rules, 99-battery-threshold.rules)
cp "$REPO_ROOT/system/udev"/*.rules "$INCLUDE_DIR/etc/udev/rules.d/"
if [ -d "$REPO_ROOT/system/modprobe.d" ]; then
	cp "$REPO_ROOT/system/modprobe.d"/*.conf "$INCLUDE_DIR/etc/modprobe.d/"
fi
cp "$REPO_ROOT/system/sysctl.d/99-zenith-performance.conf" "$INCLUDE_DIR/etc/sysctl.d/99-zenith-performance.conf"
cp "$REPO_ROOT/session/turnstile/turnstile.conf" "$INCLUDE_DIR/etc/turnstile/turnstile.conf"
cp "$REPO_ROOT/system/xbps.d/zenith.conf" "$INCLUDE_DIR/etc/xbps.d/zenith.conf"

# Desktop session entry for display managers
cat >"$INCLUDE_DIR/usr/share/wayland-sessions/zenith.desktop" <<'EOF'
[Desktop Entry]
Name=Zenith
Comment=Zenith OS Desktop Environment
Exec=/usr/share/zenith/session/zenith-session
Type=Application
DesktopNames=Zenith
EOF

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
)
chmod 0755 "$INCLUDE_DIR/usr/bin"/zenith* "$INCLUDE_DIR/usr/bin"/zpm
# Symlink os-ctl to zenith for role switcher parity
ln -sf zenith "$INCLUDE_DIR/usr/bin/os-ctl"

# Stage system runit services (zenith-sysd)
mkdir -p "$INCLUDE_DIR/etc/sv/zenith-sysd"
cp "$REPO_ROOT/system/runit/services/zenith-sysd/run" "$INCLUDE_DIR/etc/sv/zenith-sysd/run"
chmod +x "$INCLUDE_DIR/etc/sv/zenith-sysd/run"

# Extract package list from system/packages-void.yaml
PACKAGES=$(python3 - "$REPO_ROOT/system/packages-void.yaml" "$MIRROR" <<'PY'
import sys, urllib.request, tarfile, io, re, yaml
from pathlib import Path

manifest = yaml.safe_load(Path(sys.argv[1]).read_text())
mirror = sys.argv[2] if len(sys.argv) > 2 else "https://repo-default.voidlinux.org/current"

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

avail_keys = None
try:
    url = f"{mirror.rstrip('/')}/x86_64-repodata"
    req = urllib.request.Request(url, headers={"User-Agent": "Zenith-OS/1.0"})
    with urllib.request.urlopen(req, timeout=10) as resp:
        with tarfile.open(fileobj=io.BytesIO(resp.read()), mode="r:*") as tar:
            content = tar.extractfile("index.plist").read().decode("utf-8", errors="ignore")
            avail_keys = set(re.findall(r'<key>([^<]+)</key>', content))
except Exception:
    pass

if avail_keys:
    out = [p for p in all_requested if p in avail_keys]
else:
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

# Clean up void-mklive bootstrap repository flags so glibc x86_64 targets don't query 404 musl repodata
if [ -f "$MKLIVE_DIR/mklive.sh" ]; then
	sed -i 's|--repository=https://repo-default.voidlinux.org/current/musl||g' "$MKLIVE_DIR/mklive.sh"
	sed -i "s|https://repo-default.voidlinux.org/current|$MIRROR|g" "$MKLIVE_DIR/mklive.sh"
fi

# Ensure XBPS tools are available (if running on a non-Void host, fetch xbps-static)
if ! command -v xbps-install >/dev/null 2>&1; then
	echo "build-iso-void: xbps-install not found on host, setting up xbps-static..."
	XBPS_STATIC_DIR="$BUILD_DIR/xbps-static"
	if [ ! -x "$XBPS_STATIC_DIR/usr/bin/xbps-install" ]; then
		mkdir -p "$XBPS_STATIC_DIR"
		curl -sSL "https://repo-default.voidlinux.org/static/xbps-static-latest.x86_64-musl.tar.xz" | tar -xJ -C "$XBPS_STATIC_DIR"
	fi
	export PATH="$XBPS_STATIC_DIR/usr/bin:$PATH"
fi

# Post-setup script to configure users and seat permissions inside ROOTFS
POSTSETUP_SCRIPT="$BUILD_DIR/postsetup.sh"
cat >"$POSTSETUP_SCRIPT" <<'EOF'
#!/bin/sh
ROOTFS="$1"

# Enable core system runit services
mkdir -p "$ROOTFS/etc/runit/runsvdir/default"
for svc in dbus seatd elogind udevd NetworkManager acpid turnstiled tlp zramen zenith-sysd; do
	if [ -d "$ROOTFS/etc/sv/$svc" ]; then
		ln -sf "/etc/sv/$svc" "$ROOTFS/etc/runit/runsvdir/default/$svc"
	fi
done

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

# Sudo permissions for wheel group
echo '%wheel ALL=(ALL:ALL) NOPASSWD: ALL' > "$ROOTFS/etc/sudoers.d/wheel"
chmod 0440 "$ROOTFS/etc/sudoers.d/wheel"

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
		-S "dbus seatd elogind NetworkManager zramen" \
		-C "nvidia-drm.modeset=1 nvidia-drm.fbdev=1 NVreg_DynamicPowerManagement=0x02 nowatchdog mitigations=off preempt=full quiet splash" \
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
