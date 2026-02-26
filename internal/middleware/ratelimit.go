package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cortexai/cortexai/internal/models"
)

type slidingWindow struct {
	mu        sync.Mutex
	requests  []time.Time
	limit     int
	windowDur time.Duration
}

func (sw *slidingWindow) allow() (remaining int, ok bool) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sw.windowDur)

	// Drop old entries
	valid := sw.requests[:0]
	for _, t := range sw.requests {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	sw.requests = valid

	if len(sw.requests) >= sw.limit {
		return 0, false
	}
	sw.requests = append(sw.requests, now)
	return sw.limit - len(sw.requests), true
}

type RateLimiter struct {
	mu      sync.Mutex
	windows map[string]*slidingWindow
	limit   int
}

func NewRateLimiter(limitPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		windows: make(map[string]*slidingWindow),
		limit:   limitPerMinute,
	}
	// Cleanup goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()
	return rl
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-time.Minute)
	for key, sw := range rl.windows {
		sw.mu.Lock()
		if len(sw.requests) == 0 || sw.requests[len(sw.requests)-1].Before(cutoff) {
			delete(rl.windows, key)
		}
		sw.mu.Unlock()
	}
}

func (rl *RateLimiter) window(key string) *slidingWindow {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if sw, ok := rl.windows[key]; ok {
		return sw
	}
	sw := &slidingWindow{limit: rl.limit, windowDur: time.Minute}
	rl.windows[key] = sw
	return sw
}

func RateLimit(limitPerMinute int) func(http.Handler) http.Handler {
	rl := NewRateLimiter(limitPerMinute)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Key: prefer API key, fall back to IP
			key := r.Header.Get("X-API-Key")
			if key == "" {
				key = r.RemoteAddr
			}

			sw := rl.window(key)
			remaining, ok := sw.allow()

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limitPerMinute))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

			if !ok {
				w.Header().Set("Retry-After", "60")
				models.WriteError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
