// Package secret provides authenticated symmetric encryption for secrets that
// must be stored and later recovered in plaintext (i.e. SMTP passwords or
// third-party API tokens persisted in the database).
//
// Encryption is AES-256-GCM. The 32-byte key is loaded once from the
// SPECTRA_SECRET_KEY environment variable (base64-standard-encoded).
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// KeyEnvVar is the environment variable holding the base64-encoded 32-byte key.
const KeyEnvVar = "SPECTRA_SECRET_KEY"

const keySize = 32 // AES-256

// scheme prefixes every encrypted value, namespaced and versioned so that
// algorithm or key-rotation changes can add new schemes while old values
// remain decryptable. The prefix also makes an encrypted blob self-identifying
// if it appears outside its column.
const scheme = "spectra.v1"

// ErrNoKey is returned by NewFromEnv when the key variable is unset or empty.
var ErrNoKey = errors.New("secret: " + KeyEnvVar + " is not set")

// Cipher encrypts and decrypts values using AES-256-GCM with a fixed key.
type Cipher struct {
	aead cipher.AEAD
}

// New builds a Cipher from a raw 32-byte key. The key is copied, so the caller
// may reuse or zero its slice afterward without affecting the Cipher.
func New(key []byte) (*Cipher, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("secret: key must be %d bytes, got %d", keySize, len(key))
	}
	k := make([]byte, keySize)
	copy(k, key)
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, fmt.Errorf("secret: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: new GCM: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// NewFromEnv builds a Cipher from the base64-encoded key in SPECTRA_SECRET_KEY.
// Returns ErrNoKey if the variable is unset, so callers can distinguish "no key
// configured" (acceptable when the dependent feature is disabled) from "key is
// present but malformed" (a hard configuration error).
func NewFromEnv() (*Cipher, error) {
	raw := os.Getenv(KeyEnvVar)
	if raw == "" {
		return nil, ErrNoKey
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("secret: decode %s: %w", KeyEnvVar, err)
	}
	return New(key)
}

// Encrypt encrypts plaintext and returns:
//
//	spectra.v1:base64(nonce||ciphertext||tag)
//
// suitable for storage in a TEXT column.
func (c *Cipher) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("secret: nonce: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, plaintext, nil)
	return scheme + ":" + base64.StdEncoding.EncodeToString(sealed), nil
}

// EncryptString is a convenience wrapper around Encrypt for string plaintext.
func (c *Cipher) EncryptString(plaintext string) (string, error) {
	return c.Encrypt([]byte(plaintext))
}

// Decrypt reverses Encrypt, returning the original plaintext bytes. The input
// must carry a recognized scheme prefix; values without one are rejected.
func (c *Cipher) Decrypt(encoded string) ([]byte, error) {
	prefix, b64, ok := strings.Cut(encoded, ":")
	if !ok {
		return nil, errors.New("secret: missing scheme prefix")
	}
	if prefix != scheme {
		return nil, fmt.Errorf("secret: unknown scheme %q", prefix)
	}

	sealed, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("secret: decode ciphertext: %w", err)
	}
	ns := c.aead.NonceSize()
	if len(sealed) < ns+c.aead.Overhead() {
		return nil, errors.New("secret: ciphertext too short")
	}
	nonce, ct := sealed[:ns], sealed[ns:]
	plaintext, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("secret: decrypt: %w", err)
	}
	return plaintext, nil
}

// DecryptString is a convenience wrapper around Decrypt returning a string.
func (c *Cipher) DecryptString(encoded string) (string, error) {
	b, err := c.Decrypt(encoded)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
