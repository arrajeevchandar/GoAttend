package httpmiddleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SimpleTokenBucket is an in-memory rate limiter; for prod swap to Redis.
type SimpleTokenBucket struct {
	capacity int
	rate     int
	mu       sync.Mutex
	state    map[string]*bucket
}

type bucket struct {
	tokens int
	last   time.Time
}

// NewSimpleTokenBucket creates limiter with capacity tokens and rate per minute.
func NewSimpleTokenBucket(capacity, perMinute int) *SimpleTokenBucket {
	if capacity <= 0 {
		capacity = perMinute
	}
	return &SimpleTokenBucket{
		capacity: capacity,
		rate:     perMinute,
		state:    make(map[string]*bucket),
	}
}

// GinMiddleware returns gin handler enforcing per-IP limits.
func (l *SimpleTokenBucket) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}
		if !l.allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit"})
			return
		}
		c.Next()
	}
}

func (l *SimpleTokenBucket) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.state[key]
	now := time.Now()
	if !ok {
		b = &bucket{tokens: l.capacity - 1, last: now}
		l.state[key] = b
		return true
	}
	elapsed := now.Sub(b.last).Minutes()
	refill := int(elapsed * float64(l.rate))
	if refill > 0 {
		b.tokens += refill
		if b.tokens > l.capacity {
			b.tokens = l.capacity
		}
		b.last = now
	}
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}
