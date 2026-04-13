package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	// RequestsPerMinute is the number of requests allowed per minute
	RequestsPerMinute int
	// BurstSize is the maximum number of requests that can be made at once
	BurstSize int
	// KeyFunc extracts the key from the request (e.g., IP address, user ID)
	KeyFunc func(*http.Request) string
	// OnLimitExceeded is called when rate limit is exceeded
	OnLimitExceeded func(http.ResponseWriter, *http.Request)
}

// DefaultRateLimitConfig returns a default rate limit configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerMinute: 60,  // 60 requests per minute
		BurstSize:         10,  // Allow bursts of up to 10 requests
		KeyFunc:           extractClientIP,
		OnLimitExceeded:   defaultRateLimitHandler,
	}
}

// extractClientIP extracts the client IP address from the request
func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxy/load balancer scenarios)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if ip := parseFirstIP(xff); ip != "" {
			return ip
		}
	}

	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// parseFirstIP extracts the first IP address from a comma-separated list
func parseFirstIP(xff string) string {
	for i, char := range xff {
		if char == ',' {
			return xff[:i]
		}
	}
	return xff
}

// defaultRateLimitHandler is the default handler for rate limit exceeded
func defaultRateLimitHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "60") // Suggest retry after 60 seconds
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"error": "Rate limit exceeded", "message": "Too many requests"}`))
}

// bucket represents a token bucket for rate limiting
type bucket struct {
	tokens      float64
	lastRefill  time.Time
	maxTokens   float64
	refillRate  float64 // tokens per second
	mu          sync.Mutex
}

// newBucket creates a new token bucket
func newBucket(maxTokens float64, refillRate float64) *bucket {
	return &bucket{
		tokens:     maxTokens,
		lastRefill: time.Now(),
		maxTokens:  maxTokens,
		refillRate: refillRate,
	}
}

// tryConsume tries to consume tokens from the bucket
func (b *bucket) tryConsume(tokens float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	
	// Refill tokens based on elapsed time
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now

	// Check if we have enough tokens
	if b.tokens >= tokens {
		b.tokens -= tokens
		return true
	}
	return false
}

// RateLimiter manages rate limiting for multiple clients
type RateLimiter struct {
	buckets map[string]*bucket
	config  RateLimitConfig
	mu      sync.RWMutex
	
	// cleanup goroutine management
	stopCleanup chan struct{}
	cleanupDone chan struct{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		buckets:     make(map[string]*bucket),
		config:      config,
		stopCleanup: make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}
	
	// Start cleanup goroutine to remove stale buckets
	go rl.cleanup()
	
	return rl
}

// cleanup removes unused buckets periodically
func (rl *RateLimiter) cleanup() {
	defer close(rl.cleanupDone)
	
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			rl.removeStableBuckets()
		case <-rl.stopCleanup:
			return
		}
	}
}

// removeStableBuckets removes buckets that haven't been used recently
func (rl *RateLimiter) removeStableBuckets() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	staleThreshold := 10 * time.Minute
	
	for key, bucket := range rl.buckets {
		bucket.mu.Lock()
		if now.Sub(bucket.lastRefill) > staleThreshold {
			delete(rl.buckets, key)
		}
		bucket.mu.Unlock()
	}
}

// Stop stops the rate limiter and cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
	<-rl.cleanupDone
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.RLock()
	bucket, exists := rl.buckets[key]
	rl.mu.RUnlock()
	
	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		bucket, exists = rl.buckets[key]
		if !exists {
			// Calculate refill rate: tokens per second
			refillRate := float64(rl.config.RequestsPerMinute) / 60.0
			bucket = newBucket(float64(rl.config.BurstSize), refillRate)
			rl.buckets[key] = bucket
		}
		rl.mu.Unlock()
	}
	
	return bucket.tryConsume(1.0)
}

// Middleware returns a middleware function for rate limiting
func (rl *RateLimiter) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rl.config.KeyFunc(r)
			
			if !rl.Allow(key) {
				// Add rate limit headers
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.config.RequestsPerMinute))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))
				
				rl.config.OnLimitExceeded(w, r)
				return
			}
			
			// Add rate limit headers for successful requests
			rl.mu.RLock()
			bucket := rl.buckets[key]
			rl.mu.RUnlock()
			
			if bucket != nil {
				bucket.mu.Lock()
				remaining := int(bucket.tokens)
				bucket.mu.Unlock()
				
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.config.RequestsPerMinute))
				w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit creates a rate limiting middleware with the given configuration
func RateLimit(config RateLimitConfig) Middleware {
	limiter := NewRateLimiter(config)
	return limiter.Middleware()
}

// RateLimitSimple creates a simple rate limiting middleware with requests per minute
func RateLimitSimple(requestsPerMinute int) Middleware {
	config := DefaultRateLimitConfig()
	config.RequestsPerMinute = requestsPerMinute
	config.BurstSize = requestsPerMinute / 6 // Allow burst of 1/6 of the rate
	if config.BurstSize < 1 {
		config.BurstSize = 1
	}
	return RateLimit(config)
}

// RateLimitByUserID creates a rate limiting middleware that uses user ID as the key
func RateLimitByUserID(requestsPerMinute int, userIDExtractor func(*http.Request) string) Middleware {
	config := DefaultRateLimitConfig()
	config.RequestsPerMinute = requestsPerMinute
	config.BurstSize = requestsPerMinute / 6
	if config.BurstSize < 1 {
		config.BurstSize = 1
	}
	config.KeyFunc = func(r *http.Request) string {
		userID := userIDExtractor(r)
		if userID == "" {
			return extractClientIP(r) // Fallback to IP
		}
		return fmt.Sprintf("user:%s", userID)
	}
	return RateLimit(config)
}

// RateLimitGlobal creates a global rate limiting middleware (same limit for all clients)
func RateLimitGlobal(requestsPerMinute int) Middleware {
	config := DefaultRateLimitConfig()
	config.RequestsPerMinute = requestsPerMinute
	config.BurstSize = requestsPerMinute / 6
	if config.BurstSize < 1 {
		config.BurstSize = 1
	}
	config.KeyFunc = func(r *http.Request) string {
		return "global" // Same key for all requests
	}
	return RateLimit(config)
}