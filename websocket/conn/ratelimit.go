package conn

import (
	"sync"
	"time"

	"github.com/gnemade360/go-server/websocket/internal/types"
)

// NewTokenBucketRateLimiter returns a per-connection limiter capped at
// messagesPerSecond and bytesPerSecond, with a one-second burst. A zero
// argument disables that dimension; both zero returns nil.
func NewTokenBucketRateLimiter(messagesPerSecond, bytesPerSecond float64) types.MessageRateLimiter {
	if messagesPerSecond <= 0 && bytesPerSecond <= 0 {
		return nil
	}
	now := time.Now()
	return &tokenBucket{
		msgRate:   messagesPerSecond,
		msgBurst:  messagesPerSecond,
		msgTok:    messagesPerSecond,
		byteRate:  bytesPerSecond,
		byteBurst: bytesPerSecond,
		byteTok:   bytesPerSecond,
		last:      now,
	}
}

type tokenBucket struct {
	mu sync.Mutex

	msgRate, msgBurst, msgTok    float64
	byteRate, byteBurst, byteTok float64

	last time.Time
}

func (b *tokenBucket) Allow(n int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now

	if b.msgRate > 0 {
		b.msgTok = clamp(b.msgTok+elapsed*b.msgRate, 0, b.msgBurst)
	}
	if b.byteRate > 0 {
		b.byteTok = clamp(b.byteTok+elapsed*b.byteRate, 0, b.byteBurst)
	}

	if b.msgRate > 0 && b.msgTok < 1 {
		return false
	}
	if b.byteRate > 0 && b.byteTok < float64(n) {
		return false
	}

	if b.msgRate > 0 {
		b.msgTok--
	}
	if b.byteRate > 0 {
		b.byteTok -= float64(n)
	}
	return true
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
