# The store

ASURA Store is the package store window and command. It is a port of Ryoku's
RyoStore: the software installs through package helpers, and the privilege comes from polkit.

| Piece | Where | What it does |
|---|---|---|
| `asura-store` | `desktop/cmd/asura-store` | The data plane: catalogue, search, install, remove, open. |
| The window | `shell/store.qml`, `shell/store/` | RyoStore's window, ported as it was. |
| `asura-pkg-add`, `asura-pkg-remove` | `system/helpers/pkg` | Run apt-get as root through `pkexec`. |
| `session/asura-store` | `/usr/share/asura/session/asura-store` | Starts the window under a lock. It is not on `PATH`, because `/usr/bin/asura-store` is the command. |

## How an install happens

1. The window runs `asura-store install apps vlc`.
2. `asura-store` runs `asura-pkg-add --status vlc` as the user. Before any prompt, the
   helper simulates the change with `apt-get -s` and refuses two things: a change that
   would also remove other packages, and a package that only installs a snap (Ubuntu's
   `firefox`, `chromium-browser` and `thunderbird` are snap launchers, and ASURA does not
   use snap). The refusal is a sentence on stderr, which the window shows.
3. The helper re-runs itself through `pkexec`. polkit asks the session's authentication
   agent, which is `asura-shell`, for the password. Nothing in the store runs `sudo`.
4. As root the helper runs `apt-get install` with apt's machine status on stdout, and
   `asura-store` turns it into one `{"stage":"DOWNLOADING","percent":42}` line per step.
   The stages are `AUTHORIZING`, `DOWNLOADING`, then `INSTALLING` (or `REMOVING`).

A declined prompt, or no agent to ask, ends `pkexec` with 126 or 127, and the window says
that authorization was declined. A removal goes the same way through `asura-pkg-remove`,
and refuses to take any package besides the one named.

Cancel is honoured until dpkg starts. The caller cannot signal a root process, so
`asura-store` writes `cancel` to the helper's stdin, which stops apt-get only while it
is downloading and only if no dpkg is running. After that the transaction finishes, because
stopping apt-get under dpkg leaves a half-configured system. `asura-store` also ignores
`SIGPIPE` and `SIGHUP`, so closing the window never kills a transaction partway.

## The commands

```text
asura-store catalog [--refresh] [--category ID]   JSON: categories and items
asura-store search QUERY                          JSON: {"items": [...]} from the whole archive
asura-store install CATEGORY ID                   progress lines on stdout, an error on stderr
asura-store remove CATEGORY ID
asura-store open SECTION                          discover | library | a category id
asura-store settings CATEGORY [ID]                opens Settings (asura-shell hub open) on its page
```

`catalog` has two categories. `apps` is a shelf of about thirty packages with their
summary, description, version and home page read from apt, and what is installed read
from dpkg. `barstyles` lists the three built-in bar styles and the one `shell.json`
selects, which is what Bar Studio's style shelf reads. Items follow Ryoku's JSON
contract (`id`, `category`, `name`, `summary`, `installed`, `active`, `updateAvailable`,
`unavailable`, ...), so a page ported from Ryoku reads them unchanged. Any other category
is an error: `plugins`, `bundles`, `lockscreens`, `rices` and `fastfetch` do not exist.

`search` reads `apt-cache search`, keeps the hits where every word is in the name or the
one-line summary, leaves out libraries, headers and debug packages, and ranks an exact name
first. The window runs it after a typed query pauses and shows the hits with the shelf.

## What is not here, and why

- **No remote catalogue.** Ryoku's plugins, bundles, rices, lockscreens, bar styles and
  fastfetch styles are products of its `ryostore` repository, and none of it is ASURA's.
  The extras actuator and the receipt transactions that installed them were not ported. A
  plugin is a folder in `~/.local/share/asura/plugins`, and `asura plugin` installs and
  removes it, so the Add-ons page calls that instead of the store.
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
