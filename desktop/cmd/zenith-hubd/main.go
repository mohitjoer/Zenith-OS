// zenith-hubd is the Go backend for the Zenith Settings GUI. The Quickshell front end
// (qs -c hub) shells out to it the same way the rest of the desktop talks to
// zenith-shell: a subcommand prints data on stdout or mutates persisted state.
//
//	zenith-hubd keybinds            print the keybind legend as JSON
//	zenith-hubd config get <key>    print a stored config value
//	zenith-hubd config set <k> <v>  persist a config value (TOML)
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "keybinds":
		b, err := json.Marshal(keybinds())
		if err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
		os.Stdout.Write(b)
		fmt.Println()
	case "apps":
		if err := printApps(); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
		fmt.Println()
	case "config":
		if err := runConfig(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "shell":
		if err := runShellPref(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "desktop":
		if err := runDesktop(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "wm":
		if err := runWm(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "outputs":
		if err := runOutputs(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "lock":
		if err := runLock(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "gpu":
		if err := runGpu(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "cpu":
		if err := runCpu(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "lighting":
		if err := runLighting(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "voxtype":
		if err := unavailable("voxtype", "dictation needs the voxtype engine, which Zenith does not ship", args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "rice":
		if err := runRice(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "fastfetch":
		if err := runFastfetch(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "profile":
		if err := runProfile(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "reload-cover":
		if err := runReloadCover(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "import":
		if err := runImport(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "palette-bridge":
		if err := runPaletteBridge(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "share":
		if err := runShare(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	case "clipboard":
		if err := runClipboard(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "zenith-hubd:", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func runConfig(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("config needs get|set")
	}
	switch args[0] {
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("config get needs a key")
		}
		v, ok := configGet(args[1])
		if !ok {
			return fmt.Errorf("unknown config key: %s", args[1])
		}
		fmt.Println(v)
		return nil
	case "set":
		if len(args) < 3 {
			return fmt.Errorf("config set needs a key and value")
		}
		return configSet(args[1], args[2])
	default:
		return fmt.Errorf("config needs get|set")
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  zenith-hubd keybinds")
	fmt.Fprintln(os.Stderr, "  zenith-hubd config get <key>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd config set <key> <value>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd shell get|set <fish|bash|zsh>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd desktop get|defaults|cursors|layouts")
	fmt.Fprintln(os.Stderr, "  zenith-hubd desktop variants <layout>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd desktop save|preview <json>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd desktop restore")
	fmt.Fprintln(os.Stderr, "  zenith-hubd desktop set-rebind <default> <chosen>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd desktop plugins list")
	fmt.Fprintln(os.Stderr, "  zenith-hubd desktop plugins rebuild [--all|--stale] [--checkout <dir>] [<id>...]")
	fmt.Fprintln(os.Stderr, "  zenith-hubd desktop plugins add [--inspect] <git-url> [<plugin>...]")
	fmt.Fprintln(os.Stderr, "  zenith-hubd desktop plugins remove <id>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd wm list")
	fmt.Fprintln(os.Stderr, "  zenith-hubd wm preview <name>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd outputs [list]")
	fmt.Fprintln(os.Stderr, "  zenith-hubd outputs apply <json>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd outputs profiles")
	fmt.Fprintln(os.Stderr, "  zenith-hubd outputs save <name> <json>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd outputs load|rm <name>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd lock list|set <slug>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd gpu caps|mode get|set <mode>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd gpu tune caps|get")
	fmt.Fprintln(os.Stderr, "  zenith-hubd gpu tune set <gpu> <id> <value>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd gpu tune reset [<gpu>]")
	fmt.Fprintln(os.Stderr, "  zenith-hubd cpu caps [<profile>]")
	fmt.Fprintln(os.Stderr, "  zenith-hubd cpu active")
	fmt.Fprintln(os.Stderr, "  zenith-hubd cpu set <scope> <id> <value>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd lighting state|scan|enable|disable|apply")
	fmt.Fprintln(os.Stderr, "  zenith-hubd lighting set <device> <json>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd lighting accent [#RRGGBB]")
	fmt.Fprintln(os.Stderr, "  zenith-hubd lighting save|release <device>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd voxtype ...   (answers \"unavailable\": no dictation engine on Zenith)")
	fmt.Fprintln(os.Stderr, "  zenith-hubd palette-bridge status|install|service|integration|doctor [<source>]")
	fmt.Fprintln(os.Stderr, "  zenith-hubd share status")
	fmt.Fprintln(os.Stderr, "  zenith-hubd rice list|preflight|capture|apply|restore|save|fork|delete|import|setwall|files|export")
	fmt.Fprintln(os.Stderr, "  zenith-hubd fastfetch get|preview <json>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd fastfetch save <json>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd fastfetch import-logo <path>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd reload-cover import <path>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd reload-cover prune [<managed-path>]")
	fmt.Fprintln(os.Stderr, "  zenith-hubd clipboard stats|prune")
	fmt.Fprintln(os.Stderr, "  zenith-hubd import scan <path|url>")
	fmt.Fprintln(os.Stderr, "  zenith-hubd import apply <decisions.json|->")
	fmt.Fprintln(os.Stderr, "  zenith-hubd import undo [<ts>]")
}
