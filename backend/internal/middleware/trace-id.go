package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/defined"
)

// traceIDBytes is the random length of a generated trace ID: 4 bytes are 8 hex
// characters, short enough to read in a log line and to quote in a bug report.
// It only has to tell apart the requests in flight around one another, not be
// globally unique.
const traceIDBytes = 4

// newTraceID returns a short random hex ID (crypto/rand, so IDs are not guessable
// from one another).
func newTraceID() string {
	b := make([]byte, traceIDBytes)
	_, _ = rand.Read(b) // never fails on the platforms Go supports
	return hex.EncodeToString(b)
}

// TraceID stamps every request's context with a correlation ID — reused
// from the client's X-Trace-Id header if present, generated otherwise — so
// every log line for this request can be tied back to it, and echoes it on
// the response. Register it first so downstream middleware/handlers already
// have it in context.
func TraceID(c *gin.Context) {
	id := c.GetHeader(defined.TraceIDHeader)
	if id == "" {
		id = newTraceID()
	}
	c.Header(defined.TraceIDHeader, id)
	c.Request = c.Request.WithContext(logs.ContextWithTraceID(c.Request.Context(), id))
	c.Next()
}
