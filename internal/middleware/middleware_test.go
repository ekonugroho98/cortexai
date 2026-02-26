package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cortexai/cortexai/internal/middleware"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
})

// ─── Security Headers ─────────────────────────────────────────────────────────

func TestSecurityHeaders(t *testing.T) {
	handler := middleware.SecurityHeaders(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
	}
	for h, want := range headers {
		if got := rr.Header().Get(h); got != want {
			t.Errorf("header %s = %q, want %q", h, got, want)
		}
	}
	// CSP and HSTS should be non-empty
	if rr.Header().Get("Content-Security-Policy") == "" {
		t.Error("Content-Security-Policy header missing")
	}
	if rr.Header().Get("Strict-Transport-Security") == "" {
		t.Error("Strict-Transport-Security header missing")
	}
}

// ─── Request ID ───────────────────────────────────────────────────────────────

func TestRequestIDGenerated(t *testing.T) {
	handler := middleware.RequestID(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	id := rr.Header().Get("X-Request-ID")
	if id == "" {
		t.Error("X-Request-ID should be generated if not present")
	}
}

func TestRequestIDPropagated(t *testing.T) {
	handler := middleware.RequestID(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "my-trace-id-123")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Request-ID"); got != "my-trace-id-123" {
		t.Errorf("X-Request-ID should propagate existing ID, got %q", got)
	}
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

func TestAuthMissingKey(t *testing.T) {
	h := middleware.Auth([]string{"secret"}, "X-API-Key")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/datasets", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthInvalidKey(t *testing.T) {
	h := middleware.Auth([]string{"secret"}, "X-API-Key")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/datasets", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAuthValidKey(t *testing.T) {
	h := middleware.Auth([]string{"secret"}, "X-API-Key")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/datasets", nil)
	req.Header.Set("X-API-Key", "secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuthPublicPath(t *testing.T) {
	h := middleware.Auth([]string{"secret"}, "X-API-Key")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// No API key set
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("health endpoint should be public, got %d", rr.Code)
	}
}

// ─── Rate Limiter ─────────────────────────────────────────────────────────────

func TestRateLimiter(t *testing.T) {
	limit := 3
	h := middleware.RateLimit(limit)(okHandler)

	for i := 0; i < limit; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after limit, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header missing on rate limit response")
	}
}

func TestRateLimiterDifferentClients(t *testing.T) {
	limit := 2
	h := middleware.RateLimit(limit)(okHandler)

	// Client A uses up its limit
	for i := 0; i < limit; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
	}

	// Client B should still be allowed
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:1"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("different client should not be rate-limited, got %d", rr.Code)
	}
}

// ─── Recovery ─────────────────────────────────────────────────────────────────

func TestRecovery(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("intentional panic for test")
	})
	h := middleware.Recovery(panicHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	// Should not panic the test
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on panic, got %d", rr.Code)
	}
}

// ─── CORS ─────────────────────────────────────────────────────────────────────

func TestCORSPreflight(t *testing.T) {
	cfg := middleware.DefaultCORSConfig([]string{"http://localhost:3000"})
	h := middleware.CORS(cfg)(okHandler)

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("preflight should return 204, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("Access-Control-Allow-Origin header missing")
	}
}

func TestCORSUnknownOrigin(t *testing.T) {
	cfg := middleware.DefaultCORSConfig([]string{"http://localhost:3000"})
	h := middleware.CORS(cfg)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://evil.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("unknown origin should not get CORS header")
	}
}
