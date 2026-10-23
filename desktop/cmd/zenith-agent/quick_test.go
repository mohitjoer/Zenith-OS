package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// With no quick block there is no fast lane: the target does not resolve, and the
// reason says asks go to the engine.
func TestResolveQuickTargetNeedsAConfigBlock(t *testing.T) {
	_, err := resolveQuickTarget(defaultConfig())
	if err == nil || !strings.Contains(err.Error(), "agent engine") {
		t.Fatalf("an empty quick block must not resolve, and must say asks use the engine: %v", err)
	}
}

func TestResolveQuickTargetRejectsOAuthBackends(t *testing.T) {
	cfg := defaultConfig()
	cfg.Quick.BaseURL = "https://chatgpt.com/backend-api/codex"
	cfg.Quick.Model = "gpt-5.5"
	if _, err := resolveQuickTarget(cfg); err == nil {
		t.Fatal("an OAuth-only backend must not resolve to a direct target")
	}
}

func TestResolveQuickTargetBaseURLNeedsNoProvider(t *testing.T) {
	cfg := defaultConfig()
	cfg.Quick.Model = "llama3.2"
	cfg.Quick.BaseURL = "http://127.0.0.1:11434/v1"
	tgt, err := resolveQuickTarget(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tgt.Model != "llama3.2" || !isLocalURL(tgt.BaseURL) || tgt.Label != "quick:llama3.2" {
		t.Fatalf("target %+v", tgt)
	}
}

// A provider preset resolves its endpoint, and its key comes from the daemon's
// environment.
func TestResolveQuickTargetProviderPreset(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-ds-test")
	cfg := defaultConfig()
	cfg.Quick.Provider = "deepseek"
	cfg.Quick.Model = "deepseek-chat"
	tgt, err := resolveQuickTarget(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tgt.BaseURL, "deepseek.com") {
		t.Fatalf("baseURL = %q, want deepseek", tgt.BaseURL)
	}
	if tgt.Key != "sk-ds-test" {
		t.Fatalf("key = %q, want sk-ds-test from the environment", tgt.Key)
	}
	if tgt.Label != "deepseek:deepseek-chat" {
		t.Fatalf("label = %q, want deepseek:deepseek-chat", tgt.Label)
	}
	t.Setenv("DEEPSEEK_API_KEY", "")
	if _, err := resolveQuickTarget(cfg); err == nil {
		t.Fatal("a hosted endpoint with no key must not resolve")
	}
}

// envValue reads the process environment and nothing else: a key file must never
// be a source of secrets.
func TestEnvValueReadsNoFiles(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, "cfg"))
	if err := os.MkdirAll(filepath.Join(h, "cfg", "zenith"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(h, "cfg", "zenith", "agent.env"), []byte("K=from-file\n"), 0o644)
	if got := envValue("K"); got != "" {
		t.Fatalf("a key file was read: got %q", got)
	}
	t.Setenv("K", "from-proc")
	if got := envValue("K"); got != "from-proc" {
		t.Fatalf("process env must win: got %q", got)
	}
	if got := envValue(""); got != "" {
		t.Fatalf("empty key must be empty: got %q", got)
	}
}

func sseServer(t *testing.T, deltas []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, d := range deltas {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", d)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func TestQuickCompleteStreamsAnswer(t *testing.T) {
	t.Setenv(envName("VAULT"), t.TempDir())
	srv := sseServer(t, []string{"The kernel ", "is 7.0.12."})
	defer srv.Close()
	var streamed strings.Builder
	got, err := quickComplete(context.Background(),
		quickTarget{BaseURL: srv.URL, Model: "m", Key: "k"},
		"kernel?", func(d string) { streamed.WriteString(d) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "The kernel is 7.0.12." || streamed.String() != got {
		t.Fatalf("answer %q streamed %q", got, streamed.String())
	}
}

func TestQuickCompleteSentinelEscalates(t *testing.T) {
	t.Setenv(envName("VAULT"), t.TempDir())
	srv := sseServer(t, []string{"TOOLS_", "REQUIRED"})
	defer srv.Close()
	streamedAny := false
	_, err := quickComplete(context.Background(),
		quickTarget{BaseURL: srv.URL, Model: "m", Key: "k"},
		"summarize this video", func(string) { streamedAny = true }, nil)
	if err != errNeedsTools {
		t.Fatalf("err = %v, want errNeedsTools", err)
	}
	if streamedAny {
		t.Fatal("sentinel must never stream to the transcript")
	}
}

func TestQuickCompleteToolLoop(t *testing.T) {
	t.Setenv(envName("VAULT"), t.TempDir())
	// First call: stream a tool_call. Second call: stream the answer.
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if calls == 0 {
			calls++
			fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"system_query","arguments":"{\"topic\":\"kernel\"}"}}]}}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"Kernel is current."}}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	var tools []string
	got, err := quickComplete(context.Background(),
		quickTarget{BaseURL: srv.URL, Model: "m", Key: "k"},
		"what kernel?", nil,
		func(id, title, status string) {
			if status == "in_progress" {
				tools = append(tools, title)
			}
		})
	if err != nil {
		t.Fatal(err)
	}
	if got != "Kernel is current." {
		t.Fatalf("answer %q", got)
	}
	if len(tools) != 1 || !strings.Contains(tools[0], "kernel") {
		t.Fatalf("tool events %v", tools)
	}
}
