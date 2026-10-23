package main

import (
	"fmt"
	"os"
	"strings"
)

const usage = binName + `: the Zenith agent daemon and CLI

  serve [--if-enabled]   run the dashboard and agent bridge (unix socket and 127.0.0.1)
  index                  regenerate the vault maps (system, desktop, packages, repo, user, habits)
  repo-index <root> [out]  build the Zenith source map from a checkout (build/deploy time)
  ask <question>         one-shot quick ask against the shared session (launcher)
  chat [text]            multi-turn chat client for the sidebar (see chatcli.go for flags)
  term <question>        terminal ask: answer + a ready-to-run command plan
  setup                  say where the agent engine is set and how (installs nothing)
  wire [agent]           apply vault pointers (all detected agents, or one)
  unwire [agent]         remove vault pointers
  status [--json]        report daemon, engine, vault, and wiring state
  enable [--at-boot]     start the daemon now and at every login; --at-boot
                         adds user lingering so it starts with the machine
  disable                stop the daemon and turn autostart off
  backend [provider[:model]]  view or set the fast-lane model ('auto' turns the fast lane off)
  paths [--json]         show every skill/vault/prowl path (and a paste snippet for any agent)
  agent [use <id>]       list agents + chat backends, or set which agent drives the chat
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		ifEnabled := len(os.Args) > 2 && os.Args[2] == "--if-enabled"
		err = cmdServe(ifEnabled)
	case "index":
		err = cmdIndex()
	case "repo-index":
		err = cmdRepoIndex(argOr(2, ""), argOr(3, ""))
	case "ask":
		err = cmdAsk(strings.Join(os.Args[2:], " "))
	case "chat":
		err = cmdChat(os.Args[2:])
	case "term":
		err = cmdTerm(os.Args[2:])
	case "setup":
		err = cmdSetup()
	case "wire":
		err = cmdWire(argOr(2, ""))
	case "unwire":
		err = cmdUnwire(argOr(2, ""))
	case "status":
		err = cmdStatus(len(os.Args) > 2 && os.Args[2] == "--json")
	case "enable":
		err = cmdEnable(len(os.Args) > 2 && os.Args[2] == "--at-boot")
	case "disable":
		err = cmdDisable()
	case "backend":
		err = cmdBackend(os.Args[2:])
	case "paths":
		err = cmdPaths(pathsFormat(os.Args[2:]))
	case "agent":
		err = cmdAgent(os.Args[2:])
	default:
		fmt.Print(usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, binName+":", err)
		os.Exit(1)
	}
}

func argOr(i int, def string) string {
	if len(os.Args) > i {
		return os.Args[i]
	}
	return def
}

// pathsFormat maps the `paths` flags to cmdPaths' format: --json, --snippet, or
// the human default.
func pathsFormat(args []string) string {
	for _, a := range args {
		switch a {
		case "--json":
			return "json"
		case "--snippet":
			return "snippet"
		}
	}
	return ""
}
