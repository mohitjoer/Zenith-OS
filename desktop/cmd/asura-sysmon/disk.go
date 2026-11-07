package main

import (
	"bufio"
	"bytes"
	"os"
	"strings"
	"syscall"
)

type DiskUsage struct {
	MountPoint  string  `json:"mountPoint"`
	Device      string  `json:"device"`
	FSType      string  `json:"fsType"`
	TotalBytes  uint64  `json:"totalBytes"`
	UsedBytes   uint64  `json:"usedBytes"`
	FreeBytes   uint64  `json:"freeBytes"`
	UsedPercent float64 `json:"usedPercent"`
}

// ignoredFSTypes are virtual, kernel, or pseudo filesystems to filter out
var ignoredFSTypes = map[string]bool{
	"sysfs":           true,
	"proc":            true,
	"devtmpfs":        true,
	"devpts":          true,
	"tmpfs":           true,
	"securityfs":      true,
	"cgroup":          true,
	"cgroup2":         true,
	"pstore":          true,
	"bpf":             true,
	"autofs":          true,
	"mqueue":          true,
	"hugetlbfs":       true,
	"debugfs":         true,
	"tracefs":         true,
	"configfs":        true,
	"ramfs":           true,
	"fusectl":         true,
	"efivarfs":        true,
	"binfmt_misc":     true,
	"fuse.portal":     true,
	"fuse.gvfsd-fuse": true,
	"rpc_pipefs":      true,
	"nfsd":            true,
	"overlay":         false, // keep overlay if used for root/live
	"squashfs":        false, // keep live image if mounted
}

func CollectDiskUsages() ([]DiskUsage, error) {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return []DiskUsage{}, err
	}

	seenMounts := make(map[string]bool)
	usages := []DiskUsage{}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		device := f[0]
		mount := f[1]
		fstype := f[2]

		if ignoredFSTypes[fstype] {
			continue
		}

		// Skip pseudo or virtual kernel mount points, container layers, and snaps
		if mount != "/" {
			if strings.HasPrefix(mount, "/sys") ||
				strings.HasPrefix(mount, "/proc") ||
				strings.HasPrefix(mount, "/dev") ||
				strings.HasPrefix(mount, "/run") ||
				strings.HasPrefix(mount, "/var/lib/docker") ||
				strings.HasPrefix(mount, "/snap") ||
				strings.HasPrefix(mount, "/var/lib/containers") {
				continue
			}
		}

		if seenMounts[mount] {
			continue
		}
		seenMounts[mount] = true

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount, &stat); err != nil {
			continue
		}

		bsize := uint64(stat.Bsize)
		total := stat.Blocks * bsize
		free := stat.Bavail * bsize
		if total == 0 {
			continue
		}
		var used uint64
		if total > free {
			used = total - free
		}
		pct := float64(used) / float64(total) * 100.0

		usages = append(usages, DiskUsage{
			MountPoint:  mount,
			Device:      device,
			FSType:      fstype,
			TotalBytes:  total,
			UsedBytes:   used,
			FreeBytes:   free,
			UsedPercent: pct,
		})
	}

	if usages == nil {
		usages = []DiskUsage{}
	}

	return usages, nil
}
