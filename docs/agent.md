# Zenith Agent

Zenith Agent is the optional resident assistant: a machine-generated knowledge vault
(hardware, packages, the desktop map, what the user changed, habits), a local daemon with
a browser dashboard, the launcher's `\` quick ask, the sidebar chat, and a terminal ask
that proposes commands. It is ported from Ryoku's Rashin (GPL-3.0, see `NOTICE.md`) and
lives in `desktop/cmd/zenith-agent`. It is off until the user switches it on in Settings,
Agent, and while it is off nothing runs, indexes, or reaches a network.

It ships without an agent engine. The vault, the dashboard and the shell's surfaces work
on their own; the part that thinks is yours to plug in (next section).

## Plug in your own engine

The engine is one key in `~/.config/zenith/agent/config.json`:

```json
{ "engine": { "command": ["your-agent", "acp"] } }
```

`engine.command` is an argv for any program that speaks the Agent Client Protocol
(newline-delimited JSON-RPC 2.0) over its stdin and stdout. The daemon starts it in the
vault directory the first time the chat is used, never at boot, starts a fresh one on the
next message if it dies, and replaces it on the next turn when the command changes. It is
not installed, looked up on PATH or guessed for you. The client side is `acp.go`; the key
and the messages around it are `engine.go`. An engine needs `initialize`, `session/new`,
`session/prompt` and `session/update`; `session/request_permission`, `session/load`,
`session/list`, models and commands are used when it offers them. If it dies, the last
line it wrote to stderr is shown. `zenith-agent setup` prints this, and `zenith-agent
status` says whether the command resolves.

**With no engine** the daemon still runs when enabled and keeps the vault current. `status`
reports `ready: false` with `no agent engine configured: set engine.command in
~/.config/zenith/agent/config.json`, and the sidebar chat, the launcher ask and the
terminal ask answer with that same sentence once, then stop. Nothing is started and
nothing retries. A config file that does not parse is reported as such, and the daemon
will not overwrite it.

## Names

The name is spelled once, in `product.go` (`productName`). The binary (`zenith-agent`),
the unit (`zenith-agent.service`), the socket (`$XDG_RUNTIME_DIR/zenith-agent.sock`), the
config, data and state directories, the `ZENITH_AGENT_*` environment and the markers
written into other agents' files all derive from it. `product_test.go` fails until the
names a compiler cannot compute follow a rename: the cmd directory, the unit file, this
doc, `shell/hub/pages/AgentPage.qml`, `shell/agent/skills`, and the shell's call sites.
The embedded dashboard under `web/` is static text and needs a find and replace.

| What | Where |
|---|---|
| config (gate, port, `engine.command`, fast-lane provider) | `~/.config/zenith/agent/config.json` |
| vault (the maps agents read; `memory/`, `journal/` are theirs) | `~/.local/share/zenith/agent/` |
| ask history, recipes, run reports, remembered model | `~/.local/state/zenith/agent/` |
| lock, pid | `$XDG_RUNTIME_DIR/zenith-agent/` |
| skill shipped to agents | `/usr/share/zenith/shell/agent/skills/zenith` |

## Doors

The daemon serves one handler set on two doors. The unix socket (mode 0600 in the
per-user runtime directory) is for the CLI, the shell and the Hub, and carries the verbs
only they call (`/api/ask`, `/api/term*`, `/api/perm`). The loopback port (default 3600)
is for the browser dashboard and never carries those: any web page can aim a request at
127.0.0.1, and `/api/perm` answers an engine's approval request. WebSocket upgrades on
the port also check that the Origin is localhost.

## How it answers, and what it may do

- **Quick ask (launcher) and terminal ask** call an OpenAI-compatible endpoint directly,
  with four read-only tools (system state, read file, list directory, code search), when a
  fast-lane provider is set: `zenith-agent backend <provider>[:model]`. `ollama` and
  `local` need no key, the hosted presets read their key from the daemon's environment
  (`systemctl --user edit zenith-agent.service`). A key is never read from or written to
  a file. Without a provider, or when a question needs more than those tools, the ask goes
  to the engine.
- **Chat** (sidebar and dashboard) is the engine's session. `zenith-agent agent --json`
  lists the engine, or nothing; `agent use` has one engine to name and says how to change it.
- **The daemon never runs a command.** The terminal ask returns a plan with a danger tier
  per command (`danger.go`: read, write, system, danger; unknown binaries are write, never
  read); only `term --run` executes, after `y` or, for danger, typing `yes`. The engine's
  own permission requests are relayed to the sidebar, the dashboard or the terminal, and
  nothing proceeds while one is open.

## Verbs

`serve [--if-enabled]`, `enable` / `disable`, `status [--json]`, `ask <q> | --recent |
--cancel`, `chat [text] | --image P | --new | --cancel | --models | --commands | --skills |
--history | --sessions | --load ID | --set-model ID | --perm ID OPTION`, `term ...`,
`agent [--json | use ID]`, `backend`, `wire` / `unwire [agent]`, `paths [--json |
--snippet]`, `index`, `repo-index`, `setup`. `TestShellCallSitesAreImplemented` reads the
QML and fails if the shell calls a verb or flag that is not here.

## Not ported or changed from Ryoku

- No engine ships. Ryoku's resident agent, its installer, its config wiring and its
  PATH auto-detection of other coding agents are gone; `engine.command` replaces them.
- No Secret Service key storage and no built-in model session: fast-lane keys come from
  the daemon's environment only.
- No daemon pre-warm of the engine at start, no default-on `ensure`, no `rashin` alias.
- `chat --skills` lists nothing: a skill on disk is a document, and an engine advertises
  the commands it can run over ACP.
- The `term` shell adapter (zsh) belongs to the apps lane; it must call `zenith-agent term
  --buffer` and export `ZENITH_AGENT_LAST_CMD` and `ZENITH_AGENT_LAST_STATUS`.
- `prowl` (code index) is not in the Ubuntu archive; the code card and `search_code` say so.
