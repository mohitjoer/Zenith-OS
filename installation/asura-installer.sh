#!/bin/sh
# Asura OS Guided System Installer
# Installs Asura OS onto target drive.
set -eu

BOLD="\033[1m"
DIM="\033[2m"
WHITE="\033[1;37m"
GREEN="\033[1;32m"
RED="\033[1;31m"
RESET="\033[0m"

clear 2>/dev/null || true

cat <<EOF
${WHITE}┌──────────────────────────────────────────────────────────┐
│                       ${BOLD}天  ASURA OS${RESET}${WHITE}                       │
│               Guided System Installer                    │
└──────────────────────────────────────────────────────────┘${RESET}

EOF

if [ "$(id -u)" -ne 0 ]; then
	printf "${RED}asura-installer: must be run as root (e.g. sudo asura-installer or sudo asura install)${RESET}\n" >&2
	exit 1
fi

# Pre-flight: Check UEFI firmware
if [ ! -d /sys/firmware/efi ]; then
	printf "${DIM}Boot Mode: Legacy BIOS (Warning: Asura OS recommends UEFI for full Wayland power features)${RESET}\n\n"
	IS_UEFI=0
else
	printf "${GREEN}Boot Mode: UEFI (Verified)${RESET}\n\n"
	IS_UEFI=1
fi

# Disk Selection
printf "${WHITE}${BOLD}Available Drives:${RESET}\n"
printf "${DIM}──────────────────────────────────────────────────────────${RESET}\n"
lsblk -d -n -o NAME,SIZE,MODEL,TYPE | grep -E 'disk' | while read -r line; do
	printf "  ${BOLD}/dev/%s${RESET}\n" "$line"
done
printf "${DIM}──────────────────────────────────────────────────────────${RESET}\n\n"

if [ $# -ge 1 ]; then
	TARGET_DISK="$1"
else
	printf "${WHITE}Select target disk to install Asura OS to (e.g. /dev/nvme0n1 or /dev/vda):${RESET} "
	read -r TARGET_DISK
fi

if [ ! -b "$TARGET_DISK" ]; then
	printf "${RED}Error: %s is not a valid block device.${RESET}\n" "$TARGET_DISK" >&2
	exit 1
fi

# Filesystem Selection
printf "\n${WHITE}${BOLD}Select Filesystem:${RESET}\n"
printf "  1) ${BOLD}Btrfs${RESET} ${DIM}(Recommended: zstd:1 compression, high-performance SSD storage)${RESET}\n"
printf "  2) ${BOLD}ext4${RESET}  ${DIM}(Standard Linux journaling filesystem)${RESET}\n"
printf "Choose filesystem [1/2] (default: 1): "
read -r FS_CHOICE || FS_CHOICE="1"
[ -z "$FS_CHOICE" ] && FS_CHOICE="1"

# Hostname & User Configuration
printf "\n${WHITE}${BOLD}System Identity:${RESET}\n"
printf "Enter hostname (default: asura-os): "
read -r HOSTNAME_INPUT || HOSTNAME_INPUT="asura-os"
[ -z "$HOSTNAME_INPUT" ] && HOSTNAME_INPUT="asura-os"


printf "Enter default username (default: asura): "
read -r USERNAME_INPUT || USERNAME_INPUT="asura"
[ -z "$USERNAME_INPUT" ] && USERNAME_INPUT="asura"

# Installation Summary & Confirmation
printf "\n${WHITE}┌──────────────────────────────────────────────────────────┐
│                   INSTALLATION SUMMARY                   │
├──────────────────────────────────────────────────────────┤${RESET}\n"
printf "${WHITE}│${RESET} Target Disk:  ${BOLD}%-42s${RESET} ${WHITE}│${RESET}\n" "$TARGET_DISK"
printf "${WHITE}│${RESET} Bootloader:   ${BOLD}%-42s${RESET} ${WHITE}│${RESET}\n" "GRUB EFI (UEFI 64-bit)"
if [ "$FS_CHOICE" = "2" ]; then
	printf "${WHITE}│${RESET} Filesystem:   ${BOLD}%-42s${RESET} ${WHITE}│${RESET}\n" "ext4 (defaults,noatime)"
else
	printf "${WHITE}│${RESET} Filesystem:   ${BOLD}%-42s${RESET} ${WHITE}│${RESET}\n" "Btrfs (compress=zstd:1,noatime)"
fi
printf "${WHITE}│${RESET} Hostname:     ${BOLD}%-42s${RESET} ${WHITE}│${RESET}\n" "$HOSTNAME_INPUT"
printf "${WHITE}│${RESET} User Account: ${BOLD}%-42s${RESET} ${WHITE}│${RESET}\n" "$USERNAME_INPUT (sudoers)"
printf "${WHITE}└──────────────────────────────────────────────────────────┘${RESET}\n\n"

printf "${RED}${BOLD}CRITICAL WARNING: ALL EXISTING DATA ON %s WILL BE DESTROYED!${RESET}\n" "$TARGET_DISK"
printf "${WHITE}Type 'YES' to proceed with disk partitioning and installation:${RESET} "
read -r CONFIRM
if [ "$CONFIRM" != "YES" ]; then
	printf "${DIM}Installation cancelled.${RESET}\n"
	exit 0
fi

printf "\n${WHITE}${BOLD}[1/6] Partitioning disk (%s)...${RESET}\n" "$TARGET_DISK"
# Wipe existing partition table signatures
wipefs -a "$TARGET_DISK" >/dev/null 2>&1 || true

# Create GPT partition table:
# Partition 1: 512MB EFI System Partition (ESP)
# Partition 2: Remainder as Linux Root
parted -s "$TARGET_DISK" mklabel gpt
parted -s "$TARGET_DISK" mkpart "ESP" fat32 1MiB 513MiB
parted -s "$TARGET_DISK" set 1 esp on
parted -s "$TARGET_DISK" mkpart "AsuraRoot" 513MiB 100%

# Resolve partition names (e.g. /dev/nvme0n1p1 vs /dev/vda1)
if [ -b "${TARGET_DISK}p1" ]; then
	PART_EFI="${TARGET_DISK}p1"
	PART_ROOT="${TARGET_DISK}p2"
else
	PART_EFI="${TARGET_DISK}1"
	PART_ROOT="${TARGET_DISK}2"
fi

sleep 2

printf "${WHITE}${BOLD}[2/6] Formatting partitions...${RESET}\n"
mkfs.vfat -F 32 -n "ASURA_ESP" "$PART_EFI" >/dev/null

TARGET_DIR="/mnt/asura-target"
mkdir -p "$TARGET_DIR"

if [ "$FS_CHOICE" != "2" ] && command -v mkfs.btrfs >/dev/null 2>&1; then
	printf "  Formatting root with Btrfs (compress=zstd:1)...\n"
	mkfs.btrfs -f -L "ASURA_ROOT" "$PART_ROOT" >/dev/null
	FS_TYPE="btrfs"
	FS_OPTS="defaults,noatime,compress=zstd:1"
	mount -o "$FS_OPTS" "$PART_ROOT" "$TARGET_DIR"
else
	printf "  Formatting root with ext4...\n"
	mkfs.ext4 -F -L "ASURA_ROOT" "$PART_ROOT" >/dev/null
	FS_TYPE="ext4"
	FS_OPTS="defaults,noatime"
	mount "$PART_ROOT" "$TARGET_DIR"
fi

printf "${WHITE}${BOLD}[3/6] Mounting target filesystems...${RESET}\n"
mkdir -p "$TARGET_DIR/boot/efi"
mount "$PART_EFI" "$TARGET_DIR/boot/efi"

printf "${WHITE}${BOLD}[4/6] Copying system files (rootfs) to target...${RESET}\n"
rsync -aAX --info=progress2 \
	--exclude='/dev/*' \
	--exclude='/proc/*' \
	--exclude='/sys/*' \
	--exclude='/tmp/*' \
	--exclude='/run/*' \
	--exclude='/mnt/*' \
	--exclude='/media/*' \
	--exclude='/lost+found' \
	--exclude='/var/cache/*' \
	/ "$TARGET_DIR/"

printf "${WHITE}${BOLD}[5/6] Configuring host, services, and hardware optimizations...${RESET}\n"
ROOT_UUID=$(blkid -s UUID -o value "$PART_ROOT")
EFI_UUID=$(blkid -s UUID -o value "$PART_EFI")

cat >"$TARGET_DIR/etc/fstab" <<EOF
# /etc/fstab: static file system information for Asura OS
UUID=$ROOT_UUID / $FS_TYPE $FS_OPTS 0 0
UUID=$EFI_UUID /boot/efi vfat defaults,noatime,umask=0077 0 2
tmpfs /tmp tmpfs defaults,nosuid,nodev 0 0
EOF

# Ensure live marker is removed on installed system so first-boot welcome tour triggers
rm -f "$TARGET_DIR/etc/asura-live"

# Set hostname
printf '%s\n' "$HOSTNAME_INPUT" > "$TARGET_DIR/etc/hostname"

# Ensure user account exists
if ! chroot "$TARGET_DIR" id -u "$USERNAME_INPUT" >/dev/null 2>&1; then
	chroot "$TARGET_DIR" useradd -m -s /bin/zsh -G wheel,_seatd,video,audio,input "$USERNAME_INPUT" || true
fi

# Enable core system runit services
mkdir -p "$TARGET_DIR/etc/runit/runsvdir/default"
for svc in dbus seatd udevd NetworkManager acpid turnstiled tlp zramen scx-lavd ananicy-cpp asura-sysd; do
	if [ -d "$TARGET_DIR/etc/sv/$svc" ]; then
		ln -sf "/etc/sv/$svc" "$TARGET_DIR/etc/runit/runsvdir/default/$svc"
	fi
done

# Layer 3 Tuning: Configure zramen zstd compression
mkdir -p "$TARGET_DIR/etc/sv/zramen"
cat >"$TARGET_DIR/etc/sv/zramen/conf" <<'ZRAMCONF'
export ZRAM_COMP_ALGORITHM=zstd
export ZRAM_SIZE=100
export ZRAM_PRIORITY=32767
ZRAMCONF

# Layer 3 Tuning: Configure global mimalloc lock-free allocator LD_PRELOAD
if [ -f "$TARGET_DIR/usr/lib/libmimalloc.so.3" ]; then
	echo "/usr/lib/libmimalloc.so.3" > "$TARGET_DIR/etc/ld.so.preload"
fi

printf "${WHITE}${BOLD}[6/6] Installing bootloader (GRUB EFI)...${RESET}\n"
if [ "$IS_UEFI" -eq 1 ]; then
	mount --bind /dev "$TARGET_DIR/dev"
	mount --bind /proc "$TARGET_DIR/proc"
	mount --bind /sys "$TARGET_DIR/sys"

	chroot "$TARGET_DIR" grub-install --target=x86_64-efi --efi-directory=/boot/efi --bootloader-id=Asura --recheck >/dev/null 2>&1 || true
	chroot "$TARGET_DIR" grub-mkconfig -o /boot/grub/grub.cfg >/dev/null 2>&1 || true

	umount "$TARGET_DIR/sys" || true
	umount "$TARGET_DIR/proc" || true
	umount "$TARGET_DIR/dev" || true
fi

# Unmount target partitions cleanly
umount "$TARGET_DIR/boot/efi" || true
umount "$TARGET_DIR" || true

cat <<EOF

${GREEN}┌──────────────────────────────────────────────────────────┐
│               Installation Complete! [✓]                 │
│                                                          │
│   Asura OS is now installed on your system.              │
│   You can now reboot and remove the live USB media.      │
└──────────────────────────────────────────────────────────┘${RESET}

EOF

printf "Would you like to reboot into Asura OS now? [y/N]: "

read -r REBOOT_CHOICE || REBOOT_CHOICE="n"
case "$REBOOT_CHOICE" in
	[yY]|[yY][eE][sS])
		printf "\n${WHITE}Rebooting...${RESET}\n"
		reboot
		;;
	*)
		printf "\n${DIM}Returning to live desktop environment. You may reboot anytime.${RESET}\n\n"
		;;
esac
