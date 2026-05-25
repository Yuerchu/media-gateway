package signing

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestSHA256Hex(t *testing.T) {
	tests := []struct {
		input []byte
		want  string
	}{
		{[]byte(""), "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{[]byte("hello"), "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
		{[]byte("media-gateway"), func() string {
			h := sha256.Sum256([]byte("media-gateway"))
			return hex.EncodeToString(h[:])
		}()},
	}
	for _, tt := range tests {
		got := SHA256Hex(tt.input)
		if got != tt.want {
			t.Errorf("SHA256Hex(%q) = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func TestSignPayload(t *testing.T) {
	sig := SignPayload([]byte("test-payload"), "secret-key")
	if sig == "" {
		t.Error("signature should not be empty")
	}
	if len(sig) != 64 {
		t.Errorf("HMAC-SHA256 hex should be 64 chars, got %d", len(sig))
	}

	sig2 := SignPayload([]byte("test-payload"), "secret-key")
	if sig != sig2 {
		t.Error("same input should produce same signature")
	}

	sig3 := SignPayload([]byte("different-payload"), "secret-key")
	if sig == sig3 {
		t.Error("different payloads should produce different signatures")
	}

	sig4 := SignPayload([]byte("test-payload"), "different-key")
	if sig == sig4 {
		t.Error("different keys should produce different signatures")
	}
}

func TestVerifyPayload(t *testing.T) {
	key := "my-secret"
	payload := []byte("verify-this")
	sig := SignPayload(payload, key)

	if !VerifyPayload(payload, key, sig) {
		t.Error("valid signature should verify")
	}

	if VerifyPayload(payload, key, "invalid-signature") {
		t.Error("invalid signature should not verify")
	}

	if VerifyPayload([]byte("wrong-payload"), key, sig) {
		t.Error("wrong payload should not verify")
	}

	if VerifyPayload(payload, "wrong-key", sig) {
		t.Error("wrong key should not verify")
	}
}

func TestSignRequest(t *testing.T) {
	key := "hmac-secret"
	sig := SignRequest("POST", "/api/v1/detect-mime", "1700000000", "abc123", key)
	if sig == "" {
		t.Error("request signature should not be empty")
	}

	sig2 := SignRequest("POST", "/api/v1/detect-mime", "1700000000", "abc123", key)
	if sig != sig2 {
		t.Error("same request params should produce same signature")
	}

	sig3 := SignRequest("GET", "/api/v1/detect-mime", "1700000000", "abc123", key)
	if sig == sig3 {
		t.Error("different method should produce different signature")
	}

	sig4 := SignRequest("POST", "/api/v1/image-meta", "1700000000", "abc123", key)
	if sig == sig4 {
		t.Error("different path should produce different signature")
	}

	sig5 := SignRequest("POST", "/api/v1/detect-mime", "1700000001", "abc123", key)
	if sig == sig5 {
		t.Error("different timestamp should produce different signature")
	}
}

func TestSignRequest_CanonicalForm(t *testing.T) {
	key := "test"
	method, path, ts, body := "POST", "/api", "12345", "bodyhash"
	sig := SignRequest(method, path, ts, body, key)
	expected := SignPayload([]byte("POST|/api|12345|bodyhash"), key)
	if sig != expected {
		t.Errorf("canonical form mismatch: got %s, want %s", sig, expected)
	}
}
