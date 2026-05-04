package sysusage

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var processStart = time.Now()
var lastSample usageSample

type Snapshot struct {
	CPUPercent float64
	MemoryMB   float64
}

type usageSample struct {
	at         time.Time
	cpuSec     float64
	cpuPercent float64
	memoryMB   float64
}

func Current() Snapshot {
	var usage syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &usage)
	cpuSeconds := timevalSeconds(usage.Utime) + timevalSeconds(usage.Stime)
	now := time.Now()
	if !lastSample.at.IsZero() && now.Sub(lastSample.at) < time.Second {
		return Snapshot{CPUPercent: lastSample.cpuPercent, MemoryMB: lastSample.memoryMB}
	}

	cpuPercent := sampleCPUPercent(now, cpuSeconds)
	memoryMB := currentRSSMB()
	lastSample = usageSample{at: now, cpuSec: cpuSeconds, cpuPercent: cpuPercent, memoryMB: memoryMB}
	return Snapshot{
		CPUPercent: cpuPercent,
		MemoryMB:   memoryMB,
	}
}

func FooterText() string {
	s := Current()
	return fmt.Sprintf("CPU:%.1f%% Mem:%.0fMB", s.CPUPercent, s.MemoryMB)
}

func timevalSeconds(tv syscall.Timeval) float64 {
	return float64(tv.Sec) + float64(tv.Usec)/1_000_000
}

func sampleCPUPercent(now time.Time, cpuSeconds float64) float64 {
	if lastSample.at.IsZero() {
		elapsed := now.Sub(processStart).Seconds()
		if elapsed <= 0 {
			return 0
		}
		return (cpuSeconds / elapsed) * 100 / float64(runtime.NumCPU())
	}
	elapsed := now.Sub(lastSample.at).Seconds()
	if elapsed <= 0 {
		return lastSample.cpuPercent
	}
	cpuDelta := cpuSeconds - lastSample.cpuSec
	if cpuDelta < 0 {
		return 0
	}
	return (cpuDelta / elapsed) * 100 / float64(runtime.NumCPU())
}

func currentRSSMB() float64 {
	if rss, ok := rssFromPS(); ok {
		return rss
	}
	if rss, ok := rssFromRusage(); ok {
		return rss
	}
	return 0
}

func rssFromPS() (float64, bool) {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		return 0, false
	}
	kb, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, false
	}
	return kb / 1024, true
}

func rssFromRusage() (float64, bool) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, false
	}
	if usage.Maxrss <= 0 {
		return 0, false
	}
	if runtime.GOOS == "darwin" {
		return float64(usage.Maxrss) / 1024 / 1024, true
	}
	return float64(usage.Maxrss) / 1024, true
}
