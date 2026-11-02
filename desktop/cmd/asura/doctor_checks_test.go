package main

import (
	"reflect"
	"testing"

	"asura/desktop/internal/runner/runnertest"
)

func TestAsuraCommands(t *testing.T) {
	config := `
# bind = SUPER, X, exec, asura-commented-out
bind = SUPER, Space, exec, asura ipc call asura togglePopover launcher
bind = SUPER, 1, exec, asura-workspace focus 1
bind = SUPER, 2, exec, asura-workspace focus 2
bindr = ALT, Tab, exec, [float] asura-app terminal
bind = SUPER, A, exec, hyprctl dispatch togglefloating
bind = ,XF86AudioMute, exec, wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle
bind = SUPER, Q, killactive,
exec-once = command -v asura-monitor >/dev/null 2>&1 && asura-monitor autoscale
exec-once = /usr/bin/asura-welcome --if-first-run
exec-once = /usr/share/asura/session/asura-activate
`
	want := []string{"asura", "asura-app", "asura-welcome", "asura-workspace"}
	if got := asuraCommands(config); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRecommendedAbsent(t *testing.T) {
	fake := runnertest.New().
		// asura-desktop is not installed, so dpkg-query fails after printing.
		On("dpkg-query -W -f=${Recommends}\n asura asura-desktop", runnertest.Answer{
			Stdout: "hyprlock, grim (>= 1.4), foot | ghostty, wl-clipboard:any\n",
			Err:    runnertest.Exit(1, "no packages found matching asura-desktop"),
		}).
		Out("dpkg-query -W -f=${Package} ${db:Status-Abbrev}",
			"hyprlock ii \ngrim rc \nghostty ii \nbash ii \n")

	absent, total, err := recommendedAbsent(fake.Run)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"grim", "wl-clipboard"}; !reflect.DeepEqual(absent, want) || total != 4 {
		t.Errorf("absent %v of %d, want %v of 4", absent, total, want)
	}
}
