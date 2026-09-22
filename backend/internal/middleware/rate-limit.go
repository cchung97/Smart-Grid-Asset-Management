package middleware

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/defined"
)

const (
	rateLimitCleanupEvery = 5 * time.Minute
)

type clientWindow struct {
	mu    sync.Mutex
	count uint
	start time.Time
}

// RateLimiter is a simple in-memory, per-(client IP, path) fixed-window
// limiter — process-local, no shared store. That's the right trade-off
// for this project (a single backend instance, no horizontal scaling);
// a multi-instance deployment would need a shared store (e.g. Redis)
// instead.
type RateLimiter struct {
	limit   uint
	window  time.Duration
	clients sync.Map // key: "ip|path" -> *clientWindow
}

// NewRateLimiter starts a limiter and its background cleanup goroutine,
// which stops when ctx is done. limit <= 0 and window <= 0 fall back to
// sane defaults (100 requests / 10s).
func NewRateLimiter(ctx context.Context, limit uint, window time.Duration) *RateLimiter {
	if limit == 0 {
		limit = defined.DefaultRateLimit
	}
	if window <= 0 {
		window = defined.DefaultRateWindow
	}
	rl := &RateLimiter{limit: limit, window: window}
	go rl.cleanupLoop(ctx)
	return rl
}

func (rl *RateLimiter) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(rateLimitCleanupEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.clients.Range(func(key, value any) bool {
				cw := value.(*clientWindow)
				cw.mu.Lock()
				expired := time.Since(cw.start) >= rl.window
				cw.mu.Unlock()
				if expired {
					rl.clients.Delete(key)
				}
				return true
			})
		}
	}
}

// Middleware enforces the limit, keyed by client IP + request path.
func (rl *RateLimiter) Middleware(c *gin.Context) {
	ip := clientIP(c.Request)
	key := ip + "|" + c.Request.URL.Path

	v, _ := rl.clients.LoadOrStore(key, &clientWindow{start: time.Now()})
	cw := v.(*clientWindow)

	cw.mu.Lock()
	if time.Since(cw.start) >= rl.window {
		cw.count = 0
		cw.start = time.Now()
	}
	cw.count++
	exceeded := cw.count > rl.limit
	cw.mu.Unlock()

	if exceeded {
		logs.WithCtx(c.Request.Context()).Warn("rate limit exceeded", "ip", ip, "path", c.Request.URL.Path)
		abort(c, http.StatusTooManyRequests, "too many requests")
		return
	}
	c.Next()
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
