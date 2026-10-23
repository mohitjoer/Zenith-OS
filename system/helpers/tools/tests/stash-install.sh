#!/usr/bin/env bash
# hermetic test for shell/scripts/stash-install.sh, ported from Ryoku's. stash
# makes dropped files launchable: AppImages + self-contained tarballs get a synth
# desktop entry; a .deb is a system package and must go through `apt-get install`
# via pkexec, never extracted into ~/.local where its /opt binary would not work.
# pkexec + flatpak stubbed; this asserts the dispatch without touching the system.
set -uo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)
SCRIPT="$ROOT/shell/scripts/stash-install.sh"
[[ -f $SCRIPT ]] || { echo "::error::missing $SCRIPT" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
fail=0
check() { if [[ $1 == "$2" ]]; then echo "  ok: $3"; else echo "::error::FAIL: $3 (got '$1' want '$2')" >&2; fail=1; fi; }
present() { if [[ -e $1 ]]; then echo "  ok: $2"; else echo "::error::FAIL: $2 (missing $1)" >&2; fail=1; fi; }
absent()  { if [[ ! -e $1 ]]; then echo "  ok: $2"; else echo "::error::FAIL: $2 ($1 exists)" >&2; fail=1; fi; }

# stubs. pkexec records args (escalation assertion) and fails on demand;
# notify-send, update-desktop-database just need to exist for presence checks.
stub="$work/bin"
mkdir -p "$stub"
cat >"$stub/pkexec" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >>"$work/pkexec.log"
[ -e "$work/PKEXEC_FAILS" ] && exit 1
exit 0
EOF
cat >"$stub/flatpak" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >>"$work/flatpak.log"
exit 0
EOF
for t in notify-send update-desktop-database; do printf '#!/bin/sh\nexit 0\n' >"$stub/$t"; done
chmod +x "$stub"/*
export PATH="$stub:$PATH"

# a real, minimal .deb (built, never installed): dpkg-deb -f reads its Package.
mkdeb() {
  local out="$1" name="$2" d="$work/debsrc"
  rm -rf "$d"; mkdir -p "$d/DEBIAN"
  printf 'Package: %s\nVersion: 1\nArchitecture: all\nMaintainer: test <t@example.com>\nDescription: fixture\n' "$name" >"$d/DEBIAN/control"
  dpkg-deb --build --root-owner-group "$d" "$out" >/dev/null 2>&1
}

run_stash() {
  rm -rf "${work:?}/home"
  HOME="$work/home" bash "$SCRIPT" >"$work/out" 2>&1
}

# case 1: .deb installs via pkexec apt-get install, not extracted.
s1="$work/stash1"; mkdir -p "$s1"
mkdeb "$s1/foo.deb" foo
: >"$work/pkexec.log"
STASH_DIR="$s1" run_stash
if grep -q "Installed foo" "$work/out"; then
  echo "  ok: reported the package name (foo)"
else
  echo "::error::FAIL: did not report package name; out: $(cat "$work/out")" >&2
  fail=1
fi
check "$(cat "$work/pkexec.log")" "apt-get install -y $s1/foo.deb" \
  ".deb handed to pkexec apt-get install"
absent "$work/home/.local/share/zenith-apps/foo" ".deb NOT extracted into ~/.local"
absent "$s1/foo.deb" "source removed from the stash after a successful install"
if grep -qx "@AUTH" "$work/out"; then
  echo "  ok: .deb install signals the deck to step aside (@AUTH)"
else
  echo "::error::FAIL: .deb install did not emit @AUTH" >&2; fail=1
fi

# case 2: generic tarball stays the extract path, never apt.
s2="$work/stash2"; mkdir -p "$s2"
g="$work/gen"; rm -rf "$g"; mkdir -p "$g"
printf '#!/bin/sh\necho hi\n' >"$g/plainbin"; chmod +x "$g/plainbin"
( cd "$g" && tar -czf "$s2/plainapp.tar.gz" plainbin )
: >"$work/pkexec.log"
STASH_DIR="$s2" run_stash
check "$(cat "$work/pkexec.log")" "" "generic tarball never escalated"
present "$work/home/.local/share/applications/plainapp.desktop" \
  "generic tarball still becomes a launcher entry"
if grep -qx "@AUTH" "$work/out"; then
  echo "::error::FAIL: a non-privileged install emitted @AUTH" >&2; fail=1
else
  echo "  ok: non-privileged install does not signal the deck"
fi

# case 3: flatpak bundle -> flatpak install --user (flatpak stubbed).
s3="$work/stash3"; mkdir -p "$s3"
: >"$s3/app.flatpak"
: >"$work/pkexec.log"; : >"$work/flatpak.log"
STASH_DIR="$s3" run_stash
if grep -qx "install --user --noninteractive $s3/app.flatpak" "$work/flatpak.log"; then
  echo "  ok: flatpak bundle handed to flatpak install --user"
else
  echo "::error::FAIL: flatpak bundle not installed; log: $(cat "$work/flatpak.log")" >&2
  fail=1
fi
check "$(cat "$work/pkexec.log")" "" "flatpak never escalated"

# case 4: ZENITH_STASH_KEEP=1 keeps the source after a successful install.
s4="$work/stash4"; mkdir -p "$s4"
mkdeb "$s4/keep.deb" keep
ZENITH_STASH_KEEP=1 STASH_DIR="$s4" run_stash
present "$s4/keep.deb" "ZENITH_STASH_KEEP=1 keeps the source in the stash"

# case 5: a failed install (polkit denied, apt error) leaves the source in place.
s5="$work/stash5"; mkdir -p "$s5"
mkdeb "$s5/broken.deb" broken
touch "$work/PKEXEC_FAILS"
STASH_DIR="$s5" run_stash || true
rm -f "$work/PKEXEC_FAILS"
present "$s5/broken.deb" "a failed install keeps its source in the stash"
grep -q "Install failed" "$work/out" && echo "  ok: a failed install says so" ||
  { echo "::error::FAIL: failed install was silent" >&2; fail=1; }

if (( fail )); then echo "stash-install: FAILED" >&2; exit 1; fi
echo "stash-install: all checks passed"
