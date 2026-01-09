package protocol

import (
	"encoding/json"
	"time"
)

type LogLevel string

const (
	LevelDebug     LogLevel = "DEBUG"     // Syslog 7
	LevelInfo      LogLevel = "INFO"      // Syslog 6
	LevelNotice    LogLevel = "NOTICE"    // Syslog 5
	LevelWarning   LogLevel = "WARNING"   // Syslog 4
	LevelError     LogLevel = "ERROR"     // Syslog 3
	LevelCritical  LogLevel = "CRITICAL"  // Syslog 2
	LevelAlert     LogLevel = "ALERT"     // Syslog 1
	LevelEmergency LogLevel = "EMERGENCY" // Syslog 0
)

var PriorityToLevel = map[int]LogLevel{
	0: LevelEmergency,
	1: LevelAlert,
	2: LevelCritical,
	3: LevelError,
	4: LevelWarning,
	5: LevelNotice,
	6: LevelInfo,
	7: LevelDebug,
}

type LogEntry struct {
	Timestamp   int64    `json:"timestamp"`
	Source      string   `json:"source"`
	Level       LogLevel `json:"level"`
	Message     string   `json:"message"`
	ProcessID   int      `json:"pid,omitempty"`
	ProcessName string   `json:"process_name,omitempty"`
}

type CommandType string

const (
	CmdFetchLogs    CommandType = "FETCH_LOGS"
	CmdDiskUsage    CommandType = "DISK_USAGE"
	CmdRestartAgent CommandType = "RESTART_AGENT"
	CmdListMounts   CommandType = "LIST_MOUNTS"
)

type Command struct {
	ID      string      `json:"id"`
	Type    CommandType `json:"type"`
	Payload []byte      `json:"payload"`
}

// CommandResult is the response to a Command sent from the server.
type CommandResult struct {
	ID      string          `json:"id"`   // Command.ID
	Type    CommandType     `json:"type"` // Command.Type
	Payload json.RawMessage `json:"payload"`
	Error   string          `json:"error,omitempty"`
}

type LogRequest struct {
	MinLevel LogLevel `json:"min_level"`
}

type ServiceMetric struct {
	Name        string `json:"name"`
	Status      string `json:"status"`     // "active", "inactive", "failed"
	SubStatus   string `json:"sub_status"` // "running", "exited", "dead"
	LoadState   string `json:"load_state"` // "loaded", "not-found"
	Description string `json:"description"`
}

func (m ServiceMetric) MetricType() string {
	return "service"
}

type ServiceListMetric struct {
	Services []ServiceMetric `json:"services"`
}

func (m ServiceListMetric) MetricType() string {
	return "service_list"
}

// TopEntry represents a single file or directory in the usage report
type TopEntry struct {
	Path  string `json:"path"`
	Size  uint64 `json:"size"`
	Count uint64 `json:"count,omitempty"` // Only relevant for directories
}

type DiskUsageTopReport struct {
	Root         string     `json:"root"`
	TopDirs      []TopEntry `json:"top_dirs"`  // immediate subdirs of root, sorted desc by size then name
	TopFiles     []TopEntry `json:"top_files"` // top N largest files anywhere in tree
	ScannedDirs  uint64     `json:"scanned_dirs"`
	ScannedFiles uint64     `json:"scanned_files"`
	ErrorCount   uint64     `json:"error_count"`
	Partial      bool       `json:"partial"`
	DurationMs   int64      `json:"duration_ms"`
	ScannedAt    time.Time  `json:"scanned_at"`
}

type DiskUsageRequest struct {
	Path string `json:"path"`  // If empty, return list of mounts from DriveCache
	TopN int    `json:"top_n"` // Default to 50 if 0
}

// MountInfo is the universal structure sent to the server.
// It normalizes data from both Windows and Linux collectors.
type MountInfo struct {
	Mountpoint string `json:"mountpoint"` // "C:", "/"
	Device     string `json:"device"`     // "Samsung SSD 970", "/dev/sda1"
	FSType     string `json:"fstype"`     // "NTFS", "ext4"
}
