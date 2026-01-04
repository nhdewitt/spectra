//go:build windows

package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// winEvent matches PowerShell's JSON output
type winEvent struct {
	TimeCreated      string `json:"TimeCreated"`
	LevelDisplayName string `json:"LevelDisplayName"`
	Message          string `json:"Message"`
	ProviderName     string `json:"ProviderName"`
	Id               int    `json:"Id"`
}

func FetchLogs(ctx context.Context, opts protocol.LogRequest) ([]protocol.LogEntry, error) {
	levels := getWindowsLevelFlag(opts.MinLevel)

	// PowerShell Command - use FilterHashtable for speed
	psCmd := fmt.Sprintf(
		`$StartTime = (Get-Date "1970-01-01 00:00:00Z").AddSeconds(%d); `+
			`Get-WinEvent -FilterHashtable @{LogName='System','Application'; Level=(%s); StartTime=$StartTime} -MaxEvents %d -ErrorAction SilentlyContinue | `+
			`Select-Object TimeCreated, LevelDisplayName, Message, @{N='ProviderName';E={$_.ProviderName}}, Id | `+
			`ConvertTo-Json -Compress`,
		opts.Since,
		levels,
		opts.Lines,
	)

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", psCmd)

	out, err := cmd.Output()
	if err != nil {
		// Exit Code 1 if no events found - ignore
		return nil, nil
	}

	data := strings.TrimSpace(string(out))
	if data == "" {
		return nil, nil
	}

	// Handle PowerShell returning an array for multiple results or a single object for 1 result
	var events []winEvent
	if strings.HasPrefix(data, "[") {
		_ = json.Unmarshal([]byte(data), &events)
	} else {
		var single winEvent
		if err := json.Unmarshal([]byte(data), &single); err == nil {
			events = append(events, single)
		}
	}

	var results []protocol.LogEntry
	var sourceBuilder strings.Builder
	sourceBuilder.Grow(64)

	for _, e := range events {
		sourceBuilder.Reset()
		sourceBuilder.WriteString("WinEvent:")
		if e.ProviderName != "" {
			sourceBuilder.WriteString(e.ProviderName)
		} else {
			sourceBuilder.WriteString("Unknown")
		}

		timestamp := parseWinDate(e.TimeCreated)
		if timestamp == 0 {
			continue
		}

		results = append(results, protocol.LogEntry{
			Timestamp:   timestamp,
			Source:      sourceBuilder.String(),
			Level:       mapWinLevel(e.LevelDisplayName),
			Message:     e.Message,
			ProcessID:   e.Id,
			ProcessName: e.ProviderName,
		})
	}

	return results, nil
}

func getWindowsLevelFlag(min protocol.LogLevel) string {
	// 1=Critical, 2=Error, 3=Warning, 4=Info
	switch min {
	case protocol.LevelEmergency, protocol.LevelAlert, protocol.LevelCritical:
		return "1"
	case protocol.LevelError:
		return "1,2"
	case protocol.LevelWarning:
		return "1,2,3"
	case protocol.LevelNotice, protocol.LevelInfo, protocol.LevelDebug:
		return "1,2,3,4"
	default:
		return "1,2"
	}
}

func mapWinLevel(l string) protocol.LogLevel {
	switch l {
	case "Critical":
		return protocol.LevelCritical
	case "Error":
		return protocol.LevelError
	case "Warning":
		return protocol.LevelWarning
	case "Information":
		return protocol.LevelInfo
	default:
		return protocol.LevelInfo
	}
}

// parseWinDate converts "/Date(x)/" to Unix Seconds
func parseWinDate(raw string) int64 {
	s := strings.TrimPrefix(raw, "/Date(")
	s = strings.TrimSuffix(s, ")/")

	if val, err := strconv.ParseInt(s, 10, 64); err == nil {
		return val / 1000
	}

	return 0
}
