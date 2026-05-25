package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yuerchu/media-gateway/pkg/signing"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupAuthRouter(secret string) *gin.Engine {
	r := gin.New()
	r.POST("/test", HMACAuth(secret), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func signedRequest(method, path, body, secret string) *http.Request {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	bodySHA := signing.SHA256Hex([]byte(body))
	sig := signing.SignRequest(method, path, ts, bodySHA, secret)

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("X-MG-Timestamp", ts)
	req.Header.Set("X-MG-Signature", sig)
	return req
}

func TestHMACAuth_EmptySecret_SkipsAuth(t *testing.T) {
	r := setupAuthRouter("")
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (auth skipped with empty secret)", w.Code)
	}
}

func TestHMACAuth_ValidSignature(t *testing.T) {
	secret := "test-secret"
	r := setupAuthRouter(secret)
	w := httptest.NewRecorder()
	req := signedRequest("POST", "/test", `{"data":"value"}`, secret)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestHMACAuth_MissingHeaders(t *testing.T) {
	r := setupAuthRouter("secret")
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHMACAuth_MissingTimestamp(t *testing.T) {
	r := setupAuthRouter("secret")
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-MG-Signature", "somesig")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHMACAuth_MissingSignature(t *testing.T) {
	r := setupAuthRouter("secret")
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-MG-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHMACAuth_InvalidTimestamp(t *testing.T) {
	r := setupAuthRouter("secret")
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-MG-Timestamp", "not-a-number")
	req.Header.Set("X-MG-Signature", "somesig")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHMACAuth_ExpiredTimestamp(t *testing.T) {
	secret := "secret"
	r := setupAuthRouter(secret)
	w := httptest.NewRecorder()

	staleTs := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	body := ""
	bodySHA := signing.SHA256Hex([]byte(body))
	sig := signing.SignRequest("POST", "/test", staleTs, bodySHA, secret)

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-MG-Timestamp", staleTs)
	req.Header.Set("X-MG-Signature", sig)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for expired timestamp", w.Code)
	}
}

func TestHMACAuth_WrongSignature(t *testing.T) {
	r := setupAuthRouter("secret")
	w := httptest.NewRecorder()

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-MG-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	req.Header.Set("X-MG-Signature", "deadbeef")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for wrong signature", w.Code)
	}
}

func TestHMACAuth_WrongKey(t *testing.T) {
	r := setupAuthRouter("correct-secret")
	w := httptest.NewRecorder()
	req := signedRequest("POST", "/test", "", "wrong-secret")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for wrong key", w.Code)
	}
}
