package main

import (
	"testing"

	"zenith/desktop/internal/wm"
)

// shouldTakeOver decides whether a starting daemon displaces the incumbent on
// the control socket; a wrong call either strands the shell on a dead compositor
// (fails to take over a stale daemon) or kills a healthy same-session one.
func TestShouldTakeOver(t *testing.T) {
	cases := []struct {
		name          string
		mySig, incSig string
		ok            bool
		want          bool
	}{
		{"stale incumbent from a dead instance", "live", "dead", true, true},
		{"stale incumbent reporting no signature", "live", "", true, true},
		{"same-session double start", "live", "live", true, false},
		{"incumbent too old to answer", "live", "", false, false},
		{"we have no session to claim", "", "dead", true, false},
	}
	for _, c := range cases {
		if got := shouldTakeOver(c.mySig, c.incSig, c.ok); got != c.want {
			t.Errorf("%s: shouldTakeOver(%q, %q, %v) = %v, want %v", c.name, c.mySig, c.incSig, c.ok, got, c.want)
		}
	}
}

// The signature verb reports the provider's opaque instance handle. With no
// provider resolvable it is empty, which reads as no session to be stale against.
func TestSignatureCommandNoProvider(t *testing.T) {
	d := &daemon{wmc: wm.OpenNamed("does-not-exist")}
	if got := d.dispatch("signature"); got != "" {
		t.Fatalf("dispatch(signature) with no provider = %q, want empty", got)
	}
}
