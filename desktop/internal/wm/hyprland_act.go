package wm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// One neutral action id in, one Hyprland dispatcher out. This is the only file
// allowed to spell a dispatcher name.
//
// Arity is checked rather than trusted: an action arrives from a keybind or a
// script, and a dispatcher called with an empty selector silently applies to the
// focused window.

func (h hyprland) act(ctx context.Context, action Action, rest []string) (string, error) {
	if actionNeedsCompositor(action) && !hyprlandLive() {
		return "", fmt.Errorf("act %s: no live Hyprland session", action)
	}

	switch action {
	case ActionWindowFocus:
		id, err := arg(rest, 0, "window id")
		if err != nil {
			return "", err
		}
		return "", dispatch(ctx, "focuswindow", "address:"+id)

	case ActionWindowClose:
		id, err := arg(rest, 0, "window id")
		if err != nil {
			return "", err
		}
		return "", dispatch(ctx, "closewindow", "address:"+id)

	case ActionWindowFullscreen:
		id, err := arg(rest, 0, "window id")
		if err != nil {
			return "", err
		}
		// fullscreen takes no window selector: it acts on the focused window, so
		// the target is focused first.
		if err := dispatch(ctx, "focuswindow", "address:"+id); err != nil {
			return "", err
		}
		return "", dispatch(ctx, "fullscreen", "0")

	case ActionWindowFloat:
		id, err := arg(rest, 0, "window id")
		if err != nil {
			return "", err
		}
		return "", dispatch(ctx, "togglefloating", "address:"+id)

	case ActionWindowMoveToWorkspace:
		id, err := arg(rest, 0, "window id")
		if err != nil {
			return "", err
		}
		ws, err := arg(rest, 1, "workspace id")
		if err != nil {
			return "", err
		}
		return "", dispatch(ctx, "movetoworkspace", workspaceSelector(ws)+",address:"+id)

	case ActionWindowSummon:
		title, err := arg(rest, 0, "window title")
		if err != nil {
			return "", err
		}
		return "", summon(ctx, title)

	case ActionAppFocus:
		class, err := arg(rest, 0, "app id")
		if err != nil {
			return "", err
		}
		return "", dispatch(ctx, "focuswindow", "class:"+class)

	case ActionWorkspaceFocus:
		ws, err := arg(rest, 0, "workspace id")
		if err != nil {
			return "", err
		}
		return "", dispatch(ctx, "workspace", workspaceSelector(ws))

	case ActionWorkspaceCycle:
		delta, err := arg(rest, 0, "delta")
		if err != nil {
			return "", err
		}
		n, convErr := strconv.Atoi(delta)
		if convErr != nil {
			return "", fmt.Errorf("act %s: delta must be an integer, got %q", action, delta)
		}
		// r+N walks relative to the current workspace, the same selector the
		// shipped scroll binds use.
		sel := "r+" + strconv.Itoa(n)
		if n < 0 {
			sel = "r-" + strconv.Itoa(-n)
		}
		return "", dispatch(ctx, "workspace", sel)

	case ActionWorkspaceMoveToOutput:
		ws, err := arg(rest, 0, "workspace id")
		if err != nil {
			return "", err
		}
		out, err := arg(rest, 1, "output name")
		if err != nil {
			return "", err
		}
		return "", dispatch(ctx, "moveworkspacetomonitor", workspaceSelector(ws)+" "+out)

	case ActionWorkspaceToggleSpecial:
		// An empty name is the default scratchpad, which is what the bare
		// special-workspace bind toggles.
		name := ""
		if len(rest) > 0 {
			name = strings.TrimSpace(rest[0])
		}
		return "", dispatch(ctx, "togglespecialworkspace", name)

	case ActionSessionExit:
		return "", dispatch(ctx, "exit")

	case ActionOutputPower:
		state, err := arg(rest, 0, "on|off")
		if err != nil {
			return "", err
		}
		if state != "on" && state != "off" {
			return "", fmt.Errorf("act %s: state must be on or off, got %q", action, state)
		}
		args := []string{state}
		if len(rest) > 1 && rest[1] != "" {
			args = append(args, rest[1])
		}
		return "", dispatch(ctx, "dpms", args...)

	case ActionKeyboardCycleLayout:
		// A top-level hyprctl command, not a dispatcher. all, so a split or
		// external board cannot drift.
		out, err := hyprctl(ctx, "switchxkblayout", "all", "next")
		if err != nil {
			return "", err
		}
		return "", hyprctlError([]string{"switchxkblayout", "all", "next"}, out)

	case ActionSubmapEnter:
		name, err := arg(rest, 0, "submap name")
		if err != nil {
			return "", err
		}
		return "", dispatch(ctx, "submap", name)

	case ActionSubmapReset:
		return "", dispatch(ctx, "submap", "reset")

	case ActionConfigReload:
		// config-only leaves monitors alone; a full reload re-applies output
		// config and can black a screen mid-update.
		argv := []string{"reload"}
		if len(rest) > 0 && rest[0] == "config-only" {
			argv = append(argv, "config-only")
		}
		out, err := hyprctl(ctx, argv...)
		if err != nil {
			return "", err
		}
		return "", hyprctlError(argv, out)

	case ActionConfigAutoreload:
		state, err := arg(rest, 0, "on|off")
		if err != nil {
			return "", err
		}
		// The knob is phrased as a disable, so invert here not in every caller.
		disable := "true"
		if state == "on" {
			disable = "false"
		}
		return "", keyword(ctx, "misc:disable_autoreload", disable)

	case ActionCursorSet:
		theme, err := arg(rest, 0, "cursor theme")
		if err != nil {
			return "", err
		}
		size, err := arg(rest, 1, "cursor size")
		if err != nil {
			return "", err
		}
		return "", setCursor(ctx, theme, size)

	case ActionCursorReassert:
		theme, size := sessionCursor(ctx)
		if theme == "" {
			return "", nil
		}
		return "", setCursor(ctx, theme, strconv.Itoa(size))

	case ActionScreenShader:
		// A shader NAME, not a path: callers must not know the config layout.
		// Empty clears.
		name := ""
		if len(rest) > 0 {
			name = strings.TrimSpace(rest[0])
		}
		path := hyprEmpty
		if name != "" {
			found, ok := shaderPath(name)
			if !ok {
				return "", fmt.Errorf("act %s: no shader named %q", action, name)
			}
			path = found
		}
		return "", keyword(ctx, "decoration:screen_shader", path)

	case ActionFocusFollowsMouse:
		mode, err := arg(rest, 0, "follow mode")
		if err != nil {
			return "", err
		}
		n, convErr := strconv.Atoi(mode)
		if convErr != nil || n < 0 || n > 3 {
			return "", fmt.Errorf("act %s: mode must be 0..3, got %q", action, mode)
		}
		// Report the previous mode so the caller restores it exactly;
		// follow_mouse is a mode, not a flag, so a hardcoded restore would
		// overwrite a deliberate non-default.
		prev := followMouseMode(ctx)
		if err := keyword(ctx, "input:follow_mouse", strconv.Itoa(n)); err != nil {
			return "", err
		}
		return prev, nil

	case ActionWorkspaceLayout:
		ws, err := arg(rest, 0, "workspace id")
		if err != nil {
			return "", err
		}
		layout, err := arg(rest, 1, "layout name")
		if err != nil {
			return "", err
		}
		if err := keyword(ctx, "workspace", workspaceSelector(ws)+",layout:"+layout); err != nil {
			return "", err
		}
		rememberLayout(ws, layout)
		return "", nil

	case ActionBorderColors:
		active, err := arg(rest, 0, "active colour")
		if err != nil {
			return "", err
		}
		inactive, err := arg(rest, 1, "inactive colour")
		if err != nil {
			return "", err
		}
		return "", setBorderColors(ctx, active, inactive)

	case ActionGameMode:
		state, err := arg(rest, 0, "on|off")
		if err != nil {
			return "", err
		}
		switch state {
		case "on":
			return "", gameMode(ctx)
		case "off":
			// A reload rereads the config and clears every live override.
			out, err := hyprctl(ctx, "reload")
			if err != nil {
				return "", err
			}
			return "", hyprctlError([]string{"reload"}, out)
		}
		return "", fmt.Errorf("act %s: state must be on or off, got %q", action, state)

	case ActionNightLightOn:
		bin, ok := nightlightBackend()
		if !ok {
			return "", fmt.Errorf("act %s: night light is not available: install gammastep", action)
		}
		return "", nightlightStart(bin, "-m", "wayland", "-O", strconv.Itoa(nightlightTemp(rest)))

	case ActionNightLightOff:
		nightlightStop(hyprlandNightLightProcess)
		return "", nil

	case ActionInputTouchpad:
		return touchpad(ctx, rest)

	case ActionOutputCycle:
		// Super+P's job: zenith-monitor owns the mirror/extend arrangement and
		// pushes it live, so the seam steps it rather than re-deriving the layout.
		return "", runTool(ctx, "zenith-monitor", "toggle")

	case ActionOutputEnable:
		conn, err := arg(rest, 0, "connector")
		if err != nil {
			return "", err
		}
		state, err := arg(rest, 1, "on|off")
		if err != nil {
			return "", err
		}
		return "", outputEnable(ctx, conn, state)
	}

	return "", fmt.Errorf("act: unknown action %q", action)
}

// hyprEmpty is hyprlang's spelling of an empty string value; passing "" to
// `hyprctl keyword` is read as a missing argument.
const hyprEmpty = "[[EMPTY]]"

// workspaceSelector turns the workspace key the state frames carry (the name,
// or the id when the workspace is unnamed) into what a dispatcher accepts. A
// number or a relative/special selector passes through; a bare word is a
// workspace name and Hyprland only takes those with the name: prefix.
func workspaceSelector(ws string) string {
	ws = strings.TrimSpace(ws)
	if ws == "" {
		return ws
	}
	if _, err := strconv.Atoi(ws); err == nil {
		return ws
	}
	for _, prefix := range []string{"name:", "special", "r+", "r-", "m+", "m-", "e+", "e-", "+", "-"} {
		if strings.HasPrefix(ws, prefix) {
			return ws
		}
	}
	switch ws {
	case "previous", "previous_per_monitor", "empty":
		return ws
	}
	return "name:" + ws
}

// summon raises an already-open window to the current workspace and focuses it,
// matched by exact title. Ported from zenith-summon: a single-instance app
// strands its window on whatever workspace it first opened on, and every
// Quickshell window shares one app id, so the title is the only handle. The live
// title is matched first and the initial title second, so a window that retitles
// itself is still found. No match is an error, which is what lets the keybind
// fall through to launching the app; a found window is always focused, so a
// partial raise never triggers a duplicate launch.
func summon(ctx context.Context, title string) error {
	raw, err := hyprctl(ctx, "clients", "-j")
	if err != nil {
		return err
	}
	var clients []struct {
		Address      string `json:"address"`
		Title        string `json:"title"`
		InitialTitle string `json:"initialTitle"`
	}
	if err := json.Unmarshal(raw, &clients); err != nil {
		return err
	}
	addr := ""
	for _, c := range clients {
		if c.Title == title {
			addr = c.Address
			break
		}
	}
	if addr == "" {
		for _, c := range clients {
			if c.InitialTitle == title {
				addr = c.Address
				break
			}
		}
	}
	if addr == "" {
		return fmt.Errorf("act %s: no window titled %q", ActionWindowSummon, title)
	}
	sel := "address:" + addr
	// The active workspace is best effort, matching the script: the window is
	// only moved when the current workspace reads, then always focused.
	if raw, err := hyprctl(ctx, "activeworkspace", "-j"); err == nil {
		var ws struct {
			ID int `json:"id"`
		}
		if json.Unmarshal(raw, &ws) == nil && ws.ID != 0 {
			_ = dispatch(ctx, "movetoworkspacesilent", strconv.Itoa(ws.ID)+","+sel)
		}
	}
	return dispatch(ctx, "focuswindow", sel)
}

// gameMode strips the compositor's decorations for a latency-first gaming pass:
// it kills present-time cost (blur, shadow, rounding, opacity blending,
// animations) and flips to the low-latency path. Fullscreen-only VRR and
// tearing on a sole fullscreen window are left to the user's own config, which
// 0.53 can only set through windowrules and not through a live keyword.
// decoration.gameMode off reloads the config, which rereads it and clears every
// override here.
func gameMode(ctx context.Context) error {
	for _, kv := range [][2]string{
		{"general:allow_tearing", "true"},
		{"decoration:rounding", "0"},
		{"decoration:active_opacity", "1.0"},
		{"decoration:inactive_opacity", "1.0"},
		{"decoration:blur:enabled", "false"},
		{"decoration:shadow:enabled", "false"},
		{"animations:enabled", "false"},
		{"misc:vrr", "2"},
	} {
		if err := keyword(ctx, kv[0], kv[1]); err != nil {
			return err
		}
	}
	return nil
}

// outputEnable turns one connector on or off without disturbing the layout the
// user set. A monitor rule names the whole mode, so enabling reads the output's
// current mode, position and scale back and writes them again; a bare
// "preferred, auto, 1" would reset a layout a user spent time on. A disabled
// output has nothing to read back, so it comes up on its preferred mode and the
// compositor's own placement.
func outputEnable(ctx context.Context, conn, state string) error {
	switch state {
	case "off":
		return keyword(ctx, "monitor", conn+",disable")
	case "on":
		rule := conn + ",preferred,auto,1"
		if mons, err := readMonitors(ctx); err == nil {
			for _, m := range mons {
				if m.Name == conn && m.Width > 0 && m.Height > 0 {
					rule = fmt.Sprintf("%s,%dx%d@%.2f,%dx%d,%g", conn, m.Width, m.Height, m.RefreshRate, m.X, m.Y, m.Scale)
				}
			}
		}
		return keyword(ctx, "monitor", rule)
	}
	return fmt.Errorf("act %s: state must be on or off, got %q", ActionOutputEnable, state)
}

// touchpad drives the FN touchpad key. The tool that flips the device is
// zenith-touchpad, the one place that knows how this compositor addresses a
// libinput device; keeping a second copy here would let the key, the Hub and
// the shell disagree about whether the pad is on.
func touchpad(ctx context.Context, args []string) (string, error) {
	mode := "toggle"
	if len(args) > 0 && args[0] != "" {
		mode = args[0]
	}
	switch mode {
	case "on", "off", "toggle", "status", "restore":
	default:
		return "", fmt.Errorf("act input.touchpad: mode must be on|off|toggle|status|restore, got %q", mode)
	}
	if _, err := exec.LookPath("zenith-touchpad"); err != nil {
		return "", fmt.Errorf("act input.touchpad: not available: zenith-touchpad is not installed")
	}
	out, err := exec.CommandContext(ctx, "zenith-touchpad", mode).Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		return "", fmt.Errorf("act input.touchpad: zenith-touchpad %s: %w", mode, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// setBorderColors pushes the palette's border colours into the running
// compositor. A reload rereads the config and puts the configured colours back,
// which is why the daemon calls this again after every reload it triggers.
func setBorderColors(ctx context.Context, active, inactive string) error {
	rgbActive, rgbInactive := hyprRGB(active), hyprRGB(inactive)
	if rgbActive == "" && rgbInactive == "" {
		return fmt.Errorf("act %s: no usable colour in %q/%q", ActionBorderColors, active, inactive)
	}
	if rgbActive != "" {
		if err := keyword(ctx, "general:col.active_border", rgbActive); err != nil {
			return err
		}
	}
	if rgbInactive != "" {
		if err := keyword(ctx, "general:col.inactive_border", rgbInactive); err != nil {
			return err
		}
	}
	return nil
}

// hyprRGB is empty for a non-hex value, so a missing role is skipped not mis-set.
func hyprRGB(hex string) string {
	h := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(h) != 6 {
		return ""
	}
	for _, r := range h {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return ""
		}
	}
	return "rgb(" + h + ")"
}

// setCursor retargets the running compositor's pointer theme.
func setCursor(ctx context.Context, theme, size string) error {
	out, err := hyprctl(ctx, "setcursor", theme, size)
	if err != nil {
		return err
	}
	return hyprctlError([]string{"setcursor", theme, size}, out)
}

// sessionCursor is the pointer theme and size the session asked for. GTK's
// settings are the one place Zenith keeps them (there is no cursor key in
// org.zenith.desktop), with the environment the session script exports as the
// fallback. Empty theme means neither answered.
func sessionCursor(ctx context.Context) (string, int) {
	theme, size := "", 0
	if out, err := exec.CommandContext(ctx, "gsettings", "get", "org.gnome.desktop.interface", "cursor-theme").Output(); err == nil {
		theme = strings.Trim(strings.TrimSpace(string(out)), "'\"")
	}
	if out, err := exec.CommandContext(ctx, "gsettings", "get", "org.gnome.desktop.interface", "cursor-size").Output(); err == nil {
		size, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}
	if theme == "" {
		theme = os.Getenv("XCURSOR_THEME")
	}
	if size <= 0 {
		size, _ = strconv.Atoi(os.Getenv("XCURSOR_SIZE"))
	}
	if size <= 0 {
		size = 24
	}
	return theme, size
}

// shaderPath resolves a shader name to a file the compositor can load: the
// user's own directory first, then the one the package ships. ok is false when
// neither has it, so a typo is an error and not a path Hyprland logs a failure
// about on every frame.
func shaderPath(name string) (string, bool) {
	if strings.ContainsAny(name, "/\\") || strings.HasPrefix(name, ".") {
		return "", false
	}
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".config", "zenith", "hyprland", "shaders"))
	}
	dirs = append(dirs, "/usr/share/zenith/hyprland/shaders")
	for _, dir := range dirs {
		p := filepath.Join(dir, name+".glsl")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

// followMouseMode reads the live mode, empty when it cannot be read.
func followMouseMode(ctx context.Context) string {
	out, err := hyprctl(ctx, "getoption", "input:follow_mouse", "-j")
	if err != nil {
		return ""
	}
	var o struct {
		Int int `json:"int"`
	}
	if json.Unmarshal(out, &o) != nil {
		return ""
	}
	return strconv.Itoa(o.Int)
}

// arg names the missing value, so a bad keybind reports it instead of an index.
func arg(args []string, i int, name string) (string, error) {
	if i >= len(args) || strings.TrimSpace(args[i]) == "" {
		return "", fmt.Errorf("act: missing %s", name)
	}
	return args[i], nil
}

// runTool runs one of Zenith's own helper commands and folds its stderr into
// the error, so a missing helper reads as such.
func runTool(ctx context.Context, name string, args ...string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("not available: %s is not installed", name)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, msg)
		}
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// layoutOverrides remembers the per-workspace layouts applied through this
// process. 0.53 has no readback for a workspace rule, and the bar's layout
// control draws the layout in effect, so the daemon (which handles every
// request from the shell) is the one place that can know. A workspace not in
// the table is on the global layout.
var (
	layoutMu        sync.Mutex
	layoutOverrides = map[string]string{}
)

func rememberLayout(ws, layout string) {
	layoutMu.Lock()
	layoutOverrides[ws] = layout
	layoutMu.Unlock()
}

func layoutFor(ws, global string) string {
	layoutMu.Lock()
	defer layoutMu.Unlock()
	if l, ok := layoutOverrides[ws]; ok {
		return l
	}
	return global
}

// nightlightBackend is the gamma client that holds the warm colour while the
// night light is on. hyprsunset is not packaged for Ubuntu; gammastep speaks the
// same wlr-gamma-control protocol Hyprland implements and is in the archive.
func nightlightBackend() (string, bool) {
	if _, err := exec.LookPath(hyprlandNightLightProcess); err == nil {
		return hyprlandNightLightProcess, true
	}
	return "", false
}

// actionNeedsCompositor reports whether an action talks to the compositor, as
// opposed to the helper tools and process signals that work without one.
func actionNeedsCompositor(a Action) bool {
	switch a {
	case ActionOutputCycle, ActionInputTouchpad, ActionNightLightOff:
		return false
	}
	return true
}

// nightlightTemp parses the colour temperature, defaulting to 4000 K and
// clamping to the range the gamma client accepts, so a stray keybind argument
// can never ask for a value it would reject.
func nightlightTemp(args []string) int {
	t := 4000
	if len(args) > 0 {
		if v, err := strconv.Atoi(strings.TrimSpace(args[0])); err == nil {
			t = v
		}
	}
	if t < 1000 {
		t = 1000
	}
	if t > 25000 {
		t = 25000
	}
	return t
}

// nightlightStart replaces any running backend with a fresh one warmed to the
// temperature. The backend is detached (its own session, stdio to /dev/null,
// released) so it outlives this short-lived invocation and holds the gamma
// until it is killed; the compositor restores the gamma when it goes away.
func nightlightStart(argv ...string) error {
	nightlightStop(argv[0])
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer null.Close()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = null, null, null
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// nightlightStop signals every process of this uid whose comm is name, which is
// how nightlight.off stops the backend without a pkill fork. comm truncates at
// 15 characters; the backend names fit, so an exact compare is right.
func nightlightStop(name string) {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	uid := os.Getuid()
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		b, err := os.ReadFile("/proc/" + e.Name() + "/comm")
		if err != nil || strings.TrimSpace(string(b)) != name {
			continue
		}
		if st, err := os.Stat("/proc/" + e.Name()); err != nil || !ownedBy(st, uid) {
			continue
		}
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
}

func ownedBy(info os.FileInfo, uid int) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == uid
}
