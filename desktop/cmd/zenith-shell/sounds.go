package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// sounds plays the five event cues the shell owns: a shutter on screenshot, a
// cue on output volume up/down/mute (input stays silent), a critical-battery
// alert, and AC plug/unplug. The clips are the desktop sound theme's own (Yaru,
// then freedesktop, both installed with the desktop), looked up at play time:
// Zenith does not ship audio of its own. Every failure mode is silent: a missing
// player or an absent theme drops the cue rather than disturbing the caller.
// There is no per-sound volume scaling; each clip plays at the sink's level.

const (
	soundShutter      = "shutter"
	soundVolumeChange = "volume-change"
	soundBatteryLow   = "battery-low"
	soundPowerPlug    = "power-plug"
	soundPowerUnplug  = "power-unplug"
)

// knownSound gates the `sound <event>` verb so an external caller (the zenith-shot
// screenshot config) can only trigger a real cue.
func knownSound(name string) bool {
	switch name {
	case soundShutter, soundVolumeChange, soundBatteryLow, soundPowerPlug, soundPowerUnplug:
		return true
	}
	return false
}

var (
	soundMu    sync.Mutex
	soundReady = map[string]string{} // event -> clip path
)

// soundThemes are searched in order; soundNames maps each cue to the theme
// names that can stand in for it, best first.
var (
	soundThemes = []string{"Yaru", "freedesktop"}
	soundNames  = map[string][]string{
		soundShutter:      {"camera-shutter", "screen-capture"},
		soundVolumeChange: {"audio-volume-change"},
		soundBatteryLow:   {"battery-low", "dialog-warning"},
		soundPowerPlug:    {"power-plug"},
		soundPowerUnplug:  {"power-unplug"},
	}
)

// soundFile resolves a cue to a clip in the sound theme and caches the answer.
func soundFile(name string) (string, bool) {
	soundMu.Lock()
	defer soundMu.Unlock()
	if p, ok := soundReady[name]; ok {
		return p, true
	}
	dirs := os.Getenv("XDG_DATA_DIRS")
	if dirs == "" {
		dirs = "/usr/local/share:/usr/share"
	}
	for _, theme := range soundThemes {
		for _, n := range soundNames[name] {
			for _, dir := range filepath.SplitList(dirs) {
				for _, ext := range []string{".oga", ".ogg", ".wav"} {
					p := filepath.Join(dir, "sounds", theme, "stereo", n+ext)
					if _, err := os.Stat(p); err == nil {
						soundReady[name] = p
						return p, true
					}
				}
			}
		}
	}
	return "", false
}

// soundPlayerArgv picks a PCM player for a clip path. PipeWire's pw-play is the
// session's own player (the same stack wpctl drives); paplay is the fallback.
func soundPlayerArgv(file string) []string {
	if p, err := exec.LookPath("pw-play"); err == nil {
		return []string{p, file}
	}
	if p, err := exec.LookPath("paplay"); err == nil {
		return []string{p, file}
	}
	return nil
}

// playSound fires an event cue without blocking the caller. The volume-change
// cue waits 50 ms first so the sink level has settled before it plays (matching
// the reference's pre-play pause). No cue applies any gain.
func playSound(name string) {
	go func() {
		if name == soundVolumeChange {
			time.Sleep(50 * time.Millisecond)
		}
		file, ok := soundFile(name)
		if !ok {
			return
		}
		argv := soundPlayerArgv(file)
		if argv == nil {
			return
		}
		_ = exec.Command(argv[0], argv[1:]...).Run()
	}()
}

// readTrim reads a sysfs attribute and trims it, returning "" on any error.
func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
