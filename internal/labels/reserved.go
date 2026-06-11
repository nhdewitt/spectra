// Package labels validates and computes per-agent key/value metadata.
//
// Two sources:
//   - "auto" labels are derived from agent info (os, arch, platform, version)
//     and managed by the server. ComputeAutoLabels produces them.
//   - "user" labels are admin-managed via the API. ValidateUserLabel enforces
//     the naming rules.
//
// The reserved-key set is the contract between auto and user labels:
// ComputeAutoLabels must only emit keys in ReservedKeys, and user writes
// to ReservedKeys are rejected. TestAutoLabelKeysAreReserved guards drift.
package labels

import (
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"
)

// Sentinel errors for user-label validation. API handlers check these with
// errors.Is to produce appropriate HTTP responses (403 for reserved, 400
// for invalid input).
var (
	ErrReservedKey  = errors.New("label key is reserved for auto labels")
	ErrInvalidKey   = errors.New("label key must match [a-z][a-z0-9_]{0,62}")
	ErrEmptyValue   = errors.New("label value cannot be empty")
	ErrInvalidValue = errors.New("label value must be valid UTF-8")
	ErrValueTooLong = errors.New("label value exceeds maximum length")
)

// MaxValueLen caps user-label values. Auto labels are not subject to this
// limit (they come from controlled sources: GOOS, GOARCH, version strings).
const MaxValueLen = 255

// ReservedKeys lists keys produced by ComputeAutoLabels. User writes to
// these keys are rejected by ValidateUserLabel. Adding a key to
// ComputeAutoLabels requires adding it here too - TestAutoLabelKeysAreReserved
// enforces this at test time.
var ReservedKeys = map[string]struct{}{
	"os":            {},
	"arch":          {},
	"hardware":      {},
	"agent_version": {},
}

// IsReservedKey reports whether key is in the auto-label namespace.
func IsReservedKey(key string) bool {
	_, ok := ReservedKeys[key]
	return ok
}

// keyPattern matches lowercase label keys, capped at 63 chars
// (1 leading alpha + up to 62 alphanumeric/underscore).
var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// ValidateKey checks key against the naming rules. Does not check whether
// the key is reserved - use ValidateUserLabel for user-write validation.
func ValidateKey(key string) error {
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	return nil
}

// ValidateValue checks value for non-emptiness, valid UTF-8, and length.
func ValidateValue(value string) error {
	if value == "" {
		return ErrEmptyValue
	}
	if !utf8.ValidString(value) {
		return ErrInvalidValue
	}
	if len(value) > MaxValueLen {
		return fmt.Errorf("%w: %d chars (max %d)", ErrValueTooLong, len(value), MaxValueLen)
	}
	return nil
}

// ValidateUserLabel checks that a (key, value) pairs is valid to write to
// agent_labels with source='user'. Combines the reserved-key check, key
// pattern check, and value validation; reserved-key check runs first so
// callers get the most specific error.
func ValidateUserLabel(key, value string) error {
	if IsReservedKey(key) {
		return fmt.Errorf("%w: %q", ErrReservedKey, key)
	}
	if err := ValidateKey(key); err != nil {
		return err
	}
	return ValidateValue(value)
}
