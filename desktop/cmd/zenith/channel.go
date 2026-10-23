package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// `zenith channel get` is read by the Hub's Updates page to say which channel
// the machine follows. `set` is not here: nothing calls it, and moving a
// machine between channels is one line in the sources file for now.

// aptSources is the deb822 file `zenith adopt` writes for Zenith's repository.
// The channel is its Suites: value, because the repository publishes each
// channel as an apt suite (release/mkrepo.sh takes the suite as an argument).
const aptSources = "/etc/apt/sources.list.d/zenith.sources"

const channelUsage = `Usage: zenith channel get [--root DIR]

Prints the channel this machine follows: the Suites value of Zenith's apt
source. Only "get" exists.
`

func init() {
	register("channel", "the apt channel this machine follows", zenithChannel)
}

func zenithChannel(args []string) int {
	set := newFlags("channel", channelUsage)
	root := set.String("root", "", "operate on a tree under this directory")
	positional, err := parseFlags(set, args)
	if err != nil {
		return exitUsage
	}
	if len(positional) != 1 || positional[0] != "get" {
		fmt.Fprint(os.Stderr, channelUsage)
		return exitUsage
	}

	channel, err := aptChannel(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith channel:", err)
		return exitFail
	}
	fmt.Println(channel)
	return exitOK
}

// aptChannel reads the first suite out of Zenith's apt source.
func aptChannel(root string) (string, error) {
	path := filepath.Join(root, aptSources)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no Zenith repository is configured: %s does not exist", path)
		}
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		field, value, found := strings.Cut(line, ":")
		if !found || !strings.EqualFold(strings.TrimSpace(field), "Suites") {
			continue
		}
		if suites := strings.Fields(value); len(suites) > 0 {
			return suites[0], nil
		}
	}
	return "", fmt.Errorf("%s names no suite", path)
}
