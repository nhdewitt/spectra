//go:build windows

package collector

import (
	"context"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/metrics"
)

func TestCountQUserLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name: "Single User",
			input: `
USERNAME			SESSIONNAME			ID	STATE	IDLE TIME	LOGON TIME
nhdewitt			console				1	Active	none		1/2/2026 8:00AM
`,
			want: 1,
		},
		{
			name: "Multiple Users",
			input: `
USERNAME			SESSIONNAME			ID	STATE	IDLE TIME	LOGON TIME
admin				console				1	Active	none		1/2/2026 8:00AM
guest				rdp-tcp#0			2	Active	none		1/2/2026 9:00AM
`,
			want: 2,
		},
		{
			name:  "Empty Output",
			input: "",
			want:  0,
		},
		{
			name: "Header Only",
			input: `
USERNAME			SESSIONNAME			ID	STATE	IDLE TIME	LOGON TIME
`,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countQUserLines([]byte(tt.input))
			if got != tt.want {
				t.Errorf("countQUserLines() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCollectSystem_Integration(t *testing.T) {
	data, err := CollectSystem(context.Background())
	if err != nil {
		t.Fatalf("CollectSystem failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Returned no metrics")
	}

	m, ok := data[0].(metrics.SystemMetric)
	if !ok {
		t.Fatalf("Expected metrics.SystemMetric, got %T", data[0])
	}

	t.Logf("Uptime: %d seconds", m.Uptime)
	t.Logf("BootTime: %d (Unix)", m.BootTime)
	t.Logf("Processes: %d", m.Processes)
	t.Logf("Users: %d", m.Users)

	if m.Uptime == 0 {
		t.Error("Uptime reported as 0")
	}
	if m.Processes == 0 {
		t.Error("Process count reported as 0")
	}

	now := uint64(time.Now().Unix())
	expectedBoot := now - m.Uptime

	diff := int64(expectedBoot) - int64(m.BootTime)
	if diff < -2 || diff > 2 {
		t.Errorf("BootTime calculation seems off: Got %d, Expected ~%d (Diff: %d)", m.BootTime, expectedBoot, diff)
	}
}
