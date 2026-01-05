//go:build windows

package collector

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"unicode/utf16"

	"github.com/nhdewitt/spectra/internal/protocol"
)

type winService struct {
	Name        string `json:"Name"`
	DisplayName string `json:"DisplayName"`
	State       string `json:"State"`
	StartMode   string `json:"StartMode"`
	Description string `json:"Description"`
}

func CollectServices(ctx context.Context) ([]protocol.Metric, error) {
	psCmd := `
			[Console]::OutputEncoding = [System.Text.Encoding]::UTF8;
			Get-CimInstance Win32_Service | 
			Select-Object Name, DisplayName, State, StartMode, Description | 
			ForEach-Object { $_ | ConvertTo-Json -Compress; "" }
	`

	encoded := encodePowerShell(psCmd)

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-EncodedCommand", encoded)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("powershell error: %w", err)
	}

	metrics := make([]protocol.Metric, 0, 256)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	var descriptionBuilder strings.Builder
	descriptionBuilder.Grow(128)

	for scanner.Scan() {
		descriptionBuilder.Reset()

		b := bytes.TrimSpace(scanner.Bytes())
		if len(b) == 0 {
			continue
		}

		var s winService
		if err := json.Unmarshal(b, &s); err != nil {
			continue
		}

		descriptionBuilder.WriteString(s.DisplayName)
		if s.Description != "" && s.Description != s.DisplayName {
			descriptionBuilder.WriteString(" - ")
			descriptionBuilder.WriteString(s.Description)
		}

		var loadState string
		switch s.StartMode {
		case "Disabled":
			loadState = "disabled"
		default:
			loadState = "loaded"
		}

		metrics = append(metrics, &protocol.ServiceMetric{
			Name:        s.Name,
			Status:      s.State,
			SubStatus:   s.StartMode,
			LoadState:   loadState,
			Description: descriptionBuilder.String(),
		})
	}

	return metrics, nil
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
