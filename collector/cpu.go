package collector

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nhdewitt/raspimon/metrics"
)

var (
	// Package-level state for delta calculation
	lastCPURawData map[string]CPURaw
	lastCPURunTime time.Time
)

type CPURaw struct {
	User      uint64
	Nice      uint64
	System    uint64
	Idle      uint64
	IOWait    uint64
	IRQ       uint64
	SoftIRQ   uint64
	Steal     uint64
	Guest     uint64
	GuestNice uint64
}

type CPUDelta struct {
	User      uint64
	Nice      uint64
	System    uint64
	Idle      uint64
	IOWait    uint64
	IRQ       uint64
	SoftIRQ   uint64
	Steal     uint64
	Guest     uint64
	GuestNice uint64
	Total     uint64 // Sum of all time
	Used      uint64 // Total - Idle - IOWait
}

func CollectCPU(ctx context.Context) ([]metrics.Metric, error) {
	cur, err := parseProcStat()
	if err != nil {
		return nil, fmt.Errorf("parsing /proc/stat: %w", err)
	}

	now := time.Now()

	// First sample - store and skip
	if len(lastCPURawData) == 0 {
		lastCPURawData = cur
		lastCPURunTime = now
		return nil, nil
	}

	deltaMap, ok := calculateDelta(cur, lastCPURawData)
	if !ok {
		lastCPURawData = nil
		lastCPURunTime = now
		return nil, nil
	}
	lastCPURawData = cur
	lastCPURunTime = now

	usage := percent(deltaMap["cpu"].Used, deltaMap["cpu"].Total)
	coreUsage := calcCoreUsage(deltaMap)

	load1, load5, load15, err := parseLoadAvg()
	if err != nil {
		return nil, fmt.Errorf("parsing /proc/loadavg: %w", err)
	}

	return []metrics.Metric{metrics.CPUMetric{
		Usage:     usage,
		CoreUsage: coreUsage,
		LoadAvg1:  load1,
		LoadAvg5:  load5,
		LoadAvg15: load15,
	}}, nil
}

// calculateDelta takes the current and previous raw maps and returns a map containing
// the delta for each key (cpu, cpu0, ...)
func calculateDelta(current, previous map[string]CPURaw) (map[string]CPUDelta, bool) {
	deltaMap := make(map[string]CPUDelta)

	for key, cur := range current {
		prev, ok := previous[key]
		if !ok {
			return nil, false
		}

		if cur.User < prev.User || cur.Nice < prev.Nice || cur.System < prev.System || cur.Idle < prev.Idle || cur.IOWait < prev.IOWait ||
			cur.IRQ < prev.IRQ || cur.SoftIRQ < prev.SoftIRQ || cur.Steal < prev.Steal {
			return nil, false
		}

		delta := CPUDelta{}

		delta.User = cur.User - prev.User
		delta.Nice = cur.Nice - prev.Nice
		delta.System = cur.System - prev.System
		delta.Idle = cur.Idle - prev.Idle
		delta.IOWait = cur.IOWait - prev.IOWait
		delta.IRQ = cur.IRQ - prev.IRQ
		delta.SoftIRQ = cur.SoftIRQ - prev.SoftIRQ
		delta.Steal = cur.Steal - prev.Steal
		delta.Guest = cur.Guest - prev.Guest
		delta.GuestNice = cur.GuestNice - prev.GuestNice
		delta.Total = delta.User + delta.Nice + delta.System + delta.Idle + delta.IOWait + delta.IRQ + delta.SoftIRQ + delta.Steal
		delta.Used = delta.Total - (delta.Idle + delta.IOWait)

		deltaMap[key] = delta
	}

	return deltaMap, true
}

func parseProcStat() (map[string]CPURaw, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := make(map[string]CPURaw)
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu") {
			break
		}

		raw, err := parseCPULine(line)
		if err != nil {
			continue
		}

		fields := strings.Fields(line)
		result[fields[0]] = raw
	}

	return result, scanner.Err()
}

func parseCPULine(line string) (CPURaw, error) {
	fields := strings.Fields(line)
	if len(fields) < 11 {
		return CPURaw{}, fmt.Errorf("insufficient fields: %d", len(fields))
	}

	parse := func(index int) uint64 {
		v, err := strconv.ParseUint(fields[index], 10, 64)
		if err != nil {
			log.Printf("error parsing /proc/stat field[%d] = %q: %v", index, fields[index], err)
			return 0
		}
		return v
	}

	return CPURaw{
		User:      parse(1),
		Nice:      parse(2),
		System:    parse(3),
		Idle:      parse(4),
		IOWait:    parse(5),
		IRQ:       parse(6),
		SoftIRQ:   parse(7),
		Steal:     parse(8),
		Guest:     parse(9),
		GuestNice: parse(10),
	}, nil
}

func calcCoreUsage(deltaMap map[string]CPUDelta) []float64 {
	numCores := len(deltaMap) - 1 // exclude aggregate "cpu"
	usage := make([]float64, numCores)

	for i := range numCores {
		coreKey := fmt.Sprintf("cpu%d", i)
		if delta, ok := deltaMap[coreKey]; ok && delta.Total > 0 {
			usage[i] = percent(delta.Used, delta.Total)
		}
	}

	return usage
}

func parseLoadAvg() (load1, load5, load15 float64, err error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("insufficient fields: %d", len(fields))
	}

	load1, err = strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parsing load1: %w", err)
	}

	load5, err = strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parsing load5: %w", err)
	}

	load15, err = strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parsing load15: %w", err)
	}

	return load1, load5, load15, nil
}
