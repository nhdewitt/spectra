//go:build windows

package collector

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/windows"
)

// winProcessState tracks state for CPU % calculation
type winProcessState struct {
	LastTime   time.Time
	LastKernel uint64
	LastUser   uint64
}

// Global map to store previous CPU times per PID
var lastWinProcessStates = make(map[uint32]winProcessState)

// schedSummary is the desired data from NtQuerySystemInformation.
type schedSummary struct {
	Status          protocol.ProcStatus
	ThreadsTotal    uint32
	ThreadsRunning  uint32
	ThreadsRunnable uint32
	ThreadsWaiting  uint32
}

func CollectProcesses(ctx context.Context) ([]protocol.Metric, error) {
	// Grab scheduler summaries (thread counts + status)
	sched, err := getProcessSchedulerSummary()
	if err != nil {
		sched = nil
	}

	// Get Total System Memory
	var memStatus memoryStatusEx
	memStatus.Length = uint32(unsafe.Sizeof(memStatus))
	procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	totalMem := float64(memStatus.TotalPhys)

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	var pe32 windows.ProcessEntry32
	pe32.Size = uint32(unsafe.Sizeof(pe32))

	if err := windows.Process32First(snapshot, &pe32); err != nil {
		return nil, nil
	}

	var results []protocol.ProcessMetric
	currentStates := make(map[uint32]winProcessState)
	now := nowFunc()

	for {
		pid := pe32.ProcessID

		// Default to Mem 0/CPU 0% if the process can't be read
		memRSS := uint64(0)
		cpuPercent := 0.0

		// Get Memory Usage
		hProcess, err := windows.OpenProcess(
			windows.PROCESS_QUERY_LIMITED_INFORMATION,
			false,
			pid,
		)
		if err == nil {
			var memCounters processMemoryCounters
			memCounters.CB = uint32(unsafe.Sizeof(memCounters))

			r1, _, _ := procGetProcessMemoryInfo.Call(
				uintptr(hProcess),
				uintptr(unsafe.Pointer(&memCounters)),
				uintptr(memCounters.CB),
			)

			if r1 != 0 {
				memRSS = uint64(memCounters.WorkingSetSize)
			}

			// Get CPU Usage
			var create, exit, kernel, user windows.Filetime
			errTimes := windows.GetProcessTimes(hProcess, &create, &exit, &kernel, &user)

			if errTimes == nil {
				kTime := uint64(kernel.HighDateTime)<<32 + uint64(kernel.LowDateTime)
				uTime := uint64(user.HighDateTime)<<32 + uint64(user.LowDateTime)

				if prevState, ok := lastWinProcessStates[pid]; ok {
					deltaSys := kTime - prevState.LastKernel
					deltaUser := uTime - prevState.LastUser
					deltaTotal := deltaSys + deltaUser
					dt := now.Sub(prevState.LastTime).Seconds()

					if dt > 0 {
						secondsUsed := float64(deltaTotal) / 10000000.0
						cpuPercent = (secondsUsed / dt) * 100.0
					}
				}

				currentStates[pid] = winProcessState{
					LastTime:   now,
					LastKernel: kTime,
					LastUser:   uTime,
				}
			}
			windows.CloseHandle(hProcess)
		}

		memPercent := 0.0
		if totalMem > 0 {
			memPercent = (float64(memRSS) / totalMem) * 100.0
		}

		name := windows.UTF16ToString(pe32.ExeFile[:])

		var status protocol.ProcStatus = protocol.ProcOther
		var threadsTotal uint32
		var threadsRunning, threadsRunnable, threadsWaiting *uint32

		if sched != nil {
			if s, ok := sched[pid]; ok {
				status = s.Status
				threadsTotal = s.ThreadsTotal
				tr := s.ThreadsRunning
				tu := s.ThreadsRunnable
				tw := s.ThreadsWaiting
				threadsRunning = &tr
				threadsRunnable = &tu
				threadsWaiting = &tw
			}
		}

		results = append(results, protocol.ProcessMetric{
			Pid:             int(pid),
			Name:            name,
			MemRSS:          memRSS,
			MemPercent:      memPercent,
			CPUPercent:      cpuPercent,
			Status:          status,
			ThreadsTotal:    threadsTotal,
			ThreadsRunning:  threadsRunning,
			ThreadsRunnable: threadsRunnable,
			ThreadsWaiting:  threadsWaiting,
		})

		if err := windows.Process32Next(snapshot, &pe32); err != nil {
			break
		}
	}

	lastWinProcessStates = currentStates
	return []protocol.Metric{
		protocol.ProcessListMetric{Processes: results},
	}, nil
}

func getProcessSchedulerSummary() (map[uint32]schedSummary, error) {
	bufSize := uint32(1024 * 1024)
	buf := make([]byte, bufSize)
	var returnLen uint32

	for {
		ret, _, _ := procNtQuerySystemInformation.Call(
			uintptr(SystemProcessInformation),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(bufSize),
			uintptr(unsafe.Pointer(&returnLen)),
		)

		if ret == 0 {
			break
		}
		if ret == statusInfoLengthMismatch {
			bufSize *= 2
			buf = make([]byte, bufSize)
			continue
		}
		return nil, fmt.Errorf("NtQuerySystemInformation failed: %v", ret)
	}

	out := make(map[uint32]schedSummary)
	offset := uint32(0)

	for {
		proc := (*systemProcessInformation)(unsafe.Pointer(&buf[offset]))
		pid := uint32(proc.UniqueProcessId)

		// Thread state check
		threadOffset := offset + uint32(unsafe.Sizeof(*proc))

		total := proc.NumberOfThreads
		var running, runnable, waiting uint32

		for i := uint32(0); i < proc.NumberOfThreads; i++ {
			thread := (*systemThreadInformation)(unsafe.Pointer(&buf[threadOffset]))
			s := ProcessState(thread.ThreadState)

			switch s {
			case StateRunning:
				running++
			case StateReady, StateStandby, StateDeferredReady:
				runnable++
			case StateWaiting, StateGateWaitObsolete, StateWaitingForProcessInSwap:
				waiting++
			default:
				// initialized, transition, terminated -> ignore
			}

			threadOffset += uint32(unsafe.Sizeof(*thread))
		}

		var status protocol.ProcStatus = protocol.ProcOther
		switch {
		case total > 0 && running > 0:
			status = protocol.ProcRunning
		case total > 0 && runnable > 0:
			status = protocol.ProcRunnable
		case total > 0 && waiting > 0:
			status = protocol.ProcWaiting
		}

		out[pid] = schedSummary{
			Status:          status,
			ThreadsTotal:    total,
			ThreadsRunning:  running,
			ThreadsRunnable: runnable,
			ThreadsWaiting:  waiting,
		}

		if proc.NextEntryOffset == 0 {
			break
		}
		offset += proc.NextEntryOffset
	}

	return out, nil
}
