package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"zenith/desktop/internal/wm"
)

func reloadCoverPath() string {
	if shellDir != "" {
		return filepath.Join(shellDir, "scripts", "zenith-reload-cover")
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "zenith-reload-cover")
	}
	return "zenith-reload-cover"
}

var reloadCoverOutput = func(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, reloadCoverPath(), "begin").Output()
}

var reloadCoverBegin = func() string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := reloadCoverOutput(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func beginReloadCover() string {
	return reloadCoverBegin()
}

type suspendRequest struct {
	cancel context.CancelFunc
}

type daemon struct {
	mu           sync.Mutex
	paintSig     chan struct{} // coalescing wake for the palette/border worker
	stageSig     chan struct{} // coalescing wake for the unified stage worker
	stageForce   atomic.Bool   // a pending forced regenerate (effect/quality change, refresh, re-cut)
	stageGen     atomic.Bool   // a pending enable: reuse an existing cut, else generate
	stageBusy    atomic.Bool   // a cut/inpaint is in flight (for the status/topic)
	ledsSig      chan struct{} // coalescing wake for the OpenRGB worker
	quit         chan struct{}
	closed       bool
	ln           net.Listener
	lock         *os.File      // exclusive single-daemon guard, held until exit
	voiceMu      sync.Mutex    // serializes voice (Super+`) toggles
	voiceOn      bool          // dictation active; guarded by voiceMu
	voiceStop    chan struct{} // reaps the live voxtype state stream; nil when none
	prompter     *prompter     // GNOME keyring system prompter (nil when unavailable)
	wmc          *wm.Client    // sole path to the compositor
	wmMu         sync.Mutex    // guards the compositor state the wm watcher keeps warm
	activeMon    string        // focused output, kept warm by watchWindowManager
	wmOutputs    []wm.Output
	wmWorkspaces []wm.Workspace
	wmWindows    []wm.Window
	wmKbdLayout  string   // active xkb layout, kept warm by watchWindowManager
	wmKbdList    []string // configured layouts, in switch order
	wmOverview   bool     // the compositor's native overview is open (niri)
	wmReady      bool
	wmVersions   map[string]int // frame kind -> publishes since daemon start
	wmTopic      *stateTopic
	topicsMu     sync.Mutex             // guards topics
	topics       map[string]*stateTopic // subsystem name -> pub/sub state topic
	callsMu      sync.Mutex             // guards calls
	calls        map[string]callFunc    // "topic.method" -> control handler
	clip         *clipState             // clipboard history state (nil until started)
	tray         *trayState             // system tray watcher/host state (nil until started)
	wallMu       sync.Mutex             // guards wall
	wall         wallpaperFrame         // last wallpaper frame seen from zenith-wallpaper; feeds the stage worker
	polkit       *polkitAgent           // PolicyKit1 authentication agent (nil when unavailable)
	settings     *settingsStore         // shell.json store (nil until startSettings)
	pp           *powerProfilesState    // power-profiles-daemon bus state
	keypress     *keypressManager       // evdev key stream while the overlay is enabled
	sun          *sunState              // latest weather sunrise/sunset window
	sleepMu      sync.RWMutex
	sleep        *sleepCycle // coordinated login1 suspend transaction; guarded by sleepMu
	suspendReqMu sync.Mutex
	suspendReq   map[string]*suspendRequest
	nightlight   *nightlightState
}

func (d *daemon) currentSleepCycle() *sleepCycle {
	d.sleepMu.RLock()
	defer d.sleepMu.RUnlock()
	return d.sleep
}

func validSuspendToken(token string) bool {
	if token == "" || len(token) > 96 {
		return false
	}
	for _, r := range token {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

func (d *daemon) startSuspendRequest(token string) (context.Context, *suspendRequest, error) {
	if !validSuspendToken(token) {
		return nil, nil, fmt.Errorf("invalid transaction token")
	}
	d.suspendReqMu.Lock()
	defer d.suspendReqMu.Unlock()
	if d.suspendReq == nil {
		d.suspendReq = make(map[string]*suspendRequest)
	}
	if pending, exists := d.suspendReq[token]; exists {
		if pending.cancel == nil {
			delete(d.suspendReq, token)
			return nil, nil, context.Canceled
		}
		return nil, nil, fmt.Errorf("transaction is already active")
	}
	ctx, cancel := context.WithCancel(context.Background())
	request := &suspendRequest{cancel: cancel}
	d.suspendReq[token] = request
	return ctx, request, nil
}

func (d *daemon) finishSuspendRequest(token string, request *suspendRequest) {
	d.suspendReqMu.Lock()
	defer d.suspendReqMu.Unlock()
	if d.suspendReq[token] == request {
		delete(d.suspendReq, token)
	}
	request.cancel()
}

func (d *daemon) cancelSuspendRequest(token string) error {
	if !validSuspendToken(token) {
		return fmt.Errorf("invalid transaction token")
	}
	d.suspendReqMu.Lock()
	if d.suspendReq == nil {
		d.suspendReq = make(map[string]*suspendRequest)
	}
	if request, exists := d.suspendReq[token]; exists {
		if request.cancel != nil {
			request.cancel()
		}
		d.suspendReqMu.Unlock()
		return nil
	}
	tombstone := &suspendRequest{}
	d.suspendReq[token] = tombstone
	d.suspendReqMu.Unlock()
	time.AfterFunc(time.Minute, func() {
		d.suspendReqMu.Lock()
		if d.suspendReq[token] == tombstone {
			delete(d.suspendReq, token)
		}
		d.suspendReqMu.Unlock()
	})
	return nil
}

func (d *daemon) setSleepCycle(cycle *sleepCycle) {
	d.sleepMu.Lock()
	d.sleep = cycle
	d.sleepMu.Unlock()
}

func runDaemon() error {
	// The provider (not this daemon) binds to the live compositor. Its opaque
	// per-session instance handle is the only thing the take-over needs.
	wmc := wm.Open()
	path := sockPath()
	if c, err := net.DialTimeout("unix", path, 300*time.Millisecond); err == nil {
		c.Close()
		// Take over only a provably stale incumbent: one bound to a different
		// compositor instance than ours (a previous session's). A same-session
		// incumbent, an unidentified one (older binary, ok=false), or our own
		// missing instance are left alone, so a genuine double-start refuses.
		caps, _ := wmc.Caps()
		incInstance, ok := daemonSignature(path)
		if !shouldTakeOver(caps.Instance, incInstance, ok) {
			return fmt.Errorf("a daemon is already running at %s", path)
		}
		quitStaleDaemon(path)
	}
	// A lock, not just the socket: a runtime dir cleaned under a live daemon (or
	// a socket file removed by hand) makes the dial above succeed at nothing, and
	// two daemons each holding the polkit agent and the clipboard watcher is a bug. The lock
	// lives for the life of the process and is released by exit, so a crash never
	// wedges the next start.
	lock, err := holdDaemonLock(path + ".lock")
	if err != nil {
		return err
	}
	_ = os.Remove(path)
	// The control socket drives session-scoped actions; keep it owner-only so a
	// second local user can't connect. net.Listen would otherwise leave it at
	// the ambient umask (0755 at the usual 022 -> unconnectable by others, but
	// that is luck, not policy). Forcing the umask around Listen makes the
	// socket 0700 atomically, with no world-visible window to chmod after.
	old := syscall.Umask(0o077)
	ln, err := net.Listen("unix", path)
	syscall.Umask(old)
	if err != nil {
		return err
	}

	d := &daemon{
		paintSig: make(chan struct{}, 1),
		stageSig: make(chan struct{}, 1),
		ledsSig:  make(chan struct{}, 1),
		quit:     make(chan struct{}),
		wmc:      wmc,
		sun:      daySun,
	}
	d.ln = ln
	d.lock = lock // held for the process lifetime: closing it would free the guard

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		d.signalQuit()
	}()

	go d.runtimeJanitor()
	d.bootstrap()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-d.quit:
				// Quickshell is systemd's to stop; the one thing the shell leaves
				// running on our account is the evdev reader behind the key overlay.
				if d.keypress != nil {
					d.keypress.configure(false, "all")
				}
				_ = os.Remove(path)
				return nil
			default:
				continue
			}
		}
		go d.handle(conn)
	}
}

// shouldTakeOver reports whether a daemon starting now should displace the
// incumbent on the control socket. It takes over only a provably stale
// incumbent: one whose compositor instance handle (ok) differs from ours
// (mySig). A same-session incumbent, an unidentified one (older binary,
// ok=false), or our own missing handle (mySig=="") leave it in place, so a
// genuine double-start still refuses.
func shouldTakeOver(mySig, incSig string, ok bool) bool {
	return ok && mySig != "" && incSig != mySig
}

// daemonSignature asks the daemon at path for its compositor instance handle. ok
// is false when the query fails or the reply is an error (an older daemon that
// predates the verb), so the caller treats the incumbent as unidentified and
// does not displace it. An empty handle from a current daemon is a valid answer
// (ok=true, sig="").
func daemonSignature(path string) (sig string, ok bool) {
	conn, err := net.DialTimeout("unix", path, 300*time.Millisecond)
	if err != nil {
		return "", false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := fmt.Fprintln(conn, "signature"); err != nil {
		return "", false
	}
	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	resp := strings.TrimSpace(string(buf[:n]))
	if strings.HasPrefix(resp, "err ") {
		return "", false
	}
	return resp, true
}

// quitStaleDaemon tells the incumbent to quit and waits, bounded, for it to
// release the control socket so this daemon can bind. The incumbent reaps its
// own supervised quickshell children on quit, so the takeover strands no process
// holding a single-instance lock.
func quitStaleDaemon(path string) {
	if conn, err := net.DialTimeout("unix", path, 300*time.Millisecond); err == nil {
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		fmt.Fprintln(conn, "quit")
		conn.Close()
	}
	for range 30 {
		conn, err := net.DialTimeout("unix", path, 100*time.Millisecond)
		if err != nil {
			return
		}
		conn.Close()
		time.Sleep(100 * time.Millisecond)
	}
}

// bootstrap brings the shell up: the settings store (sole writer of shell.json,
// served over the settings topic), the clipboard history and its selection
// watcher, the tray host, the keyring prompter, the theme workers, the in-shell
// wallpaper surface and the first wallpaper. Quickshell itself is systemd's:
// zenith-shell.service starts and restarts it, the daemon only talks to it.
func (d *daemon) bootstrap() {
	d.startSettings()
	d.startKeypress()
	d.startClipboard()
	d.startTray()
	d.startWeather()
	d.startMusic()
	d.startCalendar()
	d.startPowerProfiles()
	d.startNetwork()
	d.startNightlight()
	go d.watchSunMode()
	d.startOsd()
	d.prompter = startKeyringPrompter()
	if d.prompter != nil {
		d.prompter.mon = d.activeMonitor
	}
	d.startSession()
	d.startPolkit()
	d.startUpdates()
	go d.paintWorker()
	d.startStage()
	go d.watchWallpaper()
	go d.watchMatugenKnobs()
	go d.ledsWorker()
	d.startWM()
	go d.watchPowerSounds()
	go d.watchAutoPowerSaver()
	d.startSleepWake()
}

// holdDaemonLock takes the exclusive single-daemon lock, retrying briefly: a
// stale daemon we just asked to quit may still be letting go of it. The returned
// file must stay open for as long as the daemon runs.
func holdDaemonLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			return f, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("another zenith-shell daemon holds %s", path)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (d *daemon) signalQuit() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed {
		d.closed = true
		close(d.quit)
		if d.ln != nil {
			_ = d.ln.Close()
		}
	}
}

func (d *daemon) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return
	}
	cmd := strings.TrimSpace(line)
	// The keyring island returns the typed secret on a second line so it never
	// reaches a command line (and thus world-readable /proc/<pid>/cmdline).
	if strings.HasPrefix(cmd, "keyring-respond") {
		secret, _ := r.ReadString('\n')
		fmt.Fprintln(conn, d.keyringRespond(cmd, strings.TrimRight(secret, "\r\n")))
		return
	}
	// Typed subsystem state rides the same socket: a long-lived subscription
	// stream, or a request/response control call. Both outlive the one-shot
	// command deadline, which they clear for themselves.
	if strings.HasPrefix(cmd, "subscribe ") {
		d.serveSubscription(conn, cmd)
		return
	}
	if strings.HasPrefix(cmd, "call ") {
		d.serveCalls(conn, r, cmd)
		return
	}
	// A clipboard capture helper streams new selection bytes after its header
	// line, so they never reach a command line.
	if strings.HasPrefix(cmd, "clip-ingest ") {
		fmt.Fprintln(conn, d.clipIngest(cmd, r))
		return
	}
	if strings.HasPrefix(cmd, "clip-copy ") {
		fmt.Fprintln(conn, d.clipCopy(cmd))
		return
	}
	fmt.Fprintln(conn, d.dispatch(cmd))
}

var surfaceCommands = map[string]string{
	// One bare kebab verb per shell surface, spelled to match its CustomShortcut
	// id, so a compositor keybind reaches any surface as `zenith-shell <id>` where
	// no global-shortcuts protocol exists (niri). Flag surfaces land on ShellState
	// through the surface bus's style-independent consumer; frame-menu surfaces
	// land on the per-monitor FrameMenuManager. Both are the same transition a
	// CustomShortcut press runs in-process.
	"bar-toggle":         "barToggle",
	"launcher":           "launcher",
	"overview":           "overview",
	"visualizer":         "visualizer",
	"visualizer-overlay": "visualizer-overlay",
	"visualizer-place":   "visualizer-place",
	"quicksettings":      "quick-settings",
	"wallpaper-menu":     "wallpaper",
	"clipboard":          "clipboard",
	"stash":              "stash",
	"screenshot":         "quick-settings#capture",
	"compress":           "stash#compress",
	"install":            "stash#install",
	// Preserved aliases so nothing scripted today breaks: the menu-prefixed
	// spellings and the file-picker desktop entries (install-app.desktop,
	// compress-video.desktop) land on the same surfaces they always have.
	"menu screenshot":   "screenshot",
	"menu stash":        "stash",
	"menu app-launcher": "launcher",
}

// route resolves an IPC-style command to the single shell's IpcHandler config,
// target, and function it triggers. The consolidated shell renders every surface
// in-process, so a matched command always resolves to config and target "shell"
// and the openSurface entry point on its surface bus. ok is false for commands
// that need more than one IPC call (wallpaper, reload, status, ...).
func route(cmd string) (config, target, fn string, ok bool) {
	if _, ok := surfaceCommands[cmd]; ok {
		return "shell", "shell", "openSurface", true
	}
	if _, ok := menuID(cmd); ok {
		return "shell", "shell", "openSurface", true
	}
	return "", "", "", false
}

// unavailable names the verbs kept from Ryoku's CLI that this desktop has no
// backend for, with the reason, so a caller gets a clear error and not "unknown
// command".
var unavailable = map[string]string{
	"unlock-prepare": "hyprlock unlocks without a daemon handshake",
	"lock-quiesce":   "apt upgrades do not cut the live session over",
	"state":          "the shell no longer reports palette visibility",
}

func validBarStyleID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

// dispatch turns one command line into actions and returns "ok" or "err ...".
func (d *daemon) dispatch(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "err empty command"
	}
	cmd, args := fields[0], fields[1:]
	if why, ok := unavailable[cmd]; ok {
		return "err " + cmd + ": not available on Zenith: " + why
	}
	routeCmd := cmd
	switch cmd {
	case "menu":
		// menu close clears every open menu. App launcher and dedicated frame
		// surfaces keep their established menu commands.
		switch {
		case len(args) == 1 && args[0] == "close":
			return d.menuClose()
		case len(args) == 1 && (args[0] == "app-launcher" || args[0] == "screenshot" || args[0] == "stash"):
			routeCmd = line
		default:
			if _, ok := menuID(line); !ok {
				return "err menu: unknown or malformed id"
			}
			routeCmd = line
		}
	case "bar":
		// The data-layout verbs (list/catalog/move/show/hide/set/position/form/
		// defaults/settings) take a verb first; the reveal grammar takes an edge
		// first, so the first word disambiguates the two without collision.
		if len(args) >= 1 && barCLIVerbs[args[0]] {
			return d.barCLI(args)
		}
		// bar <edge|all> <toggle|reveal|hide> drives the bar reveal state.
		edge, action, ok := parseBarEdge(args)
		if !ok {
			return "err bar: expected <top|bottom|left|right|all> <toggle|reveal|hide>, or a widget verb (list|catalog|move|show|hide|set|position|form|defaults|settings)"
		}
		return d.barToggle(edge, action)
	case "dock":
		return d.dockCLI(args)
	}
	if _, _, _, ok := route(routeCmd); ok {
		id, menu := menuID(routeCmd)
		if !menu {
			id = surfaceCommands[routeCmd]
		}
		return shellIpc("openSurface", d.activeMonitor(), id)
	}

	switch cmd {
	case "voice":
		return d.voice()
	case "barstyle":
		if len(args) != 1 || !validBarStyleID(args[0]) {
			return "err barstyle: expected one product id"
		}
		if d.settings == nil {
			return "err barstyle: settings not ready"
		}
		value, _ := json.Marshal(args[0])
		if err := d.settings.patch("barStyle", value); err != nil {
			return "err barstyle: " + err.Error()
		}
		return "ok"
	case "lock":
		// lock status is the reference check (prints locked/unlocked, exit 0);
		// bare lock starts hyprlock unless it is already running.
		if len(args) == 1 && args[0] == "status" {
			if isLocked() {
				return "locked"
			}
			return "unlocked"
		}
		if len(args) != 0 {
			return "err lock: expected no arguments or status"
		}
		return lockSession()
	case "sleep-ready":
		if len(args) != 0 {
			return "err sleep-ready: takes no arguments"
		}
		cycle := d.currentSleepCycle()
		if cycle == nil || !cycle.ready() {
			return "err sleep-ready: guard is not ready"
		}
		return "ok"
	case "suspend":
		cycle := d.currentSleepCycle()
		if cycle == nil {
			return "err suspend: sleep guard is unavailable"
		}
		if len(args) == 0 {
			if err := cycle.requestSuspend(); err != nil {
				return "err suspend: " + err.Error()
			}
			return "ok"
		}
		if len(args) != 2 || args[0] != "transaction" {
			return "err suspend: expected no arguments or transaction <token>"
		}
		ctx, request, err := d.startSuspendRequest(args[1])
		if err != nil {
			return "err suspend: " + err.Error()
		}
		defer d.finishSuspendRequest(args[1], request)
		if err := cycle.requestSuspendContext(ctx); err != nil {
			return "err suspend: " + err.Error()
		}
		return "ok"
	case "suspend-cancel":
		if len(args) != 2 || args[0] != "transaction" {
			return "err suspend-cancel: expected transaction <token>"
		}
		if err := d.cancelSuspendRequest(args[1]); err != nil {
			return "err suspend-cancel: " + err.Error()
		}
		return "ok"
	case "audio":
		if len(args) != 1 {
			return "err audio: expected up, down, or mute"
		}
		return d.audio(args[0])
	case "brightness":
		if len(args) != 1 {
			return "err brightness: expected up or down"
		}
		return d.brightness(args[0])
	case "hub":
		switch len(args) {
		case 1:
			return d.hub(args[0], "")
		case 2:
			return d.hub(args[0], args[1])
		}
		return "err hub: expected open [section] or close"
	case "stage":
		// One verb surface for Ryostage (docs/stage.md): set-effect records the
		// three-way effect per wallpaper; set-layer applies a layer's arrangement
		// flags (enabled/front/depth); add-layer / cut-layer / remove-layer manage
		// the manual layers; refresh forces a re-cut; cancel kills a running engine
		// child; clear drops the current wall's generated artifacts; status
		// snapshots the topic; models proxies the engine catalogue. QML normally
		// subscribes to the `stage` topic; these verbs carry intent the other way.
		if len(args) == 0 {
			return "err stage: expected a verb"
		}
		// Trailing path / json arguments may contain spaces, so recover them from
		// the raw line by fixed-field split instead of the whitespace args, which
		// would truncate at the first space.
		switch args[0] {
		case "set-effect":
			if len(args) < 2 {
				return "err stage set-effect: expected off|depth|parallax"
			}
			eff := stageEffect(args[1])
			if eff != stageEffectOff && eff != stageEffectDepth && eff != stageEffectParallax {
				return "err stage set-effect: expected off|depth|parallax"
			}
			d.stageSetEffect(eff)
			return "ok"
		case "refresh":
			d.stageForce.Store(true)
			d.scheduleStage()
			return "ok"
		case "cancel":
			stageCancel()
			return "ok"
		case "status":
			return d.stageStatusJSON()
		case "models":
			return stageModelsJSON()
		case "cut-layer":
			if len(args) < 2 {
				return "err stage cut-layer: expected a path"
			}
			path, err := d.stageCutLayer(restField(line, 3))
			if err != nil {
				return "err stage cut-layer: " + err.Error()
			}
			return "ok " + path
		case "set-layer":
			if len(args) < 3 {
				return "err stage set-layer: expected <index> <json>"
			}
			if err := d.stageSetLayer(args[1], restField(line, 4)); err != nil {
				return "err stage set-layer: " + err.Error()
			}
			return "ok"
		case "add-layer":
			if len(args) < 2 {
				return "err stage add-layer: expected a path"
			}
			path, err := d.stageAddLayer(restField(line, 3))
			if err != nil {
				return "err stage add-layer: " + err.Error()
			}
			return "ok " + path
		case "remove-layer":
			if len(args) < 2 {
				return "err stage remove-layer: expected an index"
			}
			if err := d.stageRemoveLayer(args[1]); err != nil {
				return "err stage remove-layer: " + err.Error()
			}
			return "ok"
		case "clear":
			d.stageClear()
			return "ok"
		}
		return "err stage: unknown verb " + args[0]
	case "theme":
		// Apply a colour scheme by writing theme.theme through the settings store
		// (the sole writer of shell.json), which validates the name, persists,
		// broadcasts, and schedules the retheme. `theme catalog` is served
		// client-side in main.go, so it never reaches here. Join the args so a
		// name with spaces ("Tokyo Night") survives the command split.
		if len(args) == 0 {
			return "err theme: expected a scheme name (or `catalog`)"
		}
		if args[0] == "remove" && len(args) >= 2 {
			// `theme remove <id>` uninstalls a downloaded scheme. Reset the
			// applied scheme to the Zenith default first when it is the one being
			// removed, so the desktop never keeps a scheme whose files are gone;
			// the settings store drives the retheme off that write.
			id := strings.Join(args[1:], " ")
			if d.settings != nil && d.settings.themeName() == id {
				def, _ := json.Marshal("Default")
				if err := d.settings.patch("theme.theme", def); err != nil {
					return "err theme: " + err.Error()
				}
			}
			if err := removeUserTheme(id); err != nil {
				return "err theme: " + err.Error()
			}
			return "ok"
		}
		if d.settings == nil {
			return "err theme: settings not ready"
		}
		name, _ := json.Marshal(strings.Join(args, " "))
		if err := d.settings.patch("theme.theme", name); err != nil {
			return "err theme: " + err.Error()
		}
		return "ok"
	case "gtk":
		// A curated light/dark scheme is owned by the Hub: it writes colors.json
		// itself, so the paint worker idles and nothing here would land the
		// desktop's GTK settings for the new mode. The Hub calls this instead of
		// setting gsettings on its own, which keeps one writer for gtk-theme,
		// color-scheme and accent-color rather than two that can disagree.
		if len(args) != 2 || args[0] != "apply" {
			return "err gtk: expected `apply <light|dark>`"
		}
		if args[1] != "light" && args[1] != "dark" {
			return "err gtk: mode must be light or dark"
		}
		matugenReload(args[1])
		return "ok"
	case "reload":
		return d.reload()
	case "status":
		return d.status()
	case "ping":
		return "ok"
	case "signature":
		// Opaque per-session instance handle from the provider, string-compared
		// to tell a stale incumbent from a same-session double-start. Empty means
		// no live session.
		caps, _ := d.wmc.Caps()
		return caps.Instance
	case "quit":
		d.signalQuit()
		return "ok"
	case "plugin":
		// plugin <id> [toggle] -> toggle that plugin's frame popout. Reserved for
		// future per-host actions (show/hide); toggle is the default.
		if len(args) < 1 {
			return "err plugin: missing id"
		}
		return shellIpc("openSurface", d.activeMonitor(), "plugin:"+args[0])
	case "plugins":
		// plugins reload -> the per-monitor PluginPopouts watch plugins.json and
		// re-discover on change, so a Settings save retunes live; this is a no-op
		// acknowledgement kept for an explicit force path.
		if len(args) >= 1 && args[0] == "reload" {
			return "ok"
		}
		return "err plugins: unknown action"
	case "sound":
		// an event cue from a config outside the daemon (zenith-shot fires the
		// shutter on capture). The daemon owns the assets and playback; it only
		// accepts a known event name.
		if len(args) != 1 || !knownSound(args[0]) {
			return "err sound: expected a known event"
		}
		playSound(args[0])
		return "ok"

	default:
		return "err unknown command: " + cmd
	}
}

// voice handles the Super+` tap. With dictation running it toggles Voxtype's
// transcription and the pill's mic-wave together (first tap records and shows
// the wave; the next stops, transcribes, and hides it). With dictation off it
// just flashes an "off" note on the pill. Tap-to-toggle rides only the key-press
// edge: Hyprland won't deliver a release once the modifier lifts first, which
// would otherwise leave a hold-to-talk recording stuck on.
//
// Dictation can also end without a tap (Voxtype stops on silence or finishes
// transcribing), so the ON edge starts a state watcher that closes the surface
// when Voxtype reports idle (#244). The watcher is spawned before `record
// start` so it cannot miss the transition into recording.
func (d *daemon) voice() string {
	d.voiceMu.Lock()
	defer d.voiceMu.Unlock()
	if !dictationReady() {
		d.voiceOn = false
		return shellIpc("openSurface", d.activeMonitor(), "voice-off")
	}
	d.voiceOn = !d.voiceOn
	if d.voiceOn {
		stop := make(chan struct{})
		d.voiceStop = stop
		go d.watchVoice(stop)
		voxtypeRecord("start")
		return shellIpc("openSurface", d.activeMonitor(), "voice")
	}
	if d.voiceStop != nil {
		close(d.voiceStop)
		d.voiceStop = nil
	}
	voxtypeRecord("stop")
	return shellIpc("closeSurface", d.activeMonitor(), "voice")
}

func capDur(d, max time.Duration) time.Duration {
	if d > max {
		return max
	}
	return d
}

// reload restarts the shell through its unit. The reload cover is raised first
// and the new shell opens it again once it has drawn, so the restart never runs
// in front of the user. The reply waits for the shell to answer again: answering
// "ok" while it dies on the same error is what makes a reload look like it does
// nothing on a black screen.
func (d *daemon) reload() string {
	_ = beginReloadCover()
	old := shellMainPID()
	if out, err := exec.Command("systemctl", "--user", "reload", shellUnit).CombinedOutput(); err != nil {
		return "err reload: systemctl --user reload " + shellUnit + ": " + strings.TrimSpace(string(out))
	}
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(300 * time.Millisecond)
		if pid := shellMainPID(); pid != "" && pid != "0" && pid != old && shellReady() {
			return "ok"
		}
	}
	if shellReady() {
		return "ok"
	}
	return "err reload: the shell did not come back; see journalctl --user -u " + shellUnit
}

const shellUnit = "zenith-shell.service"

// shellMainPID is the unit's main process, "" when systemd cannot say.
func shellMainPID() string {
	out, err := exec.Command("systemctl", "--user", "show", "-p", "MainPID", "--value", shellUnit).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// shellReady asks the running shell whether it has finished loading.
func shellReady() bool {
	out, err := exec.Command("quickshell", "ipc", "-p", shellRoot(), "call", "lifecycle", "ready").Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// status reports the shell unit in the form Ryoku's status printed for its
// supervised components.
func (d *daemon) status() string {
	state := "stopped"
	if out, _ := exec.Command("systemctl", "--user", "is-active", shellUnit).Output(); true {
		switch strings.TrimSpace(string(out)) {
		case "active":
			state = "running"
		case "activating":
			state = "starting"
		}
	}
	return "shell: " + state
}
