package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// shellDir (ZENITH_SHELL_DIR): the shell tree of a repo checkout, for a daemon
// talking to a Quickshell started from it. Empty on an installed box, where the
// tree is /usr/share/zenith/shell.
var shellDir = os.Getenv("ZENITH_SHELL_DIR")

// shellRoot is the tree the running shell was started on; its path is what
// `quickshell ipc -p` identifies the instance by.
func shellRoot() string {
	if shellDir != "" {
		return shellDir
	}
	return "/usr/share/zenith/shell"
}

var frameBarMenuIDs = map[string]bool{
	"quick-settings": true,
	"theme":          true,
	"wallpaper":      true,
	"weather":        true,
}

func menuID(cmd string) (string, bool) {
	fields := strings.Fields(cmd)
	if len(fields) != 2 || fields[0] != "menu" {
		return "", false
	}
	id := fields[1]
	// Strip any "#page" suffix before checking the catalog; the full id
	// (including the suffix) is returned so QML receives the deep-link page.
	baseID := id
	if h := strings.IndexByte(id, '#'); h >= 0 {
		baseID = id[:h]
	}
	if !frameBarMenuIDs[baseID] {
		return "", false
	}
	return id, true
}

func validSessionID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') &&
			(r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') &&
			r != '_' && r != '.' && r != '-' {
			return false
		}
	}
	return true
}

// voxtypeRecord starts or stops dictation on the running Voxtype daemon (the
// Super+` tap). Voxtype is an optional AUR app (voxtype-bin); absent, this is a
// no-op and the voice surface stays a plain mic meter. `voxtype record` drives
// the user service in place (Voxtype's own hotkey is disabled so the shell owns
// Super+`). verb is "start" or "stop".
func voxtypeRecord(verb string) {
	if _, err := exec.LookPath("voxtype"); err != nil {
		return
	}
	// reap the short-lived record client in the background so it does not linger
	// as a zombie; voxtypeRecord fires once per tap (start on show, stop on hide).
	cmd := exec.Command("voxtype", "record", verb)
	if err := cmd.Start(); err != nil {
		return
	}
	go func() { _ = cmd.Wait() }()
}

// dictationReady reports whether a Super+` tap can actually dictate: Voxtype
// installed and its user service running. When it can't, the pill shows an
// "off" note instead of a listening wave that would capture nothing.
func dictationReady() bool {
	if _, err := exec.LookPath("voxtype"); err != nil {
		return false
	}
	return exec.Command("systemctl", "--user", "is-active", "--quiet", "voxtype.service").Run() == nil
}

func stateDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state")
}

// ipcCallN invokes an IpcHandler function on the Quickshell instance started on
// path, which may have just been (re)started, so it retries briefly until the
// instance answers. Empty trailing args still go positionally.
func ipcCallN(path, target, fn string, args ...string) string {
	argv := append([]string{"ipc", "-p", path, "call", target, fn}, args...)
	var out []byte
	var err error
	for i := 0; i < 10; i++ {
		cmd := exec.Command("quickshell", argv...)
		cmd.Env = withWaylandDisplay(os.Environ())
		out, err = cmd.CombinedOutput()
		if err == nil {
			return "ok"
		}
		time.Sleep(150 * time.Millisecond)
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = err.Error()
	}
	return "err quickshell ipc " + target + "." + fn + ": " + msg
}

// withWaylandDisplay names the compositor's display when the environment lacks
// it: Quickshell finds the instance an IPC call is for by the display it runs
// on, and the daemon can start before the session imported that variable.
func withWaylandDisplay(env []string) []string {
	for _, kv := range env {
		if strings.HasPrefix(kv, "WAYLAND_DISPLAY=") {
			return env
		}
	}
	socks, _ := filepath.Glob(filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "wayland-*"))
	for _, sock := range socks {
		if !strings.HasSuffix(sock, ".lock") {
			return append(env, "WAYLAND_DISPLAY="+filepath.Base(sock))
		}
	}
	return env
}

// shellIpc invokes a "shell" IpcHandler function on the running shell. A package
// var so tests can capture the emitted call in place of a live Quickshell.
var shellIpc = func(fn string, args ...string) string {
	return ipcCallN(shellRoot(), "shell", fn, args...)
}

var (
	// lockSettle is how long a freshly started hyprlock gets to put its surface
	// up before the session counts as locked: hyprlock gives no readiness signal
	// and the compositor only holds the lock once the surface is mapped.
	lockSettle = 400 * time.Millisecond
	lockWait   = 3 * time.Second
	lockPoll   = 50 * time.Millisecond

	lockRequestMu      sync.Mutex
	lockProcessRunning = hyprlockRunning
	lockStarter        = spawnLocker
)

// hyprlockRunning reports whether this user has a hyprlock process. hyprlock
// holds the compositor's session lock for exactly as long as it lives, so its
// presence is the lock state; there is no marker to trust or to go stale.
func hyprlockRunning() bool {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, e := range ents {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		if procCommIs(e.Name(), "hyprlock") {
			if st, err := os.Stat("/proc/" + e.Name()); err == nil && ownedBy(st, os.Getuid()) {
				return true
			}
		}
	}
	return false
}

func ownedBy(info os.FileInfo, uid int) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == uid
}

// lockSession locks the screen with hyprlock unless it is already running.
func lockSession() string {
	if err := ensureSessionLocked(time.Now().Add(lockWait)); err != nil {
		return "err lock: " + err.Error()
	}
	return "ok"
}

// ensureSessionLocked serializes launch-to-locked. The process-local mutex keeps
// two requests (a keybind and logind's Lock signal, say) from starting two
// lockers; a second hyprlock would be refused by the compositor anyway.
func ensureSessionLocked(deadline time.Time) error {
	lockRequestMu.Lock()
	defer lockRequestMu.Unlock()

	if lockProcessRunning() {
		return nil
	}
	if !deadline.After(time.Now()) {
		return fmt.Errorf("lock deadline already expired")
	}
	if err := lockStarter(); err != nil {
		return err
	}
	for {
		if lockProcessRunning() {
			// Past the settle the surface is up; an immediate exit (hyprlock
			// refusing to start) has already emptied the process table.
			time.Sleep(lockSettle)
			if lockProcessRunning() {
				return nil
			}
			return fmt.Errorf("hyprlock exited right after starting")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("hyprlock did not start before the deadline")
		}
		time.Sleep(min(lockPoll, remaining))
	}
}

// spawnLocker starts hyprlock detached, so it survives a daemon restart (the
// lock must outlive whatever asked for it), on the materialised config when
// there is one and the packaged one otherwise.
func spawnLocker() error {
	bin, err := exec.LookPath("hyprlock")
	if err != nil {
		return fmt.Errorf("hyprlock is not installed")
	}
	argv := []string{}
	home, _ := os.UserHomeDir()
	for _, conf := range []string{
		filepath.Join(home, ".config", "zenith", "hyprland", "hyprlock.conf"),
		"/usr/share/zenith/hyprland/hyprlock.conf",
	} {
		if _, err := os.Stat(conf); err == nil {
			argv = []string{"-c", conf}
			break
		}
	}
	cmd := exec.Command(bin, argv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
