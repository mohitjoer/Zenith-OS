package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

type ProcessInfo struct {
	PID         int     `json:"pid"`
	PPID        int     `json:"ppid"`
	Name        string  `json:"name"`
	Cmdline     string  `json:"cmdline"`
	User        string  `json:"user"`
	UID         int     `json:"uid"`
	State       string  `json:"state"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryBytes uint64  `json:"memoryBytes"`
	MemoryPct   float64 `json:"memoryPercent"`
	Threads     int     `json:"threads"`
}

var uidCache = make(map[int]string)

func usernameFromUID(uid int) string {
	if name, ok := uidCache[uid]; ok {
		return name
	}
	u, err := user.LookupId(strconv.Itoa(uid))
	if err == nil && u != nil && u.Username != "" {
		uidCache[uid] = u.Username
		return u.Username
	}
	name := strconv.Itoa(uid)
	uidCache[uid] = name
	return name
}

func readProcess(pid int, totalMem uint64) (*ProcessInfo, error) {
	procDir := fmt.Sprintf("/proc/%d", pid)

	// Stat file: pid (comm) state ppid ...
	statData, err := os.ReadFile(filepath.Join(procDir, "stat"))
	if err != nil {
		return nil, err
	}

	str := string(statData)
	openParen := strings.Index(str, "(")
	closeParen := strings.LastIndex(str, ")")
	if openParen == -1 || closeParen == -1 || closeParen <= openParen {
		return nil, fmt.Errorf("corrupt stat for %d", pid)
	}

	name := str[openParen+1 : closeParen]
	rest := strings.Fields(str[closeParen+1:])
	if len(rest) < 18 {
		return nil, fmt.Errorf("short stat for %d", pid)
	}

	state := rest[0]
	ppid, _ := strconv.Atoi(rest[1])
	threads, _ := strconv.Atoi(rest[17])

	// Read status for UID and VmRSS
	statusData, err := os.ReadFile(filepath.Join(procDir, "status"))
	if err != nil {
		return nil, err
	}

	var uid int
	var rssBytes uint64
	sc := bufio.NewScanner(bytes.NewReader(statusData))
	for sc.Scan() {
		l := sc.Text()
		if strings.HasPrefix(l, "Uid:") {
			f := strings.Fields(l)
			if len(f) > 1 {
				uid, _ = strconv.Atoi(f[1])
			}
		} else if strings.HasPrefix(l, "VmRSS:") {
			f := strings.Fields(l)
			if len(f) > 1 {
				v, _ := strconv.ParseUint(f[1], 10, 64)
				rssBytes = v * 1024
			}
		}
	}

	// Read cmdline
	cmdData, _ := os.ReadFile(filepath.Join(procDir, "cmdline"))
	cmdline := strings.ReplaceAll(string(cmdData), "\x00", " ")
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "" {
		cmdline = "[" + name + "]"
	}

	var memPct float64
	if totalMem > 0 {
		memPct = float64(rssBytes) / float64(totalMem) * 100.0
	}

	return &ProcessInfo{
		PID:         pid,
		PPID:        ppid,
		Name:        name,
		Cmdline:     cmdline,
		User:        usernameFromUID(uid),
		UID:         uid,
		State:       state,
		CPUPercent:  0.0, // populated when delta is sampled or from top
		MemoryBytes: rssBytes,
		MemoryPct:   memPct,
		Threads:     threads,
	}, nil
}

type procCPUSample struct {
	utime uint64
	stime uint64
}

func readProcCPUSample(pid int) (procCPUSample, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return procCPUSample{}, err
	}
	str := string(data)
	closeParen := strings.LastIndex(str, ")")
	if closeParen == -1 {
		return procCPUSample{}, fmt.Errorf("invalid stat")
	}
	fields := strings.Fields(str[closeParen+1:])
	if len(fields) < 13 {
		return procCPUSample{}, fmt.Errorf("short stat")
	}
	utime, _ := strconv.ParseUint(fields[11], 10, 64)
	stime, _ := strconv.ParseUint(fields[12], 10, 64)
	return procCPUSample{utime: utime, stime: stime}, nil
}

func ListProcesses(sortBy string, limit int) ([]ProcessInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	mem, _, _ := CollectMemoryStats()
	totalMem := mem.TotalBytes

	var pids []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}

	var procs []ProcessInfo
	for _, pid := range pids {
		pi, err := readProcess(pid, totalMem)
		if err == nil && pi != nil {
			procs = append(procs, *pi)
		}
	}

	switch strings.ToLower(sortBy) {
	case "mem", "memory":
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].MemoryBytes > procs[j].MemoryBytes
		})
	case "name":
		sort.Slice(procs, func(i, j int) bool {
			return strings.ToLower(procs[i].Name) < strings.ToLower(procs[j].Name)
		})
	case "pid":
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].PID < procs[j].PID
		})
	default:
		// Default to memory or CPU
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].MemoryBytes > procs[j].MemoryBytes
		})
	}

	if limit > 0 && len(procs) > limit {
		procs = procs[:limit]
	}

	return procs, nil
}

func KillProcess(pid int, force bool) error {
	if pid <= 1 {
		return fmt.Errorf("refusing to terminate PID %d", pid)
	}

	// Safety protection for core desktop compositor/system daemons
	proc, err := readProcess(pid, 0)
	if err == nil && proc != nil {
		lower := strings.ToLower(proc.Name)
		if lower == "systemd" || lower == "init" || lower == "runit" {
			return fmt.Errorf("refusing to terminate core init process (%s)", proc.Name)
		}
	}

	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}

	return syscall.Kill(pid, sig)
}
