package wm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

// Client resolves the provider once and speaks the verbs to it. Shared so the
// contract lives in one place: five hand-rolled callers would be five copies,
// and the first schema change would leave four wrong.

// ErrNoProvider is not fatal. The Hub must still open on a box whose compositor
// tooling is missing, so callers treat it as "no compositor features".
var ErrNoProvider = errors.New("no window manager provider available")

// ErrUnsupported is a different message to a user than ErrNoProvider: the
// provider is there, the compositor just cannot do that.
var ErrUnsupported = errors.New("window manager does not support this action")

// provider is the four verbs a compositor implements. Hyprland is built in;
// execProvider speaks the same four to a `zenith-wm-<name>` executable.
type provider interface {
	available() bool
	caps(ctx context.Context) (Caps, error)
	act(ctx context.Context, a Action, args []string) (string, error)
	state(ctx context.Context) (Snapshot, error)
	watch(ctx context.Context, kinds []FrameKind, onFrame func(Frame)) error
}

// Safe for concurrent use. A successful Caps probe is cached for the process
// lifetime: a compositor does not gain features while running, and the keybind
// path cannot afford a fork to re-ask. A failed probe is NOT cached, so a daemon
// that asked before the provider was answering re-probes on the next call rather
// than running its whole life believing the compositor has no features.
type Client struct {
	detection Detection
	prov      provider

	capsMu sync.Mutex
	capsOK bool
	caps   Caps
}

// Open resolves the active provider without running it, so constructing one in
// a daemon's init never blocks on a compositor that is still coming up.
func Open() *Client {
	d := Detect()
	return &Client{detection: d, prov: providerFor(d.Name)}
}

// OpenNamed targets a provider the caller is not necessarily in: a test, or a
// tool previewing another compositor.
func OpenNamed(name string) *Client {
	return &Client{
		detection: Detection{Name: name, Live: false, Source: "explicit"},
		prov:      providerFor(name),
	}
}

func providerFor(name string) provider {
	switch name {
	case "":
		return nil
	case ProviderHyprland:
		return hyprland{}
	}
	return execProvider{bin: "zenith-wm-" + name}
}

func (c *Client) Detection() Detection { return c.detection }

func (c *Client) Available() bool {
	return c.prov != nil && c.prov.available()
}

// Caps probes until it succeeds once, then serves the cached answer. A failed
// probe returns the error and leaves the cache cold, so the next call probes
// again: the provider may still be coming up. A zero Caps means every Has reads
// false rather than assuming Hyprland's feature set.
func (c *Client) Caps() (Caps, error) {
	return c.CapsContext(context.Background())
}

// CapsContext is Caps with cancellation for callers on latency-sensitive paths
// such as display recovery after resume.
func (c *Client) CapsContext(ctx context.Context) (Caps, error) {
	c.capsMu.Lock()
	defer c.capsMu.Unlock()
	if c.capsOK {
		return c.caps, nil
	}
	if c.prov == nil {
		return c.caps, ErrNoProvider
	}
	caps, err := c.prov.caps(ctx)
	if err != nil {
		return c.caps, err
	}
	c.caps, c.capsOK = caps, true
	return c.caps, nil
}

// Can is the gate before offering an affordance. False on a missing provider
// too, which is what a caller wants.
func (c *Client) Can(want Capability) bool {
	return c.CanContext(context.Background(), want)
}

// CanContext is Can with cancellation propagated to an uncached capability
// probe.
func (c *Client) CanContext(ctx context.Context, want Capability) bool {
	caps, err := c.CapsContext(ctx)
	if err != nil {
		return false
	}
	return caps.Has(want)
}

// Act checks the capability first, so an unsupported action is a typed error
// rather than a call that exits zero having done nothing.
func (c *Client) Act(a Action, args ...string) error {
	return c.ActContext(context.Background(), a, args...)
}

// ActContext bounds both the capability probe and the provider action.
func (c *Client) ActContext(ctx context.Context, a Action, args ...string) error {
	_, err := c.ActOutputContext(ctx, a, args...)
	return err
}

// ActOutput is Act for actions that answer with a value.
func (c *Client) ActOutput(a Action, args ...string) (string, error) {
	return c.ActOutputContext(context.Background(), a, args...)
}

// ActOutputContext bounds both the capability probe and the provider action.
func (c *Client) ActOutputContext(ctx context.Context, a Action, args ...string) (string, error) {
	if c.prov == nil {
		return "", ErrNoProvider
	}
	if need := a.Capability(); need != "" {
		caps, err := c.CapsContext(ctx)
		if err != nil {
			return "", err
		}
		if !caps.Has(need) {
			return "", fmt.Errorf("%w: %s", ErrUnsupported, a)
		}
	}
	out, err := c.prov.act(ctx, a, args)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// State is one full read, for callers that ask a single question and exit.
func (c *Client) State() (Snapshot, error) {
	return c.StateContext(context.Background())
}

// StateContext is State with a caller-owned deadline.
func (c *Client) StateContext(ctx context.Context) (Snapshot, error) {
	if c.prov == nil {
		return Snapshot{}, ErrNoProvider
	}
	return c.prov.state(ctx)
}

// Watch streams frames until ctx is done or the provider gives up. onFrame runs
// on the reader goroutine, so a consumer doing real work should hand off.
//
// The caller owns restart policy: a compositor restart must not leave a
// permanently cold cache.
func (c *Client) Watch(ctx context.Context, onFrame func(Frame)) error {
	return c.WatchKinds(ctx, nil, onFrame)
}

// WatchKinds is Watch limited to the frame kinds the caller uses. Narrowing
// matters: the provider only reads what it has to emit, so a consumer that
// wants outputs does not make the compositor answer for windows on every
// window event.
func (c *Client) WatchKinds(ctx context.Context, kinds []FrameKind, onFrame func(Frame)) error {
	if c.prov == nil {
		return ErrNoProvider
	}
	return c.prov.watch(ctx, kinds, onFrame)
}

// execProvider speaks the provider verbs to an executable: `caps`, `act <id>
// [args]`, `state` and `watch [kind...]`, each answering in JSON on stdout (act
// answers with whatever value it has to report). Nothing ships one today; it is
// the seam a second compositor plugs into, and the tests drive the Client's
// caching and cancellation through it.
type execProvider struct{ bin string }

func (p execProvider) available() bool {
	_, err := exec.LookPath(p.bin)
	return err == nil
}

func (p execProvider) caps(ctx context.Context) (Caps, error) {
	var caps Caps
	out, err := p.run(ctx, "caps")
	if err != nil {
		return caps, err
	}
	if err := json.Unmarshal(out, &caps); err != nil {
		return Caps{}, err
	}
	return caps, nil
}

func (p execProvider) act(ctx context.Context, a Action, args []string) (string, error) {
	out, err := p.run(ctx, append([]string{"act", string(a)}, args...)...)
	return string(out), err
}

func (p execProvider) state(ctx context.Context) (Snapshot, error) {
	var snap Snapshot
	out, err := p.run(ctx, "state")
	if err != nil {
		return snap, err
	}
	if err := json.Unmarshal(out, &snap); err != nil {
		return snap, fmt.Errorf("%s state: %w", p.bin, err)
	}
	return snap, nil
}

func (p execProvider) watch(ctx context.Context, kinds []FrameKind, onFrame func(Frame)) error {
	argv := []string{"watch"}
	for _, k := range kinds {
		argv = append(argv, string(k))
	}
	cmd := providerCommandContext(ctx, p.bin, argv...)
	// The command helper also kills the complete provider/IPC process group when
	// this watcher is cancelled, so a compositor query cannot outlive it.
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	scan := bufio.NewScanner(stdout)
	// Frames carry full lists; a busy session overruns the 64 KiB default and a
	// truncated frame would read as "windows closed".
	scan.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scan.Scan() {
		line := bytes.TrimSpace(scan.Bytes())
		if len(line) == 0 {
			continue
		}
		var f Frame
		if err := json.Unmarshal(line, &f); err != nil {
			// One bad frame goes stale for that update only; the next repairs it.
			continue
		}
		onFrame(f)
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return nil
	}
	if err := scan.Err(); err != nil {
		return err
	}
	return waitErr
}

func providerCommandContext(ctx context.Context, bin string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return cmd
}

// run folds stderr into the error so a provider diagnostic reaches the caller's
// log instead of being swallowed.
func (p execProvider) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := providerCommandContext(ctx, p.bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("%s %s: %w", p.bin, strings.Join(args, " "), err)
		}
		return nil, fmt.Errorf("%s %s: %w: %s", p.bin, strings.Join(args, " "), err, msg)
	}
	return stdout.Bytes(), nil
}
