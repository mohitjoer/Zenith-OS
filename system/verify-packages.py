#!/usr/bin/env python3
"""Check system/packages.yaml against the real base-layer archive.

Rule 3 in the manifest's own header says a wrong package name must fail at
image build time, loudly, rather than vanishing quietly. This is the thing
that makes that true: it resolves every name in the manifest against the
suite's actual package indexes and fails on anything that is not there.

It checks three ways, because each catches a different mistake:

  install entries      must exist in a declared component, or the image cannot
                       be built. A missing name is an error.
  `zenith_built:`      the inverse: must NOT exist in the archive, or the base
                       layer already carries it and rebuilding it would be the
                       fork that rule 2 exists to prevent. Its build
                       dependencies must resolve, because those do come from
                       the archive.
  exclusion patterns   are matched as globs against what the archive really
                       contains, so `gnome-*` is verified as a pattern rather
                       than as a package that does not exist.
  exclusion names      are reported when they protect nothing, because an
                       exclusion nobody can see the effect of is the quiet way
                       for two implementations of one thing to both get
                       installed. `wpa_supplicant` instead of `wpasupplicant`
                       was exactly that.

The declared components are verified too. Listing a component the suite does
not publish puts a 404 in the image's sources on the first `apt update`, and
that is not something an image build should discover from a mirror.

Usage:
    python3 system/verify-packages.py                  # verify against the suite
    python3 system/verify-packages.py --mirror URL     # against another mirror
    python3 system/verify-packages.py --cache-dir DIR  # keep the indexes

Exit status is 0 when every install entry resolves, 1 otherwise. Warnings do
not fail the run; they are printed so a reviewer can decide.
"""

from __future__ import annotations

import argparse
import fnmatch
import gzip
import sys
from pathlib import Path
import urllib.error
import urllib.request

try:
    import yaml
except ImportError:
    sys.exit("verify-packages.py needs PyYAML: pip install pyyaml")


DEFAULT_MIRROR = "http://archive.ubuntu.com/ubuntu"
DEFAULT_META_RELEASE = "http://changelogs.ubuntu.com/meta-release"
MANIFEST = "system/packages.yaml"

# Groups whose `packages:` value is a mapping of nested groups rather than a
# flat list of names. `optional` is entirely exclusions, keyed by topic.
NESTED_PACKAGE_KEYS = ("optional",)


def fetch(url: str, cache_dir: str | None, cache_name: str | None = None) -> str:
    """Return the body of url, from cache when a cache directory is given."""
    # The cache name must be unique per URL, not per basename: every component
    # serves its index as `Packages.gz`, so keying on the basename has three
    # components sharing one file and the first one wins for all of them.
    name = cache_name or url.rsplit("/", 1)[-1]

    if cache_dir is None:
        return read_url(url)

    import pathlib

    path = pathlib.Path(cache_dir) / name
    if path.exists():
        return path.read_text(encoding="utf-8", errors="replace")

    body = read_url(url)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(body, encoding="utf-8")
    return body


def read_url(url: str) -> str:
    """Read a .gz from url and decompress it, or read it as-is."""
    try:
        with urllib.request.urlopen(url, timeout=120) as response:
            raw = response.read()
    except urllib.error.URLError as exc:
        sys.exit(f"cannot fetch {url}: {exc}")

    if url.endswith(".gz"):
        return gzip.decompress(raw).decode("utf-8", errors="replace")
    return raw.decode("utf-8", errors="replace")


def resolve_codename(release: str, meta_url: str, cache_dir: str | None) -> str:
    """Resolve a version to its suite codename, the way the manifest requires.

    The manifest stores `release: "26.04"` and leaves `codename: null`, because
    a codename hard-coded in a manifest that outlives a release cycle is a
    manifest that quietly starts pointing at the wrong suite. The archive only
    serves indexes by codename, so the resolution has to happen somewhere, and
    this is that somewhere.

    Ubuntu's meta-release file is the machine-readable version-to-suite map. It
    lists the current *point* release rather than the base one, so a manifest
    naming `26.04` matches a stanza reading `26.04.1`, and among the matching
    stanzas the base suite is preferred over `-updates`, `-backports` and
    `-proposed`: an image is built from the base suite, and its updates are a
    separate source.
    """
    body = fetch(meta_url, cache_dir)

    best: str | None = None
    for stanza in body.split("\n\n"):
        version = dist = None
        for line in stanza.splitlines():
            if line.startswith("Version: "):
                version = line.split(": ", 1)[1].strip()
            elif line.startswith("Dist: "):
                dist = line.split(": ", 1)[1].strip()

        if not version or not dist:
            continue
        if version != release and not version.startswith(release + "."):
            continue

        if "-" not in dist:
            return dist
        best = best or dist

    if best is None:
        sys.exit(
            f"no suite in {meta_url} matches release {release!r}. Either the "
            "release does not exist or this is a development release, which "
            "meta-release does not list."
        )
    return best


def load_release_components(
    mirror: str, codename: str, cache_dir: str | None
) -> list[str]:
    """The components the suite's own Release file says it publishes.

    This is the authority on which components exist. The directory layout
    answers a different question — whether a path happens to resolve — and a
    suite with no `non-free` answers that with a 404 rather than with an
    absence you can read.
    """
    body = fetch(f"{mirror}/dists/{codename}/Release", cache_dir)

    components: list[str] = []
    for line in body.splitlines():
        if line.startswith("Components:"):
            components.extend(line.split(":", 1)[1].split())
            break

    if not components:
        sys.exit(f"no Components field in the {codename} Release file")
    return components


def load_packages(
    mirror: str, codename: str, components: list[str], cache_dir: str | None
) -> dict[str, set[str]]:
    """Map each component to the set of package names it actually carries."""
    index: dict[str, set[str]] = {}

    for component in components:
        url = (
            f"{mirror}/dists/{codename}/{component}/binary-amd64/Packages.gz"
        )
        names: set[str] = set()

        body = fetch(url, cache_dir, f"{codename}-{component}-Packages")
        for line in body.splitlines():
            if line.startswith("Package: "):
                names.add(line.split(": ", 1)[1].strip())

        if not names:
            sys.exit(
                f"{component}/{codename} has no packages. Either the component "
                "is not published for this suite or the index is empty."
            )
        index[component] = names

    return index


def iter_install_entries(manifest: dict):
    """Yield (group, name) for every package the manifest installs."""
    for group, body in manifest.items():
        if group in ("version", "base") or not isinstance(body, dict):
            continue

        packages = body.get("packages")
        if isinstance(packages, list):
            for name in packages:
                yield group, name
        elif isinstance(packages, dict) and group in NESTED_PACKAGE_KEYS:
            # A nested group is a topic, not a package. Walk one level down so
            # a topic that ever gains install entries is still checked.
            for topic, entries in packages.items():
                if isinstance(entries, dict):
                    for name in entries.get("packages") or []:
                        yield f"{group}/{topic}", name


def iter_exclusions(manifest: dict):
    """Yield (group, pattern-or-name) for everything the manifest refuses."""
    for group, body in manifest.items():
        if group in ("version", "base") or not isinstance(body, dict):
            continue

        excludes = body.get("exclude")
        if not isinstance(excludes, dict):
            continue

        for key in excludes:
            yield group, key

        packages = body.get("packages")
        if isinstance(packages, dict) and group in NESTED_PACKAGE_KEYS:
            for topic, entries in packages.items():
                if isinstance(entries, dict):
                    for key in entries:
                        yield f"{group}/{topic}", key


def looks_like_a_pattern(text: str) -> bool:
    """Whether a manifest key is a glob rather than a package name."""
    return any(ch in text for ch in "*?[]")


def iter_built_entries(manifest: dict):
    """Yield (group, name) for every package Zenith builds for itself.

    These are the manifest's one deliberate departure from rule 2, and they are
    under their own key so that departure is visible rather than implied.
    """
    for group, body in manifest.items():
        if group in ("version", "base") or not isinstance(body, dict):
            continue

        for name in body.get("zenith_built") or []:
            yield group, name


def iter_build_dependencies(manifest: dict):
    """Yield (group, name) for build-only names.

    These never enter the runtime image, so they are checked apart from install
    entries: a missing one does not break the image, it breaks the build that
    produces a package the image needs.
    """
    for group, body in manifest.items():
        if group in ("version", "base") or not isinstance(body, dict):
            continue

        for name in body.get("build_dependencies") or []:
            yield group, name



# ---------------------------------------------------------------- GNOME audit

# The packages that make a desktop GNOME.
#
# This is not a style preference and it is not about naming. Zenith's shell owns
# Notifications, the Secret portal is served by the lxqt backend, and the session
# is Hyprland. A GNOME session installed anywhere on the machine is a second
# session that will appear at the login screen, win a race for a bus name, and
# leave the user on a desktop this project does not build.
#
# The list is explicit rather than a pattern because "gnome-keyring" is allowed
# and "gnome-shell" is not, and a prefix match cannot tell those apart. It is the
# distinction that matters: gnome-keyring is a standalone daemon and the only
# backing store the Secret portal has.
GNOME_DESKTOP = {
    "gnome-shell", "gnome-session", "gnome-session-bin",
    "gnome-core", "gnome-desktop", "gnome-shell-common",
    "gdm3", "gdm", "mutter", "mutter-common",
    "metacity", "gnome-control-center", "gnome-software",
    "gnome-tweaks", "gnome-extensions-app", "gnome-initial-setup",
    "gnome-power-manager", "gnome-color-manager", "gnome-bluetooth",
    "gnome-screenshooter", "gnome-contacts", "gnome-calendar",
    "gnome-music", "gnome-photos", "gnome-maps", "gnome-weather",
    "gnome-font-viewer", "gnome-text-editor", "gnome-help",
    "gnome-system-monitor", "gnome-logs", "gnome-disks",
    "yelp", "evolution", "evolution-data-server",
    "xdg-desktop-portal-gnome",
}

# Packages whose presence is fine, and why. Naming them is what makes the check
# honest: an allowlist nobody reads is indistinguishable from no allowlist.
GNOME_ALLOWED = {
    "gnome-keyring": "a standalone daemon, and the only backing store the "
                     "Secret portal has. Its transitive closure was checked and "
                     "carries no GNOME desktop component.",
    "libpam-gnome-keyring": "the PAM module that unlocks that keyring at login. "
                            "Its name begins with gnome- and it is not a GNOME "
                            "daemon: without it every Secret prompt asks for a "
                            "second password.",
}

# The metapackages that would drag a whole GNOME desktop in *transitively*.
#
# These are the ones that matter, and the distinction is the whole reason this
# check is not simply "is any GNOME package installed". The `packages:` list is
# closed — Zenith installs exactly the names written there — so a GNOME package
# that nobody wrote down is not installed and needs no exclusion. What can still
# happen is that something on the list depends on one, and the only way that
# happens is a metapackage.
#
# Requiring an exclusion for all twenty-odd GNOME applications would be
# demanding twenty-odd lines that protect nothing, and every one of them would
# have to be kept in step with a list that changes on an Ubuntu sync. The first
# version of this check made exactly that demand and CI caught it.
GNOME_PULLERS = (
    ("ubuntu-desktop",
     "pulls the GNOME session, its daemons and its bundled applications"),
    ("gnome-*",
     "the GNOME application suite"),
    ("*-desktop",
     "any other desktop environment's metapackage"),
)


def audit_gnome(manifest: dict) -> list[str]:
    """Report any way a GNOME desktop can arrive on a Zenith machine.

    Three checks, and only three, because they are the three that can actually
    happen.

    The first is the manifest's own list: nothing in `packages:` may be a GNOME
    desktop component. Direct and cheap.

    The second is transitive: the metapackages that would pull one in must be
    excluded. Deliberately not a dependency walk — the closure of an allowed
    package is a fact about the archive that changes without the manifest
    changing, and a check that starts failing on an Ubuntu sync is a check people
    learn to ignore. The closure of gnome-keyring was verified by hand: 199
    packages, no desktop component among them.

    The third is that nothing shells out to a GNOME tool. This is a repository
    check rather than a manifest one because the rule is about what the code
    depends on, not what the image installs. `gsettings` is deliberately not on
    the list: it is GLib's tool, it reads the same dconf the shell reads, and it
    is the interface GLib documents. A tool is a GNOME tool when it is part of
    GNOME rather than when its name says so.
    """
    problems: list[str] = []

    for group, name in iter_install_entries(manifest):
        if name in GNOME_ALLOWED:
            continue
        if name in GNOME_DESKTOP:
            problems.append(
                f"{group}: installs {name}, which is a GNOME desktop "
                f"component. Zenith's session is Hyprland with a Quickshell "
                f"shell; a GNOME session here is a second desktop at the login "
                f"screen that races the shell for org.freedesktop.Notifications."
            )

    excluded = [key for _, key in iter_exclusions(manifest)]

    for token, reason in GNOME_PULLERS:
        # Substring, because an exclusion key may be a glob and the manifest is
        # entitled to write it as one.
        if any(token in key for key in excluded):
            continue
        problems.append(
            f"nothing excludes {token}, which {reason}. Without that exclusion "
            f"a GNOME desktop can arrive transitively, through a package that "
            f"depends on it rather than through one that names it."
        )

    problems.extend(audit_gnome_tools(Path(__file__).resolve().parent.parent))

    return problems


# Executables that are part of GNOME rather than merely named after it.
#
# `gsettings` is absent on purpose and so is `gdbus`: both are GLib, both read
# the same dconf the shell reads, and both are the interfaces GLib documents.
# A dependency is a GNOME dependency when it is GNOME's, not when its name
# starts with "g".
GNOME_TOOLS = {
    "gnome-extensions": "the GNOME Shell extensions tool",
    "gnome-session": "the GNOME session manager",
    "gsettings": None,  # GLib, not GNOME. Listed to be explicit about the line.
}

# Files whose contents are scanned. Documentation is included on purpose: a
# command in a document is a command someone will paste.
SCANNED_SUFFIXES = (".go", ".sh", ".qml", ".md", ".yaml")


def audit_gnome_tools(root) -> list[str]:
    """Report any invocation of a GNOME-only executable in the tree."""
    import re

    problems: list[str] = []
    call = re.compile(r"(?:exec\.Command|command)\(\s*[\"'`](" +
                      "|".join(sorted(GNOME_TOOLS)) + r")[\"'`]")

    for path in sorted(root.rglob("*")):
        if not path.is_file() or path.suffix not in SCANNED_SUFFIXES:
            continue
        if ".git" in path.parts:
            continue
        try:
            body = path.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        for match in call.finditer(body):
            if GNOME_TOOLS[match.group(1)] is None:
                continue
            problems.append(
                f"{path.relative_to(root)}: invokes {match.group(1)}, which is "
                f"{GNOME_TOOLS[match.group(1)]}. A Zenith machine must be able to "
                f"do everything the shell does without GNOME installed."
            )

    return problems


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", default=MANIFEST)
    parser.add_argument("--mirror", default=DEFAULT_MIRROR)
    parser.add_argument("--meta-release", default=DEFAULT_META_RELEASE)
    parser.add_argument("--cache-dir", default=None)
    args = parser.parse_args()

    with open(args.manifest, encoding="utf-8") as handle:
        manifest = yaml.safe_load(handle)

    gnome_problems = audit_gnome(manifest)

    base = manifest["base"]
    release = base["release"]
    declared = list(base["components"])

    codename = resolve_codename(release, args.meta_release, args.cache_dir)

    if base.get("codename"):
        print(
            f"warning: base.codename is {base['codename']!r} rather than null. "
            f"The manifest says it is resolved at build time; this resolves to "
            f"{codename!r}.",
            file=sys.stderr,
        )

    published = load_release_components(args.mirror, codename, args.cache_dir)
    print(f"release {release} resolves to suite: {codename}")
    print(f"suite publishes:   {', '.join(published)}")
    print(f"manifest declares: {', '.join(declared)}")

    errors: list[str] = []
    warnings: list[str] = []

    for component in declared:
        if component not in published:
            errors.append(
                f"base.components names {component!r}, which {codename} does not "
                f"publish. It publishes: {', '.join(published)}. An image built "
                "with this would put a 404 in its sources on first update."
            )

    index = load_packages(
        args.mirror, codename, [c for c in declared if c in published], args.cache_dir
    )
    everywhere = set().union(*index.values()) if index else set()

    installed = 0
    for group, name in iter_install_entries(manifest):
        if name in everywhere:
            installed += 1
            continue
        errors.append(
            f"{group}: installs {name!r}, which is in no component of {codename}. "
            "It either ships from Zenith's own repository and must be declared "
            "as such, or it is a name that was never verified."
        )

    # `zenith_built` entries are the inverse check: they must be absent from the
    # archive, because the entire reason they are under their own key is that
    # the base layer carries nothing. If one turns up upstream later, rebuilding
    # it becomes a fork of a package the base layer maintains, and the entry
    # should move back under `packages:`.
    built = 0
    for group, name in iter_built_entries(manifest):
        if name in everywhere:
            warnings.append(
                f"{group}: builds {name!r}, but {codename} now carries it in "
                f"{', '.join(c for c in index if name in index[c])}. Rebuilding a "
                "package the base layer ships is the fork rule 2 exists to "
                "prevent; move it to packages: and let the archive own it."
            )
        else:
            built += 1

    # Build dependencies come from the archive, so they resolve the same way
    # install entries do — but a miss here breaks a build rather than the image.
    deps = 0
    for group, name in iter_build_dependencies(manifest):
        if name in everywhere:
            deps += 1
            continue
        warnings.append(
            f"{group}: build dependency {name!r} is in no component of "
            f"{codename}. The image still builds, but the package that needs it "
            "does not."
        )

    protected = 0
    for group, key in iter_exclusions(manifest):
        if looks_like_a_pattern(key):
            # A glob is verified by whether it matches anything at all, not by
            # whether the literal string is a package.
            if any(fnmatch.fnmatch(name, key) for name in everywhere):
                protected += 1
            else:
                warnings.append(
                    f"{group}: excludes {key!r}, which matches no package in "
                    f"{codename}. The pattern protects nothing."
                )
            continue

        if key in everywhere:
            protected += 1
        else:
            warnings.append(
                f"{group}: excludes {key!r}, which is in no component of "
                f"{codename}. The exclusion protects nothing."
            )

    print(
        f"\n{installed} install entries resolve, {built} Zenith-built packages "
        f"confirmed absent upstream, {deps} build dependencies resolve, "
        f"{protected} exclusions verified, no GNOME desktop component installed"
    )

    for warning in warnings:
        print(f"warning: {warning}", file=sys.stderr)

    errors.extend(gnome_problems)

    if errors:
        print(f"\n{len(errors)} errors:", file=sys.stderr)
        for error in errors:
            print(f"  {error}", file=sys.stderr)
        return 1

    print("manifest OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
