#!/bin/sh
# One check for what the port changed, not a suite: the engine's capability
# report, the conf keys it writes, the verb that was dropped, and the launcher's
# dispatch. Nothing here starts QEMU or Quickshell.
#
#   sh desktop/cmd/zenith-vm/vm_test.sh
set -eu

here=$(cd "$(dirname "$0")" && pwd)
engine=$here/zenith-vmd
launcher=$here/../../../session/zenith-vm
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
fail() { echo "FAIL: $*" >&2; exit 1; }

# A PATH holding only what a bare machine has: no quickemu, no QEMU.
bare=$work/bare
mkdir "$bare"
for t in bash jq sed du cut head tail basename dirname cat date stat ls seq mkdir rm grep tr sort uniq timeout python3 env; do
    p=$(command -v "$t" 2>/dev/null) && ln -s "$p" "$bare/$t"
done
run() { env -i HOME="$work" XDG_DATA_HOME="$work/data" XDG_CACHE_HOME="$work/cache" ZENITH_VM_HOME="$work/vm" PATH="$bare" bash "$engine" "$@"; }

# 1. caps is JSON, carries the fields the window gates on, and is honest.
caps=$(run caps)
for k in quickemu quickget qemu kvm kvmDevice ovmf spice ready version; do
    echo "$caps" | jq -e "has(\"$k\")" >/dev/null || fail "caps has no $k: $caps"
done
echo "$caps" | jq -e '.quickemu == false and .ready == false' >/dev/null || fail "ready without quickemu: $caps"
echo "$caps" | jq -e '(.kvm | not) or .kvmDevice' >/dev/null || fail "usable /dev/kvm that does not exist: $caps"

# 2. An empty library lists as an empty array.
[ "$(run list | jq -c .)" = "[]" ] || fail "list of an empty library"

# 3. quickemu sources the conf as shell, so every key it holds must be a valid
#    shell name. Porting 'ryovm_os' with a blanket rename once made it 'zenith-vm_os'.
: >"$work/disc.iso"
run import probe "$work/disc.iso" linux ubuntu >/dev/null
conf=$work/vm/probe.conf
grep -q '^zenith_vm_os="ubuntu"' "$conf" || fail "the VM's catalogue slug was not stamped: $(cat "$conf")"
sh -c ". '$conf'" || fail "the conf does not source as shell"
[ "$(run list | jq -r '.[0].os')" = ubuntu ] || fail "list does not read the slug back"

# 4. Looking Glass is not set up on Zenith, so its verb is gone.
rc=0; run lg >/dev/null 2>"$work/err" || rc=$?
[ "$rc" = 2 ] || fail "the lg verb still exists (exit $rc)"
grep -q '|lg' "$work/err" && fail "usage still lists lg"

# 5. The launcher hands engine verbs on and opens the window for nothing else.
fake=$work/fake
mkdir "$fake"
printf '#!/bin/sh\necho "vmd $*"\n' >"$fake/zenith-vmd"
printf '#!/bin/sh\necho "qs $*"\n' >"$fake/quickshell"
chmod +x "$fake/zenith-vmd" "$fake/quickshell"
[ "$(PATH="$fake:$PATH" sh "$launcher" list foo)" = "vmd list foo" ] || fail "launcher did not forward an engine verb"
[ "$(PATH="$fake:$PATH" XDG_RUNTIME_DIR="$work" ZENITH_QUICKSHELL="$fake/quickshell" sh "$launcher")" = "qs -p /usr/share/zenith/shell/vm.qml" ] \
    || fail "launcher did not start the window with the VM entry"
rc=0; PATH="$fake:$PATH" ZENITH_QUICKSHELL="$work/missing" sh "$launcher" ui >/dev/null 2>&1 || rc=$?
[ "$rc" = 1 ] || fail "launcher with no Quickshell should exit 1, got $rc"

echo "ok"
