#!/usr/bin/env python3
"""Generate the probe test fixtures.

A fixture is a machine. These are the sysfs trees the readers in package probe
consume, committed so that detection, matching and report generation can be
tested end to end on any machine, cheaply, forever.

The negative fixtures are hand-written rather than captured, because the
interesting machines are the wrong ones and nobody captures those by accident:
an ARM board with no SMBIOS, a VM whose firmware says nothing, a hybrid laptop
whose drivers never bound, a desktop with no panel.

Run from the repository root:  python3 system/hardware/probe/testdata/gen.py
"""

import os
import shutil
import struct

ROOT = os.path.dirname(os.path.abspath(__file__))


def write(path, content, mode=0o444):
    full = os.path.join(ROOT, path)
    os.makedirs(os.path.dirname(full), exist_ok=True)
    data = content.encode() if isinstance(content, str) else content
    # Fixtures are written mode 0444 to mirror what sysfs exposes. That makes
    # them read-only, so a regeneration has to unlock first rather than fail.
    if os.path.exists(full):
        os.chmod(full, 0o644)
    with open(full, "wb") as handle:
        handle.write(data)
    os.chmod(full, mode)


def machine(name):
    """Return the path for a machine fixture, clearing it first.

    Fixtures are mode 0444 and generation is additive, so a property removed
    from this script would otherwise linger on disk and a reader would keep
    testing against it. A fixture that cannot be regenerated to match its
    script is not a fixture.
    """
    full = os.path.join(ROOT, name)
    if os.path.isdir(full):
        for root, dirs, files in os.walk(full):
            for entry in dirs + files:
                target = os.path.join(root, entry)
                # The driver symlinks deliberately dangle — they point at a
                # drivers directory the fixture does not need to carry — so a
                # chmod on one raises rather than following it.
                if os.path.islink(target):
                    os.unlink(target)
                    continue
                os.chmod(target, 0o755)
        shutil.rmtree(full)
    os.makedirs(full, exist_ok=True)
    return os.path.join(name, "")


def dmi(base, fields):
    for key, value in fields.items():
        write(os.path.join(base, "class/dmi/id", key), value + "\n")


def pci(base, address, attrs, driver=None, virtfn=False):
    prefix = os.path.join(base, "bus/pci/devices", address)
    for key, value in attrs.items():
        write(os.path.join(prefix, key), value + "\n")
    if driver:
        # The kernel exposes a bound driver as a symlink named "driver" pointing
        # at ../../../bus/pci/drivers/<module>. The reader only ever takes the
        # basename, so the path above it does not have to resolve — but keeping
        # the kernel's real shape means a fixture that breaks will break for the
        # same reason a machine does.
        link = os.path.join(ROOT, prefix, "driver")
        os.makedirs(os.path.dirname(link), exist_ok=True)
        target = "../../../bus/pci/drivers/" + driver
        if os.path.islink(link) or os.path.exists(link):
            os.remove(link)
        os.symlink(target, link)
    if virtfn:
        write(os.path.join(prefix, "virtfn"), "")


def usb(base, name, attrs, driver=None):
    prefix = os.path.join(base, "bus/usb/devices", name)
    for key, value in attrs.items():
        write(os.path.join(prefix, key), value + "\n")
    if driver:
        link = os.path.join(ROOT, prefix, "driver")
        os.makedirs(os.path.dirname(link), exist_ok=True)
        target = "../../../bus/usb/drivers/" + driver
        if os.path.islink(link) or os.path.exists(link):
            os.remove(link)
        os.symlink(target, link)


def edid_block(manufacturer="NCP", product=0xF310, serial=0x12345678,
               width=1920, height=1080, clock=148500000,
               h_blank=280, v_blank=45,
               name=b"LM156LF-2F01", hdr=False, truncate=None):
    """Build a 128-byte EDID block.

    Deliberately hand-assembled rather than captured, so the expected parse in
    the test is known independently of the parser.
    """
    # 1920x1080 at 60 Hz is the CEA reduced-blank timing: 2200 x 1125 totals at
    # 148.5 MHz, which is exactly 60.000 Hz. Inventing totals from a guessed
    # blanking gives a plausible number near 60 and a refresh rate that is
    # wrong, so the totals are stated rather than derived.
    h_total = width + h_blank
    v_total = height + v_blank

    block = bytearray(128)
    block[0:8] = b"\x00\xff\xff\xff\xff\xff\xff\x00" if not hdr else b"\x12\x34\x56\x78\x9a\xbc\xde\xf0"

    # Manufacturer ID: three 5-bit letters packed across bytes 8-9-10, with a
    # reserved 0b00010 tag in the top five bits of byte 8. The first letter's low
    # two bits are byte 8's bottom bits and its high three bits are byte 9's top
    # bits; the second letter is byte 9's bottom five; the third is byte 10's
    # bottom five. Writing this the obvious byte-aligned way produces a block
    # whose decode is plausible and wrong, which is the trap.
    m1, m2, m3 = (ord(x) - ord("A") + 1 for x in manufacturer)
    struct.pack_into("<H", block, 10, product)

    # Byte 10 is shared: its top three bits are part of the product code and its
    # bottom five are the third manufacturer letter. That overlap is in the
    # format, not an artefact of this generator, so the product code is written
    # first and the letter laid over the low five bits afterwards. A product
    # code whose low five bits differ from M3 is therefore not representable,
    # which is why the default is 0xf310 rather than an arbitrary value.
    block[8] = (0x02 << 3) | (m1 & 0x03)
    block[9] = ((m1 >> 2) & 0x07) << 5 | (m2 & 0x1F)
    block[10] = (block[10] & 0xE0) | (m3 & 0x1F)
    struct.pack_into("<I", block, 12, serial)

    block[16] = 10      # week
    block[17] = 34      # year, 1990 + 34 = 2024
    block[18] = 1       # version
    block[19] = 4       # revision

    # Detailed timing descriptor at offset 54. The layout is the standard one:
    # the low byte of each active dimension sits at 2 and 3 and their high
    # nibbles at 4 and 5, the blanking widths are plain 16-bit from 6, and the
    # totals are 16-bit little-endian at 10 and 12. A DTD is exactly 18 bytes.
    dtd = bytearray(18)
    struct.pack_into("<H", dtd, 0, clock // 10000)
    dtd[2] = width & 0xFF
    dtd[3] = height & 0xFF
    dtd[4] = (width >> 8) & 0x0F
    dtd[5] = (height >> 8) & 0x0F
    struct.pack_into("<H", dtd, 6, h_blank)
    struct.pack_into("<H", dtd, 8, v_blank)
    struct.pack_into("<H", dtd, 10, h_total)
    struct.pack_into("<H", dtd, 12, v_total)
    dtd[14] = 340 & 0xFF                    # image width, mm
    dtd[15] = 190 & 0xFF                    # image height, mm
    dtd[16] = 0x1E                          # size high nibbles, sync offsets
    dtd[17] = 0x78                          # borders
    block[54:72] = dtd

    # Monitor name descriptor at 72.
    # Display descriptor: byte 0 is 0xFF, byte 1 is the tag, and the payload
    # starts at byte 2. A monitor-name descriptor carries 13 bytes of text.
    name_desc = bytearray(b"\xff\xfc" + b" " * 16)
    payload = name[:13]
    name_desc[2:2 + len(payload)] = payload
    name_desc[15] = 0x0A
    block[72:90] = name_desc

    # Two unused descriptors, marked as such.
    for offset in (90, 108):
        block[offset] = 0xFF
        block[offset + 1] = 0xFF

    block[127] = edid_checksum(block)

    if truncate is not None:
        block = block[:truncate]
    return bytes(block)


def edid_checksum(block):
    total = sum(block[:127]) % 256
    return (256 - total) % 256


def acpi(base, tables, mem_sleep="", platform_profile="",
         choices="", backlight=()):
    # An ACPI table fixture is an empty file, and that is the point: the probe
    # reads the directory listing to learn which tables exist and never opens
    # one, so a table body here would be content the design forbids carrying.
    for table in tables:
        write(os.path.join(base, "firmware/acpi/tables", table), "")
    if mem_sleep:
        write(os.path.join(base, "power/mem_sleep"), mem_sleep + "\n")
    if platform_profile:
        write(os.path.join(base, "firmware/acpi/platform_profile"), platform_profile + "\n")
        write(os.path.join(base, "firmware/acpi/platform_profile_choices"), choices + "\n")
    for name in backlight:
        write(os.path.join(base, "class/backlight", name, "max_brightness"), "9600\n")


def modules_alias(base, entries):
    lines = ["# generated fixture", "alias pci:v* drm", "alias usb:v* usbcore"]
    for pattern, module in entries:
        lines.append("alias %s %s" % (pattern, module))
    write(os.path.join(base, "modules.alias"), "\n".join(lines) + "\n")


# ---------------------------------------------------------------------------
# 1. Hybrid gaming laptop, fully bound. The reference machine.
# ---------------------------------------------------------------------------
def hybrid_laptop():
    base = machine("hybrid-laptop")
    dmi(base, {
        "sys_vendor": "ASUSTeK COMPUTER INC.",
        "product_name": "ASUS TUF Gaming F15 FX507ZC4_FX507ZC4",
        "product_version": "1.0",
        "product_family": "ASUS TUF Gaming F15",
        "product_sku": "FX507ZC4",
        "board_vendor": "ASUSTeK COMPUTER INC.",
        "board_name": "FX507ZC4",
        "board_version": "1.0",
        "chassis_type": "10",
        "bios_vendor": "American Megatrends Inc.",
        "bios_version": "3303",
        "bios_date": "12/03/2024",
        # Serials and an asset tag are present in this fixture on purpose. The
        # reader must never emit them, and the redaction canary test depends on
        # them being here.
        "product_uuid": "4C4C4544-0051-3810-8043-B9C04F503632",
        "product_serial": "N0ABCDE12345",
        "board_serial": "BOARDSERIAL9",
        "chassis_serial": "CHASSISSERIAL7",
        "chassis_asset_tag": "ASSET-TAG-778899",
        "board_asset_tag": "BOARD-ASSET-1122",
    })

    # Integrated GPU: boot_vga, xe bound.
    pci(base, "0000:00:02.0", {
        "vendor": "0x8086",
        "device": "0x46a6",
        "class": "0x030000",
        "subsystem_vendor": "0x1043",
        "subsystem_device": "0x8762",
        "revision": "0x2c",
        "modalias": "pci:v00008086d000046A6sv00001043sd00008762bc03sc00i00",
        "boot_vga": "1",
    }, driver="xe")

    # Discrete GPU: no driver bound, but the modalias still resolves.
    pci(base, "0000:01:00.0", {
        "vendor": "0x10de",
        "device": "0x2684",
        "class": "0x030000",
        "subsystem_vendor": "0x1043",
        "subsystem_device": "0x8872",
        "revision": "0xa1",
        "modalias": "pci:v000010DED00002684sv00001043sd00008872bc03sc00i00",
        "boot_vga": "0",
    })

    # Wireless.
    pci(base, "0000:00:14.3", {
        "vendor": "0x8086",
        "device": "0x51f0",
        "class": "0x028000",
        "subsystem_vendor": "0x8086",
        "subsystem_device": "0x0034",
        "modalias": "pci:v00008086d000051F0sv00008086sd00000034bc02sc00i00",
    }, driver="iwlwifi")

    # Audio: HDA controller plus the HDMI function on the dGPU.
    pci(base, "0000:00:1f.3", {
        "vendor": "0x8086",
        "device": "0xa0c8",
        "class": "0x040300",
        "modalias": "pci:v00008086d0000A0C8sv00008086sd00002638bc04sc00i00",
    }, driver="snd_hda_intel")

    # Storage: NVMe behind VMD at 00:1e.0, the usual reason detection misses it.
    pci(base, "0000:01:00.1", {
        "vendor": "0x144d",
        "device": "0xa80a",
        "class": "0x010802",
        "modalias": "pci:v0000144Dd0000A80Asv0000144Dsd0000A801bc01sc00i00",
    }, driver="nvme")

    pci(base, "0000:00:14.0", {
        "vendor": "0x8086",
        "device": "0xa01f",
        "class": "0x0c0330",
    }, driver="xhci_hcd")

    # Host bridge, class 0600. It is the only sysfs device that names the CPU
    # vendor, which is how microcode is derived without leaving sysfs for
    # /proc/cpuinfo.
    pci(base, "0000:00:00.0", {
        "vendor": "0x8086",
        "device": "0xa802",
        "class": "0x060000",
        "modalias": "pci:v00008086d0000A802sv00001028sd00008078bc06sc00i00",
    })

    usb(base, "usb1", {"idVendor": "1d6b", "idProduct": "0002", "busnum": "1",
                       "devnum": "1", "bcdDevice": "0515"})
    usb(base, "1-0:1.0", {"idVendor": "1d6b", "idProduct": "0002", "manufacturer": "Linux Foundation",
                         "product": "2.0 root hub", "busnum": "1", "devnum": "2",
                         "authorized": "1"}, driver="hub")
    usb(base, "1-1:1.0", {"idVendor": "0c45", "idProduct": "6366", "manufacturer": "Goodix",
                         "product": "Fingerprint Device", "busnum": "1", "devnum": "3",
                         "authorized": "1"}, driver="uinput")

    acpi(base, tables=["APIC", "DMAR", "DSDT", "FACP", "FPDT", "HPET", "MCFG", "MSDM"],
         mem_sleep="[s2idle] deep", platform_profile="balanced",
         choices="cool quiet balanced performance", backlight=["intel_backlight"])

    write(os.path.join(base, "class/drm/card0-eDP-1/edid"), edid_block())
    write(os.path.join(base, "class/drm/card0-eDP-1/status"), "connected\n")
    write(os.path.join(base, "class/drm/card0-eDP-1/modes"), "1920x1080 60.03*\n1920x1080 60.00\n")
    write(os.path.join(base, "class/drm/card0-HDMI-A-1/edid"), b"")
    write(os.path.join(base, "class/drm/card0-HDMI-A-1/status"), "disconnected\n")
    write(os.path.join(base, "class/backlight/intel_backlight/max_brightness"), "9600\n")

    modules_alias(base, [
        ("pci:v00008086d000046A6sv*sd*bc03sc00i00", "i915"),
        ("pci:v00008086d000046A6sv*sd*bc03sc00i00", "xe"),
        ("pci:v00008086d000051F0sv*sd*bc02sc00i00", "iwlwifi"),
        ("pci:v0000144Dd0000A80Asv*sd*bc01sc00i00", "nvme"),
        ("pci:v000010DED00002684sv*sd*bc03sc00i00", "nvidia"),
        ("pci:v00008086d0000A0C8sv*sd*bc04sc00i00", "snd_hda_intel"),
        ("pci:v00008086d0000A0C8sv*sd*bc04sc00i00", "snd_sof"),
    ])


# ---------------------------------------------------------------------------
# 2. ARM board: no SMBIOS at all.
# ---------------------------------------------------------------------------
def device_tree(base, model, compatible, serial=None):
    """Write a flattened device tree.

    Properties are sequences of NUL-terminated strings, not lines. Writing them
    as lines would produce a fixture that a correct parser rejects, which is the
    opposite of what a fixture is for.
    """
    def prop(name, values):
        payload = b""
        for value in values:
            payload += value.encode() + b"\x00"
        write(os.path.join(base, "firmware/devicetree/base", name), payload)

    prop("model", [model])
    prop("compatible", compatible)
    if serial is not None:
        # Present on the board and never read: a device-tree serial number is
        # an asset identifier exactly as a SMBIOS one is.
        prop("serial-number", [serial])


def machine_arm_laptop():
    """An ARM laptop with no SMBIOS table at all.

    Before the device-tree probe this machine could never match a profile, so a
    perfectly working ARM laptop was permanently tier 3. That is the reach
    ceiling this fixture exists to keep closed.
    """
    base = machine("arm-laptop")
    # No class/dmi/id directory is created. That absence is the whole point.
    device_tree(base,
                model="Snapdragon X Elite Laptop",
                compatible=["qcom,x1e-generic", "qcom,sc8280"],
                serial="SN-ARM-00012345")

    pci(base, "0000:01:00.0", {
        "vendor": "0x17cb",
        "device": "0x1104",
        "class": "0x028000",
        "modalias": "pci:v000017CBd00001104sv000017CBsd00000104bc02sc00i00",
    }, driver="ath12k")

    pci(base, "0000:00:02.0", {
        "vendor": "0x14e4",
        "device": "0x2764",
        "class": "0x030000",
        "modalias": "pci:v000014E4d00002764sv000010ECsd00003277bc03sc00i00",
        "boot_vga": "1",
    })

    usb(base, "usb1", {"idVendor": "1d6b", "idProduct": "0002", "busnum": "1",
                       "devnum": "1"})
    usb(base, "1-1:1.0", {"idVendor": "04f3", "idProduct": "3110",
                         "manufacturer": "Synaptics", "product": "Fingerprint",
                         "busnum": "1", "devnum": "3", "authorized": "1"},
        driver="uinput")

    acpi(base, tables=["APIC", "DSDT", "FACP", "WSMT"], mem_sleep="s2idle",
         platform_profile="balanced", choices="quiet balanced performance",
         backlight=["msm_leds"])
    write(os.path.join(base, "class/drm/card0-eDP-1/edid"),
          edid_block(manufacturer="SYN", product=0x4110, name=b"OXF14ED01",
                     clock=148500000))
    write(os.path.join(base, "class/drm/card0-eDP-1/status"), "connected\n")
    modules_alias(base, [
        ("pci:v000017CBd00001104sv*sd*bc02sc00i00", "ath12k"),
        ("pci:v000014E4d00002764sv*sd*bc03sc00i00", "msm"),
    ])


def arm_board():
    base = machine("arm-board")
    # No class/dmi/id directory is created. That absence is the whole point.
    write(os.path.join(base, "power/state"), "freeze\n")
    # A board with no device tree either is the one machine no probe can name.
    modules_alias(base, [("pci:v* drm", "drm")])


# ---------------------------------------------------------------------------
# 3. VM whose firmware says nothing useful.
# ---------------------------------------------------------------------------
def vm_oem():
    base = machine("vm-oem")
    dmi(base, {
        "sys_vendor": "To Be Filled By O.E.M.",
        "product_name": "To Be Filled By O.E.M.",
        "product_version": "To Be Filled By O.E.M.",
        "product_family": "To Be Filled By O.E.M.",
        "bios_vendor": "SeaBIOS",
        "bios_version": "1.16.2",
        "bios_date": "04/01/2014",
    })
    pci(base, "0000:00:02.0", {
        "vendor": "0x1234",
        "device": "0x1111",
        "class": "0x030000",
        "modalias": "pci:v00001234d00001111sv00001AF4sd00001100bc03sc00i00",
    })
    pci(base, "0000:00:04.0", {
        "vendor": "0x1af4",
        "device": "0x1000",
        "class": "0x020000",
    })
    modules_alias(base, [("pci:v00001234d00001111sv*sd*bc03sc00i00", "bochs-drm")])


# ---------------------------------------------------------------------------
# 4. Hybrid machine with two GPUs and neither driver bound.
# ---------------------------------------------------------------------------
def hybrid_no_drivers():
    base = machine("hybrid-no-drivers")
    dmi(base, {
        "sys_vendor": "LENOVO",
        "product_name": "82JW",
        "product_version": "None",
        "product_family": "ThinkPad X1 Carbon Gen 11",
        "chassis_type": "10",
        "bios_version": "N3XET58W",
        "bios_date": "08/15/2023",
    })
    pci(base, "0000:00:02.0", {
        "vendor": "0x8086", "device": "0x9a49", "class": "0x030000",
        "modalias": "pci:v00008086d00009A49sv000017AAsd0000181Bbc03sc00i00",
        "boot_vga": "1",
    })
    pci(base, "0000:01:00.0", {
        "vendor": "0x10de", "device": "0x24b0", "class": "0x030000",
        "modalias": "pci:v000010DED000024B0sv000017AAsd0000B121bc03sc00i00",
    })
    modules_alias(base, [
        ("pci:v00008086d00009A49sv*sd*bc03sc00i00", "i915"),
        ("pci:v000010DED000024B0sv*sd*bc03sc00i00", "nvidia"),
    ])


# ---------------------------------------------------------------------------
# 5. Desktop: no ACPI battery, no backlight, panel EDID with no serial.
# ---------------------------------------------------------------------------
def desktop_minimal():
    base = machine("desktop-minimal")
    dmi(base, {
        "sys_vendor": "Dell Inc.",
        "product_name": "OptiPlex 7010",
        "chassis_type": "3",
        "bios_date": "06/15/2022",
    })
    pci(base, "0000:00:02.0", {
        "vendor": "0x8086", "device": "0x3e92", "class": "0x030000",
        "modalias": "pci:v00008086d00003E92sv00001028sd000007BEbc03sc00i00",
        "boot_vga": "1",
    }, driver="i915")
    pci(base, "0000:00:1f.6", {
        "vendor": "0x8086", "device": "0x15b7", "class": "0x020000",
    }, driver="e1000e")
    acpi(base, tables=["APIC", "DSDT", "FACP", "HPET"], mem_sleep="", backlight=[])
    # A sparse external panel block: no serial descriptor, no name.
    write(os.path.join(base, "class/drm/card0-DP-1/edid"),
          edid_block(manufacturer="DEL", product=0x4D15, name=b"DELL U2419H", serial=0))
    write(os.path.join(base, "class/drm/card0-DP-1/status"), "connected\n")
    modules_alias(base, [("pci:v00008086d00003E92sv*sd*bc03sc00i00", "i915")])


def main():
    hybrid_laptop()
    machine_arm_laptop()
    arm_board()
    vm_oem()
    hybrid_no_drivers()
    desktop_minimal()
    print("fixtures written under", ROOT)


if __name__ == "__main__":
    main()
