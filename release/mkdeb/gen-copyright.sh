#!/bin/sh
# The package's copyright file, emitted rather than hand-kept.
#
# The set of vendored files is already written down once — in
# shell/vendor/vendor/manifest.yaml, vendor.sh's own output — and a second list
# kept by hand is a second list that diverges: stanza patterns that approximate
# the manifest with directory globs also claim the directories Zenith grew
# inside the vendored tree, and then the copyright lies about whose code it is.
# The 2026-10-23 declaration failure was this divergence showing itself as a
# release check.
#
# So the file is generated, from the two inputs that are authoritative:
#
#   - the manifest, which names every vendored path under the component that
#     claims it, together with the component's SPDX identifier, copyright line
#     and licence text's own file;
#   - the prose below, which is the licence text each stanza reads as, and is
#     not derived from anything.
#
# Everything the manifest does not list is Zenith's own, and the catch-all
# stanza claims it GPL-3.0-or-later. The build runs this file and ships its
# output; `release_test.sh` runs it again and compares against the copy in the
# checkout, so a copyright that has drifted from the manifest is a failure,
# not a quiet difference.
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
MANIFEST="$REPO_ROOT/shell/vendor/manifest.yaml"

# The header, and the catch-all. The override paragraph stays last on purpose:
# the format resolves a file to the last stanza that names it, and these three
# paths are Zenith's own files placed at paths the vendored components use,
# which the enumerated stanzas would otherwise still claim GPL-3 — because
# their paths sit under directories the manifest largely covers. The enumeration
# is exact, but saying twice is harmless and the paragraph is the only place
# the deliberate trick is explained to a reader of the installed copy.
cat <<'HEADER'
Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/
Upstream-Name: Zenith OS
Source: https://github.com/mohitjoer/Zenith-OS
Comment:
 This package ships three kinds of work, and they are attributed separately
 below.
 .
 Zenith's own code — the control tool, the session, and the Quickshell
 configuration Zenith writes — is GPL-3.0-or-later.
 .
 The shell's configuration is a vendored copy of upstream components,
 redistributed unmodified and each pinned to a commit, enumerated by the
 stanzas below from the manifest the build consults. Every vendored file is
 byte-identical to the commit named in it, and `zenith doctor` checks the
 installed tree against /usr/share/zenith/shell/vendor/manifest.yaml. The same
 facts are written out for a reader in /usr/share/doc/zenith/vendor-manifest.md.
 .
 The SPDX identifiers of the licences used here are GPL-3.0-or-later,
 GPL-3.0-only and Apache-2.0.

Files: *
Copyright: 2026 the Zenith OS contributors
License: GPL-3+
 This program is free software: you can redistribute it and/or modify it under
 the terms of the GNU General Public License as published by the Free Software
 Foundation, either version 3 of the License, or (at your option) any later
 version.
 .
 This program is distributed in the hope that it will be useful, but WITHOUT ANY
 WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
 PARTICULAR PURPOSE. See the GNU General Public License for more details.
 .
 On Debian systems, the complete text of the GNU General Public License version
 3 can be found in "/usr/share/common-licenses/GPL-3". It is also shipped with
 this package, in "/usr/share/doc/zenith/GPL-3".
HEADER

# The vendored components' stanzas, from the manifest. One stanza per
# component, in the order the manifest lists them; the enumeration is the
# manifest's own file list, plus the component's licence text, because the
# licence text is a file the package installs and the claims the reader should
# trust are the component's, not the publisher's.
awk -v manifest="$MANIFEST" '
# The default field separator is deliberate: it skips leading whitespace, so
# the manifest line
#
#     licence_file: shell/vendor/COPYING
#
# yields $1 = licence_file: and $2 = shell/vendor/COPYING, and a component
# name arrives as $3. An explicit separator like "[ \t]+" makes the leading
# indentation its own field instead, and every column index shifts by one.

function short_name(id) {
    if (id == "GPL-3.0-only") return "GPL-3"
    if (id == "GPL-3.0-or-later") return "GPL-3+"
    if (id == "Apache-2.0") return "Apache-2.0"
    return ""
}

function flush() {
    if (name == "") return
    licence = short_name(spdx)
    if (licence == "") {
        printf "unknown licence identifier in the manifest: %s\n", spdx > "/dev/stderr"
        exit 1
    }
    print ""
    print "Files:"
    for (i = 1; i <= count; i++)
        printf " usr/share/zenith/shell/%s\n", path[i]
    if (licence_file != "")
        printf " usr/share/zenith/shell/%s\n", licence_file
    printf "Copyright: %s\n", copyright
    if (licence == "GPL-3") {
        printf ("License: GPL-3\n"                       \
                " The vendored \"%s\" component is licensed under the GNU General Public License,\n" \
                " version 3 only. The complete text of the licence is shipped with this package\n" \
                " in \"/usr/share/doc/zenith/GPL-3\" and, on Debian systems, can be found in\n" \
                " \"/usr/share/common-licenses/GPL-3\".\n"), name
    } else if (licence == "Apache-2.0") {
        printf ("License: Apache-2.0\n"                  \
                " The vendored \"%s\" component is licensed under the Apache License, version\n" \
                " 2.0. The complete text of the licence is shipped with this package in\n" \
                " \"/usr/share/doc/zenith/Apache-2.0\" and, on Debian systems, can be found in\n" \
                " \"/usr/share/common-licenses/Apache-2.0\".\n"), name
    } else {
        printf ("License: %s\n"                          \
                " The vendored \"%s\" component is dual-licensed under the terms named above.\n"), licence, name
    }
}

/^  - name: /       { flush(); name = $3; count = 0; spdx = ""; licence_file = ""; copyright = ""; next }
# The manifest writes the path of the licence text itself the way the
# repository spells it — `shell/vendor/COPYING` — while the file entries it
# lists are recorded shell-relative. The pattern entries install at
# usr/share/zenith/shell/<path>, so a `shell/` prefix left on the licence
# file would put the claim on a path that does not exist, and the licence
# text would fall through to the catch-all — GPL-3+ — the one licence that is
# wrong for the licence texts: the text is carried beside the component, and
# the component is not the publisher of that text.
/^    licence_file: / { licence_file = $2; sub(/^shell\//, "", licence_file); next }
/^    copyright: /  { line = $0; sub(/^    copyright: /, "", line); copyright = line; next }
/^    spdx: /       { line = $0; sub(/^    spdx: /, "", line); spdx = line; next }
/^      - path: /   { line = $0; sub(/^      - path: /, "", line); count++; path[count] = line; next }
END { flush() }
' "$MANIFEST"

# The override stanza. Last, because the format resolves a file to the last
# stanza that names it.
cat <<'OVERRIDE'

Files:
 usr/share/zenith/shell/modules/common/Appearance.qml
 usr/share/zenith/shell/modules/common/Config.qml
 usr/share/zenith/shell/modules/common/widgets/AppIcon.qml
Copyright: 2026 the Zenith OS contributors
License: GPL-3+
 These three files sit at the paths the vendored component uses for its own
 palette, its own settings and one of its widgets, and they are Zenith's, not
 upstream's: Zenith supplies them so the vendored components read Zenith's
 colours and Zenith's settings without a vendored file being edited. The third
 is an application icon drawn with Quickshell's own image type, because
 upstream's is a KDE Frameworks widget and this desktop does not carry KDE
 Frameworks for one icon.
 .
 This paragraph is last because the format resolves a file to the last
 paragraph that matches it, and the paragraph above matches these two paths
 as well. A paragraph that is meant to override another one has to come after
 it.
OVERRIDE
