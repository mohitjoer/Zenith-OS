# Hardware support framework

Ken targets every laptop, which is not a promise anyone can keep. What anyone *can* do is
make the set of working machines grow, without needing a hardware lab, and be honest about
which machines are confirmed.

This directory is that mechanism.

## The three tiers

[`tiers.yaml`](tiers.yaml) is the definition of a tier in Ken, and there is exactly one of
them. The installer, the matcher and `ken hardware report` all read that file. Everything in
this section is a rendering of it for a human reader: where the two disagree, `tiers.yaml`
is right and this file is stale.

Every machine is exactly one tier, and the tier is decided by whether a profile exists —
not by marketing.

| Tier | Name | Granted by | Meaning |
|---|---|---|---|
| 1 | Verified | maintainer | Installs, boots, suspend/resume, GPU acceleration, audio and peripherals all confirmed on this machine by a maintainer. |
| 2 | Community confirmed | contributor | Working on a contributor's machine, with a profile submitted from real detection output. Not independently verified. |
| 3 | Best effort | fallback | No matching profile. Installs conservative defaults — the `generic` kernel, full firmware bundle — and may need manual intervention. |

A machine is never silently upgraded between tiers. Tier 1 requires a maintainer's
sign-off, because it is a claim about hardware Ken has not been run on by the people who
answer for it.

Tier 3 is a real tier with real behaviour. It installs the `fallback:` block from
`tiers.yaml`: the `generic` kernel, the full firmware bundle, automatic GPU selection, and
the firmware rescue bundle on the ISO. That kernel is the same answer as
`default_for_unknown` in [`fleet.yaml`](../../system/kernel/fleet.yaml), and it is `generic`
because a fallback has to boot before anything can rescue it.

### The tier 1 checklist

Eleven checks, and each tier's `requires:` block is the checklist for that tier. This is
tier 1's.

| Check | Value | What must have happened on the physical machine |
|---|---|---|
| `boots` | `true` | The machine boots and reaches a login prompt. |
| `login_manager` | `true` | The login manager runs, lists accounts and starts a session. |
| `desktop_session` | `true` | The compositor draws and Ken's shell starts. |
| `network` | `true` | The machine reaches the network on its own, with no manual step. |
| `audio_output` | `true` | Sound plays from the built-in codec. |
| `suspend_resume` | `true` | It sleeps and comes back. The most-reported laptop failure, and the reason this check is on the list. |
| `gpu_acceleration` | `true` | The GPU renders accelerated rather than falling back to software. |
| `display_brightness` | `true` | The panel dims. |
| `external_display` | `any` | Exercised, where the machine can drive an external display at all. |
| `bluetooth` | `any` | Exercised, where the machine has a radio. |
| `fingerprint` | `any` | Exercised, where the machine has a reader. |

`true` means the check must have been exercised on the machine and pass. Where the hardware
a check needs is something a machine may simply not have, the value is one of `any`,
`required` or `not_applicable`:

- **`required`** — the machine has the hardware, and it was exercised.
- **`any`** — exercised where the machine has the hardware; not having it passes.
- **`not_applicable`** — the machine does not have that hardware, and the profile records
  that as a fact about the machine.

`not_applicable` is a claim about hardware, so it is the one value a profile cannot use to
skip a check the machine is capable of doing. Tier 2's checklist is the first five rows;
tier 3's is `boots` alone.

## Profiles

A profile is a verified-working recipe for one machine or machine class: which kernel, which
firmware, which drivers, which configuration. See [`profiles/example.yaml`](profiles/example.yaml)
for the full annotated schema.

```yaml
version: 1
id: asus-rog-strix-g16-2024
license: GPL-3.0
tier: 2
match:
  sys_vendor: ASUSTeK
  product_name: ROG Strix G16
  board_name: G614
kernel: hwe
gpu:
  discrete: nvidia-open
  hybrid_mux:
    mechanism: supergfxctl
network:
  wifi:
    - driver: mt7921e
      firmware: [linux-firmware, linux-firmware-mediatek]
notes: >
  Suspend requires the discrete GPU to be powered down first; the profile sets
  nvidia.PreserveVideoMemoryAllocations=1.
```

Profiles live in `profiles/` and are named `<vendor>-<model>-<variant>[-<kernel>].yaml`. The
name is not the match key — `match:` is. A name is for humans; `match:` is for the installer.

Every profile carries a `license:` field. `GPL-3.0` is the value for a profile that names
base-layer packages, because a package name is not a licence grant and the blob's own
copyright file governs. A profile that carries something a third party wrote — a vendor
patch, a quoted licence, firmware terms — records that work's terms in the same field.
See [`LICENSING.md`](../../LICENSING.md).

`kernel:` is one of `generic`, `hwe` or `oem`, which is exactly the `key:` list in
[`fleet.yaml`](../../system/kernel/fleet.yaml) and is closed. `generic` and `hwe` are fleet
flavours with an upstream package on the base layer, and each installs the fleet's common
packages alongside itself — headers, because DKMS is unavoidable for parts of the laptop
market. `oem` is not a flavour in that sense: it is a per-profile escape hatch for a
machine that boots on nothing else, the fleet carries no upstream for it, and neither
`default` nor the tier 3 fallback ever resolves to it.

Firmware is named as packages, and only with names the base archive carries:

| Package shape | Names that exist |
|---|---|
| Metapackage pulling all of them | `linux-firmware` |
| Per-vendor, pulled in by it | `linux-firmware-mediatek`, `linux-firmware-intel-wireless`, `linux-firmware-broadcom-wireless`, `linux-firmware-realtek`, `linux-firmware-amd-graphics` |
| Signed audio firmware | `firmware-sof-signed` |

There is no `linux-firmware-nonfree` and no `firmware-nonfree`: the base layer has no
`non-free-firmware` component, and every firmware blob it carries is in the packages
above. The split is a property of the archive rather than of this file, so read the
current set from `linux-firmware`'s dependencies rather than from a document, and fail
loudly on a name the archive does not carry — a profile naming a package outside the
reviewed set does not install it, and the machine falls back to tier 3 and says so.

## How a profile gets added

This is the loop that makes the matrix grow without anyone owning hardware:

1. A machine installs Ken and boots.
2. `ken hardware report` emits a candidate profile from live detection, with anything
   identifying stripped.
3. The user reviews it, fills in what only a human knows (does suspend work? does the GPU
   switch?), and opens a pull request.
4. A maintainer verifies what can be verified remotely and assigns a tier.
5. Merged profiles ship in the next ISO and are matched automatically at install time.

**Contributing a profile is the most valuable thing a new contributor can do**, and the
cheapest. It needs one machine and no code.

## Detection order

The probes are not in this directory. They belong to `ken hardware`, and
[`docs/hardware-detection.md`](../../docs/hardware-detection.md) is their implementation
record; what this directory holds is the data they read and the tier they resolve against.
Their order is fixed by `matching.probe_order` in [`tiers.yaml`](tiers.yaml), cheapest and
most decisive first, so the common case never reaches the slow probes:

1. **DMI** — `sys_vendor`, `product_name`, `product_version`, `board_name`. Free, and enough
   for most profiles.
2. **PCI** — GPU class and vendor, network controller, audio codec, storage controller.
3. **USB** — anything that only binds over USB: WWAN, external docks, fingerprint readers.
4. **ACPI** — platform capabilities, and the events that matter for lid, sleep and hotkeys.
5. **EDID** — panel and external display topology, refresh rate, HDR capability.

Match is exact-then-narrowing: try the most specific profile first, then progressively broader
ones, then fall back to tier 3. Ambiguity between two profiles of different tiers resolves to
the **higher tier**, because a confirmed-working recipe beats a more specific guess.

## What a profile must not contain

- **Anything identifying.** No usernames, no hostnames, no serial numbers, no MAC addresses,
  no home directory paths. `ken hardware report` strips these before emitting, and a profile
  containing them will be rejected.
- **A kernel version pinned to a patch number.** Pin the flavour — `generic`, `hwe` or
  `oem`, the closed set in [`fleet.yaml`](../../system/kernel/fleet.yaml) — never
  `6.8.0-31-generic`. Profiles that pin patch versions stop matching on the next security
  update, and a flavour name that is not a fleet key installs no kernel at all.
- **Firmware blobs, or a firmware file path.** A profile names firmware *packages*, by the
  names above. Never a blob, never a `modinfo` filename, and never a package the archive
  does not carry.

## The offline case

Some laptops need a driver to connect to the network, which is the network you would use to
download it. This is the failure mode that makes a Linux distribution unusable on a specific
machine, and it is why:

- The ISO carries a broad firmware rescue bundle rather than only what detection matched.
- `ken-firmware load <source>` accepts a USB stick, a local directory, or another machine on
  the LAN.
- Tier 3 is a real tier with real behaviour, not a shrug.

Any profile that documents a machine needing this should say so in `notes:`.