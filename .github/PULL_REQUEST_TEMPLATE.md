# Pull request

Keep this short. A reviewer should be able to tell what changed and why without opening five
other files.

## Type of change

- [ ] Hardware profile (`system/hardware/profiles/`)
- [ ] `zenith` control tool
- [ ] Shell — `zenith-shell`
- [ ] Settings application — `zenith-hub`
- [ ] Packaging or release (`release/`)
- [ ] Installer or ISO build (`installation/`)
- [ ] Kernel fleet or firmware (`system/`)
- [ ] Documentation only
- [ ] Build, CI or tests

## What changed

Facts and files, not a narrative of the session. For a hardware profile, name the machine and
the tier it lands at.

## Why

The problem this solves. For a hardware profile, which support tier it moves the machine from
and to.

## Related issue

Fixes #

## Definition of Done

Copied from [`agents/agents.md`](../agents/agents.md), and binding:

- [ ] Lints pass (if configured)
- [ ] Tests pass (if present)
- [ ] Change maps back to a goal in [`docs/roadmap.md`](../docs/roadmap.md)
- [ ] Any new rule or style pattern discovered this session was recorded in
      [`agents/agents.md`](../agents/agents.md)
- [ ] `AGENTS.md`, `CLAUDE.md` and `GEMINI.md` still exist and still point to
      [`agents/agents.md`](../agents/agents.md)

And, for anything that is not documentation:

- [ ] No shipped file was edited in place — the change is in an override, the store the owning
      tool reads, or the tool itself
- [ ] Zenith's packages still write only to `/usr/share/zenith/` and `/etc/zenith/`
- [ ] Every new setting has exactly one owning tool
- [ ] `gofmt -l desktop/` and `go vet ./...` are clean
- [ ] `glib-compile-schemas --strict --dry-run desktop/**/schemas/` is clean
- [ ] `qmllint desktop/shell/**/*.qml` is clean
- [ ] Any new `LICENSE` matches the root `LICENSE`

For a hardware profile specifically:

- [ ] Submitted from real `zenith hardware report` output on the machine it describes
- [ ] Kernel pinned to a flavour (`generic`, `hwe`, `oem`), never to a patch version
- [ ] No identifying data — no serials, MAC addresses, usernames, hostnames, home paths
- [ ] Firmware referenced as packages, never as files

## Licensing and sign-off

- [ ] Every commit carries a `Signed-off-by:` line (`git commit -s`)
- [ ] The contribution is under GPL-3.0, per [`LICENSING.md`](../LICENSING.md)
- [ ] No third-party code was pasted into the tree — it belongs in a package under `release/`
      with its own licence file
- [ ] No firmware blobs were added

## Notes for the reviewer

What you want looked at, what you are unsure about, and what you deliberately left alone.