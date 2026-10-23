package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// At login the wallpaper file (a late mount) or the outputs (a late monitor)
// can lag the daemon; a one-shot restore then left the grey default for the
// session. retryRestore covers a late file; a late monitor takes the retained
// frame from the shell's per-screen scope on its own.

const (
	restoreRetryWindow   = 30 * time.Second
	restoreRetryInterval = 1 * time.Second
)

func (d *daemon) retryRestore() {
	deadline := time.Now().Add(restoreRetryWindow)
	for time.Now().Before(deadline) {
		time.Sleep(restoreRetryInterval)
		if want, applied := d.restoreOutputs(); want == 0 || applied > 0 {
			return
		}
	}
	// The window covers a late mount or a finishing download; a choice still
	// missing after it is a dead path, and grey for the session is worse than
	// the default.
	d.restoreFallback()
}

// restoreFallback repaints the stored outputs against the default wallpaper and
// records it, so a dead choice heals at login instead of greying every time.
func (d *daemon) restoreFallback() {
	if want, applied := d.restoreOutputs(); want == 0 || applied > 0 {
		return // it landed as the window closed, or nothing was ever recorded
	}
	src := d.defaultWallpaper()
	if src == "" {
		return // nothing installed to fall back to
	}
	path := filepath.Join(d.config().cacheDir(), "outputs.json")
	state := map[string]map[string]interface{}{}
	loadJSON(path, &state)
	if len(state) == 0 {
		return
	}
	for _, e := range state {
		e["path"] = src
	}
	saveJSON(path, state)
	if _, applied := d.restoreOutputs(); applied > 0 {
		fmt.Fprintf(os.Stderr, "zenith-wallpaper: recorded wallpaper never arrived; painted %s\n",
			filepath.Base(src))
	}
}
