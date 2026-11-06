package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// seedAppsXbps is the curated Discover shelf for Asura OS (Void Linux).
var seedAppsXbps = []struct{ pkg, title string }{
	{"vlc", "VLC"}, {"mpv", "mpv"}, {"gimp", "GIMP"}, {"inkscape", "Inkscape"},
	{"krita", "Krita"}, {"blender", "Blender"}, {"darktable", "darktable"},
	{"obs", "OBS Studio"}, {"kdenlive", "Kdenlive"}, {"audacity", "Audacity"},
	{"handbrake", "HandBrake"}, {"easyeffects", "EasyEffects"}, {"pavucontrol", "Volume Control"},
	{"libreoffice", "LibreOffice"}, {"keepassxc", "KeePassXC"}, {"syncthing", "Syncthing"},
	{"transmission-gtk", "Transmission"}, {"qutebrowser", "qutebrowser"},
	{"zed", "Zed Editor"}, {"git", "Git"}, {"lazygit", "lazygit"}, {"tmux", "tmux"},
	{"btop", "btop"}, {"htop", "htop"}, {"ripgrep", "ripgrep"}, {"fzf", "fzf"},
	{"gparted", "GParted"}, {"steam", "Steam"}, {"lutris", "Lutris"},
	{"ghostty", "Ghostty Terminal"}, {"fastfetch", "Fastfetch"},
}

var xbpsPkgName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9+._-]+$`)

type xbpsProvider struct{}

func (xbpsProvider) Category() Category {
	return Category{
		ID:          "apps",
		Name:        "Apps",
		Group:       "find",
		Description: "Native packages from the Void Linux and Asura OS repositories, managed with XBPS.",
	}
}

func (xbpsProvider) Load(ctx context.Context, _ bool) ([]Item, SourceState, error) {
	if !have("xbps-query") {
		return nil, SourceState{}, fmt.Errorf("xbps is not available on this machine")
	}
	installed, err := xbpsInstalled(ctx)
	if err != nil {
		return nil, SourceState{}, err
	}

	items := make([]Item, 0, len(seedAppsXbps))
	for _, seed := range seedAppsXbps {
		info, ok := xbpsPackageShow(ctx, seed.pkg)
		instVer := installed[seed.pkg]
		isInst := instVer != ""
		if !ok && !isInst {
			continue
		}
		name := seed.title
		summary := info.summary
		if summary == "" && isInst {
			summary = "Installed system package"
		}
		items = append(items, Item{
			ID:               seed.pkg,
			Category:         "apps",
			Name:             name,
			Summary:          summary,
			Description:      info.desc,
			Version:          info.version,
			InstalledVersion: instVer,
			Upstream:         info.homepage,
			Installed:        isInst,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items, SourceState{}, nil
}

func (xbpsProvider) Search(ctx context.Context, query string) ([]Item, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []Item{}, nil
	}
	out, err := exec.CommandContext(ctx, "xbps-query", "-Rs", query).Output()
	if err != nil {
		return []Item{}, nil
	}
	installed, _ := xbpsInstalled(ctx)

	var items []Item
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// Format: [-] pkgname-version summary
		pkgAndVer := fields[1]
		dashIdx := strings.LastIndex(pkgAndVer, "-")
		if dashIdx <= 0 {
			continue
		}
		pkgName := pkgAndVer[:dashIdx]
		ver := pkgAndVer[dashIdx+1:]
		summary := ""
		if len(fields) > 2 {
			summary = strings.Join(fields[2:], " ")
		}
		items = append(items, Item{
			ID:               pkgName,
			Category:         "apps",
			Name:             pkgName,
			Summary:          summary,
			Version:          ver,
			Installed:        installed[pkgName] != "",
			InstalledVersion: installed[pkgName],
		})
	}
	return items, nil
}

func (xbpsProvider) Install(ctx context.Context, id string) error {
	if !xbpsPkgName.MatchString(id) {
		return fmt.Errorf("invalid package name %q", id)
	}
	cmd := exec.CommandContext(ctx, "asura-pkg-add", id)
	if !have("asura-pkg-add") {
		cmd = exec.CommandContext(ctx, "pkexec", "xbps-install", "-y", id)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install failed: %s (%v)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (xbpsProvider) Remove(ctx context.Context, id string) error {
	if !xbpsPkgName.MatchString(id) {
		return fmt.Errorf("invalid package name %q", id)
	}
	cmd := exec.CommandContext(ctx, "asura-pkg-remove", id)
	if !have("asura-pkg-remove") {
		cmd = exec.CommandContext(ctx, "pkexec", "xbps-remove", "-y", id)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove failed: %s (%v)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

type xbpsPkgInfo struct {
	version  string
	summary  string
	desc     string
	homepage string
}

func xbpsInstalled(ctx context.Context) (map[string]string, error) {
	out, err := exec.CommandContext(ctx, "xbps-query", "-l").Output()
	if err != nil {
		return nil, err
	}
	res := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[0], "ii") {
			pkgAndVer := fields[1]
			idx := strings.LastIndex(pkgAndVer, "-")
			if idx > 0 {
				res[pkgAndVer[:idx]] = pkgAndVer[idx+1:]
			}
		}
	}
	return res, nil
}

func xbpsPackageShow(ctx context.Context, pkg string) (xbpsPkgInfo, bool) {
	out, err := exec.CommandContext(ctx, "xbps-query", "-S", pkg).Output()
	if err != nil {
		return xbpsPkgInfo{}, false
	}
	info := xbpsPkgInfo{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if colon := strings.Index(line, ":"); colon > 0 {
			key := strings.TrimSpace(line[:colon])
			val := strings.TrimSpace(line[colon+1:])
			switch key {
			case "pkgver":
				if idx := strings.LastIndex(val, "-"); idx > 0 {
					info.version = val[idx+1:]
				} else {
					info.version = val
				}
			case "short_desc":
				info.summary = val
			case "homepage":
				info.homepage = val
			}
		}
	}
	return info, true
}
