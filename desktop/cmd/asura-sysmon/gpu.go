package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type GPUStats struct {
	Name           string  `json:"name"`
	Vendor         string  `json:"vendor"`         // "nvidia", "amd", "intel", "virtio", "vmware", "virtualbox", etc.
	UtilizationPct float64 `json:"utilizationPct"` // 0-100%
	MemoryTotalMB  uint64  `json:"memoryTotalMB"`
	MemoryUsedMB   uint64  `json:"memoryUsedMB"`
	MemoryFreeMB   uint64  `json:"memoryFreeMB"`
	MemoryUsedPct  float64 `json:"memoryUsedPct"`
	TempC          float64 `json:"tempC"`
	IsVirtual      bool    `json:"isVirtual"`
}

var pciVendorNames = map[string]string{
	"0x10de": "NVIDIA",
	"0x1002": "AMD",
	"0x8086": "Intel",
	"0x1af4": "VirtIO (QEMU/KVM)",
	"0x15ad": "VMware",
	"0x80ee": "VirtualBox",
	"0x1234": "QEMU Standard VGA",
	"0x1a03": "ASPEED",
}

// queryNvidiaGPU attempts to gather telemetry using nvidia-smi if installed.
func queryNvidiaGPU() []GPUStats {
	var gpus []GPUStats

	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=name,temperature.gpu,utilization.gpu,memory.total,memory.used,memory.free",
		"--format=csv,noheader,nounits")

	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		f := strings.Split(line, ",")
		if len(f) < 6 {
			continue
		}

		name := strings.TrimSpace(f[0])
		temp, _ := strconv.ParseFloat(strings.TrimSpace(f[1]), 64)
		util, _ := strconv.ParseFloat(strings.TrimSpace(f[2]), 64)
		totalMB, _ := strconv.ParseUint(strings.TrimSpace(f[3]), 10, 64)
		usedMB, _ := strconv.ParseUint(strings.TrimSpace(f[4]), 10, 64)
		freeMB, _ := strconv.ParseUint(strings.TrimSpace(f[5]), 10, 64)

		var usedPct float64
		if totalMB > 0 {
			usedPct = (float64(usedMB) / float64(totalMB)) * 100.0
		}

		gpus = append(gpus, GPUStats{
			Name:           name,
			Vendor:         "nvidia",
			UtilizationPct: util,
			MemoryTotalMB:  totalMB,
			MemoryUsedMB:   usedMB,
			MemoryFreeMB:   freeMB,
			MemoryUsedPct:  usedPct,
			TempC:          temp,
		})
	}

	return gpus
}

// resolveDeviceModel tries lspci for an accurate name; falls back to vendor name.
func resolveDeviceModel(cardPath string, defaultName string) string {
	if _, err := exec.LookPath("lspci"); err != nil {
		return defaultName
	}

	slotLink, err := filepath.EvalSymlinks(filepath.Join(cardPath, "device"))
	if err == nil {
		slot := filepath.Base(slotLink)
		if strings.Contains(slot, ":") {
			ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
			defer cancel()
			out, err := exec.CommandContext(ctx, "lspci", "-s", slot).Output()
			if err == nil {
				line := strings.TrimSpace(string(out))
				// Output format: "00:02.0 VGA compatible controller: Intel Corporation Alder Lake-P..."
				if colon := strings.LastIndex(line, ": "); colon != -1 {
					model := strings.TrimSpace(line[colon+2:])
					if model != "" {
						return model
					}
				}
			}
		}
	}
	return defaultName
}

// queryDRMGPU inspects /sys/class/drm for Intel, AMD, VirtIO, and other graphics cards.
func queryDRMGPU(hasNvidia bool) []GPUStats {
	var gpus []GPUStats

	cards, err := filepath.Glob("/sys/class/drm/card[0-9]")
	if err != nil || len(cards) == 0 {
		return nil
	}

	for _, card := range cards {
		devDir := filepath.Join(card, "device")
		vendorRaw := readTrimmed(filepath.Join(devDir, "vendor"))
		if vendorRaw == "" {
			continue
		}
		vendorID := strings.ToLower(vendorRaw)

		// If NVIDIA was already queried via nvidia-smi, skip NVIDIA drm card
		if hasNvidia && vendorID == "0x10de" {
			continue
		}

		vendorKey := "other"
		defaultName := "Graphics Card"

		if v, ok := pciVendorNames[vendorID]; ok {
			defaultName = v + " Graphics"
			switch vendorID {
			case "0x10de":
				vendorKey = "nvidia"
			case "0x1002":
				vendorKey = "amd"
			case "0x8086":
				vendorKey = "intel"
			case "0x1af4":
				vendorKey = "virtio"
			case "0x15ad":
				vendorKey = "vmware"
			case "0x80ee":
				vendorKey = "virtualbox"
			}
		}

		friendlyName := resolveDeviceModel(card, defaultName)

		var utilPct float64
		var totalMB, usedMB, freeMB uint64
		var tempC float64

		// 1. AMD GPU handling
		if vendorID == "0x1002" {
			// AMD exposes gpu_busy_percent
			busyStr := readTrimmed(filepath.Join(devDir, "gpu_busy_percent"))
			if b, err := strconv.ParseFloat(busyStr, 64); err == nil {
				utilPct = b
			}
			// VRAM in bytes
			vramTotalBytes := readUint(filepath.Join(devDir, "mem_info_vram_total"))
			vramUsedBytes := readUint(filepath.Join(devDir, "mem_info_vram_used"))
			if vramTotalBytes > 0 {
				totalMB = vramTotalBytes / (1024 * 1024)
				usedMB = vramUsedBytes / (1024 * 1024)
				if totalMB >= usedMB {
					freeMB = totalMB - usedMB
				}
			}
		}

		// 2. Intel GPU handling
		if vendorID == "0x8086" {
			// Frequency-based load estimation
			actFreq := readFloat(filepath.Join(card, "gt_act_freq_mhz"))
			maxFreq := readFloat(filepath.Join(card, "gt_max_freq_mhz"))
			if maxFreq == 0 {
				actFreq = readFloat(filepath.Join(card, "gt/gt0/rps_act_freq_mhz"))
				maxFreq = readFloat(filepath.Join(card, "gt/gt0/rps_max_freq_mhz"))
			}
			if maxFreq > 0 && actFreq > 0 {
				utilPct = (actFreq / maxFreq) * 100.0
				if utilPct > 100.0 {
					utilPct = 100.0
				}
			}
		}

		// 3. Temperature from hwmon under device, or system hwmon matching vendor
		hwmons, _ := filepath.Glob(filepath.Join(devDir, "hwmon", "hwmon*", "temp*_input"))
		for _, hw := range hwmons {
			tVal := readFloat(hw)
			if tVal > 0 {
				if tVal > 1000 {
					tVal /= 1000.0
				}
				tempC = tVal
				break
			}
		}
		if tempC == 0 {
			// Check system hwmon directories for GPU drivers
			sysHwmons, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
			for _, shw := range sysHwmons {
				hName := strings.ToLower(readTrimmed(filepath.Join(shw, "name")))
				if (vendorID == "0x1002" && strings.Contains(hName, "amdgpu")) ||
					(vendorID == "0x8086" && (strings.Contains(hName, "i915") || strings.Contains(hName, "xe"))) ||
					(vendorID == "0x10de" && strings.Contains(hName, "nouveau")) {
					tVal := readFloat(filepath.Join(shw, "temp1_input"))
					if tVal > 0 {
						if tVal > 1000 {
							tVal /= 1000.0
						}
						tempC = tVal
						break
					}
				}
			}
		}

		var usedPct float64
		if totalMB > 0 {
			usedPct = (float64(usedMB) / float64(totalMB)) * 100.0
		}

		isVirtual := vendorKey == "vmware" || vendorKey == "virtualbox" || vendorKey == "virtio" || vendorID == "0x1234"

		gpus = append(gpus, GPUStats{
			Name:           friendlyName,
			Vendor:         vendorKey,
			UtilizationPct: utilPct,
			MemoryTotalMB:  totalMB,
			MemoryUsedMB:   usedMB,
			MemoryFreeMB:   freeMB,
			MemoryUsedPct:  usedPct,
			TempC:          tempC,
			IsVirtual:      isVirtual,
		})
	}

	return gpus
}

// CollectGPUStats collects hardware telemetry across NVIDIA, AMD, Intel, and virtualized graphics.
// On systems without a graphics controller, it safely returns an empty slice.
func CollectGPUStats() []GPUStats {
	results := []GPUStats{}

	nv := queryNvidiaGPU()
	if len(nv) > 0 {
		results = append(results, nv...)
	}

	drm := queryDRMGPU(len(nv) > 0)
	if len(drm) > 0 {
		results = append(results, drm...)
	}

	return results
}
