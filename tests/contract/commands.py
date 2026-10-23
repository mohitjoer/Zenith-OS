#!/usr/bin/env python3
"""Every external command the tree invokes must come from somewhere.

Scans QML/JS (`command: [...]`, execDetached, Spawn.run, `sh -c` strings), the
Hyprland confs (exec, exec-once, bind ... exec), shell scripts, Go exec.Command
and .desktop Exec= lines (extract.py), and resolves each bare command against:
  1. a build line that installs it to /usr/bin (release/build.d/*.build, plus
     whatever release/build-debs.sh still copies there by hand);
  2. a package named in release/deps.d/*.deps, release/mkdeb/control.in or
     system/packages.yaml (the command, or its package via ALIAS below);
  3. the base layer: owned by an Essential / required / important package or by
     one ubuntu-minimal depends on (asked of dpkg, so Ubuntu only);
  4. tests/contract/optional.txt: `name  # how the caller is gated`;
  5. a `# unavailable: TOOL (fallback)` note in a .deps file.
Absolute /usr/share/zenith/... paths must exist in the repo or in a build line.

  commands.py            print UNRESOLVED (command, sites, file:line), exit 1 if any
  commands.py --report   same, exit 0
  commands.py --selftest

A trailing * means the same file probes for the command (command -v, which,
LookPath): likely gated, a candidate for optional.txt rather than a missing dep.
"""
import glob
import os
import re
import subprocess
import sys

sys.dont_write_bytecode = True
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import extract  # noqa: E402

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
BINDIRS = ("/usr/bin", "/bin", "/usr/sbin", "/sbin", "/usr/local/bin")
# Only the commands whose package is not named after them; the rest match by name.
ALIAS = {
    "qs": "quickshell", "wl-copy": "wl-clipboard", "wl-paste": "wl-clipboard", "zbarimg": "zbar-tools",
    "tesseract": "tesseract-ocr", "magick": "imagemagick", "convert": "imagemagick",
    "identify": "imagemagick", "nvim": "neovim", "rsvg-convert": "librsvg2-bin", "trash-put": "trash-cli",
    "secret-tool": "libsecret-tools", "bsdtar": "libarchive-tools", "iwctl": "iwd",
    "sensors": "lm-sensors", "ffprobe": "ffmpeg", "ffplay": "ffmpeg", "makoctl": "mako-notifier",
    "nmcli": "network-manager", "bluetoothctl": "bluez", "wpctl": "wireplumber", "pactl": "pulseaudio-utils",
    "pw-cli": "pipewire-bin", "pw-dump": "pipewire-bin", "pw-metadata": "pipewire-bin",
    "hyprctl": "hyprland", "xdg-open": "xdg-utils", "xdg-settings": "xdg-utils", "fc-list": "fontconfig",
    "notify-send": "libnotify-bin", "gsettings": "libglib2.0-bin", "gio": "libglib2.0-bin",
    "glib-compile-schemas": "libglib2.0-bin", "update-desktop-database": "desktop-file-utils",
    "powerprofilesctl": "power-profiles-daemon", "killall": "psmisc", "lspci": "pciutils",
    "fprintd-list": "fprintd", "fprintd-delete": "fprintd", "rfkill": "rfkill", "nvidia-smi": "nvidia-utils",
}


def read(pattern):
    """(file, lineno, line) for the non-blank lines of every file matching pattern."""
    for f in sorted(glob.glob(os.path.join(ROOT, pattern))):
        with open(f, errors="replace") as fh:
            for n, line in enumerate(fh, 1):
                if line.strip():
                    yield os.path.relpath(f, ROOT), n, line.rstrip("\n")


def providers():
    bins, pkgs, unavail, bad = set(), set(), set(), []
    for f, n, line in read("release/build.d/*.build"):
        if line.lstrip().startswith("#"):
            continue
        m = re.match(r"\s*(\S+)\s*->\s*(\S+)", line)
        if not m:
            bad.append(f"{f}:{n}: not `src -> dst`: {line.strip()}")
            continue
        src, dst = m.groups()
        if dst.startswith("/usr/bin/"):
            bins.add(os.path.basename(dst.rstrip("/")))
        if not any(os.path.exists(os.path.join(ROOT, p, src.rstrip("/"))) for p in ("", "desktop", "desktop/cmd")):
            bad.append(f"{f}:{n}: source does not exist: {src}")
    for f, n, line in read("release/deps.d/*.deps"):
        m = re.match(r"\s*(?:depends|recommends|suggests)\s+([a-z0-9][a-z0-9.+-]*)", line)
        if m:
            pkgs.add(m.group(1))
        elif not line.lstrip().startswith("#"):
            bad.append(f"{f}:{n}: not `depends|recommends|suggests PACKAGE`: {line.strip()}")
        unavail.update(re.findall(r"unavailable:\s*([\w.+-]+)", line))
    for _, _, line in read("release/mkdeb/control.in"):
        if re.match(r"(Depends|Recommends|Suggests):", line):
            pkgs.update(re.findall(r"[a-z0-9][a-z0-9.+-]+", re.sub(r"\([^)]*\)", "", line.split(":", 1)[1])))
    for _, _, line in read("system/packages.yaml"):
        m = re.match(r"\s+- ([a-z0-9][a-z0-9.+-]*)\s*(#.*)?$", line)
        if m:
            pkgs.add(m.group(1))
    for _, _, line in read("release/build-debs.sh"):
        bins.update(re.findall(r"\$STAGE/usr/bin/([\w.+-]+)", line))
    return bins, pkgs, unavail, bad


def base_layer():
    """Names of the packages every Ubuntu machine has: Essential, required, important, ubuntu-minimal's."""
    try:
        q = subprocess.run(["dpkg-query", "-W", "-f", "${Package} ${Essential} ${Priority}\\n"],
                           capture_output=True, text=True).stdout
        dep = subprocess.run(["dpkg-query", "-W", "-f", "${Depends}", "ubuntu-minimal"],
                             capture_output=True, text=True).stdout
    except OSError:
        return set()
    pk = {f[0] for f in map(str.split, q.splitlines()) if len(f) == 3 and (f[1] == "yes" or f[2] in ("required", "important"))}
    return pk | set(re.findall(r"[a-z0-9][a-z0-9.+-]+", re.sub(r"\([^)]*\)", "", dep)))


def owners(names):
    """command -> packages that install it in a standard bin dir (dpkg -S; installed packages only)."""
    paths = [f"{d}/{n}" for n in names for d in BINDIRS]
    try:
        out = subprocess.run(["dpkg", "-S", *paths], capture_output=True, text=True).stdout
    except OSError:
        return {}
    res = {}
    for line in out.splitlines():
        pk, _, path = line.rpartition(": ")
        res.setdefault(os.path.basename(path), set()).update(p.split(":")[0] for p in pk.split(", "))
    return res


def path_ok(path):
    m = re.match(r"/usr/share/zenith/(shell|session|hyprland)/(.*)", path)
    if m:
        return os.path.exists(os.path.join(ROOT, {"hyprland": "session/hyprland"}.get(m.group(1), m.group(1)), m.group(2)))
    return any(path == re.sub(r"^\s*\S+\s*->\s*(\S+).*", r"\1", l) for _, _, l in read("release/build.d/*.build"))


def main(argv):
    if "--selftest" in argv:
        return selftest()
    r = extract.scan_tree(ROOT)
    bins, pkgs, unavail, bad = providers()
    optional = {m.group(1) for _, _, l in read("tests/contract/optional.txt") if (m := re.match(r"\s*([^\s#]+)", l))}
    inv = [i for i in r.invocations if not i.dynamic]
    own, base = owners({i.name for i in inv if not i.path or i.path.rsplit("/", 1)[0] in BINDIRS}), base_layer()
    miss = {}
    for i in inv:
        if i.path and i.path.rsplit("/", 1)[0] not in BINDIRS:
            ok = path_ok(i.path) if i.path.startswith("/usr/share/zenith/") else os.path.exists(i.path)
        else:
            n = i.name
            ok = (n in bins or n in pkgs or ALIAS.get(n) in pkgs or n in optional or n in unavail
                  or n in ("awk", "which")    # update-alternatives symlinks: no dpkg owner, mawk/debianutils are base
                  or bool(own.get(n, set()) & (base | pkgs)))
        if not ok:
            miss.setdefault(i.path or i.name, []).append(i)
    for name, sites in sorted(miss.items()):
        probed = any(s.name in r.probes.get(s.file, ()) for s in sites)
        locs = ", ".join(f"{s.file}:{s.line}" for s in sites[:3]) + (f" (+{len(sites) - 3})" if len(sites) > 3 else "")
        print(f"UNRESOLVED {name}{'*' if probed else ''}  x{len(sites)}  {locs}")
    for b in bad:
        print("BAD HANDOFF", b)
    for e in r.errors:
        print("SCAN ERROR", e)
    print(f"commands: {len(miss)} unresolved of {len({i.name for i in inv})} distinct, "
          f"{sum(i.dynamic for i in r.invocations)} dynamic sites not analysable, {r.files_scanned} files, {len(bad)} bad hand-offs")
    return 0 if "--report" in argv or not (miss or bad) else 1


def selftest():
    r = extract.ScanResult()
    extract.scan_qml("t.qml", 'P { command: ["flock", "-n", "/l", "qs", "-p", "x"] }\n'
                     'f() { Quickshell.execDetached(["sh", "-c", `a | b && $(c) ${d}`]); }', r)
    names = {i.name for i in r.invocations}
    assert {"flock", "qs", "sh", "a", "b", "c"} <= names, names
    assert any(i.dynamic for i in r.invocations)
    r = extract.ScanResult()
    extract.scan_conf("t.conf", "$t = ghostty\nbind = SUPER, T, exec, $t -e btop\nexec-once = /usr/share/zenith/x\n", r)
    assert {i.name for i in r.invocations} >= {"ghostty", "btop", "x"}
    print("commands selftest ok")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
