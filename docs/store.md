# The store

ASURA Store is the package and customization showroom window and CLI data plane. It is a port of Ryoku's
RyoStore: software installs through native package managers (XBPS on Void Linux, apt on Debian/Ubuntu hosts)
and plugin managers (`asura plugin install`), and privilege escalation is safely handled through polkit.

The canonical standalone repository is hosted at **[github.com/ASURA-OS/asura-store](https://github.com/ASURA-OS/asura-store)**.
The tree in this repository keeps byte-identical mirrors of the standalone store:
- `desktop/cmd/asura-store/` mirrors `cmd/asura-store/`
- `shell/store/` and `shell/store.qml` mirror `ui/`
- `desktop/catalog/` mirrors `catalog/`

CI parity is automatically enforced on every push and PR via `.github/workflows/store-sync.yml`.

| Piece | Where | What it does |
|---|---|---|
| `asura-store` | `desktop/cmd/asura-store` (binary `/usr/bin/asura-store`) | The Go data plane: catalogue, search, check, install, remove, open, settings. |
| The window | `shell/store.qml`, `shell/store/` (`/usr/share/asura/store/ui`) | Quickshell showroom window: discover stage, search shelf, product overlays. |
| `asura-pkg-add`, `asura-pkg-remove` | `system/helpers/pkg` | Run package managers as root through `pkexec`. |
| `catalog/` | `desktop/catalog/` (`/usr/share/asura/store/catalog/`) | Local catalogue JSON (`plugins/`, `themes/`, `barstyles/`). A checkout update changes the shelf without a rebuild. |
| `session/asura-store` | `/usr/share/asura/session/asura-store` | Starts the window under a single-instance lock. |

## How an install happens

The window runs `asura-store install CATEGORY ID`. The store prints one
`{"stage","percent"}` JSON line per step on stdout — a negative percent means the
step has no number — and one sentence on stderr, which the window shows.

`apps` is native packages, and its privilege path is the host's:

1. On an Ubuntu host the `asura-pkg-add` helper exists. `asura-store` runs
   `asura-pkg-add --status vlc`, and before any prompt the helper simulates the
   change with `apt-get -s` and refuses two things: a change that would also
   remove other packages, and a package that only installs a snap (Ubuntu's
   `firefox`, `chromium-browser` and `thunderbird` are snap launchers, and
   ASURA does not use snap). The refusal is a sentence on stderr.
2. The helper re-runs itself through `pkexec`. polkit asks the session's
   authentication agent, which is `asura-shell`, for the password. Nothing in
   the store runs `sudo`.
3. As root the helper runs `apt-get install` with apt's machine status on
   stdout, and `asura-store` turns it into one
   `{"stage":"DOWNLOADING","percent":42}` line per step. The stages are
   `AUTHORIZING`, `DOWNLOADING`, then `INSTALLING` (or `REMOVING`).

On a Void host there is no helper: `pkexec xbps-install -y` (or `xbps-remove -y`)
runs directly, and the store brackets it with `AUTHORIZING` and `INSTALLING`
(or `REMOVING`), since xbps cannot number one transaction. A declined prompt, or
no agent to ask, ends `pkexec` with 126 or 127, and the store reports the same
declined-authorization sentence as the helper path.

Cancel is honoured until dpkg starts. The caller cannot signal a root process, so
`asura-store` writes `cancel` to the helper's stdin, which stops apt-get only while
it is downloading and only if no dpkg is running. After that the transaction
finishes, because stopping apt-get under dpkg leaves a half-configured system.
`asura-store` also ignores `SIGPIPE` and `SIGHUP`, so closing the window never
kills a transaction partway.

`plugins` installs nothing itself. The catalogue entry names the upstream repo,
and the store runs `asura plugin install <upstream> --yes` (remove runs
`asura plugin remove <id>`), exactly what the Add-ons page uses. Without that
link there is no payload — an empty folder is not a plugin — so the store
refuses, and its error names the repo to run.

`themes` is applied, not downloaded. Installing a theme writes its name into
`~/.config/asura/current/theme.name` (XDG_CONFIG_HOME honoured; the older bare
`~/.config/asura/theme.name` is still read), which the shell's theme engine
picks up live. Only a theme whose files are on disk, or one of the shipped
themes, can be activated — a catalogue-only entry stays listed as available. A
removal refuses the built-in theme and the active theme, then deletes the
theme's directories.

`barstyles` is all shelf: the styles are built into the shell, so install and
remove refuse.

## The commands

```text
asura-store catalog [--refresh] [--category ID]    JSON: categories and items
asura-store check                                  JSON: {"revision","updateAvailable","offline"}
asura-store search QUERY                           JSON: {"items": [...]} from the archive
asura-store install CATEGORY ID                    progress lines on stdout, an error on stderr
asura-store remove CATEGORY ID
asura-store open SECTION                           discover | library | a window section
asura-store settings CATEGORY [ID]                 opens Settings (asura-shell hub open) on its page
```

`catalog` has four categories. `apps` is a shelf of about thirty packages with
their summary, description, version and home page read from apt (`xbps-query` on
a Void host), and what is installed read from dpkg (`xbps-query -l`). `barstyles`
lists the four built-in bar styles and the one `shell.json` selects, which is
what Bar Studio's style shelf reads. `plugins` reads the local catalogue under
`catalog/plugins/`: each entry carries an `upstream` repo, and installed means a
folder in `~/.local/share/asura/plugins`. `themes` merges the built-ins with the
`catalog/themes/` entries and any theme directory on disk, and marks active the
one the current-theme file names. Items follow Ryoku's JSON contract (`id`,
`category`, `name`, `summary`, `installed`, `active`, `updateAvailable`,
`unavailable`, ...), so a page ported from Ryoku reads them unchanged. Any other
category is an error: `rices`, `lockscreens`, `bundles` and `fastfetch` exist
only as window sections, with no catalogue behind them.

## The revision and the refresh dot

Every freshly built catalogue gets a revision: a SHA-256 over the sorted
identity of its items — category, id, version, manifest digest, the declared
window manager, a digest of declared package managers, the paused state and the
declared upstream links — everything an upstream change can alter, and nothing
volatile (local install state, timestamps). Viewing the catalogue acknowledges
the revision by writing it to `~/.cache/asura-store/seen-revision`, and only when
it shows items — an empty first view, which is an offline machine, never becomes
the seen baseline.

`check` builds the catalogue again and answers
`{"revision","updateAvailable","offline"}`. It never writes the seen revision:
`updateAvailable` is true only when a revision has been seen at all, the machine
is not offline, and the revision differs — which is exactly when the refresh dot
lights.

Both of these, and the window-manager gate below, came across with the Ryoku
1.0.7 backport (see NOTICE.md).

## The window-manager gate

A catalogue item may name the window manager it was authored against
(`windowManager`). The store asks the running provider once per launch —
`asura wm status`, whose `Provider:` line is the answer — and compares names; a
third provider needs no change in the store. An item named for another manager
stays listed and keeps any installed copy removable, but install and update are
refused, so a control that cannot work is never offered as if it could. An item
that names nothing runs anywhere, and a machine whose provider cannot be read
gates nothing.

## Search

`search` works on whichever archive exists. On an apt host it reads
`apt-cache search`, keeps the hits where every word is in the name or the
one-line summary, leaves out libraries, headers and debug packages, and ranks an
exact name first. On a Void host it reads `xbps-query -Rs` and applies the same
ranking, also folding in what is installed. The window runs it after a typed
query pauses and shows the hits with the shelf; the query gets 20 seconds.

## Open and settings

`open` starts the store window under its lock (`session/asura-store`) and calls
`nav open` on it through quickshell ipc, retrying for 3 s while the window
settles; a second instance exits at the lock, so the call simply reaches the
first. It accepts `discover`, `library` and the window's sections (`apps`,
`barstyles`, `themes`, `colorschemes`, ... — the names beyond the four
categories are the window's own routes, empty until a catalogue exists).

`settings` maps a category to the Settings page that manages it (`themes` and
`rices` to appearance, `plugins` and `bundles` to add-ons, `barstyles` to
Bar Studio, `fastfetch` to fastfetch) and asks `asura-shell hub open` for that
page; a category without one is an error.

## What is not here, and why

- **No remote registry.** Ryoku's registry, extras actuator and receipt
  transactions stay behind. ASURA's catalogues are the local JSON files in
  `catalog/`, the package manager and the plugin folder, and the local files are
  also the cached content — there is no snapshot of a registry to reconcile.
- **No Flatpak.** Ryoku's provider seam could take it, but a Flatpak provider is its own
  listing, search, remote setup and progress parsing, not a few lines, and nothing calls
  it. Flatpak is not installed on the development machine either.
- **No PackageKit.** It is installed on the development laptop and bus-activatable, but
  `pkexec apt-get` does the job without a second daemon, and the architecture plan does not
  ship PackageKit.
- **No Arch repos, multilib or AUR.** ASURA runs on Void Linux with a custom CachyOS-BORE kernel, not Arch Linux.

## Checks

```sh
cd desktop && gofmt -l cmd/asura-store && go vet ./cmd/asura-store && go test ./cmd/asura-store
sh system/helpers/pkg/pkg_test.sh        # the refusals, the pkexec hand-off and cancel (needs fakeroot)
node --test shell/store/lib/store.test.mjs
```
