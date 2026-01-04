//go:build windows

package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const MaxLogs = 25000

// Regex to collapse multiple spaces into one
var spaceCollapser = regexp.MustCompile(`\s+`)

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

	// Logs since last boot
	startTime := `$StartTime = (Get-CimInstance -ClassName Win32_OperatingSystem).LastBootUpTime;`

	// PowerShell Command - use FilterHashtable for speed
	psCmd := fmt.Sprintf(
		`%s `+
			`Get-WinEvent -FilterHashTable @{LogName='System','Application'; Level=(%s); StartTime=$StartTime} -ErrorAction SilentlyContinue | `+
			`Select-Object TimeCreated, LevelDisplayName, Message, @{N='ProviderName';E={$_.ProviderName}}, Id | `+
			`ConvertTo-Json -Compress`,
		startTime,
		levels,
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

		msg := formatWindowsMessage(e.Message)

		results = append(results, protocol.LogEntry{
			Timestamp:   timestamp,
			Source:      sourceBuilder.String(),
			Level:       mapWinLevel(e.LevelDisplayName),
			Message:     msg,
			ProcessID:   e.Id,
			ProcessName: e.ProviderName,
		})
	}

	slices.Reverse(results)

	if len(results) > MaxLogs {
		results = results[len(results)-MaxLogs:]
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

// formatWindowsMessage makes Windows logs look like concise Linux logs
func formatWindowsMessage(raw string) string {
	if idx := strings.Index(raw, "Context:"); idx != -1 {
		raw = raw[:idx]
	}
	if idx := strings.Index(raw, "Operation:"); idx != -1 {
		raw = raw[:idx]
	}

	// Collapse all whitespace into a single space
	flat := spaceCollapser.ReplaceAllString(raw, " ")

	return strings.TrimSpace(flat)
}
