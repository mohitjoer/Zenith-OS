# Security policy

Zenith ships a signed repository, a signed ISO, an installer that partitions a disk, an updater
that runs as root, and a set of privileged helpers that materialise files into system
directories. A Linux distribution's attack surface is dominated by the code that runs before
anyone gets to log in. Report accordingly.

## Reporting a vulnerability

**Do not open a public issue, and do not open a pull request.** A public report gives an
attacker a development timeline.

Send it to **security@zenithos.dev** — `conduct@zenithos.dev` if the issue is about a person rather
than a machine.

Include:

- The Zenith version or the commit you tested, and the base layer (`26.04`).
- The reproduction steps, ideally minimal.
- What the impact is: local privilege escalation, arbitrary file write as root, signature
  bypass, silent rollback past a security fix, information disclosure.
- Whether it requires physical access, a malicious repository, or a malicious profile.

Expect an acknowledgement. We will confirm when the report is triaged, when a fix exists,
and when a release carries it. Reporters are credited in the fix unless they ask not to be.

Good-faith reports that turn out to be non-issues are not held against the reporter.

## What counts as a security issue in a distribution

Ordinary bugs go in the issue forms. These are the ones that belong here, because they let
something happen that the user's consent does not cover.

### The updater and rollback

`zenith update` fetches debs as root, re-materialises the base, and repaints. `zenith rollback`
restores a previous revision.

- An unprivileged path from a user-writable location into the base or the overlay.
- A signature check that can be skipped, bypassed by a partial failure, or satisfied by a
  repository that is not Zenith's.
- `zenith rollback` restoring a state that silently drops a security fix, or losing an overlay
  file in a way the user is not told about.
- A snapshot of `~/.config` written somewhere an unprivileged local process can read or
  modify it.

### Signature and keyring handling

- The signed APT repository under `release/`: keyring, `signed-by` in `sources.list.d`,
  expiry, rotation.
- ISO and checksum verification, including the failure path when a checksum does not match.
- The Secure Boot chain — shim, MOK, the signing of the boot loader — at any point where a
  step can be skipped or downgraded to an unsigned boot.
- Firmware fetched at install time rather than baked into the ISO: source selection, checksum
  verification, and the path that picks it.

### Privileged helpers

Anything running as root. `zenith doctor`, `zenith-firmware load`, materialisation into
`/usr/share/zenith/` and `/etc/zenith/`, and any future privileged action.

- Command injection from a filename, profile field, device label, or package metadata.
- Path traversal — a relative path or symlink resolving outside the directory the helper is
  supposed to write.
- A symlink or hard link planted by a local user so a privileged write lands somewhere else.
- Anything that reads a root-only file and passes it to a subprocess in a way that leaks it.

### The installer

Disk partitioning, ESP creation and formatting, filesystem creation, Secure Boot enrolment,
and the offline firmware rescue bundle.

- An unescaped or unvalidated value from a device label, a hostname, or a profile reaching a
  shell in the installer.
- Destructive partitioning on a device that was not unambiguously selected.
- Firmware loaded from removable media without verification.

### Packaging

- A Zenith package writing outside `/usr/share/zenith/` and `/etc/zenith/`, into another package's
  files, or into `~`.
- A setuid or file-capability bit on a binary that does not need one.
- A post-install script running with privileges it does not require, or a dependency pulled
  from a non-default source.
- A profile able to introduce a package, a firmware file, or a kernel flavour outside the
  reviewed set.

### The overlay model specifically

The overlay is the design's load-bearing trust boundary: a user file in
`~/.config/zenith/user_edits/` wins over the packaged base at its mirrored path. Reports that an
overlay file can be made to win at a privileged path, or that materialisation copies one
outside its mirrored directory, are in scope and taken seriously.

## What is not a security issue

File these as normal issues, using the hardware support form where it applies:

- A laptop that fails to boot, suspend, or accelerate its GPU without matching a profile.
- A crash, a wrong value, a broken layout, a lost setting.
- A build that is unreproducible or a package that is missing — these matter, but report them
  as bugs and they will be triaged.
- A vulnerability in Ubuntu's own packages. Those are fixed upstream; report them to Ubuntu
  unless the fix is present in Ubuntu and missing from Zenith after Zenith's repository is current.

## Supported versions

| Zenith release | Status | Security fixes |
|---|---|---|
| `zenith-0.1` — pre-alpha development line | Current | Yes |
| Anything older | No releases exist | No |

No release has shipped. There is nothing installable yet, so there is nothing to backport a
fix to. This table becomes the record of what receives fixes once the first release exists,
and the previous line moves to security-fixes-only.

**Base layer: Ubuntu 26.04 LTS.** Security updates for the substrate arrive through Ubuntu's
own archive and reach users through `zenith update`. When a fix is present in Ubuntu's archive
and absent from an up-to-date Zenith installation, that is a Zenith bug — report it here.