package main

import "testing"

func TestParseWindowManager(t *testing.T) {
	cases := map[string]string{
		"Provider: hyprland\nLive: true\n": "hyprland",
		"Provider: (none)\n":               "",
		"Capabilities: unavailable\n":      "",
		"Provider: niri\n":                 "niri",
	}
	for in, want := range cases {
		if got := parseWindowManager(in); got != want {
			t.Fatalf("parseWindowManager(%q) = %q, want %q", in, got, want)
		}
	}
}
