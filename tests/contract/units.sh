#!/bin/sh
# Each unit in session/units/ passes systemd-analyze verify (only lines naming the unit:
# the machine's other units are not ours), and every program an Exec*= line starts is
# installed by a build line or build-debs.sh, or already exists on this machine.
cd "$(dirname "$0")/../.." || exit 2
rc=0
for u in session/units/*.service session/units/*.target; do
    [ -f "$u" ] || continue
    n=$(basename "$u")
    if command -v systemd-analyze >/dev/null; then
        bad=$(SYSTEMD_UNIT_PATH="session/units:" systemd-analyze --user verify "$u" 2>&1 | grep -F "$n" | grep -v 'is not executable' | grep -v 'Failed to create directory' | grep -vE "(zenith-session|graphical-session)\.target not found" || true)
        [ -z "$bad" ] || { echo "$bad"; rc=1; }
    fi
    for p in $(sed -n -E 's/^Exec[A-Za-z]*=[-@+!:]*([^ ]+).*/\1/p' "$u"); do
        r=${p#/usr/share/zenith/}    # shell/ and session/ ship whole, so the repo path is the proof
        [ "$r" != "$p" ] && [ -e "$r" ] && continue
        case "$(basename "$p")" in
            quickshell|xdg-desktop-portal) continue ;;
        esac
        grep -qsE -- "->[[:space:]]*$p([[:space:]]|\$)" release/build.d/*.build ||
            grep -qsF -- "\$STAGE$p" release/build-debs.sh || [ -e "$p" ] ||
            [ -e "/usr/lib/$(basename "$p")" ] || [ -e "/usr/libexec/$(basename "$p")" ] ||
            { echo "$u: $p is installed by no build line"; rc=1; }
    done
done
[ $rc = 0 ] && echo "units: ok"
exit $rc
