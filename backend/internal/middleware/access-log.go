package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/base/logs"
)

// AccessLog writes one line per request (method, path, status, duration)
// carrying the trace ID, at Info for success/redirect/4xx and Error for
// 5xx. Register it after TraceID and *before* Recover so a recovered
// panic's 500 is what gets logged.
func AccessLog(c *gin.Context) {
	start := time.Now()
	c.Next()

	status := c.Writer.Status()
	args := []any{
		"method", c.Request.Method, "path", c.Request.URL.Path,
		"status", status, "dur_ms", time.Since(start).Milliseconds(),
	}
	l := logs.WithCtx(c.Request.Context())
	if status >= http.StatusInternalServerError {
		l.Error("request", args...)
		return
	}
	l.Info("request", args...)
}
