package labels

import (
	"errors"
	"strings"
	"testing"
)

func TestIsReservedKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"os", true},
		{"arch", true},
		{"hardware", true},
		{"agent_version", true},
		{"env", false},
		{"OS", false}, // case-sensitive
		{"", false},
		{"agent_versionx", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := IsReservedKey(tt.key); got != tt.want {
				t.Errorf("IsReservedKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name, key string
		wantErr   error
	}{
		{"simple", "env", nil},
		{"with_underscore", "my_label", nil},
		{"with_digits", "tier2", nil},
		{"max_length", "a" + strings.Repeat("b", 62), nil},
		{"empty", "", ErrInvalidKey},
		{"uppercase", "Env", ErrInvalidKey},
		{"leading_digit", "1env", ErrInvalidKey},
		{"leading_underscore", "_env", ErrInvalidKey},
		{"hyphen", "my-label", ErrInvalidKey},
		{"too_long", "a" + strings.Repeat("b", 63), ErrInvalidKey},
		{"space", "my label", ErrInvalidKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateKey(%q) = %v, want nil", tt.key, err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateKey(%q) = %v, want %v", tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestValidateValue(t *testing.T) {
	tests := []struct {
		name, value string
		wantErr     error
	}{
		{"simple", "prod", nil},
		{"with_spaces", "production east", nil},
		{"unicode", "café", nil},
		{"max_length", strings.Repeat("a", MaxValueLen), nil},
		{"empty", "", ErrEmptyValue},
		{"too_long", strings.Repeat("a", MaxValueLen+1), ErrValueTooLong},
		{"invalid_utf8", "\xff\xfe", ErrInvalidValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateValue(tt.value)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateValue(%q) = %v, want nil", tt.value, err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateValue(%q) = %v, want %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUserLabel(t *testing.T) {
	tests := []struct {
		name, key, value string
		wantErr          error
	}{
		{"valid", "env", "prod", nil},
		{"reserved_key", "os", "linux", ErrReservedKey},
		{"reserved_key_beats_invalid_value", "os", "", ErrReservedKey},
		{"invalid_key", "Env", "prod", ErrInvalidKey},
		{"empty_value", "env", "", ErrEmptyValue},
		{"value_too_long", "env", strings.Repeat("a", MaxValueLen+1), ErrValueTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserLabel(tt.key, tt.value)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateUserLabel(%q, %q) = %v, want nil", tt.key, tt.value, err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateUserLabel(%q, %q) = %v, want %v", tt.key, tt.value, err, tt.wantErr)
			}
		})
	}
}
