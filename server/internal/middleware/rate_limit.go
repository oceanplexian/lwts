package middleware

import (
	"encoding/json"
	"math"
	"net"
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	tokens   float64
	lastTime time.Time
}

type tokenBucket struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // tokens per second
	capacity float64
}

func newTokenBucket(rate float64, capacity float64) *tokenBucket {
	return &tokenBucket{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		capacity: capacity,
	}
}

func (tb *tokenBucket) allow(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	b, ok := tb.buckets[key]
	if !ok {
		tb.buckets[key] = &bucket{tokens: tb.capacity - 1, lastTime: now}
		return true
	}

	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens = math.Min(tb.capacity, b.tokens+elapsed*tb.rate)
	b.lastTime = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func RateLimit(globalRate float64, globalCapacity float64) func(http.Handler) http.Handler {
	limiter := newTokenBucket(globalRate, globalCapacity)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			if ip == "" {
				ip = r.RemoteAddr
			}

			if !limiter.allow(ip) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "rate limit exceeded",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
