//go:build linux

package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePiModel(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		wantIsPi  bool
		wantModel string
	}{
		{
			name:      "Pi4NulTerminated",
			data:      "Raspberry Pi 4 Model B Rev 1.4\x00",
			wantIsPi:  true,
			wantModel: "Raspberry Pi 4 Model B Rev 1.4",
		},
		{
			name:      "Pi5",
			data:      "Raspberry Pi 5 Model B Rev 1.0\x00",
			wantIsPi:  true,
			wantModel: "Raspberry Pi 5 Model B Rev 1.0",
		},
		{
			name:      "TrailingNewlineAndNul",
			data:      "Raspberry Pi Zero 2 W Rev 1.0\n\x00",
			wantIsPi:  true,
			wantModel: "Raspberry Pi Zero 2 W Rev 1.0",
		},
		{
			name:      "NonPiDeviceTree",
			data:      "Radxa ROCK 5B\x00",
			wantIsPi:  false,
			wantModel: "",
		},
		{
			name:      "Empty",
			data:      "",
			wantIsPi:  false,
			wantModel: "",
		},
		{
			name:      "OnlyNul",
			data:      "\x00",
			wantIsPi:  false,
			wantModel: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isPi, model := parsePiModel([]byte(tt.data))
			if isPi != tt.wantIsPi {
				t.Errorf("isPi = %v, want %v", isPi, tt.wantIsPi)
			}
			if model != tt.wantModel {
				t.Errorf("model = %q, want %q", model, tt.wantModel)
			}
		})
	}
}

// A non-Pi must report an empty model, not the model string it did find.
// Detect() writes PiModel unconditionally, so a leaked value would label
// every Radxa and Odroid in the fleet with its own board name under a Pi
// field.
func TestParsePiModel_NonPiLeaksNoModel(t *testing.T) {
	if _, model := parsePiModel([]byte("Radxa ROCK 5B\x00")); model != "" {
		t.Errorf("model = %q, want empty", model)
	}
}

func TestDetectPi_MissingFile(t *testing.T) {
	t.Cleanup(func(orig string) func() {
		return func() { piModelPath = orig }
	}(piModelPath))

	piModelPath = filepath.Join(t.TempDir(), "does-not-exist")

	isPi, model := detectPi()
	if isPi || model != "" {
		t.Errorf("got (%v, %q), want (false, \"\")", isPi, model)
	}
}

func TestDetectPi_ReadsFixture(t *testing.T) {
	t.Cleanup(func(orig string) func() {
		return func() { piModelPath = orig }
	}(piModelPath))

	path := filepath.Join(t.TempDir(), "model")
	if err := os.WriteFile(path, []byte("Raspberry Pi 4 Model B Rev 1.4\x00"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	piModelPath = path

	isPi, model := detectPi()
	if !isPi {
		t.Error("expected a Pi")
	}
	if model != "Raspberry Pi 4 Model B Rev 1.4" {
		t.Errorf("model = %q", model)
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "present")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	if !fileExists(path) {
		t.Error("expected true for an existing file")
	}
	if !fileExists(dir) {
		t.Error("expected true for a directory: detectInitSystem relies on this for /etc/init.d")
	}
	if fileExists(filepath.Join(dir, "absent")) {
		t.Error("expected false for a missing path")
	}
}
