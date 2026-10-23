#!/bin/sh
# zenith-pkg-add against stand-in apt tools: the refusals happen before any
# prompt, an allowed change reaches pkexec, and "cancel" on stdin stops a
# download. The root half needs fakeroot (id -u must read 0) and says so when it
# is missing.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
bin=$(mktemp -d)
trap 'rm -rf "$bin"' EXIT
log=$bin/log
: >"$log"

cat >"$bin/apt-get" <<'EOF'
#!/bin/sh
echo "apt-get $*" >>"$LOG"
case $1 in
-s)
    case $4 in
    conflicting) echo "Inst conflicting (1)"; echo "Remv other [1]" ;;
    *) echo "Inst $4 (1)" ;;
    esac
    exit 0 ;;
esac
case $* in *Status-Fd*) echo "dlstatus:0:10.0000:Retrieving"; sleep 5 ;; esac
EOF
cat >"$bin/apt-cache" <<'EOF'
#!/bin/sh
for p; do
    case $p in
    snapstub) printf 'Package: snapstub\nPre-Depends: debconf, snapd (>= 2.54)\n\n' ;;
    plain|conflicting) printf 'Package: %s\nDepends: libc6\n\n' "$p" ;;
    esac
done
EOF
cat >"$bin/pkexec" <<'EOF'
#!/bin/sh
echo "pkexec $*" >>"$LOG"
exec fakeroot "$@"
EOF
chmod +x "$bin"/*
export LOG=$log PATH="$bin:$here:$PATH"

fail() { echo "FAIL: $*" >&2; exit 1; }
code() { rc=0; "$@" >/dev/null 2>&1 </dev/null || rc=$?; echo $rc; }

[ "$(code zenith-pkg-add -x)" = 2 ] || fail "a name that looks like an option was accepted"
[ "$(code zenith-pkg-add snapstub)" = 12 ] || fail "a snap launcher was installable"
[ "$(code zenith-pkg-add conflicting)" = 13 ] || fail "an install that removes other packages went ahead"
! grep -q '^pkexec' "$log" || fail "a refused change still asked for a password"

command -v fakeroot >/dev/null || { echo "skip the root half: fakeroot is not installed"; echo ok; exit 0; }

zenith-pkg-add plain >/dev/null 2>&1 || fail "a plain install failed"
grep -q '^pkexec .*zenith-pkg-add plain' "$log" || fail "the change did not go through pkexec"
grep -q '^apt-get -y .* install -- plain' "$log" || fail "apt-get was not given the package"

start=$(date +%s)
(printf 'cancel\n'; sleep 1) | zenith-pkg-add --status plain >/dev/null 2>&1 || true
[ $(($(date +%s) - start)) -lt 3 ] || fail "cancel did not stop the download"
echo ok
