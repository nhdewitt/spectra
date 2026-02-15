//go:build windows

package collector

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/windows/registry"
)

const psScript = `$s = New-Object -ComObject Microsoft.Update.Session
$r = $s.CreateUpdateSearcher().Search('IsInstalled=0')
foreach ($u in $r.Updates) {
    $sec = 'false'
    foreach ($c in $u.Categories) {
        if ($c.Name -eq 'Security Updates') { $sec = 'true'; break }
    }
    '{0}|{1}|{2}' -f $u.Title, ($u.KBArticleIDs -join ','), $sec
}`

func CollectUpdates(ctx context.Context) ([]protocol.Metric, error) {
	ps := findPowerShell()
	if ps == "" {
		return nil, fmt.Errorf("no powershell found")
	}
	cmd := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-Command", psScript)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	updates, err := parseWindowsUpdates(bytes.NewReader(out))
	if err != nil {
		return nil, err
	}

	secCount := 0
	for _, u := range updates {
		if u.Security {
			secCount++
		}
	}

	return []protocol.Metric{protocol.UpdateMetric{
		PendingCount:   len(updates),
		SecurityCount:  secCount,
		RebootRequired: checkRebootRequired(),
		PackageManager: "windows_update",
		Packages:       updates,
	}}, nil
}

func parseWindowsUpdates(r io.Reader) (updates []protocol.PendingUpdate, err error) {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format: Title|KBArticleIDs|IsSecurity
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}

		isSec, _ := strconv.ParseBool(parts[2])

		updates = append(updates, protocol.PendingUpdate{
			Name:     parts[0],
			Version:  parts[1],
			Security: isSec,
		})
	}

	return updates, scanner.Err()
}

func findPowerShell() string {
	if path, err := exec.LookPath("pwsh"); err == nil {
		return path
	}
	if path, err := exec.LookPath("powershell"); err == nil {
		return path
	}
	return ""
}

func checkRebootRequired() bool {
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return false
	}
	key.Close()
	return true
}
