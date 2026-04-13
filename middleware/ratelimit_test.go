package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()
	
	if config.RequestsPerMinute != 60 {
		t.Errorf("Expected RequestsPerMinute 60, got %d", config.RequestsPerMinute)
	}
	
	if config.BurstSize != 10 {
		t.Errorf("Expected BurstSize 10, got %d", config.BurstSize)
	}
	
	if config.KeyFunc == nil {
		t.Error("Expected KeyFunc to be set")
	}
	
	if config.OnLimitExceeded == nil {
		t.Error("Expected OnLimitExceeded to be set")
	}
}

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name           string
		remoteAddr     string
		xForwardedFor  string
		xRealIP        string
		expectedIP     string
	}{
		{
			name:       "RemoteAddr only",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:          "X-Forwarded-For single IP",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1",
			expectedIP:    "203.0.113.1",
		},
		{
			name:          "X-Forwarded-For multiple IPs",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1, 10.0.0.2, 10.0.0.3",
			expectedIP:    "203.0.113.1",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xRealIP:    "203.0.113.2",
			expectedIP: "203.0.113.2",
		},
		{
			name:          "X-Forwarded-For takes precedence over X-Real-IP",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1",
			xRealIP:       "203.0.113.2",
			expectedIP:    "203.0.113.1",
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = test.remoteAddr
			
			if test.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", test.xForwardedFor)
			}
			
			if test.xRealIP != "" {
				req.Header.Set("X-Real-IP", test.xRealIP)
			}
			
			ip := extractClientIP(req)
			if ip != test.expectedIP {
				t.Errorf("Expected IP %s, got %s", test.expectedIP, ip)
			}
		})
	}
}

func TestParseFirstIP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"192.168.1.1", "192.168.1.1"},
		{"192.168.1.1, 10.0.0.1", "192.168.1.1"},
		{"203.0.113.1, 10.0.0.1, 172.16.0.1", "203.0.113.1"},
		{"", ""},
	}
	
	for _, test := range tests {
		result := parseFirstIP(test.input)
		if result != test.expected {
			t.Errorf("parseFirstIP(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestTokenBucket(t *testing.T) {
	bucket := newBucket(10, 1) // 10 tokens max, 1 token per second
	
	// Should be able to consume up to max tokens initially
	for i := 0; i < 10; i++ {
		if !bucket.tryConsume(1) {
			t.Errorf("Should be able to consume token %d", i+1)
		}
	}
	
	// Should not be able to consume more
	if bucket.tryConsume(1) {
		t.Error("Should not be able to consume more tokens than capacity")
	}
	
	// Wait for refill and try again
	time.Sleep(1100 * time.Millisecond) // Wait a bit more than 1 second
	
	if !bucket.tryConsume(1) {
		t.Error("Should be able to consume token after refill")
	}
}

func TestTokenBucketRefill(t *testing.T) {
	bucket := newBucket(5, 2) // 5 tokens max, 2 tokens per second
	
	// Consume all tokens
	for i := 0; i < 5; i++ {
		bucket.tryConsume(1)
	}
	
	// Wait for partial refill
	time.Sleep(500 * time.Millisecond) // Should refill 1 token
	
	if !bucket.tryConsume(1) {
		t.Error("Should be able to consume 1 token after partial refill")
	}
	
	// Should not be able to consume another
	if bucket.tryConsume(1) {
		t.Error("Should not be able to consume more than refilled tokens")
	}
}

func TestTokenBucketMaxTokens(t *testing.T) {
	bucket := newBucket(3, 10) // 3 tokens max, 10 tokens per second (high rate)
	
	// Consume all tokens
	for i := 0; i < 3; i++ {
		bucket.tryConsume(1)
	}
	
	// Wait longer than needed to fill bucket multiple times
	time.Sleep(2 * time.Second)
	
	// Should only be able to consume max tokens, not more
	tokens := 0
	for i := 0; i < 10; i++ {
		if bucket.tryConsume(1) {
			tokens++
		}
	}
	
	if tokens != 3 {
		t.Errorf("Expected to consume 3 tokens (max capacity), got %d", tokens)
	}
}

func TestRateLimiterAllow(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 60, // 1 per second
		BurstSize:         5,
		KeyFunc:           func(r *http.Request) string { return "test-key" },
		OnLimitExceeded:   defaultRateLimitHandler,
	}
	
	limiter := NewRateLimiter(config)
	defer limiter.Stop()
	
	// Should allow up to burst size
	for i := 0; i < 5; i++ {
		if !limiter.Allow("test-key") {
			t.Errorf("Should allow request %d within burst", i+1)
		}
	}
	
	// Should reject the next request
	if limiter.Allow("test-key") {
		t.Error("Should reject request exceeding burst")
	}
	
	// Wait for refill and try again
	time.Sleep(1100 * time.Millisecond)
	
	if !limiter.Allow("test-key") {
		t.Error("Should allow request after refill")
	}
}

func TestRateLimiterMultipleKeys(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         2,
		KeyFunc:           func(r *http.Request) string { return "test-key" },
		OnLimitExceeded:   defaultRateLimitHandler,
	}
	
	limiter := NewRateLimiter(config)
	defer limiter.Stop()
	
	// Key1 should allow 2 requests
	if !limiter.Allow("key1") || !limiter.Allow("key1") {
		t.Error("Should allow 2 requests for key1")
	}
	
	// Key2 should also allow 2 requests (independent bucket)
	if !limiter.Allow("key2") || !limiter.Allow("key2") {
		t.Error("Should allow 2 requests for key2")
	}
	
	// Both keys should reject next request
	if limiter.Allow("key1") {
		t.Error("Should reject additional request for key1")
	}
	
	if limiter.Allow("key2") {
		t.Error("Should reject additional request for key2")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         2,
		KeyFunc:           extractClientIP,
		OnLimitExceeded:   defaultRateLimitHandler,
	}
	
	limiter := NewRateLimiter(config)
	defer limiter.Stop()
	
	middleware := limiter.Middleware()
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// First two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		recorder := httptest.NewRecorder()
		
		handler.ServeHTTP(recorder, req)
		
		if recorder.Code != http.StatusOK {
			t.Errorf("Request %d should succeed, got status %d", i+1, recorder.Code)
		}
		
		// Check rate limit headers
		if recorder.Header().Get("X-RateLimit-Limit") != "60" {
			t.Errorf("Expected X-RateLimit-Limit: 60, got %s", recorder.Header().Get("X-RateLimit-Limit"))
		}
	}
	
	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	recorder := httptest.NewRecorder()
	
	handler.ServeHTTP(recorder, req)
	
	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", recorder.Code)
	}
	
	if recorder.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("Expected X-RateLimit-Remaining: 0, got %s", recorder.Header().Get("X-RateLimit-Remaining"))
	}
	
	if recorder.Header().Get("Retry-After") != "60" {
		t.Errorf("Expected Retry-After: 60, got %s", recorder.Header().Get("Retry-After"))
	}
}

func TestRateLimitMiddlewareDifferentIPs(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         1,
		KeyFunc:           extractClientIP,
		OnLimitExceeded:   defaultRateLimitHandler,
	}
	
	limiter := NewRateLimiter(config)
	defer limiter.Stop()
	
	middleware := limiter.Middleware()
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// Requests from different IPs should be treated independently
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12345", "10.0.0.1:54321"}
	
	for _, ip := range ips {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip
		recorder := httptest.NewRecorder()
		
		handler.ServeHTTP(recorder, req)
		
		if recorder.Code != http.StatusOK {
			t.Errorf("Request from IP %s should succeed, got status %d", ip, recorder.Code)
		}
	}
}

func TestRateLimitSimple(t *testing.T) {
	middleware := RateLimitSimple(30) // 30 requests per minute
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// Should allow burst of 5 requests (30/6)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		recorder := httptest.NewRecorder()
		
		handler.ServeHTTP(recorder, req)
		
		if recorder.Code != http.StatusOK {
			t.Errorf("Request %d should succeed, got status %d", i+1, recorder.Code)
		}
	}
	
	// Next request should be limited
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	recorder := httptest.NewRecorder()
	
	handler.ServeHTTP(recorder, req)
	
	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", recorder.Code)
	}
}

func TestRateLimitByUserID(t *testing.T) {
	userIDExtractor := func(r *http.Request) string {
		return r.Header.Get("User-ID")
	}
	
	middleware := RateLimitByUserID(60, userIDExtractor)
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// User 1 should get their own rate limit
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("User-ID", "user1")
		req.RemoteAddr = "192.168.1.1:12345"
		recorder := httptest.NewRecorder()
		
		handler.ServeHTTP(recorder, req)
		
		if recorder.Code != http.StatusOK {
			t.Errorf("User1 request %d should succeed, got status %d", i+1, recorder.Code)
		}
	}
	
	// User 2 should also get their own rate limit (independent)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("User-ID", "user2")
		req.RemoteAddr = "192.168.1.1:12345" // Same IP as user1
		recorder := httptest.NewRecorder()
		
		handler.ServeHTTP(recorder, req)
		
		if recorder.Code != http.StatusOK {
			t.Errorf("User2 request %d should succeed, got status %d", i+1, recorder.Code)
		}
	}
	
	// User1 should now be rate limited
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-ID", "user1")
	req.RemoteAddr = "192.168.1.1:12345"
	recorder := httptest.NewRecorder()
	
	handler.ServeHTTP(recorder, req)
	
	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("User1 should be rate limited, got status %d", recorder.Code)
	}
}

func TestRateLimitByUserIDFallbackToIP(t *testing.T) {
	userIDExtractor := func(r *http.Request) string {
		return r.Header.Get("User-ID") // Will be empty
	}
	
	middleware := RateLimitByUserID(60, userIDExtractor)
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// Request without User-ID should fall back to IP-based limiting
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	recorder := httptest.NewRecorder()
	
	handler.ServeHTTP(recorder, req)
	
	if recorder.Code != http.StatusOK {
		t.Errorf("Request should succeed with IP fallback, got status %d", recorder.Code)
	}
}

func TestRateLimitGlobal(t *testing.T) {
	middleware := RateLimitGlobal(60) // 60 requests per minute globally
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// Allow burst of 10 requests from any IP
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12345", "10.0.0.1:54321"}
	
	requestCount := 0
	for _, ip := range ips {
		for i := 0; i < 4; i++ { // 3 IPs × 4 requests = 12 requests total
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = ip
			recorder := httptest.NewRecorder()
			
			handler.ServeHTTP(recorder, req)
			requestCount++
			
			if requestCount <= 10 {
				// First 10 requests should succeed (burst size)
				if recorder.Code != http.StatusOK {
					t.Errorf("Request %d should succeed globally, got status %d", requestCount, recorder.Code)
				}
			} else {
				// Subsequent requests should be limited
				if recorder.Code != http.StatusTooManyRequests {
					t.Errorf("Request %d should be limited globally, got status %d", requestCount, recorder.Code)
				}
			}
		}
	}
}

func TestDefaultRateLimitHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	recorder := httptest.NewRecorder()
	
	defaultRateLimitHandler(recorder, req)
	
	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", recorder.Code)
	}
	
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type: application/json, got %s", contentType)
	}
	
	if retryAfter := recorder.Header().Get("Retry-After"); retryAfter != "60" {
		t.Errorf("Expected Retry-After: 60, got %s", retryAfter)
	}
	
	body := recorder.Body.String()
	if !strings.Contains(body, "Rate limit exceeded") {
		t.Errorf("Expected rate limit error message in body, got %s", body)
	}
}

func TestCustomOnLimitExceeded(t *testing.T) {
	customHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("Custom rate limit response"))
	}
	
	config := RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         1,
		KeyFunc:           extractClientIP,
		OnLimitExceeded:   customHandler,
	}
	
	limiter := NewRateLimiter(config)
	defer limiter.Stop()
	
	middleware := limiter.Middleware()
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// First request should succeed
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	recorder1 := httptest.NewRecorder()
	handler.ServeHTTP(recorder1, req1)
	
	if recorder1.Code != http.StatusOK {
		t.Errorf("First request should succeed, got status %d", recorder1.Code)
	}
	
	// Second request should use custom handler
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	recorder2 := httptest.NewRecorder()
	handler.ServeHTTP(recorder2, req2)
	
	if recorder2.Code != http.StatusTeapot {
		t.Errorf("Expected custom status 418, got %d", recorder2.Code)
	}
	
	if body := recorder2.Body.String(); body != "Custom rate limit response" {
		t.Errorf("Expected custom response body, got %s", body)
	}
}

func BenchmarkRateLimiterAllow(b *testing.B) {
	config := DefaultRateLimitConfig()
	limiter := NewRateLimiter(config)
	defer limiter.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "key" + strconv.Itoa(i%100) // Use 100 different keys
			limiter.Allow(key)
			i++
		}
	})
}

func BenchmarkRateLimitMiddleware(b *testing.B) {
	middleware := RateLimitSimple(10000) // High limit to avoid blocking
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
	}
}