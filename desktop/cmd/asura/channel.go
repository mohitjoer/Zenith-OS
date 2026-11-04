package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// `asura channel get` is read by the Hub's Updates page to say which channel
// the machine follows. `set` is not here: moving a machine between channels
// is one line in the repository config file.

// xbpsRepoConf is the repository configuration file for Void Linux XBPS.
const xbpsRepoConf = "/etc/xbps.d/asura.conf"

const channelUsage = `Usage: asura channel get [--root DIR]

Prints the channel this machine follows: the configured repository channel for Asura OS. Only "get" exists.
`

func init() {
	register("channel", "the package repository channel this machine follows", asuraChannel)
}

func asuraChannel(args []string) int {
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

	channel, err := xbpsChannel(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "asura channel:", err)
		return exitFail
	}
	fmt.Println(channel)
	return exitOK
}

// xbpsChannel reads the channel or primary repository from Asura's XBPS configuration.
func xbpsChannel(root string) (string, error) {
	path := filepath.Join(root, xbpsRepoConf)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			alt := filepath.Join(root, "/usr/share/xbps.d/asura.conf")
			data, err = os.ReadFile(alt)
			if err != nil {
				return "", fmt.Errorf("no Asura repository is configured: %s does not exist", path)
			}
		} else {
			return "", err
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}
		if field, value, found := strings.Cut(trimmed, "="); found {
			f := strings.ToLower(strings.TrimSpace(field))
			if f == "channel" {
				return strings.TrimSpace(value), nil
			}
			if f == "repository" {
				repo := strings.TrimSpace(value)
				if repo != "" {
					return repo, nil
				}
			}
		}
	}
	return "", fmt.Errorf("%s names no channel or repository", path)
}
