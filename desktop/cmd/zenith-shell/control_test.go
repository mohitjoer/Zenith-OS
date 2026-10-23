package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// parseBarEdge is the whole grammar of `bar <edge|all> <toggle|reveal|hide>`; a
// wrong edge or action must not reach the frame, and a missing one must not be
// silently accepted as a default.
func TestParseBarEdge(t *testing.T) {
	for _, edge := range []string{"top", "bottom", "left", "right", "all"} {
		for _, action := range []string{"toggle", "reveal", "hide"} {
			e, a, ok := parseBarEdge([]string{edge, action})
			if !ok || e != edge || a != action {
				t.Errorf("parseBarEdge(%s %s) = (%q,%q,%v), want (%q,%q,true)", edge, action, e, a, ok, edge, action)
			}
		}
	}
	for _, args := range [][]string{
		{},
		{"left"},
		{"left", "toggle", "extra"},
		{"sideways", "toggle"},
		{"left", "sideways"},
		{"toggle", "left"},
	} {
		if _, _, ok := parseBarEdge(args); ok {
			t.Errorf("parseBarEdge(%v) accepted, want rejection", args)
		}
	}
}

// The audio verb steps volume by the reference literal (5% of the [0,1] range),
// clamps the rise at 1.0, and toggles mute; those argv are the behaviour, so
// pin them.
func TestAudioArgv(t *testing.T) {
	cases := map[string][]string{
		"up":   {"set-volume", "-l", "1", "@DEFAULT_AUDIO_SINK@", "5%+"},
		"down": {"set-volume", "@DEFAULT_AUDIO_SINK@", "5%-"},
		"mute": {"set-mute", "@DEFAULT_AUDIO_SINK@", "toggle"},
	}
	for sub, want := range cases {
		got, ok := audioArgv(sub)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("audioArgv(%q) = (%v,%v), want (%v,true)", sub, got, ok, want)
		}
	}
	if _, ok := audioArgv("sideways"); ok {
		t.Errorf("audioArgv(sideways) accepted, want rejection")
	}
}

// The brightness verb steps by the reference literal (5 of the [0,100] range).
func TestBrightnessArgv(t *testing.T) {
	cases := map[string][]string{
		"up":   {"+5"},
		"down": {"-5"},
	}
	for sub, want := range cases {
		got, ok := brightnessArgv(sub)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("brightnessArgv(%q) = (%v,%v), want (%v,true)", sub, got, ok, want)
		}
	}
	if _, ok := brightnessArgv("sideways"); ok {
		t.Errorf("brightnessArgv(sideways) accepted, want rejection")
	}
}

// menuID accepts only `menu <id>` for an id in the current frame menu catalog;
// retired sidebar/power menu ids, the old bar-prefixed form, and unknown ids
// must miss.
func TestMenuID(t *testing.T) {
	if id, ok := menuID("menu quick-settings"); !ok || id != "quick-settings" {
		t.Errorf("menuID(menu quick-settings) = (%q,%v), want (quick-settings,true)", id, ok)
	}
	// #page suffix: the full id (base#page) is returned for QML's deep-link.
	if id, ok := menuID("menu quick-settings#clipboard"); !ok || id != "quick-settings#clipboard" {
		t.Errorf("menuID(menu quick-settings#clipboard) = (%q,%v), want (quick-settings#clipboard,true)", id, ok)
	}
	for _, cmd := range []string{"menu", "menu bogus", "menu clock", "menu clipboard", "menu recording", "menu system", "menu clock extra", "bar clock", "clipboard", "menu bogus#page"} {
		if _, ok := menuID(cmd); ok {
			t.Errorf("menuID(%q) accepted, want rejection", cmd)
		}
	}
}

// Every new control verb rejects a missing or malformed argument with a
// verb-scoped error rather than opening the wrong thing or a bare "unknown
// command"; these paths return before any IPC or subprocess.
func TestDispatchControlErrors(t *testing.T) {
	d := &daemon{}
	cases := map[string]string{
		"menu":                            "err menu",
		"menu bogus":                      "err menu",
		"bar":                             "err bar",
		"bar left":                        "err bar",
		"bar sideways toggle":             "err bar",
		"bar left sideways":               "err bar",
		"audio":                           "err audio",
		"audio sideways":                  "err audio",
		"brightness":                      "err brightness",
		"brightness sideways":             "err brightness",
		"hub":                             "err hub",
		"hub bogus":                       "err hub",
		"lock-quiesce now":                "err lock-quiesce",
		"sleep-ready":                     "err sleep-ready",
		"sleep-ready now":                 "err sleep-ready",
		"suspend":                         "err suspend",
		"suspend now":                     "err suspend",
		"suspend transaction":             "err suspend",
		"suspend-cancel":                  "err suspend-cancel",
		"suspend-cancel transaction":      "err suspend-cancel",
		"suspend-cancel transaction bad/": "err suspend-cancel",
	}
	for cmd, prefix := range cases {
		got := d.dispatch(cmd)
		if !strings.HasPrefix(got, prefix) {
			t.Errorf("dispatch(%q) = %q, want prefix %q", cmd, got, prefix)
		}
	}
}

// lock status is the reference check: it prints locked or unlocked and, per the
// exit-0-either-way contract, never fails. hyprlock holds the lock exactly as
// long as it lives, so its process is the whole state.
func TestLockStatus(t *testing.T) {
	old := lockProcessRunning
	defer func() { lockProcessRunning = old }()
	d := &daemon{}
	for running, want := range map[bool]string{false: "unlocked", true: "locked"} {
		lockProcessRunning = func() bool { return running }
		if got := d.dispatch("lock status"); got != want {
			t.Fatalf("dispatch(lock status) with hyprlock running=%v = %q, want %q", running, got, want)
		}
	}
}

// A lock with hyprlock already up starts nothing; one without starts it exactly
// once, and a missing hyprlock is an error naming it.
func TestLockStartsHyprlockOnce(t *testing.T) {
	oldRun, oldStart, oldSettle := lockProcessRunning, lockStarter, lockSettle
	defer func() { lockProcessRunning, lockStarter, lockSettle = oldRun, oldStart, oldSettle }()
	lockSettle = 0
	running, starts := false, 0
	lockProcessRunning = func() bool { return running }
	lockStarter = func() error { starts++; running = true; return nil }
	d := &daemon{}
	for i := 0; i < 2; i++ {
		if got := d.dispatch("lock"); got != "ok" {
			t.Fatalf("dispatch(lock) = %q, want ok", got)
		}
	}
	if starts != 1 {
		t.Fatalf("hyprlock started %d times, want 1", starts)
	}

	running = false
	lockStarter = func() error { return fmt.Errorf("hyprlock is not installed") }
	if got := d.dispatch("lock"); got != "err lock: hyprlock is not installed" {
		t.Fatalf("dispatch(lock) without hyprlock = %q", got)
	}
}

// The verbs kept from Ryoku's CLI that have no backend here say so by name.
func TestUnavailableVerbsSayWhy(t *testing.T) {
	d := &daemon{}
	for _, verb := range []string{"unlock-prepare", "lock-quiesce", "state launcher 1"} {
		got := d.dispatch(verb)
		if !strings.HasPrefix(got, "err "+strings.Fields(verb)[0]+": not available on Zenith: ") {
			t.Errorf("dispatch(%q) = %q", verb, got)
		}
	}
}
