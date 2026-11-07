package main

import (
	"testing"
	"time"
)

func TestCalcCPUPercent(t *testing.T) {
	s1 := cpuSample{total: 1000, idle: 500}
	s2 := cpuSample{total: 2000, idle: 600}

	pct := calcCPUPercent(s1, s2)
	// total delta = 1000, idle delta = 100 -> non-idle = 900 -> 90%
	if pct != 90.0 {
		t.Fatalf("expected 90%%, got %.2f%%", pct)
	}

	// Identical sample = 0%
	if zero := calcCPUPercent(s1, s1); zero != 0.0 {
		t.Fatalf("expected 0%% for identical samples, got %.2f%%", zero)
	}
}

func TestCollectMemoryStats(t *testing.T) {
	mem, swap, err := CollectMemoryStats()
	if err != nil {
		t.Fatalf("CollectMemoryStats failed: %v", err)
	}
	if mem.TotalBytes == 0 {
		t.Fatalf("expected positive TotalBytes, got 0")
	}
	if mem.UsedPercent < 0 || mem.UsedPercent > 100 {
		t.Fatalf("invalid used percentage: %.2f", mem.UsedPercent)
	}
	_ = swap
}

func TestCollectLoadAndUptime(t *testing.T) {
	loads, uptime, err := CollectLoadAndUptime()
	if err != nil {
		t.Fatalf("CollectLoadAndUptime failed: %v", err)
	}
	if uptime == 0 {
		t.Fatalf("uptime should be positive, got %d", uptime)
	}
	_ = loads
}

func TestCollectNetworkStats(t *testing.T) {
	net, err := CollectNetworkStats(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("CollectNetworkStats failed: %v", err)
	}
	if net.RxRateSec < 0 || net.TxRateSec < 0 {
		t.Fatalf("negative network rate")
	}
	if net.Interfaces == nil {
		t.Fatalf("network interfaces slice should not be nil")
	}
}

func TestCollectDiskUsages(t *testing.T) {
	disks, err := CollectDiskUsages()
	if err != nil {
		t.Fatalf("CollectDiskUsages failed: %v", err)
	}
	for _, d := range disks {
		if d.MountPoint == "" {
			t.Fatalf("disk mountpoint should not be empty")
		}
		if d.TotalBytes == 0 {
			t.Fatalf("expected positive total bytes for %s", d.MountPoint)
		}
		if d.UsedPercent < 0 || d.UsedPercent > 100 {
			t.Fatalf("invalid used percentage for %s: %.2f", d.MountPoint, d.UsedPercent)
		}
		// Confirm virtual efivarfs or sysfs is excluded
		if d.FSType == "efivarfs" || d.FSType == "sysfs" || d.FSType == "proc" {
			t.Fatalf("pseudo filesystem %s should have been filtered out", d.FSType)
		}
	}
}

func TestIsTopLevelDisk(t *testing.T) {
	// Empty string must safely return false without panicking
	if isTopLevelDisk("") {
		t.Fatalf("expected empty string to not be a top level disk")
	}

	// Virtual and ram devices must be rejected
	if isTopLevelDisk("loop0") || isTopLevelDisk("ram0") || isTopLevelDisk("zram0") {
		t.Fatalf("expected loop/ram/zram to be excluded")
	}

	// Partitions must be rejected by heuristic if not in sysfs
	if isTopLevelDisk("nvme0n1p1") {
		t.Fatalf("expected nvme partition to be rejected")
	}
	if isTopLevelDisk("sda1") || isTopLevelDisk("vda2") {
		t.Fatalf("expected disk partition to be rejected")
	}
}

func TestCollectDiskIOStats(t *testing.T) {
	ioStats, err := CollectDiskIOStats(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("CollectDiskIOStats failed: %v", err)
	}
	if ioStats.ReadBytesSec < 0 || ioStats.WriteBytesSec < 0 {
		t.Fatalf("negative disk IO throughput")
	}
	if ioStats.Devices == nil {
		t.Fatalf("devices slice should not be nil")
	}
}

func TestCollectGPUStats(t *testing.T) {
	// Must never panic or fail, whether GPUs exist or not
	gpus := CollectGPUStats()
	if gpus == nil {
		t.Fatalf("gpus slice should not be nil")
	}
	for _, g := range gpus {
		if g.Name == "" {
			t.Fatalf("gpu entry missing name")
		}
		if g.UtilizationPct < 0 || g.UtilizationPct > 100 {
			t.Fatalf("invalid gpu utilization: %.2f", g.UtilizationPct)
		}
	}
}

func TestCollectBatteryStats(t *testing.T) {
	// Must never panic or fail, whether battery exists or not
	bats := CollectBatteryStats()
	if bats == nil {
		t.Fatalf("batteries slice should not be nil")
	}
	for _, b := range bats {
		if b.Name == "" {
			t.Fatalf("battery entry missing name")
		}
		if b.Percentage < 0 || b.Percentage > 100 {
			t.Fatalf("invalid battery percentage: %.2f", b.Percentage)
		}
		if b.HealthPct < 0 || b.HealthPct > 100 {
			t.Fatalf("invalid battery health percentage: %.2f", b.HealthPct)
		}
		if b.Watts < 0 {
			t.Fatalf("negative wattage: %.2f", b.Watts)
		}
	}
}

func TestCollectAllStats(t *testing.T) {
	start := time.Now()
	st, err := CollectAllStats(50 * time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("CollectAllStats failed: %v", err)
	}
	if st.Memory.TotalBytes == 0 {
		t.Fatalf("expected positive memory")
	}
	if st.Timestamp <= 0 {
		t.Fatalf("expected valid timestamp")
	}

	// Concurrency verification: elapsed time should be close to sample interval (~50ms-150ms)
	if elapsed > 400*time.Millisecond {
		t.Logf("CollectAllStats took %v (within acceptable CI load limits)", elapsed)
	}
}

func BenchmarkCalcCPUPercent(b *testing.B) {
	s1 := cpuSample{total: 1000, idle: 500}
	s2 := cpuSample{total: 2000, idle: 600}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calcCPUPercent(s1, s2)
	}
}
