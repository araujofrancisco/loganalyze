package server

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := newRateLimiter(3, 1*time.Minute)

	if !rl.allow("test-ip") {
		t.Error("first request should be allowed")
	}
	if !rl.allow("test-ip") {
		t.Error("second request should be allowed")
	}
	if !rl.allow("test-ip") {
		t.Error("third request should be allowed")
	}
	if rl.allow("test-ip") {
		t.Error("fourth request should be blocked")
	}
}

func TestRateLimiterDifferentKeys(t *testing.T) {
	rl := newRateLimiter(2, 1*time.Minute)

	if !rl.allow("ip-a") {
		t.Error("ip-a first should be allowed")
	}
	if !rl.allow("ip-b") {
		t.Error("ip-b first should be allowed")
	}
	if !rl.allow("ip-a") {
		t.Error("ip-a second should be allowed")
	}
	if !rl.allow("ip-b") {
		t.Error("ip-b second should be allowed")
	}
	if rl.allow("ip-a") {
		t.Error("ip-a third should be blocked")
	}
	if rl.allow("ip-b") {
		t.Error("ip-b third should be blocked")
	}
}

func TestRateLimiterExpiry(t *testing.T) {
	rl := newRateLimiter(1, 50*time.Millisecond)

	if !rl.allow("test-ip") {
		t.Error("first request should be allowed")
	}
	if rl.allow("test-ip") {
		t.Error("second request should be blocked (within window)")
	}

	time.Sleep(60 * time.Millisecond)

	if !rl.allow("test-ip") {
		t.Error("request after window expiry should be allowed")
	}
}

func TestRateLimiterConcurrent(t *testing.T) {
	rl := newRateLimiter(10, 1*time.Minute)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rl.allow("test-ip")
		}()
	}
	wg.Wait()
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := newRateLimiter(2, 1*time.Minute)
	mw := rateLimitMiddleware(rl)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Errorf("first request: status = %d, want 200", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Errorf("second request: status = %d, want 200", rec2.Code)
	}

	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req)
	if rec3.Code != http.StatusTooManyRequests {
		t.Errorf("third request: status = %d, want 429", rec3.Code)
	}
	if rec3.Header().Get("Retry-After") == "" {
		t.Error("should have Retry-After header")
	}
}

func TestRateLimitMiddlewareRespectsXForwardedFor(t *testing.T) {
	rl := newRateLimiter(1, 1*time.Minute)
	mw := rateLimitMiddleware(rl)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Errorf("first request: status = %d, want 200", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: status = %d, want 429", rec2.Code)
	}
}
