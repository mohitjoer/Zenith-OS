// zenith-shell = single control plane for the Zenith desktop shell. as
// `zenith-shell daemon` it serves the shell's state (tray, network, clipboard,
// weather, ...) on one Unix socket and answers its control calls; Quickshell
// itself is started and restarted by systemd, not by the daemon. as
// `zenith-shell <command>` it forwards that command to the daemon over the
// socket; Hyprland keybinds use this client form.
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const sockName = "zenith-shell.sock"

func sockPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, sockName)
}

func main() {
	// Before anything else: strip Quickshell's inherited crash-recovery handles
	// so every quickshell we spawn honours its -p config instead of relaunching ours.
	scrubQuickshellCrashEnv()
	args := os.Args[1:]
	if len(args) == 0 {
		usage(os.Stderr)
		os.Exit(2)
	}
	switch args[0] {
	case "-h", "--help", "help":
		usage(os.Stdout)
		return
	}
	if args[0] == "daemon" {
		if err := runDaemon(); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-shell:", err)
			os.Exit(1)
		}
		return
	}
	if args[0] == "__clip-ingest" {
		// wl-paste --watch helper: read the new selection and stream it to the
		// daemon. Internal, not a user verb.
		runClipIngest()
		return
	}
	if args[0] == "theme" && len(args) == 2 && args[1] == "catalog" {
		// The colour-scheme catalog is static (compiled in) and larger than the
		// daemon's 8192-byte socket reply, so print it here without a round trip;
		// `theme <name>` still forwards to the daemon to apply.
		fmt.Println(themeCatalogJSON())
		return
	}
	if args[0] == "matugen-preview" && len(args) == 2 {
		// Generate (never apply) the palette an image would produce, for the
		// ryowalls live preview. Runs standalone -- no daemon round trip -- so the
		// same generator serves apply and preview from one binary.
		if err := matugenPreview(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-shell:", err)
			os.Exit(1)
		}
		return
	}
	if args[0] == "icons" {
		// The icon index (every name reachable through the theme chain) is far
		// larger than the daemon's socket reply and needs no running daemon, so
		// build and print it here, like `theme catalog`. `icons <name>...` prints
		// one resolved path per line for spot checks.
		os.Exit(runIcons(args[1:]))
	}
	if args[0] == "browser-host" {
		fmt.Fprintln(os.Stderr, "zenith-shell: browser-host: not available on Zenith: no browser extension ships with it")
		os.Exit(1)
	}
	if err := sendCommand(strings.Join(args, " ")); err != nil {
		fmt.Fprintln(os.Stderr, "zenith-shell:", err)
		os.Exit(1)
	}
}

// sendCommand: forward one command line to the daemon and print any reply that
// isn't a bare "ok".
func sendCommand(line string) error {
	conn, err := net.DialTimeout("unix", sockPath(), 2*time.Second)
	if err != nil {
		return fmt.Errorf("daemon not reachable at %s (is `zenith-shell daemon` running?)", sockPath())
	}
	defer conn.Close()
	// Generous: a reload waits for the surface to come back and report, and a
	// client that times out first turns a real failure into a silent success.
	_ = conn.SetDeadline(time.Now().Add(45 * time.Second))
	if _, err := fmt.Fprintln(conn, line); err != nil {
		return err
	}
	// The daemon writes one reply line and closes, so read to EOF rather than a
	// fixed buffer: `bar catalog --json` and `bar list --json` can exceed one
	// read's worth.
	data, readErr := io.ReadAll(conn)
	if len(data) == 0 && readErr != nil {
		return fmt.Errorf("no reply from the daemon: %v", readErr)
	}
	resp := strings.TrimSpace(string(data))
	if strings.HasPrefix(resp, "err ") {
		return fmt.Errorf("%s", strings.TrimPrefix(resp, "err "))
	}
	if resp != "" && resp != "ok" {
		fmt.Println(resp)
	}
	return nil
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: zenith-shell <verb> [args]

  daemon                                   run the state daemon (systemd runs this)
  launcher | overview | clipboard | stash | quicksettings | wallpaper-menu
  screenshot | compress | install | bar-toggle | visualizer | visualizer-overlay
  visualizer-place                         open that shell surface
  menu <id>|close                          open a bar menu, or close them all
  plugin <id> | plugins reload             toggle a plugin's popout
  lock [status]                            lock the session with hyprlock
  suspend [transaction <token>]            lock, then suspend
  suspend-cancel transaction <token>       cancel a pending suspend transaction
  sleep-ready                              is the sleep guard armed
  theme [<scheme>|catalog|remove <id>]     apply a colour scheme, or list them
  gtk apply <light|dark>                   land the GTK settings for a mode
  matugen-preview <image>                  the palette an image would produce
  icons [<name>...]                        the icon index, or resolve names
  bar <list|catalog|move|show|hide|set|position|form|defaults|settings>
  bar <top|bottom|left|right|all> <toggle|reveal|hide>
  barstyle <id>                            pick a bar style
  dock <show|hide|edge|autohide|pin|unpin|list>
  hub open [page] | hub close              Zenith Settings
  stage <set-effect|set-layer|add-layer|cut-layer|remove-layer|refresh|cancel|clear|status|models>
  audio <up|down|mute> | brightness <up|down>
  sound <event>                            play a cue (shutter, volume-change, ...)
  clip-copy <mime> <file>                  put a file on the clipboard
  voice                                    dictation toggle (needs voxtype)
  reload | status | ping | quit | signature

not available on Zenith: browser-host, unlock-prepare, lock-quiesce, state
`)
}
