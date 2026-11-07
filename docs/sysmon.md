# Asura Resource Monitor (`asura-sysmon`)

**Asura Resource Monitor** is the native system telemetry and task management utility for Asura OS.

The canonical standalone repository is hosted at **[github.com/ASURA-OS/asura-sysmon](https://github.com/ASURA-OS/asura-sysmon)**.
The tree in this repository keeps byte-identical mirrors:
- `desktop/cmd/asura-sysmon/` mirrors `cmd/asura-sysmon/`
- `shell/sysmon/` and `shell/sysmon.qml` mirror `ui/`
- `session/applications/asura-sysmon.desktop` mirrors `asura-sysmon.desktop`

Parity is enforced automatically on push and PR via `.github/workflows/sysmon-sync.yml`.

| Component | Path | Purpose |
|---|---|---|
| `asura-sysmon` | `/usr/bin/asura-sysmon` | High-performance Go telemetry data plane and single-instance window launcher. |
| Window UI | `/usr/share/asura/sysmon/` | Pure monochrome Quickshell window with performance gauges, task table, and drive utilization. |
| Desktop Entry | `/usr/share/applications/asura-sysmon.desktop` | System menu and application launcher integration. |

## Shortcuts

- `Ctrl + Shift + Escape`: Universal task manager shortcut.
- `Super + Shift + Escape`: Compositor app shortcut.

## CLI Subcommands

```text
asura-sysmon stats [--interval MS]     JSON: live CPU %, RAM, Swap, Disks, Network I/O
asura-sysmon proc [--sort FIELD]       JSON: active process list with memory, PID, threads
asura-sysmon kill <PID> [--force]      Terminates target PID (SIGTERM or SIGKILL)
asura-sysmon                           Launches the Resource Monitor GUI window
```

## Telemetry Sources

- **CPU**: Read directly from `/proc/stat` with sub-second sample delta, core distribution, load average from `/proc/loadavg`, and thermal sensors from `/sys/class/thermal/` and `/sys/class/hwmon/`.
- **Memory & Swap**: Instantaneous reads from `/proc/meminfo` computing physical memory, cache, buffers, and swap utilization.
- **Storage**: Monitored filesystems from `/proc/mounts` and `statvfs` system calls.
- **Network**: Real-time interface byte counters from `/proc/net/dev`.
- **Processes**: `/proc/[PID]/stat`, `status`, and `cmdline`.
