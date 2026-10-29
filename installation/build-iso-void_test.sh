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

if ! sh -n "$REPO_ROOT/session/runit/zenith-user-services"; then
	echo "build-iso-void_test: shell syntax check failed on zenith-user-services" >&2
	exit 1
fi

for run_script in "$REPO_ROOT/system/runit/user-services"/*/run "$REPO_ROOT/system/runit/services"/*/run "$REPO_ROOT/installation/zenith-installer.sh"; do
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
grep -Fq 'zenith-os-beta-001' "$SCRIPT"
grep -Fq 'LIBSEAT_BACKEND=seatd' "$SCRIPT"
grep -Fq 'void-mklive' "$SCRIPT"
grep -Fq 'zenith-user-services' "$SCRIPT"
grep -Fq 'zenith-sysd' "$SCRIPT"
grep -Fq 'zenith-userd' "$SCRIPT"
grep -Fq 'etc/sv/zenith-sysd' "$SCRIPT"
grep -Fq 'NVreg_DynamicPowerManagement=0x02' "$SCRIPT"
grep -Fq '99-zenith-power.rules' "$SCRIPT"
grep -Fq '99-zenith-performance.conf' "$SCRIPT"
grep -Fq 'turnstile.conf' "$SCRIPT"
grep -Fq 'system/xbps.d/zenith.conf' "$SCRIPT"
grep -Fq 'boot-splash.png' "$SCRIPT"
grep -Fq 'Welcome to Zenith OS!' "$SCRIPT"
grep -Fq 'nowatchdog mitigations=off preempt=full' "$SCRIPT"
grep -Fq 'os-ctl' "$SCRIPT"
grep -Fq 'etc/issue' "$SCRIPT"
grep -Fq 'etc/motd' "$SCRIPT"

# Verify boot splash asset exists
if [ ! -f "$REPO_ROOT/public/branding/boot-splash.png" ]; then
	echo "build-iso-void_test: public/branding/boot-splash.png missing" >&2
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

echo "build-iso-void_test: ok"

