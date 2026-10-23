package wm

import (
	"fmt"
	"strconv"
	"strings"
)

// Hyprland's capability manifest and the actions the Hub backend performs on the
// running compositor. The window-state actions (focus, close, workspace moves)
// belong to the shell daemon; this backend only reloads, sets the cursor,
// drives the keybind recorder's submap and flips a couple of live options.

// Every entry here must actually be honoured by this package. Claiming one it
// cannot perform is worse than omitting it: the Hub would offer a control that
// does nothing.
var capsManifest = []Capability{
	CapWindowRules,
	CapLayerRules,
	CapSubmap,
	CapPlugins,
	CapLiveConfigEval,
	CapConfigReload,
	CapAnimations,
	CapCursorSet,
	CapOutputPower,
	CapKeyboardLayoutSwitch,
	CapMonitorConfig,
	CapOutputMirror,
	CapOutputHdr,
	CapWindowFloat,
	CapTiledLayout,
}

// windowRuleActions are the neutral window-rule action ids genWindowRule and
// genLayerRule accept, in the order the Hub offers them. It is the source the
// window-rules editor reads, so a control is never shown for a property the
// config writer would drop.
var windowRuleActions = []string{
	"float", "tile", "pin", "fullscreen", "maximize", "center", "immediate",
	"pseudo", "norounding", "noborder", "opacity", "size", "move", "workspace",
	"idleinhibit", "suppressevent", "blur", "noanim", "blurpopups", "xray",
	"abovelock", "ignorealpha", "dimaround",
}

func hyprlandCaps() Caps {
	version, _ := compositorVersion()
	return Caps{
		Name:           ProviderHyprland,
		Version:        version,
		Instance:       instanceHandle(),
		Supports:       capsManifest,
		WorkspaceModel: WorkspaceModelFixed,
		// wm.hyprland.* keys are the exclusives; everything else is desktop.*.
		SettingDomains: []string{"desktop", "wm." + ProviderHyprland},
		// The user's own file is the escape hatch for anything the Hub has no
		// control for; it is sourced last and wins.
		ConfigFiles:       []string{"zenith/hyprland/user.conf"},
		GeneratedFiles:    GeneratedFiles(),
		PortalBackend:     "hyprland",
		WindowRuleActions: windowRuleActions,
	}
}

// instanceHandle is opaque to consumers, which only string-compare it.
func instanceHandle() string {
	if !live() {
		return ""
	}
	return strings.TrimSpace(getenv("HYPRLAND_INSTANCE_SIGNATURE"))
}

// runAct performs one neutral action. An action this package does not implement
// is ErrUnsupported, not a no-op that reports success.
func runAct(a Action, rest []string) (string, error) {
	arg := func(i int, what string) (string, error) {
		if i >= len(rest) || strings.TrimSpace(rest[i]) == "" {
			return "", fmt.Errorf("act %s: missing %s", a, what)
		}
		return rest[i], nil
	}
	switch a {
	case ActionConfigReload:
		// config-only leaves monitors alone; a full reload re-applies output
		// config and can black a screen mid-update.
		if len(rest) > 0 && rest[0] == "config-only" {
			_, err := ctl("reload", "config-only")
			return "", err
		}
		_, err := ctl("reload")
		return "", err

	case ActionConfigAutoreload:
		state, err := arg(0, "on|off")
		if err != nil {
			return "", err
		}
		// The knob is phrased as a disable, so invert here not in every caller.
		disable := "true"
		if state == "on" {
			disable = "false"
		}
		_, err = ctl("keyword", "misc:disable_autoreload", disable)
		return "", err

	case ActionCursorSet:
		theme, err := arg(0, "cursor theme")
		if err != nil {
			return "", err
		}
		size, err := arg(1, "cursor size")
		if err != nil {
			return "", err
		}
		_, err = ctl("setcursor", theme, size)
		return "", err

	case ActionCursorReassert:
		o := loadStore(DesktopStorePath())
		_, err := ctl("setcursor", o.Cursor.Theme, strconv.Itoa(o.Cursor.Size))
		return "", err

	case ActionSubmapEnter:
		name, err := arg(0, "submap name")
		if err != nil {
			return "", err
		}
		_, err = ctl("dispatch", "submap", name)
		return "", err

	case ActionSubmapReset:
		_, err := ctl("dispatch", "submap", "reset")
		return "", err

	case ActionKeyboardCycleLayout:
		// A top-level hyprctl command, not a keyword. "all", so a split or
		// external board cannot drift from the rest.
		_, err := ctl("switchxkblayout", "all", "next")
		return "", err

	case ActionOutputPower:
		state, err := arg(0, "on|off")
		if err != nil {
			return "", err
		}
		if state != "on" && state != "off" {
			return "", fmt.Errorf("act %s: expected on or off, got %q", a, state)
		}
		_, err = ctl("dispatch", "dpms", state)
		return "", err
	}
	return "", fmt.Errorf("%w: %s", ErrUnsupported, a)
}
