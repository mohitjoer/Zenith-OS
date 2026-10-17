# Hardware detection

How Ken identifies a machine, matches it to a profile, and decides what to install. Read this
before touching `system/hardware/` or writing a probe. The tier definitions, the profile
schema and the loop that grows the matrix live in
[`../system/hardware/README.md`](../system/hardware/README.md) and
[`../system/hardware/tiers.yaml`](../system/hardware/tiers.yaml); this document is the
implementation record for the part between the kernel and the profile.

---

## Why detection is load-bearing

Detection is the only subsystem whose failure mode is *no machine at all*. Everything else in
Ken degrades into a lesser desktop. Detection getting it wrong means a machine that will not
boot, or will boot without a network and no way to recover, which is indistinguishable from
Ken not supporting the machine at all.

One detection result decides nearly everything the installer does:

| Decision | Read from | Wrong answer looks like |
|---|---|---|
| Kernel flavour | profile, else tier-3 fallback | No boot, or no GPU, or no WiFi |
| GPU driver stack | PCI probe + profile | Black screen at session start |
| Hybrid mux mode | profile + vendor tool detection | External display dead, or suspend broken |
| Firmware packages | probe + profile | Network device present, never comes up |
| Microcode | DMI CPU family, PCI | Subtle corruption, or an early boot failure |
| Support tier | profile database | A promise Ken cannot keep |
| Offline rescue set | probe | The machine cannot install at all |

Five things therefore have to be true, and they are the acceptance criteria for any change to
this subsystem.

1. **Deterministic.** The same machine and the same image must resolve to the same profile,
   at install time and again at first boot. Detection runs at least twice per machine, in two
   different environments, and a divergence between them looks to the user like a broken
   update.
2. **Degrade, never guess.** A probe that cannot read its input returns `unknown`. It never
   substitutes a plausible default, because a wrong default is a profile-shaped lie: it
   produces a machine that boots into a broken desktop with a tier number attached.
3. **Unprivileged by default.** `ken hardware report` runs as the user. Root is requested only
   where a source genuinely needs it — the `dmidecode` fallback and the ACPI table dump — and
   both are opt-in.
4. **Bounded.** The common path is fast enough that the installer never shows a spinner for it.
   Expensive probes are behind a flag, not in the default path.
5. **Total.** No machine may end with no resolution at all. The worst permitted outcome is tier
   3 with a named reason, and that reason is written down so the user can act on it.

The output side has its own criterion, and it is not a nice-to-have: **the report is published.**
It lands in a pull request, an issue, a forum post and a bug tracker. A detection bug that
leaks a serial is a privacy bug in the distribution's most-used command, which is why
[PII stripping](#pii-stripping-for-ken-hardware-report) is specified as a policy rather than a
convention.

---

## The probe pipeline

Five probes, in the order fixed by `matching.probe_order` in
[`tiers.yaml`](../system/hardware/tiers.yaml): `dmi, pci, usb, acpi, edid`. The order is
cheapest and most decisive first, so the common case — a machine that a profile claims by DMI
string alone — never reaches the slow probes.

| Probe | Yields | Cost on the common path | Decisive for |
|---|---|---|---|
| DMI | Machine identity strings | Under 1 ms, in-process | Which profile |
| PCI | GPU, network, audio, storage, USB host, WWAN | Single-digit ms, in-process | Which drivers |
| USB | Fingerprint, card reader, dock, WWAN modem | Low ms, in-process | Which quirks |
| ACPI | Sleep states, power resources, platform profile | Free from sysfs; seconds if tables are dumped | Suspend behaviour, hotkeys |
| EDID | Panel identity, modes, HDR, backlight presence | Under 1 ms, in-process | Display quirks |

Four rules apply to every probe, and they are what keeps the subsystem maintainable.

- **Read sysfs directly.** Detection walks `/sys` itself rather than shelling out to `lspci`,
  `lsusb`, `acpidump` and `edid-decode`. A shell-out costs a process spawn and a `pci.ids` parse
  to obtain data that is two `read()` calls away — `lspci -Dnn` and `lspci -tv` measured 70–200 ms
  on the reference machine against a sysfs walk that is not measurable beside them — and it
  makes detection depend on four packages being present. Detection must assume none of them is:
  the ISO's package set is a reviewed artefact that can change without warning, and a subprocess
  design has four ways to return nothing that a sysfs walk does not. `lspci` and `edid-decode`
  are used for the *human* output in the report, where their formatting earns the cost, and
  never as the source of truth.
- **One filesystem root per probe.** Every probe takes a root path as its first argument and
  reads nothing outside it. That single property is what makes the whole subsystem testable
  without a hardware lab; see [testing](#testing-without-a-hardware-lab).
- **Absent is a result.** No DMI on an ARM board, no ACPI tables, no EDID on a disconnected
  connector, no bound driver on a device that needs one — these are recorded as
  `present: false` or `unknown`, never as an error and never as an empty string that later
  matches an empty field in a profile.
- **Never trust the source.** DMI strings, EDID blocks and ACPI tables are all writable by
  firmware and, in a virtual machine, trivially attacker-chosen. Anything derived from them
  selects a profile; nothing derived from them may introduce a package outside the reviewed
  set. That is the whole reason the profile loader clamps what it will install.

### DMI

The identity of the machine, and the match key for every profile in the database.

**Reads** `/sys/class/dmi/id/`. The fields worth having are `sys_vendor`, `product_name`,
`product_version`, `product_family`, `product_sku`, `board_vendor`, `board_name`,
`board_version`, `chassis_type`, `bios_vendor`, `bios_version`, `bios_date`. The directory also
contains `product_uuid`, `product_serial`, `board_serial`, `chassis_serial`,
`chassis_asset_tag`, `board_asset_tag` and `modalias`, all of which are handled by
[redaction](#pii-stripping-for-ken-hardware-report) rather than by the reader.

```go
// probe/dmi.go
func DMI(root string) (dmi.Fields, error)
```

The kernel already helps: on the reference machine `sys_vendor` is mode `0444` and
`product_serial` is `0400`, so an unprivileged reader gets identity and gets no serials.
That protection is not sufficient on its own, because `chassis_asset_tag` is `0444` and on
many machines holds a real asset tag, which maps to a purchasing record. The redactor, not the
kernel, is the control that matters here.

`chassis_type` is a raw SMBIOS integer. Map it to the ACPI chassis type strings before it
reaches a report or a profile: `3` desktop, `4` low-profile desktop, `5` pizza box, `6` mini
tower, `7` tower, `8` portable, `9` laptop, `10` notebook, `11` hand held, `12` docking
station, `13` all-in-one, `14` sub-notebook, `30` tablet, `31` convertible, `32` detachable,
`34` embedded, `35` mini PC.

`bios_date` is `mm/dd/yyyy` — the reference machine reports `12/03/2024` — so the
`bios_date: ">=2024-01-01"` comparator in the profile schema needs a real parser. A date that
does not parse is a validation error, never a guess; `01/02/2024` is parsed as 1 February per
the SMBIOS definition, and a value that is not six digits and two separators is rejected.

**Fallbacks.** `dmidecode` reads the SMBIOS table directly when sysfs is unavailable or
unreadable, and `dmi_decode` parses the same table without root. Both need the raw I/O port or
privileged `/dev/mem`, so both are root-only and both are opt-in.

**Cannot determine:**

- **Which sub-variant you have.** This is DMI's central failure. A whole class of machines
  ships `To Be Filled By O.E.M.` as `sys_vendor` and a product string reused across half a
  dozen vendors, and two machines with byte-identical `product_name` can carry different
  wireless chipsets and different panel suppliers. When that happens DMI cannot separate them
  and no profile schema built on DMI can either.
- **Anything about the wiring.** Which GPU drives which panel, whether the mux is set for the
  dGPU, whether the machine is in a mode its firmware did not factory-configure.
- **Firmware sufficiency.** A machine with a valid-looking DMI table can still be running BIOS
  firmware older than its own silicon needs. DMI reports what the firmware says, never whether
  it is adequate.
- **Anything at all on a machine with no DMI.** An ARM board with a device tree and no SMBIOS
  table yields an empty `Fields` and can never match a profile as the schema stands. This is a
  real limitation, recorded in [open questions](#open-questions).

### PCI

Every device the profile needs to know about: GPU, network, audio, storage, USB host, WWAN.

**Reads** `/sys/bus/pci/devices/*/`, specifically `vendor` (4 hex digits), `device`,
`class` (6 hex digits: class, subclass, interface), `subsystem_vendor`, `subsystem_device`,
`driver` (a symlink into `/sys/bus/pci/drivers/`), `boot_vga`, `modalias` and `revision`. The
`modalias` carries the candidate module list, which is how a report can name the modules that
*would* bind even when only one has bound — on the reference machine it reads
`Kernel modules: i915, xe`, which is what lets a profile record `integrated: auto` with a real
basis instead of a hope.

**Names** come from a hardware ID database whose location has moved between releases: probe
`/usr/share/hwdata/pci.ids` first and fall back to `/usr/share/misc/pci.ids`, checking both at
startup rather than assuming one. The file is pure data, parsed lazily, and only for the vendor
and device actually being named. A probe that hard-codes one path loses every device name on
the other layout. `usb.ids` in the same directory serves the USB probe.

The class codes that matter:

| Class | Meaning | Why detection needs it |
|---|---|---|
| `0300` / `0302` | VGA / 3D controller | The GPU, and which driver stack it needs |
| `0200` | Ethernet controller | Wired path during install and rescue |
| `0280` | Other network controller | Wireless, and the offline-risk signal |
| `0403` | Audio device | HDA, and the HDMI audio function on a dGPU |
| `0106` / `0108` | SATA / NVMe | Storage; a VMD-managed disk enumerates as `0108` behind `00:1e.0` |
| `0c03` | USB xHCI host | Port count and USB-C presence |
| `0c04` | USB xHCI with paging | Newer host controllers |
| `0300` + `boot_vga=1` | Boot display | Which GPU the firmware drives the panel from |

`boot_vga` is the single most useful value on a hybrid machine: it is how detection knows that
on the reference machine the panel hangs off the Intel device at `00:02.0` and not the dGPU at
`01:00.0`, which is the fact a profile's mux configuration depends on.

**Cannot determine:**

- **Whether the device works.** A `nvidia` device with no driver bound is a device, not a GPU.
  Its presence does not imply it will accelerate anything.
- **Whether a device is physically there.** A device that firmware disabled simply does not
  enumerate. A passed-through virtual function carries `virtfn` files and must not be counted
  as a second GPU — on a vGPU or a VM, an honest report says so rather than describing a
  machine that does not exist.
- **Topology behind a dock.** A dock's downstream ports are USB, not PCI, and PCI ordering
  carries no information about which socket a device is plugged into.
- **Mux state.** The PCI topology is identical in every mux mode. Only the vendor tool
  (`supergfxctl`, `envy`, `asusctl`) or the profile knows which mode the machine is in.

### USB

Devices that bind over USB rather than PCI, and the class of hardware that has no other way in.

**Reads** `/sys/bus/usb/devices/*/`: `idVendor`, `idProduct`, `bcdDevice`, `manufacturer`,
`product`, `serial`, `busnum`, `devnum`, `speed`, `version`, `driver`, `authorized`. Topology
is reconstructed from the device directory names themselves, which encode it —
`usb1`, `1-0:1.0`, `3-10:1.0` — and `lsusb -t` is the human rendering of the same tree.

The devices a profile actually cares about are fingerprint readers, card readers, docks, WWAN
modems, Bluetooth adapters, headsets and game controllers. Names come from `usb.ids` in the
hardware ID database, resolved by the same two-path probe the PCI reader uses.

**Cannot determine:**

- **Whether the port will do the thing the user wants.** A USB-C port's power delivery and video
  alternate modes belong to the port's PD partner, not to the peripheral. Enumeration tells you
  a device arrived; it does not tell you it arrived at 5 Gbps, at 100 W, or over DP-alt.
- **Whether the device functions without a driver.** A `1234:5678` with no bound driver and no
  interface class is a device detection cannot yet name. This is the same condition as the
  offline problem in the next section, and it is deliberate that it is visible.
- **Identity.** USB serial numbers are frequently per-device vendor garbage, sometimes
  duplicated across a whole production run. They are dropped unconditionally, not hashed.

### ACPI

Platform capability, and the only probe that can explain *why* a lid switch or a sleep state
behaves the way it does.

**Reads** `/sys/firmware/acpi/` for the free facts: `tables/` (uppercase table names, plus
`dynamic/` and `data/` for AML-loaded tables), `platform_profile` and
`platform_profile_choices`, `pm_profile`, `bgrt`, `fpdt`. Sleep capability comes from
`/sys/power/state` (`freeze mem disk`) and, decisively, `/sys/power/mem_sleep`, whose value is
a mask such as `[s2idle] deep` — the brackets mark the default, and that is the mode the
machine will actually use. Wake sources are the `wakeup` attributes under
`/sys/devices/platform/ACPI0003:00/wakeup/` and each device's `power/wakeup`.

**The deep path** is `acpidump` (package `acpica-tools`), which writes every AML table to a
binary file, optionally split per table with `acpixtract` and decompiled to ASL source with
`iasl -d`. From that source, detection can establish which `_CST` sleep states exist and their
types (S0 low-power idle versus S3), whether `_PSR` power resources are available, which `_HID`
devices exist under the lid, battery and hotkey paths, and what `_STA` gating is applied to
any of them.

The deep path is **opt-in and off by default**, for three reasons: it writes a few hundred
kilobytes to disk, `iasl -d` is CPU-bound, and the answer it produces is needed by roughly one
profile in fifty — the machine with a sleep problem. `ken hardware report --deep` runs it and
records the derived facts.

**A trap worth stating plainly: an AML table dump must never appear in a report.** ACPI tables
are machine-generated and routinely embed identifiers as `_DSM` method GUIDs and as strings in
arguments. The report carries derived facts only — which tables exist, which sleep states are
present — and never a table, a hex dump, or ASL source.

**Cannot determine:**

- **Behaviour.** A table's existence is not its effect. `_CST` existing says a sleep state is
  defined; only reading the AML says what it does.
- **Whether suspend will resume.** The most-reported laptop failure is a platform whose S3
  sequence does not return with the discrete GPU still powered. No table read predicts that,
  which is why `suspend_resume` lives in the profile's human-written `verified:` block and not
  in detection.
- **The machine's name.** ACPI carries no vendor identity. `SYSID` does not exist; DSDT content
  is not a product string.

### EDID

The panel, and the last resort for display identity.

**Reads** `/sys/class/drm/card*-*/`, where `card*` is a card and the rest are connectors:
`edid` (the raw 128- or 256-byte block), `status` (`connected` / `disconnected` / `unknown`),
`modes`, `enabled`, `dpms`, `connector_id`. Panel backlight is
`/sys/class/backlight/*/{brightness,max_brightness,bl_device}`, and the bl_device name says
which GPU driver owns it — `intel_backlight`, `amdgpu_bl0`, `nvidia_wmi_ec_backlight` — which is
the difference between "the panel dims" and "the panel does not dim at all".

**Three implementation rules, all of which are traps:**

1. **Do not trust `stat`.** On the reference machine `wc -c` on
   `/sys/class/drm/card2-eDP-2/edid` returns 128 while `stat -c %s` returns 0. The sysfs size
   attribute is not the read length. Read the file and use the byte count returned.
2. **A zero-length read is normal.** A disconnected connector's `edid` is 0 bytes. That is
   `present: false`, not an error. Accept 128 and 256 bytes; reject anything that is not a
   non-zero multiple of 128 as a truncated or faulty read rather than parsing the prefix of it.
3. **Distinguish the internal panel by convention, and say so.** The connector name is the
   signal — `eDP-`, `LVDS-` and `DSI-` are embedded, `DP-`, `HDMI-` and `VGA-` are external —
   corroborated by the fact that exactly one connector has a backlight device. It is a naming
   convention, so it is recorded as a hint with its evidence, never as a bare fact.

`edid-decode` parses the block for the report: the manufacturer PNP ID (three characters, such
as `NCP` for the reference panel), the product code, the monitor name string
(`LM156LF-2F01` on the reference panel), detailed timings and therefore the native mode and
maximum refresh, range limits, colour characteristics, and HDR metadata where present.

**Cannot determine:**

- **That the panel is the internal one.** No EDID field says so. It is inferred from the
  connector name and the backlight device, and it is an inference.
- **Which physical port an external monitor is in**, or whether a dock is passing video or
  charging at the rate the port advertises.
- **That a listed mode works.** EDID describes what the panel was told to support. Runtime
  behaviour is `ken hardware verify`'s job.
- **Everything the firmware omitted.** Many internal panels ship a sparse block with no HDR
  metadata, no display name, and no serial at all. Absence in EDID is absence of data, not
  absence of capability.

---

## Matching

Matching is a pure function of the detection result and the profile database. It reads no
hardware and opens no file outside `system/hardware/profiles/`, which is what makes it the
easiest and most valuable part of the subsystem to test.

### The match key

`match:` is the match key; the profile's `id` and filename are human labels and take no part in
selection. Every field present in a profile's `match:` block must be satisfied by the machine.
Fields absent from the block are unconstrained.

Values are compared after **normalisation**, applied identically to the machine's values and the
profile's:

- Unicode NFC, then ASCII lowercase.
- Runs of internal whitespace collapsed to one space; leading and trailing space trimmed.
- Nothing else. No punctuation folding, no vendor-alias mapping, no fuzzy matching.

Case folding is not cosmetic. Real DMI strings are inconsistently cased across the same vendor's
firmware revisions — `ASUSTeK COMPUTER INC.`, `ASUSTeK Computer Inc.`, `ASUS` — and a
case-sensitive comparison turns that into a class of profiles that silently stop matching after
a BIOS update.

Two schema details that matter when writing a profile:

- A YAML scalar that looks like a number is a number. `product_version: 1.0` parses as a float
  and then compares unequal to the string `"1.0"`; quote every DMI value that could parse as a
  number, boolean or null. The report emits them quoted for exactly this reason.
- `"*"` is the explicit any-match, for a narrowing field a profile wants to name for the reader
  without constraining. A profile that names `board_name` and the machine reports an empty
  `board_name` does **not** match. An absent value satisfies only an absent or `"*"` field.

### Exact-then-narrowing

`tiers.yaml` fixes `strategy: exact_then_narrowing`, which resolves as three passes:

1. **Exact.** Every profile whose `match:` block is fully satisfied by the machine's detection
   result, after normalisation. This is the candidate set.
2. **Rank.** Order the candidates by the tie-break rules below. The winner is the resolution.
3. **Fallback.** If the candidate set is empty, the machine is tier 3 and takes the `fallback:`
   block from `tiers.yaml` — `generic` kernel, full firmware bundle, `auto` GPU driver,
   `install_firmware_rescue: true`, `warn_tier: true`.

"Narrowing" is the tie-break doing the work, not a separate pass: a broader profile that
survives ranking is selected precisely because no narrower one claimed the machine.

### Tie-breaks

Applied in this order. Each step is total, and resolution never depends on file ordering on
disk or on map iteration order in the implementation.

| Order | Rule | Reason |
|---|---|---|
| 1 | **Higher tier wins** | Fixed by `matching.ambiguous_resolution: highest_tier`. A confirmed-working recipe beats a more specific guess. |
| 2 | **More identity fields** | `sys_vendor`, `product_name`, `product_version`, `product_family`, `board_name` count 1 each. |
| 3 | **More narrowing fields** | `chassis_type`, `board_version`, `product_sku` count 1 each. |
| 4 | **A comparator is present** | A profile asserting `bios_date: ">=2024-01-01"` is more specific than one without it. |
| 5 | **Lexicographically smallest `id`** | Exists only to make resolution total and reproducible. It is never reached with different behaviour. |

Rules 2 to 4 form a tuple compared lexicographically, not a weighted score: there are no magic
constants to tune, and the comparison is easy to reimplement in any language.

**Why the higher tier wins, given that it is the counterintuitive rule.** Two profiles can
match the same machine because the machine's DMI string is shared by more than one
configuration. A narrow tier-2 profile is a recipe someone ran on a *different* machine that
happens to share the string. A broad tier-1 profile is one a maintainer ran on a machine of
*this* model and tier-1 checklist. The tier-1 recipe is the one with evidence behind it, so it
is the one that wins. Specificity is a proxy for evidence and loses to evidence.

**Conflict.** Candidates that tie on tier *and* on the whole specificity tuple, and that
disagree on any actionable key — `kernel`, `gpu.*`, `hybrid_mux.*`, `network.wifi[*].driver`,
`network.wifi[*].firmware`, `firmware.*` — are not a tie-break case. Detection does not choose:
it reports a **conflict**, names both profile IDs and the keys they disagree on, and falls back
to tier 3 with the conservative defaults. Silently picking one of two verified recipes is worse
than picking neither, because the user cannot tell which one they got.

The same disagreement, when the two profiles differ in tier or specificity, is resolved by the
table above — but `ken hardware lint-profiles` warns about it at review time, because a narrow
tier-2 profile and a broad tier-1 profile covering the same machines is the shape that most
often turns into a conflict once someone adds a third.

### Resolution output

The resolution is written to `/etc/ken/hardware/resolution.yaml` at install time, which is
inside the only directory structure Ken owns outright, so nothing is written outside
`/usr/share/ken/` and `/etc/ken/`. `ken hardware match` reads it and answers the two questions a
user actually has: which tier am I, and what would change it.

```yaml
version: 1
detected_at: "2026-10-17T21:58:04Z"
detection_digest: sha256:7b1c4e8a90d35f6218a4bd77c0e93f14
profile: asus-tuf-gaming-f15
tier: 1
reason: >-
  Exact match on sys_vendor and product_family. Tier 1 selected over two
  narrower tier 2 candidates because a verified recipe outranks a more
  specific guess from an unverified machine.
kernel: hwe
matched_fields: [sys_vendor, product_family]
unconstrained_fields: [board_version, product_sku, bios_date]
also_matched:
  - id: asus-tuf-gaming-f15-fx507zc
    tier: 2
    specificity: [3, 0, 0]
    rejected_by: tier
  - id: asus-tuf-gaming-f15-fx507zc4
    tier: 2
    specificity: [4, 0, 0]
    rejected_by: tier
offline_risk: false
```

The winner is the **least specific** of the three and that is correct, which is what rule 1
means in practice. `specificity` is the tuple from the table above — identity fields, narrowing
fields, comparator — and `rejected_by` records which rule discarded each candidate, so a user
whose machine is on the wrong profile can see that a narrower one existed and why it lost.

`also_matched` is not decoration. It is what turns "why is my machine tier 3" into a one-line
answer, and it is what the lint uses to spot a database about to develop a conflict.

---

## PII stripping for `ken hardware report`

Reports are attached to public pull requests. The redaction policy is therefore part of the
detection contract, not a formatting pass, and it is specified so that it can be implemented
once rather than reviewed per report.

### Threat model

The report has to contain enough to write a profile and to diagnose a boot failure, while
carrying nothing that identifies the person or the asset. The residual risks it accepts:

- A machine model plus a firmware version identifies a fleet of identical laptops, not a
  person. That is acceptable and unavoidable — the profile needs it.
- A serial identifies a specific asset and, through a vendor's support portal, a specific
  customer. That is never acceptable.
- An asset tag identifies a purchasing record. Never acceptable.
- A MAC address is a stable cross-network and cross-machine identifier. Never acceptable.

### Field policy

Every field falls into exactly one of three classes. There is no default class; an
unclassified field is a validation error in the reader.

| Field | Class | Rule |
|---|---|---|
| `sys_vendor`, `product_name`, `product_version`, `product_family` | **Kept** | The match key. Required. |
| `board_vendor`, `board_name`, `board_version` | **Kept** | The match key, and how two configurations of one model are told apart. |
| `product_sku` | **Kept** | A model identifier, not a person. Frequently the field that distinguishes configurations `product_name` merges. |
| `chassis_type` | **Kept, mapped** | Integer to ACPI chassis type string. |
| `bios_vendor`, `bios_version`, `bios_date`, `bios_release`, `ec_firmware_release` | **Kept** | Firmware identity. Never sufficient alone to identify an asset. |
| PCI vendor, device, class, subsystem vendor, subsystem device, revision, `boot_vga`, candidate module list | **Kept** | Published hardware IDs. The most useful narrowing evidence available. |
| USB `idVendor`, `idProduct`, `bcdDevice`, `manufacturer`, `product`, topology | **Kept** | Device class, not asset. |
| ACPI table names, sleep masks, platform profile and choices, wake sources | **Kept, derived only** | Facts. Never a table dump, never AML source, never a GUID. |
| EDID manufacturer, product code, mode list, name string, backlight device name | **Kept** | Panel identity. |
| `product_uuid` | **Hashed** | Token. |
| `product_serial`, `board_serial`, `chassis_serial` | **Hashed** | Token. |
| EDID serial descriptor (offset 54, tag `0xff`, 4-byte little-endian, and the ASCII variant) | **Hashed** | Token. Internal panels do carry one. |
| `chassis_asset_tag`, `board_asset_tag` | **Dropped** | World-readable, stable, low-entropy, and maps to a purchasing record. No diagnostic value worth that risk. |
| `product_serial` on the GPU, GPU UUID, board ID | **Dropped** | From `/proc/driver/nvidia/gpus/<pci>/information`. |
| `/sys/class/net/*/address`, `perm_address`, `ifalias` | **Dropped** | MACs and user-set interface aliases. |
| Storage `serial`, `wwn`, `wwid`, `vpd_page_80`, `/dev/disk/by-id/*`, `/sys/class/scsi_host/*/proc_name` | **Dropped** | Every one is an asset identifier. The drive model is kept; the drive's identity is not. |
| WWAN IMEI | **Dropped** | Identifies the SIM and its subscriber, which is a third party who did not consent. Never hashed. |
| `/etc/machine-id`, `/var/lib/dbus/machine-id`, `product_uuid` in unredacted form | **Dropped** | A machine identifier is exactly that. |
| `$USER`, `$LOGNAME`, `$HOME`, `/etc/hostname`, `uname -n`, any path under a home directory | **Dropped** | |
| Journal excerpts, `dmesg`, service logs | **Not collected** | They are full of usernames, hostnames and paths. The hardware support form asks for journal text in its own field, beside the report, and the report itself carries none. |

Asset tags are dropped rather than hashed while serials are hashed, and the difference is
deliberate. A hashed serial exists so that support can confirm "this is the serial you sent me"
without the report ever publishing it. An asset tag has no equivalent support use — no one
debugs a driver with it — and it is the field most likely to be low-entropy enough to
enumerate, so hashing it would publish a correlatable token for nothing.

**Hashing never reaches a profile.** A profile's `match:` needs none of the hashed fields, so
the committed artefact contains no tokens at all. Hashing exists in the diagnostic report and
nowhere else.

### How a token is produced

A keyed MAC, so that tokens are stable per machine but unresolvable by anyone who does not hold
the key — including a third party reading a public pull request, and including a brute-force
attempt over a plausible serial list.

```text
key  = 32 bytes from getrandom(2), written 0600 to
       $XDG_STATE_HOME/ken/hardware/report-salt   (default ~/.local/state/ken/hardware/)
       or /etc/ken/hardware/report-salt when running as root from the installer
input = "ken/hardware-report/v1\0" || field_name || "\0" || raw_value
token = "sha256:" || lowercase hex of the first 8 bytes of HMAC-SHA256(key, input)
```

Including `field_name` in the MAC input means a token cannot be lifted from one field into
another — the DMI UUID token and the EDID serial token for the same machine differ, which is
correct, because they are different fields that happen to be equal.

Both salt locations are ones Ken already owns: a user-state path from the ownership model, and
`/etc/ken/`. Nothing is written to `~` by a package and nothing is written outside
`/usr/share/ken/` and `/etc/ken/` by anything shipped.

Two properties follow, and both belong in the report's own header so a reader knows them:

- Two reports from the same machine produce the same tokens, so a maintainer can tell that two
  pull requests came from the same laptop.
- Rotating the salt changes every token. The tokens are therefore **not** a durable machine
  fingerprint, and no external index of them is meaningful.

### Two layers, and the second one is what makes it safe

**Layer one — the reader allowlists.** The DMI reader copies named fields into a struct. It
never copies a directory. Detection has no code path that can emit a field it did not classify.

**Layer two — the emitter sweeps.** After rendering, the report is scanned and **refuses to
emit** if any of these is present, naming the offending key and exiting non-zero:

```text
1. Any value present in the raw /sys/class/dmi/id dump that is not on the keep-list.
2. A MAC address:            (?i)\b([0-9a-f]{2}:){5}[0-9a-f]{2}\b
3. An IPv6 link-local:       (?i)\bfe80(:[0-9a-f]{0,4}){1,7}\b
4. A home directory:         /home/[A-Za-z0-9._-]+|/root\b|/Users/[A-Za-z0-9._-]+
5. A username:               the literal $USER and $LOGNAME values
6. The contents of /etc/machine-id and /var/lib/dbus/machine-id
7. A hostname:               the literal `uname -n` and /etc/hostname values
```

Rule 1 is what makes this a policy rather than a regex list. It is the reason the reader is an
allowlist anyway — the sweep needs the raw values to compare against, and only the raw dump
has them.

**One false positive to design around, not to fix by weakening the check.** PCI and USB
identifiers are hex, and subsystem identifiers are written `vendor:device` in exactly the shape
of the MAC pattern's first half. `8086:46a6` is four digits a side and does not match a six-pair
MAC, so a real profile's evidence survives rule 2 — but a USB `idVendor` of `0x0011` and a
device of `0x2233` reported as `0011:2233` sits uncomfortably close to it. The sweep is
therefore **allowlist-aware**: it skips any substring that appears verbatim as an allowlisted
PCI or USB identifier in the same report. A redaction check that is disabled by hex-looking
data is not a check.

### What the report is not allowed to say

Two rules from the profile rules apply to the report's own output, because the report is what a
profile is written from:

- **No kernel patch version in anything that becomes a profile.** The report prints the running
  kernel for a bug report, because it is needed to diagnose one. The profile section carries
  the flavour only — `generic`, `hwe`, `oem` — resolved from `/etc/ken/kernel` or from the
  flavour component of `uname -r`. A candidate profile containing `6.8.0-31-generic` is a
  validation failure, not a warning.
- **No firmware file, ever.** A profile names firmware *packages*: `linux-firmware`,
  `linux-firmware-mediatek`, `linux-firmware-intel-wireless`. The report lists a device's
  required firmware as the package that ships it, resolved through the reviewed set below,
  and never as a blob, a path, or a `modinfo` filename.

The set of packages a profile is allowed to name is data, at
`/usr/share/ken/hardware/firmware-set.yaml`, mapping every firmware blob the resolver knows
about to the package that contains it. A profile naming a package outside this set does not
install it; it falls back to tier 3 and says so. This is the same boundary
[`SECURITY.md`](../SECURITY.md) draws for packaging: a profile must not be able to introduce
a package, a firmware file or a kernel flavour outside the reviewed set.

### Output shapes

| Invocation | Emits | Contains serials |
|---|---|---|
| `ken hardware report` | Diagnostic text, for pasting into the hardware support form | Tokens only |
| `ken hardware report --format json` | The same content, machine-readable | Tokens only |
| `ken hardware report --profile` | Candidate profile YAML, schema-validated | No tokens at all |
| `ken hardware report --deep` | Diagnostic text plus ACPI-derived facts | Tokens only |
| `ken hardware report --raw` | Unredacted local dump for debugging | **Yes** |

`--raw` writes to a path matching the patterns already excluded in
[`.gitignore`](../.gitignore) — `*.hardware.raw.yaml`, `*-report-raw.yaml` — prints the
absolute path and a warning that the file must not be attached to anything, and **refuses
outright when `CI` is set**, so a pipeline cannot produce a file intended for humans to keep
local.

```console
$ ken hardware report --profile
# Candidate hardware profile — generated by `ken hardware report`.
# Review before opening a pull request. `verified:` is for a human on this
# machine; detection cannot fill it in. Fields that would identify this
# machine have been removed, not redacted.
version: 1
id: vendor-model-variant
tier: 2
match:
  sys_vendor: "ASUSTeK COMPUTER INC."
  product_name: "ASUS TUF Gaming F15 FX507ZC4_FX507ZC4"
  product_version: "1.0"
  product_family: "ASUS TUF Gaming F15"
  board_vendor: "ASUSTeK COMPUTER INC."
  board_name: "FX507ZC4"
  board_version: "1.0"
  chassis_type: notebook
  bios_date: ">=2024-01-01"
kernel: hwe
gpu:
  discrete: nvidia-open
  integrated: auto
  hybrid_mux:
    enabled: true
    mechanism: supergfxctl
    default: hybrid
network:
  wifi:
    - driver: iwlwifi
      firmware: [linux-firmware]
audio:
  codec: sof
  quirks: []
storage:
  quirks: []
firmware:
  packages: []
  microcode: [intel-microcode]
verified:
  suspend_resume: untested
  external_display: untested
  bluetooth: untested
  fingerprint: untested
  known_issues: []
notes: >
  Replace with anything a future maintainer needs and cannot derive from the
  machine. Write what you tried, not just what you concluded.
```

The two hyphenated `board_version` and `product_version` values are quoted because
`1.0` would otherwise parse as a float and stop matching the string the machine reports. That
is the single most common way a correct profile fails to select its own machine.

---

## Offline operation

The failure this section exists for: **a laptop whose wireless adapter needs a driver cannot
fetch that driver over the network it does not have.** This is the wall every distribution hits,
and it is the single largest source of "Ken does not support my machine" reports. It is also
the hardest failure to detect, because the machine that cannot report its problem is the
machine with the problem.

### Two shapes of the same wall

**The driver is unloaded.** The adapter is present on PCI but no module is bound. On the
reference machine, `lspci -Dnnk` prints `Kernel modules: i915, xe` for an unbound-capable
device; for an adapter with no in-tree driver it prints `[8086:2723]` with nothing at all. The
adapter cannot be identified, so the firmware to fetch cannot be named. Worse, a chip recent
enough to need a newer kernel has a PCI ID no local database has, so even a human cannot name
it from `lspci` output alone.

**The driver is present and the firmware is not.** The module binds, the device appears, and
it never comes up. This is the more common case and the easier one, because the device is
identifiable and the fix is a file.

The asymmetry decides the design: **the rescue bundle must be indexed by hardware ID, not by
driver name.** Indexing by module name fails at exactly the moment it is needed, because the
name is the thing that is missing. `0000:00:14.3` as a vendor/device pair, `0c42:271c` as a USB
pair, and a DMI board name all have to be resolvable to a bundle item without the module
existing anywhere on the system.

### The detection signal

Offline risk is computable, and it is computed before anything is installed:

> **A device in PCI class `0280` (other network controller) or `0200` (ethernet) with no
> `driver` symlink is an offline-risk machine.**

The same rule covers a wireless device that enumerated under USB, and a machine whose only
network path is WWAN. `ken hardware match` prints the finding at the top of its output, and
`ken hardware report` records it as `offline_risk: true` with the specific devices:

```yaml
offline_risk: true
missing:
  - id: "8086:2723"
    class: "0280"
    kind: network
    reason: no driver bound; PCI ID absent from /usr/share/hwdata/pci.ids
    rescue_item: net-intel-cnvi-2723
```

That block is the input to `notes:` in a profile, which
[`system/hardware/README.md`](../system/hardware/README.md) already asks for on exactly these
machines.

### The ladder

Each rung is tried before the next. A machine needs the lowest rung that works, and the report
records which one it needed, because the rung a machine needs is the most useful single fact
in its profile.

| Rung | Situation | What it takes |
|---|---|---|
| 1 | Driver is in the kernel and the module is in the ISO initramfs | Nothing. `modinfo <module>` against the running kernel is the check. |
| 2 | Driver is in the ISO's rescue module pool | `modprobe` from `/usr/lib/modules/<flavour>/extra/ken-rescue/` |
| 3 | Driver binds, firmware blob missing | A package from the rescue pool, installed offline |
| 4 | Out-of-tree driver, buildable on the target | `linux-headers` for the running kernel plus a toolchain, both on the ISO, then a DKMS build from a pool `.deb` |
| 5 | Secure Boot refuses the out-of-tree module | MOK enrolment at reboot, with `mokutil` on the ISO |
| 6 | No route at all | `ken-firmware load` from USB, a local directory, or a LAN host |

Rung 4 is where Secure Boot bites, and it is worth being blunt about the outcome: if the user
will not enrol a MOK, the machine runs without acceleration for the discrete GPU, and the
profile records that as a known issue rather than pretending otherwise. That is the correct
answer, not a workaround.

### The rescue bundle

Broad rather than matched. The ISO carries enough that detection has not yet run to have
anything useful to say, which is the only condition under which the bundle matters.

```
/ken-firmware/
  BUNDLE.IDX            # signed index: hardware ID -> pool items -> sha256
  BUNDLE.sig            # detached signature over BUNDLE.IDX
  pool/*.deb            # firmware packages, DKMS sources, microcode
  modules/<flavour>/*.ko.zst
  README.txt
```

Verification is not optional, because firmware loaded from removable media is an untrusted
input path and [`SECURITY.md`](../SECURITY.md) scopes it accordingly. `BUNDLE.IDX` is verified
against the public key at `/usr/share/ken/hardware/keys/firmware-bundle.pub` — inside the
directory Ken owns outright, so the key ships with the image and cannot be replaced by a
mounted stick. Every `.deb` is checked against the digest in the verified index before it is
installed. A failed signature aborts the whole load; there is no partial-apply path.

`BUNDLE.IDX` is written for the machine being helped, not for the whole fleet. Its keys are
hardware IDs in the forms detection actually produces:

```yaml
version: 1
items:
  - id: net-intel-cnvi-2723
    match:
      pci: ["8086:2723"]
    provides: [linux-firmware]
    packages: [linux-firmware]
  - id: net-broadcom-bcm4360
    match:
      pci: ["14e4:4360", "14e4:4365"]
    provides: [linux-firmware-broadcom-wireless]
    packages: [linux-firmware-broadcom-wireless, linux-firmware]
  - id: net-realtek-8821ce
    match:
      pci: ["10ec:c832"]
    provides: [rtl8821ce-dkms]
    packages: [rtl8821ce-dkms, linux-firmware]
    requires_headers: true
    secure_boot: mok_required
```

Each `provides` names a package from the reviewed set, so a rescue item can never widen what
the installer is allowed to install. An ID that appears in no item is reported as an unknown
machine rather than guessed at.

### `ken-firmware load`

The two halves are a pair, and both are needed: the machine that has a working driver has the
firmware, the machine that needs the firmware cannot reach anything.

The three sources cover two different situations, and the difference matters. A machine with a
working wired port can usually reach a host on the LAN even when it cannot reach the internet —
a lab with a local mirror, a site-internal network, or simply a WAN path that depends on the
radio that will not come up. That is the LAN host row. A machine with no usable network at all
is the USB stick and local directory rows, and that is the case the wall in the issue template
was filed about.

| Source form | Example | How it is read | Behaviour on failure |
|---|---|---|---|
| A block device or partition | `ken-firmware load /dev/sdb1` | Opened read-only, mounted `ro,noexec,nosuid,nodev` in a private mount namespace, unmounted before the call returns | Partition with no `/ken-firmware/BUNDLE.IDX` is skipped, not an error |
| Any local directory | `ken-firmware load /media/ken-stick` | Same mount rules for removable media; plain read for an ordinary directory | Refuses if the index does not verify |
| A LAN host | `ken-firmware load ken-desktop:/srv/ken-firmware` | `ssh`, host key pinned in `/etc/ken/hardware/known-hosts` | Refuses on an unknown or changed host key, without prompting |

`ken-firmware push` is the other half and is what makes the LAN row worth having. Run on a
machine that already works, it indexes **its own installed packages** against the hardware IDs
it can see, so the transfer carries what that particular machine needed and nothing else. It is
a machine's working state moved as data, not a downloaded bundle.

```console
# On a machine that already has the driver, for the machine that does not
$ ken-firmware push ken-installer:/srv/ken-firmware
indexed 3 hardware ids from 2 installed packages, wrote /srv/ken-firmware (412 KiB)

# On the machine that needs it
$ ken-firmware load /dev/sdb1
scanned /dev/sdb1: found /ken-firmware/BUNDLE.IDX, signature ok
matching 1 hardware id: net-realtek-8821ce
  rtl8821ce-dkms 1.912.0-1  amd64  sha256 ok
installed 1 package; module built for 6.17.0-31-generic
Secure Boot is active: a one-time enrolment is required for the out-of-tree module.
Reboot and choose "Enroll MOK" at the prompt, or run:
  mokutil --import /var/lib/ken/firmware/MOK.der
re-running detection
  network: 10ec:c832 now has driver rtl8821ce bound
  offline_risk: false
  tier: 2 (unchanged — firmware rescue does not promote a tier)
```

Four things in that output are deliberate and are each load-bearing:

- **`signature ok` before anything is installed.** Verification precedes use, always.
- **The hardware ID is matched, not the driver.** The index is consulted with the ID detection
  produced *before* a driver existed, which is the entire reason the index is keyed by ID.
- **The tier does not change.** Firmware rescue makes tier 3's defaults work; it is not
  evidence about the machine. A tier-3 machine that installs cleanly with rescue firmware is
  still tier 3, and saying otherwise is exactly the dishonesty the tier system exists to
  prevent.
- **Re-detection runs afterwards.** A machine whose offline risk has cleared may now match a
  profile it could not match before, because a device that never enumerated is a device no
  profile could have described.

### Installing base-layer firmware does not violate the boundary

Worth stating because it looks like a violation and is not. `ken-firmware load` runs `dpkg -i`
on packages from the base layer and its firmware archive. Those packages write to `/lib/firmware`
and `/usr/lib/modules`, which Ken does not own. The boundary rule constrains *Ken's* packages
to `/usr/share/ken/` and `/etc/ken/`; installing a base-layer package is the base layer
writing its own files. Ken installs the package and never touches its contents.

---

## Tier assignment

A machine's tier is decided by whether a profile exists and how much evidence stands behind
it, never by what a machine asks for. The tiers themselves, and what each one promises, are
[`tiers.yaml`](../system/hardware/tiers.yaml); this is who is allowed to move a machine
between them.

| Transition | Automatic | Performed by | Gate |
|---|---|---|---|
| No profile → 3 | Yes | The matcher | The `fallback:` block, plus `warn_tier: true` |
| 3 → 2 | Yes, at submission | The contributor, and CI enforces the floor | The profile passes `ken hardware lint-profiles` and was generated by `ken hardware report` on the machine it describes |
| 2 → 2, editing a profile | Yes | The contributor | Re-validation. A narrower `match:` cannot lower a tier, because the tier is a claim about the machine, not about how precisely the profile found it. |
| 2 → 1 | **No** | A maintainer | The `tiers.yaml` `requires:` checklist satisfied, evidence attached, machine physically run |
| 1 → 2, or any → 3 | Yes, downward only | The loader | The profile names a kernel flavour, firmware package or DKMS module outside the reviewed set, or matches nothing in the database |

The rule behind the table: **promotion upward from evidence already on the machine is
automatic; promotion upward from an assertion is not, and demotion is always automatic.**

### Why tier 1 requires a human

Three reasons, and the first is the practical one.

**The checklist contains things no machine can check.** `tiers.yaml` requires `suspend_resume`,
`external_display` and — for some machines — `bluetooth` and `fingerprint`. Whether suspend
and resume are *clean* on a given laptop, whether an external display works through that
machine's USB-C dock, is a judgement made by a person looking at the machine. No probe reads
it and no script infers it. `ken hardware verify` collects what *can* be collected — boots,
login manager, session, network, audio output, GPU acceleration, panel backlight writable,
bluetooth enumerated — and leaves exactly the judgement calls to the human.

**Tier 1 is a claim somebody answers for.** The note in `tiers.yaml` says it plainly: tier 1
requires a maintainer's sign-off precisely because someone has to answer for it when it breaks.
That is a support obligation being accepted, and an obligation is not something software
accepts on a machine's behalf.

**Automation would make the tier meaningless.** A script that promoted every working machine to
tier 1 would produce a support matrix that reads "verified" because something verified it. The
entire value of tier 1 is that it means a person looked. Automating it destroys the signal that
distinguishes tier 1 from tier 2 and leaves the tier system as decoration.

### Enforcement, not policy

The rule is enforced in three places, because a rule only one reviewer remembers is not a rule.

- **The profile loader clamps.** A profile that claims `tier: 1` without a maintainer's
  sign-off is loaded at tier 2. The loader will not let a contributor raise a tier, which is
  what the profile schema already says it does not.
- **CI rejects it.** `ken hardware lint-profiles` fails a pull request whose profile claims
  tier 1 without the sign-off, so the question is never asked in review.
- **The installer states it.** The installer displays the resolved tier and the profile ID
  before it writes anything, and `ken hardware match` prints both afterwards. A machine that
  silently drops from tier 2 to tier 3 — a BIOS update rewriting a DMI string is the usual
  cause — is the failure mode here, and it is only visible if something says so out loud.

A tier-3 machine with a working community profile *should* be raised to tier 2 by submitting
one. That is the loop, and it costs one machine and no code.

---

## Testing without a hardware lab

Nobody in this project owns a hardware lab, and the design has to assume that permanently. The
way to make that true rather than aspirational is to split the subsystem along a line that
puts every hardware-dependent byte behind a filesystem root.

### The split

```
system/hardware/
  probe/dmi.go          func DMI(root string) (dmi.Fields, error)
  probe/pci.go          func PCI(root string) ([]pci.Device, error)
  probe/usb.go          func USB(root string) ([]usb.Device, error)
  probe/acpi.go         func ACPI(root string) (acpi.State, error)
  probe/acpi_aml.go     // opt-in: writes to a caller-supplied directory
  probe/edid.go         func EDID(root string) ([]edid.Panel, error)
  redact/redact.go      func Verify(fields Fields, rendered []byte) error
  match/match.go        func Resolve(r Result, db []Profile) Resolution
  profiles/*.yaml
  lint/lint.go          // every rule in "what a profile must not contain"
```

No function in `probe/` or `match/` reads an absolute path. Every one takes a root, and the
only absolute paths in the package are the defaults the binary passes in. That is a
mechanically checkable property, and it is checked:

```go
// probe/root_test.go — the property that makes the rest of this section possible
func TestNoAbsoluteSysfsPath(t *testing.T) {
	// Walks the AST of every file in package probe and fails if any string
	// literal beginning "/sys" or "/proc" appears outside RootDefault.
	// A new hard-coded path is a test failure, not a review comment.
}
```

Without that test the split decays on the first convenience, and with it the whole subsystem
is testable against a directory tree on disk.

### Fixtures are machines

`ken hardware capture --out tests/fixtures/asus-tuf-gaming-f15-fx507zc4/` runs the same readers
with `root` set to that directory and writes exactly the files those readers consume: the DMI
fields, one directory per PCI function, the USB device tree, the ACPI sysfs attributes, and the
raw EDID blobs. Captures are passed through the same redactor before being committed, so a
fixture physically cannot contain a serial, and the redactor's own canary test is what
guarantees that rather than reviewer diligence.

A fixture is a machine. One contributor's laptop becomes an end-to-end test for detection,
matching, report generation and profile generation at once, forever, on any machine, forever
cheaply. This is the same leverage the profile database gets from hardware contributions,
applied to the code.

### The test matrix

| Test | Input | Asserts |
|---|---|---|
| `probe/dmi` | Captured DMI directory | Exact field struct, including the empty `product_sku` and the `mm/dd/yyyy` `bios_date` parse |
| `probe/pci` | Captured PCI tree, hybrid machine | GPU, network, audio, storage classification; `boot_vga`; the candidate module list from `modalias` |
| `probe/usb` | Captured USB tree | VID/PID/name mapping, topology reconstruction from directory names |
| `probe/acpi` | Captured `/sys/firmware` and `/sys/power` | Sleep mask parsed; absent tables yield `unknown`, never an error |
| `probe/edid` | 128-byte blob | Manufacturer `NCP`, product code, native `1920x1080` at `144.000162 Hz`, name string `LM156LF-2F01` |
| `probe/edid` | Zero-byte `edid` | `present: false`, no error, no panic |
| `probe/edid` | 100-byte `edid` | Rejected as a faulty read, not partially parsed |
| `match` | Inline profile tables, no hardware | Each of the five tie-break rules, individually and in combination |
| `match` | Two profiles tying on tier and specificity, disagreeing on `gpu.discrete` | **Conflict**, both IDs named, tier 3 fallback |
| `match` | A machine matching nothing | Tier 3 with the `fallback:` block applied |
| `match` | An ARM fixture with no DMI | Tier 3 with `reason: dmi unavailable`, no crash |
| `redact` | Synthetic DMI tree seeded with canaries | Zero canaries in the rendered bytes |
| `redact` | An injected serial, MAC and home path | The sweep refuses, names the key, exits non-zero |
| `lint` | Every file in `profiles/` | No patch-version kernel pin, no firmware file, no unclassified field, `id` matches its filename stem, every `match:` field is in the detection vocabulary |
| Golden | `ken hardware report --format json` per fixture | Byte-identical to the committed output; a diff is a review artefact |

### Negative fixtures and fuzzing

Half the interesting cases are machines that are wrong or incomplete, and those are exactly the
cases nobody captures from real hardware. They are written by hand:

- An ARM board with no `/sys/class/dmi/id` at all.
- A virtual machine whose DMI is `To Be Filled By O.E.M.` in every identity field.
- A hybrid machine with two GPUs and neither driver bound.
- A desktop with no ACPI battery and no backlight device.
- A panel whose EDID has no serial descriptor.

Each one asserts that detection degrades to `unknown` and still reaches a total resolution.
*Never a crash and never a wrong profile* is the bar, and the "never a wrong profile" half is
what the negative fixtures exist for.

Three parsers take untrusted input and are fuzzed, because each of them is attacker-influenced
in a virtual machine or through a third-party pull request:

```bash
go test -fuzz=FuzzParseEDID      -fuzztime=60s ./system/hardware/probe/
go test -fuzz=FuzzParseModalias  -fuzztime=60s ./system/hardware/probe/
go test -fuzz=FuzzLoadProfile    -fuzztime=60s ./system/hardware/profiles/
```

The corpus seeds are the committed fixtures, which is one more reason they are worth keeping
pristine.

### What CI cannot do, stated plainly

CI verifies that detection never crashes, never leaks an identifier, never resolves
inconsistently, and produces the same answer on the same machine every time. It verifies
nothing about whether the machine works.

Suspend and resume, GPU acceleration, radio behaviour, dock passthrough, panel backlight, and
whether the firmware on a machine is adequate for its own silicon are all untestable without the
physical laptop. That is not a gap in the strategy; it is the definition of what the tier system
is for, and it is why `verified:` is written by a person on the machine and why tier 1 needs a
maintainer. A green CI run and a verified machine are different claims, and the report never
conflates them.

---

## Open questions

| Question | Why it matters |
|---|---|
| `match:` beyond DMI | PCI subsystem IDs and USB IDs distinguish two machines that share a DMI string, and they are the only stable identity on a board with no DMI at all. Both are non-identifying published hardware IDs. Until the schema grows a `pci_subsystem:` key, they stay in the report as evidence and in the linter as collision warnings. |
| Two profiles, one DMI string, different tiers | The rule resolves it deterministically and the linter warns about it. Whether the linter should be able to *refuse* the pair, on the grounds that a machine matching both cannot be placed with confidence, is not decided. |
| Detection on every boot, or once at install | A BIOS update can rewrite a DMI string, and a machine silently drops from tier 2 to tier 3. Re-detecting at boot would catch it; it would also make the resolution non-deterministic across an update, which is a different cost. |
| Whether the report carries any failure context | A journal excerpt would help enormously and is full of usernames, hostnames and paths. The hardware support form asks for journal text in its own field, beside the report, which keeps the report a pure inventory. Whether that separation survives contact with a real submission is worth watching. |
| Rescue bundle signing granularity | Per ISO or per release. Per release means a machine with an old ISO cannot be helped by a current bundle; per ISO means more keys in the image. |
| Tier 1 evidence collection | `ken hardware verify` can check most of the `tiers.yaml` checklist mechanically. Whether the machine-generated evidence block is ever attached to a tier 1 sign-off, or whether it is deliberately kept out to preserve the human judgement, is a call to make before the first tier 1 is granted. |