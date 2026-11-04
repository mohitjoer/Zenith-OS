package wm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Two properties must survive any rewrite here:
//
// A focus change emits straight from the event line with no compositor query,
// because the shell daemon resolves a surface target on every keypress from it.
//
// Other changes are debounced into a refresh of only the affected sections, so a
// burst costs one query pass instead of a storm.

// Half a 60 Hz frame: folds a burst, never a visible stale beat.
const resyncDebounce = 8 * time.Millisecond

const reconnectBackoff = 500 * time.Millisecond

// watch streams frames until ctx is done. A narrowed watch skips the reads it
// does not need, which keeps a burst cheap for a consumer that wants one kind.
// It reconnects across a compositor restart on its own, so the caller's cache
// is never left permanently cold.
func (hyprland) watch(ctx context.Context, kinds []FrameKind, emit func(Frame)) error {
	want := map[FrameKind]bool{}
	for _, k := range kinds {
		want[k] = true
	}
	wants := func(k FrameKind) bool { return len(want) == 0 || want[k] }

	for ctx.Err() == nil {
		ensureHyprlandSignature()
		if path := hyprlandEventSocket(); path != "" {
			var dialer net.Dialer
			conn, err := dialer.DialContext(ctx, "unix", path)
			if err == nil {
				// focusedmon only fires on change, so a single-monitor session
				// would never populate the cache from events alone.
				consumeHyprland(ctx, conn, emit, wants)
				_ = conn.Close()
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(reconnectBackoff):
		}
	}
	return nil
}

func consumeHyprland(ctx context.Context, conn net.Conn, emit func(Frame), wants func(FrameKind) bool) {
	state := newWatchState(ctx, emit, wants)
	state.refresh(refreshAll)
	emit(Frame{Kind: FrameReady})

	pending := time.NewTimer(time.Hour)
	if !pending.Stop() {
		<-pending.C
	}
	defer pending.Stop()

	// The reader ends when the connection closes, which a cancelled context
	// forces below, so there is nothing to wait on when this returns.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	lines := make(chan string, 256)
	go func() {
		defer close(lines)
		scan := bufio.NewScanner(conn)
	scanLoop:
		for scan.Scan() {
			select {
			case lines <- scan.Text():
			case <-ctx.Done():
				break scanLoop
			}
		}
		if err := scan.Err(); err != nil {
			return
		}
	}()

	armed := false
	var dirty refreshMask
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-lines:
			if !ok {
				return
			}
			if mon, isFocus := parseFocusedMon(line); isFocus && wants(FrameFocus) {
				emit(Frame{Kind: FrameFocus, FocusedOutput: mon})
			}
			if name, removed := parseMonitorRemoved(line); removed && wants(FrameFocus) {
				// A removed output must not linger until the debounce: the next
				// keybind would target a dead monitor.
				emit(Frame{Kind: FrameFocus, FocusedOutput: focusedFallback(ctx, name)})
			}
			dirty |= state.event(line)
			if dirty == 0 {
				continue
			}
			if !armed {
				armed = true
				pending.Reset(resyncDebounce)
			}
		case <-pending.C:
			armed = false
			state.refresh(dirty)
			dirty = 0
		}
	}
}

type hyprMonitor struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	Scale         float64 `json:"scale"`
	Focused       bool    `json:"focused"`
	Make          string  `json:"make"`
	Model         string  `json:"model"`
	PhysicalWidth int     `json:"physicalWidth"`
	// Hyprland can report disabled=true for a live DPMS-awake output. The field
	// is kept for fixtures, but state derives from the real mode.
	Disabled              bool     `json:"disabled"`
	X                     int      `json:"x"`
	Y                     int      `json:"y"`
	RefreshRate           float64  `json:"refreshRate"`
	Transform             int      `json:"transform"`
	VRR                   bool     `json:"vrr"`
	AvailableModes        []string `json:"availableModes"`
	MirrorOf              string   `json:"mirrorOf"`
	ColorManagementPreset string   `json:"colorManagementPreset"`
	SdrBrightness         float64  `json:"sdrBrightness"`
	ActiveWorkspace       struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"activeWorkspace"`
}

// readMonitors is read once per resync and threaded through, so workspaces and
// windows do not each re-query the monitor list to resolve a name.
func readMonitors(ctx context.Context) ([]hyprMonitor, error) {
	out, err := hyprctl(ctx, "monitors", "all", "-j")
	if err != nil {
		return nil, err
	}
	var mons []hyprMonitor
	if err := json.Unmarshal(out, &mons); err != nil {
		return nil, err
	}
	return mons, nil
}

// monitorOutputs maps the compositor's monitors onto outputs. full adds the
// editor detail (position, rotation, VRR, physical modes) the display page needs
// and the lean watch frame omits.
func monitorOutputs(mons []hyprMonitor, full bool) []Output {
	outputs := make([]Output, 0, len(mons))
	for _, m := range mons {
		o := Output{
			Name:            m.Name,
			Width:           m.Width,
			Height:          m.Height,
			Scale:           m.Scale,
			Focused:         m.Focused,
			ActiveWorkspace: workspaceKey(m.ActiveWorkspace.ID, m.ActiveWorkspace.Name),
			Make:            m.Make,
			Model:           m.Model,
			PhysicalWidth:   m.PhysicalWidth,
			Disabled:        m.Width <= 0 || m.Height <= 0,
		}
		if full {
			o.X = m.X
			o.Y = m.Y
			o.Transform = m.Transform
			o.VRR = m.VRR
			modes := make([]string, 0, len(m.AvailableModes))
			for _, am := range m.AvailableModes {
				modes = append(modes, hyprModeString(am))
			}
			o.Modes = modes
			o.Mode = currentHyprMode(m, modes)
			o.Mirror = mirrorName(m.MirrorOf)
			o.ColorMode = hyprColorMode(m.ColorManagementPreset)
			o.SdrBrightness = m.SdrBrightness
		}
		outputs = append(outputs, o)
	}
	return outputs
}

// hyprModeString strips Hyprland's trailing "Hz" so an advertised mode reads as
// "WxH@rate" and round-trips cleanly back into a monitor rule.
func hyprModeString(s string) string {
	s = strings.TrimSpace(s)
	if n := len(s); n >= 2 && strings.EqualFold(s[n-2:], "Hz") {
		s = strings.TrimSpace(s[:n-2])
	}
	return s
}

// currentHyprMode picks the advertised mode matching the monitor's live width,
// height and refresh, so the editor's current mode is one the picker also lists.
func currentHyprMode(m hyprMonitor, modes []string) string {
	prefix := fmt.Sprintf("%dx%d@", m.Width, m.Height)
	want := int(m.RefreshRate + 0.5)
	for _, s := range modes {
		if !strings.HasPrefix(s, prefix) {
			continue
		}
		if f, err := strconv.ParseFloat(s[len(prefix):], 64); err == nil && int(f+0.5) == want {
			return s
		}
	}
	return fmt.Sprintf("%dx%d@%d", m.Width, m.Height, want)
}

// mirrorName drops Hyprland's "none" sentinel so an un-mirrored output reads as
// the empty string the neutral shape uses.
func mirrorName(s string) string {
	if s == "" || s == "none" {
		return ""
	}
	return s
}

// hyprColorMode folds Hyprland's colour-management preset onto the neutral
// colour mode the editor shows: an HDR preset is hdr, sRGB/auto/none are srgb,
// anything else is a wide-gamut profile.
func hyprColorMode(preset string) string {
	switch preset {
	case "hdr", "hdredid":
		return "hdr"
	case "", "srgb", "auto":
		return "srgb"
	}
	return "wide"
}

func focusedName(mons []hyprMonitor) string {
	for _, m := range mons {
		if m.Focused {
			return m.Name
		}
	}
	return ""
}

// globalLayout reads general:layout, the layout a workspace is on until a
// per-workspace rule says otherwise. 0.53 reports no layout per workspace.
func globalLayout(ctx context.Context) string {
	out, err := hyprctl(ctx, "getoption", "general:layout", "-j")
	if err != nil {
		return ""
	}
	var o struct {
		Str string `json:"str"`
	}
	if json.Unmarshal(out, &o) != nil {
		return ""
	}
	return o.Str
}

func readWorkspaces(ctx context.Context, mons []hyprMonitor) []Workspace {
	out, err := hyprctl(ctx, "workspaces", "-j")
	if err != nil {
		return nil
	}
	var raw []struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		Monitor       string `json:"monitor"`
		Windows       int    `json:"windows"`
		HasFullscreen bool   `json:"hasfullscreen"`
		TiledLayout   string `json:"tiledLayout"`
	}
	if json.Unmarshal(out, &raw) != nil {
		return nil
	}
	// Active is per output, not per session, so it comes from the monitor list.
	active := make(map[string]bool, len(mons))
	for _, m := range mons {
		if k := workspaceKey(m.ActiveWorkspace.ID, m.ActiveWorkspace.Name); k != "" {
			active[k] = true
		}
	}
	global := ""
	list := make([]Workspace, 0, len(raw))
	for _, w := range raw {
		key := workspaceKey(w.ID, w.Name)
		layout := w.TiledLayout
		if layout == "" {
			if global == "" {
				global = globalLayout(ctx)
			}
			layout = layoutFor(key, global)
		}
		list = append(list, Workspace{
			ID:     key,
			Name:   w.Name,
			Output: w.Monitor,
			Active: active[key],
			// A negative id is how Hyprland marks a scratchpad workspace.
			Special:    isSpecialWorkspace(w.ID, w.Name),
			Windows:    w.Windows,
			Fullscreen: w.HasFullscreen,
			Layout:     layout,
		})
	}
	return list
}

// isSpecialWorkspace separates scratchpads from the negative ids name-based
// workspaces also get: specials occupy -99..-2, and the name says so too.
func isSpecialWorkspace(id int, name string) bool {
	return (id >= -99 && id <= -2) || strings.HasPrefix(name, "special:")
}

func readWindows(ctx context.Context, mons []hyprMonitor) []Window {
	out, err := hyprctl(ctx, "clients", "-j")
	if err != nil {
		return nil
	}
	var raw []struct {
		Address   string `json:"address"`
		Class     string `json:"class"`
		Title     string `json:"title"`
		Monitor   int    `json:"monitor"`
		Floating  bool   `json:"floating"`
		Mapped    bool   `json:"mapped"`
		At        []int  `json:"at"`
		Size      []int  `json:"size"`
		Workspace struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"workspace"`
		FocusHistoryID int `json:"focusHistoryID"`
	}
	if json.Unmarshal(out, &raw) != nil {
		return nil
	}
	// The client list reports a monitor index while everything else uses names.
	names := make(map[int]string, len(mons))
	for _, m := range mons {
		names[m.ID] = m.Name
	}
	list := make([]Window, 0, len(raw))
	for _, c := range raw {
		w := Window{
			ID:         c.Address,
			AppID:      c.Class,
			Title:      c.Title,
			Workspace:  workspaceKey(c.Workspace.ID, c.Workspace.Name),
			Output:     names[c.Monitor],
			FocusOrder: c.FocusHistoryID,
			Floating:   c.Floating,
		}
		if len(c.At) == 2 {
			w.X, w.Y = c.At[0], c.At[1]
		}
		if len(c.Size) == 2 {
			w.Width, w.Height = c.Size[0], c.Size[1]
		}
		list = append(list, w)
	}
	return list
}

// readKeyboard returns the active layout and the loaded set. The main keyboard
// is the first with a usable active keymap; per-device layouts are not surfaced
// because the bar shows one indicator.
//
// Hyprland reports the transient sentinel "ERROR" as a keyboard's active_keymap
// while its xkb state is mid-transition (a virtual device coming and going), and
// no further activelayout event follows once it settles, so publishing it
// latches the bar on "ERROR" forever. Treat it like an empty keymap: skip that
// device, and if none resolves, return "" so the caller keeps the last-good
// layout instead of a value the compositor never confirmed.
//
// Hyprland picks "main" itself when several keyboards are present, and on a
// laptop with a vendor hotkey device the main one is not always the typing
// keyboard; a keymap from any device that has one is better than none.
func readKeyboard(ctx context.Context) (string, []string) {
	out, err := hyprctl(ctx, "devices", "-j")
	if err != nil {
		return "", nil
	}
	var devs struct {
		Keyboards []struct {
			ActiveKeymap string `json:"active_keymap"`
			Layout       string `json:"layout"`
			Main         bool   `json:"main"`
		} `json:"keyboards"`
	}
	if json.Unmarshal(out, &devs) != nil {
		return "", nil
	}
	pick := func(requireMain bool) (string, []string) {
		for _, k := range devs.Keyboards {
			if (requireMain && !k.Main) || !usableKeymap(k.ActiveKeymap) {
				continue
			}
			var all []string
			for _, l := range strings.Split(k.Layout, ",") {
				if l = strings.TrimSpace(l); l != "" {
					all = append(all, l)
				}
			}
			return k.ActiveKeymap, all
		}
		return "", nil
	}
	if active, all := pick(true); active != "" {
		return active, all
	}
	return pick(false)
}

// usableKeymap reports whether an active_keymap is a real layout rather than the
// empty string or Hyprland's transient "ERROR" transition sentinel.
func usableKeymap(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.EqualFold(s, "error")
}

// state is one snapshot for callers that ask once and exit.
func (hyprland) state(ctx context.Context) (Snapshot, error) {
	if !hyprlandLive() {
		return Snapshot{}, fmt.Errorf("state: no live Hyprland session")
	}
	mons, err := readMonitors(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	active, all := readKeyboard(ctx)
	return Snapshot{
		FocusedOutput:   focusedName(mons),
		Outputs:         monitorOutputs(mons, true),
		Workspaces:      readWorkspaces(ctx, mons),
		Windows:         readWindows(ctx, mons),
		KeyboardLayout:  active,
		KeyboardLayouts: all,
	}, nil
}

// workspaceKey is the name when there is one and the id otherwise: the name is
// what a dispatcher accepts and what a user recognises; the id is the fallback
// for an unnamed workspace.
func workspaceKey(id int, name string) string {
	if name != "" {
		return name
	}
	if id == 0 {
		return ""
	}
	return strconv.Itoa(id)
}

// focusedFallback is queried, not guessed: Hyprland refocuses a survivor on
// unplug, and naming the wrong one would send the next surface to a monitor that
// is gone.
func focusedFallback(ctx context.Context, gone string) string {
	mons, err := readMonitors(ctx)
	if err != nil {
		return ""
	}
	if focused := focusedName(mons); focused != gone {
		return focused
	}
	return ""
}

// parseFocusedMon matches focusedmon exactly. focusedmonv2 is rejected on
// purpose: matching v1 exactly means a future Hyprland that drops it fails in
// the tests instead of going silently stale.
func parseFocusedMon(line string) (string, bool) {
	ev, data, ok := strings.Cut(line, ">>")
	if !ok || ev != "focusedmon" {
		return "", false
	}
	mon, _, _ := strings.Cut(data, ",")
	if mon == "" {
		return "", false
	}
	return mon, true
}

func parseMonitorRemoved(line string) (string, bool) {
	ev, data, ok := strings.Cut(line, ">>")
	if !ok || ev != "monitorremoved" {
		return "", false
	}
	name := strings.TrimSpace(data)
	if name == "" {
		return "", false
	}
	return name, true
}

type refreshMask uint8

const (
	refreshOutputs refreshMask = 1 << iota
	refreshWorkspaces
	refreshWindows
	refreshKeyboard
	refreshAll = refreshOutputs | refreshWorkspaces | refreshWindows | refreshKeyboard
)

type watchState struct {
	ctx      context.Context
	monitors []hyprMonitor
	windows  []Window
	last     map[FrameKind]Frame
	emit     func(Frame)
	wants    func(FrameKind) bool
}

func newWatchState(ctx context.Context, emit func(Frame), wants func(FrameKind) bool) *watchState {
	return &watchState{ctx: ctx, emit: emit, wants: wants, last: make(map[FrameKind]Frame)}
}

// publish drops a frame identical to the last one of its kind, so an unchanged
// section never wakes a consumer.
func (s *watchState) publish(f Frame) {
	if !s.wants(f.Kind) {
		return
	}
	if old, ok := s.last[f.Kind]; ok && reflect.DeepEqual(old, f) {
		return
	}
	s.last[f.Kind] = f
	s.emit(f)
}

func (s *watchState) refresh(dirty refreshMask) {
	if dirty&(refreshOutputs|refreshWorkspaces|refreshWindows) != 0 &&
		(s.monitors == nil || dirty&refreshOutputs != 0) {
		mons, err := readMonitors(s.ctx)
		if err != nil {
			return
		}
		s.monitors = mons
	}
	if dirty&refreshOutputs != 0 {
		s.publish(Frame{Kind: FrameOutputs, Outputs: monitorOutputs(s.monitors, false)})
		s.publish(Frame{Kind: FrameFocus, FocusedOutput: focusedName(s.monitors)})
	}
	if dirty&refreshWorkspaces != 0 && s.wants(FrameWorkspaces) {
		if ws := readWorkspaces(s.ctx, s.monitors); ws != nil {
			s.publish(Frame{Kind: FrameWorkspaces, Workspaces: ws})
		}
	}
	if dirty&refreshWindows != 0 && s.wants(FrameWindows) {
		if wins := readWindows(s.ctx, s.monitors); wins != nil {
			s.windows = wins
			s.publish(Frame{Kind: FrameWindows, Windows: wins})
		}
	}
	if dirty&refreshKeyboard != 0 && s.wants(FrameKeyboard) {
		if active, all := readKeyboard(s.ctx); active != "" {
			s.publish(Frame{Kind: FrameKeyboard, KeyboardLayout: active, KeyboardLayouts: all})
		}
	}
}

func (s *watchState) event(line string) refreshMask {
	name, data, ok := strings.Cut(line, ">>")
	if !ok {
		return 0
	}
	switch name {
	case "activewindow":
		// v1 has no address; v2 supplies the unambiguous window identity.
		return 0
	case "activewindowv2":
		if !s.wants(FrameWindows) || s.focusWindow(data) {
			return 0
		}
		// A newly opened window may arrive before the debounced client snapshot.
		return refreshWindows
	case "windowtitle", "windowtitlev2", "changefloatingmode", "urgent":
		return refreshWindows
	case "openwindow", "closewindow", "movewindow", "movewindowv2", "fullscreen":
		return refreshWindows | refreshWorkspaces
	case "workspace", "workspacev2", "focusedmon", "focusedmonv2",
		"createworkspace", "createworkspacev2", "destroyworkspace", "destroyworkspacev2",
		"moveworkspace", "moveworkspacev2", "renameworkspace", "activespecial", "activespecialv2":
		return refreshOutputs | refreshWorkspaces | refreshWindows
	case "monitoradded", "monitoraddedv2", "monitorremoved", "configreloaded":
		return refreshAll
	case "activelayout":
		return refreshKeyboard
	}
	return 0
}

// focusWindow promotes the known address without re-querying the compositor.
// The slice is copied: frames already handed to consumers must remain immutable.
func (s *watchState) focusWindow(address string) bool {
	address = strings.TrimSpace(address)
	if address != "" && !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}
	at := -1
	for i, w := range s.windows {
		if w.ID == address {
			at = i
			break
		}
	}
	if address != "" && at < 0 {
		return false
	}
	wins := append([]Window(nil), s.windows...)
	if at < 0 {
		focused := false
		for _, w := range wins {
			focused = focused || w.FocusOrder == 0
		}
		if !focused {
			return true
		}
		for i := range wins {
			if wins[i].FocusOrder >= 0 {
				wins[i].FocusOrder++
			}
		}
	} else {
		old := wins[at].FocusOrder
		if old == 0 {
			return true
		}
		for i := range wins {
			if i != at && wins[i].FocusOrder >= 0 && (old < 0 || wins[i].FocusOrder < old) {
				wins[i].FocusOrder++
			}
		}
		wins[at].FocusOrder = 0
	}
	s.windows = wins
	s.publish(Frame{Kind: FrameWindows, Windows: wins})
	return true
}
