#!/usr/bin/env bash
# fixture test for zenith-cmd-game-mode (the one-click competitive toggle), ported
# from Ryoku's. stubs hyprctl, pkexec, powerprofilesctl and both privileged
# helpers, points state + sysfs at a tmp dir, so the real compositor, the real
# wifi radio and the real power profile are never touched. verifies:
#   - toggle lifecycle
#   - the compositor strip goes through `hyprctl --batch` keywords, reverts via reload
#   - the power profile flips to performance and the PRIOR one comes back
#   - the privileged system tune is applied on start and restored on stop
#   - wifi power-save delegated to the privileged zenith-wifi-powersave via pkexec
#   - refuses on battery, before touching anything
#   - clean no-op when there's no wifi device or a helper is missing
#   - a hyprctl that rejects the batch is not reported as a stripped desktop
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
gm="$here/../../../../shell/scripts/zenith-cmd-game-mode"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

bin="$tmp/bin"; mkdir -p "$bin"
calls="$tmp/calls.log"

logger() {  # name -> stub that records its argv and answers `ok` per batch command
  cat >"$bin/$1" <<EOF
#!/usr/bin/env bash
echo "$1 \$*" >>"$calls"
[ -e "$tmp/HYPR_REJECTS" ] && { echo "Config error: invalid field"; exit 0; }
[ "$1" = hyprctl ] && [ "\${1:-}" = "--batch" ] && echo ok
exit 0
EOF
}
logger hyprctl
logger pkexec
logger zenith-wifi-powersave
logger zenith-game-tune

# notify-send records the message so the "stripped" claim can be asserted.
cat >"$bin/notify-send" <<EOF
#!/usr/bin/env bash
echo "notify \$*" >>"$calls"
EOF

# powerprofilesctl stub with a real backing value, so a restore that writes back
# the wrong profile is visible rather than merely unlogged.
profile="$tmp/profile"; echo balanced >"$profile"
cat >"$bin/powerprofilesctl" <<EOF
#!/usr/bin/env bash
echo "powerprofilesctl \$*" >>"$calls"
case "\${1:-}" in
  get) cat "$profile" ;;
  set) printf '%s\n' "\${2:-}" >"$profile" ;;
esac
exit 0
EOF
chmod +x "$bin"/*

# fake sysfs with one battery, charging (overridable per-test).
power="$tmp/power"; mkdir -p "$power/BAT0"; echo Charging >"$power/BAT0/status"
nopower="$tmp/nopower"; mkdir -p "$nopower"

# fake net sysfs with one wifi device (overridable per-test).
net="$tmp/net"; mkdir -p "$net/wlan0/wireless" "$net/eth0"
nonet="$tmp/nonet"; mkdir -p "$nonet/eth0"

export PATH="$bin:$PATH"
export ZENITH_STATE_PATH="$tmp/state"
export ZENITH_GAMEMODE_STATE_FILE="$tmp/state/game-mode.enabled"
export ZENITH_GAMEMODE_PROFILE_FILE="$tmp/state/game-mode.profile"
export ZENITH_NET_SYSFS="$net"
export ZENITH_POWER_SYSFS="$power"
export ZENITH_WIFI_POWERSAVE_BIN="$bin/zenith-wifi-powersave"
export ZENITH_GAME_TUNE_BIN="$bin/zenith-game-tune"
state="$ZENITH_GAMEMODE_STATE_FILE"

fail() { echo "FAIL: $1" >&2; exit 1; }
on() { [[ -f $state ]]; }

"$gm" status && fail "status reported on before any start"

# --- start: compositor strip + profile + tune + wifi off -------------------
: >"$calls"
"$gm" start
on || fail "start did not persist the request"
grep -qF 'hyprctl --batch' "$calls" || fail "compositor did not go through hyprctl --batch"
grep -qF 'general:allow_tearing true' "$calls" || fail "strip did not enable tearing"
grep -qF 'windowrule immediate on, match:class .*' "$calls" || fail "strip did not add the immediate rule"
# A fullscreen window must keep rendering off-screen. Without this a workspace
# switch stops the game's render loop dead (no frame callbacks reach an invisible
# surface), which reads as a freeze and times multiplayer out. Scoped to
# fullscreen on purpose: matching every class would render the whole desktop
# off-screen and starve the game of the GPU this protects.
grep -qF 'windowrule render_unfocused on, match:fullscreen 1' "$calls" ||
  fail "render_unfocused is not scoped to fullscreen windows"
grep -qF 'powerprofilesctl set performance' "$calls" || fail "did not ask for the performance profile"
grep -qF 'zenith-game-tune apply' "$calls" || fail "system tune not applied"
grep -qE 'pkexec .*zenith-wifi-powersave off' "$calls" || fail "WiFi helper not asked to disable power-save"
grep -qF 'compositor stripped' "$calls" || fail "start did not say the compositor was stripped"
[[ "$(cat "$profile")" == performance ]] || fail "profile did not actually reach performance"
"$gm" status || fail "status reported off while on"

# --- stop: reload + tune restored + the PRIOR profile back -----------------
: >"$calls"
"$gm" stop
on && fail "stop did not clear the request"
grep -qF 'hyprctl reload' "$calls" || fail "stop did not reload to revert the compositor"
grep -qF 'zenith-game-tune restore' "$calls" || fail "system tune not restored"
grep -qE 'pkexec .*zenith-wifi-powersave on' "$calls" || fail "WiFi helper not asked to restore power-save"
[[ "$(cat "$profile")" == balanced ]] ||
  fail "restored the wrong profile (want the prior 'balanced', got '$(cat "$profile")')"
"$gm" status && fail "status reported on after stop"

# --- the prior profile is whatever was there, not a hardcoded default ------
echo power-saver >"$profile"
"$gm" start
[[ "$(cat "$profile")" == performance ]] || fail "second start did not reach performance"
"$gm" stop
[[ "$(cat "$profile")" == power-saver ]] ||
  fail "did not restore a non-default prior profile (got '$(cat "$profile")')"

# --- toggle flips both ways ------------------------------------------------
"$gm" toggle; on || fail "toggle did not turn on"
"$gm" toggle; on && fail "toggle did not turn off"

# --- on battery: refuses, and touches nothing ------------------------------
echo Discharging >"$power/BAT0/status"
echo balanced >"$profile"
: >"$calls"
"$gm" start && fail "start succeeded on battery"
on && fail "a refused start still persisted the request"
grep -qF 'hyprctl --batch' "$calls" && fail "stripped the compositor on battery"
grep -qF 'zenith-game-tune' "$calls" && fail "applied the privileged tune on battery"
[[ "$(cat "$profile")" == balanced ]] || fail "changed the power profile on battery"
echo Charging >"$power/BAT0/status"

# --- no battery at all (a desktop): treated as AC --------------------------
ZENITH_POWER_SYSFS="$nopower" "$gm" start
on || fail "refused to start on a machine with no battery"
ZENITH_POWER_SYSFS="$nopower" "$gm" stop

# --- no wifi device: compositor still applies, wifi is a clean no-op -------
: >"$calls"
ZENITH_NET_SYSFS="$nonet" "$gm" start
on || fail "start failed on a no-WiFi host"
grep -qF 'hyprctl --batch' "$calls" || fail "compositor did not apply on a no-WiFi host"
grep -qF 'zenith-wifi-powersave' "$calls" && fail "touched WiFi on a host with no WiFi device"
ZENITH_NET_SYSFS="$nonet" "$gm" stop

# --- helpers absent: the rest still applies --------------------------------
: >"$calls"
ZENITH_WIFI_POWERSAVE_BIN="$tmp/nope" ZENITH_GAME_TUNE_BIN="$tmp/nope" "$gm" start
grep -qF 'hyprctl --batch' "$calls" || fail "compositor did not apply when helpers absent"
grep -qF 'zenith-wifi-powersave' "$calls" && fail "tried to call an absent WiFi helper"
grep -qF 'zenith-game-tune' "$calls" && fail "tried to call an absent tune helper"
ZENITH_WIFI_POWERSAVE_BIN="$tmp/nope" ZENITH_GAME_TUNE_BIN="$tmp/nope" "$gm" stop

# --- a compositor that rejects the batch: power still boosts, and the
#     notification does not claim a strip that never happened -----------------
touch "$tmp/HYPR_REJECTS"
: >"$calls"
"$gm" start
on || fail "start did not persist when the compositor rejected the strip"
grep -qF 'powerprofilesctl set performance' "$calls" || fail "power did not boost without a compositor strip"
grep -qF 'compositor stripped' "$calls" && fail "claimed a compositor strip the compositor rejected"
"$gm" stop
rm -f "$tmp/HYPR_REJECTS"

echo "game-mode: all checks passed"
