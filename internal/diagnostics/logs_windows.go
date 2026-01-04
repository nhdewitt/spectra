//go:build windows

package diagnostics

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

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
	ProcessId        int    `json:"ProcessId"`
}

func FetchLogs(ctx context.Context, opts protocol.LogRequest) ([]protocol.LogEntry, error) {
	levels := getWindowsLevelFlag(opts.MinLevel)

	psCmd := fmt.Sprintf(
		`$StartTime = (Get-CimInstance Win32_OperatingSystem).LastBootUpTime;
		Get-WinEvent -FilterHashTable @{LogName='System','Application'; Level=(%s); StartTime=$StartTime} -MaxEvents %d -ErrorAction SilentlyContinue -Oldest |
		Select-Object TimeCreated, LevelDisplayName, Message,
			@{N='ProviderName';E={$_.ProviderName}},
			@{N='ProcessId';E={$_.ProcessId}} |
		ForEach-Object { $_ | ConvertTo-Json -Compress; "" }`,
		levels,
		MaxLogs,
	)

	encoded := encodePowerShell(psCmd)

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-EncodedCommand", encoded)

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Printf("PowerShell Error: %s\n", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("powershell execution failed: %w", err)
	}

	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}

	results := make([]protocol.LogEntry, 0, 1024)

	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var sourceBuilder strings.Builder
	sourceBuilder.Grow(64)

	for scanner.Scan() {
		b := bytes.TrimSpace(scanner.Bytes())
		if len(b) == 0 {
			continue
		}

		var e winEvent
		if err := json.Unmarshal(b, &e); err != nil {
			continue
		}

		timestamp := parseWinDate(e.TimeCreated)
		if timestamp == 0 {
			continue
		}

		sourceBuilder.Reset()
		sourceBuilder.WriteString("WinEvent:")
		if e.ProviderName != "" {
			sourceBuilder.WriteString(e.ProviderName)
		} else {
			sourceBuilder.WriteString("Unknown")
		}

		results = append(results, protocol.LogEntry{
			Timestamp: timestamp,
			Source:    sourceBuilder.String(),
			Level:     mapWinLevel(e.LevelDisplayName),
			Message:   formatWindowsMessage(e.Message),
			ProcessID: e.ProcessId,
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

func encodePowerShell(cmd string) string {
	utf16Chars := utf16.Encode([]rune(cmd))

	buf := make([]byte, len(utf16Chars)*2)
	for i, c := range utf16Chars {
		buf[i*2] = byte(c)
		buf[i*2+1] = byte(c >> 8)
	}

	return base64.StdEncoding.EncodeToString(buf)
}
