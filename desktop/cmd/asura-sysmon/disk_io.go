package main

import (
	"bufio"
	"bytes"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DiskIOStats struct {
	ReadBytesSec  float64    `json:"readBytesSec"`
	WriteBytesSec float64    `json:"writeBytesSec"`
	ReadIOPS      float64    `json:"readIOPS"`
	WriteIOPS     float64    `json:"writeIOPS"`
	Devices       []DeviceIO `json:"devices"`
}

type DeviceIO struct {
	Device        string  `json:"device"`
	ReadBytesSec  float64 `json:"readBytesSec"`
	WriteBytesSec float64 `json:"writeBytesSec"`
	ReadIOPS      float64 `json:"readIOPS"`
	WriteIOPS     float64 `json:"writeIOPS"`
}

type diskSample struct {
	readsCompleted  uint64
	sectorsRead     uint64
	writesCompleted uint64
	sectorsWritten  uint64
}

// isTopLevelDisk determines if devName represents a whole disk rather than a partition or virtual device.
func isTopLevelDisk(devName string) bool {
	if len(devName) == 0 {
		return false
	}

	// Filter out non-storage / loopback / ramdisks
	if strings.HasPrefix(devName, "loop") ||
		strings.HasPrefix(devName, "ram") ||
		strings.HasPrefix(devName, "zram") {
		return false
	}

	// If /sys/block exists, top-level disks have a directory in /sys/block/<devName>
	if fi, err := os.Stat("/sys/block/" + devName); err == nil && fi.IsDir() {
		return true
	}

	// Fallback heuristic if /sys/block is unmounted or in mock environment:
	// Exclude partition names: e.g. nvme0n1p1, sda1, vda2, mmcblk0p1
	if strings.Contains(devName, "p") {
		// nvme0n1pX, mmcblk0pX
		parts := strings.Split(devName, "p")
		if len(parts) == 2 {
			if _, err := strconv.Atoi(parts[1]); err == nil {
				return false
			}
		}
	}
	// Check trailing digits on sdX1, vdX1, hdX1
	lastChar := devName[len(devName)-1]
	if lastChar >= '0' && lastChar <= '9' {
		// If it has non-digit prefix like sda1
		if strings.HasPrefix(devName, "sd") || strings.HasPrefix(devName, "vd") ||
			strings.HasPrefix(devName, "hd") || strings.HasPrefix(devName, "xvd") {
			return false
		}
	}

	return true
}

func readDiskSamples() (map[string]diskSample, error) {
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return nil, err
	}

	samples := make(map[string]diskSample)
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}

		devName := fields[2]
		if !isTopLevelDisk(devName) {
			continue
		}

		readsCompleted, _ := strconv.ParseUint(fields[3], 10, 64)
		sectorsRead, _ := strconv.ParseUint(fields[5], 10, 64)
		writesCompleted, _ := strconv.ParseUint(fields[7], 10, 64)
		sectorsWritten, _ := strconv.ParseUint(fields[9], 10, 64)

		samples[devName] = diskSample{
			readsCompleted:  readsCompleted,
			sectorsRead:     sectorsRead,
			writesCompleted: writesCompleted,
			sectorsWritten:  sectorsWritten,
		}
	}

	return samples, nil
}

// CollectDiskIOStats samples disk throughput and IOPS over sampleInterval.
func CollectDiskIOStats(sampleInterval time.Duration) (DiskIOStats, error) {
	s1, err := readDiskSamples()
	if err != nil {
		return DiskIOStats{Devices: []DeviceIO{}}, err
	}

	if sampleInterval <= 0 {
		sampleInterval = 200 * time.Millisecond
	}

	time.Sleep(sampleInterval)

	s2, err := readDiskSamples()
	if err != nil {
		return DiskIOStats{Devices: []DeviceIO{}}, err
	}

	secs := sampleInterval.Seconds()
	if secs <= 0 {
		secs = 0.2
	}

	var totalReadBytesSec, totalWriteBytesSec, totalReadIOPS, totalWriteIOPS float64
	devices := []DeviceIO{}

	for dev, cur := range s2 {
		prev, ok := s1[dev]
		if !ok {
			continue
		}

		var readSecsDelta, writeSecsDelta, readOpsDelta, writeOpsDelta uint64
		if cur.sectorsRead >= prev.sectorsRead {
			readSecsDelta = cur.sectorsRead - prev.sectorsRead
		}
		if cur.sectorsWritten >= prev.sectorsWritten {
			writeSecsDelta = cur.sectorsWritten - prev.sectorsWritten
		}
		if cur.readsCompleted >= prev.readsCompleted {
			readOpsDelta = cur.readsCompleted - prev.readsCompleted
		}
		if cur.writesCompleted >= prev.writesCompleted {
			writeOpsDelta = cur.writesCompleted - prev.writesCompleted
		}

		// Sector size is 512 bytes on Linux /proc/diskstats
		readBytesSec := (float64(readSecsDelta) * 512.0) / secs
		writeBytesSec := (float64(writeSecsDelta) * 512.0) / secs
		readIOPS := float64(readOpsDelta) / secs
		writeIOPS := float64(writeOpsDelta) / secs

		devices = append(devices, DeviceIO{
			Device:        dev,
			ReadBytesSec:  readBytesSec,
			WriteBytesSec: writeBytesSec,
			ReadIOPS:      readIOPS,
			WriteIOPS:     writeIOPS,
		})

		totalReadBytesSec += readBytesSec
		totalWriteBytesSec += writeBytesSec
		totalReadIOPS += readIOPS
		totalWriteIOPS += writeIOPS
	}

	if devices == nil {
		devices = []DeviceIO{}
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].Device < devices[j].Device
	})

	return DiskIOStats{
		ReadBytesSec:  totalReadBytesSec,
		WriteBytesSec: totalWriteBytesSec,
		ReadIOPS:      totalReadIOPS,
		WriteIOPS:     totalWriteIOPS,
		Devices:       devices,
	}, nil
}
