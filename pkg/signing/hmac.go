package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// SignPayload computes an HMAC-SHA256 signature over the given payload.
func SignPayload(payload []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyPayload checks that the provided signature matches the HMAC-SHA256
// of the payload with the given key.
func VerifyPayload(payload []byte, key, signature string) bool {
	expected := SignPayload(payload, key)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// SignRequest computes the HMAC-SHA256 request signature.
// The canonical form is: method|path|timestamp|bodySHA256
func SignRequest(method, path, timestamp string, bodySHA256 string, key string) string {
	canonical := fmt.Sprintf("%s|%s|%s|%s", method, path, timestamp, bodySHA256)
	return SignPayload([]byte(canonical), key)
}

// SHA256Hex computes the hex-encoded SHA-256 hash of data.
func SHA256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
