# ASURA Store

ASURA Store is the package and customization showroom window and CLI data plane for Asura OS. It provides browsing, discovery, and management of native packages, plugins, themes, and UI customizations, with privilege escalation safely handled through polkit.

The canonical standalone repository is hosted at **[github.com/ASURA-OS/asura-store](https://github.com/ASURA-OS/asura-store)**.
The tree in this repository keeps byte-identical mirrors of the standalone store:
- `desktop/cmd/asura-store/` mirrors `cmd/asura-store/`
- `shell/store/` and `shell/store.qml` mirror `ui/`
- `desktop/catalog/` mirrors `catalog/`

CI parity is automatically enforced on every push and PR via `.github/workflows/store-sync.yml`.

| Component | Path | Function |
|---|---|---|
| `asura-store` | `desktop/cmd/asura-store` (binary `/usr/bin/asura-store`) | The Go data plane: catalogue, search, check, install, remove, open, settings. |
| Store Window | `shell/store.qml`, `shell/store/` (`/usr/share/asura/store/ui`) | Quickshell showroom window: discover stage, search shelf, product overlays. |
| Catalog | `desktop/catalog/` (`/usr/share/asura/store/catalog/`) | Local catalogue JSON (`plugins/`, `themes/`, `barstyles/`). Updated without recompilation. |
| Launcher | `session/asura-store` (`/usr/share/asura/session/asura-store`) | Starts the store window under a single-instance lock. |

## How an Install Happens

The store UI executes `asura-store install CATEGORY ID`. The data plane outputs JSON progress events on stdout (`{"stage":"...", "percent":...}`) and status or error messages on stderr.

### 1. Applications (`apps`)
Applications are native packages.
1. The store uses native package management tools (`xbps-query`, `xbps-install`, `xbps-remove`).
2. Elevated transactions run through `pkexec`, querying the active desktop authentication agent (`asura-shell` polkit agent) for authorization.
3. Progress transitions through defined stages: `AUTHORIZING`, `DOWNLOADING`, and `INSTALLING` (or `REMOVING`).
4. If authentication is canceled or denied, `pkexec` returns cleanly and the store presents a clear authorization message.
5. `asura-store` ignores `SIGPIPE` and `SIGHUP` to ensure background package transactions finish cleanly even if the window closes.

### 2. Plugins (`plugins`)
Plugins extend Asura OS capabilities:
1. The catalogue entry references the canonical upstream repository.
2. The store runs `asura plugin install <upstream> --yes` (removal runs `asura plugin remove <id>`), matching the Hub Add-ons manager.
3. Installed plugins reside in `~/.local/share/asura/plugins`.

### 3. Themes (`themes`)
Themes modify desktop styling and colors:
1. Installing an existing theme writes its identifier into `~/.config/asura/current/theme.name`, picked up live by the shell.
2. Removal protects the currently active theme, refusing to delete active configurations.

### 4. Bar Styles (`barstyles`)
Bar styles (`qsbar`, `kairos`, `sumi`) are built directly into Asura OS shell. The store provides style discovery, preview, and selection via Bar Studio.

## CLI Commands

```text
asura-store catalog [--refresh] [--category ID]    JSON: categories and items
asura-store check                                  JSON: {"revision","updateAvailable","offline"}
asura-store search QUERY                           JSON: {"items": [...]} from repository
asura-store install CATEGORY ID                    Streamed progress lines on stdout, status on stderr
asura-store remove CATEGORY ID                     Streamed progress lines on stdout, status on stderr
asura-store open SECTION                           discover | library | window section
asura-store settings CATEGORY [ID]                 Navigates Settings Hub (asura-shell hub open)
```

### Catalog Categories
- `apps`: Curated desktop applications with summaries, descriptions, and install status queryable via `xbps-query`.
- `plugins`: Antigravity and Asura OS plugins from verified upstreams.
- `themes`: Local and curated desktop themes.
- `barstyles`: Built-in bar styles for desktop customization.

## Revisions and Update Checks

Every freshly built catalog computes a deterministic revision hash (SHA-256 over item attributes, versions, manifests, and declared links).
- Viewing the catalog acknowledges the revision at `~/.cache/asura-store/seen-revision`.
- `asura-store check` inspects the active catalog and reports `{"revision","updateAvailable","offline"}`.
- When an unseen catalog update is available, the store UI illuminates the refresh indicator.

## Search

`asura-store search <query>` queries native repository indices using `xbps-query -Rs`, ranking exact matches first, filtering system libraries and debug packages, and combining results with installed state.

## Deep Links & Settings Navigation

- `asura-store open [section]` activates or raises the single-instance store window and navigates to the target section (`discover`, `library`, `apps`, etc.).
- `asura-store settings CATEGORY [ID]` directly opens the relevant section in Asura Settings Hub (`asura-shell hub open`).

## Test Suite

```sh
# Data plane verification
cd desktop && gofmt -l cmd/asura-store && go vet ./cmd/asura-store && go test ./cmd/asura-store

# UI logic verification
node --test shell/store/lib/store.test.mjs
```
