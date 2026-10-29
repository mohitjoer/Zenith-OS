#!/bin/sh
# Zenith OS Guided System Installer
# Installs Zenith OS onto target drive.
set -eu

if [ "$(id -u)" -ne 0 ]; then
	echo "zenith-installer: must be run as root (e.g. sudo zenith-installer or sudo zenith install)" >&2
	exit 1
fi

echo "========================================================"
echo "               Zenith OS System Installer               "
echo "========================================================"
echo

# 1. Check UEFI firmware
if [ ! -d /sys/firmware/efi ]; then
	echo "WARNING: System booted in Legacy BIOS mode."
	echo "Zenith OS recommends UEFI for full hybrid GPU and Wayland power features."
	IS_UEFI=0
else
	echo "Boot Mode: UEFI (Verified)"
	IS_UEFI=1
fi
echo

# 2. List available target block devices
echo "Available drives:"
lsblk -d -n -o NAME,SIZE,MODEL,TYPE | grep -E 'disk' | while read -r line; do
	echo "  /dev/$line"
done
echo

if [ $# -ge 1 ]; then
	TARGET_DISK="$1"
else
	printf "Enter target disk to install Zenith OS to (e.g. /dev/nvme0n1 or /dev/vda): "
	read -r TARGET_DISK
fi

if [ ! -b "$TARGET_DISK" ]; then
	echo "Error: $TARGET_DISK is not a valid block device." >&2
	exit 1
fi

echo
echo "CRITICAL WARNING: ALL DATA ON $TARGET_DISK WILL BE DESTROYED!"
printf "Type 'YES' to proceed with partitioning and installation: "
read -r CONFIRM
if [ "$CONFIRM" != "YES" ]; then
	echo "Installation cancelled."
	exit 0
fi

echo
echo "[1/6] Partitioning $TARGET_DISK..."
# Wipe existing partition table signatures
wipefs -a "$TARGET_DISK"

# Create GPT partition table:
# Partition 1: 512MB EFI System Partition (type ef00)
# Partition 2: Remainder as Linux Root (type 8300)
parted -s "$TARGET_DISK" mklabel gpt
parted -s "$TARGET_DISK" mkpart "ESP" fat32 1MiB 513MiB
parted -s "$TARGET_DISK" set 1 esp on
parted -s "$TARGET_DISK" mkpart "ZenithRoot" 513MiB 100%

# Resolve partition names (e.g. /dev/nvme0n1p1 vs /dev/vda1)
if [ -b "${TARGET_DISK}p1" ]; then
	PART_EFI="${TARGET_DISK}p1"
	PART_ROOT="${TARGET_DISK}p2"
else
	PART_EFI="${TARGET_DISK}1"
	PART_ROOT="${TARGET_DISK}2"
fi

# Wait for kernel to register new partition table
sleep 2

echo "[2/6] Formatting partitions..."
mkfs.vfat -F 32 -n "ZENITH_ESP" "$PART_EFI"

TARGET_DIR="/mnt/zenith-target"
mkdir -p "$TARGET_DIR"

if command -v mkfs.btrfs >/dev/null 2>&1; then
	echo "Using high-performance Btrfs filesystem with zstd compression..."
	mkfs.btrfs -f -L "ZENITH_ROOT" "$PART_ROOT"
	FS_TYPE="btrfs"
	FS_OPTS="defaults,noatime,compress=zstd:1"
	mount -o "$FS_OPTS" "$PART_ROOT" "$TARGET_DIR"
else
	echo "Using ext4 filesystem..."
	mkfs.ext4 -F -L "ZENITH_ROOT" "$PART_ROOT"
	FS_TYPE="ext4"
	FS_OPTS="defaults,noatime"
	mount "$PART_ROOT" "$TARGET_DIR"
fi

mkdir -p "$TARGET_DIR/boot/efi"
mount "$PART_EFI" "$TARGET_DIR/boot/efi"

echo "[4/6] Copying system files to target..."
# Copy rootfs excluding runtime and virtual filesystems
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

echo "[5/6] Generating fstab and configuring host..."
ROOT_UUID=$(blkid -s UUID -o value "$PART_ROOT")
EFI_UUID=$(blkid -s UUID -o value "$PART_EFI")

cat >"$TARGET_DIR/etc/fstab" <<EOF
# /etc/fstab: static file system information for Zenith OS
UUID=$ROOT_UUID / $FS_TYPE $FS_OPTS 0 0
UUID=$EFI_UUID /boot/efi vfat defaults,noatime,umask=0077 0 2
tmpfs /tmp tmpfs defaults,nosuid,nodev 0 0
EOF

# Ensure system runit services are active on target
mkdir -p "$TARGET_DIR/etc/runit/runsvdir/default"
for svc in dbus seatd elogind udevd NetworkManager acpid turnstiled tlp zenith-sysd; do
	if [ -d "$TARGET_DIR/etc/sv/$svc" ]; then
		ln -sf "/etc/sv/$svc" "$TARGET_DIR/etc/runit/runsvdir/default/$svc"
	fi
done

echo "[6/6] Installing bootloader (GRUB EFI)..."
if [ "$IS_UEFI" -eq 1 ]; then
	mount --bind /dev "$TARGET_DIR/dev"
	mount --bind /proc "$TARGET_DIR/proc"
	mount --bind /sys "$TARGET_DIR/sys"

	chroot "$TARGET_DIR" grub-install --target=x86_64-efi --efi-directory=/boot/efi --bootloader-id=Zenith --recheck || true
	chroot "$TARGET_DIR" grub-mkconfig -o /boot/grub/grub.cfg || true

	umount "$TARGET_DIR/sys" || true
	umount "$TARGET_DIR/proc" || true
	umount "$TARGET_DIR/dev" || true
fi

# Clean up mounts
umount "$TARGET_DIR/boot/efi"
umount "$TARGET_DIR"

echo
echo "========================================================"
echo "    Zenith OS installation completed successfully!      "
echo "    You can now reboot and remove the live USB media.   "
echo "========================================================"
