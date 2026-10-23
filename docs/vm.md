# Virtual machines

The VM manager is a window, `zenith-vm`, opened with Super+Shift+V or from the launcher. It has three plates behind a rail: a dashboard of everything you run, **Machines** (a library of your virtual machines, plus a New lane that builds one from the catalogue, from a cloud image, or from an ISO you already have), and **Remotes** (servers from `~/.ssh/config`, with live reachability and a connect button).

It is Ryoku's `ryovm`, ported. Almost nothing was rewritten.

## What runs

| Name | What it is |
|---|---|
| `zenith-vm` | The launcher (`session/zenith-vm`). No arguments opens the window, once: a second launch finds the lock held. Any other word is an engine verb and is handed to `zenith-vmd`, so `zenith-vm list` works. |
| `zenith-vmd` | The engine (`desktop/cmd/zenith-vm/zenith-vmd`), a bash script over **quickemu** and **quickget**, which are in Ubuntu's archive. One quickemu `.conf` per machine. The window never talks to QEMU itself; every verb prints one JSON document. |
| `zenith-vm-mon` | Live control of a running machine through the monitor socket quickemu opens: pause, resume, reset, memory balloon, CPU pinning, host usage, and the guest's address when it runs qemu-guest-agent. |
| `zenith-vm-ssh` | The Remotes plate's engine: reads `~/.ssh/config`, probes hosts, connects in a terminal, keeps saved passwords in the login keyring (`secret-tool`). |

The window is `shell/vm.qml` with its code in `shell/vm/`, an entry at the shell root for the reason `hub.qml` is. Its title is exactly `zenith-vm`; the float rule in `session/hyprland/window-rules.conf` and the dock's title map key on it.

Terminals (`connect`, `ssh` into a machine, the install button) are opened with `zenith-app terminal --`, so they follow the terminal chosen in Settings.

## Where things are

- Machines: `~/.local/share/zenith/vm/` (`<name>.conf`, `<name>/`; `ZENITH_VM_HOME` overrides). Deleting a machine in the window removes both.
- Catalogue and logo cache: `~/.cache/zenith/`.
- The window's defaults (cores, memory, disk, display): `~/.config/zenith/zenith-vm.json`, written by the window.
- Remote hosts added in the window: `~/.config/zenith/zenith-vm/remotes.json`, and an include file `~/.ssh/config.d/zenith-vm`. Your own `~/.ssh/config` is never rewritten.

## What it needs, and what it says when that is missing

`zenith-vm caps` reports it; the window turns each gap into a message and never fails silently.

- **KVM.** `/dev/kvm` must be readable and writable by your account. If the device is missing the window says virtualization is off in the firmware. If it is there and your account cannot open it, the window explains that the device belongs to the `kvm` group and that joining gives your account direct use of the virtualization hardware. Zenith does not change groups for you.
- **quickemu and OVMF** (the UEFI firmware). The banner names whichever is missing, and its button runs `zenith-vm setup`, which installs `quickemu ovmf` through `zenith-pkg-add` (a polkit prompt, no sudo, no terminal needed). A machine can still be imported and configured without them; it cannot be launched.
- **A SPICE viewer** (`spicy`, which quickemu pulls in, or `remote-viewer`) for the Console button.

## The network

Nothing is fetched when the window opens. The catalogue (`quickget --list-json`, which builds some release lists from the web) and the OS logos load when you open the New lane. A download starts only when you ask for it, and it goes through quickget, which checks the publisher's hash before the machine is created.

## Ported, and what is different

Ported as it was: create from the catalogue, Instant (prebuilt cloud images with cloud-init), import from an ISO, launch in a window / SPICE / headless, disposable runs, seal and restore, snapshots, resize, rename, USB and port forwards, the live monitor, and the whole Remotes plate (including Proxmox guests and tunnels).

Changed because Ubuntu or Zenith forces it:

- `setup` installs with apt through `zenith-pkg-add` instead of pacman and an AUR helper.
- The conf keys the engine stamps are `zenith_vm_*`. quickemu sources the conf as shell, so a key with a hyphen would be a syntax error.
- Terminals go through `zenith-app`, not kitty; the SSH-command copy button uses the shell's clipboard, not `wl-copy`.
- The catalogue no longer loads at startup.
- Instant machines accept SSH by key only (`ssh_pwauth: false`). quickemu forwards the guest's port 22 on every host interface, and the throwaway account (`zenith`, passwordless sudo) must not be reachable from the network with a known password.

Dropped:

- **Looking Glass** (the GPU-passthrough plate). Its backend is the Hub's `gpu vm`, which Zenith does not set up.
- **`zenith-vm-fetch`**, Ryoku's parallel downloader, is not built. Its fast path gave quickget an image nothing had checked; without the helper the engine already falls back to quickget's own download, which verifies the hash. Create from the catalogue is therefore a single stream with a log line instead of a progress bar.

Known and left alone: the VirtIO driver CD that a Windows import fetches, and the Instant cloud images, are downloaded over HTTPS from their publishers with no published hash to check against.

## Checks

```sh
sh desktop/cmd/zenith-vm/vm_test.sh        # caps, conf keys, dropped verb, launcher dispatch
cd desktop && go test ./cmd/zenith-vm/...  # the ported zenith-vm-ssh tests
```
