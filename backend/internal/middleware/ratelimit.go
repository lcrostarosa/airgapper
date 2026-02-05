// Package middleware provides HTTP middleware for the API server
package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitConfig configures the rate limiter
type RateLimitConfig struct {
	// RequestsPerSecond is the rate limit (requests per second)
	RequestsPerSecond float64
	// BurstSize is the maximum burst size
	BurstSize int
	// CleanupInterval is how often to clean up old limiters
	CleanupInterval time.Duration
	// MaxAge is how long to keep unused limiters
	MaxAge time.Duration
}

// DefaultRateLimitConfig returns the default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerSecond: 10,
		BurstSize:         20,
		CleanupInterval:   time.Minute,
		MaxAge:            5 * time.Minute,
	}
}

// ipLimiter holds a rate limiter and last access time for an IP
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter implements per-IP rate limiting
type RateLimiter struct {
	config   *RateLimitConfig
	limiters map[string]*ipLimiter
	mu       sync.RWMutex
	stopCh   chan struct{}
}

// NewRateLimiter creates a new rate limiter with the given config
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	rl := &RateLimiter{
		config:   config,
		limiters: make(map[string]*ipLimiter),
		stopCh:   make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// getLimiter returns the rate limiter for the given IP, creating one if needed
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if il, exists := rl.limiters[ip]; exists {
		il.lastSeen = time.Now()
		return il.limiter
	}

	limiter := rate.NewLimiter(rate.Limit(rl.config.RequestsPerSecond), rl.config.BurstSize)
	rl.limiters[ip] = &ipLimiter{
		limiter:  limiter,
		lastSeen: time.Now(),
	}

	return limiter
}

// cleanupLoop periodically removes old limiters
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCh:
			return
		}
	}
}

// cleanup removes limiters that haven't been used recently
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.config.MaxAge)
	for ip, il := range rl.limiters {
		if il.lastSeen.Before(cutoff) {
			delete(rl.limiters, ip)
		}
	}
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// Middleware returns an HTTP middleware that applies rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		limiter := rl.getLimiter(ip)

		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (first IP is the client)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := findComma(xff); idx != -1 {
			return xff[:idx]
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// findComma finds the first comma in a string
func findComma(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}
