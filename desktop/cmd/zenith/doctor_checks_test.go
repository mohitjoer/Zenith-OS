package main

import (
	"reflect"
	"testing"

	"zenith/desktop/internal/runner/runnertest"
)

func TestZenithCommands(t *testing.T) {
	config := `
# bind = SUPER, X, exec, zenith-commented-out
bind = SUPER, Space, exec, zenith ipc call zenith togglePopover launcher
bind = SUPER, 1, exec, zenith-workspace focus 1
bind = SUPER, 2, exec, zenith-workspace focus 2
bindr = ALT, Tab, exec, [float] zenith-app terminal
bind = SUPER, A, exec, hyprctl dispatch togglefloating
bind = ,XF86AudioMute, exec, wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle
bind = SUPER, Q, killactive,
exec-once = command -v zenith-monitor >/dev/null 2>&1 && zenith-monitor autoscale
exec-once = /usr/bin/zenith-welcome --if-first-run
exec-once = /usr/share/zenith/session/zenith-activate
`
	want := []string{"zenith", "zenith-app", "zenith-welcome", "zenith-workspace"}
	if got := zenithCommands(config); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRecommendedAbsent(t *testing.T) {
	fake := runnertest.New().
		// zenith-desktop is not installed, so dpkg-query fails after printing.
		On("dpkg-query -W -f=${Recommends}\n zenith zenith-desktop", runnertest.Answer{
			Stdout: "hyprlock, grim (>= 1.4), foot | ghostty, wl-clipboard:any\n",
			Err:    runnertest.Exit(1, "no packages found matching zenith-desktop"),
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
