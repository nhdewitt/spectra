package protocol

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
	CmdRestartAgent CommandType = "RESTART_AGENT"
)

type Command struct {
	ID      string      `json:"id"`
	Type    CommandType `json:"type"`
	Payload []byte      `json:"payload"`
}

type LogRequest struct {
	Lines    int      `json:"lines"`
	MinLevel LogLevel `json:"min_level"`
	Since    int64    `json:"since"` // Unix Timestamp (seconds)
}
