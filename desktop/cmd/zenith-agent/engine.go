package main

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// engine.go is the seam for the agent engine. The engine is one program, named by
// one config key: engine.command in ~/.config/zenith/agent/config.json, an argv
// that speaks the Agent Client Protocol over stdio (acp.go). The daemon starts it
// in the vault directory the first time the chat is used and assumes nothing else
// about it. Nothing is installed, looked for on PATH, or started at boot, and with
// no command set every surface says so in the same words.

const (
	// engineKey is the config key that selects the engine.
	engineKey = "engine.command"
	// engineID is the one entry `agent --json` lists.
	engineID = "engine"
)

// noEngineMessage is what the chat, the launcher, the terminal and `status` all
// say when there is nothing to answer with.
func noEngineMessage() string {
	return "no agent engine configured: set " + engineKey + " in " + tildeAbbrev(ConfigPath())
}

// EngineStatus is the engine's state for `status --json`, the Hub page and the
// dashboard. Reason is the plain sentence to show when it cannot be used.
type EngineStatus struct {
	Configured bool     `json:"configured"`
	Command    []string `json:"command,omitempty"`
	Available  bool     `json:"available"`
	Key        string   `json:"key"`
	Config     string   `json:"config"`
	Reason     string   `json:"reason,omitempty"`
}

// engineArgv returns the command to run, or the plain reason it cannot be run.
func engineArgv(cfg Config) ([]string, error) {
	if msg := configProblem(); msg != "" {
		return nil, errors.New(msg)
	}
	cmd := cfg.Engine.Command
	if len(cmd) == 0 || strings.TrimSpace(cmd[0]) == "" {
		return nil, errors.New(noEngineMessage())
	}
	if _, err := exec.LookPath(cmd[0]); err != nil {
		return nil, fmt.Errorf("the agent engine %q was not found; fix %s in %s", cmd[0], engineKey, tildeAbbrev(ConfigPath()))
	}
	return cmd, nil
}

func engineStatus(cfg Config) EngineStatus {
	st := EngineStatus{Command: cfg.Engine.Command, Key: engineKey, Config: tildeAbbrev(ConfigPath())}
	st.Configured = len(st.Command) > 0 && strings.TrimSpace(st.Command[0]) != ""
	if _, err := engineArgv(cfg); err != nil {
		st.Reason = err.Error()
	} else {
		st.Available = true
	}
	return st
}

// engineStamp changes when the engine command does, so a running session started
// from the old command is replaced on the next turn instead of answering forever
// as the engine the user has since swapped out.
func engineStamp(cfg Config) string { return strings.Join(cfg.Engine.Command, "\x00") }

// chatBackendInfos is `agent --json`: the one engine, or an empty list.
func chatBackendInfos(cfg Config) []ChatBackendInfo {
	if !engineStatus(cfg).Configured {
		return []ChatBackendInfo{}
	}
	_, err := engineArgv(cfg)
	return []ChatBackendInfo{{
		ID: engineID, Name: filepath.Base(cfg.Engine.Command[0]),
		Available: err == nil, Recommended: true, Active: true,
	}}
}
