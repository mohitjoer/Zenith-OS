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

    import os
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
        sys.exit(f"no Components field in the {release} Release file")
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
                f"{component}/{release} has no packages. Either the component "
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


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", default=MANIFEST)
    parser.add_argument("--mirror", default=DEFAULT_MIRROR)
    parser.add_argument("--meta-release", default=DEFAULT_META_RELEASE)
    parser.add_argument("--cache-dir", default=None)
    args = parser.parse_args()

    with open(args.manifest, encoding="utf-8") as handle:
        manifest = yaml.safe_load(handle)

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
        f"{protected} exclusions verified"
    )

    for warning in warnings:
        print(f"warning: {warning}", file=sys.stderr)

    if errors:
        print(f"\n{len(errors)} errors:", file=sys.stderr)
        for error in errors:
            print(f"  {error}", file=sys.stderr)
        return 1

    print("manifest OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
