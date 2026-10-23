# The login screen

Zenith's login screen is an SDDM theme, `session/sddm/themes/zenith`. SDDM runs it
inside the kiosk Weston that `session/zenith-greeter` starts. The theme logs in
through SDDM's own API (`sddm.login`), so PAM does the authenticating exactly as
it does for any SDDM theme and Zenith touches nothing under `/etc/pam.d`.

It shows the users, a password field, the sessions with Zenith first and
preselected, and Sleep, Restart and Shut down when the daemon allows them. That is
all it does.

## It is off until you turn it on

The package ships the SDDM settings as `/usr/share/zenith/session/sddm/zenith.conf`
and installs them nowhere. A login screen that does not come up leaves nobody able
to log in, so the one file that turns Zenith's on is copied by you, with sudo.

1. `sudo apt install sddm weston`. When asked for the default display manager,
   keep the current one until the later steps have worked.
2. `zenith greeter test` draws the screen inside your session for a minute, with
   logging in switched off. Esc leaves.
3. `zenith greeter enable` checks that everything is installed and prints the
   `sudo install` command that copies the file to `/etc/sddm.conf.d/10-zenith.conf`.
4. Pick sddm as the display manager (`sudo dpkg-reconfigure sddm`) and reboot.

`zenith greeter disable` prints the command that removes the file again.

## If the login screen is broken

Switch to a text console with Ctrl+Alt+F3, log in, and run
`sudo rm /etc/sddm.conf.d/10-zenith.conf`. SDDM's own default is an X11 greeter,
and Zenith carries no X server, so if SDDM still does not come up, switch back
with `sudo dpkg-reconfigure gdm3` (or whichever display manager you had) and reboot.

To find out why: `journalctl -b -u sddm`. "Fallback to embedded theme" followed by
a QML error means the theme did not load; "Wayland socket watcher timed out" means
Weston did not start.

## What has not been run

The theme has been driven with mock SDDM objects under `qmltestrunner` and
rendered offscreen. It has never run inside `sddm-greeter-qt6` under Weston,
because neither is installed on the machine it was written on. Check first that
`zenith greeter test` shows the form, then that a real login works from the
greeter, then a second monitor: every screen shows the form, and keyboard focus
lands on whichever Weston activates.

## Left out on purpose

No clock, blur, Caps Lock hint or keyboard-layout indicator, and no "other user"
field for accounts SDDM hides. SDDM's keyboard model is empty under Wayland, so the
last two would need guessing from key events. SDDM's remembered last session is
ignored: Zenith is always preselected, and one click picks another.
