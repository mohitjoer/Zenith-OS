package wm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Client is the Hub's handle on the compositor. Ryoku ran a provider binary per
// compositor and spoke JSON to it; Zenith has the one compositor, so the
// provider is this package and a call is a function call. The method set is the
// seam's, unchanged, so the callers read the same either way.
//
// Safe for concurrent use. A successful Caps is cached for the process
// lifetime: a compositor does not gain features while running, and Hub verbs run
// once and exit.
type Client struct {
	detection Detection

	capsMu sync.Mutex
	caps   Caps
	capsOK bool
}

// Open resolves the active provider without touching it.
func Open() *Client { return &Client{detection: Detect()} }

// OpenNamed targets a compositor the caller is not necessarily in. Only
// Hyprland is known; any other name has no provider.
func OpenNamed(name string) *Client {
	return &Client{detection: Detection{Name: name, Live: false, Source: "explicit"}}
}

func (c *Client) Detection() Detection { return c.detection }

// Available is true when the named compositor is one this backend can author
// for. It does not claim the compositor is running: authoring config needs no
// session, which is what lets the Hub edit settings from a TTY or a chroot.
func (c *Client) Available() bool { return c.detection.Name == ProviderHyprland }

func (c *Client) Caps() (Caps, error) {
	return c.CapsContext(context.Background())
}

// CapsContext is Caps with the seam's signature; the probe is local and cannot
// block, so the context is only checked on entry.
func (c *Client) CapsContext(ctx context.Context) (Caps, error) {
	if err := ctx.Err(); err != nil {
		return Caps{}, err
	}
	c.capsMu.Lock()
	defer c.capsMu.Unlock()
	if c.capsOK {
		return c.caps, nil
	}
	if !c.Available() {
		return Caps{}, ErrNoProvider
	}
	c.caps, c.capsOK = hyprlandCaps(), true
	return c.caps, nil
}

// Can is the gate before offering an affordance.
func (c *Client) Can(want Capability) bool {
	caps, err := c.Caps()
	return err == nil && caps.Has(want)
}

// Act checks the capability first, so an unsupported action is a typed error
// rather than a call that succeeds having done nothing.
func (c *Client) Act(a Action, args ...string) error {
	_, err := c.ActOutput(a, args...)
	return err
}

func (c *Client) ActOutput(a Action, args ...string) (string, error) {
	if !c.Available() {
		return "", ErrNoProvider
	}
	if need := a.Capability(); need != "" {
		caps, err := c.Caps()
		if err != nil {
			return "", err
		}
		if !caps.Has(need) {
			return "", fmt.Errorf("%w: %s", ErrUnsupported, a)
		}
	}
	return runAct(a, args)
}

// Apply writes the compositor config from the neutral store at storePath.
func (c *Client) Apply(storePath string) (ApplyReport, error) {
	if !c.Available() {
		return ApplyReport{}, ErrNoProvider
	}
	return applyStore(storePath, false)
}

// DryRun asks what the compositor could not honour if the store were applied,
// and writes nothing.
func (c *Client) DryRun(storePath string) (ApplyReport, error) {
	if !c.Available() {
		return ApplyReport{}, ErrNoProvider
	}
	return ApplyReport{
		Provider:  ProviderHyprland,
		Unhonored: unhonored(storePath),
	}, nil
}

// Preview pushes the store to the live session without writing any config, for
// the Hub's appearance sliders.
func (c *Client) Preview(storePath string) (ApplyReport, error) {
	if !c.Available() {
		return ApplyReport{}, ErrNoProvider
	}
	return applyStore(storePath, true)
}

// State is one full read of what Wayland protocols do not carry.
func (c *Client) State() (Snapshot, error) {
	if !c.Available() {
		return Snapshot{}, ErrNoProvider
	}
	return readState()
}

// Defaults is the provider's default subtree of the neutral store, as JSON.
func (c *Client) Defaults() ([]byte, error) {
	if !c.Available() {
		return nil, ErrNoProvider
	}
	o := defaultOverrides()
	liveKbDefaults(&o.Input)
	o.Anim.Items, o.Anim.Curves = probeAnimations()
	ns, err := splitStore(o)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(ns, "", "  ")
}

// Schema is the provider's exclusive settings rows, each kept raw: the field set
// is the provider's to define and the Hub emits it back untouched.
func (c *Client) Schema() ([]json.RawMessage, error) {
	if !c.Available() {
		return nil, ErrNoProvider
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(schemaRows, &rows); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return rows, nil
}

// Binds is the effective bind legend.
func (c *Client) Binds() ([]BindRow, error) {
	if !c.Available() {
		return nil, ErrNoProvider
	}
	cat, err := LoadCatalogue()
	if err != nil {
		return nil, err
	}
	return buildBindRows(cat, loadStore(DesktopStorePath()), liveBinds()), nil
}

// Plugins runs one verb of the compositor-plugin subsystem and returns its JSON.
func (c *Client) Plugins(args ...string) ([]byte, error) {
	if !c.Available() {
		return nil, ErrNoProvider
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("plugins needs list|sync|rebuild|add|remove")
	}
	return runPlugins(args)
}

// ApplyOutputs persists an output layout and applies it live.
func (c *Client) ApplyOutputs(layoutPath string) (ApplyReport, error) {
	if !c.Available() {
		return ApplyReport{}, ErrNoProvider
	}
	if !c.Can(CapMonitorConfig) {
		return ApplyReport{}, fmt.Errorf("%w: output layout", ErrUnsupported)
	}
	return applyOutputsFile(layoutPath)
}

// compactJSON renders v on one line, the form every verb prints.
func compactJSON(v any) ([]byte, error) { return json.Marshal(v) }
