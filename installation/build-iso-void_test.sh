#!/bin/sh
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SCRIPT="$REPO_ROOT/installation/build-iso-void.sh"
MANIFEST="$REPO_ROOT/system/packages-void.yaml"

if [ ! -x "$SCRIPT" ]; then
	echo "build-iso-void_test: build-iso-void.sh is not executable" >&2
	exit 1
fi

if ! sh -n "$SCRIPT"; then
	echo "build-iso-void_test: shell syntax check failed on build-iso-void.sh" >&2
	exit 1
fi

if ! sh -n "$REPO_ROOT/session/runit/asura-user-services"; then
	echo "build-iso-void_test: shell syntax check failed on asura-user-services" >&2
	exit 1
fi

for run_script in "$REPO_ROOT/system/runit/user-services"/*/run "$REPO_ROOT/system/runit/services"/*/run "$REPO_ROOT/installation/asura-installer.sh"; do
	if ! sh -n "$run_script"; then
		echo "build-iso-void_test: shell syntax check failed on $run_script" >&2
		exit 1
	fi
done

if [ ! -f "$MANIFEST" ]; then
	echo "build-iso-void_test: system/packages-void.yaml missing" >&2
	exit 1
fi

grep -Fq 'system/packages-void.yaml' "$SCRIPT"
grep -Fq 'void-linux' "$SCRIPT"
grep -Fq 'asura-os-beta-001' "$SCRIPT"
grep -Fq 'LIBSEAT_BACKEND=seatd' "$SCRIPT"
grep -Fq 'void-mklive' "$SCRIPT"
grep -Fq 'asura-user-services' "$SCRIPT"
grep -Fq 'asura-sysd' "$SCRIPT"
grep -Fq 'asura-userd' "$SCRIPT"
grep -Fq 'etc/sv/asura-sysd' "$SCRIPT"
grep -Fq 'NVreg_DynamicPowerManagement=0x02' "$SCRIPT"
grep -Fq '99-asura-power.rules' "$SCRIPT"
grep -Fq '99-asura-performance.conf' "$SCRIPT"
grep -Fq 'turnstile.conf' "$SCRIPT"
grep -Fq 'system/xbps.d/asura.conf' "$SCRIPT"
grep -Fq 'boot-splash.png' "$SCRIPT"
grep -Fq 'Welcome to Asura OS!' "$SCRIPT"
grep -Fq 'nowatchdog mitigations=off preempt=full' "$SCRIPT"
grep -Fq 'os-ctl' "$SCRIPT"
grep -Fq 'etc/issue' "$SCRIPT"
grep -Fq 'etc/motd' "$SCRIPT"
grep -Fq 'DejaVuSans.ttf' "$SCRIPT"
grep -Fq 'plymouth --quit' "$SCRIPT"

# Verify boot splash asset exists
if [ ! -f "$REPO_ROOT/public/branding/boot-splash.png" ]; then
	echo "build-iso-void_test: public/branding/boot-splash.png missing" >&2
	exit 1
fi

# Verify Plymouth splash theme assets exist and are valid non-empty files
for asset in logo.png window.png progress_box.png progress_bar.png asura.plymouth asura.script; do
	if [ ! -s "$REPO_ROOT/system/plymouth/asura/$asset" ]; then
		echo "build-iso-void_test: system/plymouth/asura/$asset missing or empty" >&2
		exit 1
	fi
done

# Verify udev power rules exist
if [ ! -s "$REPO_ROOT/system/udev/99-asura-power.rules" ]; then
	echo "build-iso-void_test: system/udev/99-asura-power.rules missing or empty" >&2
	exit 1
fi

# Verify packages in packages-void.yaml
grep -Fq 'base-system' "$MANIFEST"
grep -Fq 'hyprland' "$MANIFEST"
grep -Fq 'qt6-declarative' "$MANIFEST"
grep -Fq 'ghostty' "$MANIFEST"
grep -Fq 'pipewire' "$MANIFEST"
grep -Fq 'wireplumber' "$MANIFEST"
grep -Fq 'seatd' "$MANIFEST"
grep -Fq 'turnstile' "$MANIFEST"
grep -Fq 'tlp' "$MANIFEST"
grep -Fq 'btrfs-progs' "$MANIFEST"
grep -Fq 'gamemode' "$MANIFEST"
grep -Fq 'zramen' "$MANIFEST"
grep -Fq 'scx' "$MANIFEST"
grep -Fq 'scx-loader' "$MANIFEST"
grep -Fq 'mimalloc' "$MANIFEST"
grep -Fq 'podman' "$MANIFEST"

# LAYER 2: Verify Compiler, Kernel & Direct Hardware sysfs/udev
grep -Fq 'CONFIG_SCHED_BORE=y' "$REPO_ROOT/system/kernel/x86_64-dotconfig-asura"
grep -Fq 'CONFIG_HZ=1000' "$REPO_ROOT/system/kernel/x86_64-dotconfig-asura"
grep -Fq 'CONFIG_PREEMPT=y' "$REPO_ROOT/system/kernel/x86_64-dotconfig-asura"
grep -Fq 'CONFIG_SCHED_CLASS_EXT=y' "$REPO_ROOT/system/kernel/x86_64-dotconfig-asura"
grep -Fq 'cachyos-bore' "$REPO_ROOT/system/kernel/fleet.yaml"
grep -Fq 'march=x86-64-v3' "$REPO_ROOT/packaging/xbps/conf/xbps-v3.conf"
grep -Fq 'flto=auto' "$REPO_ROOT/packaging/xbps/conf/xbps-v3.conf"
grep -Fq -- '--emit-relocs' "$REPO_ROOT/packaging/xbps/conf/xbps-v3.conf"
grep -Fq 'profile-use' "$REPO_ROOT/packaging/xbps/conf/xbps-v3.conf"
test -s "$REPO_ROOT/system/udev/80-nvidia-pm.rules"
grep -Fq 'NVreg_DynamicPowerManagement=0x02' "$REPO_ROOT/system/modprobe.d/nvidia-optimus.conf"
grep -Fq 'charge_control_end_threshold' "$REPO_ROOT/system/udev/99-battery-threshold.rules"
grep -Fq 'pwm' "$REPO_ROOT/system/udev/70-pwm-fan.rules"

# LAYER 3: Verify Process, Memory & Storage Tuning
grep -Fq 'scx_lavd' "$REPO_ROOT/system/runit/services/scx-lavd/run"
grep -Fq 'scx-lavd' "$SCRIPT"
grep -Fq 'ananicy-cpp' "$REPO_ROOT/system/runit/services/ananicy-cpp/run"
grep -Fq 'ananicy-cpp' "$SCRIPT"
grep -Fq 'compiler' "$REPO_ROOT/system/ananicy/00-asura-rules.rules"
grep -Fq 'scanner' "$REPO_ROOT/system/ananicy/00-asura-rules.rules"
grep -Fq 'libmimalloc.so.3' "$SCRIPT"
grep -Fq 'libmimalloc.so.3' "$REPO_ROOT/installation/asura-installer.sh"
grep -Fq 'ZRAM_COMP_ALGORITHM=zstd' "$SCRIPT"
grep -Fq 'ZRAM_COMP_ALGORITHM=zstd' "$REPO_ROOT/installation/asura-installer.sh"
grep -Fq 'compress=zstd:1' "$REPO_ROOT/installation/asura-installer.sh"
grep -Fq 'kyber' "$REPO_ROOT/system/udev/60-io-schedulers.rules"
grep -Fq 'bfq' "$REPO_ROOT/system/udev/60-io-schedulers.rules"

echo "build-iso-void_test: ok"


