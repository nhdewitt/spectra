//go:build darwin && cgo

package cpu

/*
#include <mach/mach.h>
#include <mach/processor_info.h>
#include <mach/mach_host.h>

static mach_port_t get_mach_host_self() {
	return mach_host_self();
}
*/
import "C"

import (
	"context"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
)

var lastCPURawData map[string]CPURaw

func Collect(ctx context.Context) ([]protocol.Metric, error) {
	cur, err := readCPURaw()
	if err != nil {
		return nil, fmt.Errorf("reading cpu ticks: %v", err)
	}

	// First sample - store and skip
	if len(lastCPURawData) == 0 {
		lastCPURawData = cur
		return nil, nil
	}

	deltaMap, ok := calculateCPUDeltas(cur, lastCPURawData)
	if !ok {
		lastCPURawData = nil
		return nil, nil
	}
	lastCPURawData = cur

	usage := percent(deltaMap["cpu"].Used, deltaMap["cpu"].Total)
	coreUsage := calcCoreUsage(deltaMap)

	load1, load5, load15, err := parseLoadAvg()
	if err != nil {
		return nil, fmt.Errorf("parsing load averages: %v", err)
	}

	return []protocol.Metric{protocol.CPUMetric{
		Usage:     usage,
		CoreUsage: coreUsage,
		LoadAvg1:  load1,
		LoadAvg5:  load5,
		LoadAvg15: load15,
	}}, nil
}

// readCPURaw calls host_processor_info and returns per-core data
// plus a cpu aggregate, matching the Linux map layout.
func readCPURaw() (map[string]CPURaw, error) {
	var numCPU C.natural_t
	var cpuInfo C.processor_info_array_t
	var numCPUInfo C.mach_msg_type_number_t

	ret := C.host_processor_info(
		C.get_mach_host_self(),
		C.PROCESSOR_CPU_LOAD_INFO,
		&numCPU,
		&cpuInfo,
		&numCPUInfo,
	)
	if ret != C.KERN_SUCCESS {
		return nil, fmt.Errorf("host_processor_info failed: %d", ret)
	}
	defer C.vm_deallocate(
		C.mach_task_self_,
		C.vm_address_t(uintptr(unsafe.Pointer(cpuInfo))),
		C.vm_size_t(numCPUInfo)*C.vm_size_t(unsafe.Sizeof(C.integer_t(0))),
	)

	n := int(numCPU)
	if n == 0 {
		n = runtime.NumCPU()
	}

	info := unsafe.Slice((*C.integer_t)(unsafe.Pointer(cpuInfo)), int(numCPUInfo))

	result := make(map[string]CPURaw, n+1)
	var agg CPURaw

	for i := range n {
		base := i * C.CPU_STATE_MAX

		raw := CPURaw{
			User:   uint64(info[base+C.CPU_STATE_USER]),
			Nice:   uint64(info[base+C.CPU_STATE_NICE]),
			System: uint64(info[base+C.CPU_STATE_SYSTEM]),
			Idle:   uint64(info[base+C.CPU_STATE_IDLE]),
		}

		result[fmt.Sprintf("cpu%d", i)] = raw

		agg.User += raw.User
		agg.Nice += raw.Nice
		agg.System += raw.System
		agg.Idle += raw.Idle
	}

	result["cpu"] = agg

	return result, nil
}

func calculateCPUDeltas(current, previous map[string]CPURaw) (map[string]CPUDelta, bool) {
	deltaMap := make(map[string]CPUDelta)

	for key, cur := range current {
		prev, ok := previous[key]
		if !ok {
			return nil, false
		}

		if cur.User < prev.User || cur.Nice < prev.Nice || cur.System < prev.System || cur.Idle < prev.Idle ||
			cur.IOWait < prev.IOWait || cur.IRQ < prev.IRQ || cur.SoftIRQ < prev.SoftIRQ || cur.Steal < prev.Steal {
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

func calcCoreUsage(deltaMap map[string]CPUDelta) []float64 {
	numCores := len(deltaMap) - 1
	usage := make([]float64, numCores)

	for i := range numCores {
		coreKey := fmt.Sprintf("cpu%d", i)
		if delta, ok := deltaMap[coreKey]; ok && delta.Total > 0 {
			usage[i] = percent(delta.Used, delta.Total)
		}
	}

	return usage
}
