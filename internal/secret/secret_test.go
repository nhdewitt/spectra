package secret

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	c, err := New(testKey(t))
	if err != nil {
		t.Fatal(err)
	}

	cases := []string{
		"",
		"hunter2",
		"a-very-long-SES-smtp-password-with-symbols-!@#$%^&*()",
		strings.Repeat("x", 4096),
	}
	for _, pt := range cases {
		enc, err := c.EncryptString(pt)
		if err != nil {
			t.Fatalf("encrypt %q: %v", pt, err)
		}
		got, err := c.DecryptString(enc)
		if err != nil {
			t.Fatalf("decrypt %q: %v", pt, err)
		}
		if got != pt {
			t.Errorf("round-trip mismatch: got %q, want %q", got, pt)
		}
	}
}

func TestEncrypt_NonceIsRandom(t *testing.T) {
	c, _ := New(testKey(t))
	a, _ := c.EncryptString("same")
	b, _ := c.EncryptString("same")
	if a == b {
		t.Error("two encryptions of the same plaintext produced identical ciphertext; nonce not random")
	}
}

func TestDecrypt_WrongKeyFails(t *testing.T) {
	c1, _ := New(testKey(t))
	c2, _ := New(testKey(t))

	enc, _ := c1.EncryptString("secret")
	if _, err := c2.DecryptString(enc); err == nil {
		t.Error("decrypt with wrong key should fail, got nil error")
	}
}

func TestDecrypt_TamperedCiphertextFails(t *testing.T) {
	c, _ := New(testKey(t))
	enc, _ := c.EncryptString("secret")

	// Split off the scheme prefix, tamper the ciphertext, reassemble.
	prefix, b64, _ := strings.Cut(enc, ":")
	raw, _ := base64.StdEncoding.DecodeString(b64)
	raw[len(raw)-1] ^= 0xFF // flip bits in the GCM tag
	tampered := prefix + ":" + base64.StdEncoding.EncodeToString(raw)

	if _, err := c.DecryptString(tampered); err == nil {
		t.Error("decrypt of tampered ciphertext should fail authentication, got nil error")
	}
}

func TestDecrypt_GarbageInput(t *testing.T) {
	c, _ := New(testKey(t))

	if _, err := c.DecryptString("spectra.v1:not-valid-base64!!!"); err == nil {
		t.Error("expected error for non-base64 payload")
	}
	short := scheme + ":" + base64.StdEncoding.EncodeToString([]byte("short"))
	if _, err := c.DecryptString(short); err == nil {
		t.Error("expected error for ciphertext shorter than nonce")
	}
}

func TestDecrypt_SchemePrefix(t *testing.T) {
	c, _ := New(testKey(t))
	enc, _ := c.EncryptString("data")

	if !strings.HasPrefix(enc, "spectra.v1:") {
		t.Errorf("encrypted value missing scheme prefix: %q", enc)
	}

	// Missing prefix.
	if _, err := c.DecryptString("dGVzdA=="); err == nil {
		t.Error("expected error for value with no scheme prefix")
	}
	// Unknown scheme.
	_, b64, _ := strings.Cut(enc, ":")
	if _, err := c.DecryptString("spectra.v2:" + b64); err == nil {
		t.Error("expected error for unknown scheme version")
	}
}

func TestNew_RejectsWrongKeySize(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33, 64} {
		if _, err := New(make([]byte, n)); err == nil {
			t.Errorf("New accepted %d-byte key, want error", n)
		}
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Run("unset returns ErrNoKey", func(t *testing.T) {
		t.Setenv(KeyEnvVar, "")
		if _, err := NewFromEnv(); err != ErrNoKey {
			t.Errorf("got %v, want ErrNoKey", err)
		}
	})

	t.Run("malformed base64 errors", func(t *testing.T) {
		t.Setenv(KeyEnvVar, "!!!not base64!!!")
		if _, err := NewFromEnv(); err == nil || err == ErrNoKey {
			t.Errorf("got %v, want a decode error", err)
		}
	})

	t.Run("wrong length errors", func(t *testing.T) {
		t.Setenv(KeyEnvVar, base64.StdEncoding.EncodeToString([]byte("tooshort")))
		if _, err := NewFromEnv(); err == nil {
			t.Error("expected error for short key")
		}
	})

	t.Run("valid key works", func(t *testing.T) {
		t.Setenv(KeyEnvVar, base64.StdEncoding.EncodeToString(testKey(t)))
		c, err := NewFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		enc, _ := c.EncryptString("ok")
		got, _ := c.DecryptString(enc)
		if got != "ok" {
			t.Errorf("round-trip failed: %q", got)
		}
	})
}

func TestEncrypt_OutputFormat(t *testing.T) {
	c, _ := New(testKey(t))
	enc, _ := c.EncryptString("data")

	prefix, b64, ok := strings.Cut(enc, ":")
	if !ok || prefix != "spectra.v1" {
		t.Fatalf("expected spectra.v1 prefix, got %q", enc)
	}
	if _, err := base64.StdEncoding.DecodeString(b64); err != nil {
		t.Errorf("payload after prefix is not valid base64: %v", err)
	}
	if strings.Contains(b64, "data") {
		t.Error("payload appears to contain plaintext")
	}
}
