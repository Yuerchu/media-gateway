package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupConcurrencyRouter(max int) *gin.Engine {
	r := gin.New()
	r.GET("/test", ConcurrencyLimiter(max), func(c *gin.Context) {
		time.Sleep(50 * time.Millisecond)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestConcurrencyLimiter_AllowsUnderLimit(t *testing.T) {
	r := setupConcurrencyRouter(5)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestConcurrencyLimiter_RejectsOverLimit(t *testing.T) {
	r := setupConcurrencyRouter(1)

	started := make(chan struct{})
	done := make(chan struct{})

	go func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		close(started)
		r.ServeHTTP(w, req)
		close(done)
	}()

	<-started
	time.Sleep(10 * time.Millisecond)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("concurrent request status = %d, want 429", w2.Code)
	}

	<-done
}

func TestConcurrencyLimiter_ReleasesSlot(t *testing.T) {
	r := setupConcurrencyRouter(1)

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest("GET", "/test", nil))
	if w1.Code != http.StatusOK {
		t.Fatalf("first request failed: %d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/test", nil))
	if w2.Code != http.StatusOK {
		t.Errorf("sequential request should succeed after slot release: %d", w2.Code)
	}
}

func TestConcurrencyLimiter_MultipleSlots(t *testing.T) {
	r := setupConcurrencyRouter(3)

	var wg sync.WaitGroup
	codes := make([]int, 3)
	for i := range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
			codes[i] = w.Code
		}()
	}
	wg.Wait()

	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("request[%d] status = %d, want 200 (within limit)", i, code)
		}
	}
}
