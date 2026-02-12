//go:build !windows

package collector

import (
	"strings"
	"testing"
)

func TestPlatformInfoFrom(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedID      string
		expectedVersion string
	}{
		{
			name: "Ubuntu Standard",
			input: `PRETTY_NAME="Ubuntu 22.04.1 LTS"
NAME="Ubuntu"
VERSION_ID="22.04"
VERSION="22.04.1 LTS (Jammy Jellyfish)"
ID=ubuntu`,
			expectedID:      "ubuntu",
			expectedVersion: "22.04",
		},
		{
			name: "Alpine (Unquoted)",
			input: `NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.17.1
PRETTY_NAME="Alpine Linux v3.17"`,
			expectedID:      "alpine",
			expectedVersion: "3.17.1",
		},
		{
			name: "CentOS (Quoted ID, Unquoted Version)",
			input: `NAME="CentOS Stream"
ID="centos"
VERSION_ID=9`,
			expectedID:      "centos",
			expectedVersion: "9",
		},
		{
			name:            "Missing Data",
			input:           `X=123`,
			expectedID:      "linux", // Default
			expectedVersion: "",
		},
		{
			name: "Partial Match (ID only)",
			input: `ID=debian
OTHER=123`,
			expectedID:      "debian",
			expectedVersion: "",
		},
		{
			name:            "Empty File",
			input:           "",
			expectedID:      "linux",
			expectedVersion: "",
		},
		{
			name:            "ID= with empty value",
			input:           "ID=\nVERSION_ID=1.0",
			expectedID:      "linux",
			expectedVersion: "1.0",
		},
		{
			name:            "Whitespace in values",
			input:           "ID=  ubuntu  \nVERSION_ID=\"  22.04  \"",
			expectedID:      "ubuntu",
			expectedVersion: "22.04",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			id, ver := getPlatformInfoFrom(r)

			if id != tt.expectedID {
				t.Errorf("expected ID %q, got %q", tt.expectedID, id)
			}
			if ver != tt.expectedVersion {
				t.Errorf("expected Version %q, got %q", tt.expectedVersion, ver)
			}
		})
	}
}

func TestGetCPUModelFrom(t *testing.T) {
	input := `
processor	: 0
vendor_id	: GenuineIntel
cpu family	: 6
model		: 142
model name	: Intel(R) Core(TM) i7-8550U CPU @ 1.80GHz
stepping	: 10
microcode	: 0xf0`
	expected := "Intel(R) Core(TM) i7-8550U CPU @ 1.80GHz"

	r := strings.NewReader(input)
	got := getCPUModelFrom(r)

	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestGetCPUModelFrom_Empty(t *testing.T) {
	r := strings.NewReader("")
	got := getCPUModelFrom(r)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestGetCPUModelFrom_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "model name without colon",
			input:    "model name Intel CPU",
			expected: "",
		},
		{
			name:     "model name with empty value",
			input:    "model name\t:",
			expected: "",
		},
		{
			name:     "multiple model names (returns first)",
			input:    "model name\t: CPU A\nmodel name\t: CPU B",
			expected: "CPU A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getCPUModelFrom(strings.NewReader(tt.input))
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestGetBootTimeFrom(t *testing.T) {
	input := `
cpu	446109 950 148507 7247783 5966 0 3804 0 0 0
intr 13732152 0 0 0 0 0 0 0 0...
ctxt 23783935
btime 1676648834
processes 38246`

	var expected int64 = 1676648834

	r := strings.NewReader(input)
	got := getBootTimeFrom(r)

	if got != expected {
		t.Errorf("expected boot time %d, got %d", expected, got)
	}
}

func TestGetBootTimeFrom_Empty(t *testing.T) {
	r := strings.NewReader("")
	got := getBootTimeFrom(r)
	if got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestGetBootTimeFrom_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			name:     "btime at start of file",
			input:    "btime 1676648834\ncpu 123 456",
			expected: 1676648834,
		},
		{
			name:     "btime with extra whitespace",
			input:    "btime   1676648834",
			expected: 1676648834,
		},
		{
			name:     "malformed btime",
			input:    "btime notanumber",
			expected: 0,
		},
		{
			name:     "btime missing value",
			input:    "btime ",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBootTimeFrom(strings.NewReader(tt.input))
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestCharsToString(t *testing.T) {
	// int8 (standard C char)
	t.Run("int8_buffer", func(t *testing.T) {
		// "Hello" + NUL + "Junk"
		input := []int8{72, 101, 108, 108, 111, 0, 87, 111, 114, 108, 100}
		expected := "Hello"
		got := charsToString(input)
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	// uint8 (unsigned char)
	t.Run("uint8_buffer", func(t *testing.T) {
		// "Hello, World!" + NUL
		input := []uint8{72, 101, 108, 108, 111, 44, 32, 87, 111, 114, 108, 100, 33, 0}
		expected := "Hello, World!"
		got := charsToString(input)
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("empty_buffer", func(t *testing.T) {
		input := []int8{}
		expected := ""
		got := charsToString(input)
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("leading_nul", func(t *testing.T) {
		input := []int8{0, 72, 101, 108, 108, 111}
		expected := ""
		got := charsToString(input)
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("all_nuls", func(t *testing.T) {
		input := []int8{0, 0, 0, 0}
		expected := ""
		got := charsToString(input)
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func BenchmarkGetPlatformInfoFrom(b *testing.B) {
	input := `
PRETTY_NAME="Ubuntu 22.04.1 LTS"
NAME="Ubuntu"
VERSION_ID="22.04"
ID=ubuntu
`

	b.ResetTimer()
	for b.Loop() {
		r := strings.NewReader(input)
		getPlatformInfoFrom(r)
	}
}

func BenchmarkGetCPUModelFrom(b *testing.B) {
	input := `
vendor_id	: GenuineIntel
model name	: Intel(R) Core(TM) i7-8550U CPU @ 1.80GHz
`

	b.ResetTimer()
	for b.Loop() {
		r := strings.NewReader(input)
		getCPUModelFrom(r)
	}
}
