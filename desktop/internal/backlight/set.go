package backlight

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
)

// ErrUnavailable is returned by a writer that cannot even try: the tool is not
// installed, the bus is not there, the file is not writable by this user. It is
// the signal to move on to the next route, as opposed to a route that tried and
// was refused.
var ErrUnavailable = errors.New("unavailable")

// A Writer is one route to changing a device's raw value.
type Writer interface {
	Name() string
	Write(ctx context.Context, d Device, raw int) error
}

// Chain tries each writer in order and stops at the first that succeeds.
type Chain []Writer

// Write returns the name of the route that worked. When none does, the error
// lists every route and why it failed, because "permission denied" with no
// route attached is exactly the message that cost this the hours it took to find.
func (c Chain) Write(ctx context.Context, d Device, raw int) (string, error) {
	var failures []string
	for _, w := range c {
		err := w.Write(ctx, d, raw)
		if err == nil {
			return w.Name(), nil
		}
		failures = append(failures, w.Name()+": "+err.Error())
	}
	if len(failures) == 0 {
		return "", errors.New("no way to write a brightness is configured")
	}
	return "", errors.New(strings.Join(failures, "; "))
}

// DefaultChain is the order the user's session should try: logind first, which
// needs no privileges for the session's owner; brightnessctl, which covers a
// build that has logind support of its own or a user who is in video; and the
// sysfs file itself, only if it is writable.
func DefaultChain() Chain {
	return Chain{Logind{}, Brightnessctl{}, Direct{}}
}

// logindDest is the bus name, and logindSession the object a caller reaches its
// own session through. "auto" is the caller's session and, for a process that
// is not in one, the user's graphical session, which is what a systemd --user
// service such as the shell daemon needs.
const (
	logindDest    = "org.freedesktop.login1"
	logindSession = dbus.ObjectPath("/org/freedesktop/login1/session/auto")
	logindMethod  = "org.freedesktop.login1.Session.SetBrightness"
	logindTimeout = 3 * time.Second
)

// Bus is the one call Logind makes, so a test can stand in for the system bus.
type Bus interface {
	Call(ctx context.Context, dest string, path dbus.ObjectPath, method string, args ...any) error
	Close() error
}

type systemBus struct{ conn *dbus.Conn }

func (b systemBus) Call(ctx context.Context, dest string, path dbus.ObjectPath, method string, args ...any) error {
	return b.conn.Object(dest, path).CallWithContext(ctx, method, 0, args...).Err
}

func (b systemBus) Close() error { return b.conn.Close() }

// Logind writes through systemd-logind: Session.SetBrightness(subsystem, name,
// value). logind checks that the caller owns the session and that the device is
// on the session's seat, then does the write as root. Nothing is installed and
// nobody joins a group, which is the point.
type Logind struct {
	// Dial opens the bus. Nil means the real system bus.
	Dial func() (Bus, error)
}

func (Logind) Name() string { return "logind" }

func (l Logind) Write(ctx context.Context, d Device, raw int) error {
	dial := l.Dial
	if dial == nil {
		dial = func() (Bus, error) {
			conn, err := dbus.ConnectSystemBus()
			if err != nil {
				return nil, err
			}
			return systemBus{conn}, nil
		}
	}
	bus, err := dial()
	if err != nil {
		return fmt.Errorf("%w: no system bus (%v)", ErrUnavailable, err)
	}
	defer bus.Close()

	ctx, cancel := context.WithTimeout(ctx, logindTimeout)
	defer cancel()
	if raw < 0 {
		raw = 0
	}
	err = bus.Call(ctx, logindDest, logindSession, logindMethod, d.Class, d.Name, uint32(raw))
	return explainLogind(err)
}

// explainLogind turns the handful of errors logind produces into a sentence that
// says what to do, and keeps the rest verbatim.
func explainLogind(err error) error {
	if err == nil {
		return nil
	}
	var dbusErr dbus.Error
	if errors.As(err, &dbusErr) {
		switch dbusErr.Name {
		case "org.freedesktop.DBus.Error.ServiceUnknown", "org.freedesktop.DBus.Error.NameHasNoOwner":
			return fmt.Errorf("%w: systemd-logind is not running", ErrUnavailable)
		case "org.freedesktop.login1.NoSessionForPID", "org.freedesktop.login1.NoSuchSession":
			return fmt.Errorf("%w: this process is not in a login session", ErrUnavailable)
		case "org.freedesktop.DBus.Error.AccessDenied":
			return fmt.Errorf("logind refused (the session does not own the device's seat): %v", dbusErr.Body)
		}
	}
	return err
}

// Brightnessctl shells out to brightnessctl with an absolute raw value. On the
// archive's build this is the same sysfs write the direct route does, so it only
// helps when the user is in video or the binary carries its own logind support,
// but it costs nothing to try and it is what a user would try by hand.
type Brightnessctl struct{}

func (Brightnessctl) Name() string { return "brightnessctl" }

func (Brightnessctl) Write(ctx context.Context, d Device, raw int) error {
	bin, err := exec.LookPath("brightnessctl")
	if err != nil {
		return fmt.Errorf("%w: brightnessctl is not installed", ErrUnavailable)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "-q", "-c", d.Class, "-d", d.Name, "set", strconv.Itoa(raw)).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	return nil
}

// Direct is the last resort: the sysfs attribute, if this process may write it.
type Direct struct{}

func (Direct) Name() string { return "sysfs" }

func (Direct) Write(_ context.Context, d Device, raw int) error {
	path := d.File()
	if err := unix.Access(path, unix.W_OK); err != nil {
		return fmt.Errorf("%w: %s is not writable by this user", ErrUnavailable, path)
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(strconv.Itoa(raw)); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// WritableBy names the route a write to this device would take right now, or ""
// when none is expected to work. It makes no change: logind is probed by asking
// it to name our session, which succeeds exactly when SetBrightness has a
// session to act for. It is a prediction for `zenith brightness list` and
// `zenith hw status`; the write is the authority.
func WritableBy(d Device, c Chain) string {
	for _, w := range c {
		switch w := w.(type) {
		case Logind:
			if logindReachable(w) {
				return w.Name()
			}
		case Brightnessctl:
			if _, err := exec.LookPath("brightnessctl"); err == nil && unix.Access(d.File(), unix.W_OK) == nil {
				return w.Name()
			}
		case Direct:
			if unix.Access(d.File(), unix.W_OK) == nil {
				return w.Name()
			}
		}
	}
	return ""
}

func logindReachable(l Logind) bool {
	if l.Dial != nil {
		// A caller that injected a bus is testing; assume it can reach logind.
		return true
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return false
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), logindTimeout)
	defer cancel()
	var active dbus.Variant
	call := conn.Object(logindDest, logindSession).CallWithContext(ctx,
		"org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Active")
	return call.Store(&active) == nil
}
