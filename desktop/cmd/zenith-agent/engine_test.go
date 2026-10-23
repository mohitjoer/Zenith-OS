package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// engineEnv points the config at a temp dir and returns the config file's path.
func engineEnv(t *testing.T) string {
	t.Helper()
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(h, ".local", "state"))
	return ConfigPath()
}

func writeConfig(t *testing.T, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ConfigPath(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// With no engine, every surface gets the same plain sentence, and it names the key
// and the file. Nothing is started.
func TestNoEngineSaysSoPlainly(t *testing.T) {
	engineEnv(t)
	st := engineStatus(LoadConfig())
	if st.Configured || st.Available {
		t.Fatalf("a fresh config has no engine: %+v", st)
	}
	for _, want := range []string{"no agent engine configured", engineKey, "config.json"} {
		if !strings.Contains(st.Reason, want) {
			t.Fatalf("reason %q is missing %q", st.Reason, want)
		}
	}
	if _, err := startACP(t.TempDir()); err == nil || err.Error() != st.Reason {
		t.Fatalf("startACP without an engine must fail with the same sentence, got %v", err)
	}
	if got := BuildStatus(LoadConfig()); got.Ready || got.Engine.Reason != st.Reason {
		t.Fatalf("status must not be ready and must carry the reason: %+v", got)
	}
	if got := chatBackendInfos(LoadConfig()); got == nil || len(got) != 0 {
		t.Fatalf("agent --json lists no engine as [], got %#v", got)
	}
}

func TestEngineProgramMustExist(t *testing.T) {
	engineEnv(t)
	writeConfig(t, `{"engine":{"command":["/no/such/engine","acp"]}}`)
	st := engineStatus(LoadConfig())
	if !st.Configured || st.Available || !strings.Contains(st.Reason, "/no/such/engine") || !strings.Contains(st.Reason, engineKey) {
		t.Fatalf("a missing program must be named with the key to fix: %+v", st)
	}
}

func TestEngineConfiguredIsAvailable(t *testing.T) {
	engineEnv(t)
	writeConfig(t, `{"engine":{"command":["sh","-c","exit 0"]}}`)
	cfg := LoadConfig()
	st := engineStatus(cfg)
	if !st.Available || st.Reason != "" || strings.Join(st.Command, " ") != "sh -c exit 0" {
		t.Fatalf("status %+v", st)
	}
	infos := chatBackendInfos(cfg)
	if len(infos) != 1 || infos[0].ID != engineID || !infos[0].Available || !infos[0].Active || infos[0].Name != "sh" {
		t.Fatalf("agent --json = %+v", infos)
	}
	if !BuildStatus(cfg).Ready {
		t.Fatal("a usable engine makes the chat ready")
	}
}

// The config file is edited by hand. A typo must be reported as one, and saving
// over it must refuse rather than replace the user's edit with defaults.
func TestBrokenConfigIsReportedAndNotOverwritten(t *testing.T) {
	engineEnv(t)
	writeConfig(t, `{"engine": {"command": ["x"`)
	st := engineStatus(LoadConfig())
	if !strings.Contains(st.Reason, "cannot read") || !strings.Contains(st.Reason, "config.json") {
		t.Fatalf("reason %q", st.Reason)
	}
	if err := SaveConfig(LoadConfig()); err == nil {
		t.Fatal("SaveConfig overwrote a config it could not parse")
	}
	if b, _ := os.ReadFile(ConfigPath()); !strings.Contains(string(b), `"command": ["x"`) {
		t.Fatal("the user's file was changed")
	}
}

// `agent use` has one engine to name; anything else says how to change it.
func TestAgentUseKnowsOneEngine(t *testing.T) {
	engineEnv(t)
	if err := cmdAgent([]string{"use", "other"}); err == nil || !strings.Contains(err.Error(), engineKey) {
		t.Fatalf("agent use other: %v", err)
	}
	if err := cmdAgent([]string{"use", engineID}); err != nil {
		t.Fatalf("agent use %s: %v", engineID, err)
	}
}

// A session started under one command is stale once the config names another.
func TestSessionIsStaleWhenTheCommandChanges(t *testing.T) {
	engineEnv(t)
	writeConfig(t, `{"engine":{"command":["sh","-c","exit 0"]}}`)
	c := &acpConn{configStamp: engineStamp(LoadConfig())}
	if c.stale() {
		t.Fatal("a session on the current command is not stale")
	}
	writeConfig(t, `{"engine":{"command":["sh","-c","exit 1"]}}`)
	if !c.stale() {
		t.Fatal("changing engine.command must make the running session stale")
	}
}

// The model picked in the sidebar is remembered per engine command: a pick made
// under one engine is not applied to another.
func TestRememberedModelIsPerEngine(t *testing.T) {
	engineEnv(t)
	writeConfig(t, `{"engine":{"command":["sh","-c","exit 0"]}}`)
	saveSessionModel("big-model")
	if got := savedSessionModel(); got != "big-model" {
		t.Fatalf("saved = %q", got)
	}
	writeConfig(t, `{"engine":{"command":["sh","-c","exit 1"]}}`)
	if got := savedSessionModel(); got != "" {
		t.Fatalf("a pick from another engine leaked: %q", got)
	}
}

// A dead engine says why: its last stderr line rides the dead state, so a
// misconfigured command is diagnosable without a debugger.
func TestDeadEngineCarriesItsLastWords(t *testing.T) {
	engineEnv(t)
	writeConfig(t, `{"engine":{"command":["sh","-c","echo engine says boom >&2"]}}`)
	c, err := startACP(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev, ok := <-c.Events():
			if !ok {
				t.Fatal("events closed without a dead state")
			}
			if ev.Type == "state" && ev.State == "dead" {
				if ev.Err != "engine says boom" {
					t.Fatalf("dead state carried %q", ev.Err)
				}
				return
			}
		case <-deadline:
			t.Fatal("no dead state")
		}
	}
}

// A user message sent with no engine is answered, not swallowed: the sender gets
// busy and then dead with the reason, which is what the CLI turns into its error.
func TestHubAnswersAUserMessageWithoutAnEngine(t *testing.T) {
	engineEnv(t)
	h := newChatHub()
	h.mu.Lock()
	h.ensureConnLocked()
	why := h.last
	h.mu.Unlock()
	if why.State != "dead" || !strings.Contains(why.Error, engineKey) {
		t.Fatalf("hub state without an engine: %+v", why)
	}
}

// Swapping engine.command replaces the session, and the model and command lists
// of the engine that left must not greet the next joiner, even when the new engine
// is missing and nothing replaces them.
func TestHubForgetsTheOldEnginesLists(t *testing.T) {
	engineEnv(t)
	h := newChatHub()
	cl := &chatClient{out: make(chan wsOut, 4)}
	h.clients[cl] = true
	h.conn = &acpConn{configStamp: "the engine that left"}
	h.models = wsOut{Type: "models", Agent: "old", Current: "old-model"}
	h.commands = wsOut{Type: "commands"}
	h.mu.Lock()
	h.ensureConnLocked()
	h.mu.Unlock()
	if h.models.Type != "" || h.commands.Type != "" {
		t.Fatalf("lists of the old engine survived: %+v %+v", h.models, h.commands)
	}
	if got := <-cl.out; got.Type != "models" || got.Agent != "" || len(got.Models) != 0 {
		t.Fatalf("open clients must be told to clear their model chip, got %+v", got)
	}
}

// The paste snippet is the whole point of "point any agent at Zenith": it must
// name the vault, prowl, and the skill so an unsupported agent gets the
// same power. A regression that drops one silently weakens every such agent.
func TestManifestSnippet(t *testing.T) {
	h := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(h, "data"))
	skill := filepath.Join(h, "skills", "zenith")
	snip := manifestSnippet(skill)
	for _, want := range []string{"prowl", "AGENTS.md", filepath.Join(skill, "SKILL.md"), binName} {
		if !strings.Contains(snip, want) {
			t.Fatalf("snippet missing %q:\n%s", want, snip)
		}
	}
}

// vaultManifest lists the read-order docs; the index (AGENTS.md) must lead so an
// agent reads the map before anything else.
func TestVaultManifestOrder(t *testing.T) {
	items := vaultManifest()
	if len(items) == 0 || items[0].Label != "AGENTS.md" {
		t.Fatalf("vault manifest must lead with AGENTS.md, got %+v", items)
	}
	var haveDesktop bool
	for _, it := range items {
		if it.Label == "desktop.md" {
			haveDesktop = true
		}
	}
	if !haveDesktop {
		t.Fatal("vault manifest missing desktop.md")
	}
}

// The manifest carries the engine, and the whole thing marshals.
func TestManifestCarriesTheEngine(t *testing.T) {
	engineEnv(t)
	b, err := json.Marshal(BuildManifest(LoadConfig()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"engine":{"configured":false`) {
		t.Fatalf("manifest has no engine block: %s", b)
	}
}
