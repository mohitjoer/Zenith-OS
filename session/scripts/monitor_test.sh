#!/usr/bin/env bash
# Fixture test for zenith-monitor's display persistence across a reboot and a
# power-cycled TV (issue #152). A layout saved from the Displays page must come
# back at the next login even when the external display is absent, reports a
# different serial, or returns on a different connector -- the reported case is
# an LG TV over HDMI that reverts on every reboot. Also guards the two matching
# hazards: an EDID-less display must key on its connector (never the bare "||"),
# and booting the same disk on another machine must not drape one panel's
# settings over a different panel on the same port. Runs in fixture mode
# (ZENITH_MONITOR_JSON), so no live compositor is needed.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
mon="$here/zenith-monitor"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

conf="$tmp/monitors-auto.conf"
export ZENITH_MONITORS_CONF="$conf"
export ZENITH_MONITORS_DIR="$tmp/profiles"
export ZENITH_MONITORS_APPLIED="$tmp/applied.json"
export ZENITH_MONITORS_USER="$tmp/none-user.conf"
export ZENITH_MONITOR_VM=0

fail() { echo "FAIL: $1" >&2; exit 1; }
has() { grep -qF -- "$2" "$1" || fail "$3"; }
hasnt() { grep -qF -- "$2" "$1" && fail "$3" || true; }

# Laptop panel + LG TV over HDMI, both live (the state when the user opens the
# Displays page and applies a layout). The TV reports an empty serial, as many do.
cat >"$tmp/both.json" <<'JSON'
[
  {"name":"eDP-1","make":"Acme","model":"Panel","serial":"LP01","width":2560,"height":1600,"refreshRate":60.0,"x":0,"y":0,"scale":1.6,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","physicalWidth":300,"physicalHeight":188,"availableModes":["2560x1600@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0},
  {"name":"HDMI-A-1","make":"LG Electronics","model":"LG TV SSCR2","serial":"","width":1920,"height":1080,"refreshRate":60.0,"x":2560,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":false,"mirrorOf":"none","physicalWidth":1600,"physicalHeight":900,"availableModes":["3840x2160@60.00Hz","1920x1080@60.00Hz","1920x1080@120.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON

# Reboot: TV asleep / not yet probed at login -> only the panel is live.
cat >"$tmp/absent.json" <<'JSON'
[
  {"name":"eDP-1","make":"Acme","model":"Panel","serial":"LP01","width":2560,"height":1600,"refreshRate":60.0,"x":0,"y":0,"scale":1.6,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","physicalWidth":300,"physicalHeight":188,"availableModes":["2560x1600@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON

# Reboot: TV present but reporting a different serial (LG TVs vary or blank it).
cat >"$tmp/serial.json" <<'JSON'
[
  {"name":"eDP-1","make":"Acme","model":"Panel","serial":"LP01","width":2560,"height":1600,"refreshRate":60.0,"x":0,"y":0,"scale":1.6,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","physicalWidth":300,"physicalHeight":188,"availableModes":["2560x1600@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0},
  {"name":"HDMI-A-1","make":"LG Electronics","model":"LG TV SSCR2","serial":"0x01010101","width":1920,"height":1080,"refreshRate":60.0,"x":2560,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":false,"mirrorOf":"none","physicalWidth":1600,"physicalHeight":900,"availableModes":["3840x2160@60.00Hz","1920x1080@60.00Hz","1920x1080@120.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON

# Reboot: TV back on a different connector, same identity.
cat >"$tmp/renamed.json" <<'JSON'
[
  {"name":"eDP-1","make":"Acme","model":"Panel","serial":"LP01","width":2560,"height":1600,"refreshRate":60.0,"x":0,"y":0,"scale":1.6,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","physicalWidth":300,"physicalHeight":188,"availableModes":["2560x1600@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0},
  {"name":"HDMI-A-2","make":"LG Electronics","model":"LG TV SSCR2","serial":"","width":1920,"height":1080,"refreshRate":60.0,"x":2560,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":false,"mirrorOf":"none","physicalWidth":1600,"physicalHeight":900,"availableModes":["3840x2160@60.00Hz","1920x1080@60.00Hz","1920x1080@120.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON

# The Displays-page layout: TV kept at 1080p to the right of the 1.6x panel.
layout='[
  {"id":"Acme|Panel|LP01","output":"eDP-1","mode":"2560x1600@60","position":"0x0","scale":1.6,"transform":0,"vrr":0,"mirror":"none","disabled":false},
  {"id":"LG Electronics|LG TV SSCR2|","output":"HDMI-A-1","mode":"1920x1080@60","position":"2560x0","scale":1,"transform":0,"vrr":0,"mirror":"none","disabled":false}
]'

apply_both() { ZENITH_MONITOR_JSON="$tmp/both.json" "$mon" apply "$layout" >/dev/null; }
login() { ZENITH_MONITOR_JSON="$tmp/$1" "$mon" autoscale >/dev/null 2>&1; }

# --- missing output: an absent TV keeps its saved stanza, and the panel keeps
# its own applied settings (the "HDMI reverts, laptop is fine" report). ---------
apply_both
login absent.json
has "$conf" 'monitor = HDMI-A-1,1920x1080@60,' \
  "an absent TV's saved stanza was dropped at login (reverts like freshly connected)"
has "$conf" ',2560x0,' "the absent TV lost its saved position"
has "$conf" 'monitor = eDP-1,2560x1600@60,0x0,1.6' \
  "the panel lost its own applied settings"

# --- identity: a drifting serial still resolves to the same TV by make|model ---
apply_both
login serial.json
has "$conf" 'monitor = HDMI-A-1,1920x1080@60,2560x0,' \
  "a serial-drifted TV was treated as a new display and reset from its saved layout"
grep -qF 'monitor = HDMI-A-1,highrr,' "$conf" \
  && fail "the TV was reset to the DPI/highrr fallback instead of its saved mode" || true

# --- identity: a connector rename remaps the saved stanza to the new connector -
apply_both
login renamed.json
has "$conf" 'monitor = HDMI-A-2,1920x1080@60,2560x0,' \
  "a renamed connector was not remapped to the saved TV settings"
hasnt "$conf" 'monitor = HDMI-A-1,' "the stale connector name survived a rename"

# --- baseline: exact identity round-trips unchanged ---------------------------
apply_both
login both.json
has "$conf" 'monitor = HDMI-A-1,1920x1080@60,2560x0,' \
  "an exact-identity login did not recall the saved TV settings"

# --- connector fallback: a display with no EDID keys on its connector, so a
# match key is never the bare "||" that every EDID-less output would collide on,
# and its applied layout is recalled. ------------------------------------------
cat >"$tmp/noedid.json" <<'JSON'
[
  {"name":"HDMI-A-1","make":"","model":"","serial":"","width":1920,"height":1080,"refreshRate":60.0,"x":0,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","physicalWidth":0,"physicalHeight":0,"availableModes":["1920x1080@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON
idval="$(ZENITH_MONITOR_JSON="$tmp/noedid.json" "$mon" list | jq -r '.[0].id')"
[[ $idval == "HDMI-A-1" ]] || fail "an EDID-less display keyed on '$idval', not its connector"
noedid_layout='[{"id":"HDMI-A-1","output":"HDMI-A-1","mode":"1920x1080@60","position":"0x0","scale":1,"transform":0,"vrr":0,"mirror":"none","disabled":false}]'
ZENITH_MONITORS_APPLIED="$tmp/noedid-applied.json" ZENITH_MONITOR_JSON="$tmp/noedid.json" "$mon" apply "$noedid_layout" >/dev/null
ZENITH_MONITORS_APPLIED="$tmp/noedid-applied.json" ZENITH_MONITOR_JSON="$tmp/noedid.json" "$mon" autoscale >/dev/null 2>&1
has "$conf" 'monitor = HDMI-A-1,1920x1080@60,' \
  "an EDID-less display did not recall its connector-keyed layout"

# --- another machine, same disk: a different panel on eDP-1 must NOT inherit the
# saved layout; DPI/live scaling wins, and the TV stanza must not leak onto it. -
cat >"$tmp/other.json" <<'JSON'
[
  {"name":"eDP-1","make":"Other","model":"Slab","serial":"S9","width":1920,"height":1080,"refreshRate":60.0,"x":0,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","physicalWidth":510,"physicalHeight":287,"availableModes":["1920x1080@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON
apply_both
login other.json
has "$conf" 'monitor = eDP-1,' "the live panel got no stanza on an unrelated machine"
hasnt "$conf" ',2560x0,' "the saved TV stanza leaked onto an unrelated machine"
hasnt "$conf" 'monitor = eDP-1,2560x1600@60,' \
  "a different panel on eDP-1 wrongly inherited the saved panel's mode"

# --- the writer: every extra Hyprland takes, and nothing a name can smuggle in ---
hub='[
  {"id":"A|B|1","output":"DP-1","mode":"3840x2160@60","position":"0x0","scale":1,"transform":1,"vrr":1,"mirror":"none","cm":"hdr","sdrbrightness":1.3,"disabled":false},
  {"id":"A|B|2","output":"DP-2","mode":"1920x1080@60","position":"2160x0","scale":1,"transform":0,"vrr":0,"mirror":"DP-1","disabled":false},
  {"id":"A|B|3","output":"DP-3","disabled":true},
  {"id":"A|B|4","output":"DP-4,disable\nmonitor = ,preferred,auto,9","mode":"1920x1080@60","position":"0x0","scale":1,"disabled":false}
]'
# (no cvt: a mode the fixture panel does not advertise would become a modeline)
ZENITH_MONITOR_CVT=/nonexistent ZENITH_MONITOR_JSON="$tmp/both.json" "$mon" apply "$hub" >/dev/null
has "$conf" 'monitor = DP-1,3840x2160@60,0x0,1,transform,1,vrr,1,bitdepth,10,cm,hdr,sdrbrightness,1.3' "the rotated VRR HDR monitor lost an argument"
has "$conf" 'monitor = DP-2,1920x1080@60,2160x0,1,mirror,DP-1,bitdepth,8,cm,srgb,sdrbrightness,1' "the mirror or the sRGB reset was not written"
has "$conf" 'monitor = DP-3,disable' "a disabled output was not written as disable"
has "$conf" 'monitor = , preferred, auto, 1' "the hotplug catch-all is missing"
has "$conf" 'env = GDK_SCALE,' "GDK_SCALE was not written"
hasnt "$conf" 'DP-4' "an output name with a comma and a newline got into the file"
# the file is valid hyprlang, when a compositor binary is here to say so
if command -v Hyprland >/dev/null 2>&1; then
  printf 'source = %s\n' "$conf" >"$tmp/verify.conf"
  Hyprland --verify-config -c "$tmp/verify.conf" 2>&1 | grep -q 'config ok' || fail "Hyprland rejects the generated monitors-auto.conf"
fi

echo "monitor-persist: all checks passed"

# --- custom resolution: a WxH@rate mode the panel does NOT advertise becomes a
# CVT modeline (so Hyprland forces it instead of ignoring it); an advertised mode
# passes through unchanged. A stub cvt keeps libxcvt out of it. ------------------
cat >"$tmp/cvt" <<'CVT'
#!/usr/bin/env bash
w="$2"; h="$3"
printf 'Modeline "%sx%sR"  241.50  %s 2608 2640 2720  %s 1443 1448 1481 +hsync -vsync\n' "$w" "$h" "$w" "$h"
CVT
chmod +x "$tmp/cvt"
cat >"$tmp/live.json" <<'JSON'
[
  {"name":"eDP-1","make":"A","model":"P","serial":"1","width":2560,"height":1440,"refreshRate":60.0,"x":0,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","availableModes":["2560x1440@60.00Hz"]},
  {"name":"DP-1","make":"B","model":"Q","serial":"2","width":1920,"height":1080,"refreshRate":60.0,"x":2560,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":false,"mirrorOf":"none","availableModes":["1920x1080@60.00Hz"]}
]
JSON
custom='[
  {"id":"A|P|1","output":"eDP-1","mode":"2560x1440@120","position":"0x0","scale":1,"transform":0,"vrr":0,"mirror":"none","disabled":false},
  {"id":"B|Q|2","output":"DP-1","mode":"1920x1080@60","position":"2560x0","scale":1,"transform":0,"vrr":0,"mirror":"none","disabled":false}
]'
ZENITH_MONITOR_JSON="$tmp/live.json" ZENITH_MONITOR_CVT="$tmp/cvt" \
  ZENITH_MONITORS_APPLIED="$tmp/custom-applied.json" "$mon" apply "$custom" >/dev/null
has "$conf" 'monitor = eDP-1,modeline 241.50 2560 2608 2640 2720 1440 1443 1448 1481 +hsync -vsync,0x0,1' \
  "custom non-advertised 2560x1440@120 was not rewritten to a CVT modeline"
hasnt "$conf" 'eDP-1,2560x1440@120' "the non-advertised WxH@rate mode was written verbatim (Hyprland would ignore it)"
has "$conf" 'monitor = DP-1,1920x1080@60,2560x0,1' "advertised 1920x1080@60 should pass through unchanged"
grep 'monitor = DP-1,' "$conf" | grep -q 'modeline' && fail "advertised mode was needlessly converted to a modeline" || true
jq -e '.monitors[] | select(.output=="eDP-1") | .mode | startswith("modeline ")' "$tmp/custom-applied.json" >/dev/null \
  || fail "applied layout did not persist the modeline"
if command -v Hyprland >/dev/null 2>&1; then
  printf 'source = %s\n' "$conf" >"$tmp/verify.conf"
  Hyprland --verify-config -c "$tmp/verify.conf" 2>&1 | grep -q 'config ok' || fail "Hyprland rejects a modeline rule"
fi

echo "monitor-custom-mode: all checks passed"

# --- autoscale at login is opt-in: nothing is written until it is switched on ---
flag="$tmp/autoscale-flag"
export ZENITH_MONITOR_AUTOSCALE_FLAG="$flag"
rm -f "$conf"
ZENITH_MONITOR_JSON="$tmp/both.json" "$mon" autoscale --at-login >/dev/null 2>&1
[[ ! -e $conf ]] || fail "autoscale wrote monitors-auto.conf at login without being switched on"
[[ "$("$mon" autoscale status)" == off ]] || fail "autoscale at login is not off by default"
"$mon" autoscale on >/dev/null
ZENITH_MONITOR_JSON="$tmp/both.json" "$mon" autoscale --at-login >/dev/null 2>&1
[[ -e $conf ]] || fail "autoscale did not run at login after being switched on"
"$mon" autoscale off >/dev/null
[[ "$("$mon" autoscale status)" == off ]] || fail "autoscale off did not switch it off"

echo "monitor-autoscale-optin: all checks passed"
