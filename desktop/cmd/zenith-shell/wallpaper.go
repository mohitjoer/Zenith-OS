package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The depth-cutout policy lives here in the Go daemon (per-wall registry,
// engine runs, the depth IPC verbs), but the wallpaper surface itself lives in
// zenith-wallpaper, the Rust wallpaper daemon. This bridge keeps the two honest: the
// daemon mirrors zenith-wallpaper's published wallpaper frame to know what is on screen,
// and hands finished cutouts back over `depth set` / `depth clear`, which
// zenith-wallpaper folds into the frame the shell's QML renders.

// wallpaperFrameEntry is the slice of zenith-wallpaper's wallpaper frame the depth worker
// needs: the source path and whether a live player owns the slot.
type wallpaperFrameEntry struct {
	Path  string `json:"path"`
	Live  bool   `json:"live"`
	Video bool   `json:"video"`
	Depth string `json:"depth"`
}

// wallpaperFrame mirrors zenith-wallpaper's `{default, outputs}` wallpaper topic frame.
type wallpaperFrame struct {
	Default wallpaperFrameEntry            `json:"default"`
	Outputs map[string]wallpaperFrameEntry `json:"outputs"`
}

func wallpaperSock() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, "zenith-wallpaper.sock")
}

// wallpaperCmd sends one command line to zenith-wallpaper and returns its single reply
// line. Timeouts are short: zenith-wallpaper answers surface commands from memory, and a
// hung daemon must never wedge the depth worker.
func wallpaperCmd(line string) (string, error) {
	conn, err := net.DialTimeout("unix", wallpaperSock(), 2*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(conn, line+"\n"); err != nil {
		return "", err
	}
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && reply == "" {
		return "", err
	}
	reply = strings.TrimSpace(reply)
	if strings.HasPrefix(reply, "err") {
		return "", fmt.Errorf("zenith-wallpaper: %s", reply)
	}
	return reply, nil
}

// currentWall is the wallpaper the depth registry keys off: the default
// (broadcast) wallpaper of the last frame zenith-wallpaper published.
func (d *daemon) currentWall() string {
	d.wallMu.Lock()
	defer d.wallMu.Unlock()
	return d.wall.Default.Path
}

// currentWallVideo reports whether that default frame is a video's still:
// zenith-wallpaper paints a clip's extracted frame with a `video` marker, and depth
// must treat it like the video it stands for, not as an image.
func (d *daemon) currentWallVideo() bool {
	d.wallMu.Lock()
	defer d.wallMu.Unlock()
	return d.wall.Default.Video
}

// wallFrame returns a copy of the last frame seen from zenith-wallpaper.
func (d *daemon) wallFrame() wallpaperFrame {
	d.wallMu.Lock()
	defer d.wallMu.Unlock()
	f := d.wall
	f.Outputs = make(map[string]wallpaperFrameEntry, len(d.wall.Outputs))
	for k, v := range d.wall.Outputs {
		f.Outputs[k] = v
	}
	return f
}

// depthPublish hands a finished cutout to zenith-wallpaper. Ryogami validates the source
// is still the one on screen (a switch mid-generation drops the stale cut) and
// republishes the wallpaper frame with the depth fields set. The revision is
// the cutout's mtime, so a regenerated file at the same path still busts the
// image cache.
func (d *daemon) depthPublish(slot, source, out string) {
	req, _ := json.Marshal(struct {
		Screen string `json:"screen"`
		Source string `json:"source"`
		Out    string `json:"out"`
		Rev    int64  `json:"rev"`
	}{slot, source, out, fileModTime(out)})
	if _, err := wallpaperCmd("depth set " + string(req)); err != nil {
		fmt.Fprintf(os.Stderr, "depthPublish: %v\n", err)
	}
}

// depthClear drops every slot's cutout from zenith-wallpaper's frame.
func (d *daemon) depthClear() {
	if _, err := wallpaperCmd("depth clear"); err != nil {
		fmt.Fprintf(os.Stderr, "depthClear: %v\n", err)
	}
}

// sameWallSources reports whether two frames show the same wallpapers, ignoring
// revisions and depth: those change on our own publishes and must not wake the
// worker again.
func sameWallSources(a, b wallpaperFrame) bool {
	if a.Default.Path != b.Default.Path || a.Default.Live != b.Default.Live {
		return false
	}
	if len(a.Outputs) != len(b.Outputs) {
		return false
	}
	for name, ae := range a.Outputs {
		be, ok := b.Outputs[name]
		if !ok || ae.Path != be.Path || ae.Live != be.Live {
			return false
		}
	}
	return true
}

// missingDepth reports whether some still slot shows a wallpaper without a
// cutout. A re-set of the same path clears the frame's depth fields, so this is
// what re-arms the worker when the sources alone look unchanged.
func missingDepth(f wallpaperFrame) bool {
	if f.Default.Path != "" && !f.Default.Live && f.Default.Depth == "" {
		return true
	}
	for _, e := range f.Outputs {
		if e.Path != "" && !e.Live && e.Depth == "" {
			return true
		}
	}
	return false
}

// consumeWallpaperFrames mirrors zenith-wallpaper's wallpaper topic into d.wall and
// wakes the workers: a changed picture reschedules the stage cutout AND the
// palette pass (the dynamic matugen pipeline follows the wallpaper), while a
// frame that merely lost its subject fold re-arms the stage worker alone. That
// worker only reuses or clears on such a wake (it never auto-recuts), so a
// spurious wake settles immediately: an unchanged publish is suppressed by
// zenith-wallpaper's topic and the chain goes quiet.
func (d *daemon) consumeWallpaperFrames(r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		var f wallpaperFrame
		if err := json.Unmarshal(sc.Bytes(), &f); err != nil {
			continue
		}
		d.wallMu.Lock()
		srcChanged := !sameWallSources(d.wall, f)
		wakeDepth := srcChanged || missingDepth(f)
		d.wall = f
		d.wallMu.Unlock()
		if wakeDepth {
			d.scheduleStage()
		}
		if srcChanged {
			d.scheduleTheme()
		}
	}
}

// watchWallpaper subscribes to zenith-wallpaper's wallpaper topic for the daemon's life,
// reconnecting with a small backoff (zenith-wallpaper restarts independently). The
// retained frame arrives on every connect, so the cache reseeds itself and the
// worker re-publishes cutouts a restarted zenith-wallpaper lost.
func (d *daemon) watchWallpaper() {
	const minBackoff = 150 * time.Millisecond
	const maxBackoff = 5 * time.Second
	backoff := minBackoff
	for {
		select {
		case <-d.quit:
			return
		default:
		}

		conn, err := net.Dial("unix", wallpaperSock())
		if err != nil {
			select {
			case <-d.quit:
				return
			case <-time.After(backoff):
			}
			backoff = capDur(backoff*2, maxBackoff)
			continue
		}
		if _, err := io.WriteString(conn, "subscribe wallpaper\n"); err != nil {
			conn.Close()
			continue
		}

		backoff = minBackoff
		done := make(chan struct{})
		go func() {
			select {
			case <-d.quit:
				conn.Close()
			case <-done:
			}
		}()
		d.consumeWallpaperFrames(conn)
		close(done)
		conn.Close()
	}
}
