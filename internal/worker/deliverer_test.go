package worker

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestComputeHMAC(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		secret  string
	}{
		{
			name:    "basic payload",
			payload: []byte(`{"event":"order.created","data":{"id":"123"}}`),
			secret:  "my-secret-key",
		},
		{
			name:    "empty payload",
			payload: []byte(`{}`),
			secret:  "secret",
		},
		{
			name:    "empty secret",
			payload: []byte(`{"test":true}`),
			secret:  "",
		},
		{
			name:    "unicode payload",
			payload: []byte(`{"name":"café","price":"€10"}`),
			secret:  "unicode-key-日本語",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := computeHMAC(tt.payload, tt.secret)

			// Verify it's a valid hex string
			decoded, err := hex.DecodeString(sig)
			if err != nil {
				t.Fatalf("signature is not valid hex: %v", err)
			}

			// HMAC-SHA256 should always produce 32 bytes (64 hex chars)
			if len(decoded) != 32 {
				t.Fatalf("expected 32 bytes, got %d", len(decoded))
			}

			// Verify against standard library
			mac := hmac.New(sha256.New, []byte(tt.secret))
			mac.Write(tt.payload)
			expected := hex.EncodeToString(mac.Sum(nil))

			if sig != expected {
				t.Errorf("signature mismatch:\n  got:  %s\n  want: %s", sig, expected)
			}
		})
	}
}

func TestComputeHMAC_Deterministic(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	secret := "test-secret"

	sig1 := computeHMAC(payload, secret)
	sig2 := computeHMAC(payload, secret)

	if sig1 != sig2 {
		t.Error("HMAC should be deterministic — same input should produce same output")
	}
}

func TestComputeHMAC_DifferentSecrets(t *testing.T) {
	payload := []byte(`{"event":"test"}`)

	sig1 := computeHMAC(payload, "secret-1")
	sig2 := computeHMAC(payload, "secret-2")

	if sig1 == sig2 {
		t.Error("different secrets should produce different signatures")
	}
}

func TestComputeHMAC_DifferentPayloads(t *testing.T) {
	secret := "my-secret"

	sig1 := computeHMAC([]byte(`{"a":1}`), secret)
	sig2 := computeHMAC([]byte(`{"a":2}`), secret)

	if sig1 == sig2 {
		t.Error("different payloads should produce different signatures")
	}
}
