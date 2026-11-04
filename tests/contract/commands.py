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
Absolute /usr/share/asura/... paths must exist in the repo or in a build line.

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
    "qs": ["quickshell"], "wl-copy": ["wl-clipboard"], "wl-paste": ["wl-clipboard"], "zbarimg": ["zbar", "zbar-tools"],
    "tesseract": ["tesseract", "tesseract-ocr"], "magick": ["ImageMagick", "imagemagick"], "convert": ["ImageMagick", "imagemagick"],
    "identify": ["ImageMagick", "imagemagick"], "nvim": ["neovim"], "rsvg-convert": ["librsvg", "librsvg2-bin"], "trash-put": ["trash-cli"],
    "secret-tool": ["libsecret-tools", "libsecret"], "bsdtar": ["libarchive", "libarchive-tools"], "iwctl": ["iwd"],
    "sensors": ["lm_sensors", "lm-sensors"], "ffprobe": ["ffmpeg"], "ffplay": ["ffmpeg"], "makoctl": ["mako-notifier", "mako"],
    "nmcli": ["NetworkManager", "network-manager"], "bluetoothctl": ["bluez"], "wpctl": ["wireplumber"], "pactl": ["pipewire", "pulseaudio-utils", "pipewire-pulse"],
    "pw-cli": ["pipewire", "pipewire-bin"], "pw-dump": ["pipewire", "pipewire-bin"], "pw-metadata": ["pipewire", "pipewire-bin"],
    "hyprctl": ["hyprland"], "xdg-open": ["xdg-utils"], "xdg-settings": ["xdg-utils"], "fc-list": ["fontconfig"],
    "notify-send": ["libnotify", "libnotify-bin"], "gsettings": ["glib", "libglib2.0-bin"], "gio": ["glib", "libglib2.0-bin"],
    "glib-compile-schemas": ["glib", "libglib2.0-bin"], "update-desktop-database": ["desktop-file-utils"],
    "powerprofilesctl": ["power-profiles-daemon"], "killall": ["psmisc"], "lspci": ["pciutils"],
    "fprintd-list": ["fprintd"], "fprintd-delete": ["fprintd"], "rfkill": ["rfkill"], "nvidia-smi": ["nvidia-utils", "nvidia"],
    "ssh": ["openssh", "openssh-client"], "ssh-add": ["openssh", "openssh-client"], "ssh-copy-id": ["openssh", "openssh-client"], "ssh-keygen": ["openssh", "openssh-client"],
    "qemu-img": ["qemu", "qemu-utils"], "quickget": ["quickemu"], "lsusb": ["usbutils"],
    "btrfs": ["btrfs-progs"], "grub-mkconfig": ["grub", "grub2-common"], "pkexec": ["polkit", "policykit-1"],
    "gtk-update-icon-cache": ["gtk+3", "gtk-update-icon-cache", "libgtk-3-bin"],
    "genisoimage": ["cdrtools", "xorriso"], "mkisofs": ["cdrtools", "xorriso"], "xorriso": ["xorriso"],
}

BASE_COMMANDS = {
    "bash", "sh", "cat", "chmod", "chown", "cp", "date", "dd", "df", "echo",
    "false", "ln", "ls", "mkdir", "mknod", "mktemp", "mv", "pwd", "readlink",
    "realpath", "rm", "rmdir", "sleep", "stty", "sync", "touch", "true", "uname",
    "base64", "basename", "dirname", "du", "env", "expr", "head", "id", "install",
    "md5sum", "printenv", "printf", "seq", "sha256sum", "sort", "stat", "stdbuf",
    "tail", "tee", "test", "timeout", "tr", "uniq", "wc", "whoami", "yes",
    "find", "xargs", "grep", "egrep", "fgrep", "sed", "tar", "gzip", "gunzip",
    "diff", "flock", "kill", "killall", "ps", "pkill", "pgrep", "free", "hostname", "setsid",
    "ip", "systemctl", "timedatectl", "localectl", "loginctl", "busctl", "systemd-run",
    "systemd-inhibit", "systemd-detect-virt", "which", "awk", "cut", "lsblk", "lscpu",
    "modinfo", "nice", "nproc", "numfmt", "od", "pidof", "reboot",
    "apt", "apt-get", "apt-cache", "dpkg", "dpkg-query", "dpkg-deb", "usermod",
    "dbus-update-activation-environment",
    "xbps-install", "xbps-query", "xbps-reconfigure", "xbps-remove", "xbps-rindex",
    "sv", "chpst", "runsv", "runsvdir", "file", "lspci", "lsusb", "rfkill",
    "grub-mkconfig", "pkexec", "btrfs", "git",
}
UBUNTU_BASE_COMMANDS = BASE_COMMANDS


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
    for _, _, line in read("system/packages-void.yaml"):
        m = re.match(r"\s+-\s+([a-zA-Z0-9][a-zA-Z0-9._+-]*)", line)
        if m:
            pkgs.add(m.group(1))
    for _, _, line in read("release/build-debs.sh"):
        bins.update(re.findall(r"\$STAGE/usr/bin/([\w.+-]+)", line))

    # Built Asura binaries & helper scripts
    cmd_dir = os.path.join(ROOT, "desktop", "cmd")
    if os.path.isdir(cmd_dir):
        for entry in os.listdir(cmd_dir):
            if os.path.isdir(os.path.join(cmd_dir, entry)):
                bins.add(entry)
    bins.update(["asura-vmd", "asura-vm-mon", "asura-vm-ssh", "os-ctl", "asura-installer", "void-installer"])

    for s in ("asura-hub", "asura-idle", "asura-pin", "asura-reload-cover", "asura-shot", "asura-vm", "asura-wallpaper-ui", "asura-welcome", "asura-greeter"):
        if os.path.isfile(os.path.join(ROOT, "session", s)):
            bins.add(s)
    session_scripts = os.path.join(ROOT, "session", "scripts")
    if os.path.isdir(session_scripts):
        for s in os.listdir(session_scripts):
            bins.add(s)

    shell_scripts = os.path.join(ROOT, "shell", "scripts")
    if os.path.isdir(shell_scripts):
        for s in os.listdir(shell_scripts):
            bins.add(s)

    for hdir in ("audio", "display", "input", "perf", "pkg", "tools"):
        hpath = os.path.join(ROOT, "system", "helpers", hdir)
        if os.path.isdir(hpath):
            for s in os.listdir(hpath):
                if not s.endswith(".test.sh"):
                    bins.add(s)
    for extra in ("prime-run", "fan-profile"):
        if os.path.isfile(os.path.join(ROOT, "system", "helpers", extra)):
            bins.add(extra)

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
    m = re.match(r"/usr/share/asura/(shell|session|hyprland)/(.*)", path)
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
            ok = path_ok(i.path) if i.path.startswith("/usr/share/asura/") else os.path.exists(i.path)
        else:
            n = i.name
            alias_match = False
            candidates = ALIAS.get(n)
            if candidates:
                if isinstance(candidates, str):
                    candidates = [candidates]
                alias_match = any(c in pkgs for c in candidates)
            ok = (n in bins or n in pkgs or alias_match or n in optional or n in unavail
                  or n in ("awk", "which")    # update-alternatives symlinks: no dpkg owner, mawk/debianutils are base
                  or n in BASE_COMMANDS
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
    extract.scan_conf("t.conf", "$t = ghostty\nbind = SUPER, T, exec, $t -e btop\nexec-once = /usr/share/asura/x\n", r)
    assert {i.name for i in r.invocations} >= {"ghostty", "btop", "x"}
    print("commands selftest ok")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
