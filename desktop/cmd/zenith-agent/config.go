package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
	// Engine is the agent that answers the chat: any program that speaks the Agent
	// Client Protocol over stdio. Nothing is installed or guessed; with no command
	// here the chat says so and starts nothing. See engine.go.
	Engine struct {
		Command []string `json:"command,omitempty"`
	} `json:"engine,omitzero"`
	// Quick overrides the launcher fast lane's model connection. Empty means the
	// fast lane is off and quick asks go to the engine. Provider names one of the
	// built-in openai-compatible providers (see quickProviders); BaseURL/KeyEnv
	// override it for anything else.
	Quick struct {
		Provider string `json:"provider,omitempty"`
		Model    string `json:"model,omitempty"`
		BaseURL  string `json:"baseUrl,omitempty"`
		KeyEnv   string `json:"keyEnv,omitempty"`
	} `json:"quick,omitzero"`
	// Habits gates the vault's user-habits mining. History defaults on;
	// nil means enabled so an absent key keeps the feature.
	Habits struct {
		History *bool `json:"history,omitempty"`
	} `json:"habits,omitzero"`
}

// HabitsHistoryEnabled: shell-history mining is opt-out.
func (c Config) HabitsHistoryEnabled() bool {
	return c.Habits.History == nil || *c.Habits.History
}

// defaultConfig is off: nothing in Zenith turns the assistant on for the user,
// so a box with no config must read as disabled or the Hub switch would claim it
// is on while no unit runs. `enable` flips it.
func defaultConfig() Config {
	return Config{Port: 3600}
}

func LoadConfig() Config {
	c := defaultConfig()
	b, err := os.ReadFile(ConfigPath())
	if err != nil {
		return c
	}
	if json.Unmarshal(b, &c) != nil {
		return defaultConfig()
	}
	if c.Port <= 0 || c.Port > 65535 {
		c.Port = 3600
	}
	return c
}

// configProblem says why the config file cannot be used: "" when it parses or is
// absent. The file is edited by hand (that is how the engine is set), and a typo
// must not read as "nothing configured".
func configProblem() string {
	b, err := os.ReadFile(ConfigPath())
	if err != nil {
		return ""
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return "cannot read " + tildeAbbrev(ConfigPath()) + ": " + err.Error()
	}
	return ""
}

func SaveConfig(c Config) error {
	// Saving over a file we could not parse would replace the user's edit with defaults.
	if msg := configProblem(); msg != "" {
		return errors.New(msg)
	}
	p := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
