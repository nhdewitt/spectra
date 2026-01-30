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
	"time"
	"unicode/utf16"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/windows"
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

var (
	kernel32           = windows.NewLazySystemDLL("kernel32.dll")
	procGetTickCount64 = kernel32.NewProc("GetTickCount64")
)

func FetchLogs(ctx context.Context, opts protocol.LogRequest) ([]protocol.LogEntry, error) {
	levels := getWindowsLevelFlag(opts.MinLevel)

	bootTime := getBootTime().Format(time.RFC3339)

	xpathQuery := fmt.Sprintf(
		`*[System[(Level=%s) and TimeCreated[@SystemTime>='%s']]]`,
		strings.ReplaceAll(levels, ",", " or Level="),
		bootTime,
	)

	psCmd := fmt.Sprintf(
		`$ProgressPreference = 'SilentlyContinue';
		[Console]::OutputEncoding = [System.Text.Encoding]::UTF8;
		$query = "%s";
		Get-WinEvent -LogName @('System','Application') -FilterXPath $query -MaxEvents %d -ErrorAction SilentlyContinue -Oldest |
		Select-Object TimeCreated, LevelDisplayName, Message, ProviderName, ProcessId |
		ForEach-Object { $_ | ConvertTo-Json -Compress }`,
		xpathQuery,
		MaxLogs,
	)

	encoded := encodePowerShell(psCmd)

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-EncodedCommand", encoded)

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			// Ignore progress/module loading messages
			if strings.Contains(stderr, "Preparing modules") || strings.Contains(stderr, "CLIXML") {
				// Continue processing stdout if it exists
				if len(bytes.TrimSpace(out)) > 0 {
					err = nil
				} else {
					return nil, nil
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("powershell execution failed: %w", err)
		}
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
			Timestamp:   timestamp,
			Source:      sourceBuilder.String(),
			Level:       mapWinLevel(e.LevelDisplayName),
			Message:     formatWindowsMessage(e.Message),
			ProcessID:   e.ProcessId,
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
	if !strings.HasPrefix(raw, "/Date(") || !strings.HasSuffix(raw, ")/") {
		return 0
	}
	s := raw[6 : len(raw)-2] // Extract the number

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

func getBootTime() time.Time {
	ret, _, _ := procGetTickCount64.Call()
	tickCount := uint64(ret)
	return time.Now().Add(time.Duration(-tickCount) * time.Millisecond)
}
