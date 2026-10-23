package wm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The Hyprland provider. It is the only code allowed to run hyprctl, read
// HYPRLAND_INSTANCE_SIGNATURE or spell a dispatcher name.
//
// Hyprland here is 0.53, whose configuration is hyprlang: `hyprctl dispatch`
// takes the classic "<dispatcher> <args>" form and `hyprctl keyword` changes a
// value live. Ryoku's provider spoke the Lua API of 0.55+ (hl.dsp.*, eval);
// none of that exists on this compositor, which is why every action below is a
// translation and not a copy.

// Hyprland's runtime layout: the instance signature, the two sockets, and the
// one way this provider runs hyprctl.
//
// The signature rebind matters because the shell daemon restarts mid-session
// from the user manager's environment, which can still name a compositor that
// exited days ago. hyprctl then dials a dead socket and every call silently
// no-ops. Resolving the live signature into this process fixes every later fork.

func hyprRunDir() string {
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		return filepath.Join(rt, "hypr")
	}
	return filepath.Join("/tmp", "hypr")
}

// hyprlandSocketAlive reports whether the instance a signature names still
// answers. A signature whose socket file lingers after the instance exited
// refuses the dial and reads dead. A var so tests can satisfy live() without a
// compositor.
var hyprlandSocketAlive = func(sig string) bool {
	if sig == "" {
		return false
	}
	return handleAlive(filepath.Join(hyprRunDir(), sig, ".socket.sock"))
}

// hyprlandLiveSignature picks the newest instance by socket mtime, so the
// current login wins over stale leftovers. Empty when none answer.
func hyprlandLiveSignature() string {
	entries, err := os.ReadDir(hyprRunDir())
	if err != nil {
		return ""
	}
	best := ""
	var bestMod time.Time
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sig := e.Name()
		fi, err := os.Stat(filepath.Join(hyprRunDir(), sig, ".socket.sock"))
		if err != nil || !hyprlandSocketAlive(sig) {
			continue
		}
		if best == "" || fi.ModTime().After(bestMod) {
			best, bestMod = sig, fi.ModTime()
		}
	}
	return best
}

// ensureHyprlandSignature is idempotent and cheap: the alive case is one local
// dial.
func ensureHyprlandSignature() {
	if hyprlandSocketAlive(os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")) {
		return
	}
	if live := hyprlandLiveSignature(); live != "" {
		_ = os.Setenv("HYPRLAND_INSTANCE_SIGNATURE", live)
	}
}

// hyprlandLive lets verbs fail fast instead of hanging on a dead socket.
func hyprlandLive() bool {
	ensureHyprlandSignature()
	return hyprlandSocketAlive(os.Getenv("HYPRLAND_INSTANCE_SIGNATURE"))
}

// hyprlandEventSocket prefers whichever layout exists, so a login-time race
// retries the right place.
func hyprlandEventSocket() string {
	sig := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if sig == "" {
		return ""
	}
	var candidates []string
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		candidates = append(candidates, filepath.Join(rt, "hypr", sig, ".socket2.sock"))
	}
	candidates = append(candidates, filepath.Join("/tmp", "hypr", sig, ".socket2.sock"))
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return candidates[0]
}

// hyprctl is the single choke point for every compositor call. A var so tests
// can pin the emitted argv without a live compositor.
var hyprctl = runHyprctl

func runHyprctl(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "hyprctl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
		}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("hyprctl %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return nil, fmt.Errorf("hyprctl %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

// hyprctlError turns hyprctl's textual answer into an error. hyprctl exits 0
// even when the compositor refuses a request, and prints the reason ("Invalid
// dispatcher", "no such workspace"...) instead, so a caller that only checks
// the exit status would report success for an action that did nothing. A
// successful dispatch or keyword prints "ok".
func hyprctlError(args []string, out []byte) error {
	reply := strings.TrimSpace(string(out))
	if reply == "" || reply == "ok" {
		return nil
	}
	return fmt.Errorf("hyprctl %s: %s", strings.Join(args, " "), reply)
}

// dispatch runs one classic dispatcher: `hyprctl dispatch <name> <arg>`.
func dispatch(ctx context.Context, name string, args ...string) error {
	argv := []string{"dispatch", name}
	if joined := strings.Join(args, " "); joined != "" {
		argv = append(argv, joined)
	}
	out, err := hyprctl(ctx, argv...)
	if err != nil {
		return err
	}
	return hyprctlError(argv, out)
}

// keyword applies one live config change: `hyprctl keyword <key> <value>`. The
// change lasts until the compositor rereads its config.
func keyword(ctx context.Context, key, value string) error {
	argv := []string{"keyword", key, value}
	out, err := hyprctl(ctx, argv...)
	if err != nil {
		return err
	}
	return hyprctlError(argv, out)
}

// Every entry here must actually be honoured by act below or by a lane that
// owns the matching surface. Claiming one this provider cannot perform is worse
// than omitting it: the desktop would offer a control that does nothing.
//
// Left out on purpose, with the reason the Hub's page for it stays hidden:
//   - nativeOverview, overviewBackdrop: Hyprland ships no overview, so the shell
//     draws its own.
//   - globalShortcuts, focusGrab: the Quickshell bridge modules that speak those
//     protocols are not part of this shell, so a binding would reach nothing.
//   - plugins: Zenith ships no plugin manager for the compositor.
//   - outputMirror, outputHdr: the display editor's write path is the Hub
//     backend's, and these describe what it can express.
var hyprlandCapabilities = []Capability{
	CapWorkspaces,
	CapSpecialWorkspace,
	CapWorkspaceMoveToOutput,
	CapWindowWorkspaceMap,
	CapWindowGeometry,
	CapFocusHistory,
	CapWindowRules,
	CapLayerRules,
	CapSubmap,
	CapScreenShader,
	CapLiveConfigEval,
	CapConfigReload,
	CapAnimations,
	CapCursorSet,
	CapOutputPower,
	CapKeyboardLayoutSwitch,
	CapMonitorConfig,
	CapWindowFloat,
	CapTiledLayout,
	CapSessionExit,
	CapPaletteBorder,
}

// helperCapabilities are the capabilities that exist only while their tool does.
// Offering a control whose tool is missing would be a button that fails; hiding
// it is the visible degradation, and the tool is the package's to install.
var helperCapabilities = []struct {
	capability Capability
	tool       string
}{
	{CapNightLight, hyprlandNightLightProcess},
	{CapTouchpadToggle, "zenith-touchpad"},
}

// hyprlandWindowRuleActions are the neutral window-rule action ids the Hub's
// window-rules editor may offer, in the order it should offer them. They are the
// properties `windowrule` accepts on 0.53.
var hyprlandWindowRuleActions = []string{
	"float", "tile", "pin", "fullscreen", "maximize", "center", "immediate",
	"pseudo", "norounding", "noborder", "opacity", "size", "move", "workspace",
	"idleinhibit", "suppressevent", "blur", "noanim", "blurpopups", "xray",
	"abovelock", "noshadow", "ignorealpha", "dimaround",
}

// hyprlandNightLightProcess is the gamma client the night light runs. hyprsunset,
// which Ryoku used here, is not in Ubuntu's archive; gammastep speaks the same
// wlr-gamma-control protocol Hyprland implements. Its comm name fits /proc's 15
// characters, so an exact compare is right.
const hyprlandNightLightProcess = "gammastep"

type hyprland struct{}

func (hyprland) available() bool {
	_, err := exec.LookPath("hyprctl")
	return err == nil
}

// caps is fixed apart from the helper-backed entries: Hyprland does not gain
// features while running, and the manifest is read during startup.
func (hyprland) caps(ctx context.Context) (Caps, error) {
	supports := append([]Capability(nil), hyprlandCapabilities...)
	for _, h := range helperCapabilities {
		if _, err := exec.LookPath(h.tool); err == nil {
			supports = append(supports, h.capability)
		}
	}
	return Caps{
		Name:           ProviderHyprland,
		Version:        hyprlandVersion(ctx),
		Instance:       hyprlandInstance(),
		Supports:       supports,
		WorkspaceModel: WorkspaceModelFixed,
		SettingDomains: []string{"desktop", "wm." + ProviderHyprland},
		// Relative to ~/.config. The hand-edit file comes first: it is the one the
		// Hub offers as "your own compositor config", and it wins over everything
		// the Hub writes.
		ConfigFiles:       []string{"zenith/hyprland/user.conf"},
		GeneratedFiles:    []string{"zenith/hyprland/settings.conf", "zenith/hyprland/rebinds.conf"},
		PortalBackend:     "hyprland",
		NightLightProcess: hyprlandNightLightProcess,
		WindowRuleActions: append([]string(nil), hyprlandWindowRuleActions...),
	}, nil
}

// hyprlandInstance is opaque to consumers, which only string-compare it.
func hyprlandInstance() string {
	if !hyprlandLive() {
		return ""
	}
	return os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
}

// hyprlandVersion is probed best-effort: doctor needs a manifest before any
// compositor is running.
func hyprlandVersion(ctx context.Context) string {
	if !hyprlandLive() {
		return ""
	}
	out, err := hyprctl(ctx, "version", "-j")
	if err != nil {
		return ""
	}
	var v struct {
		Tag string `json:"tag"`
	}
	if json.Unmarshal(out, &v) != nil {
		return ""
	}
	return strings.TrimSpace(v.Tag)
}
