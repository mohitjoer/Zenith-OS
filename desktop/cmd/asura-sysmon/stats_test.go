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

func TestCollectAllStats(t *testing.T) {
	st, err := CollectAllStats(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("CollectAllStats failed: %v", err)
	}
	if st.Memory.TotalBytes == 0 {
		t.Fatalf("expected positive memory")
	}
	if st.Timestamp <= 0 {
		t.Fatalf("expected valid timestamp")
	}
}
