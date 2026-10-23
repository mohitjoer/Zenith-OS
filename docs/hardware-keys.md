# Hardware keys

Brightness, the keyboard backlight and the touchpad key work for the person at the
machine without joining any group. This is what each one does, and the facts about
the ASUS TUF Gaming F15 (FX507ZC4) they were checked against.

## Brightness and the keyboard backlight

`/sys/class/backlight/*/brightness` is group `video` and Ubuntu's `brightnessctl`
is built without logind support, so `brightnessctl set` is refused for a user who
is not in `video`. `docs/architecture.md` ("Writing a backlight") records that and
declines a udev rule. systemd-logind makes the rule unnecessary:
`Session.SetBrightness(subsystem, name, value)` writes the value, as root, for the
owner of the session, for both `backlight` and `leds`. `zenith brightness` makes
that call, then tries `brightnessctl`, then the sysfs file if it is writable
(`internal/backlight`). `zenith brightness list` prints which route each device has.

| Entry point | Does |
|---|---|
| `zenith-cmd-brightness +5`, `-5`, `40%` | what the keys, the daemon and the sliders call |
| `zenith brightness up\|down\|set\|get [--device kbd]` | the same, with the keyboard backlight (whole levels) |
| `zenith-hw-backlight` | prints the screen's backlight, for readers that need the name |

Steps are five points of *perceived* brightness (a cube-root curve), about sixteen
presses from full to the floor, and a screen never goes below 1%. A held key is
serialised so presses are not lost. The screen is the backlight whose connector is a
connected internal panel (eDP, LVDS, DSI); a name list is the fallback when the DRM
tree cannot say.

External monitors (DDC/CI) are not handled: `ddcutil` is not installed here and
nothing in the shell sends the keys to it.

## Touchpad

`zenith-touchpad on|off|toggle|status|restore` runs `hyprctl -r keyword
device[NAME]:enabled` for every pointer device whose name ends in `touchpad`.
Hyprland cannot say whether a device is enabled and enables every pad again when it
rereads its config, so the intent is a flag file under `$XDG_RUNTIME_DIR` and the
conf runs `zenith-touchpad restore` on each reload. Nothing outlives the session.

## This machine

| | |
|---|---|
| Screen | `intel_backlight`, raw, 0..96000, on `card1-eDP-1`; `root:video 0664` |
| Keyboard | `asus::kbd_backlight`, 0..3. World-writable only because of the user's own `/etc/udev/rules.d/99-asus-tuf.rules`; the default is `root:input` |
| Touchpad | `asup1205:00-093a:2003-touchpad` |
| Keys | `Asus WMI hotkeys` advertises `KEY_BRIGHTNESSUP/DOWN`, `KEY_KBDILLUMUP/DOWN`, `KEY_TOUCHPAD_TOGGLE/ON`. The ASUS driver steps the keyboard LED itself, so the two `XF86Kbd*` binds do nothing here |
| Platform profile | `balanced` of `quiet balanced performance`, root-writable; power-profiles-daemon owns it |
| Battery | `BAT1` (the user's `99-asus-tuf.rules` says `BAT0`), `charge_control_end_threshold` 80, root-writable |

## Charge limit

Not handled by code. The threshold is `root` only, and UPower's passwordless route
(`org.freedesktop.UPower.enable-charging-limit`) is unavailable because it needs a
start threshold and ASUS has only an end one (`ChargeThresholdSupported` is false).
On this machine the limit is already 80 and survives reboots through the user's own
`battery-charge-limit.service`, which writes `BAT1` at boot. Changing it is
`echo 70 | sudo tee /sys/class/power_supply/BAT1/charge_control_end_threshold`, plus
the same edit in the unit. A change from the Hub without a password would need a
`pkexec` helper and a polkit rule pinned to its path (Ryoku's `ryoku-power` and
`47-ryoku-power.rules`, with group `sudo` in place of `wheel`); that is not ported.
