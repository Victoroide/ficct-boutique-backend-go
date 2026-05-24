package middleware

import (
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	tokens     float64
	lastRefill time.Time
}

type RateLimiter struct {
	rps      float64
	burst    float64
	mu       sync.Mutex
	buckets  map[string]*bucket
	ticker   *time.Ticker
	stopChan chan struct{}
}

func NewRateLimiter(rps, burst int) *RateLimiter {
	rl := &RateLimiter{
		rps:      float64(rps),
		burst:    float64(burst),
		buckets:  make(map[string]*bucket),
		ticker:   time.NewTicker(1 * time.Minute),
		stopChan: make(chan struct{}),
	}
	go rl.gcLoop()
	return rl
}

func (rl *RateLimiter) gcLoop() {
	for {
		select {
		case <-rl.stopChan:
			return
		case <-rl.ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-5 * time.Minute)
			for k, b := range rl.buckets {
				if b.lastRefill.Before(cutoff) {
					delete(rl.buckets, k)
				}
			}
			rl.mu.Unlock()
		}
	}
}

func (rl *RateLimiter) Stop() {
	close(rl.stopChan)
	rl.ticker.Stop()
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{tokens: rl.burst, lastRefill: now}
		rl.buckets[key] = b
	}
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * rl.rps
	if b.tokens > rl.burst {
		b.tokens = rl.burst
	}
	b.lastRefill = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientKey(r)
		if !rl.allow(key) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientKey(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}
