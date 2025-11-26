package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nhdewitt/raspimon/metrics"
)

// Package-level state for delta calculation
var lastCPURawData map[string]CPURaw

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
	currentRaw, err := parseProcStat()
	if err != nil {
		return nil, fmt.Errorf("parsing /proc/stat: %w", err)
	}

	// First sample - store and skip
	if len(lastCPURawData) == 0 {
		lastCPURawData = currentRaw
		return nil, nil
	}

	deltaMap := calculateDelta(currentRaw, lastCPURawData)
	lastCPURawData = currentRaw

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
func calculateDelta(current, previous map[string]CPURaw) map[string]CPUDelta {
	deltaMap := make(map[string]CPUDelta)

	for key, currentRaw := range current {
		previousRaw, ok := previous[key]
		if !ok {
			continue
		}

		delta := CPUDelta{}

		delta.User = currentRaw.User - previousRaw.User
		delta.Nice = currentRaw.Nice - previousRaw.Nice
		delta.System = currentRaw.System - previousRaw.System
		delta.Idle = currentRaw.Idle - previousRaw.Idle
		delta.IOWait = currentRaw.IOWait - previousRaw.IOWait
		delta.IRQ = currentRaw.IRQ - previousRaw.IRQ
		delta.SoftIRQ = currentRaw.SoftIRQ - previousRaw.SoftIRQ
		delta.Steal = currentRaw.Steal - previousRaw.Steal
		delta.Guest = currentRaw.Guest - previousRaw.Guest
		delta.Total = delta.User + delta.Nice + delta.System + delta.Idle + delta.IOWait + delta.IRQ + delta.SoftIRQ + delta.Steal + delta.Guest
		delta.Used = delta.Total - (delta.Idle + delta.IOWait)

		deltaMap[key] = delta
	}

	return deltaMap
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
	if len(fields) < 10 {
		return CPURaw{}, fmt.Errorf("insufficient fields: %d", len(fields))
	}

	parse := func(s string) uint64 {
		v, _ := strconv.ParseUint(s, 10, 64)
		return v
	}

	return CPURaw{
		User:    parse(fields[1]),
		Nice:    parse(fields[2]),
		System:  parse(fields[3]),
		Idle:    parse(fields[4]),
		IOWait:  parse(fields[5]),
		IRQ:     parse(fields[6]),
		SoftIRQ: parse(fields[7]),
		Steal:   parse(fields[8]),
		Guest:   parse(fields[9]),
	}, nil
}

func calcCoreUsage(deltaMap map[string]CPUDelta) []float64 {
	numCores := len(deltaMap) - 1 // exclude aggregate "cpu"
	usage := make([]float64, numCores)

	for i := range numCores {
		coreKey := fmt.Sprintf("cpu%d", i)
		if delta, ok := deltaMap[coreKey]; ok {
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
