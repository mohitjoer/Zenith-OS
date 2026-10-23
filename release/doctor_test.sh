#!/bin/sh
# The installed package, checked by its own doctor.
#
# Every other test in this repository runs against the repository's own layout:
# QML beside the schemas that describe them, sources beside the sources that
# import them, files where the checkout puts them. Nothing ran against the layout
# dpkg produces, and that is the layout the project is judged in. Faults this
# script exists to catch were visible only after installation:
#
#   - schemas searched for in the shell's directory, which is not where GLib
#     puts them, so a clean install reported every setting as unreachable;
#   - zenith-hub and zenith-greeter installed non-executable, because the list
#     that sets modes was written by hand and two programs were missing from it;
#   - an installed zenith unable to find the profile database it shipped with,
#     reporting an empty database instead of a broken install;
#   - the portal configuration the package installs never being a file doctor
#     would look at, so an installed machine reported none at all;
#   - the Hub's entry file left out of the package, which nothing else notices:
#     the doctor checks the one tree it walks, and zenith-hub would still be
#     installed and executable, and would open a window that is not there.
#
# What this asserts is that everything the package owns is right. What it does
# not assert is that a CI runner is a working Zenith machine: Quickshell is
# built from source and is not in the archive, there is no SDDM configured
# here, and dpkg-deb -x unpacks zenith alone — none of the portal backends that
# the shipped configuration names are in the stage.
#
# The allowlist is the interesting half. Each entry is named rather than
# patterned, so a new fault cannot hide behind it, and each is something a
# runner genuinely cannot have. It is expected to shrink and never to grow.
#
# Usage: release/doctor_test.sh [path-to-deb-or-directory]
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
INPUT=${1:-"$REPO_ROOT/build/debs"}

if [ -d "$INPUT" ]; then
	DEB=$(ls "$INPUT"/zenith_*.deb 2>/dev/null | head -1 || true)
	if [ -z "$DEB" ]; then
		echo "doctor_test: no .deb in $INPUT; run release/build-debs.sh first" >&2
		exit 2
	fi
else
	DEB=$INPUT
fi

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
STAGE=$WORK/stage
mkdir -p "$STAGE"
dpkg-deb -x "$DEB" "$STAGE"

# Modes are checked here as well as in the release suite, because a program that
# is not executable fails at login rather than at test time, and this is the
# first place the installed tree is exercised end to end. The three daemons are
# the ones the session starts: a unit whose binary is not executable fails once
# per login with a code nobody reads.
for program in usr/bin/zenith usr/bin/zenith-hub usr/bin/zenith-shell \
		usr/bin/zenith-hubd usr/bin/zenith-wallpaper \
		usr/share/zenith/session/zenith-greeter; do
	if [ ! -x "$STAGE/$program" ]; then
		echo "doctor_test: $program is not executable in the installed tree" >&2
		exit 1
	fi
done

# The two entry points the session files name, at the paths they name them at.
#
# The doctor walks one tree and checks one entry point, so it cannot notice a
# package that dropped the Hub's entry file: the tree it does walk would still
# pass. The cost of not noticing is a window that fails at login with nothing
# but a shell error in the journal, and the session file that starts it is
# installed and executable, so every other check here would be green.
for entry in usr/share/zenith/shell/shell.qml usr/share/zenith/shell/hub.qml; do
	if [ ! -f "$STAGE/$entry" ]; then
		echo "doctor_test: $entry is not in the installed tree" >&2
		exit 1
	fi
done

# The same for every other window. Each has a launcher that names its entry file,
# and the launchers are what is read here rather than a list of windows, so a
# window added next month is covered by being added.
for entry in $(grep -ho '/usr/share/zenith/shell/[A-Za-z0-9_.-]*\.qml' \
		"$STAGE"/usr/bin/zenith-* "$STAGE"/usr/share/zenith/session/zenith-* 2>/dev/null | sort -u); do
	if [ ! -f "$STAGE$entry" ]; then
		echo "doctor_test: a launcher starts $entry, which is not in the installed tree" >&2
		exit 1
	fi
done

# The config tree is the half of doctor that needs the base to have been
# materialized once, exactly as a machine would after install.
"$STAGE/usr/bin/zenith" materialize --root "$STAGE" >/dev/null

# The session identity is the environment's, and a runner's is not Zenith's.
# Everything else the portal check reads comes from the stage.
OUTPUT=$WORK/doctor.out
XDG_CURRENT_DESKTOP=Zenith "$STAGE/usr/bin/zenith" doctor --root "$STAGE" >"$OUTPUT" 2>&1 || true
cat "$OUTPUT"

fail=0

# ------------------------------------------------------------------ the pass
#
# Each line names the fault it exists to catch. If one of these disappears from
# doctor's output the check fails, because a rule that stopped running and a
# rule that passes look identical in a green log.
must_contain() {
	if ! grep -Fq "$1" "$OUTPUT"; then
		echo "doctor_test: expected to find: $1" >&2
		fail=1
	fi
}

# For a line whose numbers change whenever a lane adds a setting, a page or a
# command: the sentence is the claim, the count is not.
must_match() {
	if ! grep -Eq "$1" "$OUTPUT"; then
		echo "doctor_test: expected to find a line matching: $1" >&2
		fail=1
	fi
}

must_not_contain() {
	if grep -Fq "$1" "$OUTPUT"; then
		echo "doctor_test: should not have found: $1" >&2
		fail=1
	fi
}

# The shell's source, checked against the schemas where GLib actually puts them.
#
# One window and not two. The Hub's entry file sits beside the shell's in the
# same tree, because both are drawn from the same component kit and the kit's
# imports name nothing from a root of shell/hub/ — so the doctor walks one
# directory and checks one entry point. A second pass over the same directory
# would not be evidence about a second window; it would be the same check
# counted twice. That the Hub's entry file is in the tree is stated separately,
# by the entry-point list above.
must_contain "source checks pass for 1 window(s)"
must_not_contain "declares no schema id"
must_not_contain "settings unreachable"
must_not_contain "contains no .gschema.xml"
must_not_contain "no surface can reach it"

# Every setting has a page, every page a command, every type a control.
must_match 'all [0-9]+ setting\(s\) have a page'
must_not_contain "no control is mapped to this setting type"

# Every zenith command the compositor's configuration runs is in the package. A
# bind that calls a script nobody installed runs, fails and says nothing, and
# this is the section of the doctor that reads the installed tree for it.
must_match 'all [0-9]+ zenith command\(s\) the compositor runs are installed'

# The overlay materializes into the extracted tree rather than reporting a tree
# full of files that are missing from it.
must_contain "no drift: the config tree matches the base"
must_not_contain "missing "
must_not_contain "materialize\` fixes them"

# The portal configuration the package ships, found where portals.conf(5) says
# the dispatcher looks for it — the directory gnome-portals.conf sits in.
must_contain "configuration     system default"
must_not_contain "no portal configuration was found"

# --------------------------------------------------------- allowed, and why
#
# Lines under these headings that are not in the list are a regression.
allowed='login manager preferences not installed; the package that should write it did not
zenith-shell.service   ExecStart points at /usr/bin/quickshell, which does not exist
zenith-portal.service  ExecStart points at /usr/libexec/xdg-desktop-portal, which does not exist
problem           named backend "hyprland" is not installed; nothing implements org.freedesktop.impl.portal.ScreenCast
problem           named backend "hyprland" is not installed; nothing implements org.freedesktop.impl.portal.Screenshot
problem           named backend "gtk" is not installed; nothing implements org.freedesktop.impl.portal.FileChooser
problem           named backend "gtk" is not installed; nothing implements org.freedesktop.impl.portal.OpenURI
problem           named backend "gtk" is not installed; nothing implements org.freedesktop.impl.portal.Print
problem           named backend "lxqt" is not installed; nothing implements org.freedesktop.impl.portal.Secret
problem           named backend "gtk" is not installed; nothing implements org.freedesktop.impl.portal.Access'

# is_allowed trims doctor's leading spaces and padded column name, because the
# list names the finding and not the formatting.
is_allowed() {
	trimmed=$(printf '%s\n' "$1" | sed 's/^[[:space:]]*//')
	printf '%s\n' "$allowed" | grep -Fxq "$trimmed"
}

# Portal faults are uniformly prefixed, so the whole class is checkable without
# having to recognise the informational lines beside them.
grep '^  problem ' "$OUTPUT" | while IFS= read -r line; do
	if ! is_allowed "$line"; then
		echo "doctor_test: portal problem the allowlist does not explain:" >&2
		printf '%s\n' "$line" >&2
		printf 'unbalanced\n' >"$WORK/fail"
	fi
done

# Registrations: the heading, then one line per registration until the section
# ends. Every line is a finding — there is no pass line to exempt.
#
# The extraction is checked rather than assumed. An awk that does not parse here
# once produced an empty section, which the loop below then reported no findings
# for, and the script passed while checking nothing: a test that cannot fail is
# the one result that must never be printed.
registrations=$(awk 'f && /^$/{f=0} /^Registrations$/{f=1; next} f' "$OUTPUT") || {
	echo "doctor_test: could not read the registrations section" >&2
	fail=1
}
if [ -z "$registrations" ]; then
	echo "doctor_test: doctor printed no registrations section" >&2
	fail=1
fi

printf '%s\n' "$registrations" |
	while IFS= read -r line; do
		[ -z "$line" ] && continue
		if ! is_allowed "$line"; then
			echo "doctor_test: a registration problem the allowlist does not explain:" >&2
			printf '%s\n' "$line" >&2
			printf 'unbalanced\n' >"$WORK/fail"
		fi
	done

if [ -f "$WORK/fail" ]; then
	fail=1
fi

if [ "$fail" -ne 0 ]; then
	echo "doctor_test: FAILED against $DEB" >&2
	exit 1
fi
echo "doctor_test: the installed package passes its own doctor"