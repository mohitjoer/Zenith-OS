# Third-party notices

The vendored tree in `shell/modules/` and `shell/services/` is upstream's code, except at the
few paths Zenith supplies itself. Some of it upstream took from other projects, and that code
carries its own attribution and its own licence. This file records what that is.

It exists to answer one question in one lookup: **which licence applies to this file?**

## The components

| Component | Upstream project | Pinned commit | Licence | Full text |
|---|---|---|---|---|
| `ii` | [end-4/dots-hyprland](https://github.com/end-4/dots-hyprland) | `2f2a376fdb2ad738bf55f693dd0b474b64bb0ab8` | GPL-3.0-only | [`shell/vendor/COPYING`](COPYING) |
| `shapes` | [end-4/rounded-polygon-qmljs](https://github.com/end-4/rounded-polygon-qmljs) | `e31ec4cb4ebf6a46b267f5c42eabf6874916fa16` | Apache-2.0 | [`shell/vendor/LICENSE.apache-2.0`](LICENSE.apache-2.0) |

`shell/vendor/COPYING` and `shell/vendor/LICENSE.apache-2.0` are byte-identical to the
upstream projects' own `LICENSE` files, copied by `sh shell/vendor/vendor.sh sync`.

Neither upstream `LICENSE` file carries a project-specific copyright line. The `ii` file is
the unmodified GNU GPL v3 text, which names only the Free Software Foundation as the holder of
the licence text itself; the `shapes` file is the unmodified Apache 2.0 text, whose
`Copyright [yyyy] [name of copyright owner]` line is the unfilled placeholder the licence
itself ships with. The copyright holders are recorded in
[`shell/vendor/manifest.yaml`](manifest.yaml) as the contributors of each upstream project.

## Which licence applies to which file

Every file in the vendored tree is covered by its component's licence **unless** it appears in
this table. A file that appears here carries an upstream credit of its own, and that credit
decides its licence.

| File | Came from | Licence | Copyright line in the file | Full text |
|---|---|---|---|---|
| `modules/common/Icons.qml` | caelestia-dots/shell | GPL-3.0-only | none stated | [`COPYING`](COPYING) |
| `modules/common/widgets/DirectoryIcon.qml` | caelestia-dots/shell, with modifications | GPL-3.0-only | none stated | [`COPYING`](COPYING) |
| `services/Brightness.qml` | caelestia-dots/shell, with modifications | GPL-3.0-only | none stated | [`COPYING`](COPYING) |
| `services/Network.qml` | caelestia-dots/shell | GPL-3.0-only | none stated | [`COPYING`](COPYING) |
| `modules/common/functions/levendist.js` | koeqaife/hyprland-material-you | GPL-3.0-only | none stated | [`COPYING`](COPYING) |
| `modules/common/functions/fuzzysort.js` | farzher/fuzzysort | MIT | `Copyright (c) 2018 Stephen Kamenar` | [inline below](#the-mit-licence) |
| `modules/common/functions/fuzzysort.js` — the priority queue inlined at the end of the file | lemire/FastPriorityQueue.js | Apache-2.0 | none stated | [`LICENSE.apache-2.0`](LICENSE.apache-2.0) |
| `services/MprisController.qml` | outfoxxed/nixnew | no licence granted; the author permits redistribution | none stated | none — see below |

The Copyright column quotes what the file says, and where it says nothing the column says so.
Only `fuzzysort.js` names a copyright holder anywhere in the vendored tree. In the
GPL-3.0 and Apache-2.0 files the copyright in the code is held by the contributors of the
project the file names; no file says so, and this file does not put words in its mouth.

`GPLv3` is what the files say. It is the GNU General Public License version 3, whose text is
installed at `shell/vendor/COPYING`, and it is the licence the component carries. Whether a
file's own grant runs to "or later" is a question about the upstream project's own headers,
and none of these files states one.

Everything else in the vendored tree — every other file under `shell/modules/` and
`shell/services/`, including `services/ai/` and the other helpers under `services/` —
**carries no attribution of its own** and inherits its component's licence: GPL-3.0-only for
the `ii` component, Apache-2.0 for the `shapes` component.

The files at upstream's paths that Zenith supplies itself are not vendored files and are not
in this table: `shell/GlobalStates.qml`, `shell/modules/common/Appearance.qml`,
`shell/modules/common/Config.qml` and `shell/modules/common/widgets/AppIcon.qml`. They are
Zenith's own code under Zenith's own licence, and they carry no upstream credit.

## The credits, as the files state them

These are quoted from the files themselves. Nothing below is inferred.

### `modules/common/Icons.qml`

```
// From https://github.com/caelestia-dots/shell (GPLv3)
```

### `modules/common/widgets/DirectoryIcon.qml`

```
// From https://github.com/caelestia-dots/shell with modifications.
// License: GPLv3
```

### `services/Brightness.qml`

```
// From https://github.com/caelestia-dots/shell with modifications.
// License: GPLv3
```

### `services/Network.qml`

```
// Took many bits from https://github.com/caelestia-dots/shell (GPLv3)
```

### `modules/common/functions/levendist.js`

```
// Original code from https://github.com/koeqaife/hyprland-material-you
// Original code license: GPLv3
// Translated to Js from Cython with an LLM and reviewed
```

That project's own repository is licensed GPL-3.0, which is the licence carried by the GPL-3.0
text in `shell/vendor/COPYING`.

### `modules/common/functions/fuzzysort.js`

```
// https://github.com/farzher/fuzzysort
// License: MIT | Copyright (c) 2018 Stephen Kamenar
// A copy of the license is available in the `licenses` folder of this repository
```

The file's own note about the `licenses` folder points at a directory in the **upstream**
repository. That directory was not vendored — the vendoring takes the panel family's own
directories, not the upstream project's licence folder — so the MIT text is not otherwise
installed in this repository. It is reproduced [inline below](#the-mit-licence) instead.

The same file carries one more credit, at its last line:

```
// Hacked version of https://github.com/lemire/FastPriorityQueue.js
```

That is a minified, modified copy of the priority queue, inlined rather than imported. The
file states no licence of its own for it. The upstream project is licensed Apache-2.0, and
that text is installed in this repository at `shell/vendor/LICENSE.apache-2.0`.

### `services/MprisController.qml`

```
// From https://git.outfoxxed.me/outfoxxed/nixnew
// It does not have a license, but the author is okay with redistribution.
```

The project has no licence file. Its author has stated in the file's own header that
redistribution is permitted, and that header travels with the code because the file is
byte-identical to upstream's copy of it.

## What GPL-3.0 asks for, and where it is satisfied

The GPL-3.0 obligations on the vendored tree are three, and all three are met.

**Keep the copyright and licence notices.** The notices are where upstream put them, in the
files themselves, and they are still there because every vendored file is byte-identical to
its pinned commit. This file records them a second time, in one place, for a reader who wants
the list rather than the archaeology.

**State the changes.** There are none to the vendored files themselves: that is what the
byte-identical rule means. The difference between what upstream ships and what Zenith ships is
the overlay files at upstream's paths — `shell/GlobalStates.qml`,
`shell/modules/common/Appearance.qml`, `shell/modules/common/Config.qml` and
`shell/modules/common/widgets/AppIcon.qml` — which are Zenith's own files, and the fact that
upstream's versions of those are not shipped. Every other byte in the tree is upstream's,
unchanged.

**Ship the corresponding source.** The corresponding source is **this repository**. The
vendored tree is here in full, at the pins recorded in `shell/vendor/manifest.yaml`, together
with the manifest, the licence texts and these notices — so the complete corresponding source
of everything under `shell/modules/` and `shell/services/` is in the same tree as the code
that uses it, and not somewhere else.

Apache-2.0 asks for the same three things, less the requirement to publish a source
offering: the licence text is installed at `shell/vendor/LICENSE.apache-2.0`, the changes are
none, and the source is this repository.

## Fonts and artwork

The vendored tree contains no font, no image and no artwork. It names fonts it expects to be
present on the system — a Material Symbols family and a Nerd Font — and Zenith's base layer
carries neither. The base layer supplies the icon fonts the shell actually renders, and those
are distribution packages with their own copyright files, outside this repository.

Nothing in the vendored tree is therefore covered by a font licence or an artwork licence, and
no such licence text is needed here. If an artwork or font set is ever vendored, its licence
text goes in this file with it.

## The MIT licence

Reproduced here because the MIT-licensed code in the vendored tree is the one whose licence
text is not otherwise installed in this repository. The upstream project's `licenses` folder,
which carries an MIT text of its own, was not vendored.

```
MIT License

Copyright (c) 2018 Stephen Kamenar

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
