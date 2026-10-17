# Contributing to Ken OS

Ken is a distribution, and a distribution is a union of other people's work. Nobody can test
every laptop; everybody can test one. Those two facts decide what a contribution to this
project is worth.

Ken runs pre-alpha. Nothing is bootable yet, so the contributions that matter today are a
hardware profile, a decision, or documentation. See [`docs/roadmap.md`](docs/roadmap.md) for
what lands in what order.

---

## Hardware profiles — the primary path

This is the most valuable contribution in Ken and the cheapest to make. It needs one machine
and no code.

1. Install Ken and boot it.
2. Run `ken hardware report`. It emits a candidate profile from live detection, with
   identifying data stripped.
3. Fill in what only a human on the machine knows: does suspend work, does the GPU switch,
   what fails.
4. Open a pull request against `system/hardware/profiles/`.

Read [`system/hardware/README.md`](system/hardware/README.md) first. It carries the profile
schema, the match rules, the tier definitions, and the list of things a profile must never
contain. Do not write a profile from memory — the report is the input, and
[`profiles/example.yaml`](system/hardware/profiles/example.yaml) is the annotated target.

A profile submitted from real detection output lands at **tier 2, community confirmed**. Tier
1 requires a maintainer's sign-off, because it is a claim about hardware Ken has not been run
on by the people who answer for it. Expect the tier rather than asking for it.

### If your machine is boring

Submit it anyway. A working profile on a laptop nobody has heard of is more useful than
nothing, and the offline-firmware cases are the ones that most need writing down.

### If you do not own the machine

Do not describe someone else's laptop. A hand-written profile for hardware you have not
tested is tier 3, which is behaviourally identical to no profile at all.

---

## Rules that constrain every change

Two rules in [`agents/agents.md`](agents/agents.md) bind every contribution to this
repository, not just some of them.

**One writer per setting.** If a setting has a command or a tool that owns it, that tool is
the only thing allowed to write it. `ken-shell` and `ken-hub` are the only writers of
`org.ken.shell` and `org.ken.desktop`. Hand-editing dconf causes drift, and drift is the most
expensive bug class in a distribution. A new setting needs a schema entry and an owning tool
in the same pull request, or it does not land.

**Never edit a shipped file in place.** Files laid into the system by a package are replaced
on update. A change to one belongs in an override, in the store the owning tool reads, or in
the tool itself. Ken's packages write only to `/usr/share/ken/` and `/etc/ken/` — never to
another package's files, and never to `~`. A patch against `/etc/NetworkManager/` is not a
contribution, it is a fork, and it is what stops the next rebase onto a newer Ubuntu.

Read [`agents/agents.md`](agents/agents.md) in full before your first change. It records
constraints that are expensive to rediscover and it is the contract any agent working in this
repository is held to.

---

## Signing your work

Every contribution is made under the **Developer Certificate of Origin** (DCO 1.1). No CLA,
no copyright assignment, no exception to be approved. Sign-off is one command:

```bash
git commit -s -m "system/hardware: add profile for the ROG Strix G16"
```

`git commit -s` appends a `Signed-off-by:` line naming you. That is the entire requirement.

If you have already committed without it:

```bash
git rebase --signoff HEAD~3
git push --force-with-lease
```

An unsigned commit is not rejected on principle; it is unmergeable, because the licence of
the contribution is undetermined. See [`LICENSING.md`](LICENSING.md) for what Ken's own
GPL-3.0 code, the base layer, and the firmware blobs each oblige you to.

---

## What a pull request should contain

- **One concern.** One machine, one profile, one fix. Unrelated changes split cleanly or not
  at all.
- **Facts, not conclusions.** "External display over the dock drops to 30Hz after resume" is
  usable. "Display is broken" is not.
- **A link to the issue**, and one sentence saying which milestone in
  [`docs/roadmap.md`](docs/roadmap.md) this advances.
- **A dated note** in [`docs/journal/`](docs/journal/) if you investigated a failure. See
  [when something breaks](agents/agents.md#when-something-breaks).
- **Nothing identifying in a profile.** No serial numbers, MAC addresses, usernames, hostnames
  or home directory paths. A profile containing them is rejected rather than sanitised.

Run the verification commands in [`agents/agents.md`](agents/agents.md) before you open the
request. If a check is not configured yet, the checklist item says so — that is acceptable.

---

## Where to ask questions

| Question about | Ask in |
|---|---|
| Hardware, profiles, tiers | [`system/hardware/README.md`](system/hardware/README.md) first, then an issue. The answer is usually already written down there. |
| A bug | The issue forms. If it is about one specific laptop, use the **hardware support** form — it is the intake path that becomes a profile. |
| A design decision | An issue. [`docs/architecture.md`](docs/architecture.md) separates settled decisions from open questions; check which side of that line your question is on before arguing it. |
| Packaging, repository, release | An issue. Say which command failed and paste what it printed. |
| Conduct | `conduct@kenos.dev`. See [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md). |
| A vulnerability | **Not** a public issue. See [`SECURITY.md`](SECURITY.md). |

Before opening an issue, read [`docs/architecture.md`](docs/architecture.md) and
[`docs/roadmap.md`](docs/roadmap.md). Most questions that arrive as an issue were answered in
one of them already, and finding the answer yourself is the fastest route to having it
confirmed.