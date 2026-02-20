//go:build freebsd

package collector

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"golang.org/x/sys/unix"
)

// kinfoProc mirrors the first 600 bytes of FreeBSD's struct kinfo_proc
// (sys/user.h) on amd64, covering through ki_numthreads.
//
// Offset map of exported fields:
// 0 StructSize
// 4 Layout
// 72 Pid
// 265 Rssize
// 308 Pctcpu
// 328 Runtime
// 388 Stat
// 447 Comm
// 596 NumThreads
type kinfoProc struct {
	StructSize int32      // 0: ki_structsize
	Layout     int32      // 4: ki_layout
	_          [8]uint64  // 8: ki_args..ki_wchan (8 pointers)
	Pid        int32      // 72: ki_pid
	_          [5]int32   // 76: ki_ppid..ki_tsid
	_          [2]int16   // 96: ki_jobc, ki_spare_short1
	_          uint32     // 100: ki_tdev_freebsd11
	_          [16]uint32 // 104: siglist+sigmask+sigignore+sigcatch (4*sigset_t)
	_          [5]uint32  // 168: ki_uid..ki_svgid
	_          [2]int16   // 188: ki_ngroups, ki_spare_short2
	_          [16]uint32 // 192: ki_groups[KI_NGROUPS=16]
	_          uint64     // 256: ki_size
	Rssize     int64      // 265: ki_rssize
	_          [4]int64   // 272: ki_swrss..ki_ssize
	_          [2]uint16  // 304: ki_xstat, ki_acflag
	Pctcpu     uint32     // 308: ki_pctcpu
	_          [4]uint32  // 312: ki_estcpu..ki_cow
	Runtime    uint64     // 328: ki_runtime
	_          [4]int64   // 336: ki_start, ki_childtime (2*timeval)
	_          [2]int64   // 368: ki_flag, ki_kiflag
	_          int32      // 384: ki_traceflag
	Stat       int8       // 388: ki_stat
	_          [3]int8    // 389: ki_nice, ki_lock, ki_reindex
	_          [2]uint8   // 392: ki_oncpu_old, ki_lastcpu_old
	_          [17]byte   // 394: ki_tdname[TDNAMELEN+1=17]
	_          [9]byte    // 411: ki_wmesg[WMESGLEN+1=9]
	_          [18]byte   // 420: ki_login[LOGNAMELEN+1=18]
	_          [9]byte    // 438: ki_lockname[LOCKNAMELEN+1=9]
	Comm       [20]byte   // 447: ki_comm[COMMLEN+1=20]
	_          [17]byte   // 476: ki_emul[KI_EMULNAMELEN+1=17]
	_          [18]byte   // 484: ki_loginclass[LOGINCLASSLEN+1=18]
	_          [4]byte    // 502: ki_moretdname[MAXCOMLEN-TDNAMELEN+1=4]
	_          [46]byte   // 506: ki_sparestrings[46]
	_          [2]int32   // 552: ki_spareints[KI_NSPARE_INT=2]
	_          uint64     // 560: ki_tdev
	_          [2]int32   // 568: ki_oncpu, ki_lastcpu
	_          [4]int32   // 576: ki_tracer, ki_flag2, ki_fibnum, ki_cr_flags
	_          int32      // 592: ki_jid
	NumThreads int32      // 596: ki_numthreads
} // total: 600 bytes

// kinfoSize is the number of bytes binary.Read consumes per kinfoProc.
// This covers the kinfo_proc layout through ki_numthreads (offset 596).
// The full C struct is larger; the remainder is skipped using ki_structsize.
const kinfoSize = 600

var clkTck = 1_000_000.0 // ki_runtime is in microseconds

func collectProcessRaw() ([]processRaw, int64, error) {
	// Get total memory for RSS percentage calc
	physmem, err := unix.SysctlUint64("hw.physmem")
	if err != nil {
		return nil, 0, fmt.Errorf("hw.physmem: %w", err)
	}

	// Page size to convert ki_rssize (pages) to bytes
	pageSize, err := unix.SysctlUint32("hw.pagesize")
	if err != nil {
		pageSize = 4096
	}

	buf, err := unix.SysctlRaw("kern.proc.proc", 0)
	if err != nil {
		return nil, 0, fmt.Errorf("kern.proc.proc: %w", err)
	}

	reader := bytes.NewReader(buf)
	var procs []processRaw

	for reader.Len() >= kinfoSize {
		var kp kinfoProc
		if err := binary.Read(reader, binary.LittleEndian, &kp); err != nil {
			break
		}

		// Skip remaining bytes
		if skip := int64(kp.StructSize) - kinfoSize; skip > 0 {
			reader.Seek(skip, io.SeekCurrent)
		}

		procs = append(procs, processRaw{
			PID:        int(kp.Pid),
			Name:       unix.ByteSliceToString(kp.Comm[:]),
			State:      statToString(kp.Stat),
			RSSBytes:   uint64(kp.Rssize) * uint64(pageSize),
			TotalTicks: uint64(kp.Runtime),
			NumThreads: uint32(kp.NumThreads),
		})
	}

	return procs, int64(physmem), nil
}

func statToString(stat int8) string {
	// sys/proc.h:
	// SIDL = 1 (process being created)
	// SRUN = 2 (process currently runnable)
	// SSLEEP = 3 (process sleeping)
	// SSTOP = 4 (process suspended)
	// SZOMB = 5 (process awaiting collection)
	// SWAIT = 6 (process waiting for interrupt)
	// SLOCK = 7 (process blocked on a lock)
	switch stat {
	case 2: // SRUN
		return "R"
	case 1, 3, 6, 7: //SIDL, SSLEEP, SWAIT, SLOCK
		return "S"
	case 4: // SSTOP
		return "T"
	case 5: // SZOMB
		return "Z"
	default:
		return "?"
	}
}
