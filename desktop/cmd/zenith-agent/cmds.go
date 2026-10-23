package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Thin verb wrappers over the concern files (vault.go, index.go, agents.go,
// engine.go, server.go, setup.go) so main.go never changes shape.

func cmdIndex() error {
	if err := EnsureVault(); err != nil {
		return err
	}
	return Reindex()
}

func cmdWire(agent string) error {
	if err := EnsureVault(); err != nil {
		return err
	}
	if agent == "" {
		fmt.Printf("wired %d agents\n", WireAll()) // WireAll also links the skill everywhere
		return nil
	}
	if err := Wire(agent); err != nil {
		return err
	}
	// A single-agent wire still refreshes the always-created skill link
	// (~/.agents) and prowl's skills for the detected clients, so one wire is a
	// complete inject.
	_, _ = WireSkill()
	wireProwlSkills()
	return nil
}

// cmdPaths prints the agent manifest: every path Zenith Agent exposes plus a
// paste-ready snippet, so any coding agent (supported or not) can be pointed at
// the vault, the skill, and prowl.
func cmdPaths(format string) error {
	m := BuildManifest(LoadConfig())
	switch format {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(m)
	case "snippet":
		fmt.Print(m.Snippet)
		return nil
	}
	fmt.Println(displayName + " exposes these to your coding agent:")
	fmt.Println()
	fmt.Printf("  skill   %s\n          %s\n", dashIfEmpty(m.Skill.Path), m.Skill.Desc)
	fmt.Printf("  prowl   %s\n          %s\n", dashIfEmpty(m.Prowl.Path), m.Prowl.Desc)
	fmt.Println("  vault (generated = read-only, refreshed each reindex; yours = write here):")
	for _, v := range m.Vault {
		mark := "+"
		if !v.Exists {
			mark = "-"
		}
		fmt.Printf("    %s %-11s %-10s %s\n", mark, v.Label, "["+v.Owner+"]", v.Path)
	}
	fmt.Println()
	fmt.Println("Agents (one-click wire drops the pointer + skill + prowl):")
	for _, a := range m.Agents {
		fmt.Printf("  %-10s present=%-5v wired=%-5v skill=%-5v %s\n", a.ID, a.Present, a.Wired, a.SkillWired, a.File)
	}
	fmt.Println()
	fmt.Println("Agent engine (" + engineKey + " in " + m.Engine.Config + "):")
	if m.Engine.Reason != "" {
		fmt.Println("  " + m.Engine.Reason)
	} else {
		fmt.Println("  " + strings.Join(m.Engine.Command, " "))
	}
	fmt.Println()
	fmt.Println("Point any other agent at Zenith by pasting this into its instructions:")
	fmt.Println()
	fmt.Println(m.Snippet)
	return nil
}

// cmdAgent lists the coding agents the vault can be wired into and the engine,
// as JSON for the sidebar's picker with --json. There is one engine, named by
// engine.command, so `agent use` has nothing to switch and says so.
func cmdAgent(args []string) error {
	cfg := LoadConfig()
	if len(args) >= 1 && args[0] == "use" {
		if len(args) >= 2 && args[1] != engineID && args[1] != "auto" {
			return fmt.Errorf("there is one agent engine; to change it set %s in %s", engineKey, tildeAbbrev(ConfigPath()))
		}
		fmt.Println(engineLine(engineStatus(cfg)))
		return nil
	}
	if len(args) >= 1 && args[0] == "--json" {
		return json.NewEncoder(os.Stdout).Encode(chatBackendInfos(cfg))
	}
	for _, a := range DetectAgents() {
		fmt.Printf("agent:   %-10s present=%-5v wired=%-5v skill=%-5v\n", a.ID, a.Present, a.Wired, a.SkillWired)
	}
	fmt.Println("engine: ", engineLine(engineStatus(cfg)))
	return nil
}

// engineLine is the engine in one line: the command, or why there is none.
func engineLine(e EngineStatus) string {
	if e.Reason != "" {
		return e.Reason
	}
	return strings.Join(e.Command, " ")
}

// dashIfEmpty renders an empty path as "-" (not installed).
func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func cmdUnwire(agent string) error {
	if agent == "" {
		for _, a := range DetectAgents() {
			if a.Wired {
				if err := Unwire(a.ID); err != nil {
					return err
				}
			}
		}
		UnwireSkill()
		return nil
	}
	return Unwire(agent)
}

func cmdStatus(asJSON bool) error {
	st := BuildStatus(LoadConfig())
	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(st)
	}
	running := "stopped"
	if st.Running {
		running = fmt.Sprintf("running on http://127.0.0.1:%d", st.Port)
	}
	fmt.Printf("daemon:  %s (enabled: %v)\n", running, st.Enabled)
	fmt.Printf("vault:   %s (%d files)\n", st.Vault.Path, st.Vault.Files)
	fmt.Println("engine: ", engineLine(st.Engine))
	for _, a := range st.Agents {
		fmt.Printf("agent:   %-10s present=%-5v wired=%-5v skill=%-5v %s\n", a.ID, a.Present, a.Wired, a.SkillWired, a.File)
	}
	return nil
}
