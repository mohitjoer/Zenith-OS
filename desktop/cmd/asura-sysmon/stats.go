package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type SystemStats struct {
	CPU        CPUStats       `json:"cpu"`
	Memory     MemStats       `json:"memory"`
	Swap       SwapStats      `json:"swap"`
	Network    NetworkStats   `json:"network"`
	Disks      []DiskUsage    `json:"disks"`
	DiskIO     DiskIOStats    `json:"diskIO"`
	GPUs       []GPUStats     `json:"gpus"`
	Batteries  []BatteryStats `json:"batteries"`
	UptimeSecs uint64         `json:"uptimeSecs"`
	LoadAvg    [3]float64     `json:"loadAvg"`
	Timestamp  int64          `json:"timestamp"`
}

type CPUStats struct {
	UsagePercent float64   `json:"usagePercent"`
	Cores        []float64 `json:"cores"`
	TempC        float64   `json:"tempC"`
}

type MemStats struct {
	TotalBytes     uint64  `json:"totalBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	UsedPercent    float64 `json:"usedPercent"`
	CachedBytes    uint64  `json:"cachedBytes"`
	BuffersBytes   uint64  `json:"buffersBytes"`
}

type SwapStats struct {
	TotalBytes  uint64  `json:"totalBytes"`
	UsedBytes   uint64  `json:"usedBytes"`
	FreeBytes   uint64  `json:"freeBytes"`
	UsedPercent float64 `json:"usedPercent"`
}

type NetworkStats struct {
	RxBytes    uint64  `json:"rxBytes"`
	TxBytes    uint64  `json:"txBytes"`
	RxRateSec  float64 `json:"rxRateSec"` // bytes per second
	TxRateSec  float64 `json:"txRateSec"`
	Interfaces []NetIf `json:"interfaces"`
}

type NetIf struct {
	Name    string `json:"name"`
	RxBytes uint64 `json:"rxBytes"`
	TxBytes uint64 `json:"txBytes"`
}

// cpuSample captures total and idle ticks
type cpuSample struct {
	total uint64
	idle  uint64
}

func readCPUSamples() (cpuSample, []cpuSample, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuSample{}, nil, err
	}
	var overall cpuSample
	var cores []cpuSample

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		var total, idle uint64
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			total += v
			if i == 4 || i == 5 { // idle + iowait
				idle += v
			}
		}
		s := cpuSample{total: total, idle: idle}
		if fields[0] == "cpu" {
			overall = s
		} else if strings.HasPrefix(fields[0], "cpu") {
			cores = append(cores, s)
		}
	}
	return overall, cores, nil
}

func calcCPUPercent(s1, s2 cpuSample) float64 {
	if s2.total <= s1.total {
		return 0.0
	}
	totDelta := float64(s2.total - s1.total)
	idleDelta := float64(s2.idle - s1.idle)
	if idleDelta > totDelta {
		idleDelta = totDelta
	}
	pct := (totDelta - idleDelta) / totDelta * 100.0
	if pct < 0.0 {
		return 0.0
	}
	if pct > 100.0 {
		return 100.0
	}
	return pct
}

func CollectCPUStats(sampleInterval time.Duration) (CPUStats, error) {
	o1, c1, err := readCPUSamples()
	if err != nil {
		return CPUStats{}, err
	}
	if sampleInterval <= 0 {
		sampleInterval = 200 * time.Millisecond
	}
	time.Sleep(sampleInterval)
	o2, c2, err := readCPUSamples()
	if err != nil {
		return CPUStats{}, err
	}

	overallPct := calcCPUPercent(o1, o2)
	corePcts := make([]float64, len(c1))
	for i := 0; i < len(c1) && i < len(c2); i++ {
		corePcts[i] = calcCPUPercent(c1[i], c2[i])
	}

	temp := readCPUTemp()

	return CPUStats{
		UsagePercent: overallPct,
		Cores:        corePcts,
		TempC:        temp,
	}, nil
}

func readCPUTemp() float64 {
	// Look through /sys/class/thermal/thermal_zone*
	matches, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err == nil {
			v, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
			if err == nil && v > 0 {
				if v > 1000 {
					return v / 1000.0
				}
				return v
			}
		}
	}
	// Fallback to /sys/class/hwmon/hwmon*/temp1_input
	hwmon, _ := filepath.Glob("/sys/class/hwmon/hwmon*/temp*_input")
	for _, m := range hwmon {
		data, err := os.ReadFile(m)
		if err == nil {
			v, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
			if err == nil && v > 0 {
				if v > 1000 {
					return v / 1000.0
				}
				return v
			}
		}
	}
	return 0.0
}

func CollectMemoryStats() (MemStats, SwapStats, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return MemStats{}, SwapStats{}, err
	}

	var total, avail, free, cached, buffers, swapTotal, swapFree uint64
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(f[1], 10, 64)
		valBytes := val * 1024 // meminfo reports in kB

		switch f[0] {
		case "MemTotal:":
			total = valBytes
		case "MemAvailable:":
			avail = valBytes
		case "MemFree:":
			free = valBytes
		case "Cached:":
			cached = valBytes
		case "Buffers:":
			buffers = valBytes
		case "SwapTotal:":
			swapTotal = valBytes
		case "SwapFree:":
			swapFree = valBytes
		}
	}

	if avail == 0 && free > 0 {
		avail = free + cached + buffers
	}
	var used uint64
	if total > avail {
		used = total - avail
	}
	var memPct float64
	if total > 0 {
		memPct = float64(used) / float64(total) * 100.0
	}

	var swapUsed uint64
	if swapTotal > swapFree {
		swapUsed = swapTotal - swapFree
	}
	var swapPct float64
	if swapTotal > 0 {
		swapPct = float64(swapUsed) / float64(swapTotal) * 100.0
	}

	return MemStats{
			TotalBytes:     total,
			AvailableBytes: avail,
			UsedBytes:      used,
			UsedPercent:    memPct,
			CachedBytes:    cached,
			BuffersBytes:   buffers,
		}, SwapStats{
			TotalBytes:  swapTotal,
			UsedBytes:   swapUsed,
			FreeBytes:   swapFree,
			UsedPercent: swapPct,
		}, nil
}

func readNetDev() ([]NetIf, uint64, uint64, error) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return []NetIf{}, 0, 0, err
	}
	ifs := []NetIf{}
	var totalRx, totalTx uint64

	scanner := bufio.NewScanner(bytes.NewReader(data))
	// skip headers (first 2 lines)
	if scanner.Scan() && scanner.Scan() {
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			colon := strings.Index(line, ":")
			if colon == -1 {
				continue
			}
			name := strings.TrimSpace(line[:colon])
			if name == "lo" {
				continue
			}
			fields := strings.Fields(line[colon+1:])
			if len(fields) < 9 {
				continue
			}
			rx, _ := strconv.ParseUint(fields[0], 10, 64)
			tx, _ := strconv.ParseUint(fields[8], 10, 64)

			ifs = append(ifs, NetIf{Name: name, RxBytes: rx, TxBytes: tx})
			totalRx += rx
			totalTx += tx
		}
	}
	if ifs == nil {
		ifs = []NetIf{}
	}
	return ifs, totalRx, totalTx, nil
}

func CollectNetworkStats(interval time.Duration) (NetworkStats, error) {
	ifs1, rx1, tx1, err := readNetDev()
	if err != nil {
		return NetworkStats{Interfaces: []NetIf{}}, err
	}
	if interval <= 0 {
		return NetworkStats{
			RxBytes:    rx1,
			TxBytes:    tx1,
			Interfaces: ifs1,
		}, nil
	}
	time.Sleep(interval)
	ifs2, rx2, tx2, err := readNetDev()
	if err != nil {
		return NetworkStats{Interfaces: []NetIf{}}, err
	}
	secs := interval.Seconds()
	rxRate := float64(rx2-rx1) / secs
	txRate := float64(tx2-tx1) / secs
	if rxRate < 0 {
		rxRate = 0
	}
	if txRate < 0 {
		txRate = 0
	}

	return NetworkStats{
		RxBytes:    rx2,
		TxBytes:    tx2,
		RxRateSec:  rxRate,
		TxRateSec:  txRate,
		Interfaces: ifs2,
	}, nil
}

func CollectLoadAndUptime() ([3]float64, uint64, error) {
	var loads [3]float64
	data, err := os.ReadFile("/proc/loadavg")
	if err == nil {
		f := strings.Fields(string(data))
		if len(f) >= 3 {
			loads[0], _ = strconv.ParseFloat(f[0], 64)
			loads[1], _ = strconv.ParseFloat(f[1], 64)
			loads[2], _ = strconv.ParseFloat(f[2], 64)
		}
	}

	var uptime uint64
	upData, err := os.ReadFile("/proc/uptime")
	if err == nil {
		f := strings.Fields(string(upData))
		if len(f) >= 1 {
			u, _ := strconv.ParseFloat(f[0], 64)
			uptime = uint64(u)
		}
	}

	return loads, uptime, nil
}

func CollectAllStats(sampleInterval time.Duration) (SystemStats, error) {
	if sampleInterval <= 0 {
		sampleInterval = 200 * time.Millisecond
	}

	type instantRes struct {
		loads     [3]float64
		uptime    uint64
		mem       MemStats
		swap      SwapStats
		disks     []DiskUsage
		gpus      []GPUStats
		batteries []BatteryStats
	}

	instChan := make(chan instantRes, 1)
	go func() {
		loads, uptime, _ := CollectLoadAndUptime()
		mem, swap, _ := CollectMemoryStats()
		disks, _ := CollectDiskUsages()
		gpus := CollectGPUStats()
		batteries := CollectBatteryStats()
		instChan <- instantRes{
			loads:     loads,
			uptime:    uptime,
			mem:       mem,
			swap:      swap,
			disks:     disks,
			gpus:      gpus,
			batteries: batteries,
		}
	}()

	// Sample delta-based stats concurrently over sampleInterval
	type cpuRes struct {
		stats CPUStats
		err   error
	}
	type netRes struct {
		stats NetworkStats
		err   error
	}
	type diskIORes struct {
		stats DiskIOStats
		err   error
	}

	cChan := make(chan cpuRes, 1)
	nChan := make(chan netRes, 1)
	dChan := make(chan diskIORes, 1)

	go func() {
		s, err := CollectCPUStats(sampleInterval)
		cChan <- cpuRes{stats: s, err: err}
	}()

	go func() {
		s, err := CollectNetworkStats(sampleInterval)
		nChan <- netRes{stats: s, err: err}
	}()

	go func() {
		s, err := CollectDiskIOStats(sampleInterval)
		dChan <- diskIORes{stats: s, err: err}
	}()

	inst := <-instChan
	cr := <-cChan
	nr := <-nChan
	dr := <-dChan

	if cr.err != nil {
		return SystemStats{}, fmt.Errorf("cpu collect: %w", cr.err)
	}

	return SystemStats{
		CPU:        cr.stats,
		Memory:     inst.mem,
		Swap:       inst.swap,
		Network:    nr.stats,
		Disks:      inst.disks,
		DiskIO:     dr.stats,
		GPUs:       inst.gpus,
		Batteries:  inst.batteries,
		UptimeSecs: inst.uptime,
		LoadAvg:    inst.loads,
		Timestamp:  time.Now().Unix(),
	}, nil
}
