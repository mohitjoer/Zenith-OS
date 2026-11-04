package main

import "testing"

// Ready gates the sidebar's first-run prompt: it is true only when an engine is
// configured and its program exists. A regression here would either nag a working
// box or leave a fresh user staring at a dead chat with no guidance.
func TestStatusReadyTracksTheEngine(t *testing.T) {
	engineEnv(t)

	if BuildStatus(LoadConfig()).Ready {
		t.Fatal("no engine configured must not be ready")
	}

	writeConfig(t, `{"engine":{"command":["/no/such/engine"]}}`)
	if st := BuildStatus(LoadConfig()); st.Ready || !st.Engine.Configured || st.Engine.Available {
		t.Fatalf("an engine whose program is missing must not be ready: %+v", st.Engine)
	}

	writeConfig(t, `{"engine":{"command":["sh","-c","exit 0"]}}`)
	if st := BuildStatus(LoadConfig()); !st.Ready || !st.Engine.Available || st.Engine.Reason != "" {
		t.Fatalf("a usable engine must be ready: %+v", st.Engine)
	}

	// A fast-lane provider alone does not make the chat ready: the chat needs the engine.
	writeConfig(t, `{"quick":{"provider":"ollama","model":"llama3.2"}}`)
	if BuildStatus(LoadConfig()).Ready {
		t.Fatal("a quick provider without an engine must not make the chat ready")
	}
}
