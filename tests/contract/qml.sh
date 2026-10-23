#!/bin/sh
# qmllint over shell/ against tests/contract/qml.baseline.
#
# Quickshell's QML modules are compiled into /usr/bin/quickshell and no qmldir or
# qmltypes ships for them, so qmllint cannot resolve Quickshell, Quickshell.Io, ...
# and reports ~33k import / unqualified-access / missing-property warnings for code
# that loads fine. Those are noise and are not gated. What never depends on those
# types and always means "this file will not load" is a syntax-class diagnostic
# (an `Error:` line or a `[syntax...]` tag, e.g. a duplicated id). Those are counted
# per file; a file with more of them than its baseline line fails.
#
#   qml.sh            compare with the baseline, exit 1 on a regression
#   qml.sh --update   rewrite the baseline from the tree as it is now
#
# (To resolve the qs.* modules too, pass -I DIR where DIR/qs is a symlink to shell/;
# syntax errors do not need it.)
cd "$(dirname "$0")/../../shell" || exit 2
LINT=${QMLLINT:-/usr/lib/qt6/bin/qmllint}
BASE=../tests/contract/qml.baseline
[ -x "$LINT" ] || { echo "qml: skip, no qmllint at $LINT"; exit 0; }
now=$(mktemp) && trap 'rm -f "$now"' EXIT
find . -name '*.qml' -not -path './vendor/*' -not -path './i18n/*' -not -path './modules/ii/*' \
    -exec "$LINT" {} + 2>&1 | grep -E '^Error:|\[syntax[].]' |
    sed -E 's/^[A-Za-z]+: (\.\/)?([^:]+):.*/\2/' | sort | uniq -c | awk '{print $2, $1}' >"$now"
if [ "$1" = --update ]; then cat "$now" >"$BASE"; echo "qml: baseline now $(wc -l <"$BASE") file(s)"; exit 0; fi
awk 'NR == FNR { b[$1] = $2; next } $2 > b[$1] { print "qml: NEW syntax error(s) in " $1 ": " b[$1] + 0 " -> " $2; bad = 1 } END { exit bad }' \
    "$BASE" "$now" || exit 1
echo "qml: ok, $(awk '{n += $2} END {print n + 0}' "$now") syntax-class error(s), none beyond the baseline"
